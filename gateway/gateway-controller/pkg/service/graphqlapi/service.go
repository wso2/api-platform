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

package graphqlapi

import (
	"log/slog"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// graphQLAPIKind is the artifact kind every operation here reads and writes.
const graphQLAPIKind = string(api.GraphQLAPIKindGraphQLApi)

// GraphQLAPIService creates, updates, reads and deletes GraphQL APIs.
//
// GraphQLApi is wired into the shared APIDeploymentService through
// utils.RegisterKindDeployParser / RegisterKindConfigValidator (see
// pkg/utils/graphql_deployment.go), so Create and Update reuse the same generic
// deployment that RestApi and WebSubApi share.
type GraphQLAPIService struct {
	db                 storage.Storage
	deploymentService  *utils.APIDeploymentService
	parser             *config.Parser
	eventHub           eventhub.EventHub
	gatewayID          string
	controlPlaneClient controlplane.ControlPlaneClient
	systemConfig       *config.Config
	store              *storage.ConfigStore
}

// NewGraphQLAPIService creates a new GraphQLAPIService.
func NewGraphQLAPIService(
	store *storage.ConfigStore,
	db storage.Storage,
	deploymentService *utils.APIDeploymentService,
	parser *config.Parser,
	eventHub eventhub.EventHub,
	gatewayID string,
	controlPlaneClient controlplane.ControlPlaneClient,
	systemConfig *config.Config,
) *GraphQLAPIService {
	if db == nil {
		panic("GraphQLAPIService requires non-nil storage")
	}
	if deploymentService == nil {
		panic("GraphQLAPIService requires APIDeploymentService")
	}
	if parser == nil {
		panic("GraphQLAPIService requires a parser")
	}
	if eventHub == nil {
		panic("GraphQLAPIService requires non-nil EventHub")
	}
	if systemConfig == nil {
		panic("GraphQLAPIService requires non-nil system config")
	}

	return &GraphQLAPIService{
		db:                 db,
		deploymentService:  deploymentService,
		parser:             parser,
		eventHub:           eventHub,
		gatewayID:          gatewayID,
		controlPlaneClient: controlPlaneClient,
		systemConfig:       systemConfig,
		store:              store,
	}
}

// CreateParams holds parameters for the Create operation.
type CreateParams struct {
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// Create deploys a new GraphQL API. A deployment error is returned unchanged, so
// the caller can map it as it would any DeployAPIConfiguration error.
func (s *GraphQLAPIService) Create(params CreateParams) (*models.StoredConfig, error) {
	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          params.Body,
		ContentType:   params.ContentType,
		Kind:          graphQLAPIKind,
		APIID:         "", // empty to generate a new UUID
		Origin:        models.OriginGatewayAPI,
		CorrelationID: params.CorrelationID,
		Logger:        params.Logger,
	})
	if err != nil {
		return nil, err
	}

	s.pushDeployableGraphQLArtifact(result, params.CorrelationID, params.Logger)

	return result.StoredConfig, nil
}

// List returns every GraphQL API.
func (s *GraphQLAPIService) List() ([]*models.StoredConfig, error) {
	return s.db.GetAllConfigsByKind(graphQLAPIKind)
}

// GetByHandle returns the GraphQL API with the given handle.
func (s *GraphQLAPIService) GetByHandle(handle string) (*models.StoredConfig, error) {
	return s.loadByHandle(handle)
}

// UpdateParams holds parameters for the Update operation.
type UpdateParams struct {
	Handle        string
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// Update redeploys an existing GraphQL API under its stored UUID and origin. A
// deployment error is returned unchanged, as in Create.
func (s *GraphQLAPIService) Update(params UpdateParams) (*models.StoredConfig, error) {
	existing, err := s.loadByHandle(params.Handle)
	if err != nil {
		return nil, err
	}

	// Validate handle match BEFORE persisting anything — mirrors
	// RestAPIService.Update's ordering. Checking this only after
	// DeployAPIConfiguration (which upserts immediately) would let a mismatched
	// body silently rename the stored config to the body's handle before the
	// mismatch is ever reported, orphaning the original path handle even though
	// the client receives a 400.
	var graphqlConfig api.GraphQLAPI
	if err := s.parser.Parse(params.Body, params.ContentType, &graphqlConfig); err != nil {
		return nil, &ParseError{Cause: err}
	}
	if graphqlConfig.Metadata.Name != "" && graphqlConfig.Metadata.Name != params.Handle {
		return nil, &HandleMismatchError{PathHandle: params.Handle, YAMLHandle: graphqlConfig.Metadata.Name}
	}

	// Ensure the deployment uses the existing UUID so DeployAPIConfiguration performs
	// an update (upsert) rather than creating a second artifact.
	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          params.Body,
		ContentType:   params.ContentType,
		Kind:          graphQLAPIKind,
		APIID:         existing.UUID,
		Origin:        existing.Origin,
		CorrelationID: params.CorrelationID,
		Logger:        params.Logger,
	})
	if err != nil {
		return nil, err
	}

	s.pushDeployableGraphQLArtifact(result, params.CorrelationID, params.Logger)

	return result.StoredConfig, nil
}

