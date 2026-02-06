package utils

import (
	"fmt"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"gopkg.in/yaml.v3"
)

type LLMProviderTransformer struct {
	store                 *storage.ConfigStore
	routerConfig          *config.RouterConfig
	policyVersionResolver PolicyVersionResolver
}

// pathMethodKey represents a unique path+method combination
type pathMethodKey struct {
	path   string
	method string
}

func NewLLMProviderTransformer(store *storage.ConfigStore, routerConfig *config.RouterConfig, policyVersionResolver PolicyVersionResolver) *LLMProviderTransformer {
	return &LLMProviderTransformer{
		store:                 store,
		routerConfig:          routerConfig,
		policyVersionResolver: policyVersionResolver,
	}
}

func (t *LLMProviderTransformer) Transform(input any, output *api.APIConfiguration) (*api.APIConfiguration, error) {
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

func (t *LLMProviderTransformer) transformProxy(proxy *api.LLMProxyConfiguration,
	output *api.APIConfiguration) (*api.APIConfiguration, error) {

	// Step 1: Retrieve and validate provider reference
	provider := t.store.GetByKindAndHandle(string(api.LlmProvider), proxy.Spec.Provider)
	if provider == nil {
		return nil, fmt.Errorf("failed to retrieve provider by id '%s'", proxy.Spec.Provider)
	}

	// Step 1.5: Get provider's template and extract template params
	providerConfig, ok := provider.SourceConfiguration.(api.LLMProviderConfiguration)
	if !ok {
		return nil, fmt.Errorf("provider source configuration is not LLMProviderConfiguration")
	}

	tmpl, err := t.store.GetTemplateByHandle(providerConfig.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve template '%s' from provider: %w", providerConfig.Spec.Template, err)
	}

	templateParams, err := buildTemplateParams(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to build template params: %w", err)
	}

	// Step 2: Configure API metadata and basic spec
	output.Kind = api.RestApi
	output.ApiVersion = api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1
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
	upstream := fmt.Sprintf("%s://%s:%d%s",
		constants.SchemeHTTP, constants.LocalhostIP, t.routerConfig.ListenerPort, provider.GetContext())
	spec.Upstream.Main = api.Upstream{
		Url: &upstream,
	}
	// If provider has vhost configured add a host adding policy
	apiData, err := provider.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		return nil, fmt.Errorf("failed to get API config from provider: %w", err)
	}
	if apiData.Vhosts != nil && apiData.Vhosts.Main != "" {
		providerVhost := apiData.Vhosts.Main
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
		for _, llmPol := range *proxy.Spec.Policies {
			for _, pathEntry := range llmPol.Paths {
				// Expand wildcard methods in policy
				var policyMethods []string
				if len(pathEntry.Methods) == 1 && string(pathEntry.Methods[0]) == "*" {
					policyMethods = constants.WILDCARD_HTTP_METHODS
				} else {
					policyMethods = make([]string, len(pathEntry.Methods))
					for i, m := range pathEntry.Methods {
						policyMethods[i] = string(m)
					}
				}

				for _, policyMethod := range policyMethods {
					// Create operation if it doesn't exist (dynamic operation creation)
					key := pathMethodKey{path: pathEntry.Path, method: policyMethod}
					if _, exists := operationRegistry[key]; !exists {
						op := &api.Operation{
							Path:   pathEntry.Path,
							Method: api.OperationMethod(policyMethod),
						}
						operationRegistry[key] = op
					}

					// Attach policy to matching operations using pathsMatch helper
					pol := api.Policy{
						Name:    llmPol.Name,
						Version: llmPol.Version,
						Params:  mergeParams(pathEntry.Params, templateParams),
					}

					for opKey, op := range operationRegistry {
						// Only consider operations with matching method
						if opKey.method != policyMethod {
							continue
						}

						// Use pathsMatch to determine if policy applies to this operation
						if pathsMatch(op.Path, pathEntry.Path) {
							if op.Policies == nil {
								op.Policies = &[]api.Policy{pol}
							} else {
								existing := *op.Policies
								existing = append(existing, pol)
								op.Policies = &existing
							}
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
	spec.Operations = ops

	// Finalize output
	var specUnion api.APIConfiguration_Spec
	if err := specUnion.FromAPIConfigData(spec); err != nil {
		return nil, err
	}
	output.Spec = specUnion
	return output, nil
}

func (t *LLMProviderTransformer) transformProvider(provider *api.LLMProviderConfiguration,
	output *api.APIConfiguration) (*api.APIConfiguration, error) {
	// @TODO: Step 1) Configure token based rate-limiting policy based on template configs
	// Retrieve and validate template
	tmpl, err := t.store.GetTemplateByHandle(provider.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve template '%s': %w", provider.Spec.Template, err)
	}

	// Build template params for injection into policies
	templateParams, err := buildTemplateParams(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to build template params: %w", err)
	}

	output.Kind = api.RestApi
	output.ApiVersion = api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1
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
			spec.Policies = &[]api.Policy{mh}
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
			for _, llmPol := range *provider.Spec.Policies {
				for _, pathEntry := range llmPol.Paths {
					// Expand wildcard methods in policy
					var policyMethods []string
					if len(pathEntry.Methods) == 1 && string(pathEntry.Methods[0]) == "*" {
						policyMethods = constants.WILDCARD_HTTP_METHODS
					} else {
						policyMethods = make([]string, len(pathEntry.Methods))
						for i, m := range pathEntry.Methods {
							policyMethods[i] = string(m)
						}
					}

					for _, policyMethod := range policyMethods {
						// CRITICAL: Skip if this path+method is denied by exception
						if isDeniedByException(pathEntry.Path, policyMethod, deniedPathMethods) {
							continue // Exception deny policy takes precedence
						}

						// Check if explicit operation exists for this policy path+method
						key := pathMethodKey{path: pathEntry.Path, method: policyMethod}
						if _, exists := operationRegistry[key]; !exists {
							// Create operation for user policy
							op := &api.Operation{
								Path:   pathEntry.Path,
								Method: api.OperationMethod(policyMethod),
							}
							operationRegistry[key] = op
						}

						// Find matching operations and attach policy
						pol := api.Policy{
							Name:    llmPol.Name,
							Version: llmPol.Version,
							Params:  mergeParams(pathEntry.Params, templateParams),
						}

						for opKey, op := range operationRegistry {
							if opKey.method != policyMethod {
								continue
							}

							// Skip if this operation has deny policy (from exceptions)
							if denyPolicyVersion != "" && hasDenyPolicy(op, denyPolicyVersion) {
								continue
							}

							if pathsMatch(op.Path, pathEntry.Path) {
								if op.Policies == nil {
									op.Policies = &[]api.Policy{pol}
								} else {
									existing := *op.Policies
									existing = append(existing, pol)
									op.Policies = &existing
								}
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
			for _, llmPol := range *provider.Spec.Policies {
				for _, pathEntry := range llmPol.Paths {
					// Expand wildcard methods in policy
					var policyMethods []string
					if len(pathEntry.Methods) == 1 && string(pathEntry.Methods[0]) == "*" {
						policyMethods = constants.WILDCARD_HTTP_METHODS
					} else {
						policyMethods = make([]string, len(pathEntry.Methods))
						for i, m := range pathEntry.Methods {
							policyMethods[i] = string(m)
						}
					}

					for _, policyMethod := range policyMethods {
						// Check if this policy path+method is allowed by access control
						if isAllowedByAccessControl(pathEntry.Path, policyMethod, normalizedExceptions) {
							// Check if explicit operation exists for this policy path+method
							key := pathMethodKey{path: pathEntry.Path, method: policyMethod}
							if _, exists := operationRegistry[key]; !exists {
								// Create operation for specific policy path covered by wildcard access control
								op := &api.Operation{
									Path:   pathEntry.Path,
									Method: api.OperationMethod(policyMethod),
								}
								operationRegistry[key] = op
							}
						}

						// Find and attach policy to matching operations
						pol := api.Policy{
							Name:    llmPol.Name,
							Version: llmPol.Version,
							Params:  mergeParams(pathEntry.Params, templateParams),
						}

						for opKey, op := range operationRegistry {
							if opKey.method != policyMethod {
								continue
							}

							if pathsMatch(op.Path, pathEntry.Path) {
								if op.Policies == nil {
									op.Policies = &[]api.Policy{pol}
								} else {
									existing := *op.Policies
									existing = append(existing, pol)
									op.Policies = &existing
								}
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
	spec.Operations = ops

	// finalize output
	var specUnion api.APIConfiguration_Spec
	if err := specUnion.FromAPIConfigData(spec); err != nil {
		return nil, err
	}
	output.Spec = specUnion
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

// GetHostAdditionPolicyParams renders the policy params with given header and value
func GetHostAdditionPolicyParams(value string) (map[string]interface{}, error) {
	rendered := fmt.Sprintf(constants.PROXY_HOST__HEADER_POLICY_PARAMS, value)
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(rendered), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// buildTemplateParams extracts all template parameters from the LLM provider template
func buildTemplateParams(template *models.StoredLLMProviderTemplate) (map[string]interface{}, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	templateParams := make(map[string]interface{})

	spec := template.Configuration.Spec

	// Extract requestModel if available
	if spec.RequestModel != nil {
		requestModel := make(map[string]interface{})
		requestModel["location"] = spec.RequestModel.Location
		requestModel["identifier"] = spec.RequestModel.Identifier
		templateParams["requestModel"] = requestModel
	}

	// Extract responseModel if available
	if spec.ResponseModel != nil {
		responseModel := make(map[string]interface{})
		responseModel["location"] = spec.ResponseModel.Location
		responseModel["identifier"] = spec.ResponseModel.Identifier
		templateParams["responseModel"] = responseModel
	}

	// Extract promptTokens if available
	if spec.PromptTokens != nil {
		promptTokens := make(map[string]interface{})
		promptTokens["location"] = spec.PromptTokens.Location
		promptTokens["identifier"] = spec.PromptTokens.Identifier
		templateParams["promptTokens"] = promptTokens
	}

	// Extract completionTokens if available
	if spec.CompletionTokens != nil {
		completionTokens := make(map[string]interface{})
		completionTokens["location"] = spec.CompletionTokens.Location
		completionTokens["identifier"] = spec.CompletionTokens.Identifier
		templateParams["completionTokens"] = completionTokens
	}

	// Extract totalTokens if available
	if spec.TotalTokens != nil {
		totalTokens := make(map[string]interface{})
		totalTokens["location"] = spec.TotalTokens.Location
		totalTokens["identifier"] = spec.TotalTokens.Identifier
		templateParams["totalTokens"] = totalTokens
	}

	// Extract remainingTokens if available
	if spec.RemainingTokens != nil {
		remainingTokens := make(map[string]interface{})
		remainingTokens["location"] = spec.RemainingTokens.Location
		remainingTokens["identifier"] = spec.RemainingTokens.Identifier
		templateParams["remainingTokens"] = remainingTokens
	}

	return templateParams, nil
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
