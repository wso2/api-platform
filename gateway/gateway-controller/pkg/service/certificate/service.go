/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package certificate holds the business rules for custom upstream TLS
// certificates, so every entry point — the management REST API, the MCP
// endpoint, or any future caller — validates and applies a certificate the same
// way.
package certificate

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CertStore is the slice of certstore.CertStore this service uses.
// *certstore.CertStore satisfies it structurally, so the service imports
// neither pkg/certstore nor pkg/xds.
type CertStore interface {
	Reload() error
	GetCombinedCertificates() []byte
}

// SnapshotRefresher is the slice of xds.SnapshotManager this service uses.
// *xds.SnapshotManager satisfies it structurally.
type SnapshotRefresher interface {
	UpdateSnapshot(ctx context.Context, correlationID string) error
}

// XDSTargets bundles both collaborators so a caller resolves them together, in
// one nil check, at call time.
type XDSTargets struct {
	Store    CertStore
	Snapshot SnapshotRefresher
}

// XDSResolver returns the live xDS targets, or nil when this gateway has no
// custom cert store (router.upstream.tls.customCertsPath unset) or no xDS layer
// at all, as in unit tests.
//
// It is a function rather than two injected fields because it must be evaluated
// per operation: the concrete snapshot manager is not nil-receiver safe, so the
// nil check has to happen in the package that can see its concrete type.
type XDSResolver func() *XDSTargets

// UploadResult holds the result of an Upload operation.
type UploadResult struct {
	Certificate *models.StoredCertificate
}

// ListResult holds the result of a List operation.
type ListResult struct {
	Certificates []*models.StoredCertificate
	TotalBytes   int
}

// GetResult holds the result of a Get operation.
type GetResult struct {
	Certificate *models.StoredCertificate
}

// DeleteResult holds the result of a Delete operation.
type DeleteResult struct {
	ID string
}

// ReloadResult holds the result of a Reload operation.
type ReloadResult struct {
	TotalBytes int
}

// CertificateService encapsulates the business rules for custom upstream TLS
// certificates: PEM validation, metadata extraction, persistence, and
// propagation of the resulting trust store to the router over SDS.
type CertificateService struct {
	db         storage.Storage
	resolveXDS XDSResolver
	logger     *slog.Logger
}

// NewCertificateService creates a new CertificateService.
//
// resolveXDS is optional: when nil, or when it returns nil, every operation that
// must reach the router fails with ErrCertStoreNotConfigured instead of
// panicking. Unlike the other services this one takes no EventHub or gateway
// ID — certificate changes publish no replica-sync event today.
func NewCertificateService(db storage.Storage, resolveXDS XDSResolver, logger *slog.Logger) *CertificateService {
	if db == nil {
		panic("CertificateService requires non-nil storage")
	}

	return &CertificateService{
		db:         db,
		resolveXDS: resolveXDS,
		logger:     logger,
	}
}

// UploadParams holds parameters for the Upload operation.
type UploadParams struct {
	Name           string
	CertificatePEM []byte
	CorrelationID  string
	Logger         *slog.Logger
}

// Upload validates a PEM chain, stores it, and republishes the trust store.
//
// On a sync failure the certificate stays in the database and a *SyncError with
// Persisted set is returned: the reload is idempotent and recoverable through
// Reload, whereas rolling back a committed write is not.
func (s *CertificateService) Upload(params UploadParams) (*UploadResult, error) {
	log := s.loggerOr(params.Logger)

	name := strings.TrimSpace(params.Name)
	if name == "" || len(params.CertificatePEM) == 0 {
		return nil, ErrMissingFields
	}

	count, err := ValidateChain(params.CertificatePEM)
	if err != nil {
		return nil, &InvalidCertificateError{Cause: err}
	}

	subject, issuer, notBefore, notAfter, err := ExtractMetadata(params.CertificatePEM)
	if err != nil {
		return nil, &MetadataError{Cause: err}
	}

	certID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIDGeneration, err)
	}

	now := time.Now()
	cert := &models.StoredCertificate{
		UUID:        certID,
		Name:        name,
		Certificate: params.CertificatePEM,
		Subject:     subject,
		Issuer:      issuer,
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		CertCount:   count,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.db.SaveCertificate(cert); err != nil {
		return nil, &PersistError{Op: OpSave, Cause: err}
	}

	log.Info("Certificate saved to database successfully",
		slog.String("id", certID),
		slog.String("name", name),
		slog.Int("cert_count", count))

	if _, err := s.syncCertStore(true, params.CorrelationID, log); err != nil {
		return nil, err
	}

	log.Info("SDS snapshot updated with new certificate",
		slog.String("id", certID),
		slog.String("name", name))

	return &UploadResult{Certificate: cert}, nil
}

// List returns every stored certificate together with the total size of the
// stored PEM data.
func (s *CertificateService) List() (*ListResult, error) {
	certs, err := s.db.ListCertificates()
	if err != nil {
		return nil, &PersistError{Op: OpList, Cause: err}
	}

	totalBytes := 0
	for _, cert := range certs {
		totalBytes += len(cert.Certificate)
	}

	return &ListResult{Certificates: certs, TotalBytes: totalBytes}, nil
}

