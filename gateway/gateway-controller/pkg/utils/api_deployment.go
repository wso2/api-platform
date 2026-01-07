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

package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

// APIDeploymentParams contains parameters for API deployment operations
type APIDeploymentParams struct {
	Data          []byte      // Raw configuration data (YAML/JSON)
	ContentType   string      // Content type for parsing
	APIID         string      // API ID (if provided, used for updates; if empty, generates new UUID)
	CorrelationID string      // Correlation ID for tracking
	Logger        *zap.Logger // Logger instance
}

// APIDeploymentResult contains the result of API deployment
type APIDeploymentResult struct {
	StoredConfig *models.StoredConfig
	IsUpdate     bool
}

// APIUpdateParams contains parameters for API update operations
type APIUpdateParams struct {
	Handle        string      // API handle from URL path
	Data          []byte      // Raw configuration data (YAML/JSON)
	ContentType   string      // Content type for parsing
	CorrelationID string      // Correlation ID for tracking
	Logger        *zap.Logger // Logger instance
}

// Custom error types for HTTP status code mapping

// NotFoundError indicates the requested resource was not found
type NotFoundError struct {
	Handle string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("API configuration with handle '%s' not found", e.Handle)
}

// HandleMismatchError indicates the handle in the URL doesn't match the YAML metadata
type HandleMismatchError struct {
	PathHandle string
	YamlHandle string
}

func (e *HandleMismatchError) Error() string {
	return fmt.Sprintf("Handle mismatch: path has '%s' but YAML metadata.name has '%s'", e.PathHandle, e.YamlHandle)
}

// APIValidationError wraps validation errors for API configurations
type APIValidationError struct {
	Errors []config.ValidationError
}

func (e *APIValidationError) Error() string {
	return fmt.Sprintf("configuration validation failed with %d errors", len(e.Errors))
}

// TopicOperationError indicates WebSub topic operations failed
type TopicOperationError struct {
	Message string
}

func (e *TopicOperationError) Error() string {
	return e.Message
}

// ConflictError indicates a resource conflict (e.g., handle already exists)
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}

// ParseError indicates configuration parsing failed
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

// DatabaseUnavailableError indicates the database is not available
type DatabaseUnavailableError struct{}

func (e *DatabaseUnavailableError) Error() string {
	return "Database storage not available"
}

// APIDeploymentService provides utilities for API configuration deployment
type APIDeploymentService struct {
	store             *storage.ConfigStore
	db                storage.Storage
	snapshotManager   *xds.SnapshotManager
	policyManager     *policyxds.PolicyManager
	parser            *config.Parser
	validator         config.Validator
	routerConfig      *config.RouterConfig
	httpClient        *http.Client
	eventHub          eventhub.EventHub
	enableReplicaSync bool
}

// NewAPIDeploymentService creates a new API deployment service
func NewAPIDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	validator config.Validator,
	routerConfig *config.RouterConfig,
	eventHub eventhub.EventHub,
	enableReplicaSync bool,
) *APIDeploymentService {
	return &APIDeploymentService{
		store:             store,
		db:                db,
		snapshotManager:   snapshotManager,
		policyManager:     policyManager,
		parser:            config.NewParser(),
		validator:         validator,
		httpClient:        &http.Client{Timeout: 10 * time.Second},
		routerConfig:      routerConfig,
		eventHub:          eventHub,
		enableReplicaSync: enableReplicaSync,
	}
}

