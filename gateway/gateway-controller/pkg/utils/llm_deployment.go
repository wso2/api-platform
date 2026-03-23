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
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const (
	LazyResourceTypeLLMProviderTemplate     = "LlmProviderTemplate"
	LazyResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"
)

// LLMDeploymentParams carries input to deploy/update a provider
type LLMDeploymentParams struct {
	Data          []byte        // Raw configuration data (YAML/JSON)
	ContentType   string        // Content type for parsing
	ID            string        // Optional ID; if empty, generated
	DeploymentID  string        // Platform deployment ID (empty for gateway-api origin)
	Origin        models.Origin // Origin of the deployment: "control_plane" or "gateway_api"
	DeployedAt    *time.Time    // Deployment timestamp from platform event (nil for gateway-api origin)
	CorrelationID string        // Correlation ID for tracking
	Logger        *slog.Logger  // Logger
}

// LLMDeploymentService encapsulates validate+transform+persist+deploy for LLM Providers
type LLMDeploymentService struct {
	store                        *storage.ConfigStore
	db                           storage.Storage
	snapshotManager              *xds.SnapshotManager
	lazyResourceManager          *lazyresourcexds.LazyResourceStateManager
	templateDefinitions          map[string]*api.LLMProviderTemplate
	deploymentService            *APIDeploymentService
	parser                       *config.Parser
	validator                    *config.LLMValidator
	policyValidator              *config.PolicyValidator
	transformer                  Transformer
	routerConfig                 *config.RouterConfig
	policyManager                *policyxds.PolicyManager
	policyRouteConfigTransformer models.ConfigTransformer
}

// NewLLMDeploymentService initializes the service
func NewLLMDeploymentService(store *storage.ConfigStore, db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	lazyResourceManager *lazyresourcexds.LazyResourceStateManager,
	templateDefinitions map[string]*api.LLMProviderTemplate,
	deploymentService *APIDeploymentService,
	routerConfig *config.RouterConfig,
	policyVersionResolver PolicyVersionResolver,
	policyValidator *config.PolicyValidator,
	policyManager *policyxds.PolicyManager,
	policyRouteConfigTransformer models.ConfigTransformer,
) *LLMDeploymentService {
	service := &LLMDeploymentService{
		store:                        store,
		db:                           db,
		snapshotManager:              snapshotManager,
		lazyResourceManager:          lazyResourceManager,
		templateDefinitions:          templateDefinitions,
		deploymentService:            deploymentService,
		parser:                       config.NewParser(),
		validator:                    config.NewLLMValidator(),
		policyValidator:              policyValidator,
		transformer:                  NewLLMProviderTransformer(store, db, routerConfig, policyVersionResolver),
		policyManager:                policyManager,
		policyRouteConfigTransformer: policyRouteConfigTransformer,
	}

	// Initialize OOB templates
	if err := service.InitializeOOBTemplates(templateDefinitions); err != nil {
		slog.Error("Failed to initialize out-of-the-box LLM provider templates", slog.Any("error", err))
	}
	if err := service.InitializeExistingLLMState(); err != nil {
		slog.Warn("Failed to initialize stored LLM state", slog.Any("error", err))
	}

	return service
}

// SetEventHub configures EventHub publishing for replica-synced LLM provider flows.
func (s *LLMDeploymentService) SetEventHub(eventHub eventhub.EventHub, gatewayID string) {
	if s.deploymentService != nil {
		s.deploymentService.SetEventHub(eventHub, gatewayID)
	}
}

func (s *LLMDeploymentService) isEventDriven() bool {
	return s.deploymentService != nil && s.deploymentService.eventHub != nil
}

func (s *LLMDeploymentService) publishLLMProviderEvent(action, entityID, correlationID string, logger *slog.Logger) {
	if s.deploymentService == nil {
		return
	}
	s.deploymentService.publishEvent(eventhub.EventTypeLLMProvider, action, entityID, correlationID, logger)
}

func (s *LLMDeploymentService) publishLLMProxyEvent(action, entityID, correlationID string, logger *slog.Logger) {
	if s.deploymentService == nil {
		return
	}
	s.deploymentService.publishEvent(eventhub.EventTypeLLMProxy, action, entityID, correlationID, logger)
}

