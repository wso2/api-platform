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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pkg/cel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// ─── test doubles ────────────────────────────────────────────────────────────

// faultRecorderPolicy records that it ran and stamps a header, standing in for a
// real side-effecting fault policy such as interceptor-service.
type faultRecorderPolicy struct {
	name      string
	order     *[]string
	setHeader string
	sawErrCtx *policy.FaultContext
}

func (p *faultRecorderPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (p *faultRecorderPolicy) OnFault(_ context.Context, faultCtx *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	*p.order = append(*p.order, p.name)
	if p.sawErrCtx != nil {
		*p.sawErrCtx = *faultCtx
	}
	if p.setHeader == "" {
		// nil rather than an empty struct — the contract's way of saying "changed nothing".
		return nil
	}
	return &policy.FaultResponse{
		HeadersToSet: map[string]string{p.setHeader: p.name},
	}
}

// faultReplacerPolicy replaces the error response and ends the chain, like the respond
// policy. Final is what ends it now; the body and status are ordinary field settings.
type faultReplacerPolicy struct{ order *[]string }

func (p *faultReplacerPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (p *faultReplacerPolicy) OnFault(_ context.Context, _ *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	*p.order = append(*p.order, "replacer")
	status := 500
	return &policy.FaultResponse{
		StatusCode:   &status,
		HeadersToSet: map[string]string{"content-type": "application/json"},
		Body:         []byte(`{"replaced":"by fault policies"}`),
		Final:        true,
	}
}

// ─── fixtures ────────────────────────────────────────────────────────────────

func faultExecCtx(t *testing.T, faultPolicies []policy.Policy, specs []policy.PolicySpec) *PolicyExecutionContext {
	t.Helper()
	k := NewKernel()
	// A REAL CEL evaluator, not nil. With nil, any fault entry carrying an
	// executionCondition panicked in the executor, so the whole conditional-skip path was
	// unreachable from a kernel test — conditions could only be exercised in the cel
	// package or against a running gateway. Production never passes nil (both entry points
	// exit if the evaluator cannot be built), so the fixture was the only thing pretending
	// it could.
	celEval, err := cel.NewCELEvaluator()
	require.NoError(t, err, "the fixture needs a real evaluator to exercise conditions")
	chainExecutor := executor.NewChainExecutor(nil, celEval, noop.NewTracerProvider().Tracer(""))
	// Upstream faults are opted IN here. This fixture exists to exercise the fault flow, and
	// the deployment flag gating upstream/router failures is not what any test using it is
	// about — leaving it off would make every passthrough test in this file silently assert
	// nothing. The flag's own behaviour, both ways, lives in handle_upstream_faults_config_test.go.
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20,
		WithHandleUpstreamFaults(true))

	hasConditions := false
	for _, s := range specs {
		if s.ExecutionCondition != nil && *s.ExecutionCondition != "" {
			hasConditions = true
		}
	}

	chain := &registry.PolicyChain{
		FaultPolicies:               faultPolicies,
		FaultPolicySpecs:            specs,
		HasFaultPolicies:            len(faultPolicies) > 0,
		FaultHasExecutionConditions: hasConditions,
	}

	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.sharedCtx = &policy.SharedContext{APIName: "TestAPI", APIVersion: "v1.0"}
	ec.requestHeaderCtx = &policy.RequestHeaderContext{
		SharedContext: ec.sharedCtx,
		Headers:       policy.NewHeaders(map[string][]string{"x-caller": {"probe"}}),
		Path:          "/orders/v1/submit",
		Method:        "POST",
		Authority:     "api.example.com",
		Scheme:        "https",
		Vhost:         "default",
		Upstream:      &policy.UpstreamRequestContext{Name: "orders-backend", URL: "https://orders.internal", BasePath: "/v1"},
	}
	return ec
}

// The request identity reaches a handler whatever phase failed. Authority, scheme and vhost
// live on the two REQUEST-phase contexts and on neither response-phase one, so before the
// gateway restated them a fault handler could not read them at all — not even for a failure
// raised in the request header phase, which is the phase that has them.
func TestFaultPolicies_ErrorContext_CarriesTheRequestIdentity(t *testing.T) {
	var order []string
	var seen policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &seen},
	}, specs(1))

	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
	})

	assert.Equal(t, "api.example.com", seen.RequestAuthority)
	assert.Equal(t, "https", seen.RequestScheme)
	assert.Equal(t, "default", seen.RequestVhost)
	// And the fields that were already carried are unchanged.
	assert.Equal(t, "/orders/v1/submit", seen.RequestPath)
	assert.Equal(t, "POST", seen.RequestMethod)
}

// A REQUEST-phase rejection reports the upstream the route resolved, even though the request
// never reached it. That is the field a handler needs to say which backend a rejected request
// was bound for, and it was nil on this path.
//
// Response stays nil: no upstream response existed, and synthesising an empty one would let a
// handler believe the backend answered with a zero status.
func TestFaultPolicies_ErrorContext_CarriesTheUpstreamTargetOnARequestRejection(t *testing.T) {
	var order []string
	var seen policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &seen},
	}, specs(1))

	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
	})

	require.NotNil(t, seen.Upstream, "a rejected request still has a resolved upstream target")
	assert.Equal(t, "orders-backend", seen.Upstream.Name)
	assert.Equal(t, "https://orders.internal", seen.Upstream.URL)
	assert.Equal(t, "/v1", seen.Upstream.BasePath)
	assert.Nil(t, seen.Upstream.Response,
		"there was no upstream response; an empty one would read as a zero-status answer")
}

// A response-phase failure uses the response view's own upstream, which carries the response
// snapshot too — the request-phase projection must not shadow it.
func TestFaultPolicies_ErrorContext_PrefersTheResponseUpstreamWhenThereIsOne(t *testing.T) {
	var order []string
	var seen policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &seen},
	}, specs(1))
	withRejectedBody(ec, 446)
	ec.responseBodyCtx.Upstream = &policy.UpstreamResponseContext{
		Name: "orders-backend", URL: "https://orders.internal", BasePath: "/v1",
		Response: &policy.UpstreamResponse{StatusCode: 200},
	}

	ec.runFaultPoliciesOnResponse(context.Background(), bodyRejection(446), originGateway)

	require.NotNil(t, seen.Upstream)
	require.NotNil(t, seen.Upstream.Response,
		"the response-phase view has a snapshot; the request-phase projection must not replace it")
	assert.Equal(t, 200, seen.Upstream.Response.StatusCode)
}

