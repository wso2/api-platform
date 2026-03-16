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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policybuilder "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
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
	Items []api.RestAPIListItem
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
}

// RestAPIService encapsulates business logic for REST API CRUD operations.
type RestAPIService struct {
	store              *storage.ConfigStore
	db                 storage.Storage
	snapshotManager    *xds.SnapshotManager
	policyManager      *policyxds.PolicyManager
	policyDefinitions  map[string]api.PolicyDefinition
	policyDefMu        *sync.RWMutex
	deploymentService  *utils.APIDeploymentService
	apiKeyXDSManager   *apikeyxds.APIKeyStateManager
	controlPlaneClient controlplane.ControlPlaneClient
	routerConfig       *config.RouterConfig
	systemConfig       *config.Config
	httpClient         *http.Client
	parser             *config.Parser
	validator          config.Validator
	logger             *slog.Logger
}

// NewRestAPIService creates a new RestAPIService.
func NewRestAPIService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	policyDefinitions map[string]api.PolicyDefinition,
	policyDefMu *sync.RWMutex,
	deploymentService *utils.APIDeploymentService,
	apiKeyXDSManager *apikeyxds.APIKeyStateManager,
	controlPlaneClient controlplane.ControlPlaneClient,
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	httpClient *http.Client,
	parser *config.Parser,
	validator config.Validator,
	logger *slog.Logger,
) *RestAPIService {
	return &RestAPIService{
		store:              store,
		db:                 db,
		snapshotManager:    snapshotManager,
		policyManager:      policyManager,
		policyDefinitions:  policyDefinitions,
		policyDefMu:        policyDefMu,
		deploymentService:  deploymentService,
		apiKeyXDSManager:   apiKeyXDSManager,
		controlPlaneClient: controlPlaneClient,
		routerConfig:       routerConfig,
		systemConfig:       systemConfig,
		httpClient:         httpClient,
		parser:             parser,
		validator:          validator,
		logger:             logger,
	}
}

// CreateParams holds parameters for the Create operation.
type CreateParams struct {
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// Create deploys a new REST API configuration.
func (s *RestAPIService) Create(params CreateParams) (*CreateResult, error) {
	log := params.Logger

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          params.Body,
		ContentType:   params.ContentType,
		Kind:          "RestApi",
		APIID:         "",
		CorrelationID: params.CorrelationID,
		Logger:        log,
	})
	if err != nil {
		return nil, err
	}

	// Push to control plane asynchronously if connected
	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentPushEnabled {
		go s.waitForDeploymentAndPush(result.StoredConfig.UUID, params.CorrelationID, log)
	}

	// Build and add policy config derived from API configuration
	s.updatePolicyForConfig(result.StoredConfig, result.IsUpdate, log)

	return &CreateResult{
		StoredConfig: result.StoredConfig,
		IsUpdate:     result.IsUpdate,
	}, nil
}

// List returns REST API configurations, optionally filtered.
func (s *RestAPIService) List(params api.ListRestAPIsParams) *ListResult {
	configs := s.store.GetAllByKind(string(api.RestApi))

	items := make([]api.RestAPIListItem, 0, len(configs))
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
		if params.Status != nil && *params.Status != "" && string(cfg.Status) != string(*params.Status) {
			continue
		}

		status := string(cfg.Status)
		items = append(items, api.RestAPIListItem{
			Id:          stringPtr(cfg.Handle),
			DisplayName: stringPtr(cfg.DisplayName),
			Version:     stringPtr(cfg.Version),
			Context:     stringPtr(cfgContext),
			Status:      (*api.RestAPIListItemStatus)(&status),
			CreatedAt:   timePtr(cfg.CreatedAt),
			UpdatedAt:   timePtr(cfg.UpdatedAt),
		})
	}

	return &ListResult{Items: items}
}

