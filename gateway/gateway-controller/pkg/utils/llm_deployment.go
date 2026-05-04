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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const (
	LazyResourceTypeLLMProviderTemplate     = "LlmProviderTemplate"
	LazyResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"
)

var (
	// ErrLLMTemplateNotFound is returned when an LLM provider template is not found.
	ErrLLMTemplateNotFound = errors.New("llm provider template not found")

	// ErrLLMTemplateValidation is returned when an LLM provider template fails parsing or validation.
	ErrLLMTemplateValidation = errors.New("template configuration invalid")

	// ErrLLMProxyValidation is returned when an LLM proxy configuration fails parsing or validation.
	ErrLLMProxyValidation = errors.New("proxy configuration invalid")
)

// LLMDeploymentParams carries input to deploy/update a provider
type LLMDeploymentParams struct {
	Data          []byte              // Raw configuration data (YAML/JSON)
	ContentType   string              // Content type for parsing
	ID            string              // Optional ID; if empty, generated
	DeploymentID  string              // Platform deployment ID (empty for gateway-api origin)
	Origin        models.Origin       // Origin of the deployment: "control_plane" or "gateway_api"
	DeployedAt    *time.Time          // Deployment timestamp from platform event (nil for gateway-api origin)
	CorrelationID string              // Correlation ID for tracking
	Logger        *slog.Logger        // Logger
	IsUpdate      bool                // True when the caller has resolved this as an update (e.g. from DB lookup)
	DesiredState  models.DesiredState // Desired deployment state; empty defaults to StateDeployed
}

