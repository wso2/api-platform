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

// Package handler implements the eventgateway.ServerInterface (WebSub/WebBroker
// management API), built against gateway-controller (core) as a library.
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers/handlerkit"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"

	commonmodels "github.com/wso2/api-platform/common/models"
	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
	eventgatewayconfig "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/webhooksecretservice"
)

// toManagementAPIKeyCreationRequest converts the eventgateway-local
// APIKeyCreationRequest (bound from the request body against this module's
// own generated spec) into core's management.APIKeyCreationRequest, which
// utils.APIKeyCreationParams/APIKeyUpdateParams require. The two types are
// generated from identical schema definitions (core's spec duplicates the
// shared api-key schemas into this module's spec — see
// api/eventgateway-openapi.yaml) but a plain Go struct conversion isn't legal
// here since the nested ExpiresIn.Unit field uses a package-local named type
// on each side, so each field is copied explicitly instead.
func toManagementAPIKeyCreationRequest(r eventgateway.APIKeyCreationRequest) api.APIKeyCreationRequest {
	out := api.APIKeyCreationRequest{
		ApiKey:        r.ApiKey,
		ExpiresAt:     r.ExpiresAt,
		ExternalRefId: r.ExternalRefId,
		Issuer:        r.Issuer,
		MaskedApiKey:  r.MaskedApiKey,
		Name:          r.Name,
	}
	if r.ExpiresIn != nil {
		out.ExpiresIn = &struct {
			Duration int                                          `json:"duration" yaml:"duration"`
			Unit     api.APIKeyCreationRequestExpiresInUnit `json:"unit" yaml:"unit"`
		}{
			Duration: r.ExpiresIn.Duration,
			Unit:     api.APIKeyCreationRequestExpiresInUnit(r.ExpiresIn.Unit),
		}
	}
	return out
}

// toManagementAPIKeyRegenerationRequest converts the eventgateway-local
// APIKeyRegenerationRequest into core's management.APIKeyRegenerationRequest.
// See toManagementAPIKeyCreationRequest for why field-by-field copying is
// used instead of a direct struct conversion.
func toManagementAPIKeyRegenerationRequest(r eventgateway.APIKeyRegenerationRequest) api.APIKeyRegenerationRequest {
	out := api.APIKeyRegenerationRequest{
		ExpiresAt: r.ExpiresAt,
	}
	if r.ExpiresIn != nil {
		out.ExpiresIn = &struct {
			Duration int                                              `json:"duration" yaml:"duration"`
			Unit     api.APIKeyRegenerationRequestExpiresInUnit `json:"unit" yaml:"unit"`
		}{
			Duration: r.ExpiresIn.Duration,
			Unit:     api.APIKeyRegenerationRequestExpiresInUnit(r.ExpiresIn.Unit),
		}
	}
	return out
}

// DeploymentSearcher is satisfied by core's *handlers.APIServer. WebSub/WebBroker
// list endpoints delegate filtered search queries to it rather than
// duplicating the generic search-by-kind implementation.
type DeploymentSearcher interface {
	SearchDeployments(w http.ResponseWriter, r *http.Request, kind string)
}

// WebSubServer implements eventgateway.ServerInterface (WebSub and WebBroker
// management endpoints).
type WebSubServer struct {
	store               *storage.ConfigStore
	db                  storage.Storage
	deploymentService   *utils.APIDeploymentService
	apiKeyService        *utils.APIKeyService
	controlPlaneClient  controlplane.ControlPlaneClient
	systemConfig        *config.Config
	eventGatewayConfig  eventgatewayconfig.EventGatewayConfig
	httpClient          *http.Client
	logger              *slog.Logger
	webhookSecretService *webhooksecretservice.WebhookSecretService
	deploymentSearcher  DeploymentSearcher

	eventHub  eventhub.EventHub
	gatewayID string
}

// Deps bundles the dependencies WebSubServer needs, sourced from gateway-controller
// (core) constructors plus this module's own webhook-secret and config packages.
type Deps struct {
	Store                *storage.ConfigStore
	DB                   storage.Storage
	DeploymentService    *utils.APIDeploymentService
	APIKeyService        *utils.APIKeyService
	ControlPlaneClient   controlplane.ControlPlaneClient
	SystemConfig         *config.Config
	EventGatewayConfig   eventgatewayconfig.EventGatewayConfig
	HTTPClient           *http.Client
	Logger               *slog.Logger
	WebhookSecretService *webhooksecretservice.WebhookSecretService
	DeploymentSearcher   DeploymentSearcher
	EventHub             eventhub.EventHub
	GatewayID            string
}

// NewWebSubServer creates a new WebSubServer.
func NewWebSubServer(d Deps) *WebSubServer {
	return &WebSubServer{
		store:                d.Store,
		db:                   d.DB,
		deploymentService:    d.DeploymentService,
		apiKeyService:        d.APIKeyService,
		controlPlaneClient:   d.ControlPlaneClient,
		systemConfig:         d.SystemConfig,
		eventGatewayConfig:   d.EventGatewayConfig,
		httpClient:           d.HTTPClient,
		logger:               d.Logger,
		webhookSecretService: d.WebhookSecretService,
		deploymentSearcher:   d.DeploymentSearcher,
		eventHub:             d.EventHub,
		gatewayID:            d.GatewayID,
	}
}

func (s *WebSubServer) deploymentPusher() *handlerkit.DeploymentPusher {
	return &handlerkit.DeploymentPusher{
		Store:              s.store,
		ControlPlaneClient: s.controlPlaneClient,
		SystemConfig:       s.systemConfig,
	}
}

func (s *WebSubServer) waitForDeploymentAndPush(configID string, correlationID string, minDeployedAt *time.Time, log *slog.Logger) {
	s.deploymentPusher().WaitForDeploymentAndPush(configID, correlationID, minDeployedAt, log)
}

func (s *WebSubServer) publishWebSubEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	(&handlerkit.EventPublisher{EventHub: s.eventHub, GatewayID: s.gatewayID}).PublishEvent(eventType, action, entityID, correlationID, logger)
}

func (s *WebSubServer) bindRequestBody(r *http.Request, request interface{}) error {
	return handlerkit.BindRequestBody(r, request)
}

func (s *WebSubServer) extractAuthenticatedUser(w http.ResponseWriter, r *http.Request, operationName string, correlationID string) (*commonmodels.AuthContext, bool) {
	return handlerkit.ExtractAuthenticatedUser(w, r, s.logger, operationName, correlationID)
}
