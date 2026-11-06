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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway/inbound"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
)

var (
	// ErrModelEndpointValidation is returned when a model endpoint fails parsing or
	// validation.
	ErrModelEndpointValidation = errors.New("model endpoint configuration invalid")

	// ErrModelEndpointReserved is returned for a write the provisioned default
	// endpoint does not allow through the API: creating or deleting `openai`, or
	// moving it off its configured context, while provisioning is on.
	ErrModelEndpointReserved = errors.New("model endpoint reserved")
)

// The default authentication a model endpoint gets when it omits spec.policies:
// api-key-auth reading `Authorization: Bearer <key>`, which is how every OpenAI
// client library sends its key.
const (
	modelEndpointAuthHeader      = "Authorization"
	modelEndpointAuthValuePrefix = "Bearer"
)

func (s *LLMDeploymentService) publishModelEndpointEvent(action, entityID, correlationID string, logger *slog.Logger) {
	s.deploymentService.publishEvent(eventhub.EventTypeModelEndpoint, action, entityID, correlationID, logger)
}

// DeployModelEndpointConfiguration parses, validates and persists a model
// endpoint, then publishes the event every replica recompiles it from.
func (s *LLMDeploymentService) DeployModelEndpointConfiguration(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	if !models.IsValidOrigin(params.Origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", params.Origin)
	}
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	var endpoint api.ModelEndpointConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &endpoint); err != nil {
		return nil, fmt.Errorf("%w: failed to parse model endpoint configuration: %v", ErrModelEndpointValidation, err)
	}
	handle := endpoint.Metadata.Name

	existing, err := s.db.GetConfigByKindAndHandle(models.KindModelEndpoint, handle)
	if err != nil && !storage.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to look up model endpoint '%s': %w", handle, err)
	}
	if storage.IsNotFoundError(err) {
		existing = nil
	}
	if err := s.applyDefaultEndpointRules(&endpoint, existing, params.Provisioning); err != nil {
		return nil, err
	}

	// An endpoint that omits spec.policies authenticates with api-key-auth, as the
	// provisioned one does. An explicit list, even an empty one, is the author's
	// choice and is used as given.
	if endpoint.Spec.Policies == nil {
		auth, err := s.defaultEndpointAuthPolicy()
		if err != nil {
			return nil, err
		}
		endpoint.Spec.Policies = &[]api.Policy{auth}
	}

	// Render template expressions into a copy for validation; the unrendered
	// endpoint is what is persisted, and every replica re-renders it.
	renderHolder := &models.StoredConfig{Configuration: endpoint}
	if err := templateengine.RenderSpec(renderHolder, s.deploymentService.secretResolver, logger); err != nil {
		return nil, err
	}
	rendered, ok := renderHolder.Configuration.(api.ModelEndpointConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected configuration type %T after rendering model endpoint", renderHolder.Configuration)
	}

	if errs := config.NewModelEndpointValidator().Validate(&rendered); len(errs) > 0 {
		return nil, validationErrorOf(ErrModelEndpointValidation, logger, "Model endpoint", handle, errs)
	}
	if s.policyValidator != nil {
		if errs := s.policyValidator.ValidateModelEndpointPolicies(&rendered); len(errs) > 0 {
			return nil, validationErrorOf(ErrModelEndpointValidation, logger, "Model endpoint", handle, errs)
		}
	}
	if err := s.validateInboundTemplate(rendered.Spec); err != nil {
		return nil, err
	}
	for _, w := range config.ModelEndpointWarnings(rendered.Spec) {
		logger.Warn("Model endpoint configuration warning", slog.String("handle", handle), slog.String("warning", w))
	}

	id := models.ModelEndpointUUID(handle)
	if existing != nil {
		id = existing.UUID
	}

	if err := s.validateEndpointDisplayNameUnique(id, rendered.Spec.DisplayName); err != nil {
		return nil, err
	}
	if err := s.validateEndpointRouteUnique(id, rendered.Spec); err != nil {
		return nil, err
	}

	// An endpoint change recompiles every Model it serves. One that would leave a
	// Model it serves today unable to compile — a removed transformer, an inbound
	// format its providers cannot be reached from — is refused, naming the Models,
	// rather than silently taking them off the endpoint.
	if existing != nil {
		if old, ok := existing.SourceConfiguration.(api.ModelEndpointConfiguration); ok {
			broken, err := s.modelsBrokenByEndpointChange(handle, &old.Spec, rendered.Spec)
			if err != nil {
				return nil, err
			}
			if len(broken) > 0 {
				return nil, conflictf("this change to model endpoint '%s' would stop it serving Model(s) it serves "+
					"today: %s", handle, strings.Join(broken, "; "))
			}
		}
	}

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
		UUID:                id,
		Kind:                models.KindModelEndpoint,
		Handle:              handle,
		DisplayName:         endpoint.Spec.DisplayName,
		Version:             endpoint.Spec.Version,
		Configuration:       endpoint,
		SourceConfiguration: endpoint,
		DesiredState:        desiredState,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		// Not pushed to the control plane until it supports the kind.
		CPSyncStatus: "",
		CreatedAt:    now,
		UpdatedAt:    now,
		DeployedAt:   deployedAt,
	}

	isUpdate := existing != nil || params.IsUpdate
	if err := s.deploymentService.validateArtifactConflicts(models.KindModelEndpoint, storedCfg.UUID,
		storedCfg.DisplayName, storedCfg.Version, storedCfg.Handle); err != nil {
		return nil, err
	}

	// Rendered once more against the stored shape, so an invalid secret reference
	// fails the save rather than the first replica that consumes the event.
	if err := templateengine.RenderSpec(storedCfg, s.deploymentService.secretResolver, logger); err != nil {
		return nil, err
	}
	storedCfg.Configuration = endpoint
	storedCfg.SensitiveValues = nil

	affected, err := s.deploymentService.saveOrUpdateConfig(storedCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to save or update model endpoint configuration: %w", err)
	}
	if !affected {
		return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate, IsStale: true}, nil
	}

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	logger.Info("Model endpoint configuration saved",
		slog.String("uuid", id),
		slog.String("handle", handle),
		slog.String("inbound_template", models.EffectiveInboundTemplate(endpoint.Spec)),
		slog.String("action", action),
		slog.String("correlation_id", params.CorrelationID))
	s.publishModelEndpointEvent(action, id, params.CorrelationID, logger)

	return &APIDeploymentResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// applyDefaultEndpointRules enforces what the provisioned endpoint does not let the