func (s *LLMDeploymentService) publishLLMTemplateEvent(action, entityID, correlationID string, logger *slog.Logger) {
	if s.deploymentService == nil {
		return
	}
	s.deploymentService.publishEvent(eventhub.EventTypeLLMTemplate, action, entityID, correlationID, logger)
}

// LLM configs are persisted as their original management payloads. The derived
// RestAPI form is rebuilt on demand because it depends on in-memory template and
// policy state that is not stored in SQL.
func (s *LLMDeploymentService) hydrateStoredLLMConfig(cfg *models.StoredConfig) error {
	if cfg == nil {
		return nil
	}

	switch src := cfg.SourceConfiguration.(type) {
	case api.LLMProviderConfiguration:
		var restAPI api.RestAPI
		if _, err := s.transformer.Transform(&src, &restAPI); err != nil {
			return fmt.Errorf("failed to transform stored LLM provider %s: %w", cfg.UUID, err)
		}
		cfg.Configuration = restAPI
	case api.LLMProxyConfiguration:
		var restAPI api.RestAPI
		if _, err := s.transformer.Transform(&src, &restAPI); err != nil {
			return fmt.Errorf("failed to transform stored LLM proxy %s: %w", cfg.UUID, err)
		}
		cfg.Configuration = restAPI
	}

	return nil
}

