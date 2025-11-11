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
	"strings"
	"sync"
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

// DeployAPIConfiguration handles the complete API configuration deployment process
func (s *APIDeploymentService) DeployAPIConfiguration(params APIDeploymentParams) (*APIDeploymentResult, error) {
	var apiConfig api.APIConfiguration
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
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

	// Before saving to the cache we get the topics to register and unregister per API
	// Topic URLs are in the format of /<context>/<version>/<topic_path> which is unique per API
	// We maintain only the latest revision of the API in the store so we can get the delta topics here
	topicsToRegister, topicsToUnregister := s.getAllTopicsToRegisterAndUnregister(*storedCfg)

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

		if apiConfig.Kind == "http/websub" {
			if err := s.snapshotManager.UpdateSnapshotofAsyncAPI(ctx, params.CorrelationID); err != nil {
				params.Logger.Error("Failed to update xDS snapshot",
					zap.Error(err),
					zap.String("api_id", apiID),
					zap.String("correlation_id", params.CorrelationID))
				return // Do not proceed with topic lifecycle if snapshot failed
			}

			// Grace period to let Envoy apply new resources
			select {
			case <-time.After(6 * time.Second):
			case <-ctx.Done():
				params.Logger.Warn("Context cancelled before topic lifecycle",
					zap.String("api_id", apiID))
				return
			}

			var wg sync.WaitGroup

			// Register topics
			if len(topicsToRegister) > 0 {
				wg.Add(1)
				go func(list []string) {
					defer wg.Done()
					for _, topic := range list {
						if err := s.registerTopicWithHub(topic, "http://localhost", params.Logger); err != nil {
							params.Logger.Error("Failed to register topic with WebSubHub",
								zap.Error(err),
								zap.String("topic", topic),
								zap.String("api_id", apiID))
						} else {
							params.Logger.Info("Successfully registered topic with WebSubHub",
								zap.String("topic", topic),
								zap.String("api_id", apiID))
						}
					}
				}(topicsToRegister)
			}

			// Deregister topics
			if len(topicsToUnregister) > 0 {
				wg.Add(1)
				go func(list []string) {
					defer wg.Done()
					for _, topic := range list {
						if err := s.unregisterTopicWithHub(topic, "http://localhost", params.Logger); err != nil {
							params.Logger.Error("Failed to deregister topic from WebSubHub",
								zap.Error(err),
								zap.String("topic", topic),
								zap.String("api_id", apiID))
						} else {
							params.Logger.Info("Successfully deregistered topic from WebSubHub",
								zap.String("topic", topic),
								zap.String("api_id", apiID))
						}
					}
				}(topicsToUnregister)
			}

			// Log completion (non-blocking to caller)
			go func() {
				wg.Wait()
				params.Logger.Info("Topic lifecycle operations completed",
					zap.String("api_id", apiID),
					zap.Int("registered", len(topicsToRegister)),
					zap.Int("deregistered", len(topicsToUnregister)))
			}()

		} else {
			if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
				params.Logger.Error("Failed to update xDS snapshot",
					zap.Error(err),
					zap.String("api_id", apiID),
					zap.String("correlation_id", params.CorrelationID))
			}
		}
	}()

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
}

func (s *APIDeploymentService) getAllTopicsToRegisterAndUnregister(apiConfig models.StoredAPIConfig) ([]string, []string) {
	topics := s.store.TopicManager.GetAll()
	topicsToRegister := []string{}
	topicsToUnregister := []string{}
	apiTopicsPerRevision := make(map[string]bool)
	for _, topic := range apiConfig.Configuration.Data.Operations {
		modifiedTopic := fmt.Sprintf("%s/%s%s", apiConfig.Configuration.Data.Context, apiConfig.Configuration.Data.Version, topic.Path)
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
			fmt.Println("Topic to register:", topic)
		}
	}

	return topicsToRegister, topicsToUnregister
}

// saveOrUpdateConfig handles the atomic dual-write operation for saving/updating configuration
func (s *APIDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredAPIConfig, logger *zap.Logger) (bool, error) {
	// Try to save to database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			// Check if it's a conflict (API already exists)
			if storage.IsConflictError(err) {
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
func (s *APIDeploymentService) registerTopicWithHub(topic, gwHost string, logger *zap.Logger) error {
	return s.sendTopicRequestToHub(topic, "register", gwHost, logger)
}

// unregisterTopicWithHub unregisters a topic from the WebSubHub
func (s *APIDeploymentService) unregisterTopicWithHub(topic, gwHost string, logger *zap.Logger) error {
	return s.sendTopicRequestToHub(topic, "deregister", gwHost, logger)
}

// sendTopicRequestToHub sends a topic registration/unregistration request to the WebSubHub
func (s *APIDeploymentService) sendTopicRequestToHub(topic string, mode string, gwHost string, logger *zap.Logger) error {
	// Prepare form data
	formData := fmt.Sprintf("hub.mode=%s&hub.topic=%s", mode, topic)

	endpoint := fmt.Sprintf("%s:8080%s", gwHost, topic)

	// HTTP client with timeout
	client := &http.Client{Timeout: 5 * time.Second}

	// Retry on 404 Not Found (hub might not be ready immediately)
	const maxRetries = 5
	backoff := 500 * time.Millisecond
	var lastStatus int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create a fresh request each attempt (body readers are one-shot)
		req, err := http.NewRequest("POST", endpoint, strings.NewReader(formData))
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
			// Exponential backoff
			backoff *= 2
			// Reset lastStatus for next attempt's success detection
			lastStatus = 0
			continue
		}

		// Non-retryable status or retries exhausted
		return fmt.Errorf("WebSubHub returned non-success status: %d", lastStatus)
	}

	return fmt.Errorf("WebSubHub request failed after %d retries; last status: %d", maxRetries, lastStatus)
}

// generateUUID generates a new UUID string
func generateUUID() string {
	return uuid.New().String()
}
