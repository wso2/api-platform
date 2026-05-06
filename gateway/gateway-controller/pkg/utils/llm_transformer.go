package utils

import (
	"fmt"
	"sort"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"gopkg.in/yaml.v3"
)

type LLMProviderTransformer struct {
	store                 *storage.ConfigStore
	db                    storage.Storage
	routerConfig          *config.RouterConfig
	policyVersionResolver PolicyVersionResolver
}

// pathMethodKey represents a unique path+method combination
type pathMethodKey struct {
	path   string
	method string
}

type llmPolicyAttachment struct {
	policy    api.LLMPolicy
	pathEntry api.LLMPolicyPath
}

func NewLLMProviderTransformer(store *storage.ConfigStore, db storage.Storage, routerConfig *config.RouterConfig, policyVersionResolver PolicyVersionResolver) *LLMProviderTransformer {
	if db == nil {
		panic("LLMProviderTransformer requires non-nil storage")
	}

	return &LLMProviderTransformer{
		store:                 store,
		db:                    db,
		routerConfig:          routerConfig,
		policyVersionResolver: policyVersionResolver,
	}
}

// HydrateLLMConfig populates cfg.Configuration with a derived RestAPI for LlmProvider and
// LlmProxy kinds. These are stored with only SourceConfiguration set (Configuration is nil
// by design) and must be hydrated before policy derivation or xDS snapshot generation.
// For other kinds (RestApi, WebSubApi, Mcp) the function is a no-op.
func HydrateLLMConfig(cfg *models.StoredConfig, store *storage.ConfigStore, db storage.Storage, routerConfig *config.RouterConfig, policyDefinitions map[string]models.PolicyDefinition) error {
	if cfg == nil {
		return nil
	}
	if _, ok := cfg.Configuration.(api.RestAPI); ok {
		return nil
	}

	transformer := NewLLMProviderTransformer(store, db, routerConfig, NewLoadedPolicyVersionResolver(policyDefinitions))

	var restAPI api.RestAPI
	switch source := cfg.SourceConfiguration.(type) {
	case api.LLMProviderConfiguration:
		if _, err := transformer.Transform(&source, &restAPI); err != nil {
			return err
		}
	case api.LLMProxyConfiguration:
		if _, err := transformer.Transform(&source, &restAPI); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported LLM source configuration type: %T", cfg.SourceConfiguration)
	}

	cfg.Configuration = restAPI
	return nil
}

func (t *LLMProviderTransformer) Transform(input any, output *api.RestAPI) (*api.RestAPI, error) {
	switch v := input.(type) {
	case *api.LLMProviderConfiguration:
		return t.transformProvider(v, output)
	case *api.LLMProxyConfiguration:
		return t.transformProxy(v, output)
	default:
		return nil, fmt.Errorf("invalid input type: expected *api.LLMProviderConfiguration or *api.LLMProxyConfiguration")
	}
}

func (t *LLMProviderTransformer) resolvePolicyVersion(name string) (string, error) {
	if t.policyVersionResolver == nil {
		return "", &PolicyDefinitionMissingError{PolicyName: name}
	}
	return t.policyVersionResolver.Resolve(name)
}

func (t *LLMProviderTransformer) getTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	return t.db.GetLLMProviderTemplateByHandle(handle)
}

