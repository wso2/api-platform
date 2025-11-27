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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
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
	StoredConfig *models.StoredAPIConfig
	IsUpdate     bool
}

// APIDeploymentService provides utilities for API configuration deployment
type APIDeploymentService struct {
	store           *storage.ConfigStore
	db              storage.Storage
	snapshotManager *xds.SnapshotManager
	parser          *config.Parser
	validator       config.Validator
}

// NewAPIDeploymentService creates a new API deployment service
func NewAPIDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	validator config.Validator,
) *APIDeploymentService {
	return &APIDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       validator,
	}
}

func (s *APIDeploymentService) UpdateAPIConfiguration(name, version string, params APIDeploymentParams) (*APIDeploymentResult, error) {
	apiConfig, err := s.parser.Parse(params.Data, params.ContentType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	if apiConfig.Kind != "http/rest" {
		return s.handleRestAPIUpdate(name, version, apiConfig, params)
	} else if apiConfig.Kind != "async/websub" {
		return s.handleAsyncAPIUpdate(name, version, apiConfig, params)
	}
	return nil, fmt.Errorf("unsupported API kind: %s", apiConfig.Kind)
}

func (s *APIDeploymentService) UndeployAPIConfiguration(name, version string, params APIDeploymentParams) (*APIDeploymentResult, error) {
	apiConfig, err := s.store.GetByNameVersion(name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve configuration: %w", err)
	}

	switch apiConfig.Configuration.Kind {
	case "http/rest":
		return s.handleRestAPIUndeploy(apiConfig, params)
	case "async/websub":
		return s.handleAsyncAPIUndeploy(apiConfig, params)
	}
	return nil, fmt.Errorf("unsupported API kind: %s", apiConfig.Configuration.Kind)
}

func (s *APIDeploymentService) handleRestAPIUndeploy(apiConfig *models.StoredAPIConfig, params APIDeploymentParams) (*APIDeploymentResult, error) {
	// Delete from database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.DeleteConfig(apiConfig.ID); err != nil {
			params.Logger.Error("Failed to delete config from database", zap.Error(err))
			return nil, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}

	// Delete from in-memory store
	if err := s.store.Delete(apiConfig.ID); err != nil {
		params.Logger.Error("Failed to delete config from memory store", zap.Error(err))
		return nil, fmt.Errorf("failed to delete configuration from memory store: %w", err)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
			params.Logger.Error("Failed to update xDS snapshot", zap.Error(err))
		}
		// Grace period to let Envoy apply new resources
		select {
		case <-time.After(6 * time.Second):
		case <-ctx.Done():
			params.Logger.Warn("Context cancelled before topic lifecycle", zap.String("api_id", apiConfig.ID))
			return
		}
	}()

	return &APIDeploymentResult{
		StoredConfig: nil,
		IsUpdate:     true,
	}, nil
}

func (s *APIDeploymentService) handleAsyncAPIUndeploy(apiConfig *models.StoredAPIConfig, params APIDeploymentParams) (*APIDeploymentResult, error) {
	// Fetch all topics to remove for this API before deletion
	topicsToRemove, _ := s.GetAllTopicsToRegisterAndUnregister(*apiConfig)

	// Delete from database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.DeleteConfig(apiConfig.ID); err != nil {
			params.Logger.Error("Failed to delete config from database", zap.Error(err))
			return nil, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}

	// Delete from in-memory store
	if err := s.store.Delete(apiConfig.ID); err != nil {
		params.Logger.Error("Failed to delete config from memory store", zap.Error(err))
		return nil, fmt.Errorf("failed to delete configuration from memory store: %w", err)
	}

	// Execute topic deregistration with wait group and error tracking
	var wg sync.WaitGroup
	var deregErrs int32

	if len(topicsToRemove) > 0 {
		wg.Add(1)
		go func(list []string) {
			defer wg.Done()
			for _, topic := range list {
				// Topic register and deregister should be proxy calls to avoid blocking
				// If these requests are handled with the reverse proxy then we need to remove all before XDS removes the routes
				// If websubhub is down or unreachable we should log and continue and once it becomes
				// reachable again we need to remove/add the topics that were failed
				if err := s.UnregisterTopicWithHub(topic, "localhost", params.Logger); err != nil {
					params.Logger.Error("Failed to deregister topic from WebSubHub",
						zap.Error(err),
						zap.String("topic", topic),
						zap.String("api_id", apiConfig.ID))
					atomic.AddInt32(&deregErrs, 1)
				} else {
					params.Logger.Info("Successfully deregistered topic from WebSubHub",
						zap.String("topic", topic),
						zap.String("api_id", apiConfig.ID))
				}
			}
		}(topicsToRemove)
	}

	// Wait for topic deregistration to complete
	wg.Wait()
	params.Logger.Info("Topic deregistration operations completed",
		zap.String("api_id", apiConfig.ID),
		zap.Int("total_topics", len(topicsToRemove)),
		zap.Int("deregister_errors", int(deregErrs)))

	// Check if topic deregistration failed
	if deregErrs > 0 {
		params.Logger.Error("Topic deregistration failed, skipping xDS snapshot update",
			zap.Int("deregister_errors", int(deregErrs)))
		return nil, fmt.Errorf("failed to deregister %d topic(s) from WebSubHub", deregErrs)
	}

	// Update xDS snapshot asynchronously only if all topics were deregistered successfully
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshotofAsyncAPI(ctx, params.CorrelationID); err != nil {
			params.Logger.Error("Failed to update xDS snapshot", zap.Error(err))
		}
		// Grace period to let Envoy apply new resources
		select {
		case <-time.After(6 * time.Second):
		case <-ctx.Done():
			params.Logger.Warn("Context cancelled before topic lifecycle", zap.String("api_id", apiConfig.ID))
			return
		}
	}()

	apiData, err := apiConfig.Configuration.Data.AsWebhookAPIData()
	if err != nil {
		return nil, fmt.Errorf("failed to parse async API data: %w", err)
	}
	params.Logger.Info("API configuration deleted",
		zap.String("id", apiConfig.ID),
		zap.String("name", apiData.Name),
		zap.String("version", apiData.Version))

	return &APIDeploymentResult{
		StoredConfig: nil,
		IsUpdate:     true,
	}, nil
}