// API change. While provisioning is on, `openai` is the gateway's: the API can
// neither create it nor move it off controller.models.endpoint.context. Its context is
// configuration, so a write that omits it keeps the configured one. With
// provisioning off it is an ordinary endpoint, still defaulting to the configured
// context.
func (s *LLMDeploymentService) applyDefaultEndpointRules(endpoint *api.ModelEndpointConfiguration,
	existing *models.StoredConfig, provisioning bool) error {
	if endpoint.Metadata.Name != models.DefaultModelEndpointHandle {
		return nil
	}
	reserved := s.modelsConfig.ProvisionsDefaultEndpoint()
	if reserved && existing == nil && !provisioning {
		return fmt.Errorf("%w: the model endpoint '%s' is provisioned by the gateway and cannot be created "+
			"through the API", ErrModelEndpointReserved, models.DefaultModelEndpointHandle)
	}
	configured := s.modelsConfig.EffectiveContext()
	if endpoint.Spec.Context == nil || strings.TrimSpace(*endpoint.Spec.Context) == "" {
		endpoint.Spec.Context = &configured
		return nil
	}
	if reserved && strings.TrimSpace(*endpoint.Spec.Context) != configured {
		return fmt.Errorf("%w: the model endpoint '%s' is served at '%s', set by controller.models.endpoint.context",
			ErrModelEndpointReserved, models.DefaultModelEndpointHandle, configured)
	}
	return nil
}

