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
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2/go-httpkit/httputil"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// CreateSecret handles POST /secrets
func (s *APIServer) CreateSecret(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	// Enforce body size before reading to prevent memory exhaustion.
	// Limit is the secret value cap plus 1 KB for JSON field name overhead.
	r.Body = http.MaxBytesReader(w, r.Body, secrets.MaxSecretSize+1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Request body too large or unreadable",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(r)

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delegate to service which parses/validates/encrypt and persists
	secret, err := s.secretService.CreateSecret(secrets.SecretParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to encrypt Secret", slog.Any("error", err))
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	log.Info("Secret created successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Echo back the created secret in the k8s-shaped resource form. The plaintext
	// value is omitted so response logs and clients don't surface secret material.
	httputil.WriteJSON(w, http.StatusCreated, buildSecretResourceResponse(secret, false))
}

// ListSecrets implements ServerInterface.ListSecrets
// (GET /secrets)
func (s *APIServer) ListSecrets(w http.ResponseWriter, r *http.Request) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(r)

	log.Debug("Retrieving secretsList", slog.String("correlation_id", correlationID))

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	secretsMeta, err := s.secretService.GetSecrets(correlationID)
	if err != nil {
		log.Error("Failed to retrieve secretsList",
			slog.String("correlation_id", correlationID),
			slog.Any("error", err),
		)
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve secretsList",
		})
		return
	}

	items := make([]any, 0, len(secretsMeta))
	for _, meta := range secretsMeta {
		items = append(items, buildSecretMetaResourceResponse(meta))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"count":   len(items),
		"secrets": items,
	})
}

// GetSecret handles GET /secrets/{id}
func (s *APIServer) GetSecret(w http.ResponseWriter, r *http.Request, id string) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(r)

	log.Debug("Retrieving secret",
		slog.String("secret_handle", id),
		slog.String("correlation_id", correlationID))

	// Validate secret ID format
	if id == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Retrieve secret
	secret, err := s.secretService.Get(id, correlationID)
	if err != nil {
		// Check for not found error
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		// Generic error for decryption failures (security-first)
		log.Error("Failed to retrieve secret",
			slog.String("secret_handle", id),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to decrypt secret",
		})
		return
	}

	log.Debug("Secret retrieved successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	// Include the decrypted value on single-item GET (caller supplied id). The
	// value is omitted from list views but exposed here so automation can read
	// back a secret it just created/updated.
	httputil.WriteJSON(w, http.StatusOK, buildSecretResourceResponse(secret, true))
}

// UpdateSecret handles PUT /secrets/{id}
func (s *APIServer) UpdateSecret(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	// Enforce body size before reading to prevent memory exhaustion.
	r.Body = http.MaxBytesReader(w, r.Body, secrets.MaxSecretSize+1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Request body too large or unreadable",
		})
		return
	}

	// Get correlation ID from context
	correlationID := middleware.GetCorrelationID(r)

	// Validate secret ID format
	if id == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delegate to service which parses/validates/encrypt and persists
	secret, err := s.secretService.UpdateSecret(id, secrets.SecretParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to encrypt Secret", slog.Any("error", err))
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	log.Info("Secret updated successfully",
		slog.String("secret_handle", secret.Handle),
		slog.String("correlation_id", correlationID))

	httputil.WriteJSON(w, http.StatusOK, buildSecretResourceResponse(secret, false))
}

// DeleteSecret handles DELETE /secrets/{id}
func (s *APIServer) DeleteSecret(w http.ResponseWriter, r *http.Request, id string) {
	log := s.logger
	correlationID := middleware.GetCorrelationID(r)

	log.Debug("Deleting secret",
		slog.String("secret_id", id),
		slog.String("correlation_id", correlationID))

	// Validate secret ID format
	if id == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Missing required field: id",
		})
		return
	}
	if len(id) > 255 {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Secret ID too long (max 255 characters)",
		})
		return
	}

	// Avoid secretService nil panic
	if s.secretService == nil {
		log.Error("Secret service is not initialized properly")
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error",
			Message: "Secret service is not initialized properly"})
		return
	}

	// Delete secret
	if err := s.secretService.Delete(id, correlationID); err != nil {
		// Check for not found error
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		// Generic error for storage failures
		log.Error("Failed to delete secret",
			slog.String("secret_id", id),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete secret",
		})
		return
	}

	log.Info("Secret deleted successfully",
		slog.String("secret_id", id),
		slog.String("correlation_id", correlationID))

	// Return 200 OK on successful deletion
	w.WriteHeader(http.StatusOK)
}
