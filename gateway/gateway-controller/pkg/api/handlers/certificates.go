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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers/handlerkit"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// UploadCertificateRequest represents the request body for certificate upload
type UploadCertificateRequest struct {
	Certificate string                   `json:"certificate" binding:"required"` // PEM-encoded certificate
	Name        string                   `json:"name" binding:"required"`        // Unique certificate name
	Usage       string                   `json:"usage"`                          // "upstream" (default), "downstream" or "identity"
	Role        string                   `json:"role"`                           // "client" (default) or "relay" for usage: downstream; "default" for usage: identity
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

// certNotAfterLayout is the timestamp format certificate responses use.
const certNotAfterLayout = "2006-01-02 15:04:05"

// uploadResultToResponse renders a successful upload exactly as
// POST /certificates returns it. The REST handler and the MCP certificate tool
// both use it, so the two report an upload identically. CertificateResponse
// carries no PEM and no key material, so nothing stored reaches a client.
func uploadResultToResponse(result *certificate.UploadResult) CertificateResponse {
	cert := result.Certificate
	resp := CertificateResponse{
		ID:       cert.UUID,
		Name:     cert.Name,
		Subject:  cert.Subject,
		Issuer:   cert.Issuer,
		NotAfter: cert.NotAfter.Format(certNotAfterLayout),
		Count:    cert.CertCount,
		Usage:    result.Usage,
		IsLeaf:   result.IsLeaf,
		Warnings: result.Warnings,
		Message:  "Certificate uploaded and SDS updated successfully",
		Status:   "success",
	}
	if result.Usage == models.CertificateUsageDownstream {
		resp.Role = result.Role
		if result.Role == models.CertificateRoleRelay {
			resp.Match = cert.Match
		}
	}
	if result.Usage == models.CertificateUsageIdentity {
		resp.Role = result.Role
		resp.KeyAlgorithm = result.KeyAlgorithm
		resp.ChainLength = cert.CertCount
	}
	return resp
}

// listedCertificateToResponse renders one listed certificate exactly as
// GET /certificates returns it.
func listedCertificateToResponse(item certificate.ListedCertificate) CertificateResponse {
	cert := item.Certificate
	return CertificateResponse{
		ID:               cert.UUID,
		Name:             cert.Name,
		Subject:          cert.Subject,
		Issuer:           cert.Issuer,
		NotAfter:         cert.NotAfter.Format(certNotAfterLayout),
		Count:            cert.CertCount,
		Usage:            item.Usage,
		Role:             item.Role,
		Match:            item.Match,
		IsLeaf:           item.IsLeaf,
		KeyAlgorithm:     item.KeyAlgorithm,
		ChainLength:      item.ChainLength,
		Warnings:         item.Warnings,
		ReferencedByApis: item.ReferencedByApis,
		Status:           "success",
	}
}

// updateResultToResponse renders a successful identity rotation exactly as
// PUT /certificates/{id} returns it.
func updateResultToResponse(result *certificate.UpdateResult) CertificateResponse {
	updated := result.Certificate
	resp := CertificateResponse{
		ID:           updated.UUID,
		Name:         updated.Name,
		Subject:      updated.Subject,
		Issuer:       updated.Issuer,
		NotAfter:     updated.NotAfter.Format(certNotAfterLayout),
		Count:        updated.CertCount,
		Usage:        models.CertificateUsageIdentity,
		IsLeaf:       result.IsLeaf,
		KeyAlgorithm: result.KeyAlgorithm,
		ChainLength:  updated.CertCount,
		Warnings:     result.Warnings,
		Message:      "Certificate updated and SDS updated successfully",
		Status:       "success",
	}
	if result.IsDefault {
		resp.Role = models.CertificateRoleDefault
	}
	return resp
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

	result, err := s.getCertificateService().Upload(certificate.UploadParams{
		Name:          req.Name,
		Certificate:   req.Certificate,
		Usage:         req.Usage,
		Role:          req.Role,
		Match:         req.Match,
		PrivateKey:    req.PrivateKey,
		FieldErrors:   shapeFieldErrors(shapeErrors),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		var validationErr *certificate.ValidationError
		var defaultErr *certificate.DefaultIdentityExistsError
		var encryptErr *certificate.EncryptionError
		var syncErr *certificate.SyncError
		switch {
		case errors.As(err, &validationErr):
			writeCertificateValidationError(w, validationErr)
		case errors.Is(err, certificate.ErrIdentityStorageUnavailable):
			writeCertificateMessage(w, http.StatusInternalServerError, "gateway identity storage is not available")
		case errors.As(err, &encryptErr):
			writeCertificateMessage(w, http.StatusInternalServerError, "failed to store gateway identity")
		case errors.As(err, &defaultErr):
			writeCertificateMessage(w, http.StatusConflict, defaultErr.Error())
		case errors.Is(err, certificate.ErrIDGeneration):
			writeCertificateMessage(w, http.StatusInternalServerError, "Failed to generate certificate ID")
		case storage.IsConflictError(err):
			// Names are one namespace across usages, so the existing entry may have
			// any usage; name only the clash.
			writeCertificateMessage(w, http.StatusConflict, fmt.Sprintf("a certificate named %s already exists", req.Name))
		case errors.Is(err, certificate.ErrCertStoreNotConfigured):
			writeCertificateMessage(w, http.StatusInternalServerError, "Certificate store not configured")
		case errors.As(err, &syncErr):
			writeCertificateMessage(w, http.StatusInternalServerError, certificateSyncFailureMessage("upload", syncErr.Stage))
		default:
			writeCertificateMessage(w, http.StatusInternalServerError, "Failed to save certificate")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, uploadResultToResponse(result))
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

	result, err := s.getCertificateService().List(certificate.ListParams{Usage: usageFilter, Logger: log})
	if err != nil {
		var validationErr *certificate.ValidationError
		if errors.As(err, &validationErr) {
			writeCertificateValidationError(w, validationErr)
			return
		}
		writeCertificateMessage(w, http.StatusInternalServerError, "Failed to list certificates")
		return
	}

	certificates := []CertificateResponse{}
	for _, item := range result.Items {
		certificates = append(certificates, listedCertificateToResponse(item))
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

	if _, err := s.getCertificateService().Delete(certificate.DeleteParams{
		ID:            id,
		CorrelationID: correlationID,
		Logger:        log,
	}); err != nil {
		var persistErr *certificate.PersistError
		var syncErr *certificate.SyncError
		switch {
		case errors.Is(err, certificate.ErrMissingID):
			writeCertificateMessage(w, http.StatusBadRequest, "Certificate ID is required")
		case errors.Is(err, certificate.ErrCertStoreNotConfigured):
			writeCertificateMessage(w, http.StatusInternalServerError, "Certificate store not configured")
		case errors.As(err, &persistErr) && persistErr.Op == certificate.OpLoad:
			writeCertificateMessage(w, http.StatusInternalServerError, "the certificate could not be read")
		case errors.As(err, &persistErr) && persistErr.Op == certificate.OpDelete:
			writeCertificateMessage(w, http.StatusNotFound, "Certificate not found or failed to delete: "+persistErr.Cause.Error())
		case errors.As(err, &syncErr):
			writeCertificateMessage(w, http.StatusInternalServerError, certificateSyncFailureMessage("delete", syncErr.Stage))
		default:
			if resp, status := certificateRefusal(err); status != 0 {
				httputil.WriteJSON(w, status, resp)
				return
			}
			writeCertificateMessage(w, http.StatusInternalServerError, "Failed to delete certificate")
		}
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
		var syncErr *certificate.SyncError
		switch {
		case errors.Is(err, certificate.ErrCertStoreNotConfigured):
			writeCertificateMessage(w, http.StatusInternalServerError, "Certificate store not configured")
		case errors.As(err, &syncErr):
			writeCertificateMessage(w, http.StatusInternalServerError, certificateSyncFailureMessage("reload", syncErr.Stage))
		default:
			writeCertificateMessage(w, http.StatusInternalServerError, "Failed to reload certificates")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"message":    "Certificates reloaded and SDS updated successfully",
		"totalBytes": result.TotalBytes,
	})
}

// UpdateCertificate rotates a usage: identity certificate's chain and private
// key in place, keeping its name. Other usages are refused.
// PUT /certificates/{id}
func (s *APIServer) UpdateCertificate(w http.ResponseWriter, r *http.Request, id string) {
	correlationID := middleware.GetCorrelationID(r)
	log := s.logger.With(slog.String("correlation_id", correlationID))

	svc := s.getCertificateService()

	// The certificate is checked before the body is read, so a missing or
	// non-identity certificate is reported whatever the body holds.
	existing, err := svc.PrepareUpdate(id, log)
	if err != nil {
		writeCertificateUpdateError(w, err)
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

	result, err := svc.Update(certificate.UpdateParams{
		ID:            id,
		Existing:      existing,
		Certificate:   req.Certificate,
		PrivateKey:    req.PrivateKey,
		FieldErrors:   shapeFieldErrors(shapeErrors),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		writeCertificateUpdateError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, updateResultToResponse(result))
}

// writeCertificateUpdateError maps a PrepareUpdate or Update failure to the
// response UpdateCertificate returns.
func writeCertificateUpdateError(w http.ResponseWriter, err error) {
	var validationErr *certificate.ValidationError
	var encryptErr *certificate.EncryptionError
	var persistErr *certificate.PersistError
	var syncErr *certificate.SyncError
	switch {
	case errors.Is(err, certificate.ErrNotFound):
		writeCertificateMessage(w, http.StatusNotFound, "certificate not found")
	case errors.As(err, &persistErr) && persistErr.Op == certificate.OpLoad:
		writeCertificateMessage(w, http.StatusInternalServerError, "the certificate could not be read")
	case errors.As(err, &validationErr):
		writeCertificateValidationError(w, validationErr)
	case errors.Is(err, certificate.ErrIdentityStorageUnavailable):
		writeCertificateMessage(w, http.StatusInternalServerError, "gateway identity storage is not available")
	case errors.As(err, &encryptErr):
		writeCertificateMessage(w, http.StatusInternalServerError, "failed to store gateway identity")
	case errors.As(err, &syncErr):
		writeCertificateMessage(w, http.StatusInternalServerError, certificateSyncFailureMessage("update", syncErr.Stage))
	default:
		writeCertificateMessage(w, http.StatusInternalServerError, "failed to update certificate")
	}
}

// writeCertificateValidationError renders a certificate ValidationError as the
// 400 the certificate endpoints return.
func writeCertificateValidationError(w http.ResponseWriter, ve *certificate.ValidationError) {
	fieldErrors := toAPIValidationErrors(ve.Fields)
	httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
		Status:  "error",
		Message: ve.Message,
		Errors:  &fieldErrors,
	})
}

func writeCertificateMessage(w http.ResponseWriter, status int, message string) {
	httputil.WriteJSON(w, status, map[string]any{
		"status":  "error",
		"message": message,
	})
}

// toAPIValidationErrors converts service field errors to the wire type.
func toAPIValidationErrors(fields []certificate.FieldError) []api.ValidationError {
	out := make([]api.ValidationError, len(fields))
	for i, f := range fields {
		out[i] = api.ValidationError{Field: stringPtr(f.Field), Message: stringPtr(f.Message)}
	}
	return out
}

// shapeFieldErrors converts decodeCertificateUpload's shape errors to service
// field errors.
func shapeFieldErrors(shape []uploadShapeError) []certificate.FieldError {
	out := make([]certificate.FieldError, len(shape))
	for i, fe := range shape {
		out[i] = certificate.FieldError{Field: fe.field, Message: fe.message}
	}
	return out
}

// certificateRefusal renders a refused delete: an InUseError as a 409 naming
// each reference, a ReferenceCheckError as a 500. Any other error yields a
// zero status.
func certificateRefusal(err error) (api.ErrorResponse, int) {
	var inUse *certificate.InUseError
	if errors.As(err, &inUse) {
		errs := make([]api.ValidationError, len(inUse.References))
		for i, ref := range inUse.References {
			errs[i] = api.ValidationError{
				Field:   stringPtr(ref.Field),
				Message: stringPtr(fmt.Sprintf("referenced by API '%s'", ref.API)),
			}
		}
		return api.ErrorResponse{Status: "error", Message: inUse.Message, Errors: &errs}, http.StatusConflict
	}
	var check *certificate.ReferenceCheckError
	if errors.As(err, &check) {
		return api.ErrorResponse{Status: "error", Message: check.Message}, http.StatusInternalServerError
	}
	return api.ErrorResponse{}, 0
}

// certificateSyncFailures holds the message each certificate operation reports
// when a stage of propagating its change fails.
var certificateSyncFailures = map[string]map[certificate.SyncStage]string{
	"upload": {
		certificate.StageReload:       "Certificate saved but failed to reload",
		certificate.StageSnapshot:     "Certificate reloaded but failed to update SDS",
		certificate.StagePolicyEngine: "Certificate saved but failed to update the policy engine",
	},
	"delete": {
		certificate.StageReload:       "Certificate deleted but failed to reload",
		certificate.StageSnapshot:     "Certificate deleted and reloaded but failed to update SDS",
		certificate.StagePolicyEngine: "Certificate deleted but failed to update the policy engine",
	},
	"reload": {
		certificate.StageReload:       "Failed to reload certificates",
		certificate.StageSnapshot:     "Certificates reloaded but failed to update SDS",
		certificate.StagePolicyEngine: "Certificates reloaded but failed to update the policy engine",
	},
	"update": {
		certificate.StageSnapshot: "certificate updated but failed to update SDS",
	},
}

func certificateSyncFailureMessage(op string, stage certificate.SyncStage) string {
	return certificateSyncFailures[op][stage]
}

// extractCertificateMetadata extracts metadata from the first certificate in the chain
func (s *APIServer) extractCertificateMetadata(data []byte) (subject, issuer string, notBefore, notAfter time.Time, err error) {
	return certificate.ExtractMetadata(data)
}

func (s *APIServer) validateCertificate(data []byte) (int, error) {
	return certificate.ValidateChain(data)
}

// checkClientAuthorityDeletable reports whether cert, a non-relay usage:
// downstream authority, can be removed. It refuses when a deployed API names the
// authority, or when it is the last non-relay authority and any deployed API
// attaches mtls-auth. A failed read refuses with a 500. When refused it
// returns the error body, the status and true.
func (s *APIServer) checkClientAuthorityDeletable(cert *models.StoredCertificate) (api.ErrorResponse, int, bool) {
	err := s.getCertificateService().CheckClientAuthorityDeletable(cert)
	if err == nil {
		return api.ErrorResponse{}, 0, false
	}
	resp, status := certificateRefusal(err)
	return resp, status, true
}

// publishCertificateEvent tells every replica sharing the event hub that the
// certificate rows changed, so each rebuilds its certificate store, client
// authority pool and xDS snapshot from the database. It runs once the write
// has committed; the event carries the row id, never certificate material.
func (s *APIServer) publishCertificateEvent(action, certID, correlationID string, log *slog.Logger) {
	(&handlerkit.EventPublisher{EventHub: s.eventHub, GatewayID: s.gatewayID}).
		PublishEvent(eventhub.EventTypeCertificate, action, certID, correlationID, log)
}

// publishClientAuthorities republishes the usage: downstream pool to the policy
// engine after a committed change.
func (s *APIServer) publishClientAuthorities(correlationID string) error {
	return s.clientAuthorities.Publish(correlationID)
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
const certificateUploadInvalidMessage = certificate.MsgUploadInvalid
