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
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Each phase's constructor names its OWN phase, which is the whole reason the value is
// carried in the attribution rather than read from ec.phase at fault time: ec.phase is
// maintained for getModeOverride and is not current on the bodyless paths.
func TestPolicyPhase_EachConstructorNamesItsOwnPhase(t *testing.T) {
	rejection := policy.ImmediateResponse{
		StatusCode: 401,
		IsFault:    true,
		Fault:      &policy.FaultDetails{Code: "900902", Type: policy.FaultTypeAuthentication},
	}

	t.Run("request headers", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		f, isFault := faultFromRequestHeaders(&executor.RequestHeaderExecutionResult{
			ShortCircuited: true,
			FinalAction:    rejection,
			Results: []executor.RequestHeaderPolicyResult{
				{PolicyName: "api-key-auth", PolicyVersion: "v1", Action: rejection},
			},
		})
		require.True(t, isFault, "a short-circuit with an ImmediateResponse is a fault the phase produced")
		ec.record(f.attrib)
		assert.Equal(t, "api-key-auth", ec.faultPolicyName)
		assert.Equal(t, policy.PolicyPhaseRequestHeaders, ec.faultPolicyPhase)
	})

	t.Run("request body", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		f, isFault := faultFromRequestBody(&executor.RequestExecutionResult{
			ShortCircuited: true,
			FinalAction:    rejection,
			Results: []executor.RequestPolicyResult{
				{PolicyName: "request-guardrail", PolicyVersion: "v1", Action: rejection},
			},
		})
		require.True(t, isFault, "a short-circuit with an ImmediateResponse is a fault the phase produced")
		ec.record(f.attrib)
		assert.Equal(t, "request-guardrail", ec.faultPolicyName)
		assert.Equal(t, policy.PolicyPhaseRequestBody, ec.faultPolicyPhase)
	})

	t.Run("response headers", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		f, isFault := faultFromResponseHeaders(&executor.ResponseHeaderExecutionResult{
			ShortCircuited: true,
			FinalAction:    rejection,
			Results: []executor.ResponseHeaderPolicyResult{
				{PolicyName: "header-check", PolicyVersion: "v1", Action: rejection},
			},
		})
		require.True(t, isFault, "a short-circuit with an ImmediateResponse is a fault the phase produced")
		ec.record(f.attrib)
		assert.Equal(t, "header-check", ec.faultPolicyName)
		assert.Equal(t, policy.PolicyPhaseResponseHeaders, ec.faultPolicyPhase)
	})

	t.Run("response body", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		ec.record(faultFromResponseBody(&executor.ResponseExecutionResult{
			ShortCircuited: true,
			FinalAction:    rejection,
			Results: []executor.ResponsePolicyResult{
				{PolicyName: "word-count-guardrail", PolicyVersion: "v1", Action: rejection},
			},
		}, false).attrib)
		assert.Equal(t, "word-count-guardrail", ec.faultPolicyName)
		assert.Equal(t, policy.PolicyPhaseResponseBody, ec.faultPolicyPhase)
	})
}

// A response guardrail rejects by CHANGING THE STATUS, which does not short-circuit — so it
// arrives through the statusOverridden branch rather than a rejection. It is still a
// response-body-phase policy failure and must be named as one.
func TestPolicyPhase_StatusOverrideIsStillTheResponseBodyPhase(t *testing.T) {
	status := 446
	mods := policy.DownstreamResponseModifications{
		StatusCode: &status,
		IsFault:    true,
		Fault:      &policy.FaultDetails{Code: "906000", Type: policy.FaultTypeGuardrail},
	}
	ec := faultExecCtx(t, nil, nil)
	ec.record(faultFromResponseBody(&executor.ResponseExecutionResult{
		Results: []executor.ResponsePolicyResult{
			{PolicyName: "word-count-guardrail", PolicyVersion: "v1", Action: mods},
		},
	}, true).attrib)

	assert.Equal(t, "word-count-guardrail", ec.faultPolicyName)
	assert.Equal(t, policy.PolicyPhaseResponseBody, ec.faultPolicyPhase)
}

// The invariant, enforced in record rather than trusted to five constructors: PolicyPhase
// describes the policy named beside it, so a failure with no policy has no phase.
//
// An engine failure is the case that would otherwise slip through — it reaches record with a
// DECLARED error and no policy name, so the early return does not fire and the fields are
// written. Reporting a phase there would answer "where did the gateway notice this?", which
// FaultContext deliberately does not expose.
func TestPolicyPhase_EmptyWheneverNoPolicyIsAttributed(t *testing.T) {
	t.Run("engine failure: declared, but no policy", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		ec.record(faultAttribution{
			phase:    policy.PolicyPhaseResponseBody, // a caller supplying one anyway
			declared: &policy.FaultDetails{Code: codeEngineInternal, Type: policy.FaultTypeInternal},
		})
		require.NotNil(t, ec.faultDeclared, "the declared error is still recorded")
		assert.Empty(t, ec.faultPolicyName)
		assert.Empty(t, ec.faultPolicyPhase,
			"no policy means no policy phase, whatever the caller passed")
	})

	t.Run("router failure: nothing attributed at all", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		ec.record(faultAttribution{})
		assert.Empty(t, ec.faultPolicyName)
		assert.Empty(t, ec.faultPolicyPhase)
	})

	t.Run("mid-stream: attributes no policy", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		ec.record(faultFromStream(assert.AnError, nil).attrib)
		assert.Empty(t, ec.faultPolicyName,
			"once chunks are flowing the engine cannot say which entry interrupted")
		assert.Empty(t, ec.faultPolicyPhase)
	})
}

// It has to reach the handler, not just the context: describeFault is what a fault policy
// actually reads.
func TestPolicyPhase_ReachesTheErrorContext(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.faultPolicyName = "api-key-auth"
	ec.faultPolicyVersion = "v1"
	ec.faultPolicyPhase = policy.PolicyPhaseRequestHeaders

	faultCtx := ec.newFaultErrorContext()
	ec.describeFault(faultCtx, originGateway)

	assert.Equal(t, "api-key-auth", faultCtx.Policy)
	assert.Equal(t, "v1", faultCtx.PolicyVersion)
	assert.Equal(t, policy.PolicyPhaseRequestHeaders, faultCtx.PolicyPhase)
}

// The kernel's own phase names and the SDK's must be one set of strings, or a log line and a
// fault handler describe the same phase differently.
func TestProcessingPhaseStringMatchesTheSDKConstants(t *testing.T) {
	assert.Equal(t, policy.PolicyPhaseRequestHeaders, phaseRequestHeaders.String())
	assert.Equal(t, policy.PolicyPhaseRequestBody, phaseRequestBody.String())
	assert.Equal(t, policy.PolicyPhaseResponseHeaders, phaseResponseHeaders.String())
	assert.Equal(t, policy.PolicyPhaseResponseBody, phaseResponseBody.String())
	assert.Equal(t, "unknown", processingPhase(99).String())
}