// defaultEndpointAuthPolicy is api-key-auth on `Authorization: Bearer <key>`, at
// the loaded major version.
func (s *LLMDeploymentService) defaultEndpointAuthPolicy() (api.Policy, error) {
	authVersion, err := s.loadedMajorVersion(constants.API_KEY_AUTH_POLICY_NAME)
	if err != nil {
		// Without it the omitted policies cannot default to authentication. Refuse,
		// rather than serve the endpoint open when its author did not ask for that.
		return api.Policy{}, fmt.Errorf("%w: %s is not loaded, so spec.policies cannot default to it; "+
			"set spec.policies explicitly: %v", ErrModelEndpointValidation,
			constants.API_KEY_AUTH_POLICY_NAME, err)
	}
	params := map[string]interface{}{
		"key":         modelEndpointAuthHeader,
		"in":          "header",
		"valuePrefix": modelEndpointAuthValuePrefix,
	}
	return api.Policy{Name: constants.API_KEY_AUTH_POLICY_NAME, Version: authVersion, Params: &params}, nil
}

// validateInboundTemplate checks that the endpoint's inbound template is loaded
// and declares a requestModel its inbound profile can read: the resolver reads the
// model where the template says, so a template it cannot follow would leave the
// endpoint unable to dispatch anything.
func (s *LLMDeploymentService) validateInboundTemplate(spec api.ModelEndpointConfigData) error {
	template := models.EffectiveInboundTemplate(spec)
	profile, ok := inbound.Lookup(template)
	if !ok {
		return fmt.Errorf("%w: spec.inboundTemplate '%s' is not supported", ErrModelEndpointValidation, template)
	}
	tmpl, err := s.db.GetLLMProviderTemplateByHandle(template)
	if err != nil || tmpl == nil {
		return fmt.Errorf("%w: spec.inboundTemplate '%s' names a provider template that is not loaded",
			ErrModelEndpointValidation, template)
	}
	params, err := ModelEndpointTemplateParams(tmpl, profile.Dispatch[0].Path)
	if err != nil {
		return fmt.Errorf("%w: spec.inboundTemplate '%s': %v", ErrModelEndpointValidation, template, err)
	}
	loc, found := inbound.LocationFromParams(params, "requestModel")
	if !found {
		return fmt.Errorf("%w: provider template '%s' declares no requestModel, so the endpoint cannot tell which "+
			"Model a request names", ErrModelEndpointValidation, template)
	}
	if _, err := profile.ValidateModelLocation(loc); err != nil {
		return fmt.Errorf("%w: provider template '%s': %v", ErrModelEndpointValidation, template, err)
	}
	return nil
}

// validateEndpointDisplayNameUnique refuses a display name another endpoint has.
// API-level rate-limit counters key on the API's display name, so two endpoints
// sharing one would share their endpoint-wide limits.
func (s *LLMDeploymentService) validateEndpointDisplayNameUnique(currentID, displayName string) error {
	existing, err := s.db.GetAllConfigsByKind(models.KindModelEndpoint)
	if err != nil {
		return fmt.Errorf("failed to check model endpoint display name uniqueness: %w", err)
	}
	for _, cfg := range existing {
		if cfg.UUID != currentID && strings.EqualFold(strings.TrimSpace(cfg.DisplayName), strings.TrimSpace(displayName)) {
			return fmt.Errorf("%w: model endpoint '%s' already has the display name '%s'", storage.ErrConflict,
				cfg.Handle, displayName)
		}
	}
	return nil
}

// validateEndpointRouteUnique refuses a context and vhost another artifact on the
// gateway already serves: the endpoint's routes would shadow that artifact's, or
// be shadowed by them.
func (s *LLMDeploymentService) validateEndpointRouteUnique(currentID string, spec api.ModelEndpointConfigData) error {
	if spec.Context == nil {
		return nil
	}
	configs, err := s.db.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("failed to check the model endpoint's context: %w", err)
	}
	if other := conflictingArtifact(configs, currentID, *spec.Context, spec.Vhost); other != nil {
		return conflictf("context '%s' is already served by %s '%s' on the same vhost; choose another context or "+
			"vhost for the model endpoint", strings.TrimSpace(*spec.Context), other.Kind, other.Handle)
	}
	return nil
}