func (t *LLMProviderTransformer) transformProxy(proxy *api.LLMProxyConfiguration,
	output *api.RestAPI) (*api.RestAPI, error) {

	// Step 1: Retrieve and validate provider reference
	provider, err := t.db.GetConfigByKindAndHandle(string(api.LLMProviderConfigurationKindLlmProvider), proxy.Spec.Provider.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to look up provider '%s': %w", proxy.Spec.Provider.Id, err)
	}
	if provider == nil {
		return nil, fmt.Errorf("failed to retrieve provider by id '%s'", proxy.Spec.Provider.Id)
	}

	// Step 1.5: Get provider's template and extract template params
	providerConfig, ok := provider.SourceConfiguration.(api.LLMProviderConfiguration)
	if !ok {
		return nil, fmt.Errorf("provider source configuration is not LLMProviderConfiguration")
	}

	tmpl, err := t.getTemplateByHandle(providerConfig.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve template '%s' from provider: %w", providerConfig.Spec.Template, err)
	}

	// Step 2: Configure API metadata and basic spec
	output.Kind = api.RestAPIKindRestApi
	output.ApiVersion = api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1
	output.Metadata = proxy.Metadata

	spec := api.APIConfigData{}
	spec.DisplayName = proxy.Spec.DisplayName
	spec.Version = proxy.Spec.Version
	spec.Context = constants.BASE_PATH
	if proxy.Spec.Context != nil {
		spec.Context = *proxy.Spec.Context
	}

	// Step 3: Map the referenced local provider as an upstream in the transformed API config
	// Always use HTTP for internal loopback routing (proxy to provider) since:
	// 1. Traffic stays on localhost and never leaves the machine
	// 2. TLS adds unnecessary overhead for internal routing
	// 3. Self-signed listener certificates can cause TLS verification failures
	providerContext, err := provider.GetContext()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider context: %w", err)
	}
	upstream := fmt.Sprintf("%s://%s:%d%s",
		constants.SchemeHTTP, constants.LocalhostIP, t.routerConfig.ListenerPort, providerContext)
	spec.Upstream.Main = api.Upstream{
		Url: &upstream,
	}
	// If provider has vhost configured add a host adding policy
	if providerConfig.Spec.Vhost != nil && *providerConfig.Spec.Vhost != "" {
		providerVhost := *providerConfig.Spec.Vhost
		// Add host header adding policy at API level
		hParams, err := GetHostAdditionPolicyParams(providerVhost)
		if err != nil {
			return nil, fmt.Errorf("failed to build host addition policy params: %w", err)
		}
		policyVersion, err := t.resolvePolicyVersion(constants.PROXY_HOST__HEADER_POLICY_NAME)
		if err != nil {
			return nil, err
		}

		hh := api.Policy{
			Name:    constants.PROXY_HOST__HEADER_POLICY_NAME,
			Version: policyVersion, Params: &hParams}
		spec.Policies = &[]api.Policy{hh}

		// Update spec upstream hostRewrite to Manual
		hostRewrite := api.Manual
		spec.Upstream.Main.HostRewrite = &hostRewrite
	}

	// Set proxy-specific vhost if provided
	if proxy.Spec.Vhost != nil {
		spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: *proxy.Spec.Vhost,
		}
	}

	// Step 3.5: Apply proxy-level provider auth for proxy->provider loopback upstream
	var upstreamAuthPolicy *api.Policy
	if proxy.Spec.Provider.Auth != nil {
		auth := proxy.Spec.Provider.Auth
		switch auth.Type {
		case api.LLMUpstreamAuthTypeApiKey:
			if auth.Value == nil || *auth.Value == "" {
				return nil, fmt.Errorf("provider.auth.value is required")
			}
			header := ""
			if auth.Header != nil {
				header = *auth.Header
			}
			params, err := GetUpstreamAuthApikeyPolicyParams(header, *auth.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to build upstream auth params: %w", err)
			}
			policyVersion, err := t.resolvePolicyVersion(constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME)
			if err != nil {
				return nil, err
			}
			mh := api.Policy{
				Name:    constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME,
				Version: policyVersion,
				Params:  &params,
			}
			upstreamAuthPolicy = &mh
		default:
			return nil, fmt.Errorf("unsupported upstream auth type: %s", auth.Type)
		}
	}

	// Step 4: Build operations (AllowAll mode without exceptions)
	// This follows the same pattern as transformProvider AllowAll mode but simplified
	var ops []api.Operation

	// Phase 1: Create Catch-All Base Operations
	// In proxy mode, we always allow all requests (no access control)
	operationRegistry := make(map[pathMethodKey]*api.Operation)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := &api.Operation{
			Path:   constants.BASE_PATH + constants.WILD_CARD,
			Method: api.OperationMethod(method),
		}
		operationRegistry[pathMethodKey{path: op.Path, method: method}] = op
	}

	// Phase 2: Process User-Defined Policies
	if proxy.Spec.Policies != nil {
		registerExplicitLLMPolicyOperations(operationRegistry, *proxy.Spec.Policies, nil)

		for _, attachment := range orderedLLMPolicyAttachments(*proxy.Spec.Policies) {
			policyMethods := expandLLMPolicyMethods(attachment.pathEntry.Methods)

			for _, policyMethod := range policyMethods {
				attachedPolicyPaths := make(map[string]bool)
				methodOperations := getOperationsForMethod(operationRegistry, policyMethod)

				for _, op := range methodOperations {
					// Use pathsMatch to determine if policy applies to this operation
					if pathsMatch(op.Path, attachment.pathEntry.Path) {
						for _, targetPath := range expandPolicyTargetPaths(op.Path, &tmpl.Configuration.Spec) {
							if attachedPolicyPaths[targetPath] {
								continue
							}
							targetKey := pathMethodKey{path: targetPath, method: policyMethod}
							targetOp, exists := operationRegistry[targetKey]
							if !exists {
								targetOp = &api.Operation{
									Path:   targetPath,
									Method: api.OperationMethod(policyMethod),
								}
								operationRegistry[targetKey] = targetOp
							}

							templateParams, err := buildTemplateParams(tmpl, targetPath)
							if err != nil {
								return nil, fmt.Errorf("failed to build template params: %w", err)
							}
							pol := api.Policy{
								Name:    attachment.policy.Name,
								Version: attachment.policy.Version,
								Params:  mergeParams(attachment.pathEntry.Params, templateParams),
							}
							appendOperationPolicy(targetOp, pol)
							attachedPolicyPaths[targetPath] = true
						}
					}
				}
			}
		}
	}

	// Phase 3: Sort and Finalize Operations
	for _, op := range operationRegistry {
		ops = append(ops, *op)
	}
	ops = sortOperationsBySpecificity(ops)
	if upstreamAuthPolicy != nil {
		for i := range ops {
			if ops[i].Policies == nil {
				ops[i].Policies = &[]api.Policy{*upstreamAuthPolicy}
			} else {
				existing := *ops[i].Policies
				existing = append(existing, *upstreamAuthPolicy)
				ops[i].Policies = &existing
			}
		}
	}
	spec.Operations = ops

	output.Spec = spec
	return output, nil
}