// DeployAPIConfiguration handles the complete API configuration deployment process
func (s *APIDeploymentService) DeployAPIConfiguration(params APIDeploymentParams) (*APIDeploymentResult, error) {
	var apiConfig api.APIConfiguration
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	var apiName string
	var apiVersion string

	switch apiConfig.Kind {
	case api.RestApi:
		apiData, err := apiConfig.Spec.AsAPIConfigData()
		fmt.Println("APIData: ", apiData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse REST API data: %w", err)
		}
		apiName = apiData.DisplayName
		apiVersion = apiData.Version
	case api.Asyncwebsub:
		webhookData, err := apiConfig.Spec.AsWebhookAPIData()
		if err != nil {
			return nil, fmt.Errorf("failed to parse WebSub API data: %w", err)
		}
		apiName = webhookData.Name
		apiVersion = webhookData.Version
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		params.Logger.Warn("Configuration validation failed",
			zap.String("api_id", params.APIID),
			zap.String("name", apiName),
			zap.Int("num_errors", len(validationErrors)))

		for _, e := range validationErrors {
			fmt.Println(e.Message)
			params.Logger.Warn("Validation error",
				zap.String("field", e.Field),
				zap.String("message", e.Message))
		}
		return nil, fmt.Errorf("configuration validation failed with %d errors", len(validationErrors))
	}

	// Generate API ID if not provided
	apiID := params.APIID
	if apiID == "" {
		apiID = generateUUID()
	}

	handle := apiConfig.Metadata.Name

	if s.store != nil {
		if _, err := s.store.GetByNameVersion(apiName, apiVersion); err == nil {
			return nil, fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", storage.ErrConflict, apiName, apiVersion)
		}
		if handle != "" {
			for _, c := range s.store.GetAll() {
				if c.GetHandle() == handle {
					return nil, fmt.Errorf("%w: configuration with handle '%s' already exists", storage.ErrConflict, handle)
				}
			}
		}
	}

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredConfig{
		ID:                  apiID,
		Kind:                string(apiConfig.Kind),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Status:              models.StatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          nil,
		DeployedVersion:     0,
	}

	// Handle AsyncWebSub topic lifecycle management
	if apiConfig.Kind == api.Asyncwebsub {
		if err := s.handleTopicLifecycle(storedCfg, params.Logger); err != nil {
			return nil, err
		}
	}

	// Try to save/update the configuration
	isUpdate, err := s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// Log success
	if isUpdate {
		params.Logger.Info("API configuration updated",
			zap.String("api_id", apiID),
			zap.String("name", apiName),
			zap.String("version", apiVersion),
			zap.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("API configuration created",
			zap.String("api_id", apiID),
			zap.String("name", apiName),
			zap.String("version", apiVersion),
			zap.String("correlation_id", params.CorrelationID))
	}

	// Update xDS snapshot asynchronously
	if (s.enableReplicaSync) {
		// Multi-replica mode: Publish event to eventhub
		if s.eventHub != nil {
			// Determine action based on whether it's an update or create
			action := "CREATE"
			if isUpdate {
				action = "UPDATE"
			}

			// Use default organization ID (can be made configurable in future)
			organizationID := "default"

			// Publish event with empty payload as per requirements
			ctx := context.Background()
			if err := s.eventHub.PublishEvent(ctx, organizationID, eventhub.EventTypeAPI, action, apiID, params.CorrelationID, []byte{}); err != nil {
				params.Logger.Error("Failed to publish event to eventhub",
					zap.Error(err),
					zap.String("api_id", apiID),
					zap.String("action", action),
					zap.String("organization_id", string(organizationID)),
					zap.String("correlation_id", params.CorrelationID))
			} else {
				params.Logger.Info("Event published to eventhub",
					zap.String("api_id", apiID),
					zap.String("action", action),
					zap.String("organization_id", string(organizationID)),
					zap.String("correlation_id", params.CorrelationID))
			}
		} else {
			params.Logger.Warn("Multi-tenant mode enabled but eventhub is not initialized",
				zap.String("api_id", apiID))
		}
	} else {
		s.triggerXDSSnapshotUpdate(apiID, params.CorrelationID, params.Logger)
		s.updatePolicyConfiguration(storedCfg, params.Logger)
	}

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
}

// UpdateAPIConfiguration handles the complete API configuration update process
func (s *APIDeploymentService) UpdateAPIConfiguration(params APIUpdateParams) (*APIDeploymentResult, error) {
	handle := params.Handle

	// Parse configuration
	var apiConfig api.APIConfiguration
	err := s.parser.Parse(params.Data, params.ContentType, &apiConfig)
	if err != nil {
		params.Logger.Error("Failed to parse configuration", zap.Error(err))
		return nil, &ParseError{Message: "Failed to parse configuration"}
	}

	// Validate that the handle in the YAML matches the path parameter
	if apiConfig.Metadata.Name != "" {
		if apiConfig.Metadata.Name != handle {
			params.Logger.Warn("Handle mismatch between path and YAML metadata",
				zap.String("path_handle", handle),
				zap.String("yaml_handle", apiConfig.Metadata.Name))
			return nil, &HandleMismatchError{
				PathHandle: handle,
				YamlHandle: apiConfig.Metadata.Name,
			}
		}
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		params.Logger.Warn("Configuration validation failed",
			zap.String("handle", handle),
			zap.Int("num_errors", len(validationErrors)))
		return nil, &APIValidationError{Errors: validationErrors}
	}

	if s.db == nil {
		return nil, &DatabaseUnavailableError{}
	}

	// Check if config exists
	existing, err := s.db.GetConfigByHandle(handle)
	if err != nil {
		params.Logger.Warn("API configuration not found", zap.String("handle", handle))
		return nil, &NotFoundError{Handle: handle}
	}

	// Update stored configuration
	now := time.Now()
	existing.Configuration = apiConfig
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	// Handle AsyncWebSub topic lifecycle management
	if apiConfig.Kind == api.Asyncwebsub {
		if err := s.handleTopicLifecycle(existing, params.Logger); err != nil {
			return nil, err
		}
	}

	// Update database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(existing); err != nil {
			params.Logger.Error("Failed to update config in database", zap.Error(err))
			return nil, fmt.Errorf("failed to persist configuration update: %w", err)
		}
	}

	// Update in-memory store
	if err := s.store.Update(existing); err != nil {
		if storage.IsConflictError(err) {
			params.Logger.Info("API configuration handle already exists",
				zap.String("id", existing.ID),
				zap.String("handle", handle))
			return nil, &ConflictError{Message: err.Error()}
		}
		params.Logger.Error("Failed to update config in memory store", zap.Error(err))
		return nil, fmt.Errorf("failed to update configuration in memory store: %w", err)
	}

	params.Logger.Info("API configuration updated",
		zap.String("id", existing.ID),
		zap.String("handle", handle))

	// Update xDS snapshot asynchronously
	s.triggerXDSSnapshotUpdate(existing.ID, params.CorrelationID, params.Logger)

	// Update derived policy configuration
	s.updatePolicyConfiguration(existing, params.Logger)

	return &APIDeploymentResult{
		StoredConfig: existing,
		IsUpdate:     true,
	}, nil
}

