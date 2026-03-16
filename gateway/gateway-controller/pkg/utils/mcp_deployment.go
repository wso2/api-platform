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
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const (
	LATEST_SUPPORTED_MCP_SPEC_VERSION = "2025-06-18"
)

type MCPDeploymentParams struct {
	Data          []byte       // Raw configuration data (YAML/JSON)
	ContentType   string       // Content type for parsing
	ID            string       // ID (if provided, used for updates; if empty, generates new UUID)
	CorrelationID string       // Correlation ID for tracking
	Logger        *slog.Logger // Logger instance
}

// MCPDeploymentService provides utilities for MCP proxy configuration deployment
type MCPDeploymentService struct {
	store           *storage.ConfigStore
	db              storage.Storage
	snapshotManager *xds.SnapshotManager
	parser          *config.Parser
	validator       *config.MCPValidator
	transformer     Transformer
	policyManager   *policyxds.PolicyManager
}

// NewMCPDeploymentService creates a new MCP deployment service
func NewMCPDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
) *MCPDeploymentService {
	return &MCPDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       config.NewMCPValidator(),
		transformer:     NewMCPTransformer(),
		policyManager:   policyManager,
	}
}

// DeployMCPConfiguration handles the complete MCP configuration deployment process
func (s *MCPDeploymentService) DeployMCPConfiguration(params MCPDeploymentParams) (*APIDeploymentResult, error) {
	var existingConfig *models.StoredConfig
	var isUpdate bool

	mcpConfig, apiConfig, err := s.parseValidateAndTransform(params)
	if err != nil {
		return nil, err
	}
	// Generate API ID if not provided
	apiID := params.ID
	if apiID == "" {
		var err error
		apiID, err = GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API ID: %w", err)
		}
	}

	handle := mcpConfig.Metadata.Name

	name, version, err := ExtractNameVersion(*apiConfig)
	if err != nil {
		return nil, err
	}

	existingConfig, _ = s.store.Get(apiID)
	isUpdate = existingConfig != nil

	if s.store != nil {
		if conflicting, _ := s.store.GetByKindNameAndVersion(models.KindMcp, name, version); conflicting != nil {
			// For updates: only error if the conflict is with a different API
			// For creates: any conflict is an error
			if !isUpdate || conflicting.UUID != apiID {
				return nil, fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", storage.ErrConflict, name, version)
			}
		}

		// Check handle conflict
		if handle != "" {
			for _, c := range s.store.GetAll() {
				if c.Handle == handle {
					// For updates: only error if the conflict is with a different API
					// For creates: any conflict is an error
					if !isUpdate || c.UUID != apiID {
						return nil, fmt.Errorf("%w: configuration with handle '%s' already exists", storage.ErrConflict, handle)
					}
				}
			}
		}
	}

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.Mcp),
		Handle:              mcpConfig.Metadata.Name,
		DisplayName:         mcpConfig.Spec.DisplayName,
		Version:             mcpConfig.Spec.Version,
		Configuration:       *apiConfig,
		SourceConfiguration: *mcpConfig,
		Status:              models.StatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          nil,
		DeployedVersion:     0,
	}

	// Try to save/update the configuration
	isUpdate, err = s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// Log success
	if isUpdate {
		params.Logger.Info("MCP configuration updated",
			slog.String("api_id", apiID),
			slog.String("displayName", mcpConfig.Spec.DisplayName),
			slog.String("version", mcpConfig.Spec.Version),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("MCP configuration created",
			slog.String("api_id", apiID),
			slog.String("displayName", mcpConfig.Spec.DisplayName),
			slog.String("version", mcpConfig.Spec.Version),
			slog.String("correlation_id", params.CorrelationID))
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
			params.Logger.Error("Failed to update xDS snapshot",
				slog.Any("error", err),
				slog.String("api_id", apiID),
				slog.String("correlation_id", params.CorrelationID))
		}
	}()

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
}

// saveOrUpdateConfig handles the atomic dual-write operation for saving/updating configuration
func (s *MCPDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredConfig, logger *slog.Logger) (bool, error) {
	// Try to save to database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			// Check if it's a conflict (Configuration already exists)
			if storage.IsConflictError(err) {
				logger.Info("MCP configuration already exists in database, updating instead",
					slog.String("id", storedCfg.UUID),
					slog.String("displayName", storedCfg.DisplayName),
					slog.String("version", storedCfg.Version))

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
				slog.String("id", storedCfg.UUID),
				slog.String("displayName", storedCfg.DisplayName),
				slog.String("version", storedCfg.Version))

			// Try to update instead
			return s.updateExistingConfig(storedCfg, logger)
		} else {
			// Rollback database write (only if persistent mode)
			if s.db != nil {
				_ = s.db.DeleteConfig(storedCfg.UUID)
			}
			return false, fmt.Errorf("failed to add config to memory store: %w", err)
		}
	}

	return false, nil // Successfully created new config
}