// conflictingArtifact returns an artifact, other than currentID, that serves the
// same context on a vhost that may overlap. Two explicit, different vhosts never
// overlap; an absent vhost is the gateway's default and may overlap any.
func conflictingArtifact(configs []*models.StoredConfig, currentID, context string, vhost *string) *models.StoredConfig {
	want := normalizedContext(context)
	mine := trimmedOrEmpty(vhost)
	for _, cfg := range configs {
		if cfg == nil || cfg.UUID == currentID || models.IsContributingKind(cfg.Kind) {
			continue
		}
		ctx, err := cfg.GetContext()
		if err != nil || strings.TrimSpace(ctx) == "" || normalizedContext(ctx) != want {
			continue
		}
		theirs := artifactVhost(cfg)
		if mine != "" && theirs != "" && !strings.EqualFold(mine, theirs) {
			continue
		}
		return cfg
	}
	return nil
}

// artifactVhost is the explicit vhost an artifact declares, or "" for one that
// declares none or whose kind keeps vhosts elsewhere.
func artifactVhost(cfg *models.StoredConfig) string {
	switch sc := cfg.SourceConfiguration.(type) {
	case api.ModelEndpointConfiguration:
		return trimmedOrEmpty(sc.Spec.Vhost)
	case api.LLMProviderConfiguration:
		return trimmedOrEmpty(sc.Spec.Vhost)
	case api.LLMProxyConfiguration:
		return trimmedOrEmpty(sc.Spec.Vhost)
	case api.MCPProxyConfiguration:
		return trimmedOrEmpty(sc.Spec.Vhost)
	}
	return ""
}

func normalizedContext(ctx string) string {
	ctx = strings.TrimSpace(ctx)
	if ctx == "" {
		return "/"
	}
	if !strings.HasPrefix(ctx, "/") {
		ctx = "/" + ctx
	}
	return path.Clean(ctx)
}

func trimmedOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// modelsBrokenByEndpointChange lists the deployed Models naming the endpoint that
// bind to it under the old spec and not under the new one, each with its reason.
// A Model that did not bind before is not counted: the change did not break it.
//
// It fails closed: when the Models cannot be listed, it returns the error and the
// change is refused, rather than saved without the check.
func (s *LLMDeploymentService) modelsBrokenByEndpointChange(handle string, oldSpec *api.ModelEndpointConfigData,
	newSpec api.ModelEndpointConfigData) ([]string, error) {
	configs, err := s.db.GetAllConfigsByKind(models.KindModel)
	if err != nil {
		return nil, fmt.Errorf("failed to list the Models model endpoint '%s' may serve: %w", handle, err)
	}
	newView, newErr := s.endpointBindingView(handle, newSpec)
	var oldView modelgateway.Endpoint
	var oldErr error = errors.New("no previous spec")
	if oldSpec != nil {
		oldView, oldErr = s.endpointBindingView(handle, *oldSpec)
	}

	lookup := s.modelLookup()
	opts := ModelResolveOptions(s.routerConfig, false)
	var broken []string
	sort.Slice(configs, func(i, j int) bool { return configs[i].Handle < configs[j].Handle })
	for _, cfg := range configs {
		if cfg.DesiredState != models.StateDeployed {
			continue
		}
		source, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if !ok || !models.ModelNamesEndpoint(source.Spec, handle) {
			continue
		}
		rendered, err := renderModel(source, s)
		if err != nil {
			continue
		}
		model, err := modelgateway.Resolve(rendered, lookup, opts)
		if err != nil {
			continue
		}
		if oldErr != nil {
			continue
		}
		if _, err := modelgateway.Bind(model, oldView); err != nil {
			continue
		}
		if newErr != nil {
			broken = append(broken, fmt.Sprintf("%s (%s)", cfg.Handle, newErr.Error()))
			continue
		}
		if _, err := modelgateway.Bind(model, newView); err != nil {
			broken = append(broken, fmt.Sprintf("%s (%s)", cfg.Handle, err.Error()))
		}
	}
	return broken, nil
}