func (t *LLMProviderTransformer) transformProvider(provider *api.LLMProviderConfiguration,
	output *api.RestAPI) (*api.RestAPI, error) {
	// @TODO: Step 1) Configure token based rate-limiting policy based on template configs
	// Retrieve and validate template
	tmpl, err := t.getTemplateByHandle(provider.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve template '%s': %w", provider.Spec.Template, err)
	}

	output.Kind = api.RestAPIKindRestApi
	output.ApiVersion = api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1
	output.Metadata = provider.Metadata

	spec := api.APIConfigData{}
	spec.DisplayName = provider.Spec.DisplayName
	spec.Version = provider.Spec.Version
	spec.Context = constants.BASE_PATH
	if provider.Spec.Context != nil {
		spec.Context = *provider.Spec.Context
	}

	// Step 2) Upstreams: map provider.Spec.Upstreams to api.Upstreams
	// Map provider upstream and vhost to API main upstream and vhost
	spec.Upstream.Main = api.Upstream{
		Url: provider.Spec.Upstream.Url,
	}
	if provider.Spec.Vhost != nil {
		spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: *provider.Spec.Vhost,
		}
	}

	// Step 3) Map upstream auth to corresponding api policy
	upstream := provider.Spec.Upstream
	var upstreamAuthPolicy *api.Policy
	if upstream.Auth != nil {
		switch upstream.Auth.Type {
		case api.LLMProviderConfigDataUpstreamAuthTypeApiKey:
			// Add API Key auth policy at API level
			params, err := GetUpstreamAuthApikeyPolicyParams(*upstream.Auth.Header, *upstream.Auth.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to build upstream auth params: %w", err)
			}
			policyVersion, err := t.resolvePolicyVersion(constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME)
			if err != nil {
				return nil, err
			}
			mh := api.Policy{
				Name:    constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME,
				Version: policyVersion, Params: &params}
			upstreamAuthPolicy = &mh
		default:
			return nil, fmt.Errorf("unsupported upstream auth type: %s", upstream.Auth.Type)
		}
	}

	// Step 4) Apply access control
	mode := provider.Spec.AccessControl.Mode
	var exceptions []api.RouteException
	if provider.Spec.AccessControl.Exceptions != nil {
		exceptions = *provider.Spec.AccessControl.Exceptions
	}

	var ops []api.Operation

	switch mode {
	case api.AllowAll:
		var denyPolicyVersion string
		if len(exceptions) > 0 {
			var err error
			denyPolicyVersion, err = t.resolvePolicyVersion(constants.ACCESS_CONTROL_DENY_POLICY_NAME)
			if err != nil {
				return nil, err
			}
		}

		// Phase 1: Create Catch-All Base Operations
		operationRegistry := make(map[pathMethodKey]*api.Operation)
		for _, method := range constants.WILDCARD_HTTP_METHODS {
			op := &api.Operation{
				Path:   constants.BASE_PATH + constants.WILD_CARD,
				Method: api.OperationMethod(method),
			}
			operationRegistry[pathMethodKey{path: op.Path, method: method}] = op
		}

		// Phase 2: Normalize and Process Access Control Exceptions (Deny List)
		deniedPathMethods := make(map[pathMethodKey]bool)

		for _, ex := range exceptions {
			var methods []string
			// Expand wildcard methods
			if len(ex.Methods) == 1 && string(ex.Methods[0]) == "*" {
				methods = constants.WILDCARD_HTTP_METHODS
			} else {
				methods = make([]string, len(ex.Methods))
				for i, m := range ex.Methods {
					methods[i] = string(m)
				}
			}

			for _, method := range methods {
				key := pathMethodKey{path: ex.Path, method: method}
				deniedPathMethods[key] = true

				// Check if operation exists
				if _, exists := operationRegistry[key]; !exists {
					// Create operation for this specific denied path
					op := &api.Operation{
						Path:   ex.Path,
						Method: api.OperationMethod(method),
					}
					operationRegistry[key] = op
				}

				// Attach deny policy to this operation
				var policyParams map[string]interface{}
				if err := yaml.Unmarshal([]byte(constants.ACCESS_CONTROL_DENY_POLICY_PARAMS), &policyParams); err != nil {
					return nil, err
				}
				denyPolicy := api.Policy{
					Name:    constants.ACCESS_CONTROL_DENY_POLICY_NAME,
					Version: denyPolicyVersion,
					Params:  &policyParams,
				}
				op := operationRegistry[key]
				if op.Policies == nil {
					op.Policies = &[]api.Policy{denyPolicy}
				} else {
					existing := *op.Policies
					existing = append(existing, denyPolicy)
					op.Policies = &existing
				}
			}
		}

		// Phase 3: Process User-Defined Policies
		if provider.Spec.Policies != nil {
			registerExplicitLLMPolicyOperations(operationRegistry, *provider.Spec.Policies, func(path, method string) bool {
				return !isDeniedByException(path, method, deniedPathMethods)
			})

			for _, attachment := range orderedLLMPolicyAttachments(*provider.Spec.Policies) {
				policyMethods := expandLLMPolicyMethods(attachment.pathEntry.Methods)

				for _, policyMethod := range policyMethods {
					attachedPolicyPaths := make(map[string]bool)
					methodOperations := getOperationsForMethod(operationRegistry, policyMethod)

					// CRITICAL: Skip if this path+method is denied by exception
					if isDeniedByException(attachment.pathEntry.Path, policyMethod, deniedPathMethods) {
						continue // Exception deny policy takes precedence
					}

					for _, op := range methodOperations {
						// Skip if this operation has deny policy (from exceptions)
						if denyPolicyVersion != "" && hasDenyPolicy(op, denyPolicyVersion) {
							continue
						}

						if pathsMatch(op.Path, attachment.pathEntry.Path) {
							for _, targetPath := range expandPolicyTargetPaths(op.Path, &tmpl.Configuration.Spec) {
								if attachedPolicyPaths[targetPath] {
									continue
								}
								targetKey := pathMethodKey{path: targetPath, method: policyMethod}
								targetOp, exists := operationRegistry[targetKey]
								if !exists {
									targetOp = &api.Operation{
										Path:   targetPath,
										Method: api.OperationMethod(policyMethod),
									}
									operationRegistry[targetKey] = targetOp
								}

								if denyAppliesToTarget(targetPath, policyMethod, denyPolicyVersion, operationRegistry) {
									continue
								}

								templateParams, err := buildTemplateParams(tmpl, targetPath)
								if err != nil {
									return nil, fmt.Errorf("failed to build template params: %w", err)
								}
								pol := api.Policy{
									Name:    attachment.policy.Name,
									Version: attachment.policy.Version,
									Params:  mergeParams(attachment.pathEntry.Params, templateParams),
								}
								appendOperationPolicy(targetOp, pol)
								attachedPolicyPaths[targetPath] = true
							}
						}
					}
				}
			}
		}

		// Phase 4: Sort and Finalize
		for _, op := range operationRegistry {
			ops = append(ops, *op)
		}

	case api.DenyAll:
		// Phase 1: Normalize Access Control - expand wildcard methods
		normalizedExceptions := make(map[pathMethodKey]bool)

		for _, ex := range exceptions {
			var methods []string
			// Expand wildcard methods
			if len(ex.Methods) == 1 && string(ex.Methods[0]) == "*" {
				methods = constants.WILDCARD_HTTP_METHODS
			} else {
				methods = make([]string, len(ex.Methods))
				for i, m := range ex.Methods {
					methods[i] = string(m)
				}
			}

			for _, method := range methods {
				normalizedExceptions[pathMethodKey{path: ex.Path, method: method}] = true
			}
		}

		// Phase 2: Build Operation Registry - create base operations
		operationRegistry := make(map[pathMethodKey]*api.Operation)
		for key := range normalizedExceptions {
			op := &api.Operation{
				Path:   key.path,
				Method: api.OperationMethod(key.method),
			}
			operationRegistry[key] = op
		}

		// Phase 3: Process Policies with Dynamic Operation Creation
		if provider.Spec.Policies != nil {
			registerExplicitLLMPolicyOperations(operationRegistry, *provider.Spec.Policies, func(path, method string) bool {
				return isAllowedByAccessControl(path, method, normalizedExceptions)
			})

			for _, attachment := range orderedLLMPolicyAttachments(*provider.Spec.Policies) {
				policyMethods := expandLLMPolicyMethods(attachment.pathEntry.Methods)

				for _, policyMethod := range policyMethods {
					attachedPolicyPaths := make(map[string]bool)
					methodOperations := getOperationsForMethod(operationRegistry, policyMethod)

					for _, op := range methodOperations {
						if pathsMatch(op.Path, attachment.pathEntry.Path) {
							for _, targetPath := range expandPolicyTargetPaths(op.Path, &tmpl.Configuration.Spec) {
								if attachedPolicyPaths[targetPath] {
									continue
								}
								targetKey := pathMethodKey{path: targetPath, method: policyMethod}
								targetOp, exists := operationRegistry[targetKey]
								if !exists {
									targetOp = &api.Operation{
										Path:   targetPath,
										Method: api.OperationMethod(policyMethod),
									}
									operationRegistry[targetKey] = targetOp
								}

								templateParams, err := buildTemplateParams(tmpl, targetPath)
								if err != nil {
									return nil, fmt.Errorf("failed to build template params: %w", err)
								}
								pol := api.Policy{
									Name:    attachment.policy.Name,
									Version: attachment.policy.Version,
									Params:  mergeParams(attachment.pathEntry.Params, templateParams),
								}
								appendOperationPolicy(targetOp, pol)
								attachedPolicyPaths[targetPath] = true
							}
						}
					}
				}
			}
		}

		// Phase 4: Sort and Finalize - convert map to sorted slice
		for _, op := range operationRegistry {
			ops = append(ops, *op)
		}

	default:
		return nil, fmt.Errorf("unsupported access control mode: %s", mode)
	}

	ops = sortOperationsBySpecificity(ops)
	if upstreamAuthPolicy != nil {
		for i := range ops {
			if ops[i].Policies == nil {
				ops[i].Policies = &[]api.Policy{*upstreamAuthPolicy}
			} else {
				existing := *ops[i].Policies
				existing = append(existing, *upstreamAuthPolicy)
				ops[i].Policies = &existing
			}
		}
	}
	spec.Operations = ops

	output.Spec = spec
	return output, nil
}

