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

package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// SoapAPI CRUD handlers, implemented directly on *APIServer the same way as the
// GraphQLAPI handlers (graphql_api_handler.go): a SoapApi has no operations or
// upstreamDefinitions to warrant a bespoke service layer, so Create/Update reuse the
// generic s.deploymentService (SoapApi is wired into it via
// utils.RegisterKindDeployParser/RegisterKindConfigValidator — see
// pkg/utils/soap_deployment.go — not a hardcoded case in api_deployment.go).

// CreateSoapAPI implements ServerInterface.CreateSoapAPI
// (POST /soap-apis)
func (s *APIServer) CreateSoapAPI(w http.ResponseWriter, r *http.Request) {
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

	// An unparsable body falls through to DeployAPIConfiguration, which reports it.
	var soapConfig api.SoapAPI
	_ = s.parser.Parse(body, r.Header.Get("Content-Type"), &soapConfig)

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          string(api.SoapAPIKindSoapApi),
		APIID:         "", // empty to generate a new UUID
		Origin:        models.OriginGatewayAPI,
		DesiredState:  soapDesiredState(soapConfig),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy SOAP API configuration", slog.Any("error", err))
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
		if isSoapAPICreateBadRequest(err) {
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

	s.pushDeployableSoapArtifact(result, correlationID, log)

	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig))
}

// ListSoapAPIs implements ServerInterface.ListSoapAPIs
// (GET /soap-apis)
func (s *APIServer) ListSoapAPIs(w http.ResponseWriter, r *http.Request, params api.ListSoapAPIsParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") || (params.Version != nil && *params.Version != "") || (params.Context != nil && *params.Context != "") || (params.Status != nil && *params.Status != "") {
		s.SearchDeployments(w, r, string(api.SoapAPIKindSoapApi))
		return
	}
	configs, err := s.db.GetAllConfigsByKind(string(api.SoapAPIKindSoapApi))
	if err != nil {
		s.logger.Error("Failed to get SOAP APIs", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve SOAP API configurations",
		})
		return
	}

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"count":    len(items),
		"soapApis": items,
	})
}

// GetSoapAPIById implements ServerInterface.GetSoapAPIById
// (GET /soap-apis/{id})
func (s *APIServer) GetSoapAPIById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(string(api.SoapAPIKindSoapApi), id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("SOAP API configuration not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("SoapApi with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get SOAP API configuration", slog.Any("error", err), slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve configuration",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// UpdateSoapAPI implements ServerInterface.UpdateSoapAPI
// (PUT /soap-apis/{id})
func (s *APIServer) UpdateSoapAPI(w http.ResponseWriter, r *http.Request, id string) {
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

	existing, err := s.db.GetConfigByKindAndHandle(string(api.SoapAPIKindSoapApi), id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("SOAP API configuration not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("SoapApi with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get SOAP API configuration", slog.Any("error", err), slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve configuration",
		})
		return
	}

	// Validate handle match BEFORE persisting anything — mirrors
	// RestAPIService.Update's ordering. Checking this only after
	// DeployAPIConfiguration (which upserts immediately) would let a mismatched
	// body silently rename the stored config to the body's handle before the
	// mismatch is ever reported, orphaning the original path handle even though
	// the client receives a 400.
	var soapConfig api.SoapAPI
	if err := s.parser.Parse(body, r.Header.Get("Content-Type"), &soapConfig); err != nil {
		log.Error("Failed to parse SOAP API configuration", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("failed to parse configuration: %v", err),
		})
		return
	}
	if soapConfig.Metadata.Name != "" && soapConfig.Metadata.Name != id {
		log.Warn("SOAP API update handle mismatch", slog.String("pathHandle", id), slog.String("bodyHandle", soapConfig.Metadata.Name))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("metadata.name '%s' does not match path id '%s'", soapConfig.Metadata.Name, id),
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	// Ensure the deployment uses the existing UUID so DeployAPIConfiguration performs
	// an update (upsert) rather than creating a second artifact.
	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          string(api.SoapAPIKindSoapApi),
		APIID:         existing.UUID,
		Origin:        existing.Origin,
		DesiredState:  soapDesiredState(soapConfig),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update SOAP API configuration", slog.Any("error", err))
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
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to update configuration",
		})
		return
	}

	s.pushDeployableSoapArtifact(result, correlationID, log)

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig))
}

