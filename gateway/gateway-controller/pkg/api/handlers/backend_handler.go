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

package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/backend"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// maxBackendRequestBodyBytes bounds a Backend definition read from a request, as the
// subscription handlers bound theirs. A Backend is a short document; 256 KiB leaves ample room
// for many endpoints and labels.
const maxBackendRequestBodyBytes = 256 << 10

// readBackendBody reads a bounded Backend definition. It writes 413 for an oversized body and
// 400 for any other unreadable body, and reports whether the caller may continue.
func readBackendBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBackendRequestBodyBytes))
	if err == nil {
		return body, true
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeBackendError(w, http.StatusRequestEntityTooLarge, "Request body too large", nil)
		return nil, false
	}
	writeBackendError(w, http.StatusBadRequest, "Failed to read request body", nil)
	return nil, false
}

// CreateBackend implements ServerInterface.CreateBackend (POST /backends).
func (s *APIServer) CreateBackend(w http.ResponseWriter, r *http.Request) {
	if !s.backendsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)
	body, ok := readBackendBody(w, r)
	if !ok {
		return
	}
	stored, err := s.backendService.Create(backend.Request{
		Body:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Origin:        models.OriginGatewayAPI,
	})
	if err != nil {
		s.writeBackendServiceError(w, log, "create", "", err)
		return
	}
	log.Info("Backend created", slog.String("name", stored.GetHandle()), slog.String("uuid", stored.UUID))
	httputil.WriteJSON(w, http.StatusCreated, s.backendResponseWithWarnings(log, stored))
}

// ListBackends implements ServerInterface.ListBackends (GET /backends).
func (s *APIServer) ListBackends(w http.ResponseWriter, r *http.Request) {
	if !s.backendsAvailable(w) {
		return
	}
	backends, err := s.backendService.List()
	if err != nil {
		middleware.GetLogger(r, s.logger).Error("Failed to list backends", slog.Any("error", err))
		writeBackendError(w, http.StatusInternalServerError, "Failed to list backends", nil)
		return
	}
	items := make([]api.Backend, 0, len(backends))
	for _, e := range backends {
		items = append(items, buildBackendResponse(e))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"count":    len(items),
		"backends": items,
	})
}

// GetBackendById implements ServerInterface.GetBackendById (GET /backends/{id}).
func (s *APIServer) GetBackendById(w http.ResponseWriter, r *http.Request, id string) {
	if !s.backendsAvailable(w) {
		return
	}
	stored, err := s.backendService.Get(id)
	if err != nil {
		s.writeBackendServiceError(w, middleware.GetLogger(r, s.logger), "get", id, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, buildBackendResponse(stored))
}

// UpdateBackend implements ServerInterface.UpdateBackend (PUT /backends/{id}).
func (s *APIServer) UpdateBackend(w http.ResponseWriter, r *http.Request, id string) {
	if !s.backendsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)
	body, ok := readBackendBody(w, r)
	if !ok {
		return
	}
	stored, err := s.backendService.Update(id, backend.Request{
		Body:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
	})
	if err != nil {
		s.writeBackendServiceError(w, log, "update", id, err)
		return
	}
	log.Info("Backend updated", slog.String("name", stored.GetHandle()), slog.String("uuid", stored.UUID))
	httputil.WriteJSON(w, http.StatusOK, s.backendResponseWithWarnings(log, stored))
}

// DeleteBackend implements ServerInterface.DeleteBackend (DELETE /backends/{id}).
func (s *APIServer) DeleteBackend(w http.ResponseWriter, r *http.Request, id string) {
	if !s.backendsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)
	deleted, err := s.backendService.Delete(id, middleware.GetCorrelationID(r))
	if err != nil {
		s.writeBackendServiceError(w, log, "delete", id, err)
		return
	}
	log.Info("Backend deleted", slog.String("name", deleted.GetHandle()), slog.String("uuid", deleted.UUID))
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Backend deleted successfully",
		"id":      deleted.GetHandle(),
	})
}

func (s *APIServer) backendsAvailable(w http.ResponseWriter) bool {
	if s.backendService != nil {
		return true
	}
	writeBackendError(w, http.StatusServiceUnavailable, "Global backends are not available on this gateway", nil)
	return false
}