// DeleteParams holds parameters for the Delete operation.
type DeleteParams struct {
	Handle        string
	CorrelationID string
	Logger        *slog.Logger
}

// Delete removes a GraphQL API, publishes the deletion to every replica and tells
// the control plane.
func (s *GraphQLAPIService) Delete(params DeleteParams) error {
	cfg, err := s.loadByHandle(params.Handle)
	if err != nil {
		return err
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		return &DeleteError{Cause: err}
	}

	s.publishGraphQLAPIEvent("DELETE", cfg.UUID, params.CorrelationID, params.Logger)

	// Notify the control plane (DP->CP) that this artifact was deleted; it keeps
	// the artifact and marks it undeployed.
	s.pushArtifactUndeploy(cfg, params.Logger)

	return nil
}

// loadByHandle reads one GraphQL API, returning ErrNotFound when the storage layer
// reports it missing and a LoadError for any other failure.
func (s *GraphQLAPIService) loadByHandle(handle string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(graphQLAPIKind, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, &LoadError{Cause: err}
	}

	return cfg, nil
}

// pushDeployableGraphQLArtifact pushes a newly created/updated GraphQL API to the
// control plane, mirroring RestAPIHandler's create/update push behavior. It is a
// no-op (like the other kinds) when push is disabled, disconnected, or the result
// was a stale/no-op deployment.
func (s *GraphQLAPIService) pushDeployableGraphQLArtifact(result *utils.APIDeploymentResult, correlationID string, log *slog.Logger) {
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

// canPushToControlPlane reports whether DP->CP pushes are enabled: connected,
// not on-prem, and deployment sync turned on.
func (s *GraphQLAPIService) canPushToControlPlane() bool {
	return s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() &&
		!s.controlPlaneClient.IsOnPrem() && s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled
}

// pushArtifactUndeploy tells the control plane a gateway-originated GraphQL API was
// deleted from this gateway. The control plane keeps the artifact and marks it
// undeployed (it can be re-deployed later).
func (s *GraphQLAPIService) pushArtifactUndeploy(cfg *models.StoredConfig, log *slog.Logger) {
	if cfg == nil || cfg.Origin != models.OriginGatewayAPI || !s.canPushToControlPlane() {
		return
	}
	undeploy := *cfg
	undeploy.DesiredState = models.StateUndeployed
	pusher := s.controlPlaneClient
	pusher.SubmitArtifactPush(func() {
		uc := undeploy
		if err := pusher.PushArtifact(uc.UUID, &uc, uc.DeploymentID); err != nil {
			log.Error("Failed to push artifact undeploy to control plane",
				slog.String("artifact_id", uc.UUID), slog.Any("error", err))
		}
	})
}

// waitForDeploymentAndPush waits for the deployment to complete and pushes it to the
// control plane.
//
// minDeployedAt is the DeployedAt of the deployment this push was triggered for.
func (s *GraphQLAPIService) waitForDeploymentAndPush(configID string, correlationID string, minDeployedAt *time.Time, log *slog.Logger) {
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsOnPrem() {
		return
	}
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
			cfg, err := s.store.Get(configID)
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

			if err := s.controlPlaneClient.PushArtifact(apiID, cfg, deploymentID); err != nil {
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

// publishGraphQLAPIEvent publishes a delete event to the event hub so all replicas
// (including self) converge through the event listener sync.
func (s *GraphQLAPIService) publishGraphQLAPIEvent(action, entityID, correlationID string, logger *slog.Logger) {
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