// handleTopicLifecycle manages WebSub topic registration/deregistration for AsyncWebSub APIs
func (s *APIDeploymentService) handleTopicLifecycle(storedCfg *models.StoredConfig, logger *zap.Logger) error {
	topicsToRegister, topicsToUnregister := s.GetTopicsForUpdate(*storedCfg)

	var wg sync.WaitGroup
	var regErrs int32
	var deregErrs int32

	// Register new topics
	if len(topicsToRegister) > 0 {
		wg.Add(1)
		go func(list []string) {
			defer wg.Done()
			logger.Info("Starting topic registration",
				zap.Int("total_topics", len(list)),
				zap.String("api_id", storedCfg.ID))
			var childWg sync.WaitGroup
			for _, topic := range list {
				childWg.Add(1)
				go func(topic string) {
					defer childWg.Done()
					if err := s.RegisterTopicWithHub(s.httpClient, topic, "localhost", 8083, logger); err != nil {
						logger.Error("Failed to register topic with WebSubHub",
							zap.Error(err),
							zap.String("topic", topic),
							zap.String("api_id", storedCfg.ID))
						atomic.AddInt32(&regErrs, 1)
					} else {
						logger.Info("Successfully registered topic with WebSubHub",
							zap.String("topic", topic),
							zap.String("api_id", storedCfg.ID))
					}
				}(topic)
			}
			childWg.Wait()
		}(topicsToRegister)
	}

	// Deregister removed topics
	if len(topicsToUnregister) > 0 {
		wg.Add(1)
		go func(list []string) {
			defer wg.Done()
			logger.Info("Starting topic deregistration",
				zap.Int("total_topics", len(list)),
				zap.String("api_id", storedCfg.ID))
			var childWg sync.WaitGroup
			for _, topic := range list {
				childWg.Add(1)
				go func(topic string) {
					defer childWg.Done()
					if err := s.UnregisterTopicWithHub(s.httpClient, topic, "localhost", 8083, logger); err != nil {
						logger.Error("Failed to deregister topic from WebSubHub",
							zap.Error(err),
							zap.String("topic", topic),
							zap.String("api_id", storedCfg.ID))
						atomic.AddInt32(&deregErrs, 1)
					} else {
						logger.Info("Successfully deregistered topic from WebSubHub",
							zap.String("topic", topic),
							zap.String("api_id", storedCfg.ID))
					}
				}(topic)
			}
			childWg.Wait()
		}(topicsToUnregister)
	}

	wg.Wait()

	logger.Info("Topic lifecycle operations completed",
		zap.String("api_id", storedCfg.ID),
		zap.Int("registered", len(topicsToRegister)),
		zap.Int("deregistered", len(topicsToUnregister)),
		zap.Int("register_errors", int(regErrs)),
		zap.Int("deregister_errors", int(deregErrs)))

	if regErrs > 0 || deregErrs > 0 {
		return &TopicOperationError{
			Message: fmt.Sprintf("Topic lifecycle operations failed: %d registration error(s), %d deregistration error(s)", regErrs, deregErrs),
		}
	}

	return nil
}

