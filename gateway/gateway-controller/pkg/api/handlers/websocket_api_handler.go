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

// WebSocketApi management endpoints.
//
// Like graphql_api_handler.go (and mcp_proxy_handler.go before it), these are
// methods on the shared *APIServer rather than a separate embedded handler
// struct: the kind is wired into the generic deployment path through
// RegisterKindDeployParser / RegisterKindConfigValidator, so there is no
// kind-specific service for a struct to hold.

package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// webSocketKind is the artifact kind string these endpoints operate on.
const webSocketKind = string(api.WebSocketAPIKindWebSocketApi)

// CreateWebSocketAPI implements ServerInterface.CreateWebSocketAPI
// (POST /websocket-apis)
func (s *APIServer) CreateWebSocketAPI(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          webSocketKind,
		APIID:         "", // empty to generate a new UUID
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy WebSocket API configuration", slog.Any("error", err))
		if mapRenderError(w, "create", err) {
			return
		}
		if mapValidationError(w, err) {
			return
		}
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if isWebSocketAPICreateBadRequest(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to create configuration",
		})
		return
	}

	s.pushDeployableWebSocketArtifact(result, correlationID, log)

	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig))
}

// ListWebSocketAPIs implements ServerInterface.ListWebSocketAPIs
// (GET /websocket-apis)
func (s *APIServer) ListWebSocketAPIs(w http.ResponseWriter, r *http.Request, params api.ListWebSocketAPIsParams) {
	configs, err := s.db.GetAllConfigsByKind(webSocketKind)
	if err != nil {
		s.logger.Error("Failed to get WebSocket APIs", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve WebSocket API configurations",
		})
		return
	}

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		if params.DisplayName != nil && *params.DisplayName != "" && cfg.DisplayName != *params.DisplayName {
			continue
		}
		if params.Version != nil && *params.Version != "" && cfg.Version != *params.Version {
			continue
		}
		if params.Context != nil && *params.Context != "" {
			cfgContext, err := cfg.GetContext()
			if err != nil {
				s.logger.Error("Failed to get context for WebSocket API config", slog.Any("error", err), slog.String("uuid", cfg.UUID))
				continue
			}
			if cfgContext != *params.Context {
				continue
			}
		}
		if params.Status != nil && *params.Status != "" && string(cfg.DesiredState) != string(*params.Status) {
			continue
		}
		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":        "success",
		"count":         len(items),
		"websocketApis": items,
	})
}

// GetWebSocketAPIById implements ServerInterface.GetWebSocketAPIById
// (GET /websocket-apis/{id})
func (s *APIServer) GetWebSocketAPIById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(webSocketKind, id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("WebSocket API configuration not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("WebSocketApi with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get WebSocket API configuration", slog.Any("error", err), slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve configuration",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// UpdateWebSocketAPI implements ServerInterface.UpdateWebSocketAPI
// (PUT /websocket-apis/{id})
func (s *APIServer) UpdateWebSocketAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	existing, err := s.db.GetConfigByKindAndHandle(webSocketKind, id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("WebSocket API configuration not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("WebSocketApi with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get WebSocket API configuration", slog.Any("error", err), slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve configuration",
		})
		return
	}

	// Validate the handle match BEFORE persisting anything. DeployAPIConfiguration
	// upserts immediately, so checking afterwards would let a mismatched body
	// rename the stored config before the mismatch is reported, orphaning the
	// original handle even though the client gets a 400.
	var websocketConfig api.WebSocketAPI
	if err := s.parser.Parse(body, r.Header.Get("Content-Type"), &websocketConfig); err != nil {
		log.Error("Failed to parse WebSocket API configuration", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("failed to parse configuration: %v", err),
		})
		return
	}
	if websocketConfig.Metadata.Name != "" && websocketConfig.Metadata.Name != id {
		log.Warn("WebSocket API update handle mismatch", slog.String("pathHandle", id), slog.String("bodyHandle", websocketConfig.Metadata.Name))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("metadata.name '%s' does not match path id '%s'", websocketConfig.Metadata.Name, id),
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	// Reuse the existing UUID so DeployAPIConfiguration updates rather than
	// creating a second artifact.
	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          webSocketKind,
		APIID:         existing.UUID,
		Origin:        existing.Origin,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update WebSocket API configuration", slog.Any("error", err))
		if mapRenderError(w, "update", err) {
			return
		}
		if mapValidationError(w, err) {
			return
		}
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if isWebSocketAPICreateBadRequest(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to update configuration",
		})
		return
	}

	s.pushDeployableWebSocketArtifact(result, correlationID, log)

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig))
}

