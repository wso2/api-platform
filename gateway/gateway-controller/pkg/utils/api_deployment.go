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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// APIDeploymentParams contains parameters for API deployment operations
type APIDeploymentParams struct {
	Data          []byte       // Raw configuration data (YAML/JSON)
	ContentType   string       // Content type for parsing
	APIID         string       // API ID (if provided, used for updates; if empty, generates new UUID)
	CorrelationID string       // Correlation ID for tracking
	Logger        *slog.Logger // Logger instance
}

// APIDeploymentResult contains the result of API deployment
type APIDeploymentResult struct {
	StoredConfig *models.StoredConfig
	IsUpdate     bool
}

// APIDeletionParams contains parameters for API deletion operations
type APIDeletionParams struct {
	APIID         string // API ID
	Handle        string // API handle (metadata.name)
	CorrelationID string // Correlation ID for tracking
	Logger        *slog.Logger
}

// APIDeletionResult contains the result of API deletion
type APIDeletionResult struct {
	StoredConfig *models.StoredConfig
}

// APIUndeploymentParams contains parameters for API undeployment operations
type APIUndeploymentParams struct {
	APIID         string // API ID
	CorrelationID string // Correlation ID for tracking
	Logger        *slog.Logger
}

// APIUndeploymentResult contains the result of API undeployment
type APIUndeploymentResult struct {
	StoredConfig *models.StoredConfig
}

// ValidationErrorListError wraps validation errors for API configuration.
// This allows callers to return structured validation errors in API responses.
type ValidationErrorListError struct {
	Errors []config.ValidationError
}

func (e *ValidationErrorListError) Error() string {
	return fmt.Sprintf("configuration validation failed with %d errors", len(e.Errors))
}

// APIDeploymentService provides utilities for API configuration deployment
type APIDeploymentService struct {
	store        *storage.ConfigStore
	db           storage.Storage
	parser       *config.Parser
	validator    config.Validator
	routerConfig *config.RouterConfig
	httpClient   *http.Client
	eventHub     eventhub.EventHub
}

// NewAPIDeploymentService creates a new API deployment service
func NewAPIDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	validator config.Validator,
	routerConfig *config.RouterConfig,
	hub eventhub.EventHub,
) *APIDeploymentService {
	return &APIDeploymentService{
		store:        store,
		db:           db,
		parser:       config.NewParser(),
		validator:    validator,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		routerConfig: routerConfig,
		eventHub:     hub,
	}
}

