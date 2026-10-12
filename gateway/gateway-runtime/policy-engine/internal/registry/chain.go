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

package registry

import (
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// PolicyChain is a container for a complete policy processing pipeline for a route
type PolicyChain struct {
	// Ordered list of policies to execute (all implement Policy interface)
	Policies []policy.Policy

	// Policy specifications (aligned with Policies)
	PolicySpecs []policy.PolicySpec

	// Computed flag: true if any policy requires request body access.
	// Determines whether ext_proc uses SKIP or BUFFERED mode for request body.
	RequiresRequestBody bool

	// Computed flag: true if any policy requires response body access.
	// Determines whether ext_proc uses SKIP or BUFFERED mode for response body.
	RequiresResponseBody bool

	// Computed flag: true when every request-body policy also implements
	// StreamingRequestPolicy. When false, the kernel forces BUFFERED mode
	// for request body even if some policies support streaming.
	SupportsRequestStreaming bool

	// Computed flag: true when every response-body policy also implements
	// StreamingResponsePolicy. When true and the upstream response signals
	// streaming (Transfer-Encoding: chunked or Content-Type: text/event-stream),
	// the kernel upgrades Envoy to FULL_DUPLEX_STREAMED mode for the response body.
	// Any buffered-only policy in the chain forces this to false.
	SupportsResponseStreaming bool

	// Computed flag: true if any policy has a CEL execution condition.
	// When false, CEL evaluation is skipped entirely during execution.
	HasExecutionConditions bool

	// Computed flag: true if any policy declares RequestHeaderMode=PROCESS in Mode()
	// AND implements the RequestHeaderPolicy interface. Note: this flag does NOT
	// control Envoy header transport (headers always flow for lifecycle reasons).
	// It reflects callback participation intent.
	RequiresRequestHeader bool

	// Computed flag: true if any policy declares ResponseHeaderMode=PROCESS in Mode()
	// AND implements the ResponseHeaderPolicy interface. Note: this flag does NOT
	// control Envoy header transport (headers always flow for lifecycle reasons).
	// It reflects callback participation intent.
	RequiresResponseHeader bool

	// FaultPolicies is the API's fault policies: an ordered list of policies that
	// run ONLY on the fault path, never on a successful response. They are executed
	// over a ResponseHeaderContext describing the error, so the existing policy
	// catalogue works unchanged — no fault-specific policy interface is required.
	//
	// Kept separate from Policies rather than flagged within it, so a fault policy
	// can never accidentally execute in the normal request/response phases.
	FaultPolicies []policy.Policy

	// FaultPolicySpecs holds the specs aligned with FaultPolicies (same ordering),
	// carrying each entry's parameters and optional CEL execution condition.
	FaultPolicySpecs []policy.PolicySpec

	// Computed flag: true when FaultPolicies is non-empty. Lets the kernel skip
	// all fault-policies work — including context synthesis — for the common case
	// of an API that configures none.
	HasFaultPolicies bool

	// Computed flag: true if any FAULT policy declares a CEL execution condition.
	// Tracked separately from HasExecutionConditions so evaluating the normal chain
	// and the fault chain stay independent.
	FaultHasExecutionConditions bool
}
