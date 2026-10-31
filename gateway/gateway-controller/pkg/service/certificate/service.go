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

// Package certificate holds the business rules for the certificates this
// Gateway stores — upstream trust (usage: upstream), client certificate
// authorities for mTLS (usage: downstream) and gateway identities
// (usage: identity) — so every entry point (the management REST API, the MCP
// endpoint) validates, stores and propagates a certificate the same way.
package certificate

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certmetrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/gatewayidentity"
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

// XDSResolver returns the live xDS targets, or nil when there is no xDS layer,
// as in hand-constructed servers in tests. The controller always builds the
// cert store, so in production it never returns nil.
//
// It is a function rather than two injected fields because it must be evaluated
// per operation: the concrete snapshot manager is not nil-receiver safe, so the
// nil check has to happen in the package that can see its concrete type.
type XDSResolver func() *XDSTargets

// Dependencies are the collaborators the service reaches. Everything a caller
// may replace after building the service is a function evaluated at the moment
// of use, never a value captured at construction: the encryption manager is set
// after NewAPIServer, and tests swap the snapshot manager and the client
// authority publisher on an already-built server.
type Dependencies struct {
	DB         storage.Storage
	Store      func() *storage.ConfigStore
	ResolveXDS XDSResolver

	EncryptionManager func() *encryption.ProviderManager

	// PublishClientAuthorities republishes the usage: downstream pool to the
	// policy engine. Optional; nil skips the step.
	PublishClientAuthorities func(correlationID string) error

	// PublishEvent tells the other replicas that the certificate rows changed.
	// Optional; nil skips the step.
	PublishEvent func(action, certID, correlationID string, log *slog.Logger)

	// DefaultIdentityMu serialises the role: default check with the save. Pass
	// the server's mutex so every service instance shares one; nil uses the
	// service's own.
	DefaultIdentityMu *sync.Mutex

	Logger *slog.Logger
}

// CertificateService encapsulates the business rules for certificates:
// validation, metadata extraction, identity key encryption, referential
// integrity on delete, persistence, and propagation to the router, the policy
// engine and the other replicas.
type CertificateService struct {
	deps  Dependencies
	ownMu sync.Mutex
}

// NewCertificateService creates a new CertificateService.
//
// ResolveXDS is optional: when nil, or when it returns nil, every operation that
// must reach the router fails with ErrCertStoreNotConfigured instead of
// panicking.
func NewCertificateService(deps Dependencies) *CertificateService {
	if deps.DB == nil {
		panic("CertificateService requires non-nil storage")
	}

	return &CertificateService{deps: deps}
}

// UploadParams holds parameters for the Upload operation.
type UploadParams struct {
	Name        string
	Certificate string
	Usage       string
	Role        string
	Match       *models.CertificateMatch
	PrivateKey  string

	// FieldErrors are request-shape problems the caller found while decoding
	// (an unknown field, a malformed match). They are reported together with
	// the validation errors, after them.
	FieldErrors []FieldError

	CorrelationID string
	Logger        *slog.Logger
}

// UploadResult holds the result of an Upload operation.
type UploadResult struct {
	Certificate  *models.StoredCertificate
	Usage        string
	Role         string
	IsLeaf       bool
	KeyAlgorithm string
	Warnings     []clientca.Warning
}