// Startup can populate the in-memory store from disk before any EventHub replay
// happens. Rehydrate those entries and restore provider-template mappings so the
// local replica has the same derived state it would get from an event replay.
func (s *LLMDeploymentService) InitializeExistingLLMState() error {
	var errs []string

	for _, cfg := range s.store.GetAllByKind(string(api.LlmProvider)) {
		if err := s.hydrateStoredLLMConfig(cfg); err != nil {
			errs = append(errs, err.Error())
			continue
		}

		providerCfg, ok := cfg.SourceConfiguration.(api.LLMProviderConfiguration)
		if !ok {
			continue
		}
		if err := s.publishProviderTemplateMappingAsLazyResource(cfg.Handle, providerCfg.Spec.Template, ""); err != nil {
			errs = append(errs, fmt.Sprintf("failed to publish provider-template mapping for %s: %v", cfg.Handle, err))
		}
	}

	for _, cfg := range s.store.GetAllByKind(string(api.LlmProxy)) {
		if err := s.hydrateStoredLLMConfig(cfg); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil
}

// updatePolicyRouteConfig transforms cfg and upserts it into the policy route config manager.
func (s *LLMDeploymentService) updatePolicyRouteConfig(cfg *models.StoredConfig, logger *slog.Logger) {
	if s.policyManager == nil || s.policyRouteConfigTransformer == nil {
		return
	}
	rdc, err := s.policyRouteConfigTransformer.Transform(cfg)
	if err != nil {
		logger.Error("Failed to transform LLM config for policy route config", slog.Any("error", err))
		return
	}
	key := storage.Key(cfg.Kind, cfg.Handle)
	if err := s.policyManager.AddRuntimeConfig(key, rdc); err != nil {
		logger.Error("Failed to update policy route config for LLM", slog.Any("error", err))
	}
}

// DeployLLMProviderConfiguration parses, validates, transforms and persists the provider, then triggers xDS
func (s *LLMDeploymentService) DeployLLMProviderConfiguration(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	if !models.IsValidOrigin(params.Origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", params.Origin)
	}

	var providerConfig api.LLMProviderConfiguration
	var apiConfig api.RestAPI

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

	// Transform to RestAPI configuration
	_, err := s.transformer.Transform(&providerConfig, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform LLM provider to API configuration: %w", err)
	}

	// Validate policies against loaded policy definitions
	// if s.policyValidator != nil {
	// 	policyErrors := s.policyValidator.ValidatePolicies(&apiConfig)
	// 	if len(policyErrors) > 0 {
	// 		errs := make([]string, 0, len(policyErrors))
	// 		for i, e := range policyErrors {
	// 			if params.Logger != nil {
	// 				params.Logger.Warn("Policy validation error", slog.String("field", e.Field), slog.String("message", e.Message))
	// 			}
	// 			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
	// 		}
	// 		return nil, fmt.Errorf("policy validation failed with %d error(s): %s", len(policyErrors), strings.Join(errs, "; "))
	// 	}
	// }

	// Generate API ID if not provided
	apiID := params.ID
	if apiID == "" {
		var err error
		apiID, err = GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API ID: %w", err)
		}
	}

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.LlmProvider),
		Handle:              providerConfig.Metadata.Name,
		DisplayName:         providerConfig.Spec.DisplayName,
		Version:             providerConfig.Spec.Version,
		Configuration:       apiConfig,
		SourceConfiguration: providerConfig,
		DesiredState:        models.StateDeployed,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          params.DeployedAt,
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
			slog.String("handle", storedCfg.Handle),
			slog.String("display_name", storedCfg.DisplayName),
			slog.String("version", storedCfg.Version),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("LLM provider configuration created",
			slog.String("api_uuid", apiID),
			slog.String("handle", storedCfg.Handle),
			slog.String("display_name", storedCfg.DisplayName),
			slog.String("version", storedCfg.Version),
			slog.String("correlation_id", params.CorrelationID))
	}

	// In multi-replica mode the database write above is the only local mutation
	// on the write path. Store, lazy-resource, xDS, and policy updates happen in
	// the EventListener after all replicas consume the same event.
	if s.isEventDriven() {
		action := "CREATE"
		if isUpdate {
			action = "UPDATE"
		}
		s.publishLLMProviderEvent(action, apiID, params.CorrelationID, params.Logger)
	} else {
		// Publish provider-to-template mapping as lazy resource for policy engine
		if providerConfig.Metadata.Name != "" && providerConfig.Spec.Template != "" {
			if err := s.publishProviderTemplateMappingAsLazyResource(
				providerConfig.Metadata.Name,
				providerConfig.Spec.Template,
				params.CorrelationID,
			); err != nil {
				params.Logger.Warn("Failed to publish provider-to-template mapping",
					slog.String("provider_name", providerConfig.Metadata.Name),
					slog.String("template_handle", providerConfig.Spec.Template),
					slog.Any("error", err))
			}
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

		s.updatePolicyRouteConfig(storedCfg, params.Logger)
	}

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// DeployLLMProxyConfiguration parses, validates, transforms and persists the provider, then triggers xDS
func (s *LLMDeploymentService) DeployLLMProxyConfiguration(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	if !models.IsValidOrigin(params.Origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", params.Origin)
	}

	var proxyConfig api.LLMProxyConfiguration
	var apiConfig api.RestAPI

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

	// Transform to RestAPI configuration
	_, err := s.transformer.Transform(&proxyConfig, &apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform LLM proxy to API configuration: %w", err)
	}

	// Validate policies against loaded policy definitions
	// if s.policyValidator != nil {
	// 	policyErrors := s.policyValidator.ValidatePolicies(&apiConfig)
	// 	if len(policyErrors) > 0 {
	// 		errs := make([]string, 0, len(policyErrors))
	// 		for i, e := range policyErrors {
	// 			if params.Logger != nil {
	// 				params.Logger.Warn("Policy validation error", slog.String("field", e.Field), slog.String("message", e.Message))
	// 			}
	// 			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
	// 		}
	// 		return nil, fmt.Errorf("policy validation failed with %d error(s): %s", len(policyErrors), strings.Join(errs, "; "))
	// 	}
	// }

	// Generate API ID if not provided
	apiID := params.ID
	if apiID == "" {
		var err error
		apiID, err = GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API ID: %w", err)
		}
	}

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.LlmProxy),
		Handle:              proxyConfig.Metadata.Name,
		DisplayName:         proxyConfig.Spec.DisplayName,
		Version:             proxyConfig.Spec.Version,
		Configuration:       apiConfig,
		SourceConfiguration: proxyConfig,
		DesiredState:        models.StateDeployed,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          params.DeployedAt,
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
			slog.String("handle", storedCfg.Handle),
			slog.String("display_name", storedCfg.DisplayName),
			slog.String("version", storedCfg.Version),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("LLM proxy configuration created",
			slog.String("api_uuid", apiID),
			slog.String("handle", storedCfg.Handle),
			slog.String("display_name", storedCfg.DisplayName),
			slog.String("version", storedCfg.Version),
			slog.String("correlation_id", params.CorrelationID))
	}

	if s.isEventDriven() {
		action := "CREATE"
		if isUpdate {
			action = "UPDATE"
		}
		s.publishLLMProxyEvent(action, apiID, params.CorrelationID, params.Logger)
	} else {
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

		s.updatePolicyRouteConfig(storedCfg, params.Logger)
	}

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// LLMTemplateParams Template params for CRUD
type LLMTemplateParams struct {
	Spec          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
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

	id, err := GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate template ID: %w", err)
	}

	stored := &models.StoredLLMProviderTemplate{
		UUID:          id,
		Configuration: *tmpl,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Persist to DB if available
	if s.db != nil {
		if err := s.db.SaveLLMProviderTemplate(stored); err != nil {
			if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
				return nil, fmt.Errorf("template with handle '%s' already exists", tmpl.Metadata.Name)
			}
			return nil, fmt.Errorf("failed to save template to database: %w", err)
		}
	}

	if s.isEventDriven() && s.db != nil {
		s.publishLLMTemplateEvent("CREATE", stored.UUID, params.CorrelationID, params.Logger)
		return stored, nil
	}

	// Add to memory store (with rollback if it fails)
	if err := s.store.AddTemplate(stored); err != nil {
		// Rollback: Remove from DB if memory store fails
		if s.db != nil {
			if delErr := s.db.DeleteLLMProviderTemplate(stored.UUID); delErr != nil {
				if params.Logger != nil {
					params.Logger.Error("Failed to rollback template from database after memory store failure",
						slog.String("template_handle", tmpl.Metadata.Name),
						slog.Any("rollback_error", delErr))
				}
			}
		}
		return nil, fmt.Errorf("failed to add template to memory store: %w", err)
	}

	// Publish to policy engine via lazy resource xDS
	// Following API key pattern: xDS operations are critical
	if err := s.publishTemplateAsLazyResource(tmpl, ""); err != nil {
		// Rollback: Remove from memory store and DB if xDS publish fails
		if delErr := s.store.DeleteTemplate(stored.UUID); delErr != nil {
			if params.Logger != nil {
				params.Logger.Error("Failed to rollback template from memory store after xDS failure",
					slog.String("template_handle", tmpl.Metadata.Name),
					slog.Any("rollback_error", delErr))
			}
		}
		if s.db != nil {
			if delErr := s.db.DeleteLLMProviderTemplate(stored.UUID); delErr != nil {
				if params.Logger != nil {
					params.Logger.Error("Failed to rollback template from database after xDS failure",
						slog.String("template_handle", tmpl.Metadata.Name),
						slog.Any("rollback_error", delErr))
				}
			}
		}
		if params.Logger != nil {
			params.Logger.Error("Failed to publish template to policy engine via lazy resource xDS",
				slog.String("template_handle", tmpl.Metadata.Name),
				slog.Any("error", err))
		}
		return nil, fmt.Errorf("failed to publish template to policy engine: %w", err)
	}

	return stored, nil
}

