/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package certstore

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// filterUpstreamCertificates returns only the certificates whose Usage is
// upstream trust. A row with no usage stored is an upstream certificate.
func filterUpstreamCertificates(certs []*models.StoredCertificate) []*models.StoredCertificate {
	filtered := make([]*models.StoredCertificate, 0, len(certs))
	for _, cert := range certs {
		if cert.EffectiveUsage() == models.CertificateUsageUpstream {
			filtered = append(filtered, cert)
		}
	}
	return filtered
}

// generateCertificateID creates a unique ID for a certificate (UUID v7)
func generateCertificateID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate certificate UUID: %w", err)
	}
	return u.String(), nil
}

// CertStore manages custom certificates for upstream TLS verification
type CertStore struct {
	logger         *slog.Logger
	certsDir       string
	systemCertPath string
	combinedCerts  []byte
	db             storage.Storage
	mu             sync.RWMutex // Protects combinedCerts from concurrent access

	// encryptionManager decrypts gateway identity private keys. It is set
	// after construction; while nil, GetGatewayIdentityMaterial fails.
	encryptionManager *encryption.ProviderManager

	// loggedDefaultIdentities names the default identities the last
	// duplicate warning listed, so it is logged once per change.
	loggedDefaultIdentities string
	defaultIdentitiesLogMu  sync.Mutex
}

// SetEncryptionManager wires the encryption provider manager used to
// decrypt a gateway identity's private key ciphertext.
func (cs *CertStore) SetEncryptionManager(mgr *encryption.ProviderManager) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.encryptionManager = mgr
}

// NewCertStore creates a new certificate store
// db: database storage for custom certificates
// certsDir: legacy directory containing custom certificates (deprecated, for backward compatibility)
// systemCertPath: path to system CA certificates (e.g., "/etc/ssl/certs/ca-certificates.crt")
func NewCertStore(logger *slog.Logger, db storage.Storage, certsDir string, systemCertPath string) *CertStore {
	return &CertStore{
		logger:         logger,
		db:             db,
		certsDir:       certsDir,
		systemCertPath: systemCertPath,
	}
}

// LoadCertificates loads and combines custom certificates from database with system certificates
// Returns the combined PEM-encoded certificate bundle
func (cs *CertStore) LoadCertificates() ([]byte, error) {
	var certBuffer bytes.Buffer
	loadedCount := 0

	// Bootstrap: Sync filesystem certificates to database on first run
	if cs.certsDir != "" {
		if err := cs.bootstrapCertificatesFromFilesystem(); err != nil {
			cs.logger.Warn("Failed to bootstrap certificates from filesystem",
				slog.Any("error", err))
		}
	}

	// Load custom certificates from database (primary and only source for custom certs)
	dbCerts, count, err := cs.loadDatabaseCertificates()
	if err != nil {
		// A read failure must not yield a bundle with rows silently missing.
		return nil, fmt.Errorf("loading certificates from database: %w", err)
	}
	if count > 0 {
		certBuffer.Write(dbCerts)
		loadedCount += count
		cs.logger.Info("Loaded custom certificates from database",
			slog.Int("count", count))
	}

	// Load system certificates
	if cs.systemCertPath != "" {
		systemCerts, err := os.ReadFile(cs.systemCertPath)
		if err != nil {
			cs.logger.Warn("Failed to load system certificates",
				slog.String("path", cs.systemCertPath),
				slog.Any("error", err))
		} else {
			// Add system certificates to the buffer
			certBuffer.Write(systemCerts)
			cs.logger.Info("Loaded system certificates",
				slog.String("path", cs.systemCertPath))
		}
	}

	// An empty bundle is a valid state: the store still serves the listener
	// certificate, client-CA pool and gateway identities over SDS, and an
	// upstream definition that needs trust is refused at translation until
	// a certificate exists. It is loud, because every HTTPS upstream that
	// relies on the gateway bundle fails its handshake meanwhile.
	if certBuffer.Len() == 0 {
		cs.logger.Warn("No upstream trust certificates loaded; HTTPS upstreams without their own trustedCAs will fail until an upstream trust certificate is added",
			slog.String("custom_certs_path", cs.certsDir),
			slog.String("system_cert_path", cs.systemCertPath))
	}

	cs.mu.Lock()
	cs.combinedCerts = certBuffer.Bytes()
	cs.mu.Unlock()

	cs.logger.Info("Certificate trust store initialized",
		slog.Int("custom_certs", loadedCount),
		slog.Int("total_bytes", len(certBuffer.Bytes())))

	return certBuffer.Bytes(), nil
}

