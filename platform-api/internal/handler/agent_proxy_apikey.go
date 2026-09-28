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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// agentProxyAPIKeyMaxBodyBytes bounds an Agent proxy API-key request body. The
// body carries a display name, an identifier, an expiry, an issuer and at most
// one key value, so the ceiling is small on purpose.
const agentProxyAPIKeyMaxBodyBytes = 16 << 10 // 16 KiB

// AgentProxyAPIKeyHandler serves the four public Agent proxy API-key operations.
//
// Create, update and revoke delegate to the shared APIKeyService with kind
// constants.AgentProxy — the same service and request/response shapes REST,
// WebSub and WebBroker API keys use — so Agent keys behave exactly like REST API
// keys. Listing has no REST counterpart and is served by AgentProxyAPIKeyService.
//
// These are distinct from the gateway-internal /api/internal/v1/agents/api-keys
// backfill route: that one is gateway-authenticated and returns key hashes for
// gateways to enforce, while everything here is publisher-facing and never
// returns stored key material.
//
// Route patterns must match resources/openapi.yaml exactly; ScopeEnforcer keys
// on the registered pattern and is deny-by-default.
type AgentProxyAPIKeyHandler struct {
	apiKeyService *service.APIKeyService
	listService   *service.AgentProxyAPIKeyService
	identity      *service.IdentityService
	authzMode     string
	slogger       *slog.Logger
}

// NewAgentProxyAPIKeyHandler creates a new AgentProxyAPIKeyHandler.
func NewAgentProxyAPIKeyHandler(apiKeyService *service.APIKeyService, listService *service.AgentProxyAPIKeyService,
	identity *service.IdentityService, authzMode string, slogger *slog.Logger) *AgentProxyAPIKeyHandler {
	return &AgentProxyAPIKeyHandler{
		apiKeyService: apiKeyService,
		listService:   listService,
		identity:      identity,
		authzMode:     authzMode,
		slogger:       slogger,
	}
}

// RegisterRoutes wires the Agent proxy API-key collection and member routes.
func (h *AgentProxyAPIKeyHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/agent-proxies/{agentProxyId}/api-keys"
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListAPIKeys))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateAPIKey))
	mux.HandleFunc("PUT "+base+"/{apiKeyId}", middleware.MapErrors(h.slogger, h.UpdateAPIKey))
	mux.HandleFunc("DELETE "+base+"/{apiKeyId}", middleware.MapErrors(h.slogger, h.RevokeAPIKey))
}

// isKeyAdmin reports whether the caller holds constants.ScopeAPIKeyAllManage and
// may therefore act on keys other users created. It is resolved from the verified
// token here and passed to the service as an explicit argument (GO-AUTH-020);
// the Agent proxy's own :manage scopes authorize the operation, not cross-user
// access to someone else's key.
func (h *AgentProxyAPIKeyHandler) isKeyAdmin(r *http.Request) bool {
	return middleware.HasEffectiveScope(r, h.authzMode, constants.ScopeAPIKeyAllManage)
}

