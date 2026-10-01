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

package handlers

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers/handlerkit"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certmetrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/gatewayidentity"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// UploadCertificateRequest represents the request body for certificate upload
type UploadCertificateRequest struct {
	Certificate string                   `json:"certificate" binding:"required"` // PEM-encoded certificate
	Name        string                   `json:"name" binding:"required"`        // Unique certificate name
	Usage       string                   `json:"usage"`                          // "upstream" (default), "downstream" or "identity"
	Role        string                   `json:"role"`                           // "client" (default) or "relay"; usage: downstream only
	Match       *models.CertificateMatch `json:"match,omitempty"`                // Only valid for role: relay
	PrivateKey  string                   `json:"privateKey,omitempty"`           // Required (and only valid) for usage: identity
}

// CertificateResponse represents a certificate information response
type CertificateResponse struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Subject          string                   `json:"subject,omitempty"`
	Issuer           string                   `json:"issuer,omitempty"`
	NotAfter         string                   `json:"notAfter,omitempty"`
	Count            int                      `json:"count"` // Number of certs in file
	Usage            string                   `json:"usage"`
	Role             string                   `json:"role,omitempty"`
	Match            *models.CertificateMatch `json:"match,omitempty"`
	IsLeaf           bool                     `json:"isLeaf"`
	KeyAlgorithm     string                   `json:"keyAlgorithm,omitempty"` // Only present for usage: identity
	ChainLength      int                      `json:"chainLength,omitempty"`  // Only present for usage: identity
	Warnings         []clientca.Warning       `json:"warnings,omitempty"`
	ReferencedByApis *int                     `json:"referencedByApis,omitempty"`
	Message          string                   `json:"message,omitempty"`
	Status           string                   `json:"status"` // success, error
}

// ListCertificatesResponse represents the response for listing certificates
type ListCertificatesResponse struct {
	Certificates []CertificateResponse `json:"certificates"`
	TotalCount   int                   `json:"totalCount"`
	TotalBytes   int                   `json:"totalBytes"`
	Status       string                `json:"status"`
}