// writeBackendServiceError maps a Backend service error to its HTTP response.
func (s *APIServer) writeBackendServiceError(w http.ResponseWriter, log *slog.Logger, op, name string, err error) {
	var invalid *backend.ValidationError
	var inUse *backend.InUseError
	switch {
	case errors.As(err, &invalid):
		log.Warn("Backend configuration invalid", slog.String("operation", op), slog.Any("error", err))
		errs := make([]api.ValidationError, 0, len(invalid.Errors))
		for _, v := range invalid.Errors {
			errs = append(errs, api.ValidationError{Field: stringPtr(v.Field), Message: stringPtr(v.Message)})
		}
		writeBackendError(w, http.StatusBadRequest, "Backend configuration validation failed", &errs)
	case errors.As(err, &inUse):
		errs := make([]api.ValidationError, 0, len(inUse.References))
		for _, ref := range inUse.References {
			errs = append(errs, api.ValidationError{
				Field:   stringPtr(ref.FieldPath),
				Message: stringPtr(fmt.Sprintf("referenced by API '%s'", ref.APIHandle)),
			})
		}
		writeBackendError(w, http.StatusConflict, inUse.Error(), &errs)
	case errors.Is(err, backend.ErrNotFound):
		writeBackendError(w, http.StatusNotFound, fmt.Sprintf("Backend '%s' not found", name), nil)
	case storage.IsConflictError(err):
		writeBackendError(w, http.StatusConflict, err.Error(), nil)
	default:
		log.Error("Backend operation failed", slog.String("operation", op), slog.String("name", name), slog.Any("error", err))
		writeBackendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to %s backend", op), nil)
	}
}

func writeBackendError(w http.ResponseWriter, status int, message string, errs *[]api.ValidationError) {
	httputil.WriteJSON(w, status, api.ErrorResponse{Status: "error", Message: message, Errors: errs})
}

// buildBackendResponse renders a stored Backend as the resource body, with its server-managed
// status.
func buildBackendResponse(stored *models.StoredBackend) api.Backend {
	id := stored.GetHandle()
	createdAt, updatedAt := stored.CreatedAt, stored.UpdatedAt
	state := api.ResourceStatusStateDeployed
	cfg := stored.Configuration
	return api.Backend{
		ApiVersion: api.BackendApiVersion(cfg.ApiVersion),
		Kind:       api.BackendKind(cfg.Kind),
		Metadata:   cfg.Metadata,
		Spec:       cfg.Spec,
		Status: &api.ResourceStatus{
			Id:        &id,
			State:     &state,
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}
}

// backendResponseWithWarnings renders a created or updated Backend with its warnings in
// status.warnings, and logs each warning so the operator sees what the caller was told.
func (s *APIServer) backendResponseWithWarnings(log *slog.Logger, stored *models.StoredBackend) api.Backend {
	resp := buildBackendResponse(stored)
	warnings := s.backendService.Warnings(stored)
	if len(warnings) == 0 {
		return resp
	}
	apiWarnings := make([]api.Warning, len(warnings))
	for i, w := range warnings {
		code := api.WarningCode(w.Code)
		apiWarnings[i] = api.Warning{Code: &code, Field: stringPtr(w.Field), Message: stringPtr(w.Message)}
		log.Warn("Backend warning", slog.String("backend", stored.GetHandle()),
			slog.String("code", w.Code), slog.String("field", w.Field), slog.String("message", w.Message))
	}
	resp.Status.Warnings = &apiWarnings
	return resp
}

// checkBackendCertificateReferences refuses a certificate delete while a global Backend names
// the certificate in its tls block: as its identity when identity is true, else in trustedCAs.
func (s *APIServer) checkBackendCertificateReferences(certName string, identity bool) (api.ErrorResponse, int, bool) {
	if s.backendService == nil {
		return api.ErrorResponse{}, 0, false
	}
	backends, err := s.backendService.List()
	if err != nil {
		s.logger.Error("Failed to read global backends for certificate referential-integrity check",
			slog.String("certificate", certName), slog.Any("error", err))
		return api.ErrorResponse{Status: "error", Message: "Failed to verify certificate references"},
			http.StatusInternalServerError, true
	}
	var errs []api.ValidationError
	var names []string
	for _, e := range backends {
		defs := []api.UpstreamDefinition{e.UpstreamDefinition()}
		var paths []string
		if identity {
			paths = config.NamedTLSIdentityFieldPaths(&defs, certName)
		} else {
			paths = config.NamedTLSTrustedCAFieldPaths(&defs, certName)
		}
		if len(paths) == 0 {
			continue
		}
		names = append(names, e.GetHandle())
		errs = append(errs, api.ValidationError{
			Field:   stringPtr(strings.Replace(paths[0], "spec.upstreamDefinitions[0]", "spec", 1)),
			Message: stringPtr(fmt.Sprintf("referenced by global backend '%s'", e.GetHandle())),
		})
	}
	if len(errs) == 0 {
		return api.ErrorResponse{}, 0, false
	}
	message := fmt.Sprintf("certificate '%s' is named by global backends: %s; remove those references first",
		certName, strings.Join(names, ", "))
	return api.ErrorResponse{Status: "error", Message: message, Errors: &errs}, http.StatusConflict, true
}
