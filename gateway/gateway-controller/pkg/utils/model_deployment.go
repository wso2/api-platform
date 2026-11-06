/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
)

var (
	// ErrModelValidation is returned when a Model fails parsing or validation.
	ErrModelValidation = errors.New("model configuration invalid")
)

// SetModelsConfig sets the [controller.models] configuration the Model and model
// endpoint paths check against. Unset, no default endpoint is provisioned.
func (s *LLMDeploymentService) SetModelsConfig(cfg config.ModelsConfig) {
	s.modelsConfig = cfg
}

func (s *LLMDeploymentService) publishModelEvent(action, entityID, correlationID string, logger *slog.Logger) {
	s.deploymentService.publishEvent(eventhub.EventTypeModel, action, entityID, correlationID, logger)
}

func (s *LLMDeploymentService) modelLookup() *ModelLookup {
	var definitions map[string]models.PolicyDefinition
	if s.policyValidator != nil {
		definitions = s.policyValidator.Definitions()
	}
	return NewDBModelLookup(s.db, definitions, s.deploymentService.secretResolver)
}

// ModelResolveOptions are the options a Model is resolved with: strictly on a
// save, leniently in the compiler (see modelgateway.Options.Strict).
func ModelResolveOptions(routerConfig *config.RouterConfig, strict bool) modelgateway.Options {
	opts := modelgateway.Options{Strict: strict}
	if routerConfig != nil {
		opts.DefaultVhost = routerConfig.VHosts.Main.Default
	}
	return opts
}