// DeployAPIConfiguration handles the complete API configuration deployment process
func (s *APIDeploymentService) DeployAPIConfiguration(params APIDeploymentParams) (*APIDeploymentResult, error) {
	var apiConfig api.APIConfiguration
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	switch apiConfig.Kind {
	case "http/rest":
		return s.handleRestDeployment(apiConfig, params)
	case "async/websub":
		return s.handleAsyncDeployment(apiConfig, params)
	}
	return nil, fmt.Errorf("unsupported API kind: %s", apiConfig.Kind)
}

func (s *APIDeploymentService) handleRestAPIUpdate(name, version string, apiConfig *api.APIConfiguration, params APIDeploymentParams) (*APIDeploymentResult, error) {
	// var restData models.APIConfigData
	// if err := utils.MapToStruct(apiConfig.Data, &restData); err != nil {
	// 	return nil, fmt.Errorf("failed to map configuration data: %w", err)
	// }

	restData, err := apiConfig.Data.AsAPIConfigData()
	if err != nil {
		return nil, fmt.Errorf("failed to parse REST API data: %w", err)
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&apiConfig)
	if len(validationErrors) > 0 {
		params.Logger.Warn("Configuration validation failed",
			zap.String("api_id", params.APIID),
			zap.String("name", apiConfig.Spec.Name),
			zap.Int("num_errors", len(validationErrors)))

		for _, e := range validationErrors {
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

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredAPIConfig{
		ID:              apiID,
		Configuration:   apiConfig,
		Status:          models.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	topicsToRegister, topicsToUnregister := s.GetAllTopicsToRegisterAndUnregister(*storedCfg)

	// Try to save/update the configuration
	isUpdate, err := s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// TODO: Pre configure the dynamic forward proxy rules for event gw
	// This was communication bridge will be created on the gw startup
	// Can perform internal communication with websub hub without relying on the dynamic rules
	// Execute topic operations with wait group and errors tracking
	var wg2 sync.WaitGroup
	var regErrs int32
	var deregErrs int32

	wg2.Add(1)
	go func(list []string) {
		defer wg2.Done()
		if len(list) == 0 {
			return
		}
		fmt.Println("Topics Registering Started")
		for _, topic := range list {
			if err := s.RegisterTopicWithHub(topic, "localhost", params.Logger); err != nil {
				params.Logger.Error("Failed to register topic with WebSubHub",
					zap.Error(err),
					zap.String("topic", topic),
					zap.String("api_id", apiID))
				atomic.AddInt32(&regErrs, 1)
			} else {
				params.Logger.Info("Successfully registered topic with WebSubHub",
					zap.String("topic", topic),
					zap.String("api_id", apiID))
			}
		}
	}(topicsToRegister)

	wg2.Add(1)
	go func(list []string) {
		defer wg2.Done()
		if len(list) == 0 {
			return
		}
		fmt.Println("Topics Deregistering Started")
		for _, topic := range list {
			if err := s.UnregisterTopicWithHub(topic, "localhost", params.Logger); err != nil {
				params.Logger.Error("Failed to deregister topic from WebSubHub",
					zap.Error(err),
					zap.String("topic", topic),
					zap.String("api_id", apiID))
				atomic.AddInt32(&deregErrs, 1)
			} else {
				params.Logger.Info("Successfully deregistered topic from WebSubHub",
					zap.String("topic", topic),
					zap.String("api_id", apiID))
			}
		}
	}(topicsToUnregister)

	wg2.Wait()
	params.Logger.Info("Topic lifecycle operations completed",
		zap.String("api_id", apiID),
		zap.Int("registered", len(topicsToRegister)),
		zap.Int("deregistered", len(topicsToUnregister)),
		zap.Int("register_errors", int(regErrs)),
		zap.Int("deregister_errors", int(deregErrs)))

	// Check if topic operations failed and return error
	if regErrs > 0 || deregErrs > 0 {
		params.Logger.Error("Topic lifecycle operations failed",
			zap.Int("register_errors", int(regErrs)),
			zap.Int("deregister_errors", int(deregErrs)))
		return nil, fmt.Errorf("failed to complete topic operations: %d registration error(s), %d deregistration error(s)", regErrs, deregErrs)
	}

	// Execute snapshot update only if topic operations had no errors
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshotofAsyncAPI(ctx, params.CorrelationID); err != nil {
			params.Logger.Error("Failed to update xDS snapshot",
				zap.Error(err),
				zap.String("api_id", apiID),
				zap.String("correlation_id", params.CorrelationID))
			return
		}
	}()
	// Log success
	if isUpdate {
		params.Logger.Info("API configuration updated",
			zap.String("api_id", apiID),
			zap.String("name", asyncData.Name),
			zap.String("version", asyncData.Version),
			zap.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("API configuration created",
			zap.String("api_id", apiID),
			zap.String("name", asyncData.Name),
			zap.String("version", asyncData.Version),
			zap.String("correlation_id", params.CorrelationID))
	}
	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
}

func (s *APIDeploymentService) handleRestDeployment(apiConfig *api.APIConfiguration, params APIDeploymentParams) (*APIDeploymentResult, error) {
	// Convert map[string]interface{} to typed struct
	//var restData models.APIConfigData
	restData, err := apiConfig.Data.AsAPIConfigData()
	if err != nil {
		return nil, fmt.Errorf("failed to parse REST API data: %w", err)
	}
	// if err := MapToStruct(apiConfig.Data, &restData); err != nil {
	// 	return nil, fmt.Errorf("failed to parse REST API data: %w", err)
	// }

	validationErrors := s.validator.Validate(apiConfig)
	if len(validationErrors) > 0 {
		params.Logger.Warn("Configuration validation failed",
			zap.String("api_id", params.APIID),
			zap.String("name", restData.Name),
			zap.Int("num_errors", len(validationErrors)))

		for _, e := range validationErrors {
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

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredAPIConfig{
		ID:              apiID,
		Configuration:   *apiConfig,
		Status:          models.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		DeployedAt:      nil,
		DeployedVersion: 0,
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
			zap.String("name", apiConfig.Spec.Name),
			zap.String("version", apiConfig.Spec.Version),
			zap.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("API configuration created",
			zap.String("api_id", apiID),
			zap.String("name", apiConfig.Spec.Name),
			zap.String("version", apiConfig.Spec.Version),
			zap.String("correlation_id", params.CorrelationID))
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
			params.Logger.Error("Failed to update xDS snapshot",
				zap.Error(err),
				zap.String("api_id", apiID),
				zap.String("correlation_id", params.CorrelationID))
			return // Do not proceed with topic lifecycle if snapshot failed
		}
	}()

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil

}

func (s *APIDeploymentService) GetAllTopicsToRegisterAndUnregister(apiConfig models.StoredAPIConfig) ([]string, []string) {
	topics := s.store.TopicManager.GetAll()
	topicsToRegister := []string{}
	topicsToUnregister := []string{}
	apiTopicsPerRevision := make(map[string]bool)

	asyncData, err := apiConfig.Configuration.Data.AsWebhookAPIData()
	if err != nil {
		// Return empty lists if parsing fails
		return topicsToRegister, topicsToUnregister
	}

	// // Convert map to typed struct
	// var asyncData struct {
	// 	Context  string `json:"context"`
	// 	Version  string `json:"version"`
	// 	Channels []struct {
	// 		Path string `json:"path"`
	// 	} `json:"channels"`
	// }
	// if err := mapToStruct(apiConfig.Configuration.Data, &asyncData); err != nil {
	// 	// Return empty lists if parsing fails
	// 	return topicsToRegister, topicsToUnregister
	// }

	for _, topic := range asyncData.Channels {
		modifiedTopic := fmt.Sprintf("%s/%s%s", asyncData.Context, asyncData.Version, topic.Path)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for topic := range topics {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			topicsToUnregister = append(topicsToUnregister, topic)
			fmt.Println("Topic to unregister:", topic)
		}
	}

	for topic := range apiTopicsPerRevision {
		if _, exists := topics[topic]; !exists {
			topicsToRegister = append(topicsToRegister, topic)
		}
	}

	return topicsToRegister, topicsToUnregister
}

// saveOrUpdateConfig handles the atomic dual-write operation for saving/updating configuration
func (s *APIDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredAPIConfig, logger *zap.Logger) (bool, error) {
	// Try to save to database first (only if persistent mode)
	// configParsed, _ := s.configParser.Parse(storedCfg)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			// Check if it's a conflict (API already exists)
			if storage.IsConflictError(err) {
				apiData, err := s.apiDataFactory.FromConfiguration(&storedCfg.Configuration)
				if err != nil {
					return false, fmt.Errorf("failed to parse API data for logging: %w", err)
				}
				logger.Info("API configuration already exists in database, updating instead",
					zap.String("api_id", storedCfg.ID),
					zap.String("name", storedCfg.Configuration.Spec.Name),
					zap.String("version", storedCfg.Configuration.Spec.Version))

				// Try to update instead
				return s.updateExistingConfig(storedCfg)
			} else {
				return false, fmt.Errorf("failed to save config to database: %w", err)
			}
		}
	}

	// Try to add to in-memory store
	if err := s.store.Add(storedCfg); err != nil {
		// Rollback database write (only if persistent mode)
		if s.db != nil {
			_ = s.db.DeleteConfig(storedCfg.ID)
		}

		// Check if it's a conflict (API already exists)
		if storage.IsConflictError(err) {
			apiData, err := s.apiDataFactory.FromConfiguration(&storedCfg.Configuration)
			if err != nil {
				return false, fmt.Errorf("failed to parse API data for logging: %w", err)
			}
			logger.Info("API configuration already exists in memory, updating instead",
				zap.String("api_id", storedCfg.ID),
				zap.String("name", storedCfg.Configuration.Spec.Name),
				zap.String("version", storedCfg.Configuration.Spec.Version))

			// Try to update instead
			return s.updateExistingConfig(storedCfg)
		} else {
			return false, fmt.Errorf("failed to add config to memory store: %w", err)
		}
	}

	return false, nil // Successfully created new config
}

// updateExistingConfig updates an existing API configuration
func (s *APIDeploymentService) updateExistingConfig(newConfig *models.StoredAPIConfig) (bool, error) {
	// Get existing config
	existing, err := s.store.GetByNameVersion(newConfig.GetAPIName(), newConfig.GetAPIVersion())
	if err != nil {
		return false, fmt.Errorf("failed to get existing config: %w", err)
	}

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
		return false, fmt.Errorf("failed to update config in memory store: %w", err)
	}

	// Update the newConfig to reflect the changes
	*newConfig = *existing

	return true, nil // Successfully updated existing config
}

// registerTopicWithHub registers a topic with the WebSubHub
func (s *APIDeploymentService) RegisterTopicWithHub(topic, gwHost string, logger *zap.Logger) error {
	return s.sendTopicRequestToHub(topic, "register", gwHost, logger)
}

// unregisterTopicWithHub unregisters a topic from the WebSubHub
func (s *APIDeploymentService) UnregisterTopicWithHub(topic, gwHost string, logger *zap.Logger) error {
	return s.sendTopicRequestToHub(topic, "deregister", gwHost, logger)
}

// sendTopicRequestToHub sends a topic registration/unregistration request to the WebSubHub
func (s *APIDeploymentService) sendTopicRequestToHub(topic string, mode string, gwHost string, logger *zap.Logger) error {
	// Prepare form data
	formData := fmt.Sprintf("hub.mode=%s&hub.topic=%s", mode, topic)

	// Build target URL to gwHost reverse proxy endpoint (no proxy)
	targetURL := fmt.Sprintf("http://%s:8083/websubhub/operations", gwHost)

	// HTTP client with timeout (no proxy)
	client := &http.Client{Timeout: 5 * time.Second}

	// Retry on 404 Not Found (hub might not be ready immediately)
	const maxRetries = 5
	backoff := 500 * time.Millisecond
	var lastStatus int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", targetURL, strings.NewReader(formData))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
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

		// Retry only on 404
		if lastStatus == http.StatusNotFound || lastStatus == http.StatusServiceUnavailable && attempt < maxRetries {
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

// mapToStruct converts a map[string]interface{} to a typed struct
func mapToStruct(data map[string]interface{}, out interface{}) error {
	// Convert map -> JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal map to JSON: %w", err)
	}

	// Unmarshal JSON bytes -> target struct
	if err := json.Unmarshal(jsonBytes, out); err != nil {
		return fmt.Errorf("failed to unmarshal JSON to struct: %w", err)
	}

	return nil
}
