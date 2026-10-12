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
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
)

// An error status wins over a declared non-fault: the gate is status-OR-declaration, so a
// policy cannot hold a 4xx out of the flow.
//
// This asserted the opposite while the declaration was the whole gate, and the reversal is
// the point of the current rule. A `respond` policy answering a configured 404, or an auth
// challenge that is a required handshake step, now reaches the chain — narrowed with an
// execution condition on the entry rather than by the policy, which is the same place a
// deployment already narrows backend errors.
func TestFaultPolicies_DeclaredFault_AnErrorStatusWinsOverADeclaredNonFault(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

	out := ec.handleRejection(t, policy.ImmediateResponse{
		StatusCode: 404,
		IsFault:    false,
	})

	assert.Equal(t, []string{"notify"}, order,
		"a 404 is a fault whoever set it, and IsFault: false cannot suppress it")
	assert.Equal(t, 404, out.StatusCode, "and the status the policy chose survives")
	assert.True(t, ec.faultPoliciesRan, "the chain ran, so the run-once flag is consumed")
}

// The mirror case: a status that looks fine but the policy knows is a failure.
func TestFaultPolicies_DeclaredFault_FiresOnASuccessStatus(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

	ec.handleRejection(t, policy.ImmediateResponse{
		StatusCode: 200,
		IsFault:    true,
	})

	assert.Equal(t, []string{"notify"}, order,
		"a declared fault must fire even at a 2xx — only the policy knows this failed")
}

// A rejection that declares nothing does NOT enter the fault flow, at any status — including
// a 503, which the old status rule would have caught.
//
// This is the deliberate cost of making the field the whole test. An unmigrated policy's
// genuine rejection is invisible to the fault flow until that policy sets IsFault or
// describes a Fault. The alternative default would route every unmigrated cache hit and
// preflight INTO the flow, which is not a missing feature but the wrong behaviour, and would
// land on deployments that configured no fault policies at all.
// The gate in both directions, across the boundary. An undeclared rejection is a fault from
// 400 up and not below it, which is what lets a policy predating the contract have its
// genuine rejections handled without a migration.
//
// 399 and 400 are both here on purpose: the boundary is the rule, and an off-by-one in it
// would be invisible to a test that only checked a 200 and a 500.
func TestFaultPolicies_DeclaredFault_UndeclaredIsAFaultFrom400Up(t *testing.T) {
	for _, tc := range []struct {
		status int
		fault  bool
	}{
		{200, false},
		{399, false},
		{400, true},
		{422, true},
		{503, true},
	} {
		var order []string
		ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

		ec.handleRejection(t, policy.ImmediateResponse{StatusCode: tc.status})

		if tc.fault {
			assert.Equal(t, []string{"notify"}, order,
				"status %d: at or above 400 is a fault with nothing to declare", tc.status)
			assert.True(t, ec.faultPoliciesRan, "status %d: the chain ran", tc.status)
		} else {
			assert.Empty(t, order,
				"status %d: below 400 an undeclared rejection is not a fault", tc.status)
			assert.False(t, ec.faultPoliciesRan,
				"status %d: declining must not consume the run-once flag", tc.status)
		}
	}
}

// Describing the failure is NOT opting in. Fault and IsFault answer different questions —
// Fault shapes the body a renderer produces, IsFault routes the response through the fault
// flow — and both combinations are real, so neither implies the other.
//
// The case that forces them apart: a policy returning a deliberate, non-failing response that
// should still look like an error to the client — a canned 404, an auth challenge, a cache
// miss. It needs Fault for the body and must stay out of the fault flow. If Fault implied a
// fault, that response could not be expressed at all.
func TestFaultPolicies_DeclaredFault_DescribingTheFaultDoesNotOptIn(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

	ec.handleRejection(t, policy.ImmediateResponse{
		StatusCode: 302,
		Fault:      &policy.FaultDetails{Code: "900901", Type: "authentication"},
	})

	assert.Empty(t, order,
		"below 400 a Fault alone must not opt the rejection in — IsFault is the opt-in there")

	// And with the declaration, the same sub-400 rejection runs the chain.
	ec = faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))
	ec.handleRejection(t, policy.ImmediateResponse{
		StatusCode: 302,
		IsFault:    true,
		Fault:      &policy.FaultDetails{Code: "900901", Type: "authentication"},
	})
	assert.Equal(t, []string{"notify"}, order, "declared — the chain runs")
}

// The run-once guard must survive a declared fault too: a declared error overrides the STATUS test,
// never the re-entry guard, or an error raised inside the fault chain could re-enter it.
func TestFaultPolicies_DeclaredFault_StillRespectsRunOnce(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))
	ec.faultPoliciesRan = true

	ec.handleRejection(t, policy.ImmediateResponse{
		StatusCode: 500,
		IsFault:    true,
	})

	assert.Empty(t, order, "a declared fault must not bypass the run-once guard")
}