// ListAPIKeys handles GET /api/v0.9/agent-proxies/{agentProxyId}/api-keys
func (h *AgentProxyAPIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	callerUserID, err := resolveActorErr(r, h.identity, "list Agent proxy API keys")
	if err != nil {
		return err
	}

	limit, offset := parsePagination(r)
	resp, err := h.listService.List(r.Context(), orgID, r.PathValue("agentProxyId"), callerUserID, h.isKeyAdmin(r), limit, offset)
	if err != nil {
		return serviceError(err, "failed to list Agent proxy API keys")
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// CreateAPIKey handles POST /api/v0.9/agent-proxies/{agentProxyId}/api-keys
func (h *AgentProxyAPIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	if handle == "" {
		return apperror.ValidationFailed.New("The Agent proxy id is required.")
	}

	userID, err := resolveActorErr(r, h.identity, "create Agent proxy API key")
	if err != nil {
		return err
	}

	var req api.CreateAPIKeyRequest
	if err := decodeAgentProxyAPIKeyBody(w, r, &req); err != nil {
		return err
	}
	if req.DisplayName == "" {
		return apperror.ValidationFailed.New("Display name is required")
	}

	// Same id resolution as the REST handler: a supplied id is used, otherwise one
	// is derived from the display name. APIKeyService suffixes it on collision.
	if req.Id == nil || *req.Id == "" {
		generated, err := utils.GenerateHandle(req.DisplayName, nil)
		if err != nil {
			return apperror.ValidationFailed.Wrap(err, "Failed to generate API key name")
		}
		req.Id = &generated
	}

	resp, err := h.apiKeyService.CreateAPIKey(r.Context(), handle, constants.AgentProxy, orgID, userID, &req)
	if err != nil {
		return h.mapServiceError(err, fmt.Sprintf("failed to create API key for Agent proxy %s in org %s", handle, orgID))
	}

	// Location names the id actually stored, which differs from the requested or
	// derived one when the service had to suffix it to avoid a collision.
	keyName := strOrEmpty(resp.KeyId)
	h.slogger.Info("Created Agent proxy API key", "agentProxyId", handle, "organizationId", orgID, "keyName", keyName)
	setLocation(w, "agent-proxies", handle, "api-keys", keyName)
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

// UpdateAPIKey handles PUT /api/v0.9/agent-proxies/{agentProxyId}/api-keys/{apiKeyId}
func (h *AgentProxyAPIKeyHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	if handle == "" {
		return apperror.ValidationFailed.New("The Agent proxy id is required.")
	}
	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	userID, err := resolveActorErr(r, h.identity, "update Agent proxy API key")
	if err != nil {
		return err
	}

	var req api.UpdateAPIKeyRequest
	if err := decodeAgentProxyAPIKeyBody(w, r, &req); err != nil {
		return err
	}
	if req.ApiKey == "" {
		return apperror.ValidationFailed.New("API key value is required")
	}
	if err := utils.ValidateHandleImmutable(keyName, req.Name); err != nil {
		return apperror.ValidationFailed.New(fmt.Sprintf(
			"API key name mismatch: name in request body '%s' must match the key name in URL '%s'", *req.Name, keyName))
	}

	if err := h.apiKeyService.UpdateAPIKey(r.Context(), handle, constants.AgentProxy, orgID, keyName, userID, h.isKeyAdmin(r), false, &req); err != nil {
		return h.mapServiceError(err, fmt.Sprintf("failed to update API key %s for Agent proxy %s in org %s", keyName, handle, orgID))
	}

	h.slogger.Info("Updated Agent proxy API key", "agentProxyId", handle, "organizationId", orgID, "keyName", keyName)
	httputil.WriteJSON(w, http.StatusOK, api.UpdateAPIKeyResponse{
		Status:  api.UpdateAPIKeyResponseStatusSuccess,
		Message: "API key updated and broadcasted to gateways successfully",
		KeyId:   &keyName,
	})
	return nil
}

// RevokeAPIKey handles DELETE /api/v0.9/agent-proxies/{agentProxyId}/api-keys/{apiKeyId}
func (h *AgentProxyAPIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}
	handle := r.PathValue("agentProxyId")
	if handle == "" {
		return apperror.ValidationFailed.New("The Agent proxy id is required.")
	}
	keyName := r.PathValue("apiKeyId")
	if keyName == "" {
		return apperror.ValidationFailed.New("API key name is required")
	}

	userID, err := resolveActorErr(r, h.identity, "revoke Agent proxy API key")
	if err != nil {
		return err
	}

	if err := h.apiKeyService.RevokeAPIKey(r.Context(), handle, constants.AgentProxy, orgID, keyName, userID, h.isKeyAdmin(r), false); err != nil {
		return h.mapServiceError(err, fmt.Sprintf("failed to revoke API key %s for Agent proxy %s in org %s", keyName, handle, orgID))
	}

	h.slogger.Info("Revoked Agent proxy API key", "agentProxyId", handle, "organizationId", orgID, "keyName", keyName)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// mapServiceError reports an unknown Agent proxy with the Agent catalog code —
// APIKeyService is kind-agnostic and says only "artifact not found" — and passes
// every other catalog error (key not found, ownership denial, no gateway) through
// unchanged, exactly as the REST handler does.
func (h *AgentProxyAPIKeyHandler) mapServiceError(err error, logMsg string) error {
	if apperror.ArtifactNotFound.Is(err) {
		return apperror.AgentProxyNotFound.New()
	}
	return serviceError(err, logMsg)
}

// decodeAgentProxyAPIKeyBody reads a bounded JSON body into dst. A declared
// non-JSON media type is a 415 (requireJSONContentType), an oversized body a 413,
// and an empty or malformed one a 400.
func decodeAgentProxyAPIKeyBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := requireJSONContentType(r); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, agentProxyAPIKeyMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.Wrap(err, "The request body could not be read.")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return apperror.ValidationFailed.New("A request body is required.")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid request body")
	}
	return nil
}