// Upload validates, stores and propagates a new certificate.
//
// On a sync failure the certificate stays in the database and a *SyncError with
// Persisted set is returned: the reload is idempotent and recoverable through
// Reload, whereas rolling back a committed write is not.
func (s *CertificateService) Upload(params UploadParams) (*UploadResult, error) {
	log := s.loggerOr(params.Logger)

	validation, effectiveUsage, effectiveRole, bundle := s.validateUpload(params)
	validation.fieldErrors = append(validation.fieldErrors, params.FieldErrors...)
	if validation.hasProblems() {
		return nil, &ValidationError{Message: MsgUploadInvalid, Fields: validation.fieldErrors}
	}

	var (
		subject, issuer      string
		notBefore            time.Time
		notAfter             time.Time
		count                int
		isLeaf               bool
		warnings             []clientca.Warning
		keyAlgorithm         string
		privateKeyCiphertext string
	)

	certData := []byte(params.Certificate)

	switch effectiveUsage {
	case models.CertificateUsageDownstream:
		subject = bundle.Identity.Subject.String()
		issuer = bundle.Identity.Issuer.String()
		notBefore = bundle.Identity.NotBefore
		notAfter = bundle.Identity.NotAfter
		count = len(bundle.Certificates)
		isLeaf = bundle.IsLeaf
		warnings = bundle.Warnings
	case models.CertificateUsageIdentity:
		ib := validation.identityBundle
		subject = ib.Leaf.Subject.String()
		issuer = ib.Leaf.Issuer.String()
		notBefore = ib.Leaf.NotBefore
		notAfter = ib.Leaf.NotAfter
		count = len(ib.Chain)
		isLeaf = !ib.Leaf.IsCA
		keyAlgorithm = ib.KeyAlgorithm
		for _, warn := range ib.Warnings {
			warnings = append(warnings, clientca.Warning{Code: warn.Code, Field: warn.Field, Message: warn.Message})
		}

		if s.encryptionManager() == nil {
			log.Error("Cannot store gateway identity: no encryption provider configured")
			return nil, ErrIdentityStorageUnavailable
		}
		ciphertext, err := s.encryptPrivateKey(params.PrivateKey)
		if err != nil {
			log.Error("Failed to encrypt gateway identity private key", slog.Any("error", err))
			return nil, &EncryptionError{Cause: err}
		}
		privateKeyCiphertext = ciphertext
	default:
		var err error
		count, err = ValidateChain(certData)
		if err == nil {
			subject, issuer, notBefore, notAfter, err = ExtractMetadata(certData)
		}
		if err != nil {
			// Unreachable: validateUpload already accepted these bytes.
			log.Error("Failed to read a validated certificate", slog.Any("error", err))
			return nil, &PersistError{Op: OpSave, Cause: err}
		}
		if firstCert, err := firstX509Certificate(certData); err == nil {
			isLeaf = !firstCert.IsCA
		} else {
			log.Warn("Failed to determine certificate authority status", slog.Any("error", err))
		}
	}

	// A client authority or identity already inside its expiry horizon gets
	// the same CERT_EXPIRES_SOON warning the listing reports.
	if effectiveUsage == models.CertificateUsageDownstream || effectiveUsage == models.CertificateUsageIdentity {
		if warning := clientca.ExpiryWarning(notAfter, time.Now()); warning != nil {
			warnings = append(warnings, *warning)
		}
	}

	for _, warning := range warnings {
		log.Warn("Client certificate authority warning", slog.String("code", warning.Code), slog.String("name", params.Name))
	}

	// At most one gateway identity has role: default. The lock covers this
	// replica only; should two replicas each store one, every replica
	// presents the first by name. The role conflict is checked before the
	// name conflict, so it is the one reported when both apply.
	if effectiveRole == models.CertificateRoleDefault {
		mu := s.defaultIdentityMu()
		mu.Lock()
		defer mu.Unlock()
		existingDefault, err := s.defaultGatewayIdentity()
		if err != nil {
			log.Error("Failed to look up the default gateway identity", slog.Any("error", err))
			return nil, &PersistError{Op: OpSave, Cause: err}
		}
		if existingDefault != nil {
			return nil, &DefaultIdentityExistsError{Name: existingDefault.Name}
		}
	}

	// Generate unique ID (UUID v7)
	certID, err := utils.GenerateUUID()
	if err != nil {
		log.Error("Failed to generate certificate ID", slog.Any("error", err))
		return nil, fmt.Errorf("%w: %v", ErrIDGeneration, err)
	}

	cert := &models.StoredCertificate{
		UUID:                 certID,
		Name:                 params.Name,
		Certificate:          certData,
		Subject:              subject,
		Issuer:               issuer,
		NotBefore:            notBefore,
		NotAfter:             notAfter,
		CertCount:            count,
		Usage:                effectiveUsage,
		Role:                 effectiveRole,
		Match:                params.Match,
		PrivateKeyCiphertext: privateKeyCiphertext,
		KeyAlgorithm:         keyAlgorithm,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if err := s.deps.DB.SaveCertificate(cert); err != nil {
		if !storage.IsConflictError(err) {
			log.Error("Failed to save certificate to database",
				slog.String("name", params.Name),
				slog.Any("error", err))
		}
		return nil, &PersistError{Op: OpSave, Cause: err}
	}

	log.Info("Certificate saved to database successfully",
		slog.String("id", certID),
		slog.String("name", params.Name),
		slog.Int("cert_count", count))
	s.publishEvent("CREATE", certID, params.CorrelationID, log)

	targets, err := s.xdsTargets()
	if err != nil {
		log.Error("Certificate store not available")
		return nil, err
	}

	if err := targets.Store.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		return nil, &SyncError{Stage: StageReload, Persisted: true, Cause: err}
	}

	// context.Background rather than a request context: an SDS push must not be
	// cancellable by a client disconnecting after the database write committed.
	if err := targets.Snapshot.UpdateSnapshot(context.Background(), params.CorrelationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		return nil, &SyncError{Stage: StageSnapshot, Persisted: true, Cause: err}
	}

	log.Info("SDS snapshot updated with new certificate",
		slog.String("id", certID),
		slog.String("name", params.Name))

	s.refreshMetrics(log, "upload")

	if effectiveUsage == models.CertificateUsageDownstream {
		if err := s.publishClientAuthorities(params.CorrelationID); err != nil {
			log.Error("Failed to publish client certificate authorities", slog.Any("error", err))
			return nil, &SyncError{Stage: StagePolicyEngine, Persisted: true, Cause: err}
		}
	}

	return &UploadResult{
		Certificate:  cert,
		Usage:        effectiveUsage,
		Role:         effectiveRole,
		IsLeaf:       isLeaf,
		KeyAlgorithm: keyAlgorithm,
		Warnings:     warnings,
	}, nil
}

// ListParams holds parameters for the List operation.
type ListParams struct {
	Usage  string // optional filter: upstream, downstream or identity
	Logger *slog.Logger
}

// ListedCertificate is one stored certificate plus what a listing reports
// about it.
type ListedCertificate struct {
	Certificate      *models.StoredCertificate
	Usage            string
	Role             string
	Match            *models.CertificateMatch
	IsLeaf           bool
	KeyAlgorithm     string
	ChainLength      int
	Warnings         []clientca.Warning
	ReferencedByApis *int
}

// ListResult holds the result of a List operation.
type ListResult struct {
	Items      []ListedCertificate
	TotalBytes int
}