// endpointBindingView is what modelgateway.Bind needs to know about an endpoint.
func (s *LLMDeploymentService) endpointBindingView(handle string, spec api.ModelEndpointConfigData) (modelgateway.Endpoint, error) {
	template := models.EffectiveInboundTemplate(spec)
	profile, ok := inbound.Lookup(template)
	if !ok {
		return modelgateway.Endpoint{}, fmt.Errorf("inbound template '%s' is not supported", template)
	}
	view := modelgateway.Endpoint{Handle: handle, InboundTemplate: template}
	if tmpl, err := s.db.GetLLMProviderTemplateByHandle(template); err == nil && tmpl != nil {
		view.InboundGroup = strings.TrimSpace(tmpl.GetGroupID())
	}
	if spec.Transformers != nil {
		view.Transformers = *spec.Transformers
	}
	for _, op := range profile.Dispatch {
		view.DispatchPaths = append(view.DispatchPaths, op.Path)
	}
	return view, nil
}

// renderModel renders a stored Model's template expressions for resolution.
func renderModel(source api.ModelConfiguration, s *LLMDeploymentService) (*api.ModelConfiguration, error) {
	holder := &models.StoredConfig{Configuration: source}
	if err := templateengine.RenderSpec(holder, s.deploymentService.secretResolver, nil); err != nil {
		return nil, err
	}
	rendered, ok := holder.Configuration.(api.ModelConfiguration)
	if !ok {
		return nil, fmt.Errorf("Model rendered to unexpected type %T", holder.Configuration)
	}
	return &rendered, nil
}

// CreateModelEndpoint creates a model endpoint.
func (s *LLMDeploymentService) CreateModelEndpoint(params LLMDeploymentParams) (*APIDeploymentResult, error) {
	var probe api.ModelEndpointConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &probe); err == nil && probe.Metadata.Name != "" {
		if existing, err := s.db.GetConfigByKindAndHandle(models.KindModelEndpoint, probe.Metadata.Name); err == nil && existing != nil {
			return nil, fmt.Errorf("%w: model endpoint '%s' already exists", storage.ErrConflict, probe.Metadata.Name)
		}
	}
	return s.DeployModelEndpointConfiguration(params)
}

// UpdateModelEndpoint updates the model endpoint with the given handle. A
// deploymentState of "undeployed" keeps it and stops serving it.
func (s *LLMDeploymentService) UpdateModelEndpoint(handle string, params LLMDeploymentParams) (*APIDeploymentResult, error) {
	existing, err := s.GetModelEndpointByHandle(handle)
	if err != nil {
		return nil, err
	}

	var incoming api.ModelEndpointConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &incoming); err != nil {
		return nil, fmt.Errorf("%w: failed to parse model endpoint configuration: %v", ErrModelEndpointValidation, err)
	}
	if incoming.Metadata.Name != handle {
		return nil, fmt.Errorf("%w: metadata.name '%s' does not match the model endpoint being updated ('%s')",
			ErrModelEndpointValidation, incoming.Metadata.Name, handle)
	}
	if incoming.Spec.DeploymentState != nil && *incoming.Spec.DeploymentState == api.ModelEndpointConfigDataDeploymentStateUndeployed {
		params.DesiredState = models.StateUndeployed
	} else {
		now := time.Now().Truncate(time.Millisecond)
		params.DeployedAt = &now
	}
	params.ID = existing.UUID
	params.IsUpdate = true
	return s.DeployModelEndpointConfiguration(params)
}

// GetModelEndpointByHandle returns the model endpoint with the given handle.
func (s *LLMDeploymentService) GetModelEndpointByHandle(handle string) (*models.StoredConfig, error) {
	return s.db.GetConfigByKindAndHandle(models.KindModelEndpoint, handle)
}