// publishEvent publishes an event to the event hub
func (s *APIDeploymentService) publishEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	if s.eventHub == nil {
		return
	}

	event := eventhub.Event{
		OrganizationID:      "default",
		OriginatedTimestamp: time.Now(),
		EventType:           eventType,
		Action:              action,
		EntityID:            entityID,
		CorrelationID:       correlationID,
		EventData:           eventhub.EmptyEventData,
	}

	if err := s.eventHub.PublishEvent("default", event); err != nil {
		logger.Warn("Failed to publish event to event hub",
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	} else {
		logger.Debug("Published event to event hub",
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID))
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
		if err != nil {
			return nil, fmt.Errorf("failed to parse REST API data: %w", err)
		}
		apiName = apiData.DisplayName
		apiVersion = apiData.Version
	case api.WebSubApi:
		webhookData, err := apiConfig.Spec.AsWebhookAPIData()
		if err != nil {
			return nil, fmt.Errorf("failed to parse WebSub API data: %w", err)
		}
		apiName = webhookData.DisplayName
		apiVersion = webhookData.Version
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		params.Logger.Warn("Configuration validation failed",
			slog.String("api_id", params.APIID),
			slog.String("name", apiName),
			slog.Int("num_errors", len(validationErrors)))

		for _, e := range validationErrors {
			fmt.Println(e.Message)
			params.Logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
		}
		return nil, &ValidationErrorListError{Errors: validationErrors}
	}

	// Generate API ID if not provided
	apiID := params.APIID
	if apiID == "" {
		var err error
		apiID, err = GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API ID: %w", err)
		}
	}

	handle := apiConfig.Metadata.Name

	// Determine if this is an update or create by checking if config with apiID already exists
	var isUpdate bool

	// Check for conflicts with other configurations
	// For updates: only error if name/version/handle belong to a different config ID
	// For creates: any conflict is an error
	if s.db != nil {
		existingConfig, err := s.db.GetConfig(apiID)
		if err != nil && !storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("failed to check existing configuration in database: %w", err)
		}
		isUpdate = existingConfig != nil

		// Check name+version conflict
		if conflicting, err := s.db.GetConfigByNameVersion(apiName, apiVersion); err == nil {
			// For updates: only error if the conflict is with a different API
			// For creates: any conflict is an error
			if !isUpdate || conflicting.ID != apiID {
				return nil, fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", storage.ErrConflict, apiName, apiVersion)
			}
		} else if !storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("failed to check name/version conflicts in database: %w", err)
		}

		// Check handle conflict
		if handle != "" {
			if conflicting, err := s.db.GetConfigByHandle(handle); err == nil {
				// For updates: only error if the conflict is with a different API
				// For creates: any conflict is an error
				if !isUpdate || conflicting.ID != apiID {
					return nil, fmt.Errorf("%w: configuration with handle '%s' already exists", storage.ErrConflict, handle)
				}
			} else if !storage.IsNotFoundError(err) {
				return nil, fmt.Errorf("failed to check handle conflicts in database: %w", err)
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

	if apiConfig.Kind == api.WebSubApi {
		topicsToRegister, topicsToUnregister := s.GetTopicsForUpdate(*storedCfg)
		// TODO: Pre configure the dynamic forward proxy rules for event gw
		// This was communication bridge will be created on the gw startup
		// Can perform internal communication with websub hub without relying on the dynamic rules
		// Execute topic operations with wait group and errors tracking
		var wg2 sync.WaitGroup
		var regErrs int32
		var deregErrs int32

		if len(topicsToRegister) > 0 {
			wg2.Add(1)
			go func(list []string) {
				defer wg2.Done()
				params.Logger.Info("Starting topic registration", slog.Int("total_topics", len(list)), slog.String("api_id", apiID))
				var childWg sync.WaitGroup
				for _, topic := range list {
					childWg.Add(1)
					go func(topic string) {
						defer childWg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
						defer cancel()

						if err := s.RegisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, params.Logger); err != nil {
							params.Logger.Error("Failed to register topic with WebSubHub",
								slog.Any("error", err),
								slog.String("topic", topic),
								slog.String("api_id", apiID))
							atomic.AddInt32(&regErrs, 1)
							return
						} else {
							params.Logger.Info("Successfully registered topic with WebSubHub",
								slog.String("topic", topic),
								slog.String("api_id", apiID))
						}
					}(topic)
				}
				childWg.Wait()
			}(topicsToRegister)
		}

		if len(topicsToUnregister) > 0 {
			wg2.Add(1)
			go func(list []string) {
				defer wg2.Done()
				var childWg sync.WaitGroup
				params.Logger.Info("Starting topic deregistration", slog.Int("total_topics", len(list)), slog.String("api_id", apiID))
				for _, topic := range list {
					childWg.Add(1)
					go func(topic string) {
						defer childWg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
						defer cancel()

						if err := s.UnregisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, params.Logger); err != nil {
							params.Logger.Error("Failed to deregister topic from WebSubHub",
								slog.Any("error", err),
								slog.String("topic", topic),
								slog.String("api_id", apiID))
							atomic.AddInt32(&deregErrs, 1)
							return
						} else {
							params.Logger.Info("Successfully deregistered topic from WebSubHub",
								slog.String("topic", topic),
								slog.String("api_id", apiID))
						}
					}(topic)
				}
				childWg.Wait()
			}(topicsToUnregister)
		}

		wg2.Wait()
		params.Logger.Info("Topic lifecycle operations completed",
			slog.String("api_id", apiID),
			slog.Int("registered", len(topicsToRegister)),
			slog.Int("deregistered", len(topicsToUnregister)),
			slog.Int("register_errors", int(regErrs)),
			slog.Int("deregister_errors", int(deregErrs)))

		// Check if topic operations failed and return error
		if regErrs > 0 || deregErrs > 0 {
			params.Logger.Error("Topic lifecycle operations failed",
				slog.Int("register_errors", int(regErrs)),
				slog.Int("deregister_errors", int(deregErrs)))
			return nil, fmt.Errorf("failed to complete topic operations: %d registration error(s), %d deregistration error(s)", regErrs, deregErrs)
		}
	}

	// Try to save/update the configuration
	isUpdate, err = s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// Log success
	if isUpdate {
		params.Logger.Info("API configuration updated",
			slog.String("api_id", apiID),
			slog.String("name", apiName),
			slog.String("version", apiVersion),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("API configuration created",
			slog.String("api_id", apiID),
			slog.String("name", apiName),
			slog.String("version", apiVersion),
			slog.String("correlation_id", params.CorrelationID))
	}

	// Publish event to event hub for multi-replica sync
	if isUpdate {
		s.publishEvent(eventhub.EventTypeAPI, "UPDATE", apiID, params.CorrelationID, params.Logger)
	} else {
		s.publishEvent(eventhub.EventTypeAPI, "CREATE", apiID, params.CorrelationID, params.Logger)
	}

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
}