// Get retrieves a certificate by ID.
func (s *CertificateService) Get(id string) (*GetResult, error) {
	cert, err := s.db.GetCertificate(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, &PersistError{Op: OpLoad, Cause: err}
	}
	if cert == nil {
		return nil, ErrNotFound
	}

	return &GetResult{Certificate: cert}, nil
}

// GetByName retrieves a certificate by its unique name.
func (s *CertificateService) GetByName(name string) (*GetResult, error) {
	cert, err := s.db.GetCertificateByName(name)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, &PersistError{Op: OpLoad, Cause: err}
	}
	if cert == nil {
		return nil, ErrNotFound
	}

	return &GetResult{Certificate: cert}, nil
}

// DeleteParams holds parameters for the Delete operation.
type DeleteParams struct {
	ID            string
	CorrelationID string
	Logger        *slog.Logger
}

// Delete removes a certificate and republishes the trust store.
//
// The cert store is checked before the row is deleted, not after: on a gateway
// with no custom cert store the operation must fail without having already
// removed the certificate.
func (s *CertificateService) Delete(params DeleteParams) (*DeleteResult, error) {
	log := s.loggerOr(params.Logger)

	if strings.TrimSpace(params.ID) == "" {
		return nil, ErrMissingFields
	}
	if _, err := s.xdsTargets(); err != nil {
		return nil, err
	}

	if err := s.db.DeleteCertificate(params.ID); err != nil {
		return nil, &PersistError{Op: OpDelete, Cause: err}
	}

	log.Info("Certificate deleted from database", slog.String("id", params.ID))

	if _, err := s.syncCertStore(true, params.CorrelationID, log); err != nil {
		return nil, err
	}

	log.Info("SDS snapshot updated after certificate deletion", slog.String("id", params.ID))

	return &DeleteResult{ID: params.ID}, nil
}

// ReloadParams holds parameters for the Reload operation.
type ReloadParams struct {
	CorrelationID string
	Logger        *slog.Logger
}

// Reload re-reads the trust store from the database and republishes it, without
// changing any stored certificate.
func (s *CertificateService) Reload(params ReloadParams) (*ReloadResult, error) {
	log := s.loggerOr(params.Logger)

	totalBytes, err := s.syncCertStore(false, params.CorrelationID, log)
	if err != nil {
		return nil, err
	}

	log.Info("Certificates reloaded and SDS snapshot updated")

	return &ReloadResult{TotalBytes: totalBytes}, nil
}

// syncCertStore reloads the router's cert store from the database and pushes a
// fresh SDS snapshot. persisted records whether a database write has already
// committed, so a SyncError tells the caller whether the store and the router
// have diverged. It returns the size of the combined certificate bundle.
func (s *CertificateService) syncCertStore(persisted bool, correlationID string, log *slog.Logger) (int, error) {
	targets, err := s.xdsTargets()
	if err != nil {
		return 0, err
	}

	if err := targets.Store.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		return 0, &SyncError{Stage: StageReload, Persisted: persisted, Cause: err}
	}

	// context.Background rather than a request context: an SDS push must not be
	// cancellable by a client disconnecting after the database write committed.
	if err := targets.Snapshot.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		return 0, &SyncError{Stage: StageSnapshot, Persisted: persisted, Cause: err}
	}

	return len(targets.Store.GetCombinedCertificates()), nil
}

// xdsTargets resolves the router-side collaborators, reporting
// ErrCertStoreNotConfigured when this gateway has none.
//
// These checks cover an absent resolver, a resolver that reports no xDS layer,
// and a half-populated result. They cannot catch a typed-nil pointer stored in
// the interface — `Store == nil` is false for that — so the contract is that a
// resolver returns a nil *XDSTargets rather than a struct holding nil pointers.
func (s *CertificateService) xdsTargets() (*XDSTargets, error) {
	if s.resolveXDS == nil {
		return nil, ErrCertStoreNotConfigured
	}
	targets := s.resolveXDS()
	if targets == nil || targets.Store == nil || targets.Snapshot == nil {
		return nil, ErrCertStoreNotConfigured
	}

	return targets, nil
}

// loggerOr prefers the per-call logger, falls back to the service logger, and
// finally to the default — a nil logger must never panic a write path that has
// already committed to the database.
func (s *CertificateService) loggerOr(log *slog.Logger) *slog.Logger {
	if log != nil {
		return log
	}
	if s.logger != nil {
		return s.logger
	}

	return slog.Default()
}

// ValidateChain reports how many CERTIFICATE blocks the PEM data contains,
// returning an error if any block fails to parse or none are present.
// Non-CERTIFICATE PEM blocks are skipped rather than rejected.
func ValidateChain(data []byte) (int, error) {
	count := 0
	rest := data

	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		_, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return 0, fmt.Errorf("invalid certificate: %w", err)
		}

		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid certificates found in PEM data")
	}

	return count, nil
}

// ExtractMetadata returns the subject, issuer and validity window of the first
// certificate in the chain. The leaf is taken as representative of the bundle,
// which is what the stored certificate row records.
func ExtractMetadata(data []byte) (subject, issuer string, notBefore, notAfter time.Time, err error) {
	rest := data

	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			err = parseErr
			return
		}

		// Use first certificate for metadata
		subject = cert.Subject.String()
		issuer = cert.Issuer.String()
		notBefore = cert.NotBefore
		notAfter = cert.NotAfter
		return
	}

	err = fmt.Errorf("no valid certificate found")
	return
}