// InitializeOOBTemplates persists OOB templates to database and memory store
func (s *LLMDeploymentService) InitializeOOBTemplates(templateDefinitions map[string]*api.LLMProviderTemplate) error {
	if len(templateDefinitions) == 0 {
		return nil
	}

	var allErrors []string
	processedHandles := make(map[string]bool) // Track which templates were processed from files

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
				UUID:          existing.UUID,
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

			// Publish updated template to policy engine via lazy resource xDS (ID = template ID)
			if err := s.publishTemplateAsLazyResource(tmpl, ""); err != nil {
				allErrors = append(allErrors,
					fmt.Sprintf("failed to publish template '%s' to policy engine via lazy resource xDS: %v", tmpl.Metadata.Name, err))
				continue
			}

			processedHandles[tmpl.Metadata.Name] = true
			continue
		}

		// ---------------------------
		// CREATE new template
		// ---------------------------

		id, err := GenerateUUID()
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("failed to generate ID for template '%s': %v", tmpl.Metadata.Name, err))
			continue
		}

		stored := &models.StoredLLMProviderTemplate{
			UUID:          id,
			Configuration: *tmpl,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		// persist to DB if available
		if s.db != nil {
			if err := s.db.SaveLLMProviderTemplate(stored); err != nil {
				if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
					continue
				}
				allErrors = append(allErrors, fmt.Sprintf("failed to save template '%s' to database: %v",
					tmpl.Metadata.Name, err))
				continue
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

		// Publish new template to policy engine via lazy resource xDS (ID = template ID)
		if err := s.publishTemplateAsLazyResource(tmpl, ""); err != nil {
			allErrors = append(allErrors,
				fmt.Sprintf("failed to publish template '%s' to policy engine via lazy resource xDS: %v", tmpl.Metadata.Name, err))
			continue
		}

		processedHandles[tmpl.Metadata.Name] = true
	}

	// Publish all templates from store that weren't processed from files (DB-only templates)
	allTemplates := s.store.GetAllTemplates()
	for _, stored := range allTemplates {
		handle := stored.GetHandle()
		if !processedHandles[handle] {
			// This template exists in store but wasn't in file definitions - publish it
			if err := s.publishTemplateAsLazyResource(&stored.Configuration, ""); err != nil {
				allErrors = append(allErrors,
					fmt.Sprintf("failed to publish DB-only template '%s' to policy engine via lazy resource xDS: %v", handle, err))
			}
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
	existing, err := s.GetLLMProviderTemplateByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("template with handle '%s' not found", handle)
	}

	tmpl, err := s.parseAndValidateLLMTemplate(params)
	if err != nil {
		return nil, err
	}

	// Validate that handle doesn't change unexpectedly
	// If the new template has a different handle, that's a different template
	oldHandle := existing.GetHandle()
	newHandle := tmpl.Metadata.Name
	if oldHandle != newHandle {
		return nil, fmt.Errorf("cannot change template handle from '%s' to '%s'. Use create/delete instead", oldHandle, newHandle)
	}

	updated := &models.StoredLLMProviderTemplate{
		UUID:          existing.UUID,
		Configuration: *tmpl,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     time.Now(),
	}

	// Update DB
	if s.db != nil {
		if err := s.db.UpdateLLMProviderTemplate(updated); err != nil {
			return nil, fmt.Errorf("failed to update template in database: %w", err)
		}
	}

	if s.isEventDriven() && s.db != nil {
		s.publishLLMTemplateEvent("UPDATE", updated.UUID, params.CorrelationID, params.Logger)
		return updated, nil
	}

	// Update memory store (with rollback if it fails)
	if err := s.store.UpdateTemplate(updated); err != nil {
		// Rollback: Revert DB update if memory store update fails
		if s.db != nil {
			if rollbackErr := s.db.UpdateLLMProviderTemplate(existing); rollbackErr != nil {
				if params.Logger != nil {
					params.Logger.Error("Failed to rollback template in database after memory store update failure",
						slog.String("template_handle", handle),
						slog.Any("rollback_error", rollbackErr))
				}
			}
		}
		return nil, fmt.Errorf("failed to update template in memory store: %w", err)
	}

	// Publish updated template to policy engine via lazy resource xDS
	if err := s.publishTemplateAsLazyResource(tmpl, ""); err != nil {
		// Rollback: Revert memory store and DB if xDS publish fails
		if rollbackErr := s.store.UpdateTemplate(existing); rollbackErr != nil {
			if params.Logger != nil {
				params.Logger.Error("Failed to rollback template in memory store after xDS failure",
					slog.String("template_handle", handle),
					slog.Any("rollback_error", rollbackErr))
			}
		}
		if s.db != nil {
			if rollbackErr := s.db.UpdateLLMProviderTemplate(existing); rollbackErr != nil {
				if params.Logger != nil {
					params.Logger.Error("Failed to rollback template in database after xDS failure",
						slog.String("template_handle", handle),
						slog.Any("rollback_error", rollbackErr))
				}
			}
		}
		if params.Logger != nil {
			params.Logger.Error("Failed to publish updated template to policy engine via lazy resource xDS",
				slog.String("template_handle", handle),
				slog.Any("error", err))
		}
		return nil, fmt.Errorf("failed to publish updated template to policy engine: %w", err)
	}

	return updated, nil
}

// DeleteLLMProviderTemplate deletes a template by name
func (s *LLMDeploymentService) DeleteLLMProviderTemplate(handle, correlationID string, logger *slog.Logger) (*models.StoredLLMProviderTemplate, error) {
	tmpl, err := s.GetLLMProviderTemplateByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("template with handle '%s' not found", handle)
	}

	if s.isEventDriven() && s.db != nil {
		if err := s.db.DeleteLLMProviderTemplate(tmpl.UUID); err != nil {
			return nil, fmt.Errorf("failed to delete template from database: %w", err)
		}
		s.publishLLMTemplateEvent("DELETE", tmpl.UUID, correlationID, logger)
		return tmpl, nil
	}

	// Remove from policy engine via lazy resource xDS (ID = handle)
	if s.lazyResourceManager != nil {
		if err := s.lazyResourceManager.RemoveResourceByIDAndType(handle, LazyResourceTypeLLMProviderTemplate, ""); err != nil {
			// Don't fail deletion if xDS publish fails; just log.
			slog.Warn("Failed to remove LLM provider template from policy engine via lazy resource xDS",
				slog.String("template_id", tmpl.UUID),
				slog.Any("error", err))
		}
	}

	if s.db != nil {
		if err := s.db.DeleteLLMProviderTemplate(tmpl.UUID); err != nil {
			// Rollback: Re-add to lazy resource store if database deletion fails.
			// publishTemplateAsLazyResource restores the template in lazy resources when
			// s.db.DeleteLLMProviderTemplate fails (only if lazy resource manager is available).
			if s.lazyResourceManager != nil {
				if rollbackErr := s.publishTemplateAsLazyResource(&tmpl.Configuration, ""); rollbackErr != nil {
					slog.Error("Failed to rollback lazy resource after database deletion failure",
						slog.String("template_handle", handle),
						slog.Any("rollback_error", rollbackErr))
				}
			}
			return nil, fmt.Errorf("failed to delete template from database: %w", err)
		}
	}
	if err := s.store.DeleteTemplate(tmpl.UUID); err != nil {
		// Rollback: Re-add to DB and lazy resource if memory deletion fails
		if s.db != nil {
			if rollbackErr := s.db.SaveLLMProviderTemplate(tmpl); rollbackErr != nil {
				slog.Error("Failed to rollback template to database after memory store deletion failure",
					slog.String("template_handle", handle),
					slog.Any("rollback_error", rollbackErr))
			}
		}
		if s.lazyResourceManager != nil {
			if rollbackErr := s.publishTemplateAsLazyResource(&tmpl.Configuration, ""); rollbackErr != nil {
				slog.Error("Failed to rollback lazy resource after memory store deletion failure",
					slog.String("template_handle", handle),
					slog.Any("rollback_error", rollbackErr))
			}
		}

		return nil, fmt.Errorf("failed to delete template from memory store: %w", err)
	}

	return tmpl, nil
}