// loadDatabaseCertificates loads all upstream-trust certificates from the
// database. usage: downstream rows are never included: the two trust purposes
// must never share a bundle.
func (cs *CertStore) loadDatabaseCertificates() ([]byte, int, error) {
	if cs.db == nil {
		return nil, 0, nil
	}
	certs, err := cs.db.ListCertificates()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list certificates: %w", err)
	}

	certs = filterUpstreamCertificates(certs)

	if len(certs) == 0 {
		cs.logger.Debug("No certificates found in database")
		return nil, 0, nil
	}

	var certBuffer bytes.Buffer
	certCount := 0

	for _, cert := range certs {
		// Validate certificate data
		count, err := cs.validateCertificateData(cert.Name, cert.Certificate)
		if err != nil {
			cs.logger.Warn("Invalid certificate in database",
				slog.String("name", cert.Name),
				slog.String("id", cert.UUID),
				slog.Any("error", err))
			continue
		}

		if count > 0 {
			// Add certificate to buffer (ensure it ends with newline)
			certBuffer.Write(cert.Certificate)
			if !bytes.HasSuffix(cert.Certificate, []byte("\n")) {
				certBuffer.WriteString("\n")
			}
			certCount += count
			cs.logger.Debug("Loaded certificate from database",
				slog.String("name", cert.Name),
				slog.String("id", cert.UUID),
				slog.Int("certs_in_chain", count))
		}
	}

	return certBuffer.Bytes(), certCount, nil
}

// loadCustomCertificates loads all PEM certificates from the certificates directory
func (cs *CertStore) loadCustomCertificates() ([]byte, int, error) {
	// Check if directory exists
	if _, err := os.Stat(cs.certsDir); os.IsNotExist(err) {
		cs.logger.Debug("Certificates directory does not exist",
			slog.String("path", cs.certsDir))
		return nil, 0, nil
	}

	var certBuffer bytes.Buffer
	certCount := 0

	err := cs.walkCertificateFiles(func(path string, certData []byte) error {
		count, err := cs.validateAndExtractCertificates(path, certData)
		if err != nil {
			cs.logger.Warn("Invalid certificate file",
				slog.String("file", path),
				slog.Any("error", err))
			return nil // Continue with other files
		}

		if count > 0 {
			// Add certificate to buffer (ensure it ends with newline)
			certBuffer.Write(certData)
			if !bytes.HasSuffix(certData, []byte("\n")) {
				certBuffer.WriteString("\n")
			}
			certCount += count
			cs.logger.Debug("Loaded certificate file",
				slog.String("file", path),
				slog.Int("certs_in_file", count))
		}

		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to walk certificates directory: %w", err)
	}

	return certBuffer.Bytes(), certCount, nil
}

// validateAndExtractCertificates validates that the data contains valid PEM certificates
// Returns the number of valid certificates found
func (cs *CertStore) validateAndExtractCertificates(filename string, data []byte) (int, error) {
	count := 0
	rest := data

	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		// Only accept CERTIFICATE blocks
		if block.Type != "CERTIFICATE" {
			cs.logger.Debug("Skipping non-certificate PEM block",
				slog.String("file", filename),
				slog.String("type", block.Type))
			continue
		}

		// Parse the certificate to ensure it's valid
		_, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return 0, fmt.Errorf("invalid certificate in file: %w", err)
		}

		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid certificates found in file")
	}

	return count, nil
}

// validateCertificateData validates that the data contains valid PEM certificates
// Returns the number of valid certificates found
func (cs *CertStore) validateCertificateData(name string, data []byte) (int, error) {
	count := 0
	rest := data

	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		// Only accept CERTIFICATE blocks
		if block.Type != "CERTIFICATE" {
			cs.logger.Debug("Skipping non-certificate PEM block",
				slog.String("name", name),
				slog.String("type", block.Type))
			continue
		}

		// Parse the certificate to ensure it's valid
		_, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return 0, fmt.Errorf("invalid certificate: %w", err)
		}

		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid certificates found")
	}

	return count, nil
}

// GetCombinedCertificates returns the combined certificate bundle
// Returns nil if LoadCertificates hasn't been called yet
func (cs *CertStore) GetCombinedCertificates() []byte {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if cs.combinedCerts == nil {
		return nil
	}
	// Return a copy to prevent external modifications
	result := make([]byte, len(cs.combinedCerts))
	copy(result, cs.combinedCerts)
	return result
}

// GetClientCABundle returns the concatenated PEM bundle of every usage:
// downstream certificate, in store order. It returns (nil, nil) for an empty
// pool or a store with no database; deploy-time validation keeps mtls-auth
// off an empty pool.
func (cs *CertStore) GetClientCABundle() ([]byte, error) {
	bundle, _, err := cs.GetClientCAPool()
	return bundle, err
}

