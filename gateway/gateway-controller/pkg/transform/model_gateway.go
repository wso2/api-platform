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

package transform

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway/inbound"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// ModelCompileStatus is the outcome of compiling one Model into one endpoint.
type ModelCompileStatus struct {
	Endpoint string
	Handle   string
	Served   bool
	// Reason says why a Model is not served. Empty for a served one.
	Reason string
	// Warning is advice about a served Model, logged once.
	Warning string
}

// ModelGatewayCompiler compiles one model endpoint, and the Models that name it,
// into one runtime config.
//
// One config per endpoint, not one per Model, because the policy engine requires
// it: a request on the endpoint's dispatch route may only be bound to a chain
// carrying that route's own API id (the resolver's partition check), and a named
// upstream resolves to a cluster under that same id. So every chain for a Model
// the endpoint serves, and every target's cluster, lives in the endpoint's config;
// a Model on two endpoints is compiled twice, once into each. A Model on its own
// compiles to nothing (models.IsContributingKind).
//
// The compiler reads the Models from the in-memory store each time it runs. The
// Envoy side recompiles every config on every snapshot, so routes always reflect the
// current Models; the policy side recompiles an endpoint whenever a Model,
// provider or template event re-upserts it.
type ModelGatewayCompiler struct {
	store             *storage.ConfigStore
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string
	llmTransformer    *utils.LLMProviderTransformer
	secretResolver    funcs.SecretResolver

	// mu guards lastStatus, which is how an outcome is logged once when it changes
	// rather than on every snapshot that recompiles the endpoint. Keyed by endpoint
	// handle, then Model handle.
	mu         sync.Mutex
	lastStatus map[string]map[string]ModelCompileStatus
}

// NewModelGatewayCompiler creates the model endpoint compiler.
func NewModelGatewayCompiler(
	store *storage.ConfigStore,
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
	llmTransformer *utils.LLMProviderTransformer,
	secretResolver funcs.SecretResolver,
) *ModelGatewayCompiler {
	return &ModelGatewayCompiler{
		store:             store,
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		latestVersions:    config.BuildLatestVersionIndex(policyDefinitions),
		llmTransformer:    llmTransformer,
		secretResolver:    secretResolver,
		lastStatus:        make(map[string]map[string]ModelCompileStatus),
	}
}

// modelsConfig returns the [controller.models] configuration.
func (c *ModelGatewayCompiler) modelsConfig() config.ModelsConfig {
	if c.systemConfig == nil {
		return config.ModelsConfig{}
	}
	return c.systemConfig.Controller.Models
}

// endpointContext is the context an endpoint is served at. The provisioned
// endpoint's comes from configuration, which also fills it when the stored spec
// predates a change to controller.models.endpoint.context.
func endpointContext(handle string, spec api.ModelEndpointConfigData, mcfg config.ModelsConfig) string {
	if handle == models.DefaultModelEndpointHandle && mcfg.ProvisionsDefaultEndpoint() {
		return mcfg.EffectiveContext()
	}
	if spec.Context != nil && strings.TrimSpace(*spec.Context) != "" {
		return strings.TrimSpace(*spec.Context)
	}
	if handle == models.DefaultModelEndpointHandle {
		return mcfg.EffectiveContext()
	}
	return ""
}

// endpointCompile is everything one endpoint's compile derives from the endpoint
// itself, before any Model is read.
type endpointCompile struct {
	handle         string
	profile        inbound.Profile
	view           modelgateway.Endpoint
	templateParams map[string]interface{}
	modelLocation  inbound.Location
}

