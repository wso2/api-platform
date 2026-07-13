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

package restapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// CreateResult holds the result of a Create operation.
type CreateResult struct {
	StoredConfig *models.StoredConfig
	IsUpdate     bool
}

// ListResult holds the result of a List operation.
type ListResult struct {
	Items []*models.StoredConfig
}

// GetResult holds the result of a GetByHandle operation.
type GetResult struct {
	Config *models.StoredConfig
}

// UpdateResult holds the result of an Update operation.
type UpdateResult struct {
	Config *models.StoredConfig
}

// DeleteResult holds the result of a Delete operation.
type DeleteResult struct {
	Handle string
	// Config is the stored configuration that was deleted, so the handler can notify
	// the control plane (undeploy) for gateway-originated artifacts.
	Config *models.StoredConfig
}

// RestAPIService encapsulates business logic for REST API CRUD operations.
type RestAPIService struct {
	store              *storage.ConfigStore
	db                 storage.Storage
	snapshotManager    *xds.SnapshotManager
	policyManager      *policyxds.PolicyManager
	deploymentService  *utils.APIDeploymentService
	apiKeyXDSManager   *apikeyxds.APIKeyStateManager
	controlPlaneClient controlplane.ControlPlaneClient
	routerConfig       *config.RouterConfig
	systemConfig       *config.Config
	httpClient         *http.Client
	parser             *config.Parser
	validator          config.Validator
	logger             *slog.Logger
	eventHub           eventhub.EventHub
	secretResolver     funcs.SecretResolver

	// webSubTopicDeregistrar is an optional hook, set by an event-gateway-controller
	// binary, that deregisters WebSub hub topics for a deleted WebSubApi config.
	// It is nil (and skipped) when event-gateway support is not compiled in.
	webSubTopicDeregistrar func(cfg *models.StoredConfig, log *slog.Logger) error
}

// SetWebSubTopicDeregistrar registers the event-gateway hook invoked from Delete
// when a WebSubApi config is removed. Passing nil (the default) disables it.
func (s *RestAPIService) SetWebSubTopicDeregistrar(fn func(cfg *models.StoredConfig, log *slog.Logger) error) {
	s.webSubTopicDeregistrar = fn
}

// NewRestAPIService creates a new RestAPIService.
func NewRestAPIService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	deploymentService *utils.APIDeploymentService,
	apiKeyXDSManager *apikeyxds.APIKeyStateManager,
	controlPlaneClient controlplane.ControlPlaneClient,
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	httpClient *http.Client,
	parser *config.Parser,
	validator config.Validator,
	logger *slog.Logger,
	eventHub eventhub.EventHub,
	secretResolver funcs.SecretResolver,
) *RestAPIService {
	if db == nil {
		panic("RestAPIService requires non-nil storage")
	}
	if eventHub == nil {
		panic("RestAPIService requires non-nil EventHub")
	}
	if deploymentService == nil {
		panic("RestAPIService requires APIDeploymentService")
	}
	if systemConfig == nil {
		panic("RestAPIService requires non-nil system config")
	}
	if strings.TrimSpace(systemConfig.Controller.Server.GatewayID) == "" {
		panic("RestAPIService requires non-empty gateway ID")
	}

	return &RestAPIService{
		store:              store,
		db:                 db,
		snapshotManager:    snapshotManager,
		policyManager:      policyManager,
		deploymentService:  deploymentService,
		apiKeyXDSManager:   apiKeyXDSManager,
		controlPlaneClient: controlPlaneClient,
		routerConfig:       routerConfig,
		systemConfig:       systemConfig,
		httpClient:         httpClient,
		parser:             parser,
		validator:          validator,
		logger:             logger,
		eventHub:           eventHub,
		secretResolver:     secretResolver,
	}
}

// CreateParams holds parameters for the Create operation.
type CreateParams struct {
	Body          []byte
	ContentType   string
	CorrelationID string
	Kind          string
	Logger        *slog.Logger
}