// List returns the stored certificates, optionally only those of one usage,
// together with the total size of their stored PEM data.
func (s *CertificateService) List(params ListParams) (*ListResult, error) {
	log := s.loggerOr(params.Logger)

	usageFilter := params.Usage
	if usageFilter != "" && usageFilter != models.CertificateUsageUpstream &&
		usageFilter != models.CertificateUsageDownstream && usageFilter != models.CertificateUsageIdentity {
		return nil, &ValidationError{
			Message: MsgInvalidUsageFilter,
			Fields:  []FieldError{{Field: "usage", Message: "usage must be upstream, downstream or identity"}},
		}
	}

	var certs []*models.StoredCertificate
	var err error
	if usageFilter != "" {
		certs, err = s.deps.DB.ListCertificatesByUsage(usageFilter)
	} else {
		certs, err = s.deps.DB.ListCertificates()
	}
	if err != nil {
		log.Error("Failed to list certificates from database", slog.Any("error", err))
		return nil, &PersistError{Op: OpList, Cause: err}
	}

	items := []ListedCertificate{}
	totalBytes := 0
	now := time.Now()

	for _, cert := range certs {
		totalBytes += len(cert.Certificate)

		usage := cert.EffectiveUsage()
		item := ListedCertificate{Certificate: cert, Usage: usage}

		if usage == models.CertificateUsageDownstream {
			role := cert.EffectiveRole()
			item.Role = role
			if role == models.CertificateRoleRelay {
				item.Match = cert.Match
			}

			if referencedByApis, err := s.countClientCertificateReferences(cert.Name); err != nil {
				log.Warn("Failed to compute referencedByApis for client-CA authority",
					slog.String("name", cert.Name), slog.Any("error", err))
				// Leave referencedByApis absent rather than report a count
				// that may be wrong.
			} else {
				item.ReferencedByApis = &referencedByApis
			}

			if identity, err := clientca.IdentityCertificate(cert.Certificate); err == nil {
				item.IsLeaf = !identity.IsCA
			} else {
				log.Warn("Failed to parse stored client certificate authority",
					slog.String("name", cert.Name), slog.Any("error", err))
			}

			if warning := clientca.ExpiryWarning(cert.NotAfter, now); warning != nil {
				// Not logged here: the periodic expiry sweep logs it.
				item.Warnings = []clientca.Warning{*warning}
			}
		} else if usage == models.CertificateUsageIdentity {
			if cert.IsDefaultIdentity() {
				item.Role = models.CertificateRoleDefault
			}
			item.KeyAlgorithm = cert.KeyAlgorithm

			if chain, err := gatewayidentity.ParseChain(cert.Certificate); err == nil {
				item.ChainLength = len(chain)
				item.IsLeaf = !chain[0].IsCA
			} else {
				log.Warn("Failed to parse stored gateway identity certificate chain",
					slog.String("name", cert.Name), slog.Any("error", err))
			}

			if referencedByApis, err := s.countGatewayIdentityReferences(cert.Name); err != nil {
				log.Warn("Failed to compute referencedByApis for gateway identity",
					slog.String("name", cert.Name), slog.Any("error", err))
			} else {
				item.ReferencedByApis = &referencedByApis
			}

			if warning := clientca.ExpiryWarning(cert.NotAfter, now); warning != nil {
				item.Warnings = []clientca.Warning{*warning}
			}
		} else {
			if firstCert, err := firstX509Certificate(cert.Certificate); err == nil {
				item.IsLeaf = !firstCert.IsCA
				if warning := clientca.ExpiryWarning(firstCert.NotAfter, now); warning != nil {
					item.Warnings = []clientca.Warning{*warning}
				}
			} else {
				log.Warn("Failed to parse stored certificate",
					slog.String("name", cert.Name), slog.Any("error", err))
			}
		}

		items = append(items, item)
	}

	return &ListResult{Items: items, TotalBytes: totalBytes}, nil
}

// GetResult holds the result of a Get operation.
type GetResult struct {
	Certificate *models.StoredCertificate
}

// Get retrieves a certificate by ID.
func (s *CertificateService) Get(id string) (*GetResult, error) {
	cert, err := s.deps.DB.GetCertificate(id)
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
	cert, err := s.deps.DB.GetCertificateByName(name)
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

// UpdateParams holds parameters for the Update operation. Only a usage:
// identity certificate can be updated; its name, usage, role and match never
// change.
type UpdateParams struct {
	ID          string
	Certificate string
	PrivateKey  string

	// Existing is the certificate PrepareUpdate returned, when the caller
	// already ran it, so Update does not read the row a second time. Nil
	// makes Update run PrepareUpdate itself.
	Existing *models.StoredCertificate

	// FieldErrors are request-shape problems the caller found while decoding.
	FieldErrors []FieldError

	CorrelationID string
	Logger        *slog.Logger
}

// UpdateResult holds the result of an Update operation.
type UpdateResult struct {
	Certificate  *models.StoredCertificate
	IsLeaf       bool
	KeyAlgorithm string
	Warnings     []clientca.Warning
	IsDefault    bool
}

// PrepareUpdate checks that id names a certificate that can be updated and
// returns it. The REST handler calls it before reading the request body, as it
// always has; Update calls it again itself, so every caller gets the check.
func (s *CertificateService) PrepareUpdate(id string, log *slog.Logger) (*models.StoredCertificate, error) {
	log = s.loggerOr(log)

	existing, err := s.deps.DB.GetCertificate(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrNotFound
		}
		log.Error("Failed to read certificate before update", slog.String("id", id), slog.Any("error", err))
		return nil, &PersistError{Op: OpLoad, Cause: err}
	}

	if existing.Usage != models.CertificateUsageIdentity {
		return nil, &ValidationError{
			Message: MsgUploadInvalid,
			Fields: []FieldError{{
				Field:   "usage",
				Message: "only usage: identity certificates can be updated; delete and re-upload other certificates",
			}},
		}
	}

	return existing, nil
}

