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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// LLMDeploymentParams carries input to deploy/update a provider
type LLMDeploymentParams struct {
	Data          []byte       // Raw configuration data (YAML/JSON)
	ContentType   string       // Content type for parsing
	ID            string       // Optional ID; if empty, generated
	CorrelationID string       // Correlation ID for tracking
	Logger        *slog.Logger // Logger
}

// LLMDeploymentService encapsulates validate+transform+persist+deploy for LLM Providers
type LLMDeploymentService struct {
	store               *storage.ConfigStore
	db                  storage.Storage
	snapshotManager     *xds.SnapshotManager
	templateDefinitions map[string]*api.LLMProviderTemplate
	deploymentService   *APIDeploymentService
	parser              *config.Parser
	validator           *config.LLMValidator
	transformer         Transformer
	routerConfig        *config.RouterConfig
}

// NewLLMDeploymentService initializes the service
func NewLLMDeploymentService(store *storage.ConfigStore, db storage.Storage,
	snapshotManager *xds.SnapshotManager, templateDefinitions map[string]*api.LLMProviderTemplate,
	deploymentService *APIDeploymentService, routerConfig *config.RouterConfig) *LLMDeploymentService {
	service := &LLMDeploymentService{
		store:               store,
		db:                  db,
		snapshotManager:     snapshotManager,
		templateDefinitions: templateDefinitions,
		deploymentService:   deploymentService,
		parser:              config.NewParser(),
		validator:           config.NewLLMValidator(),
		transformer:         NewLLMProviderTransformer(store, routerConfig),
	}

	// Initialize OOB templates
	if err := service.InitializeOOBTemplates(templateDefinitions); err != nil {
		slog.Error("Failed to initialize out-of-the-box LLM provider templates", slog.Any("error", err))
	}

	return service
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
			slog.String("handle", providerConfig.Metadata.Name),
			slog.Int("num_errors", len(validationErrors)))
		for i, e := range validationErrors {
			params.Logger.Warn("Validation error", slog.String("field", e.Field), slog.String("message", e.Message))
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
		Kind:                string(api.LlmProvider),
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
		return nil, fmt.Errorf("failed to save or update LLM provider configuration: %w", err)
	}

	// Log success
	if isUpdate {
		params.Logger.Info("LLM provider configuration updated",
			slog.String("api_uuid", apiID),
			slog.String("handle", storedCfg.GetHandle()),
			slog.String("display_name", storedCfg.GetDisplayName()),
			slog.String("version", storedCfg.GetVersion()),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("LLM provider configuration created",
			slog.String("api_uuid", apiID),
			slog.String("handle", storedCfg.GetHandle()),
			slog.String("display_name", storedCfg.GetDisplayName()),
			slog.String("version", storedCfg.GetVersion()),
			slog.String("correlation_id", params.CorrelationID))
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
			params.Logger.Error("Failed to update xDS snapshot",
				slog.Any("error", err),
				slog.String("api_uuid", apiID),
				slog.String("correlation_id", params.CorrelationID))
		}
	}()

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// DeployLLMProxyConfiguration parses, validates, transforms and persists the provider, then triggers xDS
func (s *LLMDeploymentService) DeployLLMProxyConfiguration(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	var proxyConfig api.LLMProxyConfiguration
	var apiConfig api.APIConfiguration

	// Parse configuration
	if err := s.parser.Parse(params.Data, params.ContentType, &proxyConfig); err != nil {
		return nil, fmt.Errorf("failed to parse proxy configuration: %w", err)
	}

	// Validate
	validationErrors := s.validator.Validate(&proxyConfig)
	if len(validationErrors) > 0 {
		errs := make([]string, 0, len(validationErrors))
		params.Logger.Warn("LLM proxy validation failed",
			slog.String("handle", proxyConfig.Metadata.Name),
			slog.Int("num_errors", len(validationErrors)))
		for i, e := range validationErrors {
			params.Logger.Warn("Validation error", slog.String("field", e.Field), slog.String("message", e.Message))
			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}
		return nil, fmt.Errorf("proxy validation failed with %d error(s): %s", len(validationErrors), strings.Join(errs, "; "))
	}

	// Transform to APIConfiguration
	_, err := s.transformer.Transform(&proxyConfig, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform LLM proxy to API configuration: %w", err)
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
		Kind:                string(api.LlmProxy),
		Configuration:       apiConfig,
		SourceConfiguration: proxyConfig,
		Status:              models.StatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          nil,
		DeployedVersion:     0,
	}

	// Save or update
	isUpdate, err := s.deploymentService.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to save or update LLM proxy configuration: %w", err)
	}
	// Log success
	if isUpdate {
		params.Logger.Info("LLM proxy configuration updated",
			slog.String("api_uuid", apiID),
			slog.String("handle", storedCfg.GetHandle()),
			slog.String("display_name", storedCfg.GetDisplayName()),
			slog.String("version", storedCfg.GetVersion()),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("LLM proxy configuration created",
			slog.String("api_uuid", apiID),
			slog.String("handle", storedCfg.GetHandle()),
			slog.String("display_name", storedCfg.GetDisplayName()),
			slog.String("version", storedCfg.GetVersion()),
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

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// LLMTemplateParams Template params for CRUD
type LLMTemplateParams struct {
	Spec        []byte
	ContentType string
	Logger      *slog.Logger
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
			params.Logger.Warn("Template validation failed", slog.String("handle", tmpl.Metadata.Name), slog.Int("error_count", len(validationErrors)))
		}
		for i, e := range validationErrors {
			if params.Logger != nil {
				params.Logger.Warn("Validation error", slog.String("field", e.Field), slog.String("message", e.Message))
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
					return nil, fmt.Errorf("template with handle '%s' already exists", tmpl.Metadata.Name)
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

// InitializeOOBTemplates persists OOB templates to database and memory store
func (s *LLMDeploymentService) InitializeOOBTemplates(templateDefinitions map[string]*api.LLMProviderTemplate) error {
	if len(templateDefinitions) == 0 {
		return nil
	}

	var allErrors []string

	for _, tmpl := range templateDefinitions {
		// Validate the template configuration
		validationErrors := s.validator.Validate(tmpl)
		if len(validationErrors) > 0 {
			errs := make([]string, 0, len(validationErrors))
			for _, ve := range validationErrors {
				errs = append(errs, fmt.Sprintf("%s: %s", ve.Field, ve.Message))
			}
			allErrors = append(allErrors, fmt.Sprintf(
				"template '%s' validation failed: %s",
				tmpl.Metadata.Name,
				strings.Join(errs, "; "),
			))
			continue
		}

		// Check if template already exists
		existing, err := s.store.GetTemplateByHandle(tmpl.Metadata.Name)
		if err == nil && existing != nil {
			// ---------------------------
			// UPDATE existing template
			// ---------------------------

			updated := &models.StoredLLMProviderTemplate{
				ID:            existing.ID,
				Configuration: *tmpl,
				CreatedAt:     existing.CreatedAt,
				UpdatedAt:     time.Now(),
			}

			// Update DB
			if s.db != nil {
				if err := s.db.UpdateLLMProviderTemplate(updated); err != nil {
					allErrors = append(allErrors,
						fmt.Sprintf("failed to update template '%s' in database: %v", tmpl.Metadata.Name, err))
					continue
				}
			}

			// Update memory store
			if err := s.store.UpdateTemplate(updated); err != nil {
				allErrors = append(allErrors,
					fmt.Sprintf("failed to update template '%s' in memory store: %v", tmpl.Metadata.Name, err))
				continue
			}

			continue
		}

		// ---------------------------
		// CREATE new template
		// ---------------------------

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
						continue
					}
					allErrors = append(allErrors, fmt.Sprintf("failed to save template '%s' to database: %v",
						tmpl.Metadata.Name, err))
					continue
				}
			}
		}

		// add to memory store
		if err := s.store.AddTemplate(stored); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			allErrors = append(allErrors, fmt.Sprintf("failed to add template '%s' to memory store: %v",
				tmpl.Metadata.Name, err))
			continue
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("failed to initialize %d template(s): %s", len(allErrors),
			strings.Join(allErrors, "; "))
	}

	return nil
}

// UpdateLLMProviderTemplate validates and updates existing template by handle
func (s *LLMDeploymentService) UpdateLLMProviderTemplate(handle string, params LLMTemplateParams) (*models.StoredLLMProviderTemplate, error) {
	existing, err := s.store.GetTemplateByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("template with handle '%s' not found", handle)
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
func (s *LLMDeploymentService) DeleteLLMProviderTemplate(handle string) (*models.StoredLLMProviderTemplate, error) {
	tmpl, err := s.store.GetTemplateByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("template with handle '%s' not found", handle)
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

// ListLLMProviderTemplates retrieves all LLM provider templates, optionally filtered by display name.
// If displayName is nil or empty, all templates are returned.
func (s *LLMDeploymentService) ListLLMProviderTemplates(displayName *string) []*models.StoredLLMProviderTemplate {
	templates := s.store.GetAllTemplates()

	// Return all templates if no filter is specified
	if displayName == nil || *displayName == "" {
		return templates
	}

	// Filter templates by display name
	filtered := make([]*models.StoredLLMProviderTemplate, 0, len(templates))
	for _, template := range templates {
		if template.Configuration.Spec.DisplayName == *displayName {
			filtered = append(filtered, template)
		}
	}

	return filtered
}

// GetLLMProviderTemplateByHandle returns template by handle
func (s *LLMDeploymentService) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	return s.store.GetTemplateByHandle(handle)
}

// CreateLLMProvider is a convenience wrapper around DeployLLMProviderConfiguration for creating providers
func (s *LLMDeploymentService) CreateLLMProvider(params LLMDeploymentParams) (*models.StoredConfig, error) {
	res, err := s.DeployLLMProviderConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// ListLLMProviders returns all stored LLM provider configurations with optional filtering
func (s *LLMDeploymentService) ListLLMProviders(params api.ListLLMProvidersParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(string(api.LlmProvider))

	// If no filters are provided, return all configs
	if params.DisplayName == nil && params.Version == nil &&
		params.Context == nil && params.Status == nil && params.Vhost == nil {
		return configs
	}

	// Filter configs based on provided parameters
	filtered := make([]*models.StoredConfig, 0, len(configs))

	for _, cfg := range configs {
		if !matchesFilters(cfg, params) {
			continue
		}
		filtered = append(filtered, cfg)
	}

	return filtered
}

// matchesFilters checks if a config matches all provided filter criteria
func matchesFilters(config *models.StoredConfig, params any) bool {
	apiCfg, err := config.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		return false
	}

	var name, version, cnt, vhost, status *string

	switch p := params.(type) {
	case api.ListLLMProvidersParams:
		name, version, cnt, status, vhost = p.DisplayName, p.Version, p.Context, (*string)(p.Status), p.Vhost
	case api.ListLLMProxiesParams:
		name, version, cnt, status, vhost = p.DisplayName, p.Version, p.Context, (*string)(p.Status), p.Vhost
	default:
		return false
	}

	// Check DisplayName filter
	if name != nil && apiCfg.DisplayName != *name {
		return false
	}

	// Check Version filter
	if version != nil && apiCfg.Version != *version {
		return false
	}

	// Check Context filter
	if cnt != nil && apiCfg.Context != *cnt {
		return false
	}

	// Check Status filter
	if status != nil && string(config.Status) != string(*status) {
		return false
	}

	// Check Vhost filter
	if vhost != nil {
		if apiCfg.Vhosts == nil || apiCfg.Vhosts.Main != *vhost {
			return false
		}
	}

	return true
}

// UpdateLLMProvider updates an existing provider identified by name+version using DeployLLMProviderConfiguration
func (s *LLMDeploymentService) UpdateLLMProvider(handle string, params LLMDeploymentParams) (*models.StoredConfig, error) {
	existing := s.store.GetByKindAndHandle(string(api.LlmProvider), handle)
	if existing == nil {
		return nil, fmt.Errorf("LLM provider configuration with handle '%s' not found", handle)
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
func (s *LLMDeploymentService) DeleteLLMProvider(handle, correlationID string,
	logger *slog.Logger) (*models.StoredConfig, error) {
	cfg := s.store.GetByKindAndHandle(string(api.LlmProvider), handle)
	if cfg == nil {
		return cfg, fmt.Errorf("LLM provider configuration with handle '%s' not found", handle)
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
			logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
		}
	}()

	return cfg, nil
}

// ListLLMProxies returns all stored LLM proxy configurations
func (s *LLMDeploymentService) ListLLMProxies(params api.ListLLMProxiesParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(string(api.LlmProxy))

	// If no filters are provided, return all configs
	if params.DisplayName == nil && params.Version == nil &&
		params.Context == nil && params.Status == nil && params.Vhost == nil {
		return configs
	}

	// Filter configs based on provided parameters
	filtered := make([]*models.StoredConfig, 0, len(configs))

	for _, cfg := range configs {
		if !matchesFilters(cfg, params) {
			continue
		}
		filtered = append(filtered, cfg)
	}

	return filtered
}

// CreateLLMProxy is a convenience wrapper around DeployLLMProxyConfiguration for creating proxies
func (s *LLMDeploymentService) CreateLLMProxy(params LLMDeploymentParams) (*models.StoredConfig, error) {
	res, err := s.DeployLLMProxyConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// UpdateLLMProxy updates an existing provider identified by name+version using DeployLLMProxyConfiguration
func (s *LLMDeploymentService) UpdateLLMProxy(id string, params LLMDeploymentParams) (*models.StoredConfig, error) {
	existing := s.store.GetByKindAndHandle(string(api.LlmProxy), id)
	if existing == nil {
		return nil, fmt.Errorf("LLM proxy configuration with handle '%s' not found", id)
	}
	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.ID
	res, err := s.DeployLLMProxyConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// DeleteLLMProxy deletes by name+version using store/db and updates snapshot
func (s *LLMDeploymentService) DeleteLLMProxy(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	cfg := s.store.GetByKindAndHandle(string(api.LlmProxy), handle)
	if cfg == nil {
		return cfg, fmt.Errorf("LLM proxy configuration with handle '%s' not found", handle)
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
			logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
		}
	}()

	return cfg, nil
}