// Create deploys a new REST API configuration.
func (s *RestAPIService) Create(params CreateParams) (*CreateResult, error) {
	log := params.Logger

	kind := params.Kind
	if kind == "" {
		kind = "RestApi"
	}

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          params.Body,
		ContentType:   params.ContentType,
		Kind:          kind,
		APIID:         "",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: params.CorrelationID,
		Logger:        log,
	})
	if err != nil {
		return nil, err
	}

	if !result.IsStale {
		// Trigger bottom-up sync immediately if connected and control plane type is on-prem
		if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.controlPlaneClient.IsOnPrem() &&
			s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
			go func() {
				if err := s.controlPlaneClient.SyncArtifactsToOnPremAPIM(s.controlPlaneClient.GetAPIMConfig()); err != nil {
					log.Error("Failed to sync API to on-prem APIM", slog.Any("error", err))
				}
			}()
		}

		// Push to control plane asynchronously if connected
		if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
			go s.waitForDeploymentAndPush(result.StoredConfig.UUID, params.CorrelationID, result.StoredConfig.DeployedAt, log)
		}
	}

	return &CreateResult{
		StoredConfig: result.StoredConfig,
		IsUpdate:     result.IsUpdate,
	}, nil
}

func (s *RestAPIService) validateArtifactConflicts(kind, currentID, displayName, version, handle string) error {
	existingByNameVersion, err := s.db.GetConfigByKindNameAndVersion(kind, displayName, version)
	if err == nil {
		if existingByNameVersion != nil && existingByNameVersion.UUID != currentID {
			return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists",
				storage.ErrConflict, displayName, version)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing %s name/version conflict: %w", kind, err)
	}

	existingByHandle, err := s.db.GetConfigByKindAndHandle(kind, handle)
	if err == nil {
		if existingByHandle != nil && existingByHandle.UUID != currentID {
			return fmt.Errorf("%w: configuration with handle '%s' already exists",
				storage.ErrConflict, handle)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing %s handle conflict: %w", kind, err)
	}

	return nil
}

// List returns REST API configurations, optionally filtered.
func (s *RestAPIService) List(params api.ListRestAPIsParams) (*ListResult, error) {
	configs, err := s.db.GetAllConfigsByKind(string(api.RestAPIKindRestApi))
	if err != nil {
		s.logger.Error("Failed to get APIs", slog.Any("error", err))
		return nil, fmt.Errorf("Failed to retrieve API configurations")
	}

	items := make([]*models.StoredConfig, 0, len(configs))
	for _, cfg := range configs {
		// Apply filters when present
		if params.DisplayName != nil && *params.DisplayName != "" && cfg.DisplayName != *params.DisplayName {
			continue
		}
		if params.Version != nil && *params.Version != "" && cfg.Version != *params.Version {
			continue
		}
		cfgContext, err := cfg.GetContext()
		if err != nil {
			s.logger.Error("Failed to get context for API config", slog.Any("error", err), slog.String("uuid", cfg.UUID))
			continue
		}
		if params.Context != nil && *params.Context != "" && cfgContext != *params.Context {
			continue
		}
		if params.Status != nil && *params.Status != "" && string(cfg.DesiredState) != string(*params.Status) {
			continue
		}

		items = append(items, cfg)
	}

	return &ListResult{Items: items}, nil
}

// GetByHandle retrieves a REST API by its handle from the database.
func (s *RestAPIService) GetByHandle(handle string) (*GetResult, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, handle)
	if err != nil {
		return nil, ErrNotFound
	}

	return &GetResult{Config: cfg}, nil
}