// Update rotates a usage: identity certificate's chain and private key in
// place, keeping its name.
func (s *CertificateService) Update(params UpdateParams) (*UpdateResult, error) {
	log := s.loggerOr(params.Logger)

	existing := params.Existing
	if existing == nil {
		var err error
		existing, err = s.PrepareUpdate(params.ID, log)
		if err != nil {
			return nil, err
		}
	}

	// Name, usage, role and match are immutable on rotation, so only the
	// certificate and key from the request are validated.
	validation, _, _, _ := s.validateUpload(UploadParams{
		Name:        existing.Name,
		Certificate: params.Certificate,
		Usage:       models.CertificateUsageIdentity,
		PrivateKey:  params.PrivateKey,
	})
	validation.fieldErrors = append(validation.fieldErrors, params.FieldErrors...)
	if validation.hasProblems() {
		return nil, &ValidationError{Message: MsgUploadInvalid, Fields: validation.fieldErrors}
	}

	if s.encryptionManager() == nil {
		log.Error("Cannot store gateway identity: no encryption provider configured")
		return nil, ErrIdentityStorageUnavailable
	}
	ciphertext, err := s.encryptPrivateKey(params.PrivateKey)
	if err != nil {
		log.Error("Failed to encrypt gateway identity private key", slog.Any("error", err))
		return nil, &EncryptionError{Cause: err}
	}

	ib := validation.identityBundle
	updated := &models.StoredCertificate{
		UUID:                 existing.UUID,
		Name:                 existing.Name,
		Certificate:          []byte(params.Certificate),
		Subject:              ib.Leaf.Subject.String(),
		Issuer:               ib.Leaf.Issuer.String(),
		NotBefore:            ib.Leaf.NotBefore,
		NotAfter:             ib.Leaf.NotAfter,
		CertCount:            len(ib.Chain),
		Usage:                models.CertificateUsageIdentity,
		Role:                 existing.Role,
		PrivateKeyCiphertext: ciphertext,
		KeyAlgorithm:         ib.KeyAlgorithm,
		UpdatedAt:            time.Now(),
	}

	if err := s.deps.DB.UpdateCertificate(updated); err != nil {
		log.Error("Failed to update certificate", slog.String("id", params.ID), slog.Any("error", err))
		return nil, &PersistError{Op: OpUpdate, Cause: err}
	}
	s.publishEvent("UPDATE", updated.UUID, params.CorrelationID, log)

	// The SDS update rebuilds this identity's gateway_identity:<name> secret.
	// Reload does nothing for identity rows but keeps parity with upload.
	targets, err := s.xdsTargets()
	if err != nil {
		log.Error("Failed to update SDS snapshot after certificate update", slog.Any("error", err))
		return nil, &SyncError{Stage: StageSnapshot, Persisted: true, Cause: err}
	}
	if err := targets.Store.Reload(); err != nil {
		log.Warn("Failed to reload certificate store after identity rotation", slog.Any("error", err))
	}
	if err := targets.Snapshot.UpdateSnapshot(context.Background(), params.CorrelationID); err != nil {
		log.Error("Failed to update SDS snapshot after certificate update", slog.Any("error", err))
		return nil, &SyncError{Stage: StageSnapshot, Persisted: true, Cause: err}
	}

	s.refreshMetrics(log, "update")

	var warnings []clientca.Warning
	for _, warn := range ib.Warnings {
		warnings = append(warnings, clientca.Warning{Code: warn.Code, Field: warn.Field, Message: warn.Message})
	}
	if warning := clientca.ExpiryWarning(updated.NotAfter, time.Now()); warning != nil {
		warnings = append(warnings, *warning)
	}

	return &UpdateResult{
		Certificate:  updated,
		IsLeaf:       !ib.Leaf.IsCA,
		KeyAlgorithm: updated.KeyAlgorithm,
		Warnings:     warnings,
		IsDefault:    existing.IsDefaultIdentity(),
	}, nil
}

// DeleteParams holds parameters for the Delete operation.
type DeleteParams struct {
	ID            string
	CorrelationID string
	Logger        *slog.Logger
}

// DeleteResult holds the result of a Delete operation.
type DeleteResult struct {
	ID string
}

