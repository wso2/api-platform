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
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
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
	// corresponding streaming interface (StreamingRequestBodyPolicy or StreamingResponseBodyPolicy).
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
		// Create metadata with route and API information
		metadata := policy.PolicyMetadata{
			RouteName:  routeKey,
			APIId:      apiMetadata.APIId,
			APIName:    apiMetadata.APIName,
			APIVersion: apiMetadata.APIVersion,
		}

		// Create instance using factory with metadata and params.
		// CreateInstance returns the policy and merged params (initParams + runtime params).
		impl, mergedParams, err := reg.CreateInstance(spec.Name, spec.Version, metadata, spec.Parameters.Raw)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy instance %s:%s for route %s: %w",
				spec.Name, spec.Version, routeKey, err)
		}

		// Update spec with merged params (stored for potential kernel-internal use)
		spec.Parameters.Raw = mergedParams

		// Check if policy has CEL execution condition
		if spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			chain.HasExecutionConditions = true
		}

		// Add to policy list
		chain.Policies = append(chain.Policies, impl)
		chain.PolicySpecs = append(chain.PolicySpecs, spec)

		// Discover body capabilities via type assertions (zero per-request overhead).
		// StreamingXBodyPolicy embeds XBodyPolicy, so a streaming policy satisfies both.
		_, hasReqBody := impl.(policy.RequestPolicy)
		_, hasStreamingReqBody := impl.(policy.StreamingRequestPolicy)
		_, hasRespBody := impl.(policy.ResponsePolicy)
		_, hasStreamingRespBody := impl.(policy.StreamingResponsePolicy)

		// Request body: any body-accessing policy requires delivery
		if hasReqBody || hasStreamingReqBody {
			chain.RequiresRequestBody = true
			hasRequestBodyPolicy = true
			// A buffered-only policy forces the entire chain to BUFFERED mode
			if !hasStreamingReqBody {
				chain.SupportsRequestStreaming = false
			}
		}

		// Response body: any body-accessing policy requires delivery
		if hasRespBody || hasStreamingRespBody {
			chain.RequiresResponseBody = true
			hasResponseBodyPolicy = true
			// A buffered-only policy forces the entire chain to BUFFERED mode,
			// preserving the ability to return ImmediateResponse before the client
			// sees any bytes.
			if !hasStreamingRespBody {
				chain.SupportsResponseStreaming = false
			}
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