// specs builds n fault specs. Enabled MUST be true: the executor skips any spec with
// Enabled=false as "disabled", so fault policies whose specs are built without it
// silently never runs. Slice 2 has to set this when the controller assembles the chain.
func specs(n int, conditions ...string) []policy.PolicySpec {
	out := make([]policy.PolicySpec, n)
	for i := range out {
		out[i].Name = "fault-policy"
		out[i].Version = "v1"
		out[i].Enabled = true
		if i < len(conditions) && conditions[i] != "" {
			c := conditions[i]
			out[i].ExecutionCondition = &c
		}
	}
	return out
}

func errResp(status int) policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode: status,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       []byte(`{"error":"rejected"}`),
	}
}

// ─── rejections: errors the engine builds ────────────────────────────────────

// A guardrail 422 is the case this feature exists to serve: the engine built the
// error, so the fault policies must run even though it is a 4xx.
func TestFaultPolicies_Rejection_RunsOnGuardrail422(t *testing.T) {
	var order []string
	ec := faultExecCtx(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notify", order: &order, setHeader: "x-fault-handled"}},
		specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, []string{"notify"}, order, "fault policies must run for an engine-generated 4xx")
	assert.Equal(t, 422, out.StatusCode, "status must be preserved")
	assert.Equal(t, "notify", out.Headers["x-fault-handled"], "header mutation must reach the error response")
	assert.Equal(t, "application/json", out.Headers["content-type"], "existing headers must survive")
}

// The sequence is authored top-to-bottom, but the response-phase executor walks its
// list back to front. Declaration order must win.
func TestFaultPolicies_RunsInDeclarationOrder(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "first", order: &order},
		&faultRecorderPolicy{name: "second", order: &order},
		&faultRecorderPolicy{name: "third", order: &order},
	}, specs(3))

	ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, []string{"first", "second", "third"}, order)
}

// A skipped entry still contributes a RESULT, marked skipped, and the entries after it run.
//
// Both skip reasons are covered because the executor reports them through one shared
// recorder: a disabled entry and an entry whose condition read false produce the same four
// things (span attributes, a metric, Skipped: true, an execution time). Nothing asserted any
// of that before — flipping Skipped to false failed no test — so the recorder was
// unguarded, and a skipped entry vanishing from the results would have been invisible to
// tracing and metrics rather than to behaviour.
func TestFaultPolicies_ASkippedEntryStillReportsAResult(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]policy.PolicySpec)
		conds  []string
	}{
		{"disabled", func(sp []policy.PolicySpec) { sp[0].Enabled = false }, nil},
		{"condition false", nil, []string{"fault.Status == 999"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var order []string
			sp := specs(2, tc.conds...)
			if tc.mutate != nil {
				tc.mutate(sp)
			}
			ec := faultExecCtx(t, []policy.Policy{
				&faultRecorderPolicy{name: "skipped-one", order: &order},
				&faultRecorderPolicy{name: "second", order: &order},
			}, sp)

			faultCtx := ec.synthesizeFaultContexts(errResp(422))
			out, ok := ec.executeFaultPolicies(context.Background(), faultCtx, originGateway, true)

			require.True(t, ok, "the chain was configured, so it must have run")
			assert.Equal(t, []string{"second"}, order,
				"the skipped entry must not execute, and the one after it must")
			require.Len(t, out.results, 2,
				"a skipped entry still contributes a result — tracing and metrics read it")
			assert.True(t, out.results[0].Skipped, "and it is marked skipped")
			assert.Nil(t, out.results[0].Response, "with nothing to apply")
			assert.False(t, out.results[1].Skipped, "while the entry that ran is not")
		})
	}
}

// Final ends the chain, so an entry declared after the one that set it must not run.
//
// Distinct from the replace-the-response test below: that one runs a Final entry ALONE, so it
// passes whether or not Final is honoured — applyFaultResponse writes the body and status
// either way. Only a second entry can tell the two apart.
func TestFaultPolicies_FinalStopsTheEntriesAfterIt(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultReplacerPolicy{order: &order},
		&faultRecorderPolicy{name: "after", order: &order},
	}, specs(2))

	out := ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, []string{"replacer"}, order,
		"an entry after the one that set Final must never run")
	assert.Equal(t, 500, out.StatusCode, "and the Final entry's response still applies")
}

// A fault policy may take over the response entirely.
func TestFaultPolicies_Rejection_PolicyCanReplaceResponse(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultReplacerPolicy{order: &order}}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, []string{"replacer"}, order)
	assert.Equal(t, 500, out.StatusCode)
	assert.Contains(t, string(out.Body), "by fault policies")
}

// Opting out is what keeps a successful short-circuit out of the fault flow, and it must
// leave the response completely untouched — no chain, and no formatted body either.
func TestFaultPolicies_SkippedWhenThePolicySaysItIsNotAFault(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

	out := ec.handleRejection(t, policy.ImmediateResponse{StatusCode: 200})

	assert.Empty(t, order, "a declared non-fault must not run the fault chain")
	assert.Equal(t, 200, out.StatusCode)
	assert.Nil(t, out.Body, "and must not have a body synthesized for it")
}

func TestFaultPolicies_SkippedWhenNoneConfigured(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	require.False(t, ec.policyChain.HasFaultPolicies)

	out := ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, errResp(422).Headers, out.Headers, "response must be untouched")
}

// Run-once semantics: what stops an error raised inside the fault chain from
// re-entering it, and stops a rejection and a pass-through both firing for one failure.
func TestFaultPolicies_RunsAtMostOncePerRequest(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

	ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))
	ec.runFaultPoliciesOnRejection(context.Background(), errResp(500))
	ec.runFaultPoliciesOnResponse(context.Background(), &executor.ResponseExecutionResult{}, originUpstream)

	assert.Equal(t, []string{"notify"}, order, "the fault chain must execute exactly once per request")
}

