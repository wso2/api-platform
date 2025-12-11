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

// LLMDeploymentParams carries input to deploy/update a provider
type LLMDeploymentParams struct {
	Data          []byte      // Raw configuration data (YAML/JSON)
	ContentType   string      // Content type for parsing
	ID            string      // Optional ID; if empty, generated
	CorrelationID string      // Correlation ID for tracking
	Logger        *zap.Logger // Logger
}

// LLMDeploymentService encapsulates validate+transform+persist+deploy for LLM Providers
type LLMDeploymentService struct {
	store             *storage.ConfigStore
	db                storage.Storage
	snapshotManager   *xds.SnapshotManager
	deploymentService *APIDeploymentService
	parser            *config.Parser
	validator         *config.LLMValidator
	transformer       Transformer
}

// NewLLMDeploymentService initializes the service
func NewLLMDeploymentService(store *storage.ConfigStore, db storage.Storage,
	snapshotManager *xds.SnapshotManager, deploymentService *APIDeploymentService) *LLMDeploymentService {
	return &LLMDeploymentService{
		store:             store,
		db:                db,
		snapshotManager:   snapshotManager,
		deploymentService: deploymentService,
		parser:            config.NewParser(),
		validator:         config.NewLLMValidator(),
		transformer:       NewLLMProviderTransformer(store),
	}
}

