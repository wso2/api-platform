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

package websubapi

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// UpdateParams holds parameters for the Update operation.
type UpdateParams struct {
	Handle        string
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// UpdateResult holds the result of an Update operation.
type UpdateResult struct {
	Config *models.StoredConfig
}

// WebSubAPIService encapsulates WebSub API update behavior.
type WebSubAPIService struct {
	db                 storage.Storage
	deploymentService  *utils.APIDeploymentService
	controlPlaneClient controlplane.ControlPlaneClient
	systemConfig       *config.Config
	parser             *config.Parser
	validator          config.Validator
	logger             *slog.Logger
	eventHub           eventhub.EventHub
	secretResolver     funcs.SecretResolver
}

// NewWebSubAPIService creates a new WebSubAPIService.
func NewWebSubAPIService(
	db storage.Storage,
	deploymentService *utils.APIDeploymentService,
	controlPlaneClient controlplane.ControlPlaneClient,
	systemConfig *config.Config,
	parser *config.Parser,
	validator config.Validator,
	logger *slog.Logger,
	eventHub eventhub.EventHub,
	secretResolver funcs.SecretResolver,
) *WebSubAPIService {
	if db == nil {
		panic("WebSubAPIService requires non-nil storage")
	}
	if deploymentService == nil {
		panic("WebSubAPIService requires APIDeploymentService")
	}
	if eventHub == nil {
		panic("WebSubAPIService requires non-nil EventHub")
	}
	if systemConfig == nil {
		panic("WebSubAPIService requires non-nil system config")
	}
	if strings.TrimSpace(systemConfig.Controller.Server.GatewayID) == "" {
		panic("WebSubAPIService requires non-empty gateway ID")
	}

	return &WebSubAPIService{
		db:                 db,
		deploymentService:  deploymentService,
		controlPlaneClient: controlPlaneClient,
		systemConfig:       systemConfig,
		parser:             parser,
		validator:          validator,
		logger:             logger,
		eventHub:           eventHub,
		secretResolver:     secretResolver,
	}
}

// Update modifies an existing WebSub API configuration.
func (s *WebSubAPIService) Update(params UpdateParams) (*UpdateResult, error) {
	log := params.Logger

	var apiConfig api.WebSubAPI
	if err := s.parser.Parse(params.Body, params.ContentType, &apiConfig); err != nil {
		return nil, &ParseError{Cause: err}
	}

	if apiConfig.Metadata.Name != "" && apiConfig.Metadata.Name != params.Handle {
		return nil, &HandleMismatchError{
			PathHandle: params.Handle,
			YAMLHandle: apiConfig.Metadata.Name,
		}
	}

	existing, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, params.Handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to retrieve existing websub api: %w", err)
	}

	desiredState := models.StateDeployed
	if apiConfig.Spec.DeploymentState != nil &&
		*apiConfig.Spec.DeploymentState == api.WebhookAPIDataDeploymentStateUndeployed {
		desiredState = models.StateUndeployed
	}

	if desiredState != models.StateUndeployed {
		result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
			Data:          params.Body,
			ContentType:   params.ContentType,
			Kind:          "WebSubApi",
			APIID:         existing.UUID,
			Origin:        models.OriginGatewayAPI,
			CorrelationID: params.CorrelationID,
			Logger:        log,
		})
		if err != nil {
			return nil, err
		}
		return &UpdateResult{Config: result.StoredConfig}, nil
	}

	if apiConfig.Metadata.Name == "" {
		apiConfig.Metadata.Name = params.Handle
	}

	existing.Configuration = apiConfig
	existing.SourceConfiguration = apiConfig

	if err := templateengine.RenderSpec(existing, s.secretResolver, log); err != nil {
		return nil, err
	}

	renderedConfig, ok := existing.Configuration.(api.WebSubAPI)
	if !ok {
		return nil, fmt.Errorf("failed to render websub api configuration")
	}

	validationErrors := s.validator.Validate(&renderedConfig)
	if len(validationErrors) > 0 {
		return nil, &ValidationError{Errors: validationErrors}
	}

	if err := s.validateArtifactConflicts(existing.UUID, renderedConfig.Spec.DisplayName, renderedConfig.Spec.Version, existing.Handle); err != nil {
		return nil, err
	}

	now := time.Now()
	existing.DisplayName = renderedConfig.Spec.DisplayName
	existing.Version = renderedConfig.Spec.Version
	existing.DesiredState = desiredState
	existing.UpdatedAt = now

	truncatedNow := now.Truncate(time.Millisecond)
	existing.DeployedAt = &truncatedNow

	if existing.Origin == models.OriginGatewayAPI {
		existing.CPSyncStatus = models.CPSyncStatusPending
	}

	if err := s.db.UpdateConfig(existing); err != nil {
		log.Error("Failed to update WebSub API config in database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to persist configuration update: %w", err)
	}

	s.publishEvent(eventhub.EventTypeAPI, "UPDATE", existing.UUID, params.CorrelationID, log)

	if existing.Origin == models.OriginGatewayAPI && s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.controlPlaneClient.IsOnPrem() {
		go func() {
			if err := s.controlPlaneClient.SyncArtifactsToOnPremAPIM(s.controlPlaneClient.GetAPIMConfig()); err != nil {
				log.Error("Failed to sync WebSub API to on-prem APIM", slog.Any("error", err))
			}
		}()
	}

	log.Info("WebSub API configuration updated",
		slog.String("id", existing.UUID),
		slog.String("handle", params.Handle),
		slog.String("desired_state", string(desiredState)))

	return &UpdateResult{Config: existing}, nil
}

func (s *WebSubAPIService) validateArtifactConflicts(currentID, displayName, version, handle string) error {
	existingByNameVersion, err := s.db.GetConfigByKindNameAndVersion(models.KindWebSubApi, displayName, version)
	if err == nil {
		if existingByNameVersion != nil && existingByNameVersion.UUID != currentID {
			return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists",
				storage.ErrConflict, displayName, version)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing WebSubApi name/version conflict: %w", err)
	}

	existingByHandle, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err == nil {
		if existingByHandle != nil && existingByHandle.UUID != currentID {
			return fmt.Errorf("%w: configuration with handle '%s' already exists",
				storage.ErrConflict, handle)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing WebSubApi handle conflict: %w", err)
	}

	return nil
}

func (s *WebSubAPIService) publishEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	gatewayID := strings.TrimSpace(s.systemConfig.Controller.Server.GatewayID)
	event := eventhub.Event{
		GatewayID:           gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventType,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}

	if err := s.eventHub.PublishEvent(gatewayID, event); err != nil {
		logger.Warn("Failed to publish event to event hub",
			slog.String("gateway_id", gatewayID),
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
		return
	}

	logger.Debug("Published event to event hub",
		slog.String("gateway_id", gatewayID),
		slog.String("event_type", string(eventType)),
		slog.String("action", action),
		slog.String("entity_id", entityID))
}