// ListLLMProviderTemplates retrieves all LLM provider templates, optionally filtered by display name.
// If displayName is nil or empty, all templates are returned.
func (s *LLMDeploymentService) ListLLMProviderTemplates(displayName *string) []*models.StoredLLMProviderTemplate {
	templates := s.store.GetAllTemplates()
	if s.db != nil {
		if storedTemplates, err := s.db.GetAllLLMProviderTemplates(); err == nil {
			templates = storedTemplates
		}
	}

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
	if s.db != nil {
		templates, err := s.db.GetAllLLMProviderTemplates()
		if err == nil {
			for _, template := range templates {
				if template.GetHandle() == handle {
					return template, nil
				}
			}
			return nil, fmt.Errorf("%w: template with handle '%s' not found", storage.ErrNotFound, handle)
		}
		if !storage.IsDatabaseUnavailableError(err) {
			return nil, err
		}
	}

	return s.store.GetTemplateByHandle(handle)
}

func (s *LLMDeploymentService) publishTemplateAsLazyResource(tmpl *api.LLMProviderTemplate, correlationID string) error {
	if s.lazyResourceManager == nil {
		return nil
	}
	if tmpl == nil {
		return fmt.Errorf("template is nil")
	}
	if tmpl.Metadata.Name == "" {
		return fmt.Errorf("template handle (metadata.name) is empty")
	}

	// Convert typed template to map[string]interface{} for the generic lazy resource payload.
	b, err := json.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("failed to marshal template as JSON: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("failed to unmarshal template JSON into map: %w", err)
	}

	return s.lazyResourceManager.StoreResource(&storage.LazyResource{
		ID:           tmpl.Metadata.Name,
		ResourceType: LazyResourceTypeLLMProviderTemplate,
		Resource:     m,
	}, correlationID)
}