// Delete removes a certificate and republishes the trust store, refusing while
// a deployed API depends on it.
//
// The cert store is checked before the row is read or deleted, not after:
// without one the operation must fail without having already removed the
// certificate.
func (s *CertificateService) Delete(params DeleteParams) (*DeleteResult, error) {
	log := s.loggerOr(params.Logger)

	if params.ID == "" {
		return nil, ErrMissingID
	}

	targets, err := s.xdsTargets()
	if err != nil {
		log.Error("Certificate store not available")
		return nil, err
	}

	// The row's usage and role decide which referential checks apply, so a
	// read failure here must stop the delete: skipping the checks would let a
	// referenced authority or identity disappear on a transient store error.
	preDeleteCert, err := s.deps.DB.GetCertificate(params.ID)
	if err != nil && !storage.IsNotFoundError(err) {
		log.Error("Failed to read certificate before delete", slog.Any("error", err))
		return nil, &PersistError{Op: OpLoad, Cause: err}
	}

	// A non-relay client authority cannot be removed while a deployed API
	// depends on it. Removing the last relay entry just turns header mode off.
	if preDeleteCert != nil {
		role := preDeleteCert.EffectiveRole()
		usage := preDeleteCert.EffectiveUsage()
		if usage == models.CertificateUsageDownstream && role != models.CertificateRoleRelay {
			if err := s.CheckClientAuthorityDeletable(preDeleteCert); err != nil {
				return nil, err
			}
		}
		if usage == models.CertificateUsageUpstream {
			if err := s.checkUpstreamCertificateDeletable(preDeleteCert); err != nil {
				return nil, err
			}
		}
		if usage == models.CertificateUsageIdentity {
			if err := s.checkGatewayIdentityDeletable(preDeleteCert); err != nil {
				return nil, err
			}
		}
	}

	if err := s.deps.DB.DeleteCertificate(params.ID); err != nil {
		log.Error("Failed to delete certificate",
			slog.String("id", params.ID),
			slog.Any("error", err))
		return nil, &PersistError{Op: OpDelete, Cause: err}
	}

	log.Info("Certificate deleted from database", slog.String("id", params.ID))
	s.publishEvent("DELETE", params.ID, params.CorrelationID, log)

	if err := targets.Store.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		return nil, &SyncError{Stage: StageReload, Persisted: true, Cause: err}
	}

	if err := targets.Snapshot.UpdateSnapshot(context.Background(), params.CorrelationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		return nil, &SyncError{Stage: StageSnapshot, Persisted: true, Cause: err}
	}

	log.Info("SDS snapshot updated after certificate deletion", slog.String("id", params.ID))

	s.refreshMetrics(log, "delete")

	// The stored Usage as written, not EffectiveUsage: only a row stored as
	// downstream was ever in the client authority pool.
	if preDeleteCert != nil && preDeleteCert.Usage == models.CertificateUsageDownstream {
		if err := s.publishClientAuthorities(params.CorrelationID); err != nil {
			log.Error("Failed to publish client certificate authorities", slog.Any("error", err))
			return nil, &SyncError{Stage: StagePolicyEngine, Persisted: true, Cause: err}
		}
	}

	return &DeleteResult{ID: params.ID}, nil
}

// ReloadParams holds parameters for the Reload operation.
type ReloadParams struct {
	CorrelationID string
	Logger        *slog.Logger
}

// ReloadResult holds the result of a Reload operation.
type ReloadResult struct {
	TotalBytes int
}

// Reload re-reads every stored certificate and republishes it, without
// changing any stored certificate.
func (s *CertificateService) Reload(params ReloadParams) (*ReloadResult, error) {
	log := s.loggerOr(params.Logger)

	targets, err := s.xdsTargets()
	if err != nil {
		log.Error("Certificate store not available")
		return nil, err
	}

	if err := targets.Store.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		return nil, &SyncError{Stage: StageReload, Persisted: false, Cause: err}
	}

	if err := targets.Snapshot.UpdateSnapshot(context.Background(), params.CorrelationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		return nil, &SyncError{Stage: StageSnapshot, Persisted: false, Cause: err}
	}

	log.Info("Certificates reloaded and SDS snapshot updated")
	// A reload names no single row; the events table admits only CREATE,
	// UPDATE and DELETE, and every replica rebuilds from the database for
	// any certificate action.
	s.publishEvent("UPDATE", "", params.CorrelationID, log)

	if err := s.publishClientAuthorities(params.CorrelationID); err != nil {
		log.Error("Failed to publish client certificate authorities", slog.Any("error", err))
		return nil, &SyncError{Stage: StagePolicyEngine, Persisted: false, Cause: err}
	}

	s.refreshMetrics(log, "reload")

	return &ReloadResult{TotalBytes: len(targets.Store.GetCombinedCertificates())}, nil
}

// CheckClientAuthorityDeletable reports whether cert, a non-relay usage:
// downstream authority, can be removed. It refuses with an *InUseError when a
// deployed API names the authority, or when it is the last non-relay authority
// and any deployed API attaches mtls-auth, and with a *ReferenceCheckError when
// the deployed configurations could not be read.
func (s *CertificateService) CheckClientAuthorityDeletable(cert *models.StoredCertificate) error {
	restAPIs, err := s.deployedConfigs(models.KindRestApi)
	if err != nil {
		s.loggerOr(nil).Error("Failed to read deployed RestApi configurations for client-CA referential-integrity check",
			slog.String("certificate", cert.Name), slog.Any("error", err))
		return &ReferenceCheckError{Message: "Failed to verify certificate references", Cause: err}
	}

	var namedRefs []Reference
	for _, cfg := range restAPIs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		paths := config.NamedAcceptEntryFieldPaths(&restCfg, cert.Name)
		if len(paths) == 0 {
			continue
		}
		namedRefs = append(namedRefs, Reference{Field: paths[0], API: cfg.Handle})
	}
	if len(namedRefs) > 0 {
		return &InUseError{
			Message: fmt.Sprintf("client-CA authority '%s' is named by %s; remove those references first",
				cert.Name, pluralDeployedAPIs(len(namedRefs))),
			References: namedRefs,
		}
	}

	remainingNonRelay := 0
	if clientCerts, err := s.deps.DB.ListCertificatesByUsage(models.CertificateUsageDownstream); err == nil {
		for _, c := range clientCerts {
			if c.UUID == cert.UUID {
				continue // the one about to be deleted
			}
			if c.EffectiveRole() != models.CertificateRoleRelay {
				remainingNonRelay++
			}
		}
	}
	if remainingNonRelay > 0 {
		return nil
	}

	var attachRefs []Reference
	for _, cfg := range restAPIs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		paths := config.MtlsAuthAttachmentFieldPaths(&restCfg)
		if len(paths) == 0 {
			continue
		}
		attachRefs = append(attachRefs, Reference{Field: paths[0], API: cfg.Handle})
	}
	if len(attachRefs) == 0 {
		return nil
	}

	verb := "attach"
	if len(attachRefs) == 1 {
		verb = "attaches"
	}
	return &InUseError{
		Message: fmt.Sprintf("cannot remove the last client-CA authority while %s %s mtls-auth; add a replacement first or remove those APIs",
			pluralDeployedAPIs(len(attachRefs)), verb),
		References: attachRefs,
	}
}