// DeployLLMProviderConfiguration parses, validates, transforms and persists the provider, then triggers xDS
func (s *LLMDeploymentService) DeployLLMProviderConfiguration(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	var providerConfig api.LLMProviderConfiguration
	var apiConfig api.APIConfiguration

	// Parse configuration
	if err := s.parser.Parse(params.Data, params.ContentType, &providerConfig); err != nil {
		return nil, fmt.Errorf("failed to parse provider configuration: %w", err)
	}

	// Validate
	validationErrors := s.validator.Validate(&providerConfig)
	if len(validationErrors) > 0 {
		errs := make([]string, 0, len(validationErrors))
		params.Logger.Warn("LLM provider validation failed",
			zap.String("name", providerConfig.Spec.Name),
			zap.String("version", providerConfig.Spec.Version),
			zap.Int("num_errors", len(validationErrors)))
		for i, e := range validationErrors {
			params.Logger.Warn("Validation error", zap.String("field", e.Field), zap.String("message", e.Message))
			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}
		return nil, fmt.Errorf("provider validation failed with %d error(s): %s", len(validationErrors), strings.Join(errs, "; "))
	}

	// Transform to APIConfiguration
	_, err := s.transformer.Transform(&providerConfig, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform LLM provider to API configuration: %w", err)
	}

	// Generate API ID if not provided
	apiID := params.ID
	if apiID == "" {
		apiID = generateUUID()
	}

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredConfig{
		ID:                  apiID,
		Kind:                string(api.Llmprovider),
		Configuration:       apiConfig,
		SourceConfiguration: providerConfig,
		Status:              models.StatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          nil,
		DeployedVersion:     0,
	}

	// Save or update
	isUpdate, err := s.deploymentService.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	// Log success
	if isUpdate {
		params.Logger.Info("LLM provider configuration updated",
			zap.String("api_id", apiID),
			zap.String("name", storedCfg.GetName()),
			zap.String("version", storedCfg.GetVersion()),
			zap.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("LLM provider configuration created",
			zap.String("api_id", apiID),
			zap.String("name", storedCfg.GetName()),
			zap.String("version", storedCfg.GetVersion()),
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

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// LLMTemplateParams Template params for CRUD
type LLMTemplateParams struct {
	Spec        []byte
	ContentType string
	Logger      *zap.Logger
}

// parseAndValidateLLMTemplate parses the raw spec and validates it, returning the typed template.
func (s *LLMDeploymentService) parseAndValidateLLMTemplate(params LLMTemplateParams) (*api.LLMProviderTemplate, error) {
	var tmpl api.LLMProviderTemplate
	if err := s.parser.Parse(params.Spec, params.ContentType, &tmpl); err != nil {
		return nil, fmt.Errorf("failed to parse template configuration: %w", err)
	}
	validationErrors := s.validator.Validate(&tmpl)
	if len(validationErrors) > 0 {
		errs := make([]string, 0, len(validationErrors))
		if params.Logger != nil {
			params.Logger.Warn("Template validation failed", zap.String("name", tmpl.Spec.Name), zap.Int("error_count", len(validationErrors)))
		}
		for i, e := range validationErrors {
			if params.Logger != nil {
				params.Logger.Warn("Validation error", zap.String("field", e.Field), zap.String("message", e.Message))
			}
			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}
		return nil, fmt.Errorf("template validation failed with %d error(s): %s", len(validationErrors), strings.Join(errs, "; "))
	}
	return &tmpl, nil
}

// CreateLLMProviderTemplate parses, validates, and persists a template
func (s *LLMDeploymentService) CreateLLMProviderTemplate(params LLMTemplateParams) (*models.StoredLLMProviderTemplate, error) {
	tmpl, err := s.parseAndValidateLLMTemplate(params)
	if err != nil {
		return nil, err
	}

	stored := &models.StoredLLMProviderTemplate{
		ID:            generateUUID(),
		Configuration: *tmpl,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	// persist to DB if available
	if s.db != nil {
		if sqlite, ok := s.db.(*storage.SQLiteStorage); ok {
			if err := sqlite.SaveLLMProviderTemplate(stored); err != nil {
				if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
					return nil, fmt.Errorf("template with name '%s' already exists", tmpl.Spec.Name)
				}
				return nil, fmt.Errorf("failed to save template to database: %w", err)
			}
		}
	}
	// add to memory store
	if err := s.store.AddTemplate(stored); err != nil {
		return nil, fmt.Errorf("failed to add template to memory store: %w", err)
	}
	return stored, nil
}

// UpdateLLMProviderTemplate validates and updates existing template by name
func (s *LLMDeploymentService) UpdateLLMProviderTemplate(name string, params LLMTemplateParams) (*models.StoredLLMProviderTemplate, error) {
	existing, err := s.store.GetTemplateByName(name)
	if err != nil {
		return nil, fmt.Errorf("template with name '%s' not found", name)
	}

	tmpl, err := s.parseAndValidateLLMTemplate(params)
	if err != nil {
		return nil, err
	}

	updated := &models.StoredLLMProviderTemplate{
		ID:            existing.ID,
		Configuration: *tmpl,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     time.Now(),
	}
	if s.db != nil {
		if err := s.db.UpdateLLMProviderTemplate(updated); err != nil {
			return nil, fmt.Errorf("failed to update template in database: %w", err)
		}
	}
	if err := s.store.UpdateTemplate(updated); err != nil {
		return nil, fmt.Errorf("failed to update template in memory store: %w", err)
	}
	return updated, nil
}

// DeleteLLMProviderTemplate deletes a template by name
func (s *LLMDeploymentService) DeleteLLMProviderTemplate(name string) (*models.StoredLLMProviderTemplate, error) {
	tmpl, err := s.store.GetTemplateByName(name)
	if err != nil {
		return nil, fmt.Errorf("template with name '%s' not found", name)
	}
	if s.db != nil {
		if sqlite, ok := s.db.(*storage.SQLiteStorage); ok {
			if err := sqlite.DeleteLLMProviderTemplate(tmpl.ID); err != nil {
				return nil, fmt.Errorf("failed to delete template from database: %w", err)
			}
		}
	}
	if err := s.store.DeleteTemplate(tmpl.ID); err != nil {
		return nil, fmt.Errorf("failed to delete template from memory store: %w", err)
	}
	return tmpl, nil
}

// ListLLMProviderTemplates returns all templates
func (s *LLMDeploymentService) ListLLMProviderTemplates() []*models.StoredLLMProviderTemplate {
	return s.store.GetAllTemplates()
}

// GetLLMProviderTemplateByName returns template by name
func (s *LLMDeploymentService) GetLLMProviderTemplateByName(name string) (*models.StoredLLMProviderTemplate, error) {
	return s.store.GetTemplateByName(name)
}

// ListLLMProviders returns all stored LLM provider configurations
func (s *LLMDeploymentService) ListLLMProviders() []*models.StoredConfig {
	return s.store.GetAllByKind(string(api.Llmprovider))
}

// CreateLLMProvider is a convenience wrapper around DeployLLMProviderConfiguration for creating providers
func (s *LLMDeploymentService) CreateLLMProvider(params LLMDeploymentParams) (*models.StoredConfig, error) {
	res, err := s.DeployLLMProviderConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// UpdateLLMProvider updates an existing provider identified by name+version using DeployLLMProviderConfiguration
func (s *LLMDeploymentService) UpdateLLMProvider(name, version string, params LLMDeploymentParams) (*models.StoredConfig, error) {
	existing := s.store.GetByKindNameAndVersion(string(api.Llmprovider), name, version)
	if existing == nil {
		return nil, fmt.Errorf("LLM provider configuration with name '%s' and version '%s' not found", name, version)
	}
	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.ID
	res, err := s.DeployLLMProviderConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// DeleteLLMProvider deletes by name+version using store/db and updates snapshot
func (s *LLMDeploymentService) DeleteLLMProvider(name, version, correlationID string, logger *zap.Logger) (*models.StoredConfig, error) {
	cfg := s.store.GetByKindNameAndVersion(string(api.Llmprovider), name, version)
	if cfg == nil {
		return cfg, fmt.Errorf("LLM provider configuration with name '%s' and version '%s' not found", name, version)
	}
	if s.db != nil {
		if err := s.db.DeleteConfig(cfg.ID); err != nil {
			return cfg, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}
	if err := s.store.Delete(cfg.ID); err != nil {
		return cfg, fmt.Errorf("failed to delete configuration from memory store: %w", err)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			logger.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	return cfg, nil
}