// ─── pass-through: errors the engine did not build ───────────────────────────

// A pass-through error is handled in the response-BODY phase, so its fixture must populate
// the body context as well: that is where the status is read from and where body-hook fault
// entries are dispatched.
func responseHeaderCtxWithStatus(ec *PolicyExecutionContext, status int) {
	headers := policy.NewHeaders(map[string][]string{"content-type": {"application/json"}})
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  status,
		ResponseHeaders: headers,
	}
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  status,
		ResponseHeaders: headers,
		ResponseBody: &policy.Body{
			Content: []byte(`{"error":"upstream failure"}`), EndOfStream: true, Present: true,
		},
	}
}

// responseBodyCtxWithContent replaces the body context's content, for the cases that turn on
// whether the response carried bytes at all. nil means a bodyless response, which is a
// different thing from a body holding zero bytes and has to stay expressible.
func responseBodyCtxWithContent(ec *PolicyExecutionContext, status int, content []byte) {
	body := &policy.Body{Content: content, EndOfStream: true, Present: true}
	if content == nil {
		body = nil
	}
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  status,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseBody:    body,
	}
}

// routerError marks the response as one Envoy produced because it could not complete the
// proxy attempt. Provenance no longer decides whether the flow runs — every source reaches it
// — but it decides what the handler and any execution condition are told (see error_source.go).
func routerError(ec *PolicyExecutionContext) {
	ec.responseCodeDetails = "upstream_reset_before_response_started{remote_connection_failure}"
}

// backendError marks the response as one the upstream itself returned.
func backendError(ec *PolicyExecutionContext) {
	ec.responseCodeDetails = "via_upstream"
}

// A backend or Envoy 5xx fires an unconditional entry.
func TestFaultPolicies_Passthrough_FiresForARouterError(t *testing.T) {
	var order []string
	ec := faultExecCtx(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notify", order: &order, setHeader: "x-fault-handled"}},
		specs(1))
	responseHeaderCtxWithStatus(ec, 503)
	routerError(ec)

	execResult := &executor.ResponseExecutionResult{}
	ran := ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream)

	assert.True(t, ran)
	assert.Equal(t, []string{"notify"}, order)
	assert.NotEmpty(t, execResult.Results,
		"fault results must be merged into the caller's result so the translator emits them")
}

// An error the BACKEND returned now reaches the flow, at any error status.
//
// This inverts what the gate used to do. The operator running the gateway in front of a
// backend they do not own is exactly the person who needs to hear it is failing, and who may
// need to reshape its errors before their own callers see them; the gate removed that ability
// rather than exercising it on their behalf. Narrowing back to gateway-only failures is now an
// execution condition on the entry (`fault.Source == "gateway"`), which is a decision the
// deployment makes rather than one the engine makes for it.
func TestFaultPolicies_Passthrough_FiresForABackendErrorAtAnyStatus(t *testing.T) {
	for _, status := range []int{404, 409, 500, 503} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var order []string
			ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))
			responseHeaderCtxWithStatus(ec, status)
			backendError(ec)

			ran := ec.runFaultPoliciesOnResponse(context.Background(), &executor.ResponseExecutionResult{}, originUpstream)

			assert.True(t, ran, "a backend-produced %d must reach the fault flow", status)
			assert.Equal(t, []string{"notify"}, order)
			assert.Equal(t, sourceBackend, ec.faultSource,
				"the handler and any condition must be able to see this came from the backend")
		})
	}
}

// A backend that sent bytes decided the client's body, and the formatter must leave it alone.
//
// This is the rule that had to arrive with the gate's removal. faultBodyAuthored tracks
// gateway-side authorship, so before this an upstream error document reached the formatter
// looking unauthored — and on a formatted kind it would have been replaced by gateway house
// style. Nothing shipping formats today, which is exactly why this needs a test: the bug
// would lie dormant until SoapApi turned formatting on.
func TestFaultPolicies_Passthrough_BackendBodyIsAuthorshipButAnEmptyOneIsNot(t *testing.T) {
	t.Run("with a body", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		responseHeaderCtxWithStatus(ec, 502)
		backendError(ec)
		responseBodyCtxWithContent(ec, 502, []byte(`{"upstreamSaysNo":true}`))

		ec.faultSource = sourceBackend
		ec.noteUpstreamAuthoredBody()

		assert.True(t, ec.faultBodyAuthored,
			"the backend's own error document is the client's copy, not a fallback to render over")
	})

	t.Run("with no body", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		responseHeaderCtxWithStatus(ec, 502)
		backendError(ec)
		responseBodyCtxWithContent(ec, 502, nil)

		ec.faultSource = sourceBackend
		ec.noteUpstreamAuthoredBody()

		assert.False(t, ec.faultBodyAuthored,
			"a bodyless upstream failure left nothing to preserve, so a formatted kind may render one")
	})

	t.Run("a gateway error is unaffected", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		responseHeaderCtxWithStatus(ec, 422)
		responseBodyCtxWithContent(ec, 422, []byte(`{"policySaysNo":true}`))

		ec.faultSource = sourceGateway
		ec.noteUpstreamAuthoredBody()

		assert.False(t, ec.faultBodyAuthored,
			"gateway-side authorship is decided by the producer rules, not by whether bytes exist")
	})
}

// A router error fires at any error status. There is no threshold any more: the status was
// only ever a proxy for "probably infrastructure", and the source states it outright.
func TestFaultPolicies_Passthrough_FiresForARouterErrorAtAnyStatus(t *testing.T) {
	for _, status := range []int{404, 429, 502, 503, 504} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var order []string
			ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))
			responseHeaderCtxWithStatus(ec, status)
			routerError(ec)

			ran := ec.runFaultPoliciesOnResponse(context.Background(), &executor.ResponseExecutionResult{}, originUpstream)

			assert.True(t, ran, "a router-produced %d is the gateway's own error", status)
			assert.Equal(t, []string{"notify"}, order)
		})
	}
}

