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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"

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
func (h *LLMProxyAPIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyID := r.PathValue("llmProxyId")
	if proxyID == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}

	callerUserID, err := resolveActorErr(r, h.identity, "list LLM proxy API keys")
	if err != nil {
		return err
	}

	limit, offset := parsePagination(r)

	response, err := h.apiKeyService.ListLLMProxyAPIKeys(r.Context(), proxyID, orgID, callerUserID, limit, offset)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return serviceError(err, fmt.Sprintf("failed to list LLM proxy API keys for proxy %s in org %s", proxyID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, response)
	return nil
}

// DeleteAPIKey handles DELETE /api/v0.9/llm-proxies/{llmProxyId}/api-keys/{apiKeyId}
func (h *LLMProxyAPIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyID := r.PathValue("llmProxyId")
	if proxyID == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	callerUserID, err := resolveActorErr(r, h.identity, "delete LLM proxy API key")
	if err != nil {
		return err
	}

	if err := h.apiKeyService.DeleteLLMProxyAPIKey(r.Context(), proxyID, orgID, callerUserID, keyName); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete LLM proxy API key %s for proxy %s in org %s", keyName, proxyID, orgID))
	}

	h.slogger.Info("Successfully deleted LLM proxy API key", "proxyId", proxyID, "keyName", keyName, "organizationId", orgID)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// CreateAPIKey handles POST /api/v0.9/llm-proxies/{llmProxyId}/api-keys
func (h *LLMProxyAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	proxyID := r.PathValue("llmProxyId")
	if proxyID == "" {
		return apperror.ValidationFailed.New("LLM proxy ID is required")
	}

	var req api.CreateLLMProxyAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid LLM proxy API key creation request for proxy %s", proxyID))
	}

	// Validate that displayName is provided (name is optional; auto-generated from displayName if absent)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("'displayName' is required")
	}

	userID, err := resolveActorErr(r, h.identity, "create LLM proxy API key")
	if err != nil {
		return err
	}

	response, err := h.apiKeyService.CreateLLMProxyAPIKey(r.Context(), proxyID, orgID, userID, &req)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}

		return serviceError(err, fmt.Sprintf("failed to create LLM proxy API key for proxy %s in org %s", proxyID, orgID))
	}

	h.slogger.Info("Successfully created LLM proxy API key", "proxyId", proxyID, "organizationId", orgID, "keyId", response.Id)

	setLocation(w, "llm-proxies", proxyID, "api-keys", response.Id)
	httputil.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// RegisterRoutes registers LLM proxy API key routes with the router
func (h *LLMProxyAPIKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-proxies/{llmProxyId}/api-keys", middleware.MapErrors(h.slogger, h.CreateAPIKey))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-proxies/{llmProxyId}/api-keys", middleware.MapErrors(h.slogger, h.ListAPIKeys))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-proxies/{llmProxyId}/api-keys/{apiKeyId}", middleware.MapErrors(h.slogger, h.DeleteAPIKey))
}