func (s *APIDeploymentService) GetTopicsForUpdate(apiConfig models.StoredConfig) ([]string, []string) {
	topics := s.store.TopicManager.GetAllByConfig(apiConfig.ID)
	topicsToRegister := []string{}
	topicsToUnregister := []string{}
	apiTopicsPerRevision := make(map[string]bool)

	asyncData, err := apiConfig.Configuration.Spec.AsWebhookAPIData()
	if err != nil {
		// Return empty lists if parsing fails
		return topicsToRegister, topicsToUnregister
	}

	for _, topic := range asyncData.Channels {
		// Remove leading '/' from name, context, version and topic path if present
		name := strings.TrimPrefix(asyncData.Name, "/")
		context := strings.TrimPrefix(asyncData.Context, "/")
		version := strings.TrimPrefix(asyncData.Version, "/")
		path := strings.TrimPrefix(topic.Path, "/")

		modifiedTopic := fmt.Sprintf("%s_%s_%s_%s", name, context, version, path)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for _, topic := range topics {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			topicsToUnregister = append(topicsToUnregister, topic)
		}
	}

	for topic, _ := range apiTopicsPerRevision {
		if s.store.TopicManager.IsTopicExist(apiConfig.ID, topic) {
			continue
		}
		topicsToRegister = append(topicsToRegister, topic)
	}

	return topicsToRegister, topicsToUnregister
}

func (s *APIDeploymentService) GetTopicsForDelete(apiConfig models.StoredConfig) []string {
	return s.store.TopicManager.GetAllByConfig(apiConfig.ID)
}