// DeleteAPIConfiguration handles API deletion in the write path.
// The in-memory config store is intentionally not updated here; replica sync is eventhub-driven.
func (s *APIDeploymentService) DeleteAPIConfiguration(params APIDeletionParams) (*APIDeletionResult, error) {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if s.db == nil {
		return nil, fmt.Errorf("%w: cannot delete config without database", storage.ErrDatabaseUnavailable)
	}

	var (
		cfg *models.StoredConfig
		err error
	)

	switch {
	case params.APIID != "":
		cfg, err = s.db.GetConfig(params.APIID)
		if err != nil {
			if storage.IsNotFoundError(err) || strings.Contains(strings.ToLower(err.Error()), "not found") {
				return nil, fmt.Errorf("%w: api_id=%s", storage.ErrNotFound, params.APIID)
			}
			return nil, fmt.Errorf("failed to fetch config from database: %w", err)
		}
	case params.Handle != "":
		cfg, err = s.db.GetConfigByHandle(params.Handle)
		if err != nil {
			if storage.IsNotFoundError(err) || strings.Contains(strings.ToLower(err.Error()), "not found") {
				return nil, fmt.Errorf("%w: handle=%s", storage.ErrNotFound, params.Handle)
			}
			return nil, err
		}
	default:
		return nil, fmt.Errorf("api id or handle is required for deletion")
	}

	if err := s.db.DeleteConfig(cfg.ID); err != nil {
		return nil, fmt.Errorf("failed to delete config from database: %w", err)
	}

	// Remove associated API keys from database (best effort)
	if err := s.db.RemoveAPIKeysAPI(cfg.ID); err != nil {
		logger.Warn("Failed to remove API keys from database",
			slog.String("api_id", cfg.ID),
			slog.String("handle", cfg.GetHandle()),
			slog.Any("error", err))
	}

	var deletionErr error
	if cfg.Configuration.Kind == api.WebSubApi {
		deletionErr = s.deregisterWebSubTopicsOnDelete(cfg, logger)
	}

	// Publish deletion event so all replicas (including self) converge through event listener sync.
	s.publishEvent(eventhub.EventTypeAPI, "DELETE", cfg.ID, params.CorrelationID, logger)

	if deletionErr != nil {
		return nil, deletionErr
	}

	return &APIDeletionResult{StoredConfig: cfg}, nil
}