// Provenance absent means unknown, and unknown now runs rather than failing closed.
//
// The version-skew case: a router too old to report provenance. Declining used to be the safe
// answer because the alternative was firing on every backend error in the deployment — but
// backend errors are wanted now, so declining only loses failures. What "unknown" must not do
// is masquerade as something specific: a condition testing == "gateway" excludes it, which is
// the operator's call rather than the engine's.
func TestFaultPolicies_Passthrough_RunsWithoutProvenanceAndReportsItAsUnknown(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))
	responseHeaderCtxWithStatus(ec, 503)
	// responseCodeDetails deliberately left empty.

	ran := ec.runFaultPoliciesOnResponse(context.Background(), &executor.ResponseExecutionResult{}, originUpstream)

	assert.True(t, ran, "a failure with no provenance must still reach the flow")
	assert.Equal(t, []string{"notify"}, order)
	assert.Equal(t, sourceUnknown, ec.faultSource,
		"it must report as unknown, never be promoted to a source it cannot be shown to be")
}

// A rejection needs no provenance: the engine built the error, so it is the gateway's by
// construction, and a guardrail rejection is a 4xx by nature.
func TestFaultPolicies_Rejection_FiresOn4xxWithoutProvenance(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

	ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, []string{"notify"}, order,
		"a rejection must fire — the engine built this error, so it is a fault by construction")
}

func TestFaultPolicies_Passthrough_SkippedForSuccess(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))
	responseHeaderCtxWithStatus(ec, 200)

	ran := ec.runFaultPoliciesOnResponse(context.Background(), &executor.ResponseExecutionResult{}, originUpstream)

	assert.False(t, ran)
	assert.Empty(t, order)
}

// The property the single-object change could plausibly have broken: a later entry must
// observe what an earlier one did.
//
// Before, the chain mutated a response-shaped view and copied the status back into the
// FaultContext between entries, because the two were different objects and the status is a
// value type. There is one object now, so the second entry reads the first entry's status
// directly — and if that ever regresses, the copy-back is what is missing.
func TestFaultPolicies_ALaterEntrySeesWhatAnEarlierOneDid(t *testing.T) {
	var order []string
	var secondSaw policy.FaultContext

	first := &faultStatusSettingPolicy{name: "first", order: &order, status: 503}
	second := &faultRecorderPolicy{name: "second", order: &order, sawErrCtx: &secondSaw}

	ec := faultExecCtx(t, []policy.Policy{first, second}, specs(2))
	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
	})

	assert.Equal(t, []string{"first", "second"}, order, "entries run in declared order")
	assert.Equal(t, 503, secondSaw.ResponseStatus,
		"the second entry must read the status the first one set, not the original 401")
	assert.Equal(t, 503, out.StatusCode, "and the caller reads it off the same object")
}

// faultStatusSettingPolicy overrides the status and nothing else, so the next entry's view of
// it is the only thing under test.
type faultStatusSettingPolicy struct {
	name   string
	order  *[]string
	status int
}

func (p *faultStatusSettingPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

func (p *faultStatusSettingPolicy) OnFault(_ context.Context, _ *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	*p.order = append(*p.order, p.name)
	status := p.status
	return &policy.FaultResponse{StatusCode: &status}
}

// ─── context synthesis ──────────────────────────────────────────────────────

// A fault policy needs to see what the caller sent, even on the rejection path where no
// upstream response ever existed.
func TestFaultPolicies_SynthesizedContextCarriesRequestData(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)

	respCtx := ec.synthesizeFaultContexts(errResp(422))

	assert.Equal(t, 422, respCtx.ResponseStatus)
	assert.Equal(t, "/orders/v1/submit", respCtx.RequestPath)
	assert.Equal(t, "POST", respCtx.RequestMethod)
	require.NotNil(t, respCtx.RequestHeaders)
	assert.Equal(t, []string{"probe"}, respCtx.RequestHeaders.Get("x-caller"))
	require.NotNil(t, respCtx.SharedContext)
	assert.Equal(t, "TestAPI", respCtx.APIName)
	assert.Equal(t, []string{"application/json"}, respCtx.ResponseHeaders.Get("content-type"))
}

// ─── response-body rejections (status override, no short-circuit) ────────────

// bodyRejection is what a response guardrail returns to reject a response: a status
// override through DownstreamResponseModifications, NOT an ImmediateResponse. Because
// its StopExecution() is false the executor never sets ShortCircuited, which is why
// a short-circuit test cannot see it. See word-count-guardrail's response path.
func bodyRejection(status int) *executor.ResponseExecutionResult {
	s := status
	return &executor.ResponseExecutionResult{
		Results: []executor.ResponsePolicyResult{{
			PolicyName:    "word-count-guardrail",
			PolicyVersion: "v1",
			Action: policy.DownstreamResponseModifications{
				StatusCode:   &s,
				Body:         []byte(`{"error":"word count violation"}`),
				HeadersToSet: map[string]string{"Content-Type": "application/json"},
			},
		}},
	}
}

// withRejectedBody puts the execution context into the state the body phase leaves it in
// after applyResponseModifications has written the override onto the context.
//
// The phase is set explicitly because these tests call the fault entry points directly;
// in the real path processResponseBody sets it on entry, which the integration tests
// exercise.
func withRejectedBody(ec *PolicyExecutionContext, status int) {
	ec.phase = phaseResponseBody
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		RequestPath:     ec.requestHeaderCtx.Path,
		RequestMethod:   ec.requestHeaderCtx.Method,
		ResponseHeaders: policy.NewHeaders(map[string][]string{":status": {"200"}}),
		ResponseStatus:  200,
	}
	// Mirrors the live construction in execution_context.go: the engine builds this context
	// with the request-side fields populated, and a fixture that left them out was only
	// passing because the removed header view back-filled them from responseHeaderCtx —
	// asserting request data against a fallback production never reaches.
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		RequestPath:     ec.requestHeaderCtx.Path,
		RequestMethod:   ec.requestHeaderCtx.Method,
		Downstream:      ec.requestHeaderCtx.Downstream,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseStatus:  status,
	}
}