// The declaration separates a guardrail REJECTING a response from a policy merely relabelling
// the backend's error. Both set a status, so the status cannot tell them apart — which is why
// it is not consulted.
func TestFaultPolicies_DeclaredFault_SeparatesRejectionFromRelabelling(t *testing.T) {
	newStatus := 503

	t.Run("an error status is a rejection whoever set it", func(t *testing.T) {
		// Reversed deliberately. While the declaration was the whole gate, a policy
		// relabelling the backend's error was not a rejection; now the status carries it,
		// and a relabelled 503 reaches the chain. The response was already a fault by the
		// same rule before the policy touched it, so this adds no new class of event —
		// it changes which policy the fault is attributed to.
		assert.True(t, statusOverrideAction(policy.DownstreamResponseModifications{
			StatusCode: &newStatus,
		}))
	})

	t.Run("rejection declares a fault", func(t *testing.T) {
		assert.True(t, statusOverrideAction(policy.DownstreamResponseModifications{
			StatusCode: &newStatus,
			IsFault:    true,
		}))
	})

	t.Run("a sub-400 status is not a rejection", func(t *testing.T) {
		// interceptor-service is the case this protects: it applies whatever status an
		// external interceptor returned, including a 2xx, and is not rejecting anything.
		ok := 200
		assert.False(t, statusOverrideAction(policy.DownstreamResponseModifications{
			StatusCode: &ok,
		}))
		// And no status at all is never a rejection: the policy left the response alone,
		// so its own status governs on the pass-through path instead.
		assert.False(t, statusOverrideAction(policy.DownstreamResponseModifications{}))
	})

	t.Run("but the tracing outcome still sees the override", func(t *testing.T) {
		// Separate question, separate answer. Whether a policy SUPPLIED the status is not
		// whether the response is a failure, and the trace needs the first: the client's
		// status came from a policy rather than the upstream. That stays true however the
		// fault gate is written, which is why the two are computed independently.
		ok := 200
		// A 200 supplied by a policy: overridden yes, a rejection no. The pair has to be
		// asserted on a SUB-400 status now, because at 503 both are true and the test would
		// pass without distinguishing them at all.
		assert.True(t, responseStatusOverriddenByPolicy([]executor.ResponsePolicyResult{
			{Action: policy.DownstreamResponseModifications{StatusCode: &ok}},
		}))
		assert.False(t, responseRejectedByPolicy([]executor.ResponsePolicyResult{
			{Action: policy.DownstreamResponseModifications{StatusCode: &ok}},
		}))
		// And both are true at an error status, which is the case they agree on.
		assert.True(t, responseStatusOverriddenByPolicy([]executor.ResponsePolicyResult{
			{Action: policy.DownstreamResponseModifications{StatusCode: &newStatus}},
		}))
		assert.True(t, responseRejectedByPolicy([]executor.ResponsePolicyResult{
			{Action: policy.DownstreamResponseModifications{StatusCode: &newStatus}},
		}))
	})
}

// A fault policy must receive the producer's own account of the failure, and it must belong
// to the policy named beside it.
func TestFaultPolicies_DeclaredFault_ReachesTheFaultPolicy(t *testing.T) {
	var order []string
	var saw policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &saw},
	}, specs(1))

	declared := &policy.FaultDetails{
		Code: "906000", Type: "guardrail", Direction: policy.DirectionResponse,
		Message: "Request rejected",
	}
	ec.faultPolicyName = "word-count-guardrail"
	ec.faultDeclared = declared

	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422, Fault: declared,
	})

	require.Equal(t, []string{"notify"}, order)
	require.NotNil(t, saw.Fault, "the producer's FaultDetails must reach the fault policy")
	assert.Equal(t, "906000", saw.Fault.Code)
	assert.Equal(t, "guardrail", saw.Fault.Type)
	assert.Equal(t, "word-count-guardrail", saw.Policy,
		"and the attribution must name the policy the declared error came from")
}

// FaultDetails.Policy is gateway-owned: a formatter reads the failing policy's name out of
// the same object as the rest of the error, so the gateway fills it from the chain rather
// than trusting what the policy claimed about itself.
func TestFaultPolicies_DeclaredFault_PolicyIsAttributedByTheGateway(t *testing.T) {
	var order []string
	var saw policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &saw},
	}, specs(1))

	// The producer names something else entirely — a copy-paste in a policy, or a policy
	// reporting the upstream it wraps. The chain is authoritative, not this.
	declared := &policy.FaultDetails{Code: "906000", Policy: "some-other-policy"}
	ec.faultPolicyName = "word-count-guardrail"
	ec.faultDeclared = declared

	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 422, Fault: declared})

	require.NotNil(t, saw.Fault)
	assert.Equal(t, "word-count-guardrail", saw.Fault.Policy,
		"the gateway's own attribution must win over what the policy claimed")

	// And the overwrite must not reach back into the producer's struct. A policy returning a
	// package-level FaultDetails for every rejection would otherwise carry one request's
	// attribution into the next.
	assert.Equal(t, "some-other-policy", declared.Policy,
		"attribution must be applied to a copy, never to the policy's own value")
}

