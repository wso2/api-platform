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
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// GraphQLAPIKeyHandler handles API key operations for GraphQL APIs.
//
// Unlike LLM Provider/Proxy (which each get a dedicated <Kind>APIKeyService —
// see llm_apikey.go/llm_proxy_apikey.go), GraphQL API keys reuse the existing
// *service.APIKeyService unmodified. That service already resolves the target
// artifact via the kind-agnostic ArtifactRepository.GetAPIMetadataByHandleAndKind
// and is exercised in production with multiple kinds beyond RestApi today (the
// eventgateway plugin's WebSub/WebBroker API key handlers call the very same
// instance with constants.WebSubApi/constants.WebBrokerApi — see
// plugins/eventgateway/handler/{websub,webbroker}_apikey.go). Its only
// REST-typed dependency (apiRepo repository.APIRepository) is used solely for
// GetAPIGatewaysWithDetails, which reads the kind-agnostic
// artifact_gateway_mappings table and works correctly for any artifact kind.
// So this handler is the only new code needed here — introducing a
// GraphQLAPIKeyService would duplicate ~300 lines of hashing/broadcast logic
// that is already proven kind-agnostic.
type GraphQLAPIKeyHandler struct {
	apiKeyService *service.APIKeyService
	identity      *service.IdentityService
	authzMode     string
	slogger       *slog.Logger
}

// NewGraphQLAPIKeyHandler creates a new GraphQL API key handler.
func NewGraphQLAPIKeyHandler(apiKeyService *service.APIKeyService, identity *service.IdentityService, authzMode string, slogger *slog.Logger) *GraphQLAPIKeyHandler {
	return &GraphQLAPIKeyHandler{
		apiKeyService: apiKeyService,
		identity:      identity,
		authzMode:     authzMode,
		slogger:       slogger,
	}
}

// isKeyAdmin reports whether the caller holds constants.ScopeAPIKeyAllManage and may
// therefore act on API keys created by other users, not only their own.
func (h *GraphQLAPIKeyHandler) isKeyAdmin(r *http.Request) bool {
	return middleware.HasEffectiveScope(r, h.authzMode, constants.ScopeAPIKeyAllManage)
}

// CreateAPIKey handles POST /api/v0.9/graphql-apis/{graphqlApiId}/api-keys
func (h *GraphQLAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	userId, err := resolveActorErr(r, h.identity, "create GraphQL API key")
	if err != nil {
		return err
	}

	apiHandle := r.PathValue("graphqlApiId")
	if apiHandle == "" {
		return apperror.ValidationFailed.New("API handle is required")
	}

	var req api.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid API key creation request for user %s", userId))
	}

	if req.ApiKey == "" {
		return apperror.ValidationFailed.New("API key value is required")
	}

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

	if err := h.apiKeyService.CreateAPIKey(r.Context(), apiHandle, constants.GraphQLApi, orgId, userId, &req); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to create API key %q for GraphQL API %s in org %s by user %s", name, apiHandle, orgId, userId))
	}

	keyName := ""
	if req.Id != nil {
		keyName = *req.Id
	}
	h.slogger.Info("Successfully created GraphQL API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	setLocation(w, "graphql-apis", apiHandle, "api-keys", name)
	httputil.WriteJSON(w, http.StatusCreated, api.CreateAPIKeyResponse{
		Status:  api.CreateAPIKeyResponseStatusSuccess,
		KeyId:   req.Id,
		Message: "API key created and broadcasted to gateways successfully",
	})
	return nil
}

// UpdateAPIKey handles PUT /api/v0.9/graphql-apis/{graphqlApiId}/api-keys/{apiKeyId}
func (h *GraphQLAPIKeyHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	userId, err := resolveActorErr(r, h.identity, "update GraphQL API key")
	if err != nil {
		return err
	}

	apiHandle := r.PathValue("graphqlApiId")
	if apiHandle == "" {
		return apperror.ValidationFailed.New("API handle is required")
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	var req api.UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body").
			WithLogMessage(fmt.Sprintf("invalid API key update request for key %s of GraphQL API %s in org %s by user %s", keyName, apiHandle, orgId, userId))
	}

	if req.ApiKey == "" {
		return apperror.ValidationFailed.New("API key value is required")
	}

	if err := utils.ValidateHandleImmutable(keyName, req.Name); err != nil {
		h.slogger.Warn("API key name mismatch", "userId", userId, "orgId", orgId, "apiHandle", apiHandle, "urlKeyName", keyName, "bodyKeyName", *req.Name)
		return apperror.ValidationFailed.New(fmt.Sprintf("API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *req.Name, keyName)).
			WithLogMessage(fmt.Sprintf("API key name mismatch for GraphQL API %s in org %s by user %s", apiHandle, orgId, userId))
	}

	if err := h.apiKeyService.UpdateAPIKey(r.Context(), apiHandle, constants.GraphQLApi, orgId, keyName, userId, h.isKeyAdmin(r), false, &req); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to update API key %s for GraphQL API %s in org %s by user %s", keyName, apiHandle, orgId, userId))
	}

	h.slogger.Info("Successfully updated GraphQL API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	httputil.WriteJSON(w, http.StatusOK, api.UpdateAPIKeyResponse{
		Status:  api.UpdateAPIKeyResponseStatusSuccess,
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   &keyName,
	})
	return nil
}

// RevokeAPIKey handles DELETE /api/v0.9/graphql-apis/{graphqlApiId}/api-keys/{apiKeyId}
func (h *GraphQLAPIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgId, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	apiHandle := r.PathValue("graphqlApiId")
	if apiHandle == "" {
		return apperror.ValidationFailed.New("API handle is required")
	}

	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	userId, err := resolveActorErr(r, h.identity, "revoke GraphQL API key")
	if err != nil {
		return err
	}

	if err := h.apiKeyService.RevokeAPIKey(r.Context(), apiHandle, constants.GraphQLApi, orgId, keyName, userId, h.isKeyAdmin(r), false); err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		return apperror.Internal.Wrap(err).
			WithLogMessage(fmt.Sprintf("failed to revoke API key %s for GraphQL API %s in org %s by user %s", keyName, apiHandle, orgId, userId))
	}

	h.slogger.Info("Successfully revoked GraphQL API key", "userId", userId, "apiHandle", apiHandle, "orgId", orgId, "keyName", keyName)

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RegisterRoutes registers GraphQL API key routes with the router.
func (h *GraphQLAPIKeyHandler) RegisterRoutes(mux router.Router) {
	h.slogger.Debug("Registering GraphQL API key routes")
	base := constants.APIBasePath + "/graphql-apis/{graphqlApiId}/api-keys"
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPIKey))
	mux.HandleFunc("PUT "+base+"/{apiKeyId}", middleware.MapErrors(h.slogger, h.UpdateAPIKey))
	mux.HandleFunc("DELETE "+base+"/{apiKeyId}", middleware.MapErrors(h.slogger, h.RevokeAPIKey))
}