// The regression this trigger exists for: a response guardrail rejecting a response must
// reach the fault policies. Before this case was handled, it ran nothing at all.
func TestFaultPolicies_ResponseBody_RunsOnResponseBodyRejection(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notifier", order: &order, setHeader: "x-fault-notified"},
	}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	ran := ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway)

	assert.True(t, ran, "a response-body status override must enter the fault policies")
	assert.Equal(t, []string{"notifier"}, order, "the fault policy ran")
	assert.True(t, ec.faultPoliciesRan)
}

// A status-override rejection is one the ENGINE built, so it reaches the chain without the
// entry declaring an executionCondition to narrow it in. A guardrail rejection is a 4xx, and
// nothing about the status is what admits it.
//
// Renamed: the old name referred to a 5xx default that no longer exists anywhere in the
// engine, so it described a design rather than this assertion.
func TestFaultPolicies_ResponseBody_RunsWithoutAnExecutionCondition(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notifier", order: &order},
	}, specs(1)) // no execution condition
	withRejectedBody(ec, 422)

	ran := ec.runFaultPoliciesOnResponse(context.Background(), bodyRejection(422), originGateway)

	assert.True(t, ran, "a status-override rejection must not inherit the pass-through 5xx rule")
	assert.Equal(t, []string{"notifier"}, order)
}

// A policy overriding the status to a success value: a fault only if it SAID so.
//
// Both rows go through the real derivation rather than passing an origin literal. The origin
// is what carries the declaration on this path — originGateway is set exactly when
// statusOverrideAction matched — so handing it in directly could assert a combination the
// flow cannot produce, which is what the previous version of this test did.
//
// The declared row is the one that was broken. runFaultPoliciesOnResponse read only the
// status, so a 200 the policy called a failure passed statusOverrideAction and
// responseRejectedByPolicy and was then dropped at the gate — the two halves disagreeing
// depending on which phase produced the fault.
func TestFaultPolicies_ResponseBody_SuccessOverrideIsAFaultOnlyIfDeclared(t *testing.T) {
	run := func(t *testing.T, declared bool) ([]string, bool) {
		t.Helper()
		var order []string
		ec := faultExecCtx(t, []policy.Policy{
			&faultRecorderPolicy{name: "notifier", order: &order},
		}, specs(1))
		withRejectedBody(ec, 200)

		ok := 200
		mods := policy.DownstreamResponseModifications{StatusCode: &ok, IsFault: declared}
		res := &executor.ResponseExecutionResult{
			Results: []executor.ResponsePolicyResult{
				{PolicyName: "graphql-checker", PolicyVersion: "v1", Action: mods},
			},
		}
		// The derivation the phase handler performs, not a hand-picked origin.
		rejected := responseRejectedByPolicy(res.Results)
		assert.Equal(t, declared, rejected,
			"responseRejectedByPolicy must agree with the declaration on a 200")
		ec.handleFault(context.Background(), faultFromResponseBody(res, rejected))
		return order, ec.faultPoliciesRan
	}

	t.Run("undeclared 200 override is not a fault", func(t *testing.T) {
		order, ran := run(t, false)
		assert.Empty(t, order, "nothing claimed this response, and 200 cannot speak for it")
		assert.False(t, ran, "and the run-once flag is untouched")
	})

	t.Run("declared 200 override IS a fault", func(t *testing.T) {
		order, ran := run(t, true)
		assert.Equal(t, []string{"notifier"}, order,
			"a 200 the policy called a failure must reach the chain — no status rule can")
		assert.True(t, ran)
	})
}

// Run-once still holds: if a pass-through already handled this request, a status-override must
// not run the fault chain a second time.
func TestFaultPolicies_ResponseBody_RespectsRunOnce(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notifier", order: &order},
	}, specs(1))
	withRejectedBody(ec, 500)
	ec.faultPoliciesRan = true // as if a pass-through had already run

	ran := ec.runFaultPoliciesOnResponse(context.Background(), bodyRejection(500), originGateway)

	assert.False(t, ran)
	assert.Empty(t, order, "the fault chain must run at most once per request")
}

// Reshaping works at this phase: the buffered body has not been forwarded, so a fault
// policy can replace the rejection outright.
//
// It reaches the wire as a MODIFICATION now, not a short-circuit. The old contract could only
// express "replace" as an ImmediateResponse, which Envoy sends as a fresh response with a
// fresh header set — so the error's own headers were discarded whether or not the policy
// meant to discard them. A modification carries the status (the translator sets `:status`,
// which is still mutable here because ext_proc holds the response headers until the buffered
// body phase completes) and the body, and leaves the rest of the headers alone.
//
// So the assertion is on the OUTCOME the client gets rather than the mechanism that delivers
// it: an assertion on ShortCircuited would pin an implementation detail that does not have to
// be true for the replacement to work.
func TestFaultPolicies_ResponseBody_PolicyCanReplaceResponse(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultReplacerPolicy{order: &order}}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	ran := ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway)

	require.True(t, ran)
	assert.Equal(t, []string{"replacer"}, order)

	var replaced *policy.DownstreamResponseModifications
	for i := range execResult.Results {
		mods, ok := execResult.Results[i].Action.(policy.DownstreamResponseModifications)
		if ok && mods.Body != nil {
			replaced = &mods
		}
	}
	require.NotNil(t, replaced, "the replacement must reach the caller's results")
	require.NotNil(t, replaced.StatusCode)
	assert.Equal(t, 500, *replaced.StatusCode)
	assert.JSONEq(t, `{"replaced":"by fault policies"}`, string(replaced.Body))

	// And the formatter stood down, because the entry authored a body.
	assert.True(t, ec.faultBodyAuthored,
		"a fault entry that wrote a body has decided what the client receives")
}

// Header mutations must be CONVERTED to the buffered-body action type, because
// TranslateResponseBodyActions only reads DownstreamResponseModifications. Appending the
// header-phase type as-is would compile and then silently never reach the wire.
func TestFaultPolicies_ResponseBody_HeaderMutationsReachTheWire(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notifier", order: &order, setHeader: "x-fault-notified"},
	}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	require.Len(t, execResult.Results, 2, "the fault result must be appended to the body-phase results")
	mods, ok := execResult.Results[1].Action.(policy.DownstreamResponseModifications)
	require.True(t, ok, "the fault action must be converted to the body-phase action type")
	assert.Equal(t, "notifier", mods.HeadersToSet["x-fault-notified"])
	assert.Nil(t, mods.StatusCode, "a header-only fault policy must not alter the rejection status")
	assert.Nil(t, mods.Body, "a header-only fault policy must not blank the rejection body")
}