// GetByHandle retrieves a REST API by its handle from the database.
func (s *RestAPIService) GetByHandle(handle string) (*GetResult, error) {
	if s.db == nil {
		return nil, ErrDatabaseUnavailable
	}

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

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		return nil, &ValidationError{Errors: validationErrors}
	}

	if s.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	// Check if config exists
	existing, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, params.Handle)
	if err != nil {
		return nil, ErrNotFound
	}

	// Update stored configuration
	now := time.Now()
	existing.Configuration = apiConfig
	existing.SourceConfiguration = apiConfig
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	// Dual-write: database first, then in-memory
	if err := s.db.UpdateConfig(existing); err != nil {
		log.Error("Failed to update config in database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to persist configuration update: %w", err)
	}

	if err := s.store.Update(existing); err != nil {
		return nil, err
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
			log.Error("Failed to update xDS snapshot", slog.Any("error", err))
		}
	}()

	log.Info("API configuration updated",
		slog.String("id", existing.UUID),
		slog.String("handle", params.Handle))

	// Rebuild policy configuration
	s.updatePolicyForConfig(existing, false, log)

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

	if s.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, params.Handle)
	if err != nil {
		return nil, ErrNotFound
	}

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

	// Remove API keys from ConfigStore
	if err := s.store.RemoveAPIKeysByAPI(cfg.UUID); err != nil {
		log.Warn("Failed to remove API keys from ConfigStore",
			slog.String("handle", params.Handle),
			slog.Any("error", err))
	}

	// Remove API keys from policy engine via xDS
	if s.apiKeyXDSManager != nil {
		if restCfg, ok := cfg.Configuration.(api.RestAPI); ok {
			apiId := cfg.UUID
			apiName := restCfg.Spec.DisplayName
			apiVersion := restCfg.Spec.Version

			if err := s.apiKeyXDSManager.RemoveAPIKeysByAPI(apiId, apiName, apiVersion, params.CorrelationID); err != nil {
				log.Warn("Failed to remove API keys from policy engine",
					slog.String("api_id", apiId),
					slog.String("handle", params.Handle),
					slog.String("api_name", apiName),
					slog.String("api_version", apiVersion),
					slog.String("correlation_id", params.CorrelationID),
					slog.Any("error", err))
			} else {
				log.Info("Successfully removed API keys from policy engine",
					slog.String("api_id", apiId),
					slog.String("handle", params.Handle),
					slog.String("api_name", apiName),
					slog.String("api_version", apiVersion),
					slog.String("correlation_id", params.CorrelationID))
			}
		} else {
			log.Warn("Failed to extract API config data for API key removal",
				slog.String("handle", params.Handle))
		}
	}

	// WebSub topic deregistration
	if cfg.Kind == "WebSubApi" {
		if err := s.deregisterWebSubTopics(cfg, log); err != nil {
			return nil, err
		}
	}

	// Delete from in-memory store
	if err := s.store.Delete(cfg.UUID); err != nil {
		log.Error("Failed to delete config from memory store", slog.Any("error", err))
		return nil, fmt.Errorf("failed to delete configuration: %w", err)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
			log.Error("Failed to update xDS snapshot", slog.Any("error", err))
		}
	}()

	log.Info("API configuration deleted",
		slog.String("id", cfg.UUID),
		slog.String("handle", params.Handle))

	// Remove derived policy configuration
	if s.policyManager != nil {
		policyID := cfg.UUID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			log.Warn("Failed to remove derived policy configuration", slog.Any("error", err), slog.String("policy_id", policyID))
		} else {
			log.Info("Derived policy configuration removed", slog.String("policy_id", policyID))
		}
	}

	return &DeleteResult{Handle: params.Handle}, nil
}