// publishProviderTemplateMappingAsLazyResource publishes the provider-to-template mapping
// as a lazy resource for the policy engine to consume
func (s *LLMDeploymentService) publishProviderTemplateMappingAsLazyResource(providerName, templateHandle, correlationID string) error {
	if s.lazyResourceManager == nil {
		return nil
	}
	if providerName == "" {
		return fmt.Errorf("provider name is empty")
	}
	if templateHandle == "" {
		return fmt.Errorf("template handle is empty")
	}

	// Create a mapping resource with provider name as ID and template handle as resource data
	mappingResource := map[string]interface{}{
		"provider_name":   providerName,
		"template_handle": templateHandle,
	}

	return s.lazyResourceManager.StoreResource(&storage.LazyResource{
		ID:           providerName,
		ResourceType: LazyResourceTypeProviderTemplateMapping,
		Resource:     mappingResource,
	}, correlationID)
}

// removeProviderTemplateMappingLazyResource removes the provider-to-template mapping lazy resource
func (s *LLMDeploymentService) removeProviderTemplateMappingLazyResource(providerName, correlationID string) error {
	if s.lazyResourceManager == nil {
		return nil
	}
	if providerName == "" {
		return fmt.Errorf("provider name is empty")
	}

	return s.lazyResourceManager.RemoveResourceByIDAndType(providerName, LazyResourceTypeProviderTemplateMapping, correlationID)
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
	// Prefer database rows when available because EventHub-based flows can leave
	// the local store briefly behind the canonical state right after a write.
	if s.db != nil {
		if storedConfigs, err := s.db.GetAllConfigsByKind(string(api.LlmProvider)); err == nil {
			configs = storedConfigs
		}
	}

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
	var name, version, cnt, vhost, status *string

	switch p := params.(type) {
	case api.ListLLMProvidersParams:
		name, version, cnt, status, vhost = p.DisplayName, p.Version, p.Context, (*string)(p.Status), p.Vhost
	case api.ListLLMProxiesParams:
		name, version, cnt, status, vhost = p.DisplayName, p.Version, p.Context, (*string)(p.Status), p.Vhost
	default:
		return false
	}

	var displayName string
	var configVersion string
	var configContext string
	var configVHost *string

	switch sc := config.SourceConfiguration.(type) {
	case api.LLMProviderConfiguration:
		displayName = sc.Spec.DisplayName
		configVersion = sc.Spec.Version
		if sc.Spec.Context != nil {
			configContext = *sc.Spec.Context
		}
		configVHost = sc.Spec.Vhost
	case api.LLMProxyConfiguration:
		displayName = sc.Spec.DisplayName
		configVersion = sc.Spec.Version
		if sc.Spec.Context != nil {
			configContext = *sc.Spec.Context
		}
	default:
		restCfg, ok := config.Configuration.(api.RestAPI)
		if !ok {
			return false
		}
		displayName = restCfg.Spec.DisplayName
		configVersion = restCfg.Spec.Version
		configContext = restCfg.Spec.Context
		if restCfg.Spec.Vhosts != nil {
			configVHost = &restCfg.Spec.Vhosts.Main
		}
	}

	// Check DisplayName filter
	if name != nil && displayName != *name {
		return false
	}

	// Check Version filter
	if version != nil && configVersion != *version {
		return false
	}

	// Check Context filter
	if cnt != nil && configContext != *cnt {
		return false
	}

	// Check Status filter
	if status != nil && string(config.DesiredState) != string(*status) {
		return false
	}

	// Check Vhost filter
	if vhost != nil {
		if configVHost == nil || *configVHost != *vhost {
			return false
		}
	}

	return true
}