// The rejecting policy's own status and body must survive a fault policy that only
// notifies — the client still gets the guardrail's error.
func TestFaultPolicies_ResponseBody_PreservesOriginalRejection(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notifier", order: &order},
	}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	original, ok := execResult.Results[0].Action.(policy.DownstreamResponseModifications)
	require.True(t, ok)
	require.NotNil(t, original.StatusCode)
	assert.Equal(t, 446, *original.StatusCode, "the guardrail's rejection is untouched")
	assert.False(t, execResult.ShortCircuited, "a notify-only sequence must not short-circuit")
}

// The fault context must describe the REJECTION (446), not the upstream's success status,
// and must still carry request-side data for the notification payload.
func TestFaultPolicies_ResponseBody_ContextDescribesTheRejection(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	withRejectedBody(ec, 446)

	respCtx := ec.faultContextsFromResponse(446)

	assert.Equal(t, 446, respCtx.ResponseStatus, "the fault context must carry the overridden status")
	assert.Equal(t, "/orders/v1/submit", respCtx.RequestPath)
	assert.Equal(t, "POST", respCtx.RequestMethod)
	assert.Equal(t, "TestAPI", respCtx.APIName)
	assert.Equal(t, []string{"application/json"}, respCtx.ResponseHeaders.Get("content-type"),
		"headers must be the post-rejection set, not the upstream's originals")
}

// ─── dual-hook dispatch ─────────────────────────────────────────────────────

// bodyHookFaultPolicy implements ONLY OnResponseBody, like interceptor-service and every
// response guardrail. Before dual-hook dispatch such a policy was silently skipped in a
// fault policies by ExecuteResponseHeaderPolicies' interface assertion.
type bodyHookFaultPolicy struct {
	name        string
	order       *[]string
	sawBody     *[]byte
	sawMetadata *map[string]interface{}
	transform   []byte
}

func (p *bodyHookFaultPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		ResponseHeaderMode: policy.HeaderModeSkip, // deliberately NOT process
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

func (p *bodyHookFaultPolicy) OnFault(_ context.Context, faultCtx *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	*p.order = append(*p.order, p.name)
	if p.sawBody != nil && faultCtx.ResponseBody != nil {
		*p.sawBody = faultCtx.ResponseBody.Content
	}
	if p.sawMetadata != nil && faultCtx.SharedContext != nil {
		captured := make(map[string]interface{}, len(faultCtx.Metadata))
		for k, v := range faultCtx.Metadata {
			captured[k] = v
		}
		*p.sawMetadata = captured
	}
	if p.transform != nil {
		return &policy.FaultResponse{Body: p.transform}
	}
	return nil
}

// Every entry runs through OnFault, so a policy needs no response-phase hook at all. The
// shape that used to be silently skipped — no OnResponseHeaders — now runs.
func TestFaultPolicies_Dispatch_EveryEntryRunsThroughOnError(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "interceptor", order: &order},
	}, specs(1))

	ec.runFaultPoliciesOnRejection(context.Background(), errResp(401))

	assert.Equal(t, []string{"interceptor"}, order)
}

// Declaration order is the executed order. The fault list is authored top to bottom, unlike
// the response chain which unwinds back to front.
func TestFaultPolicies_Dispatch_KeepsDeclarationOrder(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "first", order: &order},
		&bodyHookFaultPolicy{name: "second", order: &order},
		&faultRecorderPolicy{name: "third", order: &order},
		&bodyHookFaultPolicy{name: "fourth", order: &order},
	}, specs(4))

	ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.Equal(t, []string{"first", "second", "third", "fourth"}, order)
}

// notAFaultPolicy implements a response hook but NOT OnFault — the shape of every catalogue
// policy that has not been given a fault contract.
type notAFaultPolicy struct{ ran *bool }

func (notAFaultPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (p notAFaultPolicy) OnResponseHeaders(_ context.Context, _ *policy.ResponseHeaderContext,
	_ map[string]interface{}) policy.ResponseHeaderAction {
	*p.ran = true
	return policy.DownstreamResponseHeaderModifications{}
}

// The reason OnFault exists. A policy without it is REPORTED and skipped, not quietly
// passed over — and critically, its response hook is not called as a fallback, or a
// notification policy misplaced here would still fire and the type contract would mean
// nothing.
func TestFaultPolicies_Dispatch_EntryWithoutOnFaultIsSkippedNotFallenBackTo(t *testing.T) {
	ran := false
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		notAFaultPolicy{ran: &ran},
		&faultRecorderPolicy{name: "valid", order: &order},
	}, specs(2))

	out := ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.False(t, ran, "a response hook must never be used as a fallback for a missing OnFault")
	assert.Equal(t, []string{"valid"}, order,
		"one unusable entry must not suppress the entries after it")
	assert.Equal(t, 422, out.StatusCode, "and the client's error must be untouched")
}

// A fault policy sees the error body through the embedded response context, which is what
// lets it TRANSFORM the error rather than only replace it.
func TestFaultPolicies_Dispatch_SeesAndTransformsTheError(t *testing.T) {
	var order []string
	var saw []byte
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{
			name: "transformer", order: &order, sawBody: &saw,
			transform: []byte(`{"error":"house-style"}`),
		},
	}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), errResp(422))

	assert.JSONEq(t, `{"error":"rejected"}`, string(saw),
		"the fault policy must receive the ORIGINAL error body")
	assert.JSONEq(t, `{"error":"house-style"}`, string(out.Body),
		"and its transformation must reach the response")
	assert.Equal(t, 422, out.StatusCode, "an untouched status must be preserved")
}

// ─── fault metadata ─────────────────────────────────────────────────────────