// GetUpstreamAuthApikeyPolicyParams renders the policy params with given header and value
func GetUpstreamAuthApikeyPolicyParams(header, value string) (map[string]interface{}, error) {
	rendered := fmt.Sprintf(constants.UPSTREAM_AUTH_APIKEY_POLICY_PARAMS, header, value)
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(rendered), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetHostAdditionPolicyParams renders the policy params with given host value (host-rewrite)
func GetHostAdditionPolicyParams(value string) (map[string]interface{}, error) {
	rendered := fmt.Sprintf(constants.PROXY_HOST__HEADER_POLICY_PARAMS, value)
	var m map[string]interface{}
	// For host-rewrite, params are simple mapping, so unmarshal into map
	if err := yaml.Unmarshal([]byte(rendered), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// buildTemplateParams extracts template parameters from the LLM provider template for the given resource path
func buildTemplateParams(template *models.StoredLLMProviderTemplate, resourcePath string) (map[string]interface{}, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	templateParams := make(map[string]interface{})

	spec := template.Configuration.Spec
	applyExtractionFieldsFromBaseSpec(templateParams, &spec)

	selectedMapping := selectTemplateResourceMapping(spec.ResourceMappings, resourcePath)
	if selectedMapping != nil {
		applyExtractionFieldsFromMapping(templateParams, selectedMapping)
	}

	return templateParams, nil
}

func applyExtractionFieldsFromBaseSpec(templateParams map[string]interface{}, spec *api.LLMProviderTemplateData) {
	if spec == nil {
		return
	}
	setExtractionParam(templateParams, "requestModel", spec.RequestModel)
	setExtractionParam(templateParams, "responseModel", spec.ResponseModel)
	setExtractionParam(templateParams, "promptTokens", spec.PromptTokens)
	setExtractionParam(templateParams, "completionTokens", spec.CompletionTokens)
	setExtractionParam(templateParams, "totalTokens", spec.TotalTokens)
	setExtractionParam(templateParams, "remainingTokens", spec.RemainingTokens)
}

func applyExtractionFieldsFromMapping(templateParams map[string]interface{}, mapping *api.LLMProviderTemplateResourceMapping) {
	if mapping == nil {
		return
	}
	setExtractionParam(templateParams, "requestModel", mapping.RequestModel)
	setExtractionParam(templateParams, "responseModel", mapping.ResponseModel)
	setExtractionParam(templateParams, "promptTokens", mapping.PromptTokens)
	setExtractionParam(templateParams, "completionTokens", mapping.CompletionTokens)
	setExtractionParam(templateParams, "totalTokens", mapping.TotalTokens)
	setExtractionParam(templateParams, "remainingTokens", mapping.RemainingTokens)
}

func setExtractionParam(templateParams map[string]interface{}, key string, identifier *api.ExtractionIdentifier) {
	if identifier == nil {
		return
	}
	templateParams[key] = map[string]interface{}{
		"location":   identifier.Location,
		"identifier": identifier.Identifier,
	}
}

func selectTemplateResourceMapping(mappings *api.LLMProviderTemplateResourceMappings,
	resourcePath string) *api.LLMProviderTemplateResourceMapping {
	if mappings == nil {
		return nil
	}

	var selected *api.LLMProviderTemplateResourceMapping
	if mappings.Resources == nil {
		return selected
	}

	for i := range *mappings.Resources {
		candidate := &(*mappings.Resources)[i]
		candidateResource := candidate.Resource
		if !pathsMatch(resourcePath, candidateResource) {
			continue
		}

		if selected == nil {
			selected = candidate
			continue
		}

		selectedResource := selected.Resource
		if shouldPreferTemplateResourceMapping(candidateResource, selectedResource) {
			selected = candidate
		}
	}

	return selected
}

func shouldPreferTemplateResourceMapping(candidatePath, selectedPath string) bool {
	candidateHasWildcard := strings.Contains(candidatePath, "*")
	selectedHasWildcard := strings.Contains(selectedPath, "*")

	if !candidateHasWildcard && selectedHasWildcard {
		return true
	}
	if candidateHasWildcard && !selectedHasWildcard {
		return false
	}

	return len(candidatePath) > len(selectedPath)
}

// mergeParams merges base parameters with additional parameters (deep copy to avoid mutation)
func mergeParams(base map[string]interface{}, extra map[string]interface{}) *map[string]interface{} {
	merged := make(map[string]interface{}, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return &merged
}

func appendOperationPolicy(op *api.Operation, pol api.Policy) {
	if op.Policies == nil {
		op.Policies = &[]api.Policy{pol}
		return
	}
	existing := *op.Policies
	existing = append(existing, pol)
	op.Policies = &existing
}

func orderedLLMPolicyAttachments(policies []api.LLMPolicy) []llmPolicyAttachment {
	attachments := make([]llmPolicyAttachment, 0)
	for _, llmPol := range policies {
		for _, pathEntry := range llmPol.Paths {
			attachments = append(attachments, llmPolicyAttachment{
				policy:    llmPol,
				pathEntry: pathEntry,
			})
		}
	}

	sort.SliceStable(attachments, func(i, j int) bool {
		return shouldAttachPathBefore(attachments[i].pathEntry.Path, attachments[j].pathEntry.Path)
	})

	return attachments
}

func shouldAttachPathBefore(leftPath, rightPath string) bool {
	leftHasWildcard := strings.Contains(leftPath, constants.WILD_CARD)
	rightHasWildcard := strings.Contains(rightPath, constants.WILD_CARD)

	if leftHasWildcard != rightHasWildcard {
		return leftHasWildcard
	}

	if len(leftPath) != len(rightPath) {
		return len(leftPath) < len(rightPath)
	}

	return leftPath < rightPath
}

// ensureOperation checks if an operation for the given path+method exists in the registry, and creates it if not
func ensureOperation(operationRegistry map[pathMethodKey]*api.Operation, path, method string) *api.Operation {
	key := pathMethodKey{path: path, method: method}
	if op, exists := operationRegistry[key]; exists {
		return op
	}

	op := &api.Operation{
		Path:   path,
		Method: api.OperationMethod(method),
	}
	operationRegistry[key] = op
	return op
}

// expandLLMPolicyMethods takes the methods defined in an LLM policy and expands them to actual HTTP methods if wildcard is used
func expandLLMPolicyMethods(methods []api.LLMPolicyPathMethods) []string {
	if len(methods) == 1 && string(methods[0]) == constants.WILD_CARD {
		return append([]string(nil), constants.WILDCARD_HTTP_METHODS...)
	}

	expanded := make([]string, len(methods))
	for i, method := range methods {
		expanded[i] = string(method)
	}
	return expanded
}

// registerExplicitLLMPolicyOperations iterates through the explicitly defined policies in the LLM policy and ensures that operations
// exist for their paths and methods in the operation registry. The shouldRegister callback allows conditional registration based on
// path and method (e.g., to skip paths/methods denied by access control exceptions).
func registerExplicitLLMPolicyOperations(operationRegistry map[pathMethodKey]*api.Operation, policies []api.LLMPolicy,
	shouldRegister func(path, method string) bool) {
	for _, llmPol := range policies {
		for _, pathEntry := range llmPol.Paths {
			for _, method := range expandLLMPolicyMethods(pathEntry.Methods) {
				if shouldRegister != nil && !shouldRegister(pathEntry.Path, method) {
					continue
				}
				ensureOperation(operationRegistry, pathEntry.Path, method)
			}
		}
	}
}

func getOperationsForMethod(operationRegistry map[pathMethodKey]*api.Operation, method string) []*api.Operation {
	ops := make([]*api.Operation, 0)
	for key, op := range operationRegistry {
		if key.method == method {
			ops = append(ops, op)
		}
	}
	return ops
}

func expandPolicyTargetPaths(opPath string, templateSpec *api.LLMProviderTemplateData) []string {
	if !strings.Contains(opPath, constants.WILD_CARD) {
		return []string{opPath}
	}
	if templateSpec == nil || templateSpec.ResourceMappings == nil || templateSpec.ResourceMappings.Resources == nil {
		return []string{opPath}
	}

	seen := make(map[string]bool)
	expanded := make([]string, 0)
	seen[opPath] = true
	for i := range *templateSpec.ResourceMappings.Resources {
		resource := (*templateSpec.ResourceMappings.Resources)[i].Resource
		candidatePath := resource
		if !pathsMatch(candidatePath, opPath) {
			continue
		}

		selected := selectTemplateResourceMapping(templateSpec.ResourceMappings, candidatePath)
		if selected == nil {
			continue
		}

		selectedPath := selected.Resource
		if !pathsMatch(selectedPath, opPath) || seen[selectedPath] {
			continue
		}
		seen[selectedPath] = true
		expanded = append(expanded, selectedPath)
	}

	if len(expanded) == 0 {
		return []string{opPath}
	}
	expanded = append(expanded, opPath)
	return expanded
}

// isDeniedByException checks if a policy path+method is denied by access control exceptions in AllowAll mode
func isDeniedByException(policyPath, policyMethod string, deniedPathMethods map[pathMethodKey]bool) bool {
	// Check exact match first
	key := pathMethodKey{path: policyPath, method: policyMethod}
	if deniedPathMethods[key] {
		return true
	}

	// Check if any denied wildcard exception covers this policy path+method
	for deniedKey := range deniedPathMethods {
		if deniedKey.method != policyMethod {
			continue
		}

		// Check if deniedPath is wildcard that covers policyPath
		// Example: deniedPath="chat/*" covers policyPath="chat/completions"
		if strings.Contains(deniedKey.path, "*") {
			prefix := deniedKey.path[:strings.LastIndex(deniedKey.path, "*")]
			if strings.HasPrefix(policyPath, prefix) {
				return true
			}
		}
	}

	return false
}

// hasDenyPolicy checks if an operation has a deny policy attached (from access control exceptions)
func hasDenyPolicy(op *api.Operation, denyPolicyVersion string) bool {
	if op.Policies == nil {
		return false
	}
	if denyPolicyVersion == "" {
		return false
	}
	for _, pol := range *op.Policies {
		if pol.Name == constants.ACCESS_CONTROL_DENY_POLICY_NAME &&
			pol.Version == denyPolicyVersion {
			return true
		}
	}
	return false
}

// denyAppliesToTarget checks if a deny policy applies to a target path for a specific method.
// It considers both exact deny paths and wildcard deny paths present in the operation registry.
func denyAppliesToTarget(targetPath, policyMethod, denyPolicyVersion string,
	operationRegistry map[pathMethodKey]*api.Operation) bool {
	if denyPolicyVersion == "" {
		return false
	}

	for key, op := range operationRegistry {
		if key.method != policyMethod {
			continue
		}
		if !hasDenyPolicy(op, denyPolicyVersion) {
			continue
		}
		if pathsMatch(targetPath, key.path) {
			return true
		}
	}

	return false
}

// isAllowedByAccessControl checks if a policy path+method is allowed by access control exceptions
func isAllowedByAccessControl(policyPath, policyMethod string, normalizedExceptions map[pathMethodKey]bool) bool {
	// Check each exception to see if it allows this policy path+method
	for key := range normalizedExceptions {
		if key.method != policyMethod {
			continue
		}

		// Case 1: Exact match
		if key.path == policyPath {
			return true
		}

		// Case 2: Exception path is wildcard that covers policy path
		// Example: exceptionPath="chat/*" covers policyPath="chat/completions"
		if strings.Contains(key.path, "*") {
			prefix := key.path[:strings.LastIndex(key.path, "*")]
			if strings.HasPrefix(policyPath, prefix) {
				return true
			}
		}
	}

	return false
}

// pathsMatch checks if an operation path matches a policy path for policy attachment
func pathsMatch(opPath, policyPath string) bool {
	// Case 0: policyPath is root (covers any operation path)
	if policyPath == constants.BASE_PATH+constants.WILD_CARD {
		return true
	}
	// Case 1: Exact match (including same wildcard)
	if opPath == policyPath {
		return true
	}

	// Case 2: Policy has wildcard and operation path starts with policy prefix
	// Example: policyPath="chat/*", opPath="chat/completions" or "chat/completions/stream"
	if strings.Contains(policyPath, "*") {
		prefix := policyPath[:strings.LastIndex(policyPath, "*")]
		if strings.HasPrefix(opPath, prefix) {
			return true
		}
	}

	// Note: We do NOT match if operation is wildcard and policy is specific
	// (e.g., opPath="chat/*", policyPath="chat/completions")
	// This would incorrectly apply specific policies to catch-all routes

	return false
}

// sortOperationsBySpecificity sorts operations with most specific paths first
func sortOperationsBySpecificity(ops []api.Operation) []api.Operation {
	// Sort by:
	// 1. Non-wildcard paths before wildcard paths
	// 2. Longer paths before shorter paths
	// 3. Path string lexicographically
	// 4. Method alphabetically
	sorted := make([]api.Operation, len(ops))
	copy(sorted, ops)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if shouldSwap(sorted[i], sorted[j]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// shouldSwap determines if two operations should be swapped in sorting
func shouldSwap(op1, op2 api.Operation) bool {
	path1HasWildcard := strings.Contains(op1.Path, "*")
	path2HasWildcard := strings.Contains(op2.Path, "*")

	// Non-wildcard paths come before wildcard paths
	if !path1HasWildcard && path2HasWildcard {
		return false
	}
	if path1HasWildcard && !path2HasWildcard {
		return true
	}

	// Longer paths come before shorter paths
	if len(op1.Path) != len(op2.Path) {
		return len(op1.Path) < len(op2.Path)
	}

	// Lexicographic comparison for paths
	if op1.Path != op2.Path {
		return op1.Path > op2.Path
	}

	// Method alphabetically
	return string(op1.Method) > string(op2.Method)
}