// GetClientCAPool returns the client-CA pool bundle, as GetClientCABundle
// does, and whether any entry has role: relay.
func (cs *CertStore) GetClientCAPool() (bundle []byte, hasRelay bool, err error) {
	if cs.db == nil {
		return nil, false, nil
	}
	certs, err := cs.db.ListCertificatesByUsage(models.CertificateUsageDownstream)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list client-CA pool: %w", err)
	}

	var buf bytes.Buffer
	for _, cert := range certs {
		if cert.EffectiveRole() == models.CertificateRoleRelay {
			hasRelay = true
		}
		buf.Write(cert.Certificate)
		if !bytes.HasSuffix(cert.Certificate, []byte("\n")) {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), hasRelay, nil
}

// GetGatewayIdentityMaterial resolves a usage: identity row by name to its
// PEM certificate chain, leaf first, and decrypted private key. It never
// returns a partial or still-encrypted result.
func (cs *CertStore) GetGatewayIdentityMaterial(name string) (certChainPEM []byte, privateKeyPEM []byte, err error) {
	if cs.db == nil {
		return nil, nil, fmt.Errorf("gateway identity %q not found: no certificate database", name)
	}
	cert, err := cs.db.GetCertificateByName(name)
	if err != nil {
		return nil, nil, fmt.Errorf("gateway identity %q not found: %w", name, err)
	}
	if cert.Usage != models.CertificateUsageIdentity {
		return nil, nil, fmt.Errorf("%q is not a gateway identity (usage: identity)", name)
	}

	cs.mu.RLock()
	mgr := cs.encryptionManager
	cs.mu.RUnlock()
	if mgr == nil {
		return nil, nil, fmt.Errorf("no encryption provider configured; cannot decrypt gateway identity %q", name)
	}

	payload, err := encryption.UnmarshalPayload(cert.PrivateKeyCiphertext)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal encrypted payload for gateway identity %q: %w", name, err)
	}
	plaintext, err := mgr.Decrypt(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt private key for gateway identity %q: %w", name, err)
	}

	return cert.Certificate, plaintext, nil
}