// ListModelEndpoints returns the stored model endpoints, filtered.
func (s *LLMDeploymentService) ListModelEndpoints(params api.ListModelEndpointsParams) []*models.StoredConfig {
	configs := s.store.GetAllByKind(models.KindModelEndpoint)
	if stored, err := s.db.GetAllConfigsByKind(models.KindModelEndpoint); err == nil {
		configs = stored
	}
	filtered := make([]*models.StoredConfig, 0, len(configs))
	for _, cfg := range configs {
		e, ok := cfg.SourceConfiguration.(api.ModelEndpointConfiguration)
		if !ok {
			continue
		}
		if params.DisplayName != nil && e.Spec.DisplayName != *params.DisplayName {
			continue
		}
		if params.Version != nil && e.Spec.Version != *params.Version {
			continue
		}
		if params.Context != nil && trimmedOrEmpty(e.Spec.Context) != strings.TrimSpace(*params.Context) {
			continue
		}
		if params.Vhost != nil && trimmedOrEmpty(e.Spec.Vhost) != strings.TrimSpace(*params.Vhost) {
			continue
		}
		if params.Status != nil && string(cfg.DesiredState) != string(*params.Status) {
			continue
		}
		filtered = append(filtered, cfg)
	}
	return filtered
}

// ModelsNamingEndpoint returns the handles of the deployed Models that name the
// endpoint, sorted.
func (s *LLMDeploymentService) ModelsNamingEndpoint(handle string) ([]string, error) {
	configs, err := s.db.GetAllConfigsByKind(models.KindModel)
	if err != nil {
		return nil, fmt.Errorf("failed to list Models: %w", err)
	}
	var handles []string
	for _, cfg := range configs {
		if cfg.DesiredState != models.StateDeployed {
			continue
		}
		m, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
		if ok && models.ModelNamesEndpoint(m.Spec, handle) {
			handles = append(handles, cfg.Handle)
		}
	}
	sort.Strings(handles)
	return handles, nil
}

// DeleteModelEndpoint deletes the model endpoint with the given handle, and its API
// keys with it. The provisioned endpoint cannot be deleted while provisioning is
// on — the provisioner would recreate it on the next start — and no endpoint can
// be deleted while a deployed Model names it.
func (s *LLMDeploymentService) DeleteModelEndpoint(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	cfg, err := s.GetModelEndpointByHandle(handle)
	if err != nil {
		return nil, err
	}
	if handle == models.DefaultModelEndpointHandle && s.modelsConfig.ProvisionsDefaultEndpoint() {
		return cfg, conflictf("cannot delete model endpoint '%s': it is provisioned by the gateway while "+
			"controller.models.endpoint.provision is set", handle)
	}
	named, err := s.ModelsNamingEndpoint(handle)
	if err != nil {
		return cfg, err
	}
	if len(named) > 0 {
		return cfg, conflictf("cannot delete model endpoint '%s': deployed Model(s) %s name it in spec.endpoints; "+
			"remove it from those Models first", handle, strings.Join(named, ", "))
	}
	// DeleteConfig removes the endpoint's API keys in the same transaction.
	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		return cfg, fmt.Errorf("failed to delete model endpoint from database: %w", err)
	}
	s.publishModelEndpointEvent("DELETE", cfg.UUID, correlationID, logger)
	return cfg, nil
}

// validationErrorOf joins validation errors into one error wrapping sentinel,
// logging each.
func validationErrorOf(sentinel error, logger *slog.Logger, what, handle string, errs []config.ValidationError) error {
	parts := make([]string, 0, len(errs))
	for i, e := range errs {
		if logger != nil {
			logger.Warn(what+" validation error", slog.String("handle", handle),
				slog.String("field", e.Field), slog.String("message", e.Message))
		}
		parts = append(parts, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
	}
	return fmt.Errorf("%w: %d error(s): %s", sentinel, len(errs), strings.Join(parts, "; "))
}