// saveOrUpdateConfig handles the atomic dual-write operation for saving/updating configuration
func (s *APIDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredConfig, logger *zap.Logger) (bool, error) {
	// Try to save to database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			// Check if it's a conflict (API already exists)
			if storage.IsConflictError(err) {
				logger.Info("API configuration already exists in database, updating instead",
					zap.String("api_id", storedCfg.ID),
					zap.String("displayName", storedCfg.GetDisplayName()),
					zap.String("version", storedCfg.GetVersion()))

				// Try to update instead
				return s.updateExistingConfig(storedCfg, logger)
			} else {
				return false, fmt.Errorf("failed to save config to database: %w", err)
			}
		}
	}

	// Try to add to in-memory store
	if !s.enableReplicaSync {
		if err := s.store.Add(storedCfg); err != nil {
			// Check if it's a conflict (API already exists)
			if storage.IsConflictError(err) {
				logger.Info("API configuration already exists in memory, updating instead",
					zap.String("api_id", storedCfg.ID),
					zap.String("displayName", storedCfg.GetDisplayName()),
					zap.String("version", storedCfg.GetVersion()))

				// Try to update instead
				return s.updateExistingConfig(storedCfg, logger)
			} else {
				// Rollback database write (only if persistent mode)
				if s.db != nil {
					_ = s.db.DeleteConfig(storedCfg.ID)
				}
				return false, fmt.Errorf("failed to add config to memory store: %w", err)
			}
		}
	}

	return false, nil // Successfully created new config
}

// updateExistingConfig updates an existing API configuration
func (s *APIDeploymentService) updateExistingConfig(newConfig *models.StoredConfig, logger *zap.Logger) (bool, error) {
	// Get existing config
	existing, err := s.store.GetByNameVersion(newConfig.GetDisplayName(), newConfig.GetVersion())
	if err != nil {
		return false, fmt.Errorf("failed to get existing config: %w", err)
	}

	// Backup original state for potential rollback
	original := *existing

	// Update the existing configuration
	now := time.Now()
	existing.Configuration = newConfig.Configuration
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	// Update database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(existing); err != nil {
			return false, fmt.Errorf("failed to update config in database: %w", err)
		}
	}

	// Update in-memory store
	if err := s.store.Update(existing); err != nil {
		// Rollback DB to original state since memory update failed
		if s.db != nil {
			if rbErr := s.db.UpdateConfig(&original); rbErr != nil {
				logger.Error("Failed to rollback DB after memory update failure",
					zap.Error(rbErr),
					zap.String("id", original.ID),
					zap.String("displayName", original.GetDisplayName()),
					zap.String("version", original.GetVersion()))
			}
		}
		return false, fmt.Errorf("failed to update config in memory store: %w", err)
	}

	// Update the newConfig to reflect the changes
	*newConfig = *existing

	return true, nil // Successfully updated existing config
}

// RegisterTopicWithHub registers a topic with the WebSubHub
func (s *APIDeploymentService) RegisterTopicWithHub(httpClient *http.Client, topic, webSubHubHost string, webSubPort int, logger *zap.Logger) error {
	return s.sendTopicRequestToHub(httpClient, topic, "register", webSubHubHost, webSubPort, logger)
}

// UnregisterTopicWithHub unregisters a topic from the WebSubHub
func (s *APIDeploymentService) UnregisterTopicWithHub(httpClient *http.Client, topic, webSubHubHost string, webSubPort int, logger *zap.Logger) error {
	return s.sendTopicRequestToHub(httpClient, topic, "deregister", webSubHubHost, webSubPort, logger)
}