// checkUpstreamCertificateDeletable reports whether a usage: upstream
// certificate can be removed. It refuses while a deployed API names it in
// upstreamDefinitions[].tls.trustedCAs, and on a failed read.
func (s *CertificateService) checkUpstreamCertificateDeletable(cert *models.StoredCertificate) error {
	deployed, err := s.deployedConfigs(models.KindRestApi, models.KindAgent)
	if err != nil {
		s.loggerOr(nil).Error("Failed to read deployed configurations for upstream-certificate referential-integrity check",
			slog.String("certificate", cert.Name), slog.Any("error", err))
		return &ReferenceCheckError{Message: "Failed to verify certificate references", Cause: err}
	}

	var refs []Reference
	for _, cfg := range deployed {
		paths := config.NamedTLSTrustedCAFieldPaths(upstreamDefinitionsOf(cfg), cert.Name)
		if len(paths) == 0 {
			continue
		}
		refs = append(refs, Reference{Field: paths[0], API: cfg.Handle})
	}
	if len(refs) == 0 {
		return nil
	}

	return &InUseError{
		Message: fmt.Sprintf("certificate '%s' is named by %s; remove those references first",
			cert.Name, pluralDeployedAPIs(len(refs))),
		References: refs,
	}
}

// checkGatewayIdentityDeletable reports whether a usage: identity certificate
// can be removed. It refuses while a deployed API names it in
// upstreamDefinitions[].tls.identity, and on a failed read.
func (s *CertificateService) checkGatewayIdentityDeletable(cert *models.StoredCertificate) error {
	deployed, err := s.deployedConfigs(models.KindRestApi, models.KindAgent)
	if err != nil {
		s.loggerOr(nil).Error("Failed to read deployed configurations for gateway-identity referential-integrity check",
			slog.String("identity", cert.Name), slog.Any("error", err))
		return &ReferenceCheckError{Message: "Failed to verify gateway identity references", Cause: err}
	}

	var refs []Reference
	for _, cfg := range deployed {
		paths := config.NamedTLSIdentityFieldPaths(upstreamDefinitionsOf(cfg), cert.Name)
		if len(paths) == 0 {
			continue
		}
		refs = append(refs, Reference{Field: paths[0], API: cfg.Handle})
	}
	if len(refs) == 0 {
		return nil
	}

	return &InUseError{
		Message: fmt.Sprintf("gateway identity '%s' is named by %s; remove those references first",
			cert.Name, pluralDeployedAPIs(len(refs))),
		References: refs,
	}
}

// countClientCertificateReferences counts the deployed RestApi configurations
// whose mtls-auth instances name certName in an accept entry's ca. An API
// that only inherits the pool is not counted.
func (s *CertificateService) countClientCertificateReferences(certName string) (int, error) {
	restAPIs, err := s.deployedConfigs(models.KindRestApi)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, cfg := range restAPIs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		if len(config.NamedAcceptEntryFieldPaths(&restCfg, certName)) > 0 {
			count++
		}
	}
	return count, nil
}

// countGatewayIdentityReferences counts the deployed configurations whose
// upstreamDefinitions[].tls.identity explicitly names identityName.
func (s *CertificateService) countGatewayIdentityReferences(identityName string) (int, error) {
	deployed, err := s.deployedConfigs(models.KindRestApi, models.KindAgent)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, cfg := range deployed {
		if len(config.NamedTLSIdentityFieldPaths(upstreamDefinitionsOf(cfg), identityName)) > 0 {
			count++
		}
	}
	return count, nil
}

// deployedConfigs returns every deployed configuration of the given kinds from
// both the database and the in-memory store. An error means the database read
// failed; delete paths must treat that as a refusal, never as "no references".
func (s *CertificateService) deployedConfigs(kinds ...string) ([]*models.StoredConfig, error) {
	seen := make(map[string]*models.StoredConfig)
	store := s.store()

	for _, kind := range kinds {
		configs, err := s.deps.DB.GetAllConfigsByKind(kind)
		if err != nil {
			return nil, err
		}
		for _, cfg := range configs {
			if cfg.DesiredState == models.StateDeployed {
				seen[cfg.UUID] = cfg
			}
		}

		// The in-memory store converges from the database asynchronously, so a
		// reference found in either source counts as live. Reading only one
		// lets a delete through while the other still uses the certificate.
		if store != nil {
			for _, cfg := range store.GetAllByKind(kind) {
				if cfg.DesiredState != models.StateDeployed {
					continue
				}
				if _, ok := seen[cfg.UUID]; !ok {
					seen[cfg.UUID] = cfg
				}
			}
		}
	}

	deployed := make([]*models.StoredConfig, 0, len(seen))
	for _, cfg := range seen {
		deployed = append(deployed, cfg)
	}
	return deployed, nil
}