// UpdateParams holds parameters for the Update operation.
type UpdateParams struct {
	Handle        string
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// Update modifies an existing REST API configuration.
func (s *RestAPIService) Update(params UpdateParams) (*UpdateResult, error) {
	log := params.Logger

	// Parse configuration
	var apiConfig api.RestAPI
	if err := s.parser.Parse(params.Body, params.ContentType, &apiConfig); err != nil {
		return nil, &ParseError{Cause: err}
	}

	// Validate handle match
	if apiConfig.Metadata.Name != "" && apiConfig.Metadata.Name != params.Handle {
		return nil, &HandleMismatchError{
			PathHandle: params.Handle,
			YAMLHandle: apiConfig.Metadata.Name,
		}
	}

	// Extract deploymentState from spec (defaults to "deployed" if not specified)
	desiredState := models.StateDeployed
	if apiConfig.Spec.DeploymentState != nil && *apiConfig.Spec.DeploymentState == api.APIConfigDataDeploymentStateUndeployed {
		desiredState = models.StateUndeployed
	}

	// Check if config exists
	existing, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, params.Handle)
	if err != nil {
		return nil, ErrNotFound
	}

	// Populate existing with the incoming config so RenderSpec can operate on it.
	existing.Configuration = apiConfig
	existing.SourceConfiguration = apiConfig

	// Render template expressions before validation so the validator sees resolved values
	// (e.g. {{ env "BACKEND_URL" }} → actual URL).
	if err := templateengine.RenderSpec(existing, s.secretResolver, log); err != nil {
		return nil, err
	}

	// Validate configuration against resolved values
	renderedConfig := existing.Configuration.(api.RestAPI)
	validationErrors := s.validator.Validate(&renderedConfig)
	if len(validationErrors) > 0 {
		return nil, &ValidationError{Errors: validationErrors}
	}

	if err := s.validateArtifactConflicts(models.KindRestApi, existing.UUID, renderedConfig.Spec.DisplayName, renderedConfig.Spec.Version, existing.Handle); err != nil {
		return nil, err
	}

	// Update stored configuration
	now := time.Now()
	truncatedNow := now.Truncate(time.Millisecond)
	existing.DisplayName = renderedConfig.Spec.DisplayName
	existing.Version = renderedConfig.Spec.Version
	existing.DesiredState = desiredState
	existing.UpdatedAt = now
	existing.DeployedAt = &truncatedNow

	if desiredState == models.StateUndeployed {
		log.Info("Undeploying API configuration",
			slog.String("id", existing.UUID),
			slog.String("handle", params.Handle))
	}

	if existing.Origin == models.OriginGatewayAPI {
		existing.CPSyncStatus = models.CPSyncStatusPending
	}

	// Dual-write: database first, then in-memory
	if err := s.db.UpdateConfig(existing); err != nil {
		log.Error("Failed to update config in database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to persist configuration update: %w", err)
	}

	s.publishEvent(eventhub.EventTypeAPI, "UPDATE", existing.UUID, params.CorrelationID, log)

	// Trigger bottom-up sync if enabled and connected
	if existing.Origin == models.OriginGatewayAPI && s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() &&
		s.controlPlaneClient.IsOnPrem() && s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
		go func() {
			if err := s.controlPlaneClient.SyncArtifactsToOnPremAPIM(s.controlPlaneClient.GetAPIMConfig()); err != nil {
				log.Error("Failed to sync API to on-prem APIM", slog.Any("error", err))
			}
		}()
	}

	// Push to control plane asynchronously if connected
	if existing.Origin == models.OriginGatewayAPI && s.controlPlaneClient != nil &&
		s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
		go s.waitForDeploymentAndPush(existing.UUID, params.CorrelationID, existing.DeployedAt, log)
	}

	log.Info("API configuration updated",
		slog.String("id", existing.UUID),
		slog.String("handle", params.Handle),
		slog.String("desired_state", string(desiredState)))

	return &UpdateResult{Config: existing}, nil
}

// DeleteParams holds parameters for the Delete operation.
type DeleteParams struct {
	Handle        string
	CorrelationID string
	Logger        *slog.Logger
}