// GetDefaultGatewayIdentity returns the role: default gateway identity, or
// nil when there is none or the store has no database. Should two replicas
// each have stored one, the first by name is returned.
func (cs *CertStore) GetDefaultGatewayIdentity() (*models.StoredCertificate, error) {
	if cs.db == nil {
		return nil, nil
	}
	identities, err := cs.db.ListCertificatesByUsage(models.CertificateUsageIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to list gateway identities: %w", err)
	}
	var found []*models.StoredCertificate
	for _, cert := range identities {
		if cert.IsDefaultIdentity() {
			found = append(found, cert)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	cs.logDuplicateDefaultIdentities(found)
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

// logDuplicateDefaultIdentities warns when more than one gateway identity
// has role: default, once each time that set of identities changes.
func (cs *CertStore) logDuplicateDefaultIdentities(sorted []*models.StoredCertificate) {
	names := make([]string, len(sorted))
	for i, cert := range sorted {
		names[i] = cert.Name
	}
	key := ""
	if len(names) > 1 {
		key = strings.Join(names, "\x00")
	}

	cs.defaultIdentitiesLogMu.Lock()
	defer cs.defaultIdentitiesLogMu.Unlock()
	if cs.loggedDefaultIdentities == key {
		return
	}
	cs.loggedDefaultIdentities = key
	if key != "" {
		cs.logger.Warn("More than one gateway identity has role: default; presenting the first by name",
			slog.String("presented", names[0]), slog.Int("count", len(names)))
	}
}

// GetUpstreamTrustBundle concatenates the PEM certificates of the named
// usage: upstream rows, in the given order. It errors if any name is missing
// rather than build a smaller trust set than configured.
func (cs *CertStore) GetUpstreamTrustBundle(names []string) ([]byte, error) {
	var buf bytes.Buffer
	for _, name := range names {
		if cs.db == nil {
			return nil, fmt.Errorf("certificate %q not found: no certificate database", name)
		}
		cert, err := cs.db.GetCertificateByName(name)
		if err != nil {
			return nil, fmt.Errorf("certificate %q not found: %w", name, err)
		}
		usage := cert.EffectiveUsage()
		if usage != models.CertificateUsageUpstream {
			// Never build a trust bundle out of a client authority or a
			// gateway identity.
			return nil, fmt.Errorf("certificate %q is not usage: upstream", name)
		}
		buf.Write(cert.Certificate)
		if !bytes.HasSuffix(cert.Certificate, []byte("\n")) {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}

// GetCertsDir returns the custom certificates directory path
func (cs *CertStore) GetCertsDir() string {
	return cs.certsDir
}

// bootstrapCertificatesFromFilesystem syncs filesystem certificates to database on startup
// This ensures certificates from the mounted directory are available in the database
// Uses intelligent duplicate detection to avoid re-importing on restarts
func (cs *CertStore) bootstrapCertificatesFromFilesystem() error {
	// Check if directory exists
	if _, err := os.Stat(cs.certsDir); os.IsNotExist(err) {
		cs.logger.Info("Certificates directory does not exist, skipping bootstrap",
			slog.String("path", cs.certsDir))
		return nil
	}

	bootstrapCount := 0
	skippedCount := 0

	err := cs.walkCertificateFiles(func(path string, certData []byte) error {
		// Validate certificate
		count, err := cs.validateCertificateData(filepath.Base(path), certData)
		if err != nil {
			cs.logger.Warn("Invalid certificate file during bootstrap",
				slog.String("file", path),
				slog.Any("error", err))
			return nil
		}

		if count == 0 {
			return nil
		}

		// Check if certificate already exists in database (by name)
		// This prevents duplicate imports on restart
		filename := filepath.Base(path)
		exists, err := cs.certificateExistsByName(filename)
		if err != nil {
			cs.logger.Warn("Failed to check if certificate exists",
				slog.String("filename", filename),
				slog.Any("error", err))
			return nil
		}

		if exists {
			cs.logger.Debug("Certificate already in database, skipping",
				slog.String("filename", filename))
			skippedCount++
			return nil
		}

		// Import certificate to database
		// Parse the first certificate to extract metadata
		block, _ := pem.Decode(certData)
		if block == nil {
			cs.logger.Warn("Failed to decode PEM data during bootstrap",
				slog.String("filename", filename))
			return nil
		}

		x509Cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			cs.logger.Warn("Failed to parse certificate during bootstrap",
				slog.String("filename", filename),
				slog.Any("error", err))
			return nil
		}

		certID, err := generateCertificateID()
		if err != nil {
			cs.logger.Warn("Failed to generate certificate ID during bootstrap",
				slog.String("filename", filename),
				slog.Any("error", err))
			return nil
		}

		cert := &models.StoredCertificate{
			UUID:        certID,
			Name:        filename,
			Certificate: certData,
			Subject:     x509Cert.Subject.String(),
			Issuer:      x509Cert.Issuer.String(),
			NotBefore:   x509Cert.NotBefore,
			NotAfter:    x509Cert.NotAfter,
			CertCount:   count,
			Usage:       models.CertificateUsageUpstream,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := cs.db.SaveCertificate(cert); err != nil {
			cs.logger.Warn("Failed to import certificate to database",
				slog.String("filename", filename),
				slog.Any("error", err))
			return nil
		}

		cs.logger.Info("Bootstrapped certificate from filesystem to database",
			slog.String("filename", filename),
			slog.String("id", cert.UUID),
			slog.Int("cert_count", count))
		bootstrapCount++

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to bootstrap certificates: %w", err)
	}

	if bootstrapCount > 0 || skippedCount > 0 {
		cs.logger.Info("Certificate bootstrap completed",
			slog.Int("imported", bootstrapCount),
			slog.Int("skipped", skippedCount))
	}

	return nil
}

func (cs *CertStore) walkCertificateFiles(visit func(path string, certData []byte) error) error {
	root, err := os.OpenRoot(cs.certsDir)
	if err != nil {
		return fmt.Errorf("failed to open certificates directory: %w", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			cs.logger.Warn("Failed to close certificates root",
				slog.String("path", cs.certsDir),
				slog.Any("error", closeErr))
		}
	}()

	return fs.WalkDir(root.FS(), ".", func(relPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		absPath := filepath.Join(cs.certsDir, relPath)
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".pem" && ext != ".crt" && ext != ".cer" && ext != ".cert" {
			cs.logger.Debug("Skipping non-certificate file",
				slog.String("file", absPath))
			return nil
		}

		certData, err := root.ReadFile(relPath)
		if err != nil {
			cs.logger.Warn("Failed to read certificate file",
				slog.String("file", absPath),
				slog.Any("error", err))
			return nil
		}

		return visit(absPath, certData)
	})
}

// certificateExistsByName checks if a certificate with the given name exists in database
func (cs *CertStore) certificateExistsByName(name string) (bool, error) {
	cert, err := cs.db.GetCertificateByName(name)
	if err != nil {
		return false, err
	}
	return cert != nil, nil
}

// Reload reloads certificates from disk (useful for hot-reloading)
func (cs *CertStore) Reload() error {
	cs.logger.Info("Reloading certificate trust store")
	_, err := cs.LoadCertificates()
	return err
}
