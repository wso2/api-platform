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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// generateCertificateID creates a unique ID for a certificate
func generateCertificateID() string {
	return uuid.New().String()
}

// CertStore manages custom certificates for upstream TLS verification
type CertStore struct {
	logger         *slog.Logger
	certsDir       string
	systemCertPath string
	combinedCerts  []byte
	db             storage.Storage
	mu             sync.RWMutex // Protects combinedCerts from concurrent access
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
	if cs.db != nil && cs.certsDir != "" {
		if err := cs.bootstrapCertificatesFromFilesystem(); err != nil {
			cs.logger.Warn("Failed to bootstrap certificates from filesystem",
				slog.Any("error", err))
		}
	}

	// Load custom certificates from database (primary and only source for custom certs)
	if cs.db != nil {
		dbCerts, count, err := cs.loadDatabaseCertificates()
		if err != nil {
			cs.logger.Warn("Failed to load certificates from database",
				slog.Any("error", err))
		} else if count > 0 {
			certBuffer.Write(dbCerts)
			loadedCount += count
			cs.logger.Info("Loaded custom certificates from database",
				slog.Int("count", count))
		}
	}

	// Load system certificates
	if cs.systemCertPath != "" {
		systemCerts, err := os.ReadFile(cs.systemCertPath)
		if err != nil {
			cs.logger.Warn("Failed to load system certificates",
				slog.String("path", cs.systemCertPath),
				slog.Any("error", err))
			// If we have custom certs, we can continue without system certs
			if loadedCount == 0 {
				return nil, fmt.Errorf("failed to load both custom and system certificates")
			}
		} else {
			// Add system certificates to the buffer
			certBuffer.Write(systemCerts)
			cs.logger.Info("Loaded system certificates",
				slog.String("path", cs.systemCertPath))
		}
	}

	// If no certificates were loaded, return an error
	if certBuffer.Len() == 0 {
		return nil, fmt.Errorf("no certificates loaded from custom or system sources")
	}

	cs.mu.Lock()
	cs.combinedCerts = certBuffer.Bytes()
	cs.mu.Unlock()

	cs.logger.Info("Certificate trust store initialized",
		slog.Int("custom_certs", loadedCount),
		slog.Int("total_bytes", len(certBuffer.Bytes())))

	return certBuffer.Bytes(), nil
}

// loadDatabaseCertificates loads all certificates from the database
func (cs *CertStore) loadDatabaseCertificates() ([]byte, int, error) {
	certs, err := cs.db.ListCertificates()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list certificates: %w", err)
	}

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
				slog.String("id", cert.ID),
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
				slog.String("id", cert.ID),
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

	// Walk through all files in the certificates directory
	err := filepath.Walk(cs.certsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process files with common certificate extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".pem" && ext != ".crt" && ext != ".cer" && ext != ".cert" {
			cs.logger.Debug("Skipping non-certificate file",
				slog.String("file", path))
			return nil
		}

		// Read certificate file
		certData, err := os.ReadFile(path)
		if err != nil {
			cs.logger.Warn("Failed to read certificate file",
				slog.String("file", path),
				slog.Any("error", err))
			return nil // Continue with other files
		}

		// Validate that the file contains valid PEM-encoded certificates
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
		cs.logger.Debug("Certificates directory does not exist, skipping bootstrap",
			slog.String("path", cs.certsDir))
		return nil
	}

	bootstrapCount := 0
	skippedCount := 0

	// Walk through all files in the certificates directory
	err := filepath.Walk(cs.certsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process files with common certificate extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".pem" && ext != ".crt" && ext != ".cer" && ext != ".cert" {
			return nil
		}

		// Read certificate file
		certData, err := os.ReadFile(path)
		if err != nil {
			cs.logger.Warn("Failed to read certificate file during bootstrap",
				slog.String("file", path),
				slog.Any("error", err))
			return nil // Continue with other files
		}

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

		cert := &models.StoredCertificate{
			ID:          generateCertificateID(),
			Name:        filename,
			Certificate: certData,
			Subject:     x509Cert.Subject.String(),
			Issuer:      x509Cert.Issuer.String(),
			NotBefore:   x509Cert.NotBefore,
			NotAfter:    x509Cert.NotAfter,
			CertCount:   count,
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
			slog.String("id", cert.ID),
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
