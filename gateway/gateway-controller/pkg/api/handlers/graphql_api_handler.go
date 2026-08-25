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
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/go-httpkit/httputil"
)

// GraphQLAPI CRUD handlers, implemented directly on *APIServer (mirroring
// mcp_proxy_handler.go's pattern rather than restapi's own service package) since
// GraphQLApi has no operations/upstreamDefinitions/vhosts to warrant a bespoke
// service layer: Create/Update reuse the same generic s.deploymentService that
// RestApi/WebSubApi already share (GraphQLApi is wired into it via
// utils.RegisterKindDeployParser/RegisterKindConfigValidator — see
// pkg/utils/graphql_deployment.go — not a hardcoded case in api_deployment.go).

// CreateGraphQLAPI implements ServerInterface.CreateGraphQLAPI
// (POST /graphql-apis)
func (s *APIServer) CreateGraphQLAPI(w http.ResponseWriter, r *http.Request) {
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
		Kind:          string(api.GraphQLAPIKindGraphQLApi),
		APIID:         "", // empty to generate a new UUID
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy GraphQL API configuration", slog.Any("error", err))
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
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	s.pushDeployableGraphQLArtifact(result, correlationID, log)

	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig))
}

// ListGraphQLAPIs implements ServerInterface.ListGraphQLAPIs
// (GET /graphql-apis)
func (s *APIServer) ListGraphQLAPIs(w http.ResponseWriter, r *http.Request, params api.ListGraphQLAPIsParams) {
	configs, err := s.db.GetAllConfigsByKind(string(api.GraphQLAPIKindGraphQLApi))
	if err != nil {
		s.logger.Error("Failed to get GraphQL APIs", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve GraphQL API configurations",
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
				s.logger.Error("Failed to get context for GraphQL API config", slog.Any("error", err), slog.String("uuid", cfg.UUID))
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
		"status":      "success",
		"count":       len(items),
		"graphqlApis": items,
	})
}

// GetGraphQLAPIById implements ServerInterface.GetGraphQLAPIById
// (GET /graphql-apis/{id})
func (s *APIServer) GetGraphQLAPIById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(string(api.GraphQLAPIKindGraphQLApi), id)
	if err != nil {
		log.Warn("GraphQL API configuration not found", slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("GraphQLApi with handle '%s' not found", id),
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// UpdateGraphQLAPI implements ServerInterface.UpdateGraphQLAPI
// (PUT /graphql-apis/{id})
func (s *APIServer) UpdateGraphQLAPI(w http.ResponseWriter, r *http.Request, id string) {
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

	existing, err := s.db.GetConfigByKindAndHandle(string(api.GraphQLAPIKindGraphQLApi), id)
	if err != nil {
		log.Warn("GraphQL API configuration not found", slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("GraphQLApi with handle '%s' not found", id),
		})
		return
	}

	// Validate handle match BEFORE persisting anything — mirrors
	// RestAPIService.Update's ordering. Checking this only after
	// DeployAPIConfiguration (which upserts immediately) would let a mismatched
	// body silently rename the stored config to the body's handle before the
	// mismatch is ever reported, orphaning the original path handle even though
	// the client receives a 400.
	var graphqlConfig api.GraphQLAPI
	if err := s.parser.Parse(body, r.Header.Get("Content-Type"), &graphqlConfig); err != nil {
		log.Error("Failed to parse GraphQL API configuration", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("failed to parse configuration: %v", err),
		})
		return
	}
	if graphqlConfig.Metadata.Name != "" && graphqlConfig.Metadata.Name != id {
		log.Warn("GraphQL API update handle mismatch", slog.String("pathHandle", id), slog.String("bodyHandle", graphqlConfig.Metadata.Name))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("metadata.name '%s' does not match path id '%s'", graphqlConfig.Metadata.Name, id),
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	// Ensure the deployment uses the existing UUID so DeployAPIConfiguration performs
	// an update (upsert) rather than creating a second artifact.
	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          string(api.GraphQLAPIKindGraphQLApi),
		APIID:         existing.UUID,
		Origin:        existing.Origin,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update GraphQL API configuration", slog.Any("error", err))
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
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	s.pushDeployableGraphQLArtifact(result, correlationID, log)

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig))
}

// DeleteGraphQLAPI implements ServerInterface.DeleteGraphQLAPI
// (DELETE /graphql-apis/{id})
func (s *APIServer) DeleteGraphQLAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(string(api.GraphQLAPIKindGraphQLApi), id)
	if err != nil {
		log.Warn("GraphQL API configuration not found", slog.String("handle", id))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("GraphQLApi with handle '%s' not found", id),
		})
		return
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete GraphQL API config from database", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)
	s.publishGraphQLAPIEvent("DELETE", cfg.UUID, correlationID, log)

	// Notify the control plane (DP->CP) that this artifact was deleted via the shared
	// handler path; it keeps the artifact and marks it undeployed.
	s.pushArtifactUndeploy(cfg, log)

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "GraphQLApi deleted successfully",
		"id":      id,
	})
}

// pushDeployableGraphQLArtifact pushes a newly created/updated GraphQL API to the
// control plane, mirroring RestAPIHandler's create/update push behavior. It is a
// no-op (like the other kinds) when push is disabled, disconnected, or the result
// was a stale/no-op deployment.
func (s *APIServer) pushDeployableGraphQLArtifact(result *utils.APIDeploymentResult, correlationID string, log *slog.Logger) {
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

// publishGraphQLAPIEvent publishes a delete event to the event hub so all replicas
// (including self) converge through the event listener sync, mirroring
// RestAPIService.publishEvent/MCPDeploymentService.publishMCPProxyEvent.
func (s *APIServer) publishGraphQLAPIEvent(action, entityID, correlationID string, logger *slog.Logger) {
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

// mapValidationError maps a *utils.ValidationErrorListError to a 400 response with
// structured field errors, mirroring RestAPIHandler.mapCreateError's handling of the
// same error type.
func mapValidationError(w http.ResponseWriter, err error) bool {
	var validationErr *utils.ValidationErrorListError
	if !errors.As(err, &validationErr) {
		return false
	}
	apiErrors := make([]api.ValidationError, len(validationErr.Errors))
	for i, e := range validationErr.Errors {
		apiErrors[i] = api.ValidationError{
			Field:   stringPtr(e.Field),
			Message: stringPtr(e.Message),
		}
	}
	httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
		Status:  "error",
		Message: "Configuration validation failed",
		Errors:  &apiErrors,
	})
	return true
}
