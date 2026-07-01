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
	"fmt"
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	egservice "platform-api/src/plugins/eventgateway/service"

	"github.com/wso2/go-httpkit/httputil"
)

// WebBrokerAPIKeyHandler handles API key operations for WebBroker APIs
type WebBrokerAPIKeyHandler struct {
	webbrokerAPIService *egservice.WebBrokerAPIService
	apiKeyService       *service.APIKeyService
	slogger             *slog.Logger
}

// NewWebBrokerAPIKeyHandler creates a new WebBrokerAPIKeyHandler instance
func NewWebBrokerAPIKeyHandler(webbrokerAPIService *egservice.WebBrokerAPIService, apiKeyService *service.APIKeyService, slogger *slog.Logger) *WebBrokerAPIKeyHandler {
	return &WebBrokerAPIKeyHandler{
		webbrokerAPIService: webbrokerAPIService,
		apiKeyService:       apiKeyService,
		slogger:             slogger,
	}
}

// RegisterRoutes registers WebBroker API key routes
func (h *WebBrokerAPIKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/api-keys", h.CreateAPIKey)
	mux.HandleFunc("PUT "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/api-keys/{apiKeyId}", h.UpdateAPIKey)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/webbroker-apis/{webBrokerApiId}/api-keys/{apiKeyId}", h.DeleteAPIKey)
}

// CreateAPIKey handles POST /api/v0.9/webbroker-apis/:apiId/api-keys
func (h *WebBrokerAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webBrokerApiId")
	if apiHandle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	// Verify it's a WebBroker API
	if _, err := h.webbrokerAPIService.Get(orgID, apiHandle); err != nil {
		h.handleServiceError(w, err)
		return
	}

	userId := r.Header.Get("x-user-id")

	var req api.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body"))
		return
	}

	if req.ApiKey == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API key value is required"))
		return
	}

	var name string
	if req.Id != nil && *req.Id != "" {
		name = *req.Id
	} else {
		generatedName, err := utils.GenerateHandle(req.DisplayName, nil)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Failed to generate API key name"))
			return
		}
		name = generatedName
		req.Id = &name
	}

	if err := h.apiKeyService.CreateAPIKey(r.Context(), apiHandle, orgID, userId, &req); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "No gateway connections available"))
			return
		}
		h.slogger.Error("Failed to create API key for WebBroker API", "apiHandle", apiHandle, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create API key"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, api.CreateAPIKeyResponse{
		Status:  api.CreateAPIKeyResponseStatusSuccess,
		KeyId:   req.Id,
		Message: "API key created and broadcasted to gateways successfully",
	})
}

// UpdateAPIKey handles PUT /api/v0.9/webbroker-apis/:apiId/api-keys/:keyName
func (h *WebBrokerAPIKeyHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webBrokerApiId")
	if apiHandle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Key name is required"))
		return
	}

	// Verify it's a WebBroker API
	if _, err := h.webbrokerAPIService.Get(orgID, apiHandle); err != nil {
		h.handleServiceError(w, err)
		return
	}

	userId := r.Header.Get("x-user-id")

	var req api.UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Warn("Invalid API key update request", "orgId", orgID, "apiHandle", apiHandle, "keyName", keyName, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid request body: "+err.Error()))
		return
	}

	if req.ApiKey == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API key value is required"))
		return
	}

	// Validate that the name in the request body (if provided) matches the URL path parameter
	if req.Name != nil && *req.Name != "" && *req.Name != keyName {
		h.slogger.Warn("API key name mismatch", "orgId", orgID, "apiHandle", apiHandle, "urlKeyName", keyName, "bodyKeyName", *req.Name)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			fmt.Sprintf("API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *req.Name, keyName)))
		return
	}

	if err := h.apiKeyService.UpdateAPIKey(r.Context(), apiHandle, orgID, keyName, userId, &req); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "No gateway connections available"))
			return
		}
		h.slogger.Error("Failed to update API key for WebBroker API", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update API key"))
		return
	}

	h.slogger.Info("Successfully updated API key for WebBroker API", "apiHandle", apiHandle, "orgId", orgID, "keyName", keyName)
	httputil.WriteJSON(w, http.StatusOK, api.UpdateAPIKeyResponse{
		Status:  api.UpdateAPIKeyResponseStatusSuccess,
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   &keyName,
	})
}

// DeleteAPIKey handles DELETE /api/v0.9/webbroker-apis/:apiId/api-keys/:keyName
func (h *WebBrokerAPIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	apiHandle := r.PathValue("webBrokerApiId")
	keyName := r.PathValue("apiKeyId")

	if apiHandle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API handle is required"))
		return
	}
	if keyName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Key name is required"))
		return
	}

	userId := r.Header.Get("x-user-id")

	if err := h.apiKeyService.RevokeAPIKey(r.Context(), apiHandle, orgID, keyName, userId); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API key not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable", "No gateway connections available"))
			return
		}
		h.slogger.Error("Failed to delete API key for WebBroker API", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete API key"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleServiceError maps service errors to HTTP responses
func (h *WebBrokerAPIKeyHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, constants.ErrInvalidInput):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
	case errors.Is(err, constants.ErrWebBrokerAPINotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "WebBroker API not found"))
	default:
		h.slogger.Error("WebBroker API key service error", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "An unexpected error occurred"))
	}
}