// DeployModelConfiguration parses, validates, resolves and persists a Model, then
// publishes the event every replica recompiles the Model's endpoints from.
func (s *LLMDeploymentService) DeployModelConfiguration(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	if !models.IsValidOrigin(params.Origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", params.Origin)
	}
	var modelConfig api.ModelConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &modelConfig); err != nil {
		return nil, fmt.Errorf("%w: failed to parse model configuration: %v", ErrModelValidation, err)
	}

	// On update, inherit persisted target credentials this request omits; a
	// target credential is write-only and cannot be restated by a caller.
	source, err := storedSourceForUpdate(s.db, params.ID)
	if err != nil {
		return nil, err
	}
	inheritModelTargetCredentials(&modelConfig, source)

	// Render template expressions into a copy for validation and resolution; the
	// unrendered Model is what is persisted, and every replica re-renders it.
	renderHolder := &models.StoredConfig{Configuration: modelConfig}
	if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, err
	}
	rendered, ok := renderHolder.Configuration.(api.ModelConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected configuration type %T after rendering Model", renderHolder.Configuration)
	}

	if errs := config.NewModelValidator().Validate(&rendered); len(errs) > 0 {
		return nil, s.modelValidationError(params.Logger, rendered.Metadata.Name, errs)
	}
	if s.policyValidator != nil {
		if errs := s.policyValidator.ValidateModelPolicies(&rendered); len(errs) > 0 {
			return nil, s.modelValidationError(params.Logger, rendered.Metadata.Name, errs)
		}
	}

	endpointErrs, err := s.unknownModelEndpoints(rendered.Spec)
	if err != nil {
		return nil, err
	}
	resolved, err := modelgateway.Resolve(&rendered, s.modelLookup(), ModelResolveOptions(s.routerConfig, true))
	if err != nil {
		var resolveErrs modelgateway.Errors
		if errors.As(err, &resolveErrs) {
			return nil, s.modelValidationError(params.Logger, rendered.Metadata.Name, append(endpointErrs, resolveErrs...))
		}
		return nil, fmt.Errorf("failed to resolve model: %w", err)
	}
	if len(endpointErrs) > 0 {
		return nil, s.modelValidationError(params.Logger, rendered.Metadata.Name, endpointErrs)
	}

	desiredState := params.DesiredState
	if desiredState == "" {
		desiredState = models.StateDeployed
	}
	// A deployed Model must be servable on every endpoint it names. An undeployed
	// one is a draft: it is checked when it is next saved as deployed.
	if desiredState == models.StateDeployed {
		bindErrs, err := s.modelEndpointBindingErrors(resolved, rendered.Spec)
		if err != nil {
			return nil, err
		}
		if len(bindErrs) > 0 {
			return nil, s.modelValidationError(params.Logger, rendered.Metadata.Name, bindErrs)
		}
	}

	apiID := params.ID
	if apiID == "" {
		if apiID, err = GenerateUUID(); err != nil {
			return nil, fmt.Errorf("failed to generate Model ID: %w", err)
		}
	}

	// A model name is what clients send, so two Models answering to one name would
	// leave an endpoint unable to say which one a request meant.
	if err := s.validateModelNameUnique(apiID, rendered.Spec.ModelName); err != nil {
		return nil, err
	}

	now := time.Now()
	deployedAt := params.DeployedAt
	if deployedAt == nil {
		truncated := now.Truncate(time.Millisecond)
		deployedAt = &truncated
	}

	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                models.KindModel,
		Handle:              modelConfig.Metadata.Name,
		DisplayName:         modelConfig.Spec.DisplayName,
		Version:             modelConfig.Spec.Version,
		Configuration:       modelConfig,
		SourceConfiguration: modelConfig,
		DesiredState:        desiredState,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		// Not pushed to the control plane until it supports the kind.
		CPSyncStatus: "",
		CreatedAt:    now,
		UpdatedAt:    now,
		DeployedAt:   deployedAt,
	}

	isUpdate := params.IsUpdate
	if !isUpdate && params.ID != "" {
		if existing, err := s.db.GetConfig(params.ID); err == nil && existing != nil {
			isUpdate = true
		}
	}

	if err := s.deploymentService.validateArtifactConflicts(models.KindModel, storedCfg.UUID,
		storedCfg.DisplayName, storedCfg.Version, storedCfg.Handle); err != nil {
		return nil, err
	}

	// Rendered once more against the stored shape, so an invalid secret reference
	// fails the save rather than the first replica that consumes the event.
	if err := templateengine.RenderSpec(storedCfg, s.deploymentService.secretResolver, params.Logger); err != nil {
		return nil, err
	}
	storedCfg.Configuration = modelConfig
	storedCfg.SensitiveValues = nil

	affected, err := s.deploymentService.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to save or update Model configuration: %w", err)
	}
	if !affected {
		return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate, IsStale: true}, nil
	}

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	params.Logger.Info("Model configuration saved",
		slog.String("model_uuid", apiID),
		slog.String("handle", storedCfg.Handle),
		slog.String("model_name", rendered.Spec.ModelName),
		slog.String("action", action),
		slog.String("correlation_id", params.CorrelationID))
	s.publishModelEvent(action, apiID, params.CorrelationID, params.Logger)

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