// DeleteWebSocketAPI implements ServerInterface.DeleteWebSocketAPI
// (DELETE /websocket-apis/{id})
//
// Deleting takes effect for new handshakes immediately. Connections already open
// are not torn down — Envoy applies new route state to new streams only — and end
// no later than their own maximum connection duration, which is why that bound is
// always finite.
func (s *APIServer) DeleteWebSocketAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(webSocketKind, id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("WebSocket API configuration not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("WebSocketApi with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get WebSocket API configuration", slog.Any("error", err), slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve configuration",
		})
		return
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebSocket API config from database", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)
	s.publishWebSocketAPIEvent("DELETE", cfg.UUID, correlationID, log)

	// Notify the control plane (DP->CP) that this artifact was deleted; it keeps
	// the artifact and marks it undeployed.
	s.pushArtifactUndeploy(cfg, log)

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "WebSocketApi deleted successfully",
		"id":      id,
	})
}

// pushDeployableWebSocketArtifact pushes a newly created/updated WebSocket API to
// the control plane. No-op when push is disabled, disconnected, or the result was
// a stale/no-op deployment.
func (s *APIServer) pushDeployableWebSocketArtifact(result *utils.APIDeploymentResult, correlationID string, log *slog.Logger) {
	if result.IsStale {
		return
	}
	if s.controlPlaneClient == nil || !s.controlPlaneClient.IsConnected() || s.controlPlaneClient.IsOnPrem() ||
		!s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
		return
	}
	cfgID := result.StoredConfig.UUID
	deployedAt := result.StoredConfig.DeployedAt
	s.controlPlaneClient.SubmitArtifactPush(func() {
		s.waitForDeploymentAndPush(cfgID, correlationID, deployedAt, log)
	})
}

// publishWebSocketAPIEvent publishes a delete event to the event hub so all
// replicas (including self) converge through the event listener sync.
func (s *APIServer) publishWebSocketAPIEvent(action, entityID, correlationID string, logger *slog.Logger) {
	s.publishGraphQLAPIEvent(action, entityID, correlationID, logger)
}

// CreateWebSocketAPIKey implements ServerInterface.CreateWebSocketAPIKey
// (POST /websocket-apis/{id}/api-keys)
func (s *APIServer) CreateWebSocketAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "CreateWebSocketAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		log.Error("Failed to parse request body for WebSocket API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindWebSocketApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if utils.IsAPIKeyBadRequest(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create WebSocket API key", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

// RevokeWebSocketAPIKey implements ServerInterface.RevokeWebSocketAPIKey
// (DELETE /websocket-apis/{id}/api-keys/{apiKeyName})
//
// Revoking stops new handshakes from using the key. A connection already open was
// authorized at its own handshake and is not re-checked, so it continues until its
// maximum connection duration elapses.
func (s *APIServer) RevokeWebSocketAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RevokeWebSocketAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindWebSocketApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to revoke WebSocket API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// UpdateWebSocketAPIKey implements ServerInterface.UpdateWebSocketAPIKey
// (PUT /websocket-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateWebSocketAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "UpdateWebSocketAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "apiKey is required"})
		return
	}

	params := utils.APIKeyUpdateParams{
		Kind:          models.KindWebSocketApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.UpdateAPIKey(params)
	if err != nil {
		if storage.IsOperationNotAllowedError(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if utils.IsAPIKeyBadRequest(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update WebSocket API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RegenerateWebSocketAPIKey implements ServerInterface.RegenerateWebSocketAPIKey
// (POST /websocket-apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateWebSocketAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RegenerateWebSocketAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindWebSocketApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if utils.IsAPIKeyBadRequest(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to regenerate WebSocket API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// ListWebSocketAPIKeys implements ServerInterface.ListWebSocketAPIKeys
// (GET /websocket-apis/{id}/api-keys)
func (s *APIServer) ListWebSocketAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "ListWebSocketAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindWebSocketApi,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to list WebSocket API keys", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// isWebSocketAPICreateBadRequest reports whether err is a client-input failure
// from the shared DeployAPIConfiguration path that the other error mappers
// (mapRenderError, mapValidationError, storage.IsConflictError) don't recognize.
// Without it these fall through to a generic 500 instead of the 400 the caller
// actually earned. Mirrors isGraphQLAPICreateBadRequest; DeployAPIConfiguration is
// shared across kinds, so the message classes are the same.
func isWebSocketAPICreateBadRequest(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, probe := range []string{
		"failed to parse configuration",
		"failed to unmarshal websocket api configuration",
		"resource kind is required",
		"unsupported resource kind",
		"invalid or missing origin",
		"unexpected configuration type",
	} {
		if strings.Contains(msg, probe) {
			return true
		}
	}
	return false
}
