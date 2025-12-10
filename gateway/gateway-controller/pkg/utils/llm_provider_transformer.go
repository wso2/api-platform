package utils

import (
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"gopkg.in/yaml.v3"
)

type LLMProviderTransformer struct {
	store *storage.ConfigStore
}

func NewLLMProviderTransformer(store *storage.ConfigStore) *LLMProviderTransformer {
	return &LLMProviderTransformer{store: store}
}

func (t *LLMProviderTransformer) Transform(input any, output *api.APIConfiguration) (*api.APIConfiguration, error) {
	provider, ok := input.(*api.LLMProviderConfiguration)
	if !ok {
		return nil, fmt.Errorf("invalid input type: expected *api.LLMProviderConfiguration")
	}

	// @TODO: Step 1) Retrieve the referenced template from in-memory and configure token based rate-limiting policy
	//tmpl, err := t.store.GetTemplateByName(provider.Spec.Template)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to retrieve template '%s': %w", provider.Spec.Template, err)
	//}

	output.Kind = api.APIConfigurationKindHttprest
	output.Version = api.ApiPlatformWso2Comv1

	spec := api.APIConfigData{}
	spec.Name = provider.Spec.Name
	spec.Version = provider.Spec.Version
	spec.Context = constants.BASE_PATH
	if provider.Spec.Context != nil {
		spec.Context = *provider.Spec.Context
	}
	// TODO: Map vhosts if provided in provider.Spec.Host

	// Step 2) Upstreams: map provider.Spec.Upstreams to api.Upstreams
	if len(provider.Spec.Upstreams) > 0 {
		ups := make([]api.Upstream, 0, len(provider.Spec.Upstreams))
		for _, u := range provider.Spec.Upstreams {
			ups = append(ups, api.Upstream{Url: u.Url})
		}
		spec.Upstreams = ups
	}

	// Step 3) Map upstream auth to corresponding api policy
	if len(provider.Spec.Upstreams) > 0 {
		upstream := provider.Spec.Upstreams[0]
		if upstream.Auth != nil {
			switch upstream.Auth.Type {
			case api.UpstreamWithAuthAuthTypeApiKey:
				// Add API Key auth policy at API level
				params, err := GetUpstreamAuthApikeyPolicyParams(*upstream.Auth.Header, *upstream.Auth.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to build upstream auth params: %w", err)
				}
				mh := api.Policy{
					Name:    constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME,
					Version: constants.UPSTREAM_AUTH_APIKEY_POLICY_VERSION, Params: &params}
				spec.Policies = &[]api.Policy{mh}
			default:
				return nil, fmt.Errorf("unsupported upstream auth type: %s", upstream.Auth.Type)
			}
		}
	}

	// Step 4) Apply access control
	mode := *provider.Spec.AccessControl.Mode
	var exceptions []api.RouteException
	if provider.Spec.AccessControl.Exceptions != nil {
		exceptions = *provider.Spec.AccessControl.Exceptions
	}

	var ops []api.Operation
	switch mode {
	case api.AllowAll:
		// for each exception, add an operation and attach a Response policy that returns 404
		for _, ex := range exceptions {
			// for each declared method on the exception
			for _, m := range ex.Methods {
				op := api.Operation{Method: api.OperationMethod(m), Path: ex.Path}

				// Build Respond policy params as requested
				var policyParams map[string]interface{}
				if err := yaml.Unmarshal([]byte(constants.ACCESS_CONTROL_DENY_POLICY_PARAMS), &policyParams); err != nil {
					return nil, err
				}
				pol := api.Policy{
					Name:    constants.ACCESS_CONTROL_DENY_POLICY_NAME,
					Version: constants.ACCESS_CONTROL_DENY_POLICY_VERSION, Params: &policyParams}
				op.Policies = &[]api.Policy{pol}
				ops = append(ops, op)
			}
		}

		// add catch-all operation '/'
		ops = append(ops, api.Operation{Method: constants.WILD_CARD, Path: constants.BASE_PATH + constants.WILD_CARD})

	case api.DenyAll:
		// Only include exception operations (allowed paths)
		for _, ex := range exceptions {
			for _, m := range ex.Methods {
				op := api.Operation{Method: api.OperationMethod(m), Path: ex.Path}
				ops = append(ops, op)
			}
		}

	default:
		return nil, fmt.Errorf("unsupported access control mode: %s", mode)
	}

	spec.Operations = ops

	// Step 5) Attach policies from provider.Spec.Policies to matching operations
	if provider.Spec.Policies != nil {
		// Policies are now a simple array of LLMPolicy
		for _, llmPol := range *provider.Spec.Policies {
			// For each path entry in the policy
			for _, pathEntry := range llmPol.Paths {
				// For each method in the path entry
				for _, method := range pathEntry.Methods {
					// Find matching operations (same path and either same method or wildcard '*')
					for i := range spec.Operations {
						op := &spec.Operations[i]
						if op.Path == pathEntry.Path && (string(op.Method) == string(method) || string(op.Method) == "*") {
							// Convert LLMPolicy to API Policy using path-specific params
							pol := api.Policy{
								Name:    llmPol.Name,
								Version: llmPol.Version,
								Params:  &pathEntry.Params,
							}
							if op.Policies == nil {
								op.Policies = &[]api.Policy{pol}
							} else {
								// Append to existing slice
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
