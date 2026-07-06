/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	egservice "github.com/wso2/api-platform/platform-api/plugins/eventgateway/service"

	"github.com/wso2/go-httpkit/httputil"
)

// WebSubAPIHmacSecretHandler handles HMAC secret CRUD for WebSub APIs.
type WebSubAPIHmacSecretHandler struct {
	secretService *egservice.WebSubAPIHmacSecretService
	identity      *service.IdentityService
	slogger       *slog.Logger
}

// NewWebSubAPIHmacSecretHandler creates a new WebSubAPIHmacSecretHandler.
func NewWebSubAPIHmacSecretHandler(secretService *egservice.WebSubAPIHmacSecretService, identity *service.IdentityService, slogger *slog.Logger) *WebSubAPIHmacSecretHandler {
	return &WebSubAPIHmacSecretHandler{
		secretService: secretService,
		identity:      identity,
		slogger:       slogger,
	}
}

// RegisterRoutes registers the HMAC secret routes.
func (h *WebSubAPIHmacSecretHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/websub-apis/{webSubApiId}/secrets", h.CreateHmacSecret)
	mux.HandleFunc("GET /api/v1/websub-apis/{webSubApiId}/secrets", h.ListHmacSecrets)
	mux.HandleFunc("DELETE /api/v1/websub-apis/{webSubApiId}/secrets/{secretName}", h.DeleteHmacSecret)
	mux.HandleFunc("POST /api/v1/websub-apis/{webSubApiId}/secrets/{secretName}/regenerate", h.RegenerateHmacSecret)
}

func (h *WebSubAPIHmacSecretHandler) featureUnavailable(w http.ResponseWriter) bool {
	if h.secretService == nil {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
			"HMAC secret management is not configured on this server"))
		return true
	}
	return false
}

// CreateHmacSecret handles POST /api/v1/websub-apis/:apiId/secrets
func (h *WebSubAPIHmacSecretHandler) CreateHmacSecret(w http.ResponseWriter, r *http.Request) {
	if h.featureUnavailable(w) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webSubApiId")
	if apiHandle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	var req api.WebSubAPIHmacSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Secret != nil && *req.Secret == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "secret must not be empty; omit the field to auto-generate"))
		return
	}

	var externalSecret string
	if req.Secret != nil {
		externalSecret = *req.Secret
	}

	userID, ok := resolveActor(w, r, h.identity, h.slogger, "generate WebSub HMAC secret")
	if !ok {
		return
	}
	secret, plaintext, err := h.secretService.Generate(orgID, apiHandle, req.DisplayName, externalSecret, userID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	msg := "HMAC secret generated successfully. Save the secret value — it will not be shown again."
	if externalSecret != "" {
		msg = "HMAC secret stored successfully."
	}
	info, err := h.toSecretInfo(secret)
	if err != nil {
		h.slogger.Error("Failed to resolve HMAC secret identity", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to generate HMAC secret"))
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, api.WebSubAPIHmacSecretCreationResponse{
		Secret:        plaintext,
		WebhookSecret: info,
		Message:       msg,
	})
}

// ListHmacSecrets handles GET /api/v1/websub-apis/:apiId/secrets
func (h *WebSubAPIHmacSecretHandler) ListHmacSecrets(w http.ResponseWriter, r *http.Request) {
	if h.featureUnavailable(w) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webSubApiId")
	if apiHandle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	secrets, err := h.secretService.List(orgID, apiHandle)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	items := make([]api.WebSubAPIHmacSecretInfo, 0, len(secrets))
	for _, s := range secrets {
		info, err := h.toSecretInfo(s)
		if err != nil {
			h.slogger.Error("Failed to resolve HMAC secret identity", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list HMAC secrets"))
			return
		}
		items = append(items, *info)
	}
	httputil.WriteJSON(w, http.StatusOK, api.WebSubAPIHmacSecretListResponse{Secrets: items})
}

// DeleteHmacSecret handles DELETE /api/v1/websub-apis/:apiId/secrets/:secretName
func (h *WebSubAPIHmacSecretHandler) DeleteHmacSecret(w http.ResponseWriter, r *http.Request) {
	if h.featureUnavailable(w) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webSubApiId")
	secretName := r.PathValue("secretName")
	if apiHandle == "" || secretName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle and secret name are required"))
		return
	}

	if err := h.secretService.Delete(orgID, apiHandle, secretName); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegenerateHmacSecret handles POST /api/v1/websub-apis/:apiId/secrets/:secretName/regenerate
func (h *WebSubAPIHmacSecretHandler) RegenerateHmacSecret(w http.ResponseWriter, r *http.Request) {
	if h.featureUnavailable(w) {
		return
	}
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webSubApiId")
	secretName := r.PathValue("secretName")
	if apiHandle == "" || secretName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle and secret name are required"))
		return
	}

	var req api.WebSubAPIHmacSecretRegenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.Secret != nil && *req.Secret == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "secret must not be empty; omit the field to auto-generate"))
		return
	}

	var externalSecret string
	if req.Secret != nil {
		externalSecret = *req.Secret
	}

	userID, ok := resolveActor(w, r, h.identity, h.slogger, "regenerate WebSub HMAC secret")
	if !ok {
		return
	}
	secret, plaintext, err := h.secretService.Regenerate(orgID, apiHandle, secretName, externalSecret, userID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	msg := "HMAC secret regenerated successfully. Save the new secret value — it will not be shown again."
	if externalSecret != "" {
		msg = "HMAC secret rotated to the provided value successfully."
	}
	info, err := h.toSecretInfo(secret)
	if err != nil {
		h.slogger.Error("Failed to resolve HMAC secret identity", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to regenerate HMAC secret"))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, api.WebSubAPIHmacSecretCreationResponse{
		Secret:        plaintext,
		WebhookSecret: info,
		Message:       msg,
	})
}

func (h *WebSubAPIHmacSecretHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, constants.ErrWebSubAPINotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebSub API not found"))
	case errors.Is(err, constants.ErrHmacSecretNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "HMAC secret not found"))
	case errors.Is(err, constants.ErrHmacSecretAlreadyExists):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "An HMAC secret with this name already exists"))
	case errors.Is(err, constants.ErrHmacSecretInvalidValue):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret value must be at least 32 characters"))
	case errors.Is(err, constants.ErrHmacSecretEncryptionKeyMissing):
		h.slogger.Error("HMAC secret encryption key is not configured")
		httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "HMAC secret management is not configured on this server"))
	default:
		h.slogger.Error("HMAC secret service error", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}

func secretToInfo(s *model.WebSubAPIHmacSecret) *api.WebSubAPIHmacSecretInfo {
	if s == nil {
		return nil
	}
	return &api.WebSubAPIHmacSecretInfo{
		Uuid:        s.UUID,
		Name:        s.Handle,
		DisplayName: s.Name,
		Status:      s.Status,
		CreatedBy:   utils.StringPtrIfNotEmpty(s.CreatedBy),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// toSecretInfo converts s via secretToInfo and resolves its createdBy UUID to
// its raw external identity.
func (h *WebSubAPIHmacSecretHandler) toSecretInfo(s *model.WebSubAPIHmacSecret) (*api.WebSubAPIHmacSecretInfo, error) {
	info := secretToInfo(s)
	if info == nil {
		return nil, nil
	}
	if err := h.identity.ResolveIdentityField(&info.CreatedBy); err != nil {
		return nil, err
	}
	return info, nil
}