// LLMDeploymentService encapsulates validate+transform+persist+deploy for LLM Providers
type LLMDeploymentService struct {
	store               *storage.ConfigStore
	db                  storage.Storage
	snapshotManager     *xds.SnapshotManager
	lazyResourceManager *lazyresourcexds.LazyResourceStateManager
	templateDefinitions map[string]*api.LLMProviderTemplate
	deploymentService   *APIDeploymentService
	parser              *config.Parser
	validator           *config.LLMValidator
	policyValidator     *config.PolicyValidator
	transformer         Transformer
	routerConfig        *config.RouterConfig
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
) *LLMDeploymentService {
	if db == nil {
		panic("LLMDeploymentService requires non-nil storage")
	}
	if deploymentService == nil {
		panic("LLMDeploymentService requires APIDeploymentService")
	}
	requireReplicaSyncWiring("LLMDeploymentService", deploymentService.eventHub, deploymentService.gatewayID)

	service := &LLMDeploymentService{
		store:               store,
		db:                  db,
		snapshotManager:     snapshotManager,
		lazyResourceManager: lazyResourceManager,
		templateDefinitions: templateDefinitions,
		deploymentService:   deploymentService,
		parser:              config.NewParser(),
		validator:           config.NewLLMValidator(),
		policyValidator:     policyValidator,
		transformer:         NewLLMProviderTransformer(store, db, routerConfig, policyVersionResolver),
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

func (s *LLMDeploymentService) publishLLMProviderEvent(action, entityID, correlationID string, logger *slog.Logger) {
	s.deploymentService.publishEvent(eventhub.EventTypeLLMProvider, action, entityID, correlationID, logger)
}

func (s *LLMDeploymentService) publishLLMProxyEvent(action, entityID, correlationID string, logger *slog.Logger) {
	s.deploymentService.publishEvent(eventhub.EventTypeLLMProxy, action, entityID, correlationID, logger)
}

func (s *LLMDeploymentService) validateTemplateHandleConflict(handle string) error {
	existing, err := s.db.GetLLMProviderTemplateByHandle(handle)
	if err == nil && existing != nil {
		return fmt.Errorf("%w: template with handle '%s' already exists", storage.ErrConflict, handle)
	}
	if err != nil && !storage.IsNotFoundError(err) {
		return err
	}
	return nil
}

func (s *LLMDeploymentService) publishLLMTemplateEvent(action, entityID, correlationID string, logger *slog.Logger) {
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
		// Render template expressions (secrets/env/defaults) before transforming so the
		// transformer sees resolved values, not raw template syntax. cfg.SourceConfiguration
		// is left intact — only cfg.Configuration (the derived RestAPI) is updated.
		renderHolder := &models.StoredConfig{Configuration: src}
		if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, nil); err != nil {
			return fmt.Errorf("failed to render stored LLM provider %s: %w", cfg.UUID, err)
		}
		rendered, ok := renderHolder.Configuration.(api.LLMProviderConfiguration)
		if !ok {
			return fmt.Errorf("unexpected configuration type %T after rendering stored LLM provider %s", renderHolder.Configuration, cfg.UUID)
		}
		var restAPI api.RestAPI
		if _, err := s.transformer.Transform(&rendered, &restAPI); err != nil {
			return fmt.Errorf("failed to transform stored LLM provider %s: %w", cfg.UUID, err)
		}
		cfg.Configuration = restAPI
	case api.LLMProxyConfiguration:
		// Render template expressions before transforming for the same reason as above.
		renderHolder := &models.StoredConfig{Configuration: src}
		if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, nil); err != nil {
			return fmt.Errorf("failed to render stored LLM proxy %s: %w", cfg.UUID, err)
		}
		rendered, ok := renderHolder.Configuration.(api.LLMProxyConfiguration)
		if !ok {
			return fmt.Errorf("unexpected configuration type %T after rendering stored LLM proxy %s", renderHolder.Configuration, cfg.UUID)
		}
		var restAPI api.RestAPI
		if _, err := s.transformer.Transform(&rendered, &restAPI); err != nil {
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

	for _, cfg := range s.store.GetAllByKind(string(api.LLMProviderConfigurationKindLlmProvider)) {
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

	for _, cfg := range s.store.GetAllByKind(string(api.LLMProxyConfigurationKindLlmProxy)) {
		if err := s.hydrateStoredLLMConfig(cfg); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil
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

	// Render template expressions ({{ secret "..." }}, {{ env "..." }}, {{ default ... }}, etc.)
	// BEFORE validation so the validator sees resolved values, not raw template syntax.
	// We render in a temp StoredConfig then cast back. The unrendered providerConfig is
	// what gets persisted as SourceConfiguration; each replica re-renders on consumption.
	renderHolder := &models.StoredConfig{Configuration: providerConfig}
	if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, err
	}
	renderedProvider, ok := renderHolder.Configuration.(api.LLMProviderConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected configuration type %T after rendering LLM provider", renderHolder.Configuration)
	}

	// Validate against rendered values
	validationErrors := s.validator.Validate(&renderedProvider)
	if len(validationErrors) > 0 {
		errs := make([]string, 0, len(validationErrors))
		params.Logger.Warn("LLM provider validation failed",
			slog.String("handle", renderedProvider.Metadata.Name),
			slog.Int("num_errors", len(validationErrors)))
		for i, e := range validationErrors {
			params.Logger.Warn("Validation error", slog.String("field", e.Field), slog.String("message", e.Message))
			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}
		return nil, fmt.Errorf("provider validation failed with %d error(s): %s", len(validationErrors), strings.Join(errs, "; "))
	}

	// Transform rendered config to RestAPI configuration. Configuration on the resulting
	// storedCfg is built from rendered values; SourceConfiguration retains the unrendered
	// providerConfig so replicas can re-render on consumption.
	_, err := s.transformer.Transform(&renderedProvider, &apiConfig)
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
	deployedAt := params.DeployedAt
	if deployedAt == nil {
		truncated := now.Truncate(time.Millisecond)
		deployedAt = &truncated
	}
	desiredState := params.DesiredState
	if desiredState == "" {
		desiredState = models.StateDeployed
	}

	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              providerConfig.Metadata.Name,
		DisplayName:         providerConfig.Spec.DisplayName,
		Version:             providerConfig.Spec.Version,
		Configuration:       apiConfig,
		SourceConfiguration: providerConfig,
		DesiredState:        desiredState,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          deployedAt,
	}

	isUpdate := params.IsUpdate
	if !isUpdate && params.ID != "" {
		if existing, err := s.db.GetConfig(params.ID); err == nil && existing != nil {
			isUpdate = true
		}
	}

	if err := s.deploymentService.validateArtifactConflicts(
		string(api.LLMProviderConfigurationKindLlmProvider),
		storedCfg.UUID,
		storedCfg.DisplayName,
		storedCfg.Version,
		storedCfg.Handle,
	); err != nil {
		return nil, err
	}

	// Render template expressions to catch invalid function names or secret references early.
	// The rendered Configuration is not persisted — SourceConfiguration (unrendered) is what
	// the DB stores, and each replica's EventListener re-derives Configuration from it on consumption.
	if err := templateengine.RenderSpec(storedCfg, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, err
	}

	// Save or update using timestamp-guarded upsert.
	// affected=false means a newer version already exists (stale event — no-op).
	affected, err := s.deploymentService.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		params.Logger.Error("Failed to save or update LLM provider configuration",
			slog.String("handle", storedCfg.Handle),
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to save or update LLM provider configuration")
	}

	if !affected {
		// Stale event — DB was not modified. Return success but skip event publishing, lazy-resource, and xDS update.
		return &APIDeploymentResult{
			StoredConfig: storedCfg,
			IsUpdate:     isUpdate,
			IsStale:      true,
		}, nil
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
	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	s.publishLLMProviderEvent(action, apiID, params.CorrelationID, params.Logger)

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate, IsStale: false}, nil
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
		return nil, fmt.Errorf("%w: failed to parse proxy configuration: %v", ErrLLMProxyValidation, err)
	}

	// Render template expressions BEFORE validation so the validator sees resolved
	// values, not raw template syntax. The unrendered proxyConfig is persisted as
	// SourceConfiguration; each replica re-renders on consumption.
	renderHolder := &models.StoredConfig{Configuration: proxyConfig}
	if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, err
	}
	renderedProxy, ok := renderHolder.Configuration.(api.LLMProxyConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected configuration type %T after rendering LLM proxy", renderHolder.Configuration)
	}

	// Validate against rendered values
	validationErrors := s.validator.Validate(&renderedProxy)
	if len(validationErrors) > 0 {
		errs := make([]string, 0, len(validationErrors))
		params.Logger.Warn("LLM proxy validation failed",
			slog.String("handle", renderedProxy.Metadata.Name),
			slog.Int("num_errors", len(validationErrors)))
		for i, e := range validationErrors {
			params.Logger.Warn("Validation error", slog.String("field", e.Field), slog.String("message", e.Message))
			errs = append(errs, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}
		return nil, fmt.Errorf("%w: %d error(s): %s", ErrLLMProxyValidation, len(validationErrors), strings.Join(errs, "; "))
	}

	// Transform rendered config to RestAPI. Configuration is built from rendered
	// values; SourceConfiguration retains the unrendered proxyConfig.
	_, err := s.transformer.Transform(&renderedProxy, &apiConfig)
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
	deployedAt := params.DeployedAt
	if deployedAt == nil {
		truncated := now.Truncate(time.Millisecond)
		deployedAt = &truncated
	}
	proxyDesiredState := params.DesiredState
	if proxyDesiredState == "" {
		proxyDesiredState = models.StateDeployed
	}

	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:              proxyConfig.Metadata.Name,
		DisplayName:         proxyConfig.Spec.DisplayName,
		Version:             proxyConfig.Spec.Version,
		Configuration:       apiConfig,
		SourceConfiguration: proxyConfig,
		DesiredState:        proxyDesiredState,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          deployedAt,
	}

	isUpdate := params.IsUpdate
	if !isUpdate && params.ID != "" {
		if existing, err := s.db.GetConfig(params.ID); err == nil && existing != nil {
			isUpdate = true
		}
	}

	if err := s.deploymentService.validateArtifactConflicts(
		string(api.LLMProxyConfigurationKindLlmProxy),
		storedCfg.UUID,
		storedCfg.DisplayName,
		storedCfg.Version,
		storedCfg.Handle,
	); err != nil {
		return nil, err
	}

	// Render template expressions to catch invalid function names or secret references early.
	// The rendered Configuration is not persisted — SourceConfiguration (unrendered) is what
	// the DB stores, and each replica's EventListener re-derives Configuration from it on consumption.
	if err := templateengine.RenderSpec(storedCfg, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, err
	}

	// Save or update using timestamp-guarded upsert.
	// Policy resolution happens in the EventListener after all replicas consume the
	// published event, immediately before UpsertAPIConfig is called.
	affected, err := s.deploymentService.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to save or update LLM proxy configuration: %w", err)
	}

	if !affected {
		// Stale event — DB was not modified. Return success but skip event publishing and xDS update.
		return &APIDeploymentResult{
			StoredConfig: storedCfg,
			IsUpdate:     isUpdate,
			IsStale:      true,
		}, nil
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

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	s.publishLLMProxyEvent(action, apiID, params.CorrelationID, params.Logger)

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate, IsStale: false}, nil
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
		return nil, fmt.Errorf("%w: failed to parse template configuration: %v", ErrLLMTemplateValidation, err)
	}

	// Render template expressions into a separate copy for validation only; tmpl stays unrendered for persistence.
	renderHolder := &models.StoredConfig{Configuration: tmpl}
	if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLLMTemplateValidation, err)
	}
	renderedTmpl, ok := renderHolder.Configuration.(api.LLMProviderTemplate)
	if !ok {
		return nil, fmt.Errorf("%w: template '%s' RenderSpec returned unexpected configuration type %T", ErrLLMTemplateValidation, tmpl.Metadata.Name, renderHolder.Configuration)
	}

	validationErrors := s.validator.Validate(&renderedTmpl)
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
		return nil, fmt.Errorf("%w: %d error(s): %s", ErrLLMTemplateValidation, len(validationErrors), strings.Join(errs, "; "))
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

	if err := s.validateTemplateHandleConflict(tmpl.Metadata.Name); err != nil {
		return nil, err
	}

	stored := &models.StoredLLMProviderTemplate{
		UUID:          id,
		Configuration: *tmpl,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Persist to DB
	if err := s.db.SaveLLMProviderTemplate(stored); err != nil {
		if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("%w: template with handle '%s' already exists", storage.ErrConflict, tmpl.Metadata.Name)
		}
		return nil, fmt.Errorf("failed to save template to database: %w", err)
	}

	s.publishLLMTemplateEvent("CREATE", stored.UUID, params.CorrelationID, params.Logger)
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
		// Render template expressions into a separate copy so the validator sees resolved values.
		// The original tmpl is kept intact so that unresolved template expressions are persisted.
		renderHolder := &models.StoredConfig{Configuration: *tmpl}
		if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, nil); err != nil {
			allErrors = append(allErrors, fmt.Sprintf(
				"template '%s' template rendering failed: %v",
				tmpl.Metadata.Name, err))
			continue
		}
		rendered, ok := renderHolder.Configuration.(api.LLMProviderTemplate)
		if !ok {
			allErrors = append(allErrors, fmt.Sprintf(
				"template '%s' rendered configuration type assertion failed",
				tmpl.Metadata.Name))
			continue
		}

		// Validate the rendered (resolved) copy; tmpl is not mutated.
		validationErrors := s.validator.Validate(&rendered)
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
			if err := s.db.UpdateLLMProviderTemplate(updated); err != nil {
				allErrors = append(allErrors,
					fmt.Sprintf("failed to update template '%s' in database: %v", tmpl.Metadata.Name, err))
				continue
			}

			// Update memory store
			if err := s.store.UpdateTemplate(updated); err != nil {
				allErrors = append(allErrors,
					fmt.Sprintf("failed to update template '%s' in memory store: %v", tmpl.Metadata.Name, err))
				continue
			}

			// Publish updated template to policy engine via lazy resource xDS (ID = template ID).
			// Use the rendered (resolved) copy so the policy engine receives actual values.
			if err := s.publishTemplateAsLazyResource(&rendered, ""); err != nil {
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

		// persist to DB
		if err := s.db.SaveLLMProviderTemplate(stored); err != nil {
			if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
				continue
			}
			allErrors = append(allErrors, fmt.Sprintf("failed to save template '%s' to database: %v",
				tmpl.Metadata.Name, err))
			continue
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

		// Publish new template to policy engine via lazy resource xDS (ID = template ID).
		// Use the rendered (resolved) copy so the policy engine receives actual values.
		if err := s.publishTemplateAsLazyResource(&rendered, ""); err != nil {
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
			// Render the stored (unresolved) configuration before publishing so the
			// policy engine receives actual resolved values, not raw template expressions.
			renderHolder := &models.StoredConfig{Configuration: stored.Configuration}
			if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, nil); err != nil {
				allErrors = append(allErrors, fmt.Sprintf(
					"DB-only template '%s' template rendering failed: %v", handle, err))
				continue
			}
			rendered, ok := renderHolder.Configuration.(api.LLMProviderTemplate)
			if !ok {
				allErrors = append(allErrors, fmt.Sprintf(
					"DB-only template '%s' rendered configuration type assertion failed (got %T)",
					handle, renderHolder.Configuration))
				continue
			}
			if err := s.publishTemplateAsLazyResource(&rendered, ""); err != nil {
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
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: handle=%s", ErrLLMTemplateNotFound, handle)
		}
		return nil, fmt.Errorf("failed to get LLM provider template by handle '%s': %w", handle, err)
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
		return nil, fmt.Errorf("%w: cannot change template handle from '%s' to '%s'; use create/delete instead", ErrLLMTemplateValidation, oldHandle, newHandle)
	}

	updated := &models.StoredLLMProviderTemplate{
		UUID:          existing.UUID,
		Configuration: *tmpl,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     time.Now(),
	}

	// Update DB
	if err := s.db.UpdateLLMProviderTemplate(updated); err != nil {
		return nil, fmt.Errorf("failed to update template in database: %w", err)
	}

	s.publishLLMTemplateEvent("UPDATE", updated.UUID, params.CorrelationID, params.Logger)
	return updated, nil
}