// UndeployAPIConfiguration handles API undeployment in the write path.
// The in-memory config store is intentionally not updated here; replica sync is eventhub-driven.
func (s *APIDeploymentService) UndeployAPIConfiguration(params APIUndeploymentParams) (*APIUndeploymentResult, error) {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if s.db == nil {
		return nil, fmt.Errorf("%w: cannot undeploy config without database", storage.ErrDatabaseUnavailable)
	}

	if params.APIID == "" {
		return nil, fmt.Errorf("api id is required for undeployment")
	}

	cfg, err := s.db.GetConfig(params.APIID)
	if err != nil {
		if storage.IsNotFoundError(err) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, fmt.Errorf("%w: api_id=%s", storage.ErrNotFound, params.APIID)
		}
		return nil, fmt.Errorf("failed to fetch config from database: %w", err)
	}

	// Preserve config, keys, and policies while marking undeployed.
	cfg.Status = models.StatusUndeployed
	cfg.UpdatedAt = time.Now()

	if err := s.db.UpdateConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to update config status in database: %w", err)
	}

	// Publish update event so all replicas (including self) converge through event listener sync.
	s.publishEvent(eventhub.EventTypeAPI, "UPDATE", cfg.ID, params.CorrelationID, logger)

	return &APIUndeploymentResult{StoredConfig: cfg}, nil
}

func (s *APIDeploymentService) deregisterWebSubTopicsOnDelete(cfg *models.StoredConfig, logger *slog.Logger) error {
	if s.routerConfig == nil {
		return fmt.Errorf("router configuration is required to deregister WebSub topics")
	}

	topicsToUnregister := s.GetTopicsForDelete(*cfg)
	if len(topicsToUnregister) == 0 {
		return nil
	}

	var deregErrs int32
	var wg sync.WaitGroup
	wg.Add(1)

	go func(list []string) {
		defer wg.Done()
		logger.Info("Starting topic deregistration",
			slog.Int("total_topics", len(list)),
			slog.String("api_id", cfg.ID))

		var childWg sync.WaitGroup
		for _, topic := range list {
			childWg.Add(1)
			go func(topic string) {
				defer childWg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
				defer cancel()

				if err := s.UnregisterTopicWithHub(ctx, s.httpClient, topic, s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, logger); err != nil {
					logger.Error("Failed to deregister topic from WebSubHub",
						slog.Any("error", err),
						slog.String("topic", topic),
						slog.String("api_id", cfg.ID))
					atomic.AddInt32(&deregErrs, 1)
					return
				}
				logger.Info("Successfully deregistered topic from WebSubHub",
					slog.String("topic", topic),
					slog.String("api_id", cfg.ID))
			}(topic)
		}
		childWg.Wait()
	}(topicsToUnregister)

	wg.Wait()

	logger.Info("Topic lifecycle operations completed",
		slog.String("api_id", cfg.ID),
		slog.Int("deregistered", len(topicsToUnregister)),
		slog.Int("deregister_errors", int(deregErrs)))

	if deregErrs > 0 {
		return fmt.Errorf("failed to complete topic operations: %d deregistration error(s)", deregErrs)
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
		contextWithVersion := strings.ReplaceAll(asyncData.Context, "$version", asyncData.Version)
		contextWithVersion = strings.TrimPrefix(contextWithVersion, "/")
		contextWithVersion = strings.ReplaceAll(contextWithVersion, "/", "_")
		name := strings.TrimPrefix(topic.Name, "/")
		modifiedTopic := fmt.Sprintf("%s_%s", contextWithVersion, name)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for _, topic := range topics {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			topicsToUnregister = append(topicsToUnregister, topic)
		}
	}

	for topic := range apiTopicsPerRevision {
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
func (s *APIDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredConfig, logger *slog.Logger) (bool, error) {
	// Save new config to database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			if storage.IsConflictError(err) {
				existing, getErr := s.db.GetConfig(storedCfg.ID)
				if getErr != nil {
					if storage.IsNotFoundError(getErr) {
						return false, fmt.Errorf("failed to save config to database: %w", err)
					}
					return false, fmt.Errorf("failed to check existing config in database: %w", getErr)
				}
				logger.Info("API configuration already exists in database, updating",
					slog.String("api_id", storedCfg.ID),
					slog.String("displayName", storedCfg.GetDisplayName()),
					slog.String("version", storedCfg.GetVersion()))
				return s.updateExistingConfig(storedCfg, existing, logger)
			}
			logger.Info("Error saving new API configuration to database",
				slog.String("api_id", storedCfg.ID),
				slog.String("displayName", storedCfg.GetDisplayName()),
				slog.String("version", storedCfg.GetVersion()))
			return false, fmt.Errorf("failed to save config to database: %w", err)
		}
	}
	// In-memory store save only happens during synchronization

	return false, nil // Successfully created new config
}

