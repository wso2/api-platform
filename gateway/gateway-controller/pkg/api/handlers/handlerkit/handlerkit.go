/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package handlerkit holds generic request-handling infrastructure shared by
// every gateway-controller API handler kind (REST, LLM, MCP, subscriptions,
// api-keys, and — in an event-gateway-controller binary — WebSub/WebBroker).
// It exists so that handler code living outside this module (an
// event-gateway-controller binary importing gateway-controller as a library)
// can reuse the same request-body binding, auth-context extraction, and
// control-plane push behavior as core, without duplicating it.
package handlerkit

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/go-httpkit/httputil"
	"gopkg.in/yaml.v3"
)

// errorResponse mirrors the wire shape of the generated api.ErrorResponse
// (status/message fields) without depending on any specific generated
// package's type, so this helper can be reused by handler code built against
// a different generated ServerInterface (e.g. an event-gateway-controller's
// own eventgateway.ErrorResponse).
type errorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// BindRequestBody binds the request body based on the Content-Type header.
// Supports both JSON and YAML content types. Handles Content-Type headers
// case-insensitively and strips parameters (e.g., charset).
func BindRequestBody(r *http.Request, request interface{}) error {
	contentType := r.Header.Get("Content-Type")
	contentType = strings.TrimSpace(contentType)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if contentType == "application/yaml" || contentType == "text/yaml" {
		return yaml.Unmarshal(body, request)
	}
	return json.Unmarshal(body, request)
}

// ExtractAuthenticatedUser extracts and validates the authenticated user from
// the request context. Returns the AuthContext object and handles error
// responses automatically.
func ExtractAuthenticatedUser(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operationName string, correlationID string) (*commonmodels.AuthContext, bool) {
	user, ok := authenticators.GetAuthContext(r)
	if !ok {
		logger.Error("Authentication context not found",
			slog.String("operation", operationName),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusUnauthorized, errorResponse{
			Status:  "error",
			Message: "Authentication context not available",
		})
		return nil, false
	}
	logger.Debug("Authenticated user extracted",
		slog.String("operation", operationName),
		slog.String("user_id", user.UserID),
		slog.Any("roles", user.Roles),
		slog.String("correlation_id", correlationID))
	return &user, true
}

// DeploymentPusher wraps the DP->CP (data-plane to control-plane) artifact
// push behavior shared by every artifact kind's create/update/delete flow.
type DeploymentPusher struct {
	Store              *storage.ConfigStore
	ControlPlaneClient controlplane.ControlPlaneClient
	SystemConfig       *config.Config
}

// WaitForDeploymentAndPush waits for API deployment to complete and pushes it
// to the control plane. This is only relevant for artifacts created directly
// via a gateway endpoint (not from platform API).
//
// minDeployedAt is the DeployedAt of the deployment this push was triggered for.
func (p *DeploymentPusher) WaitForDeploymentAndPush(configID string, correlationID string, minDeployedAt *time.Time, log *slog.Logger) {
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	timeout := time.NewTimer(constants.CPPushDeploymentTimeout)
	ticker := time.NewTicker(constants.CPPushPollInterval)
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			log.Warn("Timeout waiting for API deployment to complete before pushing to control plane",
				slog.String("config_id", configID))
			return

		case <-ticker.C:
			cfg, err := p.Store.Get(configID)
			if err != nil {
				log.Warn("Config not found while waiting for deployment completion",
					slog.String("config_id", configID))
				continue
			}

			// Not deployed yet, or the store still holds a snapshot older than the
			// deployment we were triggered for — keep waiting.
			if cfg.DeployedAt == nil || (minDeployedAt != nil && cfg.DeployedAt.Before(*minDeployedAt)) {
				continue
			}

			log.Info("API deployed successfully, pushing to control plane",
				slog.String("config_id", configID),
				slog.String("displayName", cfg.DisplayName))

			apiID := configID
			deploymentID := cfg.DeploymentID

			if err := p.ControlPlaneClient.PushArtifact(apiID, cfg, deploymentID); err != nil {
				log.Error("Failed to push deployment to control plane",
					slog.String("api_id", apiID),
					slog.Any("error", err))
			} else {
				log.Info("Successfully pushed deployment to control plane",
					slog.String("api_id", apiID))
			}
			return
		}
	}
}

// PushArtifactUndeploy pushes an undeploy notification for a deleted artifact
// to the control plane, if this gateway is connected and deployment sync is
// enabled. It only applies to gateway-originated artifacts.
func (p *DeploymentPusher) PushArtifactUndeploy(cfg *models.StoredConfig, log *slog.Logger) {
	if cfg == nil || cfg.Origin != models.OriginGatewayAPI {
		return
	}
	if p.ControlPlaneClient != nil && p.ControlPlaneClient.IsConnected() && p.SystemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
		undeploy := *cfg
		undeploy.DesiredState = models.StateUndeployed
		go func(uc models.StoredConfig) {
			if err := p.ControlPlaneClient.PushArtifact(uc.UUID, &uc, uc.DeploymentID); err != nil {
				log.Error("Failed to push artifact undeploy to control plane",
					slog.String("artifact_id", uc.UUID), slog.Any("error", err))
			}
		}(undeploy)
	}
}

// EventPublisher wraps event-hub publication for artifact lifecycle changes.
type EventPublisher struct {
	EventHub  eventhub.EventHub
	GatewayID string
}

// PublishEvent publishes a lifecycle event to the event hub so all replicas
// (including self) converge through the event listener sync path.
func (p *EventPublisher) PublishEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	event := eventhub.Event{
		GatewayID:           p.GatewayID,
		OriginatedTimestamp: eventTimestamp(),
		EventType:           eventType,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}

	if err := p.EventHub.PublishEvent(p.GatewayID, event); err != nil {
		logger.Warn("Failed to publish event to event hub",
			slog.String("gateway_id", p.GatewayID),
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	}
}

// eventTimestamp is a seam so tests could stub time if ever needed; today it
// simply returns the current time.
func eventTimestamp() time.Time {
	return time.Now()
}
