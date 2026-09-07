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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/httpkit/httputil"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
)

// UploadCertificateRequest represents the request body for certificate upload
type UploadCertificateRequest struct {
	Certificate string `json:"certificate" binding:"required"` // PEM-encoded certificate
	Name        string `json:"name" binding:"required"`        // Unique certificate name
}

// CertificateResponse represents a certificate information response
type CertificateResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subject  string `json:"subject,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	NotAfter string `json:"notAfter,omitempty"`
	Count    int    `json:"count"` // Number of certs in file
	Message  string `json:"message,omitempty"`
	Status   string `json:"status"` // success, error
}

// ListCertificatesResponse represents the response for listing certificates
type ListCertificatesResponse struct {
	Certificates []CertificateResponse `json:"certificates"`
	TotalCount   int                   `json:"totalCount"`
	TotalBytes   int                   `json:"totalBytes"`
	Status       string                `json:"status"`
}

// certNotAfterLayout is the timestamp format this endpoint has always emitted.
// It is not RFC 3339, which is why these responses use hand-written structs
// rather than the generated certificate types.
const certNotAfterLayout = "2006-01-02 15:04:05"

// certSyncMessages maps a failed sync stage to the message each operation
// reported before the service layer existed. Preserved verbatim: the string is
// what tells an operator how far the write actually got.
var certSyncMessages = map[string]map[certificate.SyncStage]string{
	"upload": {
		certificate.StageReload:   "Certificate saved but failed to reload",
		certificate.StageSnapshot: "Certificate reloaded but failed to update SDS",
	},
	"delete": {
		certificate.StageReload:   "Certificate deleted but failed to reload",
		certificate.StageSnapshot: "Certificate deleted and reloaded but failed to update SDS",
	},
	"reload": {
		certificate.StageReload:   "Failed to reload certificates",
		certificate.StageSnapshot: "Certificates reloaded but failed to update SDS",
	},
}

// UploadCertificate handles certificate upload via REST API
// POST /certificates
func (s *APIServer) UploadCertificate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	var req UploadCertificateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Invalid certificate upload request", slog.Any("error", err))
		writeCertError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	result, err := s.getCertificateService().Upload(certificate.UploadParams{
		Name:           req.Name,
		CertificatePEM: []byte(req.Certificate),
		CorrelationID:  correlationID,
		Logger:         log,
	})
	if err != nil {
		mapUploadCertError(w, log, err)
		return
	}

	cert := result.Certificate
	httputil.WriteJSON(w, http.StatusCreated, CertificateResponse{
		ID:       cert.UUID,
		Name:     cert.Name,
		Subject:  cert.Subject,
		Issuer:   cert.Issuer,
		NotAfter: cert.NotAfter.Format(certNotAfterLayout),
		Count:    cert.CertCount,
		Message:  "Certificate uploaded and SDS updated successfully",
		Status:   "success",
	})
}

// ListCertificates lists all custom certificates
// GET /certificates
func (s *APIServer) ListCertificates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	result, err := s.getCertificateService().List()
	if err != nil {
		log.Error("Failed to list certificates from database", slog.Any("error", err))
		writeCertError(w, http.StatusInternalServerError, "Failed to list certificates")
		return
	}

	var certificates []CertificateResponse
	for _, cert := range result.Certificates {
		certificates = append(certificates, CertificateResponse{
			ID:       cert.UUID,
			Name:     cert.Name,
			Subject:  cert.Subject,
			Issuer:   cert.Issuer,
			NotAfter: cert.NotAfter.Format(certNotAfterLayout),
			Count:    cert.CertCount,
			Status:   "success",
		})
	}

	httputil.WriteJSON(w, http.StatusOK, ListCertificatesResponse{
		Certificates: certificates,
		TotalCount:   len(certificates),
		TotalBytes:   result.TotalBytes,
		Status:       "success",
	})
}

// DeleteCertificate deletes a certificate by ID
// DELETE /certificates/:id
func (s *APIServer) DeleteCertificate(w http.ResponseWriter, r *http.Request, id string) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	if id == "" {
		writeCertError(w, http.StatusBadRequest, "Certificate ID is required")
		return
	}

	if _, err := s.getCertificateService().Delete(certificate.DeleteParams{
		ID:            id,
		CorrelationID: correlationID,
		Logger:        log,
	}); err != nil {
		mapDeleteCertError(w, log, id, err)
		return
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

	result, err := s.getCertificateService().Reload(certificate.ReloadParams{
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		mapReloadCertError(w, log, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"message":    "Certificates reloaded and SDS updated successfully",
		"totalBytes": result.TotalBytes,
	})
}

// mapUploadCertError reproduces the responses POST /certificates returned before
// the service layer existed.
func mapUploadCertError(w http.ResponseWriter, log *slog.Logger, err error) {
	if errors.Is(err, certificate.ErrMissingFields) {
		writeCertError(w, http.StatusBadRequest, "name and certificate are required fields")
		return
	}

	// The message is built here rather than passed through: the service's error
	// string is lower-case, and the integration suite asserts on the capitalised
	// "Invalid certificate" prefix this endpoint has always sent.
	var invalidCert *certificate.InvalidCertificateError
	if errors.As(err, &invalidCert) {
		log.Warn("Invalid certificate provided", slog.Any("error", err))
		writeCertError(w, http.StatusBadRequest, "Invalid certificate: "+invalidCert.Cause.Error())
		return
	}

	var metadataErr *certificate.MetadataError
	if errors.As(err, &metadataErr) {
		log.Warn("Failed to extract certificate metadata", slog.Any("error", err))
		writeCertError(w, http.StatusBadRequest, "Failed to parse certificate metadata: "+metadataErr.Cause.Error())
		return
	}

	if errors.Is(err, certificate.ErrIDGeneration) {
		log.Error("Failed to generate certificate ID", slog.Any("error", err))
		writeCertError(w, http.StatusInternalServerError, "Failed to generate certificate ID")
		return
	}

	// Reached only after the row has already been written: the upload path saves
	// first and discovers a missing cert store second, as it always did.
	if errors.Is(err, certificate.ErrCertStoreNotConfigured) {
		log.Error("Certificate store not available")
		writeCertError(w, http.StatusInternalServerError, "Certificate store not configured")
		return
	}

	if writeCertSyncError(w, "upload", err) {
		return
	}

	var persistErr *certificate.PersistError
	if errors.As(err, &persistErr) && persistErr.Op == certificate.OpSave {
		log.Error("Failed to save certificate to database", slog.Any("error", err))
		writeCertError(w, http.StatusInternalServerError, "Failed to save certificate")
		return
	}

	log.Error("Certificate upload failed", slog.Any("error", err))
	writeCertError(w, http.StatusInternalServerError, "Failed to save certificate")
}

// mapDeleteCertError reproduces the responses DELETE /certificates/{id}
// returned before the service layer existed. Note the deliberate blanket 404:
// any storage failure on delete is reported as "not found", which is what the
// integration suite asserts for an unknown ID.
func mapDeleteCertError(w http.ResponseWriter, log *slog.Logger, id string, err error) {
	if writeCertStoreUnavailable(w, log, err) {
		return
	}
	if writeCertSyncError(w, "delete", err) {
		return
	}

	var persistErr *certificate.PersistError
	if errors.As(err, &persistErr) && persistErr.Op == certificate.OpDelete {
		log.Error("Failed to delete certificate",
			slog.String("id", id),
			slog.Any("error", err))
		writeCertError(w, http.StatusNotFound, "Certificate not found or failed to delete: "+persistErr.Cause.Error())
		return
	}

	log.Error("Certificate deletion failed", slog.String("id", id), slog.Any("error", err))
	writeCertError(w, http.StatusNotFound, "Certificate not found or failed to delete")
}

// mapReloadCertError reproduces the responses POST /certificates/reload returned
// before the service layer existed.
func mapReloadCertError(w http.ResponseWriter, log *slog.Logger, err error) {
	if writeCertStoreUnavailable(w, log, err) {
		return
	}
	if writeCertSyncError(w, "reload", err) {
		return
	}

	log.Error("Certificate reload failed", slog.Any("error", err))
	writeCertError(w, http.StatusInternalServerError, "Failed to reload certificates")
}

// writeCertSyncError renders a *SyncError using the message the named operation
// has always reported for that stage. Reports whether it handled err.
func writeCertSyncError(w http.ResponseWriter, operation string, err error) bool {
	var syncErr *certificate.SyncError
	if !errors.As(err, &syncErr) {
		return false
	}

	message, ok := certSyncMessages[operation][syncErr.Stage]
	if !ok {
		message = "Failed to update certificate store"
	}
	writeCertError(w, http.StatusInternalServerError, message)

	return true
}

// writeCertStoreUnavailable renders the response every certificate operation
// gave when this gateway has no custom cert store. Reports whether it handled err.
func writeCertStoreUnavailable(w http.ResponseWriter, log *slog.Logger, err error) bool {
	if !errors.Is(err, certificate.ErrCertStoreNotConfigured) {
		return false
	}

	log.Error("Certificate store not available")
	writeCertError(w, http.StatusInternalServerError, "Certificate store not configured")

	return true
}

// writeCertError writes the untyped error body these endpoints have always used.
// The certificate routes predate the generated api.ErrorResponse and the
// integration suite matches on this exact shape.
func writeCertError(w http.ResponseWriter, status int, message string) {
	httputil.WriteJSON(w, status, map[string]any{
		"status":  "error",
		"message": message,
	})
}