// DeleteSoapAPI implements ServerInterface.DeleteSoapAPI
// (DELETE /soap-apis/{id})
func (s *APIServer) DeleteSoapAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(string(api.SoapAPIKindSoapApi), id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("SOAP API configuration not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("SoapApi with handle '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get SOAP API configuration", slog.Any("error", err), slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve configuration",
		})
		return
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete SOAP API config from database", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)
	s.publishSoapAPIEvent("DELETE", cfg.UUID, correlationID, log)

	// Notify the control plane (DP->CP) that this artifact was deleted via the shared
	// handler path; it keeps the artifact and marks it undeployed.
	s.pushArtifactUndeploy(cfg, log)

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "SoapApi deleted successfully",
		"id":      id,
	})
}

// pushDeployableSoapArtifact pushes a newly created/updated SOAP API to the
// control plane, mirroring RestAPIHandler's create/update push behavior. It is a
// no-op (like the other kinds) when push is disabled, disconnected, or the result
// was a stale/no-op deployment.
func (s *APIServer) pushDeployableSoapArtifact(result *utils.APIDeploymentResult, correlationID string, log *slog.Logger) {
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

// soapDesiredState maps spec.deploymentState onto the stored desired state.
// "undeployed" keeps the configuration but withdraws its routes.
func soapDesiredState(cfg api.SoapAPI) models.DesiredState {
	if cfg.Spec.DeploymentState != nil && *cfg.Spec.DeploymentState == api.SoapAPIDataDeploymentStateUndeployed {
		return models.StateUndeployed
	}
	return models.StateDeployed
}

// publishSoapAPIEvent publishes a delete event to the event hub so all replicas
// (including self) converge through the event listener sync, mirroring
// RestAPIService.publishEvent/MCPDeploymentService.publishMCPProxyEvent.
func (s *APIServer) publishSoapAPIEvent(action, entityID, correlationID string, logger *slog.Logger) {
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventhub.EventTypeAPI,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		logger.Warn("Failed to publish event to event hub",
			slog.String("gateway_id", s.gatewayID),
			slog.String("event_type", string(eventhub.EventTypeAPI)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	}
}

// CreateSoapAPIKey implements ServerInterface.CreateSoapAPIKey
// (POST /soap-apis/{id}/api-keys)
func (s *APIServer) CreateSoapAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "CreateSoapAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		log.Error("Failed to parse request body for SOAP API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindSoapApi,
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
		} else if errors.Is(err, utils.ErrAPIKeyExpirationInPast) || errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create SOAP API key", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

// RevokeSoapAPIKey implements ServerInterface.RevokeSoapAPIKey
// (DELETE /soap-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeSoapAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RevokeSoapAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindSoapApi,
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
			log.Error("Failed to revoke SOAP API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// UpdateSoapAPIKey implements ServerInterface.UpdateSoapAPIKey
// (PUT /soap-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateSoapAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "UpdateSoapAPIKey", correlationID)
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
		Kind:          models.KindSoapApi,
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
		} else if errors.Is(err, utils.ErrAPIKeyExpirationInPast) || errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update SOAP API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RegenerateSoapAPIKey implements ServerInterface.RegenerateSoapAPIKey
// (POST /soap-apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateSoapAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RegenerateSoapAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindSoapApi,
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
		} else if errors.Is(err, utils.ErrAPIKeyExpirationInPast) || errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to regenerate SOAP API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// ListSoapAPIKeys implements ServerInterface.ListSoapAPIKeys
// (GET /soap-apis/{id}/api-keys)
func (s *APIServer) ListSoapAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "ListSoapAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindSoapApi,
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
			log.Error("Failed to list SOAP API keys", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// isSoapAPICreateBadRequest reports whether err is a client-input failure from the
// shared DeployAPIConfiguration path that CreateSoapAPI's other error mappers don't
// recognize. SoapApi goes through the same registered-kind path as GraphQLApi, so the
// failure classes (and their messages) are identical.
func isSoapAPICreateBadRequest(err error) bool {
	return isGraphQLAPICreateBadRequest(err)
}