func (s *LLMDeploymentService) modelValidationError(logger *slog.Logger, handle string, errs []config.ValidationError) error {
	parts := make([]string, 0, len(errs))
	for i, e := range errs {
		if logger != nil {
			logger.Warn("Model validation error", slog.String("handle", handle),
				slog.String("field", e.Field), slog.String("message", e.Message))
		}
		parts = append(parts, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
	}
	return fmt.Errorf("%w: %d error(s): %s", ErrModelValidation, len(errs), strings.Join(parts, "; "))
}

// ModelEndpointTemplateParams are the extraction fields an endpoint's inbound
// template contributes to every policy in a Model chain's inbound scope, for the
// endpoint's dispatch operation: the base spec's fields, overridden by the
// template's resource mapping for that path when one matches. A multi-provider LLM
// proxy merges its inbound template the same way.
func ModelEndpointTemplateParams(tmpl *models.StoredLLMProviderTemplate, dispatchPath string) (map[string]interface{}, error) {
	return buildTemplateParams(tmpl, dispatchPath)
}

// modelEndpointBindingErrors binds the resolved Model to each endpoint it names,
// deployed or not, exactly as the compiler and the endpoint-change guard do, and
// returns why each endpoint that cannot serve it cannot: no transformer entry for
// a provider whose template is not the endpoint's format, a placeholder in an
// entry's params or upstreamPaths that the provider does not fill, or a path a
// routed target's provider refuses through its accessControl. Without this the
// Model would be saved, and left out of that endpoint with only a log line.
func (s *LLMDeploymentService) modelEndpointBindingErrors(m *modelgateway.Model,
	spec api.ModelConfigData) ([]config.ValidationError, error) {
	var errs []config.ValidationError
	for i, handle := range spec.Endpoints {
		field := fmt.Sprintf("spec.endpoints[%d]", i)
		cfg, err := s.db.GetConfigByKindAndHandle(models.KindModelEndpoint, handle)
		if err != nil {
			return nil, fmt.Errorf("failed to look up model endpoint '%s': %w", handle, err)
		}
		endpoint, ok := cfg.SourceConfiguration.(api.ModelEndpointConfiguration)
		if !ok {
			return nil, fmt.Errorf("stored model endpoint '%s' has unexpected configuration %T", handle,
				cfg.SourceConfiguration)
		}
		view, err := s.endpointBindingView(handle, endpoint.Spec)
		if err != nil {
			errs = append(errs, config.ValidationError{Field: field,
				Message: fmt.Sprintf("model endpoint '%s' cannot serve Models: %s", handle, err.Error())})
			continue
		}
		_, err = modelgateway.Bind(m, view)
		if err == nil {
			continue
		}
		var bindErrs modelgateway.Errors
		if !errors.As(err, &bindErrs) {
			return nil, fmt.Errorf("failed to bind Model to model endpoint '%s': %w", handle, err)
		}
		// Every message names the endpoint; the field says which entry of the Model's.
		for _, be := range bindErrs {
			errs = append(errs, config.ValidationError{Field: field, Message: be.Message})
		}
	}
	return errs, nil
}

// unknownModelEndpoints returns an error for every handle in spec.endpoints that
// names no model endpoint on this gateway. A Model names only endpoints that
// exist: a misspelt handle would otherwise be saved and served nowhere, with
// nothing to say why. Every handle is checked on every save, so one left behind
// by an endpoint deleted while the Model was undeployed is refused too. An
// undeployed endpoint exists, so it may be named. The compiler still skips a
// handle it cannot find, for Models saved before this check.
func (s *LLMDeploymentService) unknownModelEndpoints(spec api.ModelConfigData) ([]config.ValidationError, error) {
	var errs []config.ValidationError
	for i, handle := range spec.Endpoints {
		_, err := s.db.GetConfigByKindAndHandle(models.KindModelEndpoint, handle)
		switch {
		case err == nil:
		case storage.IsNotFoundError(err):
			errs = append(errs, config.ValidationError{
				Field:   fmt.Sprintf("spec.endpoints[%d]", i),
				Message: fmt.Sprintf("model endpoint '%s' not found", handle),
			})
		default:
			return nil, fmt.Errorf("failed to look up model endpoint '%s': %w", handle, err)
		}
	}
	return errs, nil
}

func (s *LLMDeploymentService) validateModelNameUnique(currentID, modelName string) error {
	existing, err := s.db.GetAllConfigsByKind(models.KindModel)
	if err != nil {
		return fmt.Errorf("failed to check model name uniqueness: %w", err)
	}
	for _, cfg := range existing {
		if cfg.UUID == currentID {
			continue
		}
		other, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if !ok {
			continue
		}
		if other.Spec.ModelName == modelName {
			return fmt.Errorf("%w: Model '%s' already serves model name '%s'", storage.ErrConflict, cfg.Handle, modelName)
		}
	}
	return nil
}

// CreateModel creates a Model.
func (s *LLMDeploymentService) CreateModel(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	return s.DeployModelConfiguration(params)
}

// UpdateModel updates the Model with the given handle. A deploymentState of
// "undeployed" keeps the Model and stops serving its name.
func (s *LLMDeploymentService) UpdateModel(handle string, params LLMDeploymentParams) (*APIDeploymentResult, error) {
	existing, err := s.GetModelByHandle(handle)
	if err != nil {
		return nil, err
	}

	var incoming api.ModelConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &incoming); err != nil {
		return nil, fmt.Errorf("%w: failed to parse model configuration: %v", ErrModelValidation, err)
	}
	if incoming.Metadata.Name != handle {
		return nil, fmt.Errorf("%w: metadata.name '%s' does not match the Model being updated ('%s')",
			ErrModelValidation, incoming.Metadata.Name, handle)
	}
	if incoming.Spec.DeploymentState != nil && *incoming.Spec.DeploymentState == api.ModelConfigDataDeploymentStateUndeployed {
		params.DesiredState = models.StateUndeployed
	} else {
		now := time.Now().Truncate(time.Millisecond)
		params.DeployedAt = &now
	}
	params.ID = existing.UUID
	params.IsUpdate = true
	return s.DeployModelConfiguration(params)
}