// upstreamDefinitionsOf returns the upstreamDefinitions of a RestApi or
// Agent configuration, or nil for any other kind.
func upstreamDefinitionsOf(cfg *models.StoredConfig) *[]api.UpstreamDefinition {
	switch c := cfg.Configuration.(type) {
	case api.RestAPI:
		return c.Spec.UpstreamDefinitions
	case api.AgentConfiguration:
		return c.Spec.UpstreamDefinitions
	}
	return nil
}

// pluralDeployedAPIs renders "1 deployed API" or "N deployed APIs".
func pluralDeployedAPIs(n int) string {
	if n == 1 {
		return "1 deployed API"
	}
	return fmt.Sprintf("%d deployed APIs", n)
}

const msgCertificateFieldCarriesKey = "the certificate field takes certificates only; the private key belongs in privateKey"

var certificateNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// uploadValidation accumulates every request-level problem found while
// validating an upload, so all of them can be reported together.
type uploadValidation struct {
	fieldErrors []FieldError

	// identityBundle is the inspected chain and key of a usage: identity
	// upload; nil unless both passed inspection.
	identityBundle *gatewayidentity.Bundle
}

func (v *uploadValidation) addFieldError(field, message string) {
	v.fieldErrors = append(v.fieldErrors, FieldError{Field: field, Message: message})
}

func (v *uploadValidation) hasProblems() bool {
	return len(v.fieldErrors) > 0
}

// validateMatchLists checks a relay entry's match narrowing: dnsSANs and
// uriSANs, when present, must each list at least one non-empty SAN.
func (v *uploadValidation) validateMatchLists(match *models.CertificateMatch) {
	v.validateMatchList("match.dnsSANs", match.DNSSANs)
	v.validateMatchList("match.uriSANs", match.URISANs)
}

func (v *uploadValidation) validateMatchList(fieldPath string, list []string) {
	if list == nil {
		return
	}
	const emptyMessage = "list at least one non-empty SAN"
	if len(list) == 0 {
		v.addFieldError(fieldPath, emptyMessage)
		return
	}
	for i, s := range list {
		if strings.TrimSpace(s) == "" {
			v.addFieldError(fmt.Sprintf("%s[%d]", fieldPath, i), emptyMessage)
		}
	}
}

// validateUpload validates an upload's fields and its certificate content for
// the given usage, collecting every problem. It returns the result, the
// defaulted usage and role, and for usage: downstream the inspected bundle.
func (s *CertificateService) validateUpload(p UploadParams) (*uploadValidation, string, string, *clientca.Bundle) {
	v := &uploadValidation{}

	nameProvided := p.Name != ""
	certProvided := p.Certificate != ""
	keyProvided := p.PrivateKey != ""

	if !nameProvided {
		v.addFieldError("name", "both name and certificate are required")
	}

	if nameProvided && !certificateNamePattern.MatchString(p.Name) {
		v.addFieldError("name", "name may contain only letters, digits, ., _ and -")
	}

	usageProvided := p.Usage != ""
	usageValid := true
	effectiveUsage := models.CertificateUsageUpstream
	if usageProvided {
		switch p.Usage {
		case models.CertificateUsageUpstream, models.CertificateUsageDownstream, models.CertificateUsageIdentity:
			effectiveUsage = p.Usage
		default:
			usageValid = false
			v.addFieldError("usage", "usage must be upstream, downstream or identity")
		}
	}

	if effectiveUsage == models.CertificateUsageIdentity {
		if !certProvided {
			v.addFieldError("certificate", "both certificate and privateKey are required for usage: identity")
		}
		if !keyProvided {
			v.addFieldError("privateKey", "both certificate and privateKey are required for usage: identity")
		}
	} else {
		if !certProvided {
			v.addFieldError("certificate", "both name and certificate are required")
		}
		if keyProvided {
			v.addFieldError("privateKey", "privateKey applies only to usage: identity certificates")
		}
	}

	roleProvided := p.Role != ""
	effectiveRole := ""
	if effectiveUsage == models.CertificateUsageDownstream {
		effectiveRole = models.CertificateRoleClient
	}
	if roleProvided {
		switch {
		case p.Role != models.CertificateRoleClient && p.Role != models.CertificateRoleRelay && p.Role != models.CertificateRoleDefault:
			v.addFieldError("role", "role must be client, relay or default")
		case !usageValid:
			// The usage error already reports the problem.
		case p.Role == models.CertificateRoleDefault && effectiveUsage != models.CertificateUsageIdentity:
			v.addFieldError("role", "role default applies only to usage: identity certificates")
		case p.Role != models.CertificateRoleDefault && effectiveUsage != models.CertificateUsageDownstream:
			v.addFieldError("role", fmt.Sprintf("role %s applies only to usage: downstream certificates", p.Role))
		default:
			effectiveRole = p.Role
		}
	}

	if p.Match != nil {
		if effectiveRole != models.CertificateRoleRelay {
			v.addFieldError("match", "match applies only to role: relay entries")
		} else {
			v.validateMatchLists(p.Match)
		}
	}

	var bundle *clientca.Bundle
	if usageValid {
		switch {
		case effectiveUsage == models.CertificateUsageDownstream && certProvided:
			b, err := clientca.Inspect([]byte(p.Certificate), time.Now())
			if err != nil {
				var fe *clientca.FieldError
				if errors.As(err, &fe) {
					v.addFieldError(fe.Field, fe.Message)
				} else {
					v.addFieldError("certificate", clientca.MsgNotPEMCertificate)
				}
			} else {
				bundle = b
			}
		case certProvided && pemCarriesPrivateKey([]byte(p.Certificate)):
			// clientca.Inspect refuses a key for usage: downstream with its own message.
			v.addFieldError("certificate", msgCertificateFieldCarriesKey)
		case effectiveUsage == models.CertificateUsageIdentity && certProvided && keyProvided:
			ib, err := gatewayidentity.Inspect([]byte(p.Certificate), []byte(p.PrivateKey), time.Now())
			if err != nil {
				var fe *gatewayidentity.FieldError
				if errors.As(err, &fe) {
					v.addFieldError(fe.Field, fe.Message)
				} else {
					v.addFieldError("certificate", clientca.MsgNotPEMCertificate)
				}
			} else {
				v.identityBundle = ib
			}
		case effectiveUsage == models.CertificateUsageUpstream && certProvided:
			if _, err := ValidateChain([]byte(p.Certificate)); err != nil {
				v.addFieldError("certificate", clientca.MsgNotPEMCertificate)
			}
		}
	}

	return v, effectiveUsage, effectiveRole, bundle
}