// The declared error must be captured at the same point the policy is attributed, so a handler can
// never read one policy's error beside another's name.
func TestFaultPolicies_DeclaredFault_CapturedWithTheAttribution(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: new([]string)}}, specs(1))

	declared := &policy.FaultDetails{Code: "900902"}
	rejection := policy.ImmediateResponse{StatusCode: 401, IsFault: true, Fault: declared}
	f, isFault := faultFromRequestHeaders(&executor.RequestHeaderExecutionResult{
		ShortCircuited: true,
		FinalAction:    rejection,
		Results: []executor.RequestHeaderPolicyResult{
			{PolicyName: "first", Action: policy.UpstreamRequestHeaderModifications{}},
			{PolicyName: "rejector", PolicyVersion: "v1", Action: rejection},
		},
	})
	require.True(t, isFault, "a short-circuit with an ImmediateResponse is a fault the phase produced")
	ec.record(f.attrib)

	assert.Equal(t, "rejector", ec.faultPolicyName)
	require.NotNil(t, ec.faultDeclared)
	assert.Equal(t, "900902", ec.faultDeclared.Code,
		"the declared error must come from the same result as the name")
}

// The two Faulted() unit tests that lived here are gone with the method.
//
// They asserted the precedence rule directly on the action types — "IsFault and nothing
// else" — which was worth doing while a Faulted() method could have grown an inference
// inside it. Reading the field, they would assert only that a struct literal has the value
// just assigned to it.
//
// The rule they protected is covered where it is actually enforced, by the tests above:
// AbsentIsNotAFault loops the statuses (200 through 503) to show the status is never
// consulted, DescribingTheFaultDoesNotOptIn shows a Fault alone does not opt in, and
// SeparatesRejectionFromRelabelling covers the same rule on
// DownstreamResponseModifications through statusOverrideAction.

// The undeclared-rejection diagnostic that lived here is GONE, with the two tests that
// covered it.
//
// It warned when a rejection described a Fault and left IsFault false, because that
// combination reached no handler. Under a status-OR-declaration gate it cannot: an error
// status carries the rejection in whatever the flag says. The only responses that still miss
// the gate are sub-400 ones, where a described Fault is the INTENDED shape — a redirect or an
// auth challenge wanting an error-shaped body — so the line would have fired on correct
// behaviour and told the operator to set a flag that would change nothing.

// captureLogs redirects the default slog handler into a buffer for the duration of one test,
// restoring it afterwards. Level is set to Debug so an Info-level diagnostic is never missed
// because of the ambient level.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// A short-circuit that describes nothing is NOT nagged. Under an explicit opt-in that is the
// correct and overwhelmingly common shape — a cache hit, a preflight, a redirect, a canned
// response — and warning about them would emit a line on nearly every short-circuit in the
// deployment, which is how a useful log line becomes one people filter out.
//
// The accepted cost: a genuine rejection from a policy predating this contract also goes
// unreported. There is no signal that separates it from the four cases above.
func TestFaultPolicies_SilentShortCircuitIsNotNagged(t *testing.T) {
	logged := captureLogs(t)

	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: new([]string)},
	}, specs(1))
	ec.handleRejection(t, policy.ImmediateResponse{StatusCode: 204})

	assert.NotContains(t, logged.String(), "described a fault but left IsFault false",
		"a short-circuit that describes nothing must not be nagged")
}

// A rejection that DESCRIBES its error still gets a formatted body, even when it declared
// itself not-a-fault.
//
// Found by running the demo, not by a unit test. Formatting is gated on body AUTHORSHIP, so
// the two decisions are independent: Fault asks for a rendered body, IsFault asks for the
// fault policies. Wiring formatting only into the fault path made the documented
// describe-and-let-the-gateway-render pattern return an EMPTY body to the client whenever
// the policy had not also opted in — which is every policy written before IsFault existed.
func TestFaultPolicies_UndeclaredRejectionStillGetsAFormattedBody(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)

	var out policy.ImmediateResponse
	ec.handleFault(context.Background(), fault{
		origin: originGateway,
		rejection: &policy.ImmediateResponse{
			StatusCode: 401,
			Fault: &policy.FaultDetails{Code: "900902", Type: "authentication",
				Message: "Valid credentials required"},
		},
		writeBack: func(r policy.ImmediateResponse) { out = r },
	})

	require.NotEmpty(t, out.Body,
		"a described error must still be rendered — an empty body is not a protocol")
	assert.Contains(t, string(out.Body), "900902")
	assert.Contains(t, string(out.Body), "Valid credentials required")
	assert.False(t, ec.faultPoliciesRan, "and it must still stay out of the fault flow")
}

// The other half: describing nothing means there is nothing to render, so the body stays as
// the policy left it. A cache hit or preflight must not acquire an invented error body.
func TestFaultPolicies_SilentRejectionBodyIsLeftAlone(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)

	var out policy.ImmediateResponse
	ec.handleFault(context.Background(), fault{
		origin:    originGateway,
		rejection: &policy.ImmediateResponse{StatusCode: 204},
		writeBack: func(r policy.ImmediateResponse) { out = r },
	})

	assert.Empty(t, out.Body, "nothing described, nothing to render")
	assert.False(t, ec.faultPoliciesRan)
}