// UploadCertificate handles certificate upload via REST API
// POST /certificates
func (s *APIServer) UploadCertificate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	r.Body = http.MaxBytesReader(w, r.Body, s.systemConfig.Controller.Server.MaxCertificateUploadBytes)

	var req UploadCertificateRequest
	shapeErrors, err := decodeCertificateUpload(r.Body, &req)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			// Generic message: never state the configured limit.
			httputil.WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"status":  "error",
				"message": "the request body is too large",
			})
			return
		}
		log.Warn("Invalid certificate upload request", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	validation, effectiveUsage, effectiveRole, bundle := s.validateCertificateUpload(&req)
	for _, fe := range shapeErrors {
		validation.addFieldError(fe.field, fe.message)
	}

	if validation.hasProblems() {
		fieldErrors := validation.fieldErrors
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: certificateUploadInvalidMessage,
			Errors:  &fieldErrors,
		})
		return
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

	certData := []byte(req.Certificate)

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

		if s.encryptionManager == nil {
			log.Error("Cannot store gateway identity: no encryption provider configured")
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "gateway identity storage is not available",
			})
			return
		}
		ciphertext, err := s.encryptPrivateKey(req.PrivateKey)
		if err != nil {
			log.Error("Failed to encrypt gateway identity private key", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "failed to store gateway identity",
			})
			return
		}
		privateKeyCiphertext = ciphertext
	default:
		var err error
		count, err = s.validateCertificate(certData)
		if err == nil {
			subject, issuer, notBefore, notAfter, err = s.extractCertificateMetadata(certData)
		}
		if err != nil {
			// Unreachable: validateCertificateUpload already accepted these bytes.
			log.Error("Failed to read a validated certificate", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Failed to save certificate",
			})
			return
		}
		if firstCert, err := firstX509Certificate(certData); err == nil {
			isLeaf = !firstCert.IsCA
		} else {
			log.Warn("Failed to determine certificate authority status", slog.Any("error", err))
		}
	}

	// A client authority or identity already inside its expiry horizon gets
	// the same CERT_EXPIRES_SOON warning the listing endpoint reports.
	if effectiveUsage == models.CertificateUsageDownstream || effectiveUsage == models.CertificateUsageIdentity {
		if warning := clientca.ExpiryWarning(notAfter, time.Now()); warning != nil {
			warnings = append(warnings, *warning)
		}
	}

	for _, warning := range warnings {
		fields := []any{slog.String("code", warning.Code), slog.String("name", req.Name)}
		log.Warn("Client certificate authority warning", fields...)
	}

	// Generate unique ID (UUID v7)
	certID, err := utils.GenerateUUID()
	if err != nil {
		log.Error("Failed to generate certificate ID", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Failed to generate certificate ID",
		})
		return
	}

	// Create certificate model
	cert := &models.StoredCertificate{
		UUID:                 certID,
		Name:                 req.Name,
		Certificate:          certData,
		Subject:              subject,
		Issuer:               issuer,
		NotBefore:            notBefore,
		NotAfter:             notAfter,
		CertCount:            count,
		Usage:                effectiveUsage,
		Role:                 effectiveRole,
		Match:                req.Match,
		PrivateKeyCiphertext: privateKeyCiphertext,
		KeyAlgorithm:         keyAlgorithm,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Save to database
	if err := s.db.SaveCertificate(cert); err != nil {
		if storage.IsConflictError(err) {
			message := fmt.Sprintf("a certificate named %s already exists", req.Name)
			switch effectiveUsage {
			case models.CertificateUsageDownstream:
				message = fmt.Sprintf("a client-CA authority named %s already exists", req.Name)
			case models.CertificateUsageIdentity:
				message = fmt.Sprintf("a gateway identity named %s already exists", req.Name)
			}
			httputil.WriteJSON(w, http.StatusConflict, map[string]any{
				"status":  "error",
				"message": message,
			})
			return
		}
		log.Error("Failed to save certificate to database",
			slog.String("name", req.Name),
			slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Failed to save certificate",
		})
		return
	}

	log.Info("Certificate saved to database successfully",
		slog.String("id", certID),
		slog.String("name", req.Name),
		slog.Int("cert_count", count))
	s.publishCertificateEvent("CREATE", certID, correlationID, log)

	// Get cert store from snapshot manager
	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		log.Error("Certificate store not available")
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate store not configured",
		})
		return
	}

	certStore := translator.GetCertStore()

	// Reload certificates from database
	if err := certStore.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate saved but failed to reload",
		})
		return
	}

	// Trigger SDS update by regenerating the snapshot
	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate reloaded but failed to update SDS",
		})
		return
	}

	log.Info("SDS snapshot updated with new certificate",
		slog.String("id", certID),
		slog.String("name", req.Name))

	if _, err := certmetrics.Refresh(s.db); err != nil {
		log.Warn("Failed to refresh certificate metrics after upload", slog.Any("error", err))
	}

	if effectiveUsage == models.CertificateUsageDownstream {
		if err := s.publishClientAuthorities(correlationID); err != nil {
			log.Error("Failed to publish client certificate authorities", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Certificate saved but failed to update the policy engine",
			})
			return
		}
	}

	resp := CertificateResponse{
		ID:       certID,
		Name:     req.Name,
		Subject:  subject,
		Issuer:   issuer,
		NotAfter: notAfter.Format("2006-01-02 15:04:05"),
		Count:    count,
		Usage:    effectiveUsage,
		IsLeaf:   isLeaf,
		Warnings: warnings,
		Message:  "Certificate uploaded and SDS updated successfully",
		Status:   "success",
	}
	if effectiveUsage == models.CertificateUsageDownstream {
		resp.Role = effectiveRole
		if effectiveRole == models.CertificateRoleRelay {
			resp.Match = req.Match
		}
	}
	if effectiveUsage == models.CertificateUsageIdentity {
		resp.KeyAlgorithm = keyAlgorithm
		resp.ChainLength = count
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

// ListCertificates lists all custom certificates, optionally filtered by usage
// GET /certificates
func (s *APIServer) ListCertificates(w http.ResponseWriter, r *http.Request, params api.ListCertificatesParams) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	usageFilter := ""
	if params.Usage != nil {
		usageFilter = string(*params.Usage)
	}
	if usageFilter != "" && usageFilter != models.CertificateUsageUpstream && usageFilter != models.CertificateUsageDownstream && usageFilter != models.CertificateUsageIdentity {
		fieldErrors := []api.ValidationError{{
			Field:   stringPtr("usage"),
			Message: stringPtr("usage must be upstream, downstream or identity"),
		}}
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "invalid usage filter",
			Errors:  &fieldErrors,
		})
		return
	}

	// Get certificates from database
	var certs []*models.StoredCertificate
	var err error
	if usageFilter != "" {
		certs, err = s.db.ListCertificatesByUsage(usageFilter)
	} else {
		certs, err = s.db.ListCertificates()
	}
	if err != nil {
		log.Error("Failed to list certificates from database", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Failed to list certificates",
		})
		return
	}

	certificates := []CertificateResponse{}
	totalBytes := 0
	now := time.Now()

	for _, cert := range certs {
		totalBytes += len(cert.Certificate)

		usage := cert.EffectiveUsage()

		item := CertificateResponse{
			ID:       cert.UUID,
			Name:     cert.Name,
			Subject:  cert.Subject,
			Issuer:   cert.Issuer,
			NotAfter: cert.NotAfter.Format("2006-01-02 15:04:05"),
			Count:    cert.CertCount,
			Usage:    usage,
			Status:   "success",
		}

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

		certificates = append(certificates, item)
	}

	httputil.WriteJSON(w, http.StatusOK, ListCertificatesResponse{
		Certificates: certificates,
		TotalCount:   len(certificates),
		TotalBytes:   totalBytes,
		Status:       "success",
	})
}

