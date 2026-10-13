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

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A phase that rejected nothing must report no fault, so handleFault is never entered on a
// successful request.
//
// This is a READABILITY invariant as much as a behavioural one, which is why it needs its own
// test. Returning true unconditionally would still be correct — handleFault's internal guards
// would find nothing to do and return — so nothing else in this suite would fail. What would
// break is the property the guard exists for: that an unguarded ec.handleFault in a phase
// handler means "this IS a failure", and processRequestHeaders no longer reads as though every
// request were one.
func TestFaultConstructors_ReportNothingWhenThePhaseRejectedNothing(t *testing.T) {
	t.Run("request headers", func(t *testing.T) {
		_, isFault := faultFromRequestHeaders(&executor.RequestHeaderExecutionResult{
			Results: []executor.RequestHeaderPolicyResult{
				{PolicyName: "set-headers", Action: policy.UpstreamRequestHeaderModifications{}},
			},
		})
		assert.False(t, isFault, "a chain that only mutated headers produced no failure")
	})

	t.Run("request body", func(t *testing.T) {
		_, isFault := faultFromRequestBody(&executor.RequestExecutionResult{
			Results: []executor.RequestPolicyResult{
				{PolicyName: "prompt-decorator", Action: policy.UpstreamRequestModifications{}},
			},
		})
		assert.False(t, isFault)
	})

	t.Run("response headers", func(t *testing.T) {
		_, isFault := faultFromResponseHeaders(&executor.ResponseHeaderExecutionResult{
			Results: []executor.ResponseHeaderPolicyResult{
				{PolicyName: "remove-headers", Action: policy.DownstreamResponseHeaderModifications{}},
			},
		})
		assert.False(t, isFault)
	})
}

// A short-circuit that is not an ImmediateResponse is also nothing to act on.
//
// It cannot happen through the public contract — the header phases have only two action shapes
// — but the type assertion has to fail closed rather than hand handleFault a fault with a nil
// rejection, which would take the outgoing branch and misreport a request-phase rejection as a
// pass-through response error.
func TestFaultConstructors_ReportNothingForAShortCircuitWithNoImmediateResponse(t *testing.T) {
	_, isFault := faultFromRequestHeaders(&executor.RequestHeaderExecutionResult{
		ShortCircuited: true,
		FinalAction:    policy.UpstreamRequestHeaderModifications{},
	})
	assert.False(t, isFault,
		"a short-circuit carrying no response is not a failure this flow can act on")
}

// And the positive half, so the guard cannot be satisfied by returning false always.
func TestFaultConstructors_ReportAFaultWhenAPolicyRejected(t *testing.T) {
	rejection := policy.ImmediateResponse{StatusCode: 401, IsFault: true}

	f, isFault := faultFromRequestHeaders(&executor.RequestHeaderExecutionResult{
		ShortCircuited: true,
		FinalAction:    rejection,
		Results: []executor.RequestHeaderPolicyResult{
			{PolicyName: "jwt-auth", PolicyVersion: "v1", Action: rejection},
		},
	})

	assert.True(t, isFault)
	assert.NotNil(t, f.rejection, "the rejection must travel with the report")
	assert.NotNil(t, f.writeBack, "the fault chain must be able to replace it")
	assert.Equal(t, "jwt-auth", f.attrib.policyName)
}