// Delete removes a REST API configuration.
func (s *RestAPIService) Delete(params DeleteParams) (*DeleteResult, error) {
	log := params.Logger

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, params.Handle)
	if err != nil {
		return nil, ErrNotFound
	}

	// For gateway-originated APIs that were successfully synced to on-prem APIM,
	// undeploy the revision from APIM before removing the local record.
	s.undeployFromAPIMBeforeDelete(cfg, log)

	// Delete from database
	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete config from database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to delete configuration: %w", err)
	}

	// Delete associated API keys from database
	if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		log.Warn("Failed to remove API keys from database",
			slog.String("handle", params.Handle),
			slog.Any("error", err))
	}
	//

	// WebSub topic deregistration (event-gateway-controller only)
	if cfg.Kind == "WebSubApi" && s.webSubTopicDeregistrar != nil {
		if err := s.webSubTopicDeregistrar(cfg, log); err != nil {
			return nil, err
		}
	}

	// Publish deletion event so all replicas (including self) converge through event listener sync.
	s.publishEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, params.CorrelationID, log)

	log.Info("API configuration deleted",
		slog.String("id", cfg.UUID),
		slog.String("handle", params.Handle))

	return &DeleteResult{Handle: params.Handle, Config: cfg}, nil
}

// undeployFromAPIMBeforeDelete undeploys a gateway-originated API from on-prem APIM
// synchronously before the local record is deleted. This will be skipped when:
//   - the API was not created on the gateway (origin != gateway_api)
//   - the API was never successfully synced to APIM (CPSyncInfo is empty)
//   - the control plane client is not available or not in on-prem mode
//
// Failures are logged as warnings — the local delete proceeds regardless.
func (s *RestAPIService) undeployFromAPIMBeforeDelete(cfg *models.StoredConfig, log *slog.Logger) {
	if cfg.Origin != models.OriginGatewayAPI || cfg.CPSyncInfo == "" {
		return
	}
	if s.controlPlaneClient == nil || !s.controlPlaneClient.IsOnPrem() {
		return
	}
	apimCfg := s.controlPlaneClient.GetAPIMConfig()
	if apimCfg == nil {
		return
	}

	// Mark the record as pending undeploy. SyncArtifactsToOnPremAPIM will handle the undeployment.
	cfg.DesiredState = models.StateUndeployed
	if err := s.db.UpdateConfig(cfg); err != nil {
		log.Warn("Failed to set desired_state=undeployed before APIM undeploy",
			slog.String("uuid", cfg.UUID), slog.Any("error", err))
		return
	}
	// Preserve CPSyncInfo so the undeploy sync can extract the APIM API ID and revision.
	if err := s.db.UpdateCPSyncStatus(cfg.UUID, cfg.CPArtifactID, models.CPSyncStatusPending, cfg.CPSyncInfo); err != nil {
		log.Warn("Failed to set cp_sync_status=pending before APIM undeploy",
			slog.String("uuid", cfg.UUID), slog.Any("error", err))
		return
	}

	if err := s.controlPlaneClient.SyncArtifactsToOnPremAPIM(apimCfg); err != nil {
		log.Error("Failed to undeploy API from on-prem APIM before deletion",
			slog.String("uuid", cfg.UUID),
			slog.String("handle", cfg.Handle),
			slog.Any("error", err))
	}
}

// updatePolicyForConfig upserts the runtime config for an API into the policy engine.
func (s *RestAPIService) updatePolicyForConfig(cfg *models.StoredConfig, log *slog.Logger) {
	if s.policyManager == nil {
		return
	}
	if err := s.policyManager.UpsertAPIConfig(cfg); err != nil {
		log.Error("Failed to upsert runtime config", slog.Any("error", err))
	}
}

// waitForDeploymentAndPush waits for API deployment to complete and pushes it to the control plane.
//
// minDeployedAt is the DeployedAt of the deployment this push was triggered for.
func (s *RestAPIService) waitForDeploymentAndPush(configID string, correlationID string, minDeployedAt *time.Time, log *slog.Logger) {
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

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// publishEvent publishes an event to the event hub
func (s *RestAPIService) publishEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
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
	} else {
		logger.Debug("Published event to event hub",
			slog.String("gateway_id", gatewayID),
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID))
	}
}