// updateExistingConfig updates an existing API configuration
func (s *APIDeploymentService) updateExistingConfig(newConfig *models.StoredConfig,
	existing *models.StoredConfig, logger *slog.Logger) (bool, error) {
	// Update the existing configuration
	now := time.Now()
	existing.Configuration = newConfig.Configuration
	existing.SourceConfiguration = newConfig.SourceConfiguration
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	// Update database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(existing); err != nil {
			return false, fmt.Errorf("failed to update config in database: %w", err)
		}
	} else {
		return false, fmt.Errorf("%w: cannot update config without database", storage.ErrDatabaseUnavailable)
	}

	// Update the newConfig to reflect the changes
	*newConfig = *existing

	logger.Debug("Skipped in-memory update in write path; memory sync is eventhub-driven",
		slog.String("api_id", newConfig.ID))

	return true, nil // Successfully updated existing config
}

func (s *APIDeploymentService) RegisterTopicWithHub(ctx context.Context, httpClient *http.Client, topic, webSubHubHost string, webSubPort int, logger *slog.Logger) error {
	return s.sendTopicRequestToHub(ctx, httpClient, topic, "register", webSubHubHost, webSubPort, logger)
}

// UnregisterTopicWithHub unregisters a topic from the WebSubHub
func (s *APIDeploymentService) UnregisterTopicWithHub(ctx context.Context, httpClient *http.Client, topic, webSubHubHost string, webSubPort int, logger *slog.Logger) error {
	return s.sendTopicRequestToHub(ctx, httpClient, topic, "deregister", webSubHubHost, webSubPort, logger)
}

// sendTopicRequestToHub sends a topic registration/unregistration request to the WebSubHub
func (s *APIDeploymentService) sendTopicRequestToHub(ctx context.Context, httpClient *http.Client, topic string, mode string, webSubHubHost string, webSubPort int, logger *slog.Logger) error {
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
		req, err := http.NewRequestWithContext(ctx, "POST", targetURL, strings.NewReader(formData.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			// If the context was cancelled or deadline exceeded, surface that
			select {
			case <-ctx.Done():
				return fmt.Errorf("request canceled: %w", ctx.Err())
			default:
			}
			return fmt.Errorf("failed to send HTTP request: %w", err)
		}

		// Ensure body is closed before next loop/return
		func() {
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Debug("Topic request sent to WebSubHub",
					slog.String("topic", topic),
					slog.String("mode", mode),
					slog.Int("status", resp.StatusCode))
			}

			lastStatus = resp.StatusCode
		}()

		// Success path returned above
		if lastStatus == 0 {
			return nil
		}

		// Retry only on 404 or 503, up to maxRetries
		if (lastStatus == http.StatusNotFound || lastStatus == http.StatusServiceUnavailable) && attempt < maxRetries {
			select {
			case <-ctx.Done():
				return fmt.Errorf("request canceled: %w", ctx.Err())
			case <-time.After(backoff):
			}
			backoff *= 2
			lastStatus = 0
			continue
		}
		return fmt.Errorf("WebSubHub returned non-success status: %d", lastStatus)
	}

	return fmt.Errorf("WebSubHub request failed after %d retries; last status: %d", maxRetries, lastStatus)
}
