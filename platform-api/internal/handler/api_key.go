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

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// APIKeyHandler handles API key operations for external services (Cloud APIM)
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService
	identity      *service.IdentityService
	slogger       *slog.Logger
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(apiKeyService *service.APIKeyService, identity *service.IdentityService, slogger *slog.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		identity:      identity,
		slogger:       slogger,
	}
}

// CreateAPIKey handles POST /rest-apis/{restApiId}/api-keys
// This endpoint allows users to inject external API keys to all the gateways where the API is deployed
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	userId, err := resolveActorErr(r, h.identity, "create API key")
	if err != nil {
		return err
	}

	// Extract API handle from path parameter (parameter named apiId for backward compatibility, but contains handle)
	apiHandle := r.PathValue("restApiId")
	if apiHandle == "" {
		return apperror.ValidationFailed.New("API handle is required")
	}

	// Parse and validate request body
	var req api.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid API key creation request for user %s", userId))
	}

	if req.ApiKey == "" {
		return apperror.ValidationFailed.New("API key value is required")
	}

	// If user has provided an id, use it. Otherwise, generate one from the display name.
	var name string
	if req.Id != nil && *req.Id != "" {
		name = *req.Id
	} else {
		generatedName, err := utils.GenerateHandle(req.DisplayName, nil)
		if err != nil {
			return apperror.ValidationFailed.Wrap(err, "Failed to generate API key name")
		}
		name = generatedName
		req.Id = &name
	}

	// Create the API key and broadcast to gateways
	if err := h.apiKeyService.CreateAPIKey(r.Context(), apiHandle, constants.RestApi, orgId, userId, &req); err != nil {
		// Handle specific error cases
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			return apperror.GatewayConnectionUnavailable.Wrap(err)
		}

		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create API key %q for API %s in org %s by user %s", name, apiHandle, orgId, userId))
	}

	keyName := ""
	if req.Id != nil {
		keyName = *req.Id
	}
	h.slogger.Info("Successfully created API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	// Return success response
	setLocation(w, "rest-apis", apiHandle, "api-keys", name)
	httputil.WriteJSON(w, http.StatusCreated, api.CreateAPIKeyResponse{
		Status:  api.CreateAPIKeyResponseStatusSuccess,
		KeyId:   req.Id,
		Message: "API key created and broadcasted to gateways successfully",
	})
	return nil
}

// UpdateAPIKey handles PUT /rest-apis/{restApiId}/api-keys/{apiKeyId}
// This endpoint allows external platforms to update/regenerate external API keys on hybrid gateways
func (h *APIKeyHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) error {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	userId, err := resolveActorErr(r, h.identity, "update API key")
	if err != nil {
		return err
	}

	// Extract API ID and key name from path parameters
	apiHandle := r.PathValue("restApiId")
	if apiHandle == "" {
		return apperror.ValidationFailed.New("API handle is required")
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	// Parse and validate request body
	var req api.UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid API key update request for key %s of API %s in org %s by user %s", keyName, apiHandle, orgId, userId))
	}

	// Validate new API key value
	if req.ApiKey == "" {
		return apperror.ValidationFailed.New("API key value is required")
	}

	// Validate that the name in the request body (if provided) matches the URL path parameter
	if err := utils.ValidateHandleImmutable(keyName, req.Name); err != nil {
		h.slogger.Warn("API key name mismatch", "userId", userId, "orgId", orgId, "apiHandle", apiHandle, "urlKeyName", keyName, "bodyKeyName", *req.Name)
		return apperror.ValidationFailed.New(fmt.Sprintf("API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *req.Name, keyName)).
			WithLogMessage(fmt.Sprintf("API key name mismatch for API %s in org %s by user %s", apiHandle, orgId, userId))
	}

	// Update the API key and broadcast to gateways
	if err := h.apiKeyService.UpdateAPIKey(r.Context(), apiHandle, constants.RestApi, orgId, keyName, userId, &req); err != nil {
		// Handle specific error cases
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			return apperror.GatewayConnectionUnavailable.Wrap(err)
		}

		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to update API key %s for API %s in org %s by user %s", keyName, apiHandle, orgId, userId))
	}

	h.slogger.Info("Successfully updated API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	// Return success response
	httputil.WriteJSON(w, http.StatusOK, api.UpdateAPIKeyResponse{
		Status:  api.UpdateAPIKeyResponseStatusSuccess,
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   &keyName,
	})
	return nil
}

// RevokeAPIKey handles DELETE /rest-apis/{restApiId}/api-keys/{apiKeyId}
// This endpoint allows Cloud APIM to revoke external API keys on hybrid gateways
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	// Extract API ID and key name from path parameters
	apiHandle := r.PathValue("restApiId")
	if apiHandle == "" {
		return apperror.ValidationFailed.New("API handle is required")
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	userId, err := resolveActorErr(r, h.identity, "revoke API key")
	if err != nil {
		return err
	}

	// Revoke the API key and broadcast to gateways
	if err := h.apiKeyService.RevokeAPIKey(r.Context(), apiHandle, constants.RestApi, orgId, keyName, userId); err != nil {
		// Handle specific error cases
		if errors.Is(err, constants.ErrAPINotFound) {
			return apperror.ArtifactNotFound.Wrap(err)
		}
		if errors.Is(err, constants.ErrGatewayUnavailable) {
			return apperror.GatewayConnectionUnavailable.Wrap(err)
		}

		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to revoke API key %s for API %s in org %s by user %s", keyName, apiHandle, orgId, userId))
	}

	h.slogger.Info("Successfully revoked API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	// Return success response (204 No Content)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RegisterRoutes registers API key routes with the router
func (h *APIKeyHandler) RegisterRoutes(mux *http.ServeMux) {
	h.slogger.Debug("Registering API key routes")
	base := constants.APIBasePath + "/rest-apis/{restApiId}/api-keys"
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPIKey))
	mux.HandleFunc("PUT "+base+"/{apiKeyId}", middleware.MapErrors(h.slogger, h.UpdateAPIKey))
	mux.HandleFunc("DELETE "+base+"/{apiKeyId}", middleware.MapErrors(h.slogger, h.RevokeAPIKey))
}