// DeleteCertificate deletes a certificate by ID
// DELETE /certificates/:id
func (s *APIServer) DeleteCertificate(w http.ResponseWriter, r *http.Request, id string) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	if id == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "Certificate ID is required",
		})
		return
	}

	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		log.Error("Certificate store not available")
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate store not configured",
		})
		return
	}

	// The row's usage and role decide which referential checks apply, so a
	// read failure here must stop the delete: skipping the checks would let a
	// referenced authority or identity disappear on a transient store error.
	preDeleteCert, err := s.db.GetCertificate(id)
	if err != nil && !storage.IsNotFoundError(err) {
		log.Error("Failed to read certificate before delete", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "the certificate could not be read",
		})
		return
	}

	// A non-relay client authority cannot be removed while a deployed API
	// depends on it. Removing the last relay entry just turns header mode off.
	if preDeleteCert != nil {
		role := preDeleteCert.EffectiveRole()
		usage := preDeleteCert.EffectiveUsage()
		if usage == models.CertificateUsageDownstream && role != models.CertificateRoleRelay {
			if errResp, statusCode, blocked := s.checkClientAuthorityDeletable(preDeleteCert); blocked {
				httputil.WriteJSON(w, statusCode, errResp)
				return
			}
		}
		if usage == models.CertificateUsageUpstream {
			if errResp, statusCode, blocked := s.checkUpstreamCertificateDeletable(preDeleteCert); blocked {
				httputil.WriteJSON(w, statusCode, errResp)
				return
			}
		}
		if usage == models.CertificateUsageIdentity {
			if errResp, statusCode, blocked := s.checkGatewayIdentityDeletable(preDeleteCert); blocked {
				httputil.WriteJSON(w, statusCode, errResp)
				return
			}
		}
	}

	// Delete from database
	if err := s.db.DeleteCertificate(id); err != nil {
		log.Error("Failed to delete certificate",
			slog.String("id", id),
			slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "Certificate not found or failed to delete: " + err.Error(),
		})
		return
	}

	log.Info("Certificate deleted from database", slog.String("id", id))
	s.publishCertificateEvent("DELETE", id, correlationID, log)

	certStore := translator.GetCertStore()

	// Reload certificates from database
	if err := certStore.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate deleted but failed to reload",
		})
		return
	}

	// Trigger SDS update
	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate deleted and reloaded but failed to update SDS",
		})
		return
	}

	log.Info("SDS snapshot updated after certificate deletion", slog.String("id", id))

	if _, err := certmetrics.Refresh(s.db); err != nil {
		log.Warn("Failed to refresh certificate metrics after delete", slog.Any("error", err))
	}

	if preDeleteCert != nil && preDeleteCert.Usage == models.CertificateUsageDownstream {
		if err := s.publishClientAuthorities(correlationID); err != nil {
			log.Error("Failed to publish client certificate authorities", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Certificate deleted but failed to update the policy engine",
			})
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Certificate deleted and SDS updated successfully",
		"id":      id,
	})
}

// ReloadCertificates manually triggers certificate reload and SDS update
// POST /certificates/reload
func (s *APIServer) ReloadCertificates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	translator := s.snapshotManager.GetTranslator()
	if translator == nil || translator.GetCertStore() == nil {
		log.Error("Certificate store not available")
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificate store not configured",
		})
		return
	}

	certStore := translator.GetCertStore()

	// Reload certificates from database
	if err := certStore.Reload(); err != nil {
		log.Error("Failed to reload certificates", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Failed to reload certificates",
		})
		return
	}

	// Trigger SDS update
	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificates reloaded but failed to update SDS",
		})
		return
	}

	log.Info("Certificates reloaded and SDS snapshot updated")
	// A reload names no single row; the events table admits only CREATE,
	// UPDATE and DELETE, and every replica rebuilds from the database for
	// any certificate action.
	s.publishCertificateEvent("UPDATE", "", correlationID, log)

	if err := s.publishClientAuthorities(correlationID); err != nil {
		log.Error("Failed to publish client certificate authorities", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Certificates reloaded but failed to update the policy engine",
		})
		return
	}

	if _, err := certmetrics.Refresh(s.db); err != nil {
		log.Warn("Failed to refresh certificate metrics after reload", slog.Any("error", err))
	}

	combinedCerts := certStore.GetCombinedCertificates()
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"message":    "Certificates reloaded and SDS updated successfully",
		"totalBytes": len(combinedCerts),
	})
}

