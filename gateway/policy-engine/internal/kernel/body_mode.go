package kernel

import (
	"fmt"

	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// BodyMode represents ext_proc body processing mode
type BodyMode int

const (
	// BodyModeSkip - skip body processing (headers only)
	BodyModeSkip BodyMode = iota
	// BodyModeBuffered - buffer entire body for processing
	BodyModeBuffered
)

// BuildPolicyChain creates a PolicyChain from PolicySpecs with body requirement computation
// T055: BuildPolicyChain with body requirement computation
func (k *Kernel) BuildPolicyChain(routeKey string, policySpecs []policy.PolicySpec, reg *registry.PolicyRegistry) (*registry.PolicyChain, error) {
	chain := &registry.PolicyChain{
		Policies:             make([]policy.Policy, 0),
		PolicySpecs:          make([]policy.PolicySpec, 0),
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
	}

	// Build policy list and compute body requirements
	for _, spec := range policySpecs {
		// Create metadata with route information
		metadata := policy.PolicyMetadata{
			RouteName: routeKey,
		}

		// Create instance using factory with metadata and params
		impl, err := reg.CreateInstance(spec.Name, spec.Version, metadata, spec.Parameters.Raw)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy instance %s:%s for route %s: %w",
				spec.Name, spec.Version, routeKey, err)
		}

		// Add to policy list
		chain.Policies = append(chain.Policies, impl)
		chain.PolicySpecs = append(chain.PolicySpecs, spec)

		// Get policy mode and update body requirements
		mode := impl.Mode()

		// Update request body requirement (if any policy needs buffering)
		if mode.RequestBodyMode == policy.BodyModeBuffer || mode.RequestBodyMode == policy.BodyModeStream {
			chain.RequiresRequestBody = true
		}

		// Update response body requirement (if any policy needs buffering)
		if mode.ResponseBodyMode == policy.BodyModeBuffer || mode.ResponseBodyMode == policy.BodyModeStream {
			chain.RequiresResponseBody = true
		}
	}

	return chain, nil
}

// determineRequestBodyMode determines the body mode for request phase
// T056: Request body mode determination helper
func determineRequestBodyMode(chain *registry.PolicyChain) BodyMode {
	if chain.RequiresRequestBody {
		return BodyModeBuffered
	}
	return BodyModeSkip
}

// determineResponseBodyMode determines the body mode for response phase
// T057: Response body mode determination helper
func determineResponseBodyMode(chain *registry.PolicyChain) BodyMode {
	if chain.RequiresResponseBody {
		return BodyModeBuffered
	}
	return BodyModeSkip
}

// GetRequestBodyMode returns the body mode for request phase
func (k *Kernel) GetRequestBodyMode(routeKey string) BodyMode {
	chain := k.GetPolicyChainForKey(routeKey)
	if chain == nil {
		return BodyModeSkip
	}
	return determineRequestBodyMode(chain)
}

// GetResponseBodyMode returns the body mode for response phase
func (k *Kernel) GetResponseBodyMode(routeKey string) BodyMode {
	chain := k.GetPolicyChainForKey(routeKey)
	if chain == nil {
		return BodyModeSkip
	}
	return determineResponseBodyMode(chain)
}
