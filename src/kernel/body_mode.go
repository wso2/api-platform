package kernel

import (
	"github.com/yourorg/policy-engine/worker/core"
	"github.com/yourorg/policy-engine/worker/policies"
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
		RequestPolicies:      make([]policies.RequestPolicy, 0),
		ResponsePolicies:     make([]policies.ResponsePolicy, 0),
		Metadata:             make(map[string]interface{}),
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
	}

	// Build policy lists and compute body requirements
	for _, spec := range policySpecs {
		// Get policy definition
		def, err := registry.GetDefinition(spec.Name, spec.Version)
		if err != nil {
			return nil, err
		}

		// Get policy implementation
		impl, err := registry.GetImplementation(spec.Name, spec.Version)
		if err != nil {
			return nil, err
		}

		// Add to appropriate policy list based on phase support
		if def.SupportsRequestPhase {
			if reqPolicy, ok := impl.(policies.RequestPolicy); ok {
				chain.RequestPolicies = append(chain.RequestPolicies, reqPolicy)
			}
		}

		if def.SupportsResponsePhase {
			if respPolicy, ok := impl.(policies.ResponsePolicy); ok {
				chain.ResponsePolicies = append(chain.ResponsePolicies, respPolicy)
			}
		}

		// Update body requirements (OR across all policies)
		if def.RequiresRequestBody {
			chain.RequiresRequestBody = true
		}
		if def.RequiresResponseBody {
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
func (k *Kernel) GetRequestBodyMode(routeKey string) (BodyMode, error) {
	chain, err := k.GetPolicyChainForKey(routeKey)
	if err != nil {
		return BodyModeSkip, err
	}
	return determineRequestBodyMode(chain), nil
}

// GetResponseBodyMode returns the body mode for response phase
func (k *Kernel) GetResponseBodyMode(routeKey string) (BodyMode, error) {
	chain, err := k.GetPolicyChainForKey(routeKey)
	if err != nil {
		return BodyModeSkip, err
	}
	return determineResponseBodyMode(chain), nil
}
