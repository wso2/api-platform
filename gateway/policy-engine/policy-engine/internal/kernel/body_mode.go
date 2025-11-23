package kernel

import (
	"github.com/policy-engine/sdk/core"
	"github.com/policy-engine/sdk/policies"
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
func (k *Kernel) BuildPolicyChain(routeKey string, policySpecs []policies.PolicySpec, registry *core.PolicyRegistry) (*core.PolicyChain, error) {
	chain := &core.PolicyChain{
		Policies:             make([]policies.Policy, 0),
		PolicySpecs:          make([]policies.PolicySpec, 0),
		Metadata:             make(map[string]interface{}),
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
	}

	// Build policy list and compute body requirements
	for _, spec := range policySpecs {
		// Get policy implementation
		impl, err := registry.GetImplementation(spec.Name, spec.Version)
		if err != nil {
			return nil, err
		}

		// Add to policy list
		chain.Policies = append(chain.Policies, impl)
		chain.PolicySpecs = append(chain.PolicySpecs, spec)

		// Get policy mode and update body requirements
		mode := impl.Mode()

		// Update request body requirement (if any policy needs buffering)
		if mode.RequestBodyMode == policies.BodyModeBuffer || mode.RequestBodyMode == policies.BodyModeStream {
			chain.RequiresRequestBody = true
		}

		// Update response body requirement (if any policy needs buffering)
		if mode.ResponseBodyMode == policies.BodyModeBuffer || mode.ResponseBodyMode == policies.BodyModeStream {
			chain.RequiresResponseBody = true
		}
	}

	return chain, nil
}

// determineRequestBodyMode determines the body mode for request phase
// T056: Request body mode determination helper
func determineRequestBodyMode(chain *core.PolicyChain) BodyMode {
	if chain.RequiresRequestBody {
		return BodyModeBuffered
	}
	return BodyModeSkip
}

// determineResponseBodyMode determines the body mode for response phase
// T057: Response body mode determination helper
func determineResponseBodyMode(chain *core.PolicyChain) BodyMode {
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
