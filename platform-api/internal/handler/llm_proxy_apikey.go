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
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// LLMProxyAPIKeyHandler handles API key operations for LLM proxies
type LLMProxyAPIKeyHandler struct {
	apiKeyService *service.LLMProxyAPIKeyService
	identity      *service.IdentityService
	slogger       *slog.Logger
}

// NewLLMProxyAPIKeyHandler creates a new LLM proxy API key handler
func NewLLMProxyAPIKeyHandler(apiKeyService *service.LLMProxyAPIKeyService, identity *service.IdentityService, slogger *slog.Logger) *LLMProxyAPIKeyHandler {
	return &LLMProxyAPIKeyHandler{
		apiKeyService: apiKeyService,
		identity:      identity,
		slogger:       slogger,
	}
}

// ListAPIKeys handles GET /api/v0.9/llm-proxies/{llmProxyId}/api-keys
func (h *LLMProxyAPIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	proxyID := r.PathValue("llmProxyId")
	if proxyID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "LLM proxy ID is required"))
		return
	}

	callerUserID, ok := resolveActor(w, r, h.identity, h.slogger, "list LLM proxy API keys")
	if !ok {
		return
	}

	response, err := h.apiKeyService.ListLLMProxyAPIKeys(r.Context(), proxyID, orgID, callerUserID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeArtifactNotFound, "The specified artifact could not be found."))
			return
		}
		h.slogger.Error("Failed to list LLM proxy API keys", "proxyId", proxyID, "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to list API keys"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// DeleteAPIKey handles DELETE /api/v0.9/llm-proxies/{llmProxyId}/api-keys/{apiKeyId}
func (h *LLMProxyAPIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	proxyID := r.PathValue("llmProxyId")
	if proxyID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "LLM proxy ID is required"))
		return
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "API key name is required"))
		return
	}

	callerUserID, ok := resolveActor(w, r, h.identity, h.slogger, "delete LLM proxy API key")
	if !ok {
		return
	}

	err := h.apiKeyService.DeleteLLMProxyAPIKey(r.Context(), proxyID, orgID, callerUserID, keyName)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeArtifactNotFound, "The specified artifact could not be found."))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyAPIKeyNotFound, "The specified API key could not be found."))
			return
		}
		if errors.Is(err, constants.ErrAPIKeyForbidden) {
			httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponseWithCode(
				utils.CodeLLMProxyAPIKeyForbidden, "Only the key creator can delete this API key"))
			return
		}
		h.slogger.Error("Failed to delete LLM proxy API key", "proxyId", proxyID, "keyName", keyName, "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to delete API key"))
		return
	}

	h.slogger.Info("Successfully deleted LLM proxy API key", "proxyId", proxyID, "keyName", keyName, "organizationId", orgID)
	w.WriteHeader(http.StatusNoContent)
}

// CreateAPIKey handles POST /api/v0.9/llm-proxies/{llmProxyId}/api-keys
func (h *LLMProxyAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	proxyID := r.PathValue("llmProxyId")
	if proxyID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "LLM proxy ID is required"))
		return
	}

	var req api.CreateLLMProxyAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.slogger.Error("Invalid LLM proxy API key creation request", "proxyId", proxyID, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Invalid request body"))
		return
	}

	// Validate that displayName is provided (name is optional; auto-generated from displayName if absent)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "'displayName' is required"))
		return
	}

	userID, ok := resolveActor(w, r, h.identity, h.slogger, "create LLM proxy API key")
	if !ok {
		return
	}

	response, err := h.apiKeyService.CreateLLMProxyAPIKey(r.Context(), proxyID, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeArtifactNotFound, "The specified artifact could not be found."))
			return
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, utils.NewErrorResponseWithCode(
				utils.CodeGatewayConnectionUnavailable, "No gateway connections are currently available."))
			return
		}

		h.slogger.Error("Failed to create LLM proxy API key", "proxyId", proxyID, "organizationId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to create API key"))
		return
	}

	h.slogger.Info("Successfully created LLM proxy API key", "proxyId", proxyID, "organizationId", orgID, "keyId", response.Id)

	httputil.WriteJSON(w, http.StatusCreated, response)
}

// RegisterRoutes registers LLM proxy API key routes with the router
func (h *LLMProxyAPIKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-proxies/{llmProxyId}/api-keys", h.CreateAPIKey)
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-proxies/{llmProxyId}/api-keys", h.ListAPIKeys)
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-proxies/{llmProxyId}/api-keys/{apiKeyId}", h.DeleteAPIKey)
}
