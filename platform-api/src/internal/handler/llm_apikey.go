/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// LLMProviderAPIKeyHandler handles API key operations for LLM providers
type LLMProviderAPIKeyHandler struct {
	apiKeyService *service.LLMProviderAPIKeyService
	slogger       *slog.Logger
}

// NewLLMProviderAPIKeyHandler creates a new LLM provider API key handler
func NewLLMProviderAPIKeyHandler(apiKeyService *service.LLMProviderAPIKeyService, slogger *slog.Logger) *LLMProviderAPIKeyHandler {
	return &LLMProviderAPIKeyHandler{
		apiKeyService: apiKeyService,
		slogger:       slogger,
	}
}

// ListAPIKeys handles GET /api/v0.9/llm-providers/{id}/api-keys
func (h *LLMProviderAPIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerID := r.PathValue("id")
	if providerID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}

	callerUserID := r.Header.Get("x-user-id")

	response, err := h.apiKeyService.ListLLMProviderAPIKeys(r.Context(), providerID, orgID, callerUserID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		}
		h.slogger.Error("Failed to list LLM provider API keys", "providerId", providerID, "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to list API keys"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// DeleteAPIKey handles DELETE /api/v0.9/llm-providers/{id}/api-keys/{keyName}
func (h *LLMProviderAPIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerID := r.PathValue("id")
	if providerID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}

	keyName := r.PathValue("keyName")
	if keyName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"API key name is required"))
		return
	}

	callerUserID := r.Header.Get("x-user-id")

	err := h.apiKeyService.DeleteLLMProviderAPIKey(r.Context(), providerID, orgID, callerUserID, keyName)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"API key not found"))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyForbidden) {
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
				"Only the key creator can delete this API key"))
			return
		}
		h.slogger.Error("Failed to delete LLM provider API key", "providerId", providerID, "keyName", keyName, "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to delete API key"))
		return
	}

	h.slogger.Info("Successfully deleted LLM provider API key", "providerId", providerID, "keyName", keyName, "organizationId", orgID)
	w.WriteHeader(http.StatusNoContent)
}

// CreateAPIKey handles POST /api/v0.9/llm-providers/{id}/api-keys
func (h *LLMProviderAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	providerID := r.PathValue("id")
	if providerID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"LLM provider ID is required"))
		return
	}

	var req api.CreateLLMProviderAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("Invalid LLM provider API key creation request", "providerId", providerID, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Invalid request body"))
		return
	}

	// Validate that at least one of name or displayName is provided
	nameProvided := req.Name != nil && *req.Name != ""
	displayNameProvided := req.DisplayName != nil && *req.DisplayName != ""
	if !nameProvided && !displayNameProvided {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"At least one of 'name' or 'displayName' must be provided"))
		return
	}

	userID := r.Header.Get("x-user-id")

	response, err := h.apiKeyService.CreateLLMProviderAPIKey(r.Context(), providerID, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"LLM provider not found"))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponse(503, "Service Unavailable",
				"No gateway connections available"))
			return
		}

		h.slogger.Error("Failed to create LLM provider API key", "providerId", providerID, "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create API key"))
		return
	}

	h.slogger.Info("Successfully created LLM provider API key", "providerId", providerID, "organizationId", orgID, "keyId", response.KeyId)

	httputil.WriteJSON(w, http.StatusCreated, response)
}

// RegisterRoutes registers LLM provider API key routes with the router
func (h *LLMProviderAPIKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-providers/{id}/api-keys", h.CreateAPIKey)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers/{id}/api-keys", h.ListAPIKeys)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-providers/{id}/api-keys/{keyName}", h.DeleteAPIKey)
}
