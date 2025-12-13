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
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

const (
	LATEST_SUPPORTED_MCP_SPEC_VERSION = "2025-06-18"
)

type MCPDeploymentParams struct {
	Data          []byte      // Raw configuration data (YAML/JSON)
	ContentType   string      // Content type for parsing
	ID            string      // ID (if provided, used for updates; if empty, generates new UUID)
	CorrelationID string      // Correlation ID for tracking
	Logger        *zap.Logger // Logger instance
}

// MCPDeploymentService provides utilities for MCP proxy configuration deployment
type MCPDeploymentService struct {
	store           *storage.ConfigStore
	db              storage.Storage
	snapshotManager *xds.SnapshotManager
	parser          *config.Parser
	validator       config.Validator
	transformer     Transformer
}

// NewMCPDeploymentService creates a new MCP deployment service
func NewMCPDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
) *MCPDeploymentService {
	return &MCPDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       config.NewMCPValidator(),
		transformer:     &MCPTransformer{},
	}
}

// DeployMCPConfiguration handles the complete MCP configuration deployment process
func (s *MCPDeploymentService) DeployMCPConfiguration(params MCPDeploymentParams) (*APIDeploymentResult, error) {
	var mcpConfig api.MCPProxyConfiguration
	var apiConfig api.APIConfiguration
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &mcpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&mcpConfig)
	if len(validationErrors) > 0 {
		errors := make([]string, 0, len(validationErrors))
		params.Logger.Warn("Configuration validation failed",
			zap.String("api_id", params.ID),
			zap.String("displayName", mcpConfig.Spec.DisplayName),
			zap.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			params.Logger.Warn("Validation error",
				zap.String("field", e.Field),
				zap.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, fmt.Errorf("configuration validation failed with %d error(s): %s", len(validationErrors), combinedMsg)
	}

	// Generate API ID if not provided
	apiID := params.ID
	if apiID == "" {
		apiID = generateUUID()
	}

	// Transform to API configuration
	apiConfigPtr, err := s.transformer.Transform(&mcpConfig, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform MCP configuration to API configuration")
	}
	apiConfig = *apiConfigPtr

	handle := apiConfig.Metadata.Name

	name, version, err := ExtractNameVersion(apiConfig)
	if err != nil {
		return nil, err
	}

	if s.store != nil {
		if name != "" && version != "" {
			if _, err := s.store.GetByNameVersion(name, version); err == nil {
				return nil, fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", storage.ErrConflict, name, version)
			}
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
		Kind:                string(api.Mcp),
		Status:              models.StatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          nil,
		DeployedVersion:     0,
		Configuration:       apiConfig,
		SourceConfiguration: mcpConfig,
	}

	// Try to save/update the configuration
	isUpdate, err := s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// Log success
	if isUpdate {
		params.Logger.Info("MCP configuration updated",
			zap.String("api_id", apiID),
			zap.String("displayName", mcpConfig.Spec.DisplayName),
			zap.String("version", mcpConfig.Spec.Version),
			zap.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("MCP configuration created",
			zap.String("api_id", apiID),
			zap.String("displayName", mcpConfig.Spec.DisplayName),
			zap.String("version", mcpConfig.Spec.Version),
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
func (s *MCPDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredConfig, logger *zap.Logger) (bool, error) {
	// Try to save to database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			// Check if it's a conflict (Configuration already exists)
			if storage.IsConflictError(err) {
				logger.Info("MCP configuration already exists in database, updating instead",
					zap.String("id", storedCfg.ID),
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
	if err := s.store.Add(storedCfg); err != nil {
		// Check if it's a conflict (API already exists)
		if storage.IsConflictError(err) {
			logger.Info("MCP configuration already exists in memory, updating instead",
				zap.String("id", storedCfg.ID),
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

	return false, nil // Successfully created new config
}

// updateExistingConfig updates an existing API configuration
func (s *MCPDeploymentService) updateExistingConfig(newConfig *models.StoredConfig,
	logger *zap.Logger) (bool, error) {
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