// The point of the metadata: a notifier can report WHICH policy failed, not just a status.
func TestFaultPolicies_Metadata_AttributesTheFailingPolicy(t *testing.T) {
	var order []string
	var md map[string]interface{}
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "notifier", order: &order, sawMetadata: &md},
	}, specs(1))
	ec.faultPolicyName, ec.faultPolicyVersion = "word-count-guardrail", "v1"

	ec.runFaultPoliciesOnRejection(context.Background(), errResp(446))

	require.NotNil(t, md, "the fault policy must observe SharedContext.Metadata")
	assert.Equal(t, "word-count-guardrail", md[FaultMetaPolicy])
	assert.Equal(t, "v1", md[FaultMetaPolicyVersion])
	assert.Equal(t, 446, md[FaultMetaStatus])
}

// For a pass-through no policy failed — the backend or Envoy produced the error. Naming a policy
// there would be actively misleading, so the attribution keys must be absent.
func TestFaultPolicies_Metadata_NoPolicyAttributionForPassthroughErrors(t *testing.T) {
	var order []string
	var md map[string]interface{}
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "notifier", order: &order, sawMetadata: &md},
	}, specs(1))
	responseHeaderCtxWithStatus(ec, 503)
	routerError(ec)

	require.True(t, ec.runFaultPoliciesOnResponse(
		context.Background(), &executor.ResponseExecutionResult{}, originUpstream))

	require.NotNil(t, md)
	_, hasPolicy := md[FaultMetaPolicy]
	assert.False(t, hasPolicy,
		"no policy caused a router-generated error, so attribution must be absent rather than wrong")
}

// The original upstream status must be recoverable when a policy changed it — the gap
// found in the analytics investigation, where the pre-reshape status was unrecoverable.
func TestFaultPolicies_Metadata_CarriesOriginalStatus(t *testing.T) {
	var order []string
	var md map[string]interface{}
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "notifier", order: &order, sawMetadata: &md},
	}, specs(1))
	withRejectedBody(ec, 446) // upstream 200, policy overrode to 446

	require.True(t, ec.runFaultPoliciesOnResponse(
		context.Background(), bodyRejection(446), originGateway))

	require.NotNil(t, md)
	assert.Equal(t, 446, md[FaultMetaStatus])
	assert.Equal(t, 200, md[FaultMetaOriginalStatus],
		"the upstream's pre-override status must be recoverable")
}

// ─── chain wiring ───────────────────────────────────────────────────────────

// Without this the whole pass-through path silently breaks: the response-body phase would not run
// for an API whose own policies do not need the body.
func TestApplyFaultPoliciesBodyRequirement(t *testing.T) {
	withSeq := &registry.PolicyChain{HasFaultPolicies: true}
	ApplyFaultPoliciesBodyRequirement(withSeq)
	assert.True(t, withSeq.RequiresResponseBody,
		"fault policies must force response-body processing on, or it never fires")

	without := &registry.PolicyChain{}
	ApplyFaultPoliciesBodyRequirement(without)
	assert.False(t, without.RequiresResponseBody,
		"an API with no fault policies must not be made to buffer response bodies")

	assert.NotPanics(t, func() { ApplyFaultPoliciesBodyRequirement(nil) })
}

// The three keys added so a fault policy is not blind to the payload or the route.
// bodyBytes/contentType matter most to a HEADER-hook entry, which cannot see the body at
// all — without them it cannot distinguish an empty error from a large one.
func TestFaultPolicies_Metadata_DescribesRouteAndBodyShape(t *testing.T) {
	var order []string
	var md map[string]interface{}
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "notifier", order: &order, sawMetadata: &md},
	}, specs(1))
	withRejectedBody(ec, 446)
	// The real path reaches here with the rejection body already applied to the context
	// by applyResponseModifications, so set one explicitly.
	rejection := []byte(`{"error":"word count violation"}`)
	ec.responseBodyCtx.ResponseBody = &policy.Body{
		Content: rejection, EndOfStream: true, Present: true,
	}

	require.True(t, ec.runFaultPoliciesOnResponse(
		context.Background(), bodyRejection(446), originGateway))

	require.NotNil(t, md)
	assert.Equal(t, "test-route", md[FaultMetaRoute],
		"the matched route lets a fault event be correlated to deployed config, not just a path")
	assert.Equal(t, len(rejection), md[FaultMetaBodyBytes],
		"the error body's SIZE is reported")
	assert.Equal(t, "application/json", md[FaultMetaContentType])
}

// Size and type only — never the content. For a guardrail rejection the body is precisely
// what the guardrail existed to stop from leaving, so it must not be reachable through
// metadata by a policy that would forward it.
func TestFaultPolicies_Metadata_NeverCarriesTheErrorBody(t *testing.T) {
	var order []string
	var md map[string]interface{}
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "notifier", order: &order, sawMetadata: &md},
	}, specs(1))
	withRejectedBody(ec, 446)
	ec.responseBodyCtx.ResponseBody = &policy.Body{
		Content:     []byte(`{"blocked":"secret content the guardrail stopped"}`),
		EndOfStream: true, Present: true,
	}

	require.True(t, ec.runFaultPoliciesOnResponse(
		context.Background(), bodyRejection(446), originGateway))

	for key, value := range md {
		if s, ok := value.(string); ok {
			assert.NotContains(t, s, "secret content",
				"metadata key %q leaked error-body content", key)
		}
	}
}

// A bodyless error must report zero rather than omitting the key, so a policy can branch
// on "no payload" instead of on a missing entry.
func TestFaultPolicies_Metadata_ZeroBytesForBodylessError(t *testing.T) {
	var order []string
	var md map[string]interface{}
	ec := faultExecCtx(t, []policy.Policy{
		&bodyHookFaultPolicy{name: "notifier", order: &order, sawMetadata: &md},
	}, specs(1))

	// A rejection with no body at all.
	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 401})

	require.NotNil(t, md)
	assert.Equal(t, 0, md[FaultMetaBodyBytes])
}