// sendTopicRequestToHub sends a topic registration/unregistration request to the WebSubHub
func (s *APIDeploymentService) sendTopicRequestToHub(httpClient *http.Client, topic string, mode string, webSubHubHost string, webSubPort int, logger *zap.Logger) error {
	// Prepare form data
	formData := url.Values{}
	formData.Set("hub.mode", mode)
	formData.Set("hub.topic", topic)

	// Build target URL to gwHost reverse proxy endpoint (no proxy)
	targetURL := fmt.Sprintf("http://%s:%d/websubhub/operations", webSubHubHost, webSubPort)

	// Retry on 404 Not Found (hub might not be ready immediately)
	const maxRetries = 5
	backoff := 500 * time.Millisecond
	var lastStatus int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Encode form values so special characters in hub.topic are properly percent-encoded
		req, err := http.NewRequest("POST", targetURL, strings.NewReader(formData.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send HTTP request: %w", err)
		}

		// Ensure body is closed before next loop/return
		func() {
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Debug("Topic request sent to WebSubHub",
					zap.String("topic", topic),
					zap.String("mode", mode),
					zap.Int("status", resp.StatusCode))
				err = nil
				return
			}

			lastStatus = resp.StatusCode
		}()

		// Success path returned above
		if lastStatus == 0 {
			return nil
		}

		// Retry only on 404 or 503, up to maxRetries
		if (lastStatus == http.StatusNotFound || lastStatus == http.StatusServiceUnavailable) && attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
			lastStatus = 0
			continue
		}
		return fmt.Errorf("WebSubHub returned non-success status: %d", lastStatus)
	}

	return fmt.Errorf("WebSubHub request failed after %d retries; last status: %d", maxRetries, lastStatus)
}

// generateUUID generates a new UUID string
func generateUUID() string {
	return uuid.New().String()
}

// BuildStoredPolicyFromAPI builds a StoredPolicyConfig from an API configuration.
// This builds policy chains for each route based on API-level and operation-level policies.
// RouteKey uses the fully qualified route path (context + operation path) and must match
// the route name format used by the xDS translator for consistency.
func (s *APIDeploymentService) BuildStoredPolicyFromAPI(cfg *models.StoredConfig) *models.StoredPolicyConfig {
	apiCfg := &cfg.Configuration

	// Collect API-level policies
	apiPolicies := make(map[string]policyenginev1.PolicyInstance) // name -> policy
	if cfg.GetPolicies() != nil {
		for _, p := range *cfg.GetPolicies() {
			apiPolicies[p.Name] = convertAPIPolicy(p)
		}
	}

	routes := make([]policyenginev1.PolicyChain, 0)
	switch apiCfg.Kind {
	case api.Asyncwebsub:
		// Build routes with merged policies
		apiData, err := apiCfg.Spec.AsWebhookAPIData()
		if err != nil {
			return nil
		}
		for _, ch := range apiData.Channels {
			var finalPolicies []policyenginev1.PolicyInstance

			if ch.Policies != nil && len(*ch.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*ch.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *ch.Policies {
					finalPolicies = append(finalPolicies, convertAPIPolicy(opPolicy))
					addedNames[opPolicy.Name] = struct{}{}
				}

				// Add any API-level policies not mentioned in operation policies (append at end)
				if apiData.Policies != nil {
					for _, apiPolicy := range *apiData.Policies {
						if _, exists := addedNames[apiPolicy.Name]; !exists {
							finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
						}
					}
				}
			} else {
				// No operation policies: use API-level policies in their declared order
				if apiData.Policies != nil {
					finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
					for _, p := range *apiData.Policies {
						finalPolicies = append(finalPolicies, apiPolicies[p.Name])
					}
				}
			}

			routeKey := xds.GenerateRouteName("POST", apiData.Context, apiData.Version, ch.Path, s.routerConfig.GatewayHost)
			routes = append(routes, policyenginev1.PolicyChain{
				RouteKey: routeKey,
				Policies: finalPolicies,
			})
		}
	case api.RestApi:
		// Build routes with merged policies
		apiData, err := apiCfg.Spec.AsAPIConfigData()
		if err != nil {
			return nil
		}
		for _, op := range apiData.Operations {
			var finalPolicies []policyenginev1.PolicyInstance

			if op.Policies != nil && len(*op.Policies) > 0 {
				// Operation has policies: use operation policy order as authoritative
				finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*op.Policies))
				addedNames := make(map[string]struct{})

				for _, opPolicy := range *op.Policies {
					finalPolicies = append(finalPolicies, convertAPIPolicy(opPolicy))
					addedNames[opPolicy.Name] = struct{}{}
				}

				// Add any API-level policies not mentioned in operation policies (append at end)
				if apiData.Policies != nil {
					for _, apiPolicy := range *apiData.Policies {
						if _, exists := addedNames[apiPolicy.Name]; !exists {
							finalPolicies = append(finalPolicies, apiPolicies[apiPolicy.Name])
						}
					}
				}
			} else {
				// No operation policies: use API-level policies in their declared order
				if apiData.Policies != nil {
					finalPolicies = make([]policyenginev1.PolicyInstance, 0, len(*apiData.Policies))
					for _, p := range *apiData.Policies {
						finalPolicies = append(finalPolicies, apiPolicies[p.Name])
					}
				}
			}

			// Determine effective vhosts (fallback to global router defaults when not provided)
			effectiveMainVHost := s.routerConfig.VHosts.Main.Default
			effectiveSandboxVHost := s.routerConfig.VHosts.Sandbox.Default
			if apiData.Vhosts != nil {
				if strings.TrimSpace(apiData.Vhosts.Main) != "" {
					effectiveMainVHost = apiData.Vhosts.Main
				}
				if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
					effectiveSandboxVHost = *apiData.Vhosts.Sandbox
				}
			}

			vhosts := []string{effectiveMainVHost}
			if apiData.Upstream.Sandbox != nil && apiData.Upstream.Sandbox.Url != nil &&
				strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != "" {
				vhosts = append(vhosts, effectiveSandboxVHost)
			}

			for _, vhost := range vhosts {
				routes = append(routes, policyenginev1.PolicyChain{
					RouteKey: xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost),
					Policies: finalPolicies,
				})
			}
		}
	}

	// If there are no policies at all, return nil (skip creation)
	policyCount := 0
	for _, r := range routes {
		policyCount += len(r.Policies)
	}
	if policyCount == 0 {
		return nil
	}

	now := time.Now().Unix()
	stored := &models.StoredPolicyConfig{
		ID: cfg.ID + "-policies",
		Configuration: policyenginev1.Configuration{
			Routes: routes,
			Metadata: policyenginev1.Metadata{
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 0,
				APIName:         cfg.GetDisplayName(),
				Version:         cfg.GetVersion(),
				Context:         cfg.GetContext(),
			},
		},
		Version: 0,
	}
	return stored
}