// UpdateLLMProvider updates an existing provider identified by name+version using DeployLLMProviderConfiguration
func (s *LLMDeploymentService) UpdateLLMProvider(handle string, params LLMDeploymentParams) (*models.StoredConfig, error) {
	existing, err := s.GetLLMProviderByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM provider: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("LLM provider configuration with handle '%s' not found", handle)
	}
	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.UUID
	res, err := s.DeployLLMProviderConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// DeleteLLMProvider deletes by name+version using store/db and updates snapshot
func (s *LLMDeploymentService) DeleteLLMProvider(handle, correlationID string,
	logger *slog.Logger) (*models.StoredConfig, error) {
	cfg, err := s.GetLLMProviderByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM provider: %w", err)
	}
	if cfg == nil {
		return cfg, fmt.Errorf("LLM provider configuration with handle '%s' not found", handle)
	}
	// Remove the canonical row first so every replica observes the same delete
	// via the follow-up event, instead of each writer mutating local state inline.
	if s.db != nil {
		if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
			return cfg, fmt.Errorf("failed to delete LLM provider API keys from database: %w", err)
		}
		if err := s.db.DeleteConfig(cfg.UUID); err != nil {
			return cfg, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}
	if s.isEventDriven() {
		s.publishLLMProviderEvent("DELETE", cfg.UUID, correlationID, logger)
		return cfg, nil
	}
	if err := s.store.RemoveAPIKeysByAPI(cfg.UUID); err != nil && !storage.IsNotFoundError(err) {
		return cfg, fmt.Errorf("failed to delete LLM provider API keys from memory store: %w", err)
	}
	if err := s.store.Delete(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete configuration from memory store: %w", err)
	}

	// Remove provider-to-template mapping lazy resource
	if err := s.removeProviderTemplateMappingLazyResource(handle, correlationID); err != nil {
		logger.Warn("Failed to remove provider-to-template mapping",
			slog.String("provider_name", handle),
			slog.Any("error", err))
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
		}
	}()

	if s.policyManager != nil {
		key := storage.Key(cfg.Kind, cfg.Handle)
		if err := s.policyManager.RemoveRuntimeConfig(key); err != nil {
			logger.Warn("Failed to remove policy route config for LLM provider", slog.Any("error", err))
		}
	}

	return cfg, nil
}