// updateExistingConfig updates an existing API configuration
func (s *MCPDeploymentService) updateExistingConfig(newConfig *models.StoredConfig,
	logger *slog.Logger) (bool, error) {
	// Get existing config
	existing, err := s.store.GetByKindNameAndVersion(newConfig.Kind, newConfig.DisplayName, newConfig.Version)
	if err != nil || existing == nil {
		return false, fmt.Errorf("failed to get existing config: config not found")
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
					slog.Any("error", rbErr),
					slog.String("id", original.UUID),
					slog.String("displayName", original.DisplayName),
					slog.String("version", original.Version))
			}
		}
		return false, fmt.Errorf("failed to update config in memory store: %w", err)
	}

	// Update the newConfig to reflect the changes
	*newConfig = *existing

	return true, nil // Successfully updated existing config
}

func (s *MCPDeploymentService) parseValidateAndTransform(params MCPDeploymentParams) (*api.MCPProxyConfiguration, *api.RestAPI, error) {
	var mcpConfig api.MCPProxyConfiguration
	var apiConfig api.RestAPI
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &mcpConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate configuration
	validationErrors := s.validator.Validate(&mcpConfig)
	if len(validationErrors) > 0 {
		errors := make([]string, 0, len(validationErrors))
		params.Logger.Warn("Configuration validation failed",
			slog.String("api_id", params.ID),
			slog.String("name", mcpConfig.Spec.DisplayName),
			slog.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			params.Logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, nil, fmt.Errorf("configuration validation failed with %d error(s): %s", len(validationErrors), combinedMsg)
	}

	// Transform to API configuration
	apiConfigPtr, err := s.transformer.Transform(&mcpConfig, &apiConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to transform MCP configuration to API configuration: %w", err)
	}
	apiConfig = *apiConfigPtr
	return &mcpConfig, &apiConfig, nil
}

// ListMCPProxies returns all stored MCP proxy configurations
func (s *MCPDeploymentService) ListMCPProxies() []*models.StoredConfig {
	return s.store.GetAllByKind(string(api.Mcp))
}

// GetMCPProxyByHandle returns an MCP proxy configuration by its handle (metadata.name)
func (s *MCPDeploymentService) GetMCPProxyByHandle(handle string) (*models.StoredConfig, error) {
	if s.db == nil {
		return nil, storage.ErrDatabaseUnavailable
	}

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindMcp, handle)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// CreateMCPProxy is a convenience wrapper around DeployMCPConfiguration for creating MCP proxies
func (s *MCPDeploymentService) CreateMCPProxy(params MCPDeploymentParams) (*models.StoredConfig, error) {
	res, err := s.DeployMCPConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// UpdateMCPProxy updates an existing MCP proxy identified by its handle
func (s *MCPDeploymentService) UpdateMCPProxy(handle string, params MCPDeploymentParams, logger *slog.Logger) (*models.StoredConfig, error) {
	existing, err := s.GetMCPProxyByHandle(handle)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("MCP proxy configuration with handle '%s' not found", handle)
	}

	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.UUID
	res, err := s.DeployMCPConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// DeleteMCPProxy deletes an MCP proxy by handle using store/db and updates snapshot
func (s *MCPDeploymentService) DeleteMCPProxy(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	if s.db == nil {
		return nil, storage.ErrDatabaseUnavailable
	}

	// Check if config exists
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindMcp, handle)
	if err != nil {
		logger.Error("MCP proxy configuration not found",
			slog.String("handle", handle))
		return nil, fmt.Errorf("MCP proxy configuration with handle '%s' not found", handle)
	}

	// Delete from database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.DeleteConfig(cfg.UUID); err != nil {
			logger.Error("Failed to delete config from database", slog.Any("error", err))
			return nil, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}

	// Delete from in-memory store
	if err := s.store.Delete(cfg.UUID); err != nil {
		logger.Error("Failed to delete config from memory store", slog.Any("error", err))
		return nil, fmt.Errorf("failed to delete configuration from memory store: %w", err)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
		}
	}()

	// Remove derived policy configuration
	if s.policyManager != nil {
		policyID := cfg.UUID + "-policies"
		if err := s.policyManager.RemovePolicy(policyID); err != nil {
			logger.Warn("Failed to remove derived policy configuration", slog.Any("error", err), slog.String("policy_id", policyID))
		} else {
			logger.Info("Derived policy configuration removed", slog.String("policy_id", policyID))
		}
	}

	return cfg, nil
}
