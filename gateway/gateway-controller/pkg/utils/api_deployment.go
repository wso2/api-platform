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
	validator       *config.Validator
}

// NewAPIDeploymentService creates a new API deployment service
func NewAPIDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
) *APIDeploymentService {
	return &APIDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       config.NewValidator(),
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
			zap.String("name", apiConfig.Data.Name),
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

	// Try to save/update the configuration
	isUpdate, err := s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// Log success
	if isUpdate {
		params.Logger.Info("API configuration updated",
			zap.String("api_id", apiID),
			zap.String("name", apiConfig.Data.Name),
			zap.String("version", apiConfig.Data.Version),
			zap.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("API configuration created",
			zap.String("api_id", apiID),
			zap.String("name", apiConfig.Data.Name),
			zap.String("version", apiConfig.Data.Version),
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
		}
	}()

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
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
					zap.String("name", storedCfg.Configuration.Data.Name),
					zap.String("version", storedCfg.Configuration.Data.Version))

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
				zap.String("name", storedCfg.Configuration.Data.Name),
				zap.String("version", storedCfg.Configuration.Data.Version))

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

// generateUUID generates a new UUID string
func generateUUID() string {
	return uuid.New().String()
}
