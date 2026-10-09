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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// streamingCtx is an execution context mid-stream: the status and headers have gone out.
func streamingCtx(t *testing.T, policies []policy.Policy, specs []policy.PolicySpec) *PolicyExecutionContext {
	t.Helper()
	ec := faultExecCtx(t, policies, specs)
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}}),
		ResponseStatus:  200,
	}
	ec.responseStreamContext = &policy.ResponseStreamContext{
		SharedContext:   ec.sharedCtx,
		ResponseHeaders: ec.responseHeaderCtx.ResponseHeaders,
	}
	return ec
}

// The gap this closes: a mid-stream failure used to fire nothing at all, so a guardrail
// intervening on a streamed response was the one rejection an operator never heard about.
func TestFaultStreaming_SideEffectsStillFire(t *testing.T) {
	var order []string
	var saw policy.FaultContext
	ec := streamingCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &saw},
	}, specs(1))

	ec.runFaultPoliciesOnStreamError(context.Background(), errors.New("guardrail tripped mid-stream"), nil)

	require.Equal(t, []string{"notify"}, order, "a mid-stream failure must reach fault policies")
	assert.True(t, saw.ResponseCommitted,
		"a mid-stream fault is identified by the response already being committed, not by a phase name")
}

// The handler must be TOLD it cannot change anything. Without this a policy author writes a
// reshaper that compiles, deploys, runs, and silently does nothing.
func TestFaultStreaming_ContextSaysTheResponseIsCommitted(t *testing.T) {
	var saw policy.FaultContext
	ec := streamingCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: new([]string), sawErrCtx: &saw},
	}, specs(1))

	ec.runFaultPoliciesOnStreamError(context.Background(), errors.New("boom"), nil)

	assert.True(t, saw.ResponseCommitted,
		"a mid-stream handler must know the client already has the response")
}

// And the buffered paths must NOT claim that, or every handler would think it is powerless.
func TestFaultStreaming_BufferedPathIsNotCommitted(t *testing.T) {
	var saw policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: new([]string), sawErrCtx: &saw},
	}, specs(1))

	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 401})

	assert.False(t, saw.ResponseCommitted,
		"a buffered error CAN still be changed, so this must stay false")
}

// A policy's declared error reaches the handler, so a notification can say what tripped.
func TestFaultStreaming_DeclaredErrorReachesTheHandler(t *testing.T) {
	var saw policy.FaultContext
	ec := streamingCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: new([]string), sawErrCtx: &saw},
	}, specs(1))
	ec.faultPolicyName = "word-count-guardrail"

	ec.runFaultPoliciesOnStreamError(context.Background(), nil,
		&policy.FaultDetails{Code: "906000", Type: "guardrail", Message: "too many words"})

	require.NotNil(t, saw.Fault)
	assert.Equal(t, "906000", saw.Fault.Code)
	assert.Equal(t, "word-count-guardrail", saw.Fault.Policy)
}

// Run-once still holds: a mid-stream failure must not let a later trigger fire again.
func TestFaultStreaming_RunsAtMostOnce(t *testing.T) {
	var order []string
	ec := streamingCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order},
	}, specs(1))

	ec.runFaultPoliciesOnStreamError(context.Background(), errors.New("first"), nil)
	ec.runFaultPoliciesOnStreamError(context.Background(), errors.New("second"), nil)
	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 500})

	assert.Equal(t, []string{"notify"}, order, "the fault chain must run exactly once per request")
}

// No chain configured means no work, as everywhere else.
func TestFaultStreaming_NoChainIsANoOp(t *testing.T) {
	ec := streamingCtx(t, nil, nil)
	ec.runFaultPoliciesOnStreamError(context.Background(), errors.New("boom"), nil)
	assert.False(t, ec.faultPoliciesRan)
}

// Termination is ambiguous by design — the same action ends a stream for a guardrail
// intervention and for a clean close after the upstream's final event. Undeclared must mean
// "not a fault": guessing otherwise would notify on every successful stream.
func TestFaultStreaming_TerminationIsAFaultOnlyWhenDeclared(t *testing.T) {
	cases := []struct {
		name      string
		action    policy.StreamingResponseAction
		wantFault bool
	}{
		{"clean close, nothing declared", policy.TerminateResponseChunk{Body: []byte("data: [DONE]\n\n")}, false},
		{"guardrail declares a fault", policy.TerminateResponseChunk{IsFault: true}, true},
		{"explicitly not a fault", policy.TerminateResponseChunk{IsFault: false}, false},
		{"a clean close that still carries a description", policy.TerminateResponseChunk{
			Body: []byte("data: [DONE]\n\n"), Fault: &policy.FaultDetails{Code: "906000"}}, false},
		{"describing an error does NOT imply one", policy.TerminateResponseChunk{
			Fault: &policy.FaultDetails{Code: "906000"}}, false},
		{"declared, with a description", policy.TerminateResponseChunk{
			IsFault: true, Fault: &policy.FaultDetails{Code: "906000"}}, true},
		{"a forwarded chunk is never a fault", policy.ForwardResponseChunk{}, false},
		{"nil action", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := streamTerminationIsFault(tc.action)
			assert.Equal(t, tc.wantFault, got)
		})
	}
}

// The declared error travels with the termination, so a handler can report the cause.
func TestFaultStreaming_TerminationCarriesItsError(t *testing.T) {
	_, declared := streamTerminationIsFault(policy.TerminateResponseChunk{
		IsFault: true, Fault: &policy.FaultDetails{Code: "906000", Message: "blocked"},
	})
	require.NotNil(t, declared)
	assert.Equal(t, "906000", declared.Code)
}

// Formatting must not run: a body already went to the client, so there is nothing to format
// and a second trigger must not try either.
func TestFaultStreaming_MarksTheBodyAuthoredSoFormattingCannotRun(t *testing.T) {
	ec := streamingCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: new([]string)},
	}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindMCP // would otherwise be formatted

	ec.runFaultPoliciesOnStreamError(context.Background(), errors.New("boom"), nil)

	assert.True(t, ec.faultBodyAuthored,
		"mid-stream the body is already the client's, so formatting must be suppressed")
}
