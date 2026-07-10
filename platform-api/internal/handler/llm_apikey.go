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

// LLMProviderAPIKeyHandler handles API key operations for LLM providers
type LLMProviderAPIKeyHandler struct {
	apiKeyService *service.LLMProviderAPIKeyService
	identity      *service.IdentityService
	slogger       *slog.Logger
}

// NewLLMProviderAPIKeyHandler creates a new LLM provider API key handler
func NewLLMProviderAPIKeyHandler(apiKeyService *service.LLMProviderAPIKeyService, identity *service.IdentityService, slogger *slog.Logger) *LLMProviderAPIKeyHandler {
	return &LLMProviderAPIKeyHandler{
		apiKeyService: apiKeyService,
		identity:      identity,
		slogger:       slogger,
	}
}

// ListAPIKeys handles GET /api/v0.9/llm-providers/{llmProviderId}/api-keys
func (h *LLMProviderAPIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerID := r.PathValue("llmProviderId")
	if providerID == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}

	callerUserID, err := resolveActorErr(r, h.identity, "list LLM provider API keys")
	if err != nil {
		return err
	}

	limit, offset := parsePagination(r)

	response, err := h.apiKeyService.ListLLMProviderAPIKeys(r.Context(), providerID, orgID, callerUserID, limit, offset)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to list LLM provider API keys for provider %s in org %s", providerID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, response)
	return nil
}

// DeleteAPIKey handles DELETE /api/v0.9/llm-providers/{llmProviderId}/api-keys/{apiKeyId}
func (h *LLMProviderAPIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerID := r.PathValue("llmProviderId")
	if providerID == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	callerUserID, err := resolveActorErr(r, h.identity, "delete LLM provider API key")
	if err != nil {
		return err
	}

	if err := h.apiKeyService.DeleteLLMProviderAPIKey(r.Context(), providerID, orgID, callerUserID, keyName); err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrAPIKeyNotFound) {
			return apperror.LLMProviderAPIKeyNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrAPIKeyForbidden) {
			return apperror.LLMProviderAPIKeyForbidden.Wrap(err)
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to delete LLM provider API key %s for provider %s in org %s", keyName, providerID, orgID))
	}

	h.slogger.Info("Successfully deleted LLM provider API key", "providerId", providerID, "keyName", keyName, "organizationId", orgID)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// CreateAPIKey handles POST /api/v0.9/llm-providers/{llmProviderId}/api-keys
func (h *LLMProviderAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	providerID := r.PathValue("llmProviderId")
	if providerID == "" {
		return apperror.ValidationFailed.New("LLM provider ID is required")
	}

	var req api.CreateLLMProviderAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid LLM provider API key creation request for provider %s", providerID))
	}

	// Validate that displayName is provided (name is optional; auto-generated from displayName if absent)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("'displayName' is required")
	}

	userID, err := resolveActorErr(r, h.identity, "create LLM provider API key")
	if err != nil {
		return err
	}

	response, err := h.apiKeyService.CreateLLMProviderAPIKey(r.Context(), providerID, orgID, userID, &req)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}

		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create LLM provider API key for provider %s in org %s", providerID, orgID))
	}

	h.slogger.Info("Successfully created LLM provider API key", "providerId", providerID, "organizationId", orgID, "keyId", response.Id)

	setLocation(w, "llm-providers", providerID, "api-keys", response.Id)
	httputil.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// RegisterRoutes registers LLM provider API key routes with the router
func (h *LLMProviderAPIKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+constants.APIBasePath+"/llm-providers/{llmProviderId}/api-keys", middleware.MapErrors(h.slogger, h.CreateAPIKey))
	mux.HandleFunc("GET "+constants.APIBasePath+"/llm-providers/{llmProviderId}/api-keys", middleware.MapErrors(h.slogger, h.ListAPIKeys))
	mux.HandleFunc("DELETE "+constants.APIBasePath+"/llm-providers/{llmProviderId}/api-keys/{apiKeyId}", middleware.MapErrors(h.slogger, h.DeleteAPIKey))
}
