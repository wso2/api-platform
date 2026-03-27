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

package kernel

import (
	"fmt"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// BodyMode represents ext_proc body processing mode
type BodyMode int

const (
	// BodyModeSkip skips body processing entirely — only headers are exchanged with ext_proc.
	BodyModeSkip BodyMode = iota

	// BodyModeBuffered accumulates the entire body in memory before invoking policies.
	// Required when any policy in the chain needs the complete payload at once.
	BodyModeBuffered

	// BodyModeStreamed delivers the body as individual chunks via FULL_DUPLEX_STREAMED mode.
	// Used only when every body-processing policy in the chain implements the
	// corresponding streaming interface (StreamingRequestPolicy or StreamingResponsePolicy).
	BodyModeStreamed
)

// BuildPolicyChain creates a PolicyChain from PolicySpecs with body requirement computation.
// Capabilities are discovered at chain-build time using type assertions — once, at startup,
// with zero per-request overhead.
func (k *Kernel) BuildPolicyChain(routeKey string, policySpecs []policy.PolicySpec, reg *registry.PolicyRegistry, apiMetadata policy.PolicyMetadata) (*registry.PolicyChain, error) {
	chain := &registry.PolicyChain{
		Policies:             make([]policy.Policy, 0),
		PolicySpecs:          make([]policy.PolicySpec, 0),
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
		HasExecutionConditions: false,
		// Optimistically assume full streaming support; flipped to false if any
		// body-processing policy does not implement the streaming interface.
		SupportsRequestStreaming:  true,
		SupportsResponseStreaming: true,
	}

	// Track whether any policy actually needs body access, to avoid incorrectly
	// setting SupportsXStreaming when no body policies exist at all.
	hasRequestBodyPolicy := false
	hasResponseBodyPolicy := false

	// Build policy list and compute body requirements via type assertions
	for _, spec := range policySpecs {
		metadata := policy.PolicyMetadata{
			RouteName:  routeKey,
			APIId:      apiMetadata.APIId,
			APIName:    apiMetadata.APIName,
			APIVersion: apiMetadata.APIVersion,
		}

		impl, mergedParams, err := reg.GetInstance(spec.Name, spec.Version, metadata, spec.Parameters.Raw)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy instance %s:%s for route %s: %w",
				spec.Name, spec.Version, routeKey, err)
		}

		spec.Parameters.Raw = mergedParams

		if spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			chain.HasExecutionConditions = true
		}

		chain.Policies = append(chain.Policies, impl)
		chain.PolicySpecs = append(chain.PolicySpecs, spec)

		// Get policy mode and update body requirements
		mode := impl.Mode()
		if mode.RequestBodyMode == policy.BodyModeBuffer || mode.RequestBodyMode == policy.BodyModeStream {
			chain.RequiresRequestBody = true
			hasRequestBodyPolicy = true
			// A buffered-only policy (no StreamingRequestPolicy) forces the
			// entire chain to BUFFERED mode.
			if _, streaming := impl.(policy.StreamingRequestPolicy); !streaming {
				chain.SupportsRequestStreaming = false
			}
		}
		if mode.ResponseBodyMode == policy.BodyModeBuffer || mode.ResponseBodyMode == policy.BodyModeStream {
			chain.RequiresResponseBody = true
			hasResponseBodyPolicy = true
			// A buffered-only policy forces the entire chain to BUFFERED mode,
			// preserving the ability to return ImmediateResponse before the
			// client sees any bytes.
			if _, streaming := impl.(policy.StreamingResponsePolicy); !streaming {
				chain.SupportsResponseStreaming = false
			}
		}

		// Discover header capabilities via type assertions.
		if _, ok := impl.(policy.RequestHeaderPolicy); ok {
			chain.RequiresRequestHeader = true
		}
		if _, ok := impl.(policy.ResponseHeaderPolicy); ok {
			chain.RequiresResponseHeader = true
		}
	}

	// Clear streaming flags when no body policies exist — there is nothing to stream
	if !hasRequestBodyPolicy {
		chain.SupportsRequestStreaming = false
	}
	if !hasResponseBodyPolicy {
		chain.SupportsResponseStreaming = false
	}

	return chain, nil
}

// determineRequestBodyMode determines the body mode for request phase.
func determineRequestBodyMode(chain *registry.PolicyChain) BodyMode {
	if !chain.RequiresRequestBody {
		return BodyModeSkip
	}
	if chain.SupportsRequestStreaming {
		return BodyModeStreamed
	}
	return BodyModeBuffered
}

// determineResponseBodyMode determines the body mode for response phase.
// Returns BodyModeStreamed only when all response-body policies support streaming;
// falls back to BodyModeBuffered if any policy requires the full payload.
func determineResponseBodyMode(chain *registry.PolicyChain) BodyMode {
	if !chain.RequiresResponseBody {
		return BodyModeSkip
	}
	if chain.SupportsResponseStreaming {
		return BodyModeStreamed
	}
	return BodyModeBuffered
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