// A short-circuit is NOT the same thing as an error. Several shipped policies end the
// chain with a perfectly successful ImmediateResponse, and the fault policies must not
// treat any of them as a failure. The statuses below are the ones the catalogue actually
// produces:
//
//	200  semantic-cache      — a cache hit answers without touching the upstream
//	200  mcp-auth            — an OAuth metadata document served directly
//	204  cors                — a preflight response
//	3xx  redirect            — an operator-configured redirect
//	any  respond             — whatever status the operator configured
//	any  interceptor-service — whatever the interceptor returned, defaulting to 200
//
// Each declares IsFault: false, and this asserts the explicit form is honoured rather than
// ignored.
func TestFaultPolicies_NonErrorShortCircuitsOptOut(t *testing.T) {
	for _, status := range []int{200, 201, 204, 301, 302, 304, 307, 399} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var order []string
			ec := faultExecCtx(t,
				[]policy.Policy{&faultRecorderPolicy{name: "notify", order: &order, setHeader: "x-fault"}},
				specs(1))

			out := ec.handleRejection(t, policy.ImmediateResponse{
				StatusCode: status,
				Headers:    map[string]string{"content-type": "application/json"},
				Body:       []byte(`{"cached":true}`),
			})

			assert.Empty(t, order,
				"a short-circuit the policy says is not a failure must not run the fault chain")
			assert.Equal(t, status, out.StatusCode, "the response must pass through untouched")
			assert.NotContains(t, out.Headers, "x-fault",
				"no fault policy ran, so no fault header may appear")
			assert.False(t, ec.faultPoliciesRan,
				"run-once must not be consumed by a non-fault, or a later real fault would be skipped")
		})
	}
}

// No migration required, stated as a test because the previous rule needed one and did not
// get it: a policy that has never heard of the fault flow still has its rejections reported,
// because the only thing read is the status it already sets.
//
// The gate is status OR declaration, and each row is a case one rule alone gets wrong.
//
// 401 and 503 undeclared are what a status rule catches and an opt-in misses — the rejections
// of every policy predating the contract, which need no migration to be handled. 200 declared
// is the reverse: a GraphQL-shaped failure that answers 200 with the error in the body, which
// only the policy can know and no status rule could express. The sub-400 undeclared rows are
// the ones that must stay OUT, or a redirect and a cache hit would enter the flow.
func TestFaultPolicies_TheGateIsStatusOrDeclaration(t *testing.T) {
	for _, tc := range []struct {
		status  int
		declare bool
		fault   bool
		what    string
	}{
		{200, false, false, "a cache hit"},
		{204, false, false, "a preflight"},
		{302, false, false, "a redirect"},
		{399, false, false, "the last status below the floor"},
		{401, false, true, "an undeclared auth rejection — the status carries it"},
		{503, false, true, "an undeclared upstream refusal — likewise"},
		{401, true, true, "a declared auth rejection"},
		{503, true, true, "a declared upstream refusal"},
		{200, true, true, "a 200 the policy knows is a failure"},
		{302, true, true, "a declared redirect-shaped failure"},
	} {
		t.Run(fmt.Sprintf("status_%d_declared_%t", tc.status, tc.declare), func(t *testing.T) {
			var order []string
			ec := faultExecCtx(t,
				[]policy.Policy{&faultRecorderPolicy{name: "notify", order: &order}}, specs(1))

			ec.handleRejection(t, policy.ImmediateResponse{StatusCode: tc.status, IsFault: tc.declare})

			if tc.fault {
				assert.Equal(t, []string{"notify"}, order,
					"%s (%d) declared itself a fault", tc.what, tc.status)
			} else {
				assert.Empty(t, order, "%s (%d) did not declare itself a fault", tc.what, tc.status)
			}
		})
	}
}

// handleRejection drives a request-header rejection through the SAME two calls a phase
// handler makes — the constructor, then handleFault only if it reported a fault.
//
// It goes through faultFromRequestHeaders rather than building a fault literal, and that
// matters more than it looks: the status test lives in the constructor, so a helper that
// skipped it would call handleFault for a 200 and assert behaviour no phase handler can
// produce. An earlier version did exactly that.
func (ec *PolicyExecutionContext) handleRejection(
	t *testing.T, imm policy.ImmediateResponse,
) policy.ImmediateResponse {
	t.Helper()
	result := &executor.RequestHeaderExecutionResult{
		ShortCircuited: true,
		FinalAction:    imm,
	}
	if f, isFault := faultFromRequestHeaders(result); isFault {
		ec.handleFault(context.Background(), f)
	}
	// FinalAction is what a phase handler forwards, and writeBack is what replaces it.
	if out, ok := result.FinalAction.(policy.ImmediateResponse); ok {
		return out
	}
	return imm
}

// Chain order is the order the operator authored, and it does not depend on what failed.
//
// The controller builds the list operation-level first, then API-level (pinned in the
// transformer's own tests); the engine must not reorder it, and must not reorder it
// differently for one kind of failure than another. eligibleFaultEntries never looks at the
// fault, so this holds by construction — asserted across both origins because "by
// construction" is exactly the kind of claim that stops being true quietly.
func TestFaultPolicies_ChainOrderIsIndependentOfTheFault(t *testing.T) {
	run := func(t *testing.T, drive func(ec *PolicyExecutionContext)) []string {
		t.Helper()
		var order []string
		ec := faultExecCtx(t, []policy.Policy{
			&faultRecorderPolicy{name: "operation", order: &order},
			&faultRecorderPolicy{name: "api", order: &order},
		}, specs(2))
		drive(ec)
		return order
	}

	gatewayOrder := run(t, func(ec *PolicyExecutionContext) {
		ec.runFaultPoliciesOnRejection(context.Background(),
			policy.ImmediateResponse{StatusCode: 401})
	})
	assert.Equal(t, []string{"operation", "api"}, gatewayOrder,
		"a policy rejection runs the chain in the authored order")

	upstreamOrder := run(t, func(ec *PolicyExecutionContext) {
		ec.responseBodyCtx = &policy.ResponseContext{
			SharedContext:  ec.sharedCtx,
			ResponseStatus: 503,
		}
		ec.runFaultPoliciesOnResponse(context.Background(),
			&executor.ResponseExecutionResult{}, originUpstream)
	})
	assert.Equal(t, []string{"operation", "api"}, upstreamOrder,
		"and so does an upstream failure — the order is the chain's, not the fault's")
}