func (s *CertificateService) store() *storage.ConfigStore {
	if s.deps.Store == nil {
		return nil
	}
	return s.deps.Store()
}

func (s *CertificateService) encryptionManager() *encryption.ProviderManager {
	if s.deps.EncryptionManager == nil {
		return nil
	}
	return s.deps.EncryptionManager()
}

// encryptPrivateKey encrypts a usage: identity PEM private key and marshals
// the result for storage.
func (s *CertificateService) encryptPrivateKey(privateKeyPEM string) (string, error) {
	payload, err := s.encryptionManager().Encrypt([]byte(privateKeyPEM))
	if err != nil {
		return "", err
	}
	return encryption.MarshalPayload(payload), nil
}

func (s *CertificateService) publishEvent(action, certID, correlationID string, log *slog.Logger) {
	if s.deps.PublishEvent != nil {
		s.deps.PublishEvent(action, certID, correlationID, log)
	}
}

func (s *CertificateService) publishClientAuthorities(correlationID string) error {
	if s.deps.PublishClientAuthorities == nil {
		return nil
	}
	return s.deps.PublishClientAuthorities(correlationID)
}

func (s *CertificateService) defaultIdentityMu() *sync.Mutex {
	if s.deps.DefaultIdentityMu != nil {
		return s.deps.DefaultIdentityMu
	}
	return &s.ownMu
}

func (s *CertificateService) refreshMetrics(log *slog.Logger, after string) {
	if _, err := certmetrics.Refresh(s.deps.DB); err != nil {
		log.Warn("Failed to refresh certificate metrics after "+after, slog.Any("error", err))
	}
}

// defaultGatewayIdentity returns the role: default gateway identity, or nil
// when there is none.
func (s *CertificateService) defaultGatewayIdentity() (*models.StoredCertificate, error) {
	identities, err := s.deps.DB.ListCertificatesByUsage(models.CertificateUsageIdentity)
	if err != nil {
		return nil, err
	}
	for _, cert := range identities {
		if cert.IsDefaultIdentity() {
			return cert, nil
		}
	}
	return nil, nil
}

// xdsTargets resolves the router-side collaborators, reporting
// ErrCertStoreNotConfigured when this gateway has none.
//
// These checks cover an absent resolver, a resolver that reports no xDS layer,
// and a half-populated result. They cannot catch a typed-nil pointer stored in
// the interface — `Store == nil` is false for that — so the contract is that a
// resolver returns a nil *XDSTargets rather than a struct holding nil pointers.
func (s *CertificateService) xdsTargets() (*XDSTargets, error) {
	if s.deps.ResolveXDS == nil {
		return nil, ErrCertStoreNotConfigured
	}
	targets := s.deps.ResolveXDS()
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
	if s.deps.Logger != nil {
		return s.deps.Logger
	}

	return slog.Default()
}

// ValidateChain reports how many CERTIFICATE blocks the PEM data contains,
// returning an error if any block fails to parse or none are present.
// Non-CERTIFICATE PEM blocks are skipped rather than rejected; validateUpload
// refuses a private key block separately.
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

// firstX509Certificate parses and returns the first CERTIFICATE PEM block in
// data.
func firstX509Certificate(data []byte) (*x509.Certificate, error) {
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
		return x509.ParseCertificate(block.Bytes)
	}
	return nil, fmt.Errorf("no certificate found")
}

// pemCarriesPrivateKey reports whether any PEM block in data is a private
// key of any encoding.
func pemCarriesPrivateKey(data []byte) bool {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return false
		}
		if strings.Contains(block.Type, "PRIVATE KEY") {
			return true
		}
	}
}