// updatePolicyForConfig builds and adds/removes policy config derived from an API configuration.
func (s *RestAPIService) updatePolicyForConfig(cfg *models.StoredConfig, isUpdate bool, log *slog.Logger) {
	if s.policyManager == nil {
		return
	}

	storedPolicy := s.buildStoredPolicyFromAPI(cfg)
	if storedPolicy != nil {
		if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
			log.Error("Failed to add derived policy configuration", slog.Any("error", err))
		} else {
			log.Info("Derived policy configuration added",
				slog.String("policy_id", storedPolicy.ID),
				slog.Int("route_count", len(storedPolicy.Configuration.Routes)))
		}
	} else if isUpdate {
		policyID := cfg.UUID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			if storage.IsPolicyNotFoundError(err) {
				log.Debug("No policy configuration to remove", slog.String("policy_id", policyID))
			} else {
				log.Error("Failed to remove policy configuration",
					slog.Any("error", err),
					slog.String("policy_id", policyID))
			}
		} else {
			log.Info("Derived policy configuration removed (API no longer has policies)",
				slog.String("policy_id", policyID))
		}
	}
}

// buildStoredPolicyFromAPI constructs a StoredPolicyConfig from an API config.
func (s *RestAPIService) buildStoredPolicyFromAPI(cfg *models.StoredConfig) *models.StoredPolicyConfig {
	s.policyDefMu.RLock()
	defsCopy := make(map[string]api.PolicyDefinition, len(s.policyDefinitions))
	for k, v := range s.policyDefinitions {
		defsCopy[k] = v
	}
	s.policyDefMu.RUnlock()

	return policybuilder.DerivePolicyFromAPIConfig(cfg, s.routerConfig, s.systemConfig, defsCopy)
}

// waitForDeploymentAndPush waits for API deployment to complete and pushes it to the control plane.
func (s *RestAPIService) waitForDeploymentAndPush(configID string, correlationID string, log *slog.Logger) {
	if correlationID != "" {
		log = log.With(slog.String("correlation_id", correlationID))
	}

	timeout := time.NewTimer(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
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
				return
			}

			if cfg.Status == models.StatusDeployed {
				log.Info("API deployed successfully, pushing to control plane",
					slog.String("config_id", configID),
					slog.String("displayName", cfg.DisplayName))

				apiID := configID
				deploymentID := ""

				if err := s.controlPlaneClient.PushAPIDeployment(apiID, cfg, deploymentID); err != nil {
					log.Error("Failed to push deployment to control plane",
						slog.String("api_id", apiID),
						slog.Any("error", err))
				} else {
					log.Info("Successfully pushed deployment to control plane",
						slog.String("api_id", apiID))
				}
				return

			} else if cfg.Status == models.StatusFailed {
				log.Warn("API deployment failed, skipping control plane push",
					slog.String("config_id", configID),
					slog.String("displayName", cfg.DisplayName))
				return
			}
		}
	}
}

// deregisterWebSubTopics handles WebSub topic deregistration on delete.
func (s *RestAPIService) deregisterWebSubTopics(cfg *models.StoredConfig, log *slog.Logger) error {
	topicsToUnregister := s.deploymentService.GetTopicsForDelete(*cfg)

	var deregErrs int32
	var wg sync.WaitGroup

	if len(topicsToUnregister) > 0 {
		wg.Add(1)
		go func(list []string) {
			defer wg.Done()
			log.Info("Starting topic deregistration", slog.Int("total_topics", len(list)), slog.String("api_id", cfg.UUID))
			var childWg sync.WaitGroup
			for _, topic := range list {
				childWg.Add(1)
				go func(topic string) {
					defer childWg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
					defer cancel()
					if err := s.deploymentService.UnregisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
						log.Error("Failed to deregister topic from WebSubHub",
							slog.Any("error", err),
							slog.String("topic", topic),
							slog.String("api_id", cfg.UUID))
						atomic.AddInt32(&deregErrs, 1)
					} else {
						log.Info("Successfully deregistered topic from WebSubHub",
							slog.String("topic", topic),
							slog.String("api_id", cfg.UUID))
					}
				}(topic)
			}
			childWg.Wait()
		}(topicsToUnregister)
	}

	wg.Wait()

	log.Info("Topic lifecycle operations completed",
		slog.String("api_id", cfg.UUID),
		slog.Int("deregistered", len(topicsToUnregister)),
		slog.Int("deregister_errors", int(deregErrs)))

	if deregErrs > 0 {
		return fmt.Errorf("topic lifecycle operations failed")
	}
	return nil
}

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