// DeleteLLMProviderTemplate deletes a template by name
func (s *LLMDeploymentService) DeleteLLMProviderTemplate(handle, correlationID string, logger *slog.Logger) (*models.StoredLLMProviderTemplate, error) {
	tmpl, err := s.GetLLMProviderTemplateByHandle(handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: handle=%s", ErrLLMTemplateNotFound, handle)
		}
		return nil, fmt.Errorf("failed to get LLM provider template by handle '%s': %w", handle, err)
	}

	if err := s.db.DeleteLLMProviderTemplate(tmpl.UUID); err != nil {
		return nil, fmt.Errorf("failed to delete template from database: %w", err)
	}
	s.publishLLMTemplateEvent("DELETE", tmpl.UUID, correlationID, logger)
	return tmpl, nil
}

// ListLLMProviderTemplates retrieves all LLM provider templates, optionally filtered by display name.
// If displayName is nil or empty, all templates are returned.
func (s *LLMDeploymentService) ListLLMProviderTemplates(displayName *string) []*models.StoredLLMProviderTemplate {
	templates := s.store.GetAllTemplates()
	if storedTemplates, err := s.db.GetAllLLMProviderTemplates(); err == nil {
		templates = storedTemplates
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
	template, err := s.db.GetLLMProviderTemplateByHandle(handle)
	if err == nil {
		return template, nil
	}
	if !storage.IsDatabaseUnavailableError(err) {
		return nil, err
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
func (s *LLMDeploymentService) CreateLLMProvider(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	return s.DeployLLMProviderConfiguration(params)
}

// ListLLMProviders returns all stored LLM provider configurations with optional filtering
func (s *LLMDeploymentService) ListLLMProviders(params api.ListLLMProvidersParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(string(api.LLMProviderConfigurationKindLlmProvider))
	// Prefer database rows because EventHub-based flows can leave
	// the local store briefly behind the canonical state right after a write.
	if storedConfigs, err := s.db.GetAllConfigsByKind(string(api.LLMProviderConfigurationKindLlmProvider)); err == nil {
		configs = storedConfigs
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

// UpdateLLMProvider updates an existing provider identified by name+version using DeployLLMProviderConfiguration.
// The full config is always applied. If deploymentState is "undeployed", the provider is also
// removed from router traffic while preserving the updated configuration.
func (s *LLMDeploymentService) UpdateLLMProvider(handle string, params LLMDeploymentParams) (*APIDeploymentResult, error) {
	existing, err := s.GetLLMProviderByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM provider: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("LLM provider configuration with handle '%s' not found", handle)
	}

	// Extract deploymentState from the incoming config
	if isUndeploy, err := s.isLLMProviderUndeployRequest(params); err != nil {
		return nil, err
	} else if isUndeploy {
		params.DesiredState = models.StateUndeployed
		// DeployedAt left nil — deploy method will set time.Now() to mark undeployment time
	} else {
		// Advance DeployedAt so the timestamp-guarded upsert accepts the update.
		now := time.Now().Truncate(time.Millisecond)
		params.DeployedAt = &now
	}

	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.UUID
	params.IsUpdate = true
	return s.DeployLLMProviderConfiguration(params)
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
	if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete LLM provider API keys from database: %w", err)
	}
	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete configuration from database: %w", err)
	}
	s.publishLLMProviderEvent("DELETE", cfg.UUID, correlationID, logger)
	return cfg, nil
}

func (s *LLMDeploymentService) GetLLMProviderByHandle(handle string) (*models.StoredConfig, error) {
	// The database is the source of truth.
	cfg, err := s.db.GetConfigByKindAndHandle(string(api.LLMProviderConfigurationKindLlmProvider), handle)
	if err == nil {
		_ = s.hydrateStoredLLMConfig(cfg)
		return cfg, nil
	}
	return nil, err
}

// ListLLMProxies returns all stored LLM proxy configurations
func (s *LLMDeploymentService) ListLLMProxies(params api.ListLLMProxiesParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(string(api.LLMProxyConfigurationKindLlmProxy))
	if storedConfigs, err := s.db.GetAllConfigsByKind(string(api.LLMProxyConfigurationKindLlmProxy)); err == nil {
		configs = storedConfigs
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
func (s *LLMDeploymentService) CreateLLMProxy(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	return s.DeployLLMProxyConfiguration(params)
}

// UpdateLLMProxy updates an existing proxy identified by name+version using DeployLLMProxyConfiguration.
// The full config is always applied. If deploymentState is "undeployed", the proxy is also
// removed from router traffic while preserving the updated configuration.
func (s *LLMDeploymentService) UpdateLLMProxy(id string, params LLMDeploymentParams) (*APIDeploymentResult, error) {
	existing, err := s.GetLLMProxyByHandle(id)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM proxy: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("%w: LLM proxy configuration with handle '%s' not found", storage.ErrNotFound, id)
	}

	// Extract deploymentState from the incoming config
	if isUndeploy, err := s.isLLMProxyUndeployRequest(params); err != nil {
		return nil, err
	} else if isUndeploy {
		params.DesiredState = models.StateUndeployed
		// DeployedAt left nil — deploy method will set time.Now() to mark undeployment time
	} else {
		// Advance DeployedAt so the timestamp-guarded upsert accepts the update.
		now := time.Now().Truncate(time.Millisecond)
		params.DeployedAt = &now
	}

	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.UUID
	params.IsUpdate = true
	return s.DeployLLMProxyConfiguration(params)
}

// isLLMProviderUndeployRequest parses just enough of the provider config to check if deploymentState is "undeployed".
func (s *LLMDeploymentService) isLLMProviderUndeployRequest(params LLMDeploymentParams) (bool, error) {
	var providerConfig api.LLMProviderConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &providerConfig); err != nil {
		return false, fmt.Errorf("failed to parse provider configuration: %w", err)
	}
	return providerConfig.Spec.DeploymentState != nil &&
		*providerConfig.Spec.DeploymentState == api.LLMProviderConfigDataDeploymentStateUndeployed, nil
}

// isLLMProxyUndeployRequest parses just enough of the proxy config to check if deploymentState is "undeployed".
func (s *LLMDeploymentService) isLLMProxyUndeployRequest(params LLMDeploymentParams) (bool, error) {
	var proxyConfig api.LLMProxyConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &proxyConfig); err != nil {
		return false, fmt.Errorf("failed to parse proxy configuration: %w", err)
	}
	return proxyConfig.Spec.DeploymentState != nil &&
		*proxyConfig.Spec.DeploymentState == api.LLMProxyConfigDataDeploymentStateUndeployed, nil
}

func (s *LLMDeploymentService) GetLLMProxyByHandle(handle string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(string(api.LLMProxyConfigurationKindLlmProxy), handle)
	if err == nil {
		_ = s.hydrateStoredLLMConfig(cfg)
		return cfg, nil
	}
	return nil, err
}

// DeleteLLMProxy deletes by name+version using store/db and updates snapshot
func (s *LLMDeploymentService) DeleteLLMProxy(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	cfg, err := s.GetLLMProxyByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to look up LLM proxy: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("%w: LLM proxy configuration with handle '%s' not found", storage.ErrNotFound, handle)
	}
	if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete LLM proxy API keys from database: %w", err)
	}
	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete configuration from database: %w", err)
	}
	s.publishLLMProxyEvent("DELETE", cfg.UUID, correlationID, logger)
	return cfg, nil
}