// extractCertificateMetadata extracts metadata from the first certificate in the chain
func (s *APIServer) extractCertificateMetadata(data []byte) (subject, issuer string, notBefore, notAfter time.Time, err error) {
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

func (s *APIServer) validateCertificate(data []byte) (int, error) {
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

// UpdateCertificate rotates a usage: identity certificate's chain and private
// key in place, keeping its name. Other usages are refused.
// PUT /certificates/{id}
func (s *APIServer) UpdateCertificate(w http.ResponseWriter, r *http.Request, id string) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	existing, err := s.db.GetCertificate(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, map[string]any{
				"status":  "error",
				"message": "certificate not found",
			})
			return
		}
		log.Error("Failed to read certificate before update", slog.String("id", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "the certificate could not be read",
		})
		return
	}

	if existing.Usage != models.CertificateUsageIdentity {
		fieldErrors := []api.ValidationError{{
			Field:   stringPtr("usage"),
			Message: stringPtr("only usage: identity certificates can be updated; delete and re-upload other certificates"),
		}}
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: certificateUploadInvalidMessage,
			Errors:  &fieldErrors,
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.systemConfig.Controller.Server.MaxCertificateUploadBytes)

	var req UploadCertificateRequest
	shapeErrors, err := decodeCertificateUpload(r.Body, &req)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			httputil.WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"status":  "error",
				"message": "the request body is too large",
			})
			return
		}
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "invalid request body",
		})
		return
	}
	// Name, usage, role and match are immutable on rotation, so only the
	// certificate and key from the request are validated.
	req.Name = existing.Name
	req.Usage = models.CertificateUsageIdentity
	req.Role = ""
	req.Match = nil

	validation, _, _, _ := s.validateCertificateUpload(&req)
	for _, fe := range shapeErrors {
		validation.addFieldError(fe.field, fe.message)
	}
	if validation.hasProblems() {
		fieldErrors := validation.fieldErrors
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: certificateUploadInvalidMessage,
			Errors:  &fieldErrors,
		})
		return
	}

	if s.encryptionManager == nil {
		log.Error("Cannot store gateway identity: no encryption provider configured")
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "gateway identity storage is not available",
		})
		return
	}
	ciphertext, err := s.encryptPrivateKey(req.PrivateKey)
	if err != nil {
		log.Error("Failed to encrypt gateway identity private key", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "failed to store gateway identity",
		})
		return
	}

	ib := validation.identityBundle
	updated := &models.StoredCertificate{
		UUID:                 existing.UUID,
		Name:                 existing.Name,
		Certificate:          []byte(req.Certificate),
		Subject:              ib.Leaf.Subject.String(),
		Issuer:               ib.Leaf.Issuer.String(),
		NotBefore:            ib.Leaf.NotBefore,
		NotAfter:             ib.Leaf.NotAfter,
		CertCount:            len(ib.Chain),
		Usage:                models.CertificateUsageIdentity,
		PrivateKeyCiphertext: ciphertext,
		KeyAlgorithm:         ib.KeyAlgorithm,
		UpdatedAt:            time.Now(),
	}

	if err := s.db.UpdateCertificate(updated); err != nil {
		log.Error("Failed to update certificate", slog.String("id", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "failed to update certificate",
		})
		return
	}
	s.publishCertificateEvent("UPDATE", updated.UUID, correlationID, log)

	// The SDS update rebuilds this identity's gateway_identity:<name> secret.
	// Reload does nothing for identity rows but keeps parity with upload.
	if translator := s.snapshotManager.GetTranslator(); translator != nil {
		if err := translator.GetCertStore().Reload(); err != nil {
			log.Warn("Failed to reload certificate store after identity rotation", slog.Any("error", err))
		}
	}

	if err := s.snapshotManager.UpdateSnapshot(context.Background(), correlationID); err != nil {
		log.Error("Failed to update SDS snapshot after certificate update", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "certificate updated but failed to update SDS",
		})
		return
	}

	if _, err := certmetrics.Refresh(s.db); err != nil {
		log.Warn("Failed to refresh certificate metrics after update", slog.Any("error", err))
	}

	var warnings []clientca.Warning
	for _, warn := range ib.Warnings {
		warnings = append(warnings, clientca.Warning{Code: warn.Code, Field: warn.Field, Message: warn.Message})
	}
	if warning := clientca.ExpiryWarning(updated.NotAfter, time.Now()); warning != nil {
		warnings = append(warnings, *warning)
	}

	resp := CertificateResponse{
		ID:           updated.UUID,
		Name:         updated.Name,
		Subject:      updated.Subject,
		Issuer:       updated.Issuer,
		NotAfter:     updated.NotAfter.Format("2006-01-02 15:04:05"),
		Count:        updated.CertCount,
		Usage:        models.CertificateUsageIdentity,
		IsLeaf:       !ib.Leaf.IsCA,
		KeyAlgorithm: updated.KeyAlgorithm,
		ChainLength:  updated.CertCount,
		Warnings:     warnings,
		Message:      "Certificate updated and SDS updated successfully",
		Status:       "success",
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// publishCertificateEvent tells every replica sharing the event hub that the
// certificate rows changed, so each rebuilds its certificate store, client
// authority pool and xDS snapshot from the database. It runs once the write
// has committed; the event carries the row id, never certificate material.
func (s *APIServer) publishCertificateEvent(action, certID, correlationID string, log *slog.Logger) {
	(&handlerkit.EventPublisher{EventHub: s.eventHub, GatewayID: s.gatewayID}).
		PublishEvent(eventhub.EventTypeCertificate, action, certID, correlationID, log)
}

// encryptPrivateKey encrypts a usage: identity PEM private key and marshals
// the result for storage.
func (s *APIServer) encryptPrivateKey(privateKeyPEM string) (string, error) {
	payload, err := s.encryptionManager.Encrypt([]byte(privateKeyPEM))
	if err != nil {
		return "", err
	}
	return encryption.MarshalPayload(payload), nil
}

// deployedRestAPIConfigs returns every deployed RestApi configuration from
// both the database and the in-memory store. An error means the database
// read failed; delete paths must treat that as a refusal, never as "no
// references". A nil database yields no configs.
func (s *APIServer) deployedRestAPIConfigs() ([]*models.StoredConfig, error) {
	return s.deployedConfigs(models.KindRestApi)
}

// deployedUpstreamTLSConfigs returns every deployed configuration whose
// upstreamDefinitions can carry a tls block, on the same terms as
// deployedRestAPIConfigs.
func (s *APIServer) deployedUpstreamTLSConfigs() ([]*models.StoredConfig, error) {
	return s.deployedConfigs(models.KindRestApi, models.KindAgent)
}

// deployedConfigs returns every deployed configuration of the given kinds
// from both the database and the in-memory store.
func (s *APIServer) deployedConfigs(kinds ...string) ([]*models.StoredConfig, error) {
	seen := make(map[string]*models.StoredConfig)

	for _, kind := range kinds {
		if s.db != nil {
			configs, err := s.db.GetAllConfigsByKind(kind)
			if err != nil {
				return nil, err
			}
			for _, cfg := range configs {
				if cfg.DesiredState == models.StateDeployed {
					seen[cfg.UUID] = cfg
				}
			}
		}

		// The in-memory store converges from the database asynchronously, so a
		// reference found in either source counts as live. Reading only one
		// lets a delete through while the other still uses the certificate.
		if s.store != nil {
			for _, cfg := range s.store.GetAllByKind(kind) {
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

// countClientCertificateReferences counts the deployed RestApi configurations
// whose mtls-auth instances name certName in an accept entry's ca. An API
// that only inherits the pool is not counted.
func (s *APIServer) countClientCertificateReferences(certName string) (int, error) {
	restAPIs, err := s.deployedRestAPIConfigs()
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

// clientAuthorityReference is one deployed API's dependency on the client
// authority pool, either by naming an authority or by inheriting the pool.
type clientAuthorityReference struct {
	apiHandle string
	fieldPath string
}

// pluralDeployedAPIs renders "1 deployed API" or "N deployed APIs".
func pluralDeployedAPIs(n int) string {
	if n == 1 {
		return "1 deployed API"
	}
	return fmt.Sprintf("%d deployed APIs", n)
}

// checkClientAuthorityDeletable reports whether cert, a non-relay usage:
// downstream authority, can be removed. It refuses when a deployed API names the
// authority, or when it is the last non-relay authority and any deployed API
// attaches mtls-auth. A failed read refuses with a 500. When refused it
// returns the error body, the status and true.
func (s *APIServer) checkClientAuthorityDeletable(cert *models.StoredCertificate) (api.ErrorResponse, int, bool) {
	restAPIs, err := s.deployedRestAPIConfigs()
	if err != nil {
		s.logger.Error("Failed to read deployed RestApi configurations for client-CA referential-integrity check",
			slog.String("certificate", cert.Name), slog.Any("error", err))
		return api.ErrorResponse{Status: "error", Message: "Failed to verify certificate references"},
			http.StatusInternalServerError, true
	}

	var namedRefs []clientAuthorityReference
	for _, cfg := range restAPIs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		paths := config.NamedAcceptEntryFieldPaths(&restCfg, cert.Name)
		if len(paths) == 0 {
			continue
		}
		namedRefs = append(namedRefs, clientAuthorityReference{apiHandle: cfg.Handle, fieldPath: paths[0]})
	}
	if len(namedRefs) > 0 {
		errs := make([]api.ValidationError, len(namedRefs))
		for i, ref := range namedRefs {
			errs[i] = api.ValidationError{
				Field:   stringPtr(ref.fieldPath),
				Message: stringPtr(fmt.Sprintf("referenced by API '%s'", ref.apiHandle)),
			}
		}
		message := fmt.Sprintf("client-CA authority '%s' is named by %s; remove those references first",
			cert.Name, pluralDeployedAPIs(len(namedRefs)))
		return api.ErrorResponse{Status: "error", Message: message, Errors: &errs}, http.StatusConflict, true
	}

	remainingNonRelay := 0
	if clientCerts, err := s.db.ListCertificatesByUsage(models.CertificateUsageDownstream); err == nil {
		for _, c := range clientCerts {
			if c.UUID == cert.UUID {
				continue // the one about to be deleted
			}
			role := c.EffectiveRole()
			if role != models.CertificateRoleRelay {
				remainingNonRelay++
			}
		}
	}
	if remainingNonRelay > 0 {
		return api.ErrorResponse{}, 0, false
	}

	var attachRefs []clientAuthorityReference
	for _, cfg := range restAPIs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		paths := config.MtlsAuthAttachmentFieldPaths(&restCfg)
		if len(paths) == 0 {
			continue
		}
		attachRefs = append(attachRefs, clientAuthorityReference{apiHandle: cfg.Handle, fieldPath: paths[0]})
	}
	if len(attachRefs) == 0 {
		return api.ErrorResponse{}, 0, false
	}

	errs := make([]api.ValidationError, len(attachRefs))
	for i, ref := range attachRefs {
		errs[i] = api.ValidationError{
			Field:   stringPtr(ref.fieldPath),
			Message: stringPtr(fmt.Sprintf("referenced by API '%s'", ref.apiHandle)),
		}
	}
	verb := "attach"
	if len(attachRefs) == 1 {
		verb = "attaches"
	}
	message := fmt.Sprintf("cannot remove the last client-CA authority while %s %s mtls-auth; add a replacement first or remove those APIs",
		pluralDeployedAPIs(len(attachRefs)), verb)
	return api.ErrorResponse{Status: "error", Message: message, Errors: &errs}, http.StatusConflict, true
}

// checkUpstreamCertificateDeletable reports whether a usage: upstream
// certificate can be removed. It refuses while a deployed API names it in
// upstreamDefinitions[].tls.trustedCAs, and refuses with a 500 on a failed read.
func (s *APIServer) checkUpstreamCertificateDeletable(cert *models.StoredCertificate) (api.ErrorResponse, int, bool) {
	deployed, err := s.deployedUpstreamTLSConfigs()
	if err != nil {
		s.logger.Error("Failed to read deployed configurations for upstream-certificate referential-integrity check",
			slog.String("certificate", cert.Name), slog.Any("error", err))
		return api.ErrorResponse{Status: "error", Message: "Failed to verify certificate references"},
			http.StatusInternalServerError, true
	}

	var refs []clientAuthorityReference
	for _, cfg := range deployed {
		paths := config.NamedTLSTrustedCAFieldPaths(upstreamDefinitionsOf(cfg), cert.Name)
		if len(paths) == 0 {
			continue
		}
		refs = append(refs, clientAuthorityReference{apiHandle: cfg.Handle, fieldPath: paths[0]})
	}
	if len(refs) == 0 {
		return api.ErrorResponse{}, 0, false
	}

	errs := make([]api.ValidationError, len(refs))
	for i, ref := range refs {
		errs[i] = api.ValidationError{
			Field:   stringPtr(ref.fieldPath),
			Message: stringPtr(fmt.Sprintf("referenced by API '%s'", ref.apiHandle)),
		}
	}
	message := fmt.Sprintf("certificate '%s' is named by %s; remove those references first",
		cert.Name, pluralDeployedAPIs(len(refs)))
	return api.ErrorResponse{Status: "error", Message: message, Errors: &errs}, http.StatusConflict, true
}

// countGatewayIdentityReferences counts the deployed configurations whose
// upstreamDefinitions[].tls.identity explicitly names identityName.
func (s *APIServer) countGatewayIdentityReferences(identityName string) (int, error) {
	deployed, err := s.deployedUpstreamTLSConfigs()
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

// checkGatewayIdentityDeletable reports whether a usage: identity certificate
// can be removed. It refuses while a deployed API names it in
// upstreamDefinitions[].tls.identity, and refuses with a 500 on a failed read.
func (s *APIServer) checkGatewayIdentityDeletable(cert *models.StoredCertificate) (api.ErrorResponse, int, bool) {
	deployed, err := s.deployedUpstreamTLSConfigs()
	if err != nil {
		s.logger.Error("Failed to read deployed configurations for gateway-identity referential-integrity check",
			slog.String("identity", cert.Name), slog.Any("error", err))
		return api.ErrorResponse{Status: "error", Message: "Failed to verify gateway identity references"},
			http.StatusInternalServerError, true
	}

	var refs []clientAuthorityReference
	for _, cfg := range deployed {
		paths := config.NamedTLSIdentityFieldPaths(upstreamDefinitionsOf(cfg), cert.Name)
		if len(paths) == 0 {
			continue
		}
		refs = append(refs, clientAuthorityReference{apiHandle: cfg.Handle, fieldPath: paths[0]})
	}
	if len(refs) == 0 {
		return api.ErrorResponse{}, 0, false
	}

	errs := make([]api.ValidationError, len(refs))
	for i, ref := range refs {
		errs[i] = api.ValidationError{
			Field:   stringPtr(ref.fieldPath),
			Message: stringPtr(fmt.Sprintf("referenced by API '%s'", ref.apiHandle)),
		}
	}
	message := fmt.Sprintf("gateway identity '%s' is named by %s; remove those references first",
		cert.Name, pluralDeployedAPIs(len(refs)))
	return api.ErrorResponse{Status: "error", Message: message, Errors: &errs}, http.StatusConflict, true
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

const msgCertificateFieldCarriesKey = "the certificate field takes certificates only; the private key belongs in privateKey"

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

// uploadShapeError is a request-body problem found before the typed decode:
// a field the endpoint does not define, or a narrowing given in the wrong
// shape. Either would otherwise be dropped silently, and a dropped
// narrowing widens what a relay entry vouches for.
type uploadShapeError struct {
	field, message string
}

var uploadKnownFields = map[string]bool{
	"certificate": true, "name": true, "usage": true, "role": true, "match": true, "privateKey": true,
}

var uploadKnownMatchFields = map[string]bool{"uriSANs": true, "dnsSANs": true}

// decodeCertificateUpload decodes the upload body strictly. Unknown top-level
// fields, unknown match fields and non-list match values are reported as
// field errors; the typed request is still populated from the well-formed
// remainder so every problem in the body is reported at once.
func decodeCertificateUpload(body io.Reader, req *UploadCertificateRequest) ([]uploadShapeError, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	var shape []uploadShapeError
	for key := range raw {
		if !uploadKnownFields[key] {
			shape = append(shape, uploadShapeError{field: key, message: "unknown field " + key})
			delete(raw, key)
		}
	}

	if matchRaw, ok := raw["match"]; ok && string(bytes.TrimSpace(matchRaw)) != "null" {
		var matchFields map[string]json.RawMessage
		if err := json.Unmarshal(matchRaw, &matchFields); err != nil {
			shape = append(shape, uploadShapeError{field: "match", message: "match must be an object listing uriSANs or dnsSANs"})
			delete(raw, "match")
		} else {
			dropMatch := false
			for key, value := range matchFields {
				if !uploadKnownMatchFields[key] {
					shape = append(shape, uploadShapeError{field: "match." + key, message: "unknown field " + key + "; match takes uriSANs and dnsSANs"})
					dropMatch = true
					continue
				}
				if trimmed := bytes.TrimSpace(value); len(trimmed) == 0 || trimmed[0] != '[' {
					shape = append(shape, uploadShapeError{field: "match." + key, message: key + " must be a list"})
					dropMatch = true
				}
			}
			if len(matchFields) == 0 {
				shape = append(shape, uploadShapeError{field: "match", message: "match must list uriSANs or dnsSANs, or be omitted"})
				dropMatch = true
			}
			if dropMatch {
				delete(raw, "match")
			}
		}
	}

	cleaned, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cleaned, req); err != nil {
		return nil, err
	}
	return shape, nil
}

// certificateUploadInvalidMessage is the top-level message of every
// certificate upload validation 400.
const certificateUploadInvalidMessage = "certificate upload is invalid"

var certificateNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// certUploadValidation accumulates every request-level problem found while
// validating an upload, so all of them can be reported together in one 400.
type certUploadValidation struct {
	fieldErrors []api.ValidationError

	// identityBundle is the inspected chain and key of a usage: identity
	// upload; nil unless both passed inspection.
	identityBundle *gatewayidentity.Bundle
}

func (v *certUploadValidation) addFieldError(field, message string) {
	v.fieldErrors = append(v.fieldErrors, api.ValidationError{
		Field:   stringPtr(field),
		Message: stringPtr(message),
	})
}

func (v *certUploadValidation) hasProblems() bool {
	return len(v.fieldErrors) > 0
}

// validateMatchLists checks a relay entry's match narrowing: dnsSANs and
// uriSANs, when present, must each list at least one non-empty SAN.
func (v *certUploadValidation) validateMatchLists(match *models.CertificateMatch) {
	v.validateMatchList("match.dnsSANs", match.DNSSANs)
	v.validateMatchList("match.uriSANs", match.URISANs)
}

func (v *certUploadValidation) validateMatchList(fieldPath string, list []string) {
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

// validateCertificateUpload validates an upload's fields and its certificate
// content for the given usage, collecting every problem for a single 400. It
// returns the result, the defaulted usage and role, and for usage: downstream the
// inspected bundle.
func (s *APIServer) validateCertificateUpload(req *UploadCertificateRequest) (*certUploadValidation, string, string, *clientca.Bundle) {
	v := &certUploadValidation{}

	nameProvided := req.Name != ""
	certProvided := req.Certificate != ""
	keyProvided := req.PrivateKey != ""

	if !nameProvided {
		v.addFieldError("name", "both name and certificate are required")
	}

	if nameProvided && !certificateNamePattern.MatchString(req.Name) {
		v.addFieldError("name", "name may contain only letters, digits, ., _ and -")
	}

	usageProvided := req.Usage != ""
	usageValid := true
	effectiveUsage := models.CertificateUsageUpstream
	if usageProvided {
		switch req.Usage {
		case models.CertificateUsageUpstream, models.CertificateUsageDownstream, models.CertificateUsageIdentity:
			effectiveUsage = req.Usage
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

	roleProvided := req.Role != ""
	effectiveRole := ""
	if effectiveUsage == models.CertificateUsageDownstream {
		effectiveRole = models.CertificateRoleClient
	}
	if roleProvided {
		if req.Role != models.CertificateRoleClient && req.Role != models.CertificateRoleRelay {
			v.addFieldError("role", "role must be client or relay")
		} else if usageValid && effectiveUsage != models.CertificateUsageDownstream {
			v.addFieldError("role", "role applies only to usage: downstream certificates")
		} else if usageValid {
			effectiveRole = req.Role
		}
	}

	if req.Match != nil {
		if effectiveRole != models.CertificateRoleRelay {
			v.addFieldError("match", "match applies only to role: relay entries")
		} else {
			v.validateMatchLists(req.Match)
		}
	}

	var bundle *clientca.Bundle
	if usageValid {
		switch {
		case effectiveUsage == models.CertificateUsageDownstream && certProvided:
			b, err := clientca.Inspect([]byte(req.Certificate), time.Now())
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
		case certProvided && pemCarriesPrivateKey([]byte(req.Certificate)):
			// clientca.Inspect refuses a key for usage: downstream with its own message.
			v.addFieldError("certificate", msgCertificateFieldCarriesKey)
		case effectiveUsage == models.CertificateUsageIdentity && certProvided && keyProvided:
			ib, err := gatewayidentity.Inspect([]byte(req.Certificate), []byte(req.PrivateKey), time.Now())
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
			if _, err := s.validateCertificate([]byte(req.Certificate)); err != nil {
				v.addFieldError("certificate", clientca.MsgNotPEMCertificate)
			}
		}
	}

	return v, effectiveUsage, effectiveRole, bundle
}

// publishClientAuthorities republishes the usage: downstream pool to the policy
// engine after a committed change.
func (s *APIServer) publishClientAuthorities(correlationID string) error {
	return s.clientAuthorities.Publish(correlationID)
}