// GetModelByHandle returns the Model with the given handle.
func (s *LLMDeploymentService) GetModelByHandle(handle string) (*models.StoredConfig, error) {
	return s.db.GetConfigByKindAndHandle(models.KindModel, handle)
}

// ListModels returns the stored Models, filtered.
func (s *LLMDeploymentService) ListModels(params api.ListModelsParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(models.KindModel)
	if stored, err := s.db.GetAllConfigsByKind(models.KindModel); err == nil {
		configs = stored
	}
	filtered := make([]*models.StoredConfig, 0, len(configs))
	for _, cfg := range configs {
		m, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if !ok {
			continue
		}
		if params.DisplayName != nil && m.Spec.DisplayName != *params.DisplayName {
			continue
		}
		if params.Version != nil && m.Spec.Version != *params.Version {
			continue
		}
		if params.ModelName != nil && m.Spec.ModelName != *params.ModelName {
			continue
		}
		if params.Status != nil && string(cfg.DesiredState) != string(*params.Status) {
			continue
		}
		filtered = append(filtered, cfg)
	}
	return filtered
}

// DeleteModel deletes the Model with the given handle. Its model name stops
// resolving on every endpoint once replicas consume the event.
func (s *LLMDeploymentService) DeleteModel(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	cfg, err := s.GetModelByHandle(handle)
	if err != nil {
		return nil, err
	}
	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete Model from database: %w", err)
	}
	s.publishModelEvent("DELETE", cfg.UUID, correlationID, logger)
	return cfg, nil
}

// ModelsReferencingProvider returns the handles of every stored Model with a
// target on the provider, sorted. A provider these depend on cannot be deleted or
// undeployed: every one of them would stop resolving.
func (s *LLMDeploymentService) ModelsReferencingProvider(providerHandle string) ([]string, error) {
	configs, err := s.db.GetAllConfigsByKind(models.KindModel)
	if err != nil {
		return nil, fmt.Errorf("failed to list Models: %w", err)
	}
	var handles []string
	for _, cfg := range configs {
		m, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if !ok {
			continue
		}
		for _, id := range models.ModelTargetProviderIDs(m.Spec) {
			if id == providerHandle {
				handles = append(handles, cfg.Handle)
				break
			}
		}
	}
	sort.Strings(handles)
	return handles, nil
}

// ensureProviderNotReferenced refuses to delete or undeploy a provider a Model
// targets, naming the Models.
func (s *LLMDeploymentService) ensureProviderNotReferenced(providerHandle, operation string) error {
	handles, err := s.ModelsReferencingProvider(providerHandle)
	if err != nil {
		return err
	}
	if len(handles) == 0 {
		return nil
	}
	return conflictf("cannot %s provider '%s': it is a target of Model(s) %s; remove those targets first",
		operation, providerHandle, strings.Join(handles, ", "))
}

// stateConflict is a 409 that is not a duplicate: the request conflicts with the state
// of a resource, such as a provider Models depend on. It is a storage.ErrConflict, so
// every handler answers it with 409, but its message is only its own — not the
// sentinel's "configuration already exists", which would misdescribe a refused delete.
type stateConflict struct{ msg string }

func (e *stateConflict) Error() string { return e.msg }
func (e *stateConflict) Unwrap() error { return storage.ErrConflict }

func conflictf(format string, args ...any) error {
	return &stateConflict{msg: fmt.Sprintf(format, args...)}
}