func (s *LLMDeploymentService) GetLLMProviderByHandle(handle string) (*models.StoredConfig, error) {
	// In EventHub mode the database is the source of truth.
	if s.db != nil {
		cfg, err := s.db.GetConfigByKindAndHandle(string(api.LlmProvider), handle)
		if err == nil {
			_ = s.hydrateStoredLLMConfig(cfg)
			return cfg, nil
		}
		return nil, err
	}

	cfg, err := s.store.GetByKindAndHandle(string(api.LlmProvider), handle)
	if err != nil {
		return nil, err
	}
	_ = s.hydrateStoredLLMConfig(cfg)
	return cfg, nil
}

// ListLLMProxies returns all stored LLM proxy configurations
func (s *LLMDeploymentService) ListLLMProxies(params api.ListLLMProxiesParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(string(api.LlmProxy))
	if s.db != nil {
		if storedConfigs, err := s.db.GetAllConfigsByKind(string(api.LlmProxy)); err == nil {
			configs = storedConfigs
		}
	}

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
	existing, err := s.GetLLMProxyByHandle(id)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM proxy: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("LLM proxy configuration with handle '%s' not found", id)
	}
	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.UUID
	res, err := s.DeployLLMProxyConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

func (s *LLMDeploymentService) GetLLMProxyByHandle(handle string) (*models.StoredConfig, error) {
	if s.db != nil {
		cfg, err := s.db.GetConfigByKindAndHandle(string(api.LlmProxy), handle)
		if err == nil {
			_ = s.hydrateStoredLLMConfig(cfg)
			return cfg, nil
		}
		return nil, err
	}

	cfg, err := s.store.GetByKindAndHandle(string(api.LlmProxy), handle)
	if err != nil {
		return nil, err
	}
	_ = s.hydrateStoredLLMConfig(cfg)
	return cfg, nil
}

// DeleteLLMProxy deletes by name+version using store/db and updates snapshot
func (s *LLMDeploymentService) DeleteLLMProxy(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	cfg, err := s.GetLLMProxyByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM proxy: %w", err)
	}
	if cfg == nil {
		return cfg, fmt.Errorf("LLM proxy configuration with handle '%s' not found", handle)
	}
	if s.db != nil {
		if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
			return cfg, fmt.Errorf("failed to delete LLM proxy API keys from database: %w", err)
		}
		if err := s.db.DeleteConfig(cfg.UUID); err != nil {
			return cfg, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}
	if s.isEventDriven() {
		s.publishLLMProxyEvent("DELETE", cfg.UUID, correlationID, logger)
		return cfg, nil
	}
	if err := s.store.RemoveAPIKeysByAPI(cfg.UUID); err != nil && !storage.IsNotFoundError(err) {
		return cfg, fmt.Errorf("failed to delete LLM proxy API keys from memory store: %w", err)
	}
	if err := s.store.Delete(cfg.UUID); err != nil {
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

	if s.policyManager != nil {
		key := storage.Key(cfg.Kind, cfg.Handle)
		if err := s.policyManager.RemoveRuntimeConfig(key); err != nil {
			logger.Warn("Failed to remove policy route config for LLM proxy", slog.Any("error", err))
		}
	}

	return cfg, nil
}