// Compile compiles one model endpoint from its stored config.
//
// It never fails the endpoint as a whole for something one Model did: a Model
// that no longer resolves, or that this endpoint cannot serve, is left out,
// logged, and reported in the returned statuses, and every other Model keeps
// serving. It returns an error only for an endpoint it cannot read at all.
//
// With the feature off, or an inbound format it cannot serve, it returns a config
// with no routes: the endpoint then serves nothing, and its Models stay stored.
func (c *ModelGatewayCompiler) Compile(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, []ModelCompileStatus, error) {
	endpoint, err := modelEndpointConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	mcfg := c.modelsConfig()
	template := models.EffectiveInboundTemplate(endpoint.Spec)

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID:        cfg.UUID,
			Kind:        models.KindLlmProxy,
			Handle:      cfg.Handle,
			Version:     endpoint.Spec.Version,
			DisplayName: endpoint.Spec.DisplayName,
			LLM: &models.LLMMetadata{
				TemplateHandle: template,
				ProviderName:   models.ModelEndpointProviderName(cfg.Handle),
			},
		},
		Context:             endpointContext(cfg.Handle, endpoint.Spec, mcfg),
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes:              make(map[string]*models.Route),
		PolicyChains:        make(map[string]*models.PolicyChain),
		UpstreamClusters:    make(map[string]*models.UpstreamCluster),
	}
	sensitive := newStringSet(cfg.SensitiveValues)

	if !mcfg.Enabled {
		slog.Warn("A model endpoint exists but controller.models.enabled is off; it serves nothing",
			slog.String("endpoint", cfg.Handle))
		return rdc, nil, nil
	}
	if rdc.Context == "" {
		slog.Error("Model endpoint has no context; it serves nothing", slog.String("endpoint", cfg.Handle))
		return rdc, nil, nil
	}

	ec, ok := c.prepareEndpoint(cfg.Handle, endpoint, template)
	if !ok {
		return rdc, nil, nil
	}

	served, statuses := c.modelsForEndpoint(ec)
	for i := range served {
		sensitive.add(served[i].sensitive...)
	}

	vhosts := agentVhosts(endpoint.Spec.Vhost, c.routerConfig.VHosts.Main.Default)

	routeTimeout, idleTimeout, err := xds.ResolveResilience(endpoint.Spec.Resilience)
	if err != nil {
		return nil, nil, fmt.Errorf("model endpoint %q: invalid resilience: %w", cfg.Handle, err)
	}
	timeout := buildRouteTimeout(nil, routeTimeout, nil, idleTimeout)

	systemProps := map[string]any{
		utils.SharedParamsKey: map[string]interface{}{"api_kind": models.KindLlmProxy},
	}
	endpointPolicies := resolvePolicyInstances(c.policyDefinitions, c.latestVersions,
		endpoint.Spec.Policies, policyv1alpha.LevelAPI)
	endpointFault := collectFaultPolicies(c.policyDefinitions, c.latestVersions,
		endpoint.Spec.FaultPolicies, policyv1alpha.LevelAPI)
	faultSystem := sdkChainToModel(utils.FaultSystemPolicies(c.systemConfig, nil)).Policies

	// Clusters: one per target provider, shared by every Model on it, plus the
	// fail-closed default. Each served Model built its providers' clusters when it
	// compiled, so every cluster here is one a served Model's requests name.
	unrouted := c.addUnroutedCluster(rdc)
	unroutedInfo := policyenginev1.UpstreamInfo{ClusterName: unrouted}
	for _, m := range served {
		for provider, cluster := range m.clusters {
			key := providerClusterKey(rdc, provider)
			if _, exists := rdc.UpstreamClusters[key]; !exists {
				rdc.UpstreamClusters[key] = cluster
			}
		}
	}

	// The dispatch routes resolve each request to its Model's chain.
	modelOperations := make(map[string]string, len(served))
	for _, m := range served {
		modelOperations[m.model.ModelName] = m.model.Handle
	}
	resolverConfig, err := json.Marshal(map[string]any{
		"models":         modelOperations,
		"errorOperation": constants.LLM_MODEL_DISPATCH_ERROR_OPERATION,
		"modelLocation":  ec.modelLocation,
		"stream":         ec.profile.Stream,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("model endpoint %q: resolver config: %w", cfg.Handle, err)
	}

	catalog, err := c.catalogPolicyInstance(ec.profile, served)
	if err != nil {
		return nil, nil, err
	}
	dispatchError, err := c.modelSystemPolicyInstance(map[string]any{
		constants.MODEL_POLICY_PARAM_DISPATCH_ERROR: map[string]any{
			constants.MODEL_POLICY_PARAM_FORMAT: ec.profile.Format,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	for _, vhost := range vhosts {
		order := 0
		for _, op := range ec.profile.Dispatch {
			path := xds.ConstructFullPath(rdc.Context, "", op.Path)
			key := agentRouteKey(op.Method, path, vhost)
			rdc.Routes[key] = &models.Route{
				Method:          op.Method,
				Path:            path,
				OperationPath:   op.Path,
				PathMatchType:   "Exact",
				Vhost:           vhost,
				AutoHostRewrite: true,
				NoPathRewrite:   true,
				Timeout:         timeout,
				Order:           order,
				Upstream: models.RouteUpstream{
					ClusterKey:       unrouted,
					UseClusterHeader: true,
					DefaultCluster:   unrouted,
					Default:          &unroutedInfo,
				},
				ResolverName:        constants.LLM_MODEL_DISPATCH_RESOLVER_NAME,
				ResolverConfig:      resolverConfig,
				MaxRequestBodyBytes: c.maxRequestBodyBytes(),
				ErrorFormat:         ec.profile.Format,
			}
			order++
		}

		for _, op := range ec.profile.Catalog {
			path := xds.ConstructFullPath(rdc.Context, "", op.Path)
			key := agentRouteKey(op.Method, path, vhost)
			rdc.Routes[key] = &models.Route{
				Method:          op.Method,
				Path:            path,
				OperationPath:   op.Path,
				PathMatchType:   "Exact",
				Vhost:           vhost,
				AutoHostRewrite: true,
				NoPathRewrite:   true,
				Timeout:         timeout,
				Order:           order,
				Upstream: models.RouteUpstream{
					ClusterKey: unrouted,
					Default:    &unroutedInfo,
				},
				ErrorFormat: ec.profile.Format,
			}
			order++
			// The catalog chain runs the endpoint's own policies before answering, so
			// authentication decides the request first: an unauthenticated caller is
			// refused before it can list the endpoint's models.
			rdc.PolicyChains[key] = c.chain(systemProps, concat(endpointPolicies, *catalog), endpointFault, faultSystem)
		}

		// The dispatch-error chain likewise authenticates before answering, so a
		// caller with no credential cannot learn which model names exist.
		rdc.PolicyChains[rdc.ChainKeyFor(vhost, constants.LLM_MODEL_DISPATCH_ERROR_OPERATION)] = c.chain(systemProps,
			concat(endpointPolicies, dispatchError), endpointFault, faultSystem)

		for _, m := range served {
			key := rdc.ChainKeyFor(vhost, m.model.Handle)
			if _, exists := rdc.PolicyChains[key]; exists {
				return nil, nil, fmt.Errorf("model endpoint %q: chain %q would be built twice", cfg.Handle, key)
			}
			modelFault := mergeFaultPolicies(m.fault, endpointFault)
			rdc.PolicyChains[key] = c.chain(systemProps, concat(endpointPolicies, m.policies...), modelFault, faultSystem)
		}
	}

	rdc.SensitiveValues = sensitive.values()
	if err := rdc.ValidateResolution(); err != nil {
		return nil, nil, fmt.Errorf("model endpoint %q: %w", cfg.Handle, err)
	}

	c.logStatusChanges(cfg.Handle, statuses)
	return rdc, statuses, nil
}

// prepareEndpoint derives what every Model's compile on this endpoint shares: the
// inbound profile, the inbound template's extraction fields, and where requests
// carry the model. An endpoint whose format this gateway cannot serve is logged
// and serves nothing.
func (c *ModelGatewayCompiler) prepareEndpoint(handle string, endpoint *api.ModelEndpointConfiguration,
	template string) (endpointCompile, bool) {
	profile, ok := inbound.Lookup(template)
	if !ok {
		slog.Error("Model endpoint has an inbound template with no inbound profile; it serves nothing",
			slog.String("endpoint", handle), slog.String("inbound_template", template),
			slog.String("supported", strings.Join(inbound.Templates(), ", ")))
		return endpointCompile{}, false
	}
	tmpl, err := c.store.GetTemplateByHandle(template)
	if err != nil || tmpl == nil {
		// Every chain is built around the inbound template's extraction fields;
		// without it nothing can be built correctly, so nothing is served.
		slog.Error("Model endpoint cannot be compiled: its inbound template is not loaded; it serves nothing",
			slog.String("endpoint", handle), slog.String("inbound_template", template))
		return endpointCompile{}, false
	}
	templateParams, err := utils.ModelEndpointTemplateParams(tmpl, profile.Dispatch[0].Path)
	if err != nil {
		slog.Error("Model endpoint cannot be compiled: its inbound template's fields could not be read",
			slog.String("endpoint", handle), slog.Any("error", err))
		return endpointCompile{}, false
	}
	loc, found := inbound.LocationFromParams(templateParams, "requestModel")
	if !found {
		slog.Error("Model endpoint cannot be compiled: its inbound template declares no requestModel; it serves nothing",
			slog.String("endpoint", handle), slog.String("inbound_template", template))
		return endpointCompile{}, false
	}
	loc, err = profile.ValidateModelLocation(loc)
	if err != nil {
		slog.Error("Model endpoint cannot be compiled: its inbound template's requestModel cannot be read; it serves nothing",
			slog.String("endpoint", handle), slog.String("inbound_template", template), slog.Any("error", err))
		return endpointCompile{}, false
	}

	view := modelgateway.Endpoint{
		Handle:          handle,
		InboundTemplate: template,
		InboundGroup:    strings.TrimSpace(tmpl.GetGroupID()),
	}
	if endpoint.Spec.Transformers != nil {
		view.Transformers = *endpoint.Spec.Transformers
	}
	for _, op := range profile.Dispatch {
		view.DispatchPaths = append(view.DispatchPaths, op.Path)
	}
	return endpointCompile{
		handle:         handle,
		profile:        profile,
		view:           view,
		templateParams: templateParams,
		modelLocation:  loc,
	}, true
}

// servedModel is one Model that compiled, with the chain policies it contributes
// after the endpoint's own, and the cluster of each provider it targets.
type servedModel struct {
	model     *modelgateway.Model
	createdAt int64
	policies  []policyenginev1.PolicyInstance
	fault     []models.Policy
	sensitive []string
	clusters  map[string]*models.UpstreamCluster // by provider handle
}

// modelsForEndpoint resolves every deployed Model that names the endpoint and
// builds its chain policies, returning the ones that compiled (sorted by handle)
// and a status for every one.
func (c *ModelGatewayCompiler) modelsForEndpoint(ec endpointCompile) ([]servedModel, []ModelCompileStatus) {
	configs := c.store.GetAllByKind(models.KindModel)
	sort.Slice(configs, func(i, j int) bool { return configs[i].Handle < configs[j].Handle })

	lookup := utils.NewStoreModelLookup(c.store, c.policyDefinitions, c.secretResolver)
	opts := utils.ModelResolveOptions(c.routerConfig, false)

	var served []servedModel
	var statuses []ModelCompileStatus
	names := make(map[string]string, len(configs))
	for _, cfg := range configs {
		if cfg.DesiredState != models.StateDeployed {
			continue
		}
		spec, ok := modelSpecOf(cfg)
		if !ok || !models.ModelNamesEndpoint(spec, ec.handle) {
			continue
		}
		status := ModelCompileStatus{Endpoint: ec.handle, Handle: cfg.Handle}
		sm, warnings, err := c.compileModel(cfg, lookup, opts, ec)
		switch {
		case err != nil:
			status.Reason = err.Error()
		case names[sm.model.ModelName] != "":
			// The save path refuses a duplicate name; two arriving at once can still
			// race past it. The first by handle keeps the name, deterministically.
			status.Reason = fmt.Sprintf("model name '%s' is already served by Model '%s'",
				sm.model.ModelName, names[sm.model.ModelName])
		default:
			names[sm.model.ModelName] = cfg.Handle
			status.Served = true
			if w := c.mixedFormatWarning(spec, ec.handle); w != "" {
				warnings = append(warnings, w)
			}
			status.Warning = strings.Join(warnings, "; ")
			served = append(served, *sm)
		}
		statuses = append(statuses, status)
	}
	return served, statuses
}

// mixedFormatWarning warns when a Model with policies of its own is served on
// endpoints of more than one inbound format. Template-aware policies adapt to each
// endpoint's format, but a policy with hand-written JSON paths is tied to one.
func (c *ModelGatewayCompiler) mixedFormatWarning(spec api.ModelConfigData, current string) string {
	if spec.Policies == nil || len(*spec.Policies) == 0 {
		return ""
	}
	formats := map[string]bool{}
	for _, handle := range models.ModelEndpointHandles(spec) {
		cfg, err := c.store.GetByKindAndHandle(models.KindModelEndpoint, handle)
		if err != nil || cfg == nil {
			continue
		}
		if ep, err := modelEndpointConfig(cfg); err == nil {
			formats[models.EffectiveInboundTemplate(ep.Spec)] = true
		}
	}
	if len(formats) < 2 {
		return ""
	}
	list := make([]string, 0, len(formats))
	for f := range formats {
		list = append(list, f)
	}
	sort.Strings(list)
	return fmt.Sprintf("the Model has spec.policies and is served on endpoints of more than one inbound format (%s); "+
		"a policy with hand-written JSON paths reads only one of them", strings.Join(list, ", "))
}

// compileModel resolves one Model, binds it to the endpoint, and builds the
// policies its chain carries after the endpoint's own, in the order they run:
//
//	Model.policies → router → route job → per provider: transformer, credential,
//	loopback marker, cost → providerScope job
//
// On the response the chain runs back to front, so the providerScope job switches
// the provider metadata to the selected provider's before that provider's policies
// read the provider's own response, and the route job switches it back to the
// endpoint's inbound scope before the Model's own policies read the transformed one.
func (c *ModelGatewayCompiler) compileModel(
	cfg *models.StoredConfig,
	lookup modelgateway.Lookup,
	opts modelgateway.Options,
	ec endpointCompile,
) (*servedModel, []string, error) {
	source, sensitive, err := c.renderedModel(cfg)
	if err != nil {
		return nil, nil, err
	}
	model, err := modelgateway.Resolve(source, lookup, opts)
	if err != nil {
		return nil, nil, err
	}
	binding, err := modelgateway.Bind(model, ec.view)
	if err != nil {
		return nil, nil, err
	}

	sm := &servedModel{model: model, createdAt: cfg.CreatedAt.Unix()}
	sm.sensitive = append(sm.sensitive, sensitive...)

	// Each provider's cluster. One that cannot be built leaves the Model out, as a
	// policy that cannot be resolved does: served, its requests would name a cluster
	// Envoy does not have. Validation should have refused such a provider, so this
	// is the backstop.
	sm.clusters = make(map[string]*models.UpstreamCluster, len(model.ProviderHandles()))
	for _, handle := range model.ProviderHandles() {
		cluster, err := c.providerCluster(*firstTargetOn(model, handle))
		if err != nil {
			return nil, nil, fmt.Errorf("provider '%s': its upstream cluster cannot be built: %w", handle, err)
		}
		sm.clusters[handle] = cluster
	}

	// The Model's own policies, in the endpoint's inbound scope, with the inbound
	// template's extraction fields merged in exactly as an LLM proxy's policies are.
	if source.Spec.Policies != nil {
		withTemplate := make([]api.Policy, 0, len(*source.Spec.Policies))
		for _, p := range *source.Spec.Policies {
			base := map[string]interface{}{}
			if p.Params != nil {
				base = *p.Params
			}
			merged := mergeMaps(base, ec.templateParams)
			p.Params = &merged
			withTemplate = append(withTemplate, p)
		}
		sm.policies = append(sm.policies, resolvePolicyInstances(c.policyDefinitions, c.latestVersions,
			&withTemplate, policyv1alpha.LevelRoute)...)
	}

	if model.Routing != nil {
		params := mergeMaps(model.Routing.Params, ec.templateParams)
		router := []api.Policy{{Name: model.Routing.PolicyName, Version: model.Routing.PolicyVersion, Params: &params}}
		resolved := resolvePolicyInstances(c.policyDefinitions, c.latestVersions, &router, policyv1alpha.LevelRoute)
		if len(resolved) != 1 {
			return nil, nil, fmt.Errorf("routing policy '%s' %s is not loaded", model.Routing.PolicyName, model.Routing.PolicyVersion)
		}
		sm.policies = append(sm.policies, resolved...)
	}

	route, err := c.routePolicyInstance(model, ec)
	if err != nil {
		return nil, nil, err
	}
	sm.policies = append(sm.policies, route)

	cost := c.costPolicy()
	for _, handle := range model.ProviderHandles() {
		target := firstTargetOn(model, handle)
		sm.sensitive = append(sm.sensitive, target.Provider.SensitiveValues...)
		group, err := c.llmTransformer.ModelProviderGroupPolicies(target, binding.Transformers[handle])
		if err != nil {
			return nil, nil, fmt.Errorf("provider '%s': %w", handle, err)
		}
		if cost != nil {
			pol := *cost
			cond := utils.SelectedProviderExecutionCondition(handle)
			pol.ExecutionCondition = &cond
			group = append(group, pol)
		}
		// Unlike an author's policy, none of these may be dropped: a transformer or a
		// credential that failed to resolve would send the request to the provider in
		// the wrong shape or without its key. The Model is left out instead.
		resolvedGroup := resolvePolicyInstances(c.policyDefinitions, c.latestVersions, &group, policyv1alpha.LevelRoute)
		if len(resolvedGroup) != len(group) {
			return nil, nil, fmt.Errorf("provider '%s': a policy it needs is not loaded in this gateway", handle)
		}
		sm.policies = append(sm.policies, resolvedGroup...)
	}

	scope, err := c.modelSystemPolicyInstance(map[string]any{
		constants.MODEL_POLICY_PARAM_PROVIDER_SCOPE: map[string]any{},
	})
	if err != nil {
		return nil, nil, err
	}
	sm.policies = append(sm.policies, scope)

	sm.fault = collectFaultPolicies(c.policyDefinitions, c.latestVersions, source.Spec.FaultPolicies, policyv1alpha.LevelRoute)
	return sm, binding.Warnings, nil
}

// renderedModel returns the Model with its template expressions rendered, and the
// secrets rendering produced. The event listener stores the rendered form as
// Configuration; a Model loaded any other way is rendered here.
func (c *ModelGatewayCompiler) renderedModel(cfg *models.StoredConfig) (*api.ModelConfiguration, []string, error) {
	if m, ok := cfg.Configuration.(api.ModelConfiguration); ok && cfg.SensitiveValues != nil {
		return &m, cfg.SensitiveValues, nil
	}
	source, ok := cfg.SourceConfiguration.(api.ModelConfiguration)
	if !ok {
		return nil, nil, fmt.Errorf("stored Model has unexpected configuration %T", cfg.SourceConfiguration)
	}
	holder := &models.StoredConfig{Handle: cfg.Handle, Kind: cfg.Kind, Configuration: source}
	if err := templateengine.RenderSpec(holder, c.secretResolver, nil); err != nil {
		return nil, nil, err
	}
	rendered, ok := holder.Configuration.(api.ModelConfiguration)
	if !ok {
		return nil, nil, fmt.Errorf("Model rendered to unexpected type %T", holder.Configuration)
	}
	return &rendered, holder.SensitiveValues, nil
}

// routePolicyInstance builds the model system policy's route job for one Model on
// one endpoint: the targets it may resolve the router's choice to, where the
// request carries the model, and the endpoint's inbound scope it restores on the
// response.
func (c *ModelGatewayCompiler) routePolicyInstance(model *modelgateway.Model, ec endpointCompile) (policyenginev1.PolicyInstance, error) {
	targets := make([]any, 0, len(model.Targets))
	for _, t := range model.Targets {
		displayName := t.Provider.Config.Spec.DisplayName
		if displayName == "" {
			displayName = t.ProviderHandle
		}
		targets = append(targets, map[string]any{
			constants.MODEL_POLICY_PARAM_TARGET_NAME:           t.Name,
			constants.MODEL_POLICY_PARAM_TARGET_PROVIDER:       t.ProviderHandle,
			constants.MODEL_POLICY_PARAM_TARGET_UPSTREAM_MODEL: t.UpstreamModel,
			constants.MODEL_POLICY_PARAM_TARGET_TEMPLATE:       t.Provider.Config.Spec.Template,
			constants.MODEL_POLICY_PARAM_TARGET_DISPLAY_NAME:   displayName,
			constants.MODEL_POLICY_PARAM_TARGET_ROUTED:         t.Mode == modelgateway.ModeRoute,
		})
	}
	return c.modelSystemPolicyInstance(map[string]any{
		constants.MODEL_POLICY_PARAM_ROUTE: map[string]any{
			constants.MODEL_POLICY_PARAM_MODEL_HANDLE: model.Handle,
			constants.MODEL_POLICY_PARAM_MODEL_NAME:   model.ModelName,
			constants.MODEL_POLICY_PARAM_SCOPE: map[string]any{
				constants.MODEL_POLICY_PARAM_TEMPLATE_HANDLE: ec.view.InboundTemplate,
				constants.MODEL_POLICY_PARAM_PROVIDER_NAME:   models.ModelEndpointProviderName(ec.handle),
			},
			constants.MODEL_POLICY_PARAM_MODEL_LOCATION: map[string]any{
				"location":   ec.modelLocation.Location,
				"identifier": ec.modelLocation.Identifier,
			},
			constants.MODEL_POLICY_PARAM_FORMAT:  ec.profile.Format,
			constants.MODEL_POLICY_PARAM_TARGETS: targets,
		},
	})
}

// catalogPolicyInstance builds the catalog job: the Models this endpoint serves,
// which the job renders in the endpoint's format and filters by the caller's API
// key scope. The list is decided here, once per compile, from the same Models the
// resolver config is built from.
func (c *ModelGatewayCompiler) catalogPolicyInstance(profile inbound.Profile, served []servedModel) (*policyenginev1.PolicyInstance, error) {
	sorted := append([]servedModel(nil), served...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].model.ModelName < sorted[j].model.ModelName })
	entries := make([]any, 0, len(sorted))
	for _, m := range sorted {
		displayName := m.model.DisplayName
		if displayName == "" {
			displayName = m.model.ModelName
		}
		entries = append(entries, map[string]any{
			constants.MODEL_POLICY_PARAM_CATALOG_HANDLE:       m.model.Handle,
			constants.MODEL_POLICY_PARAM_CATALOG_NAME:         m.model.ModelName,
			constants.MODEL_POLICY_PARAM_CATALOG_DISPLAY_NAME: displayName,
			constants.MODEL_POLICY_PARAM_CATALOG_CREATED:      m.createdAt,
		})
	}
	inst, err := c.modelSystemPolicyInstance(map[string]any{
		constants.MODEL_POLICY_PARAM_CATALOG: map[string]any{
			constants.MODEL_POLICY_PARAM_FORMAT:         profile.Format,
			constants.MODEL_POLICY_PARAM_CATALOG_MODELS: entries,
		},
	})
	if err != nil {
		return nil, err
	}
	return &inst, nil
}

// modelSystemPolicyInstance builds one instance of the model system policy at the
// version loaded in the gateway image.
func (c *ModelGatewayCompiler) modelSystemPolicyInstance(parameters map[string]any) (policyenginev1.PolicyInstance, error) {
	resolved, err := config.ResolvePolicyVersion(c.policyDefinitions, c.latestVersions, constants.MODEL_SYSTEM_POLICY_NAME, "")
	if err != nil {
		return policyenginev1.PolicyInstance{}, fmt.Errorf("model endpoint: %w", err)
	}
	return policyenginev1.PolicyInstance{
		Name:       constants.MODEL_SYSTEM_POLICY_NAME,
		Version:    versionutil.MajorVersion(resolved),
		Enabled:    true,
		Parameters: parameters,
	}, nil
}

// costPolicy returns the per-provider cost policy, or nil when cost is not
// configured: llm-cost is not loaded, or no pricing file is set for it.
func (c *ModelGatewayCompiler) costPolicy() *api.Policy {
	if _, err := config.ResolvePolicyVersion(c.policyDefinitions, c.latestVersions, llmCostPolicyName, ""); err != nil {
		return nil
	}
	if c.systemConfig == nil {
		return nil
	}
	section, ok := c.systemConfig.PolicyConfigurations[llmCostPolicyConfigKey].(map[string]interface{})
	if !ok {
		return nil
	}
	if file, _ := section["pricing_file"].(string); strings.TrimSpace(file) == "" {
		return nil
	}
	return &api.Policy{Name: llmCostPolicyName}
}

const (
	llmCostPolicyName      = "llm-cost"
	llmCostPolicyConfigKey = "llm_cost_v1"
)

// maxRequestBodyBytes is the dispatch routes' body ceiling.
func (c *ModelGatewayCompiler) maxRequestBodyBytes() int64 {
	if n := c.modelsConfig().Endpoint.MaxRequestBodyBytes; n > 0 {
		return n
	}
	if c.systemConfig != nil {
		return int64(c.systemConfig.Router.HTTPListener.PerConnectionBufferLimitBytes)
	}
	return 0
}

// logStatusChanges logs a Model that stopped or started being served on an
// endpoint, or whose warning changed, once, rather than on every snapshot that
// recompiles the endpoint.
func (c *ModelGatewayCompiler) logStatusChanges(endpoint string, statuses []ModelCompileStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := c.lastStatus[endpoint]
	current := make(map[string]ModelCompileStatus, len(statuses))
	for _, s := range statuses {
		current[s.Handle] = s
		if prev, seen := previous[s.Handle]; seen && prev == s {
			continue
		}
		switch {
		case !s.Served:
			slog.Error("Model is not served on this model endpoint", slog.String("endpoint", endpoint),
				slog.String("model", s.Handle), slog.String("reason", s.Reason))
		case s.Warning != "":
			slog.Warn("Model is served on this model endpoint, with a warning", slog.String("endpoint", endpoint),
				slog.String("model", s.Handle), slog.String("warning", s.Warning))
		default:
			slog.Info("Model is served on this model endpoint", slog.String("endpoint", endpoint),
				slog.String("model", s.Handle))
		}
	}
	c.lastStatus[endpoint] = current
}

// modelEndpointConfig reads the endpoint out of its stored config: the rendered
// Configuration the event listener stores, or the source.
func modelEndpointConfig(cfg *models.StoredConfig) (*api.ModelEndpointConfiguration, error) {
	switch e := cfg.Configuration.(type) {
	case api.ModelEndpointConfiguration:
		return &e, nil
	case *api.ModelEndpointConfiguration:
		if e != nil {
			return e, nil
		}
	}
	switch e := cfg.SourceConfiguration.(type) {
	case api.ModelEndpointConfiguration:
		return &e, nil
	case *api.ModelEndpointConfiguration:
		if e != nil {
			return e, nil
		}
	}
	return nil, fmt.Errorf("model endpoint %q has unexpected configuration %T", cfg.Handle, cfg.Configuration)
}

// modelSpecOf reads a stored Model's spec, from its source.
func modelSpecOf(cfg *models.StoredConfig) (api.ModelConfigData, bool) {
	switch m := cfg.SourceConfiguration.(type) {
	case api.ModelConfiguration:
		return m.Spec, true
	case *api.ModelConfiguration:
		if m != nil {
			return m.Spec, true
		}
	}
	if m, ok := cfg.Configuration.(api.ModelConfiguration); ok {
		return m.Spec, true
	}
	return api.ModelConfigData{}, false
}

// ModelEndpointTransformer is the registry's transformer for ModelEndpoint.
type ModelEndpointTransformer struct {
	compiler *ModelGatewayCompiler
}

// NewModelEndpointTransformer wraps a compiler for the registry.
func NewModelEndpointTransformer(compiler *ModelGatewayCompiler) *ModelEndpointTransformer {
	return &ModelEndpointTransformer{compiler: compiler}
}

// Transform compiles one endpoint.
func (t *ModelEndpointTransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	if t == nil || t.compiler == nil {
		return emptyRuntimeConfig(cfg), nil
	}
	rdc, _, err := t.compiler.Compile(cfg)
	return rdc, err
}

// Compiler returns the wrapped compiler.
func (t *ModelEndpointTransformer) Compiler() *ModelGatewayCompiler {
	if t == nil {
		return nil
	}
	return t.compiler
}

// addUnroutedCluster adds the model endpoint's default cluster and returns its
// key. It has no endpoints: every Model chain names its target, and every other
// chain answers itself, so nothing is sent there on purpose, and anything that
// ever is fails closed with 503 instead of reaching a provider or looping back
// into the gateway.
func (c *ModelGatewayCompiler) addUnroutedCluster(rdc *models.RuntimeDeployConfig) string {
	key := "upstream_" + rdc.Metadata.Kind + "_" + rdc.Metadata.UUID + "__unrouted"
	rdc.UpstreamClusters[key] = &models.UpstreamCluster{FailClosed: true}
	return key
}

// providerClusterKey names a target provider's cluster so the policy engine
// resolves the provider handle to it: upstream_<kind>_<api id>_<name>, the same
// formula a named upstream definition uses. A provider shared by several Models
// gets one cluster.
func providerClusterKey(rdc *models.RuntimeDeployConfig, provider string) string {
	return "upstream_" + rdc.Metadata.Kind + "_" + rdc.Metadata.UUID + "_" + SanitizeUpstreamDefinitionName(provider)
}

// providerCluster builds one target provider's cluster. Everything it reads is a
// property of the provider, so any of the provider's targets builds the same one.
func (c *ModelGatewayCompiler) providerCluster(t modelgateway.Target) (*models.UpstreamCluster, error) {
	if t.Mode == modelgateway.ModeRoute {
		// Over the gateway's own listener to the provider's route, with its context
		// as the base path, and its vhost as the endpoint's host name so auto host
		// rewrite sends the Host its route is matched on.
		basePath := strings.TrimRight(t.RouteContext, "/")
		if basePath == "" {
			basePath = "/"
		}
		return &models.UpstreamCluster{
			Name:     t.ProviderHandle,
			BasePath: basePath,
			URL:      fmt.Sprintf("%s://%s:%d", constants.SchemeHTTP, constants.LocalhostIP, c.routerConfig.ListenerPort),
			Endpoints: []models.Endpoint{{
				Host:     constants.LocalhostIP,
				Port:     c.routerConfig.ListenerPort,
				Hostname: t.RouteHostname,
			}},
			TLS: &models.UpstreamTLS{Enabled: false},
		}, nil
	}

	// Direct: the provider's own upstream, resolved by the same code a provider's
	// route uses, so the host, port, base path, TLS settings and connect timeout
	// are the ones the provider was validated with. A weighted upstream definition
	// keeps every endpoint and weight.
	spec := t.Provider.Config.Spec
	scratch := &models.RuntimeDeployConfig{UpstreamClusters: make(map[string]*models.UpstreamCluster)}
	up := &api.Upstream{Url: spec.Upstream.Url, Ref: spec.Upstream.Ref}
	resolved, err := addUpstreamCluster(scratch, "main", up, spec.UpstreamDefinitions)
	if err != nil {
		return nil, err
	}
	cluster := scratch.UpstreamClusters[resolved.ClusterKey]
	if spec.Upstream.Ref != nil {
		if def := lookupUpstreamDefinition(*spec.Upstream.Ref, spec.UpstreamDefinitions); def != nil && len(def.Upstreams) > 1 {
			endpoints, err := definitionEndpoints(def)
			if err != nil {
				return nil, err
			}
			cluster.Endpoints = endpoints
		}
	}
	cluster.Name = t.ProviderHandle
	// The provider's own upstream, as its route's default upstream names it: a credential
	// that signs the request (aws-authentication) signs for this, once the route job has
	// chosen the provider.
	cluster.URL = resolved.URL
	return cluster, nil
}

// definitionEndpoints returns every endpoint of a weighted upstream definition.
func definitionEndpoints(def *api.UpstreamDefinition) ([]models.Endpoint, error) {
	out := make([]models.Endpoint, 0, len(def.Upstreams))
	for _, up := range def.Upstreams {
		parsed, err := url.Parse(up.Url)
		if err != nil {
			return nil, fmt.Errorf("upstream definition '%s': %w", def.Name, err)
		}
		ep := models.Endpoint{Host: parsed.Hostname(), Port: ResolvePort(parsed)}
		if up.Weight != nil {
			ep.Weight = up.Weight
		}
		out = append(out, ep)
	}
	return out, nil
}

// chain assembles one chain: system policies prepended to the given policies, and
// fault policies with the fault-path system policies last.
func (c *ModelGatewayCompiler) chain(
	systemProps map[string]any,
	policies []policyenginev1.PolicyInstance,
	fault []models.Policy,
	faultSystem []models.Policy,
) *models.PolicyChain {
	chain := sdkChainToModel(utils.InjectSystemPolicies(policies, c.systemConfig, systemProps))
	chain.FaultPolicies = appendFaultSystemPolicies(fault, faultSystem)
	return chain
}

func firstTargetOn(m *modelgateway.Model, provider string) *modelgateway.Target {
	for i := range m.Targets {
		if m.Targets[i].ProviderHandle == provider {
			return &m.Targets[i]
		}
	}
	return nil
}

func concat(base []policyenginev1.PolicyInstance, more ...policyenginev1.PolicyInstance) []policyenginev1.PolicyInstance {
	out := make([]policyenginev1.PolicyInstance, 0, len(base)+len(more))
	out = append(out, base...)
	return append(out, more...)
}

func mergeMaps(base, extra map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// stringSet collects secrets for redaction without duplicates, in first-seen order.
type stringSet struct {
	seen  map[string]bool
	order []string
}

func newStringSet(initial []string) *stringSet {
	s := &stringSet{seen: make(map[string]bool)}
	s.add(initial...)
	return s
}

func (s *stringSet) add(values ...string) {
	for _, v := range values {
		if v == "" || s.seen[v] {
			continue
		}
		s.seen[v] = true
		s.order = append(s.order, v)
	}
}

func (s *stringSet) values() []string {
	if len(s.order) == 0 {
		return nil
	}
	return append([]string(nil), s.order...)
}