// convertAPIPolicy converts generated api.Policy to policyenginev1.PolicyInstance
func convertAPIPolicy(p api.Policy) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}
	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            p.Version,
		Enabled:            true, // Default to enabled
		ExecutionCondition: p.ExecutionCondition,
		Parameters:         paramsMap,
	}
}

// updatePolicyConfiguration builds and updates/removes derived policy config for an API
func (s *APIDeploymentService) updatePolicyConfiguration(storedCfg *models.StoredConfig, logger *zap.Logger) {
	if s.policyManager == nil {
		return
	}

	storedPolicy := s.BuildStoredPolicyFromAPI(storedCfg)
	if storedPolicy != nil {
		if err := s.policyManager.AddPolicy(storedPolicy); err != nil {
			logger.Error("Failed to update derived policy configuration", zap.Error(err))
		} else {
			logger.Info("Derived policy configuration updated",
				zap.String("policy_id", storedPolicy.ID),
				zap.Int("route_count", len(storedPolicy.Configuration.Routes)))
		}
	} else {
		// API no longer has policies, remove the existing policy configuration
		policyID := storedCfg.ID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			// Log at debug level since policy may not exist if API never had policies
			logger.Debug("No policy configuration to remove", zap.String("policy_id", policyID))
		} else {
			logger.Info("Derived policy configuration removed (API no longer has policies)",
				zap.String("policy_id", policyID))
		}
	}
}

// triggerXDSSnapshotUpdate asynchronously updates xDS snapshot
func (s *APIDeploymentService) triggerXDSSnapshotUpdate(apiID, correlationID string, logger *zap.Logger) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			logger.Error("Failed to update xDS snapshot",
				zap.Error(err),
				zap.String("api_id", apiID),
				zap.String("correlation_id", correlationID))
		}
	}()
}
