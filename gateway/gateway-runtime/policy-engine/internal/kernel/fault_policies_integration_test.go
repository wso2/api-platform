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
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// These tests drive the REAL phase handler rather than calling the fault-policies
// function directly. That matters: a unit test on a function the phase handler never
// invokes would pass while the feature is inert on the wire — the failure mode that let
// an earlier slice ship a dead code path with four green tests.

// responseGuardrailPolicy stands in for word-count-guardrail's response path: it rejects
// by overriding the status through DownstreamResponseModifications, which does NOT
// short-circuit the chain.
type responseGuardrailPolicy struct{ status int }

func (p *responseGuardrailPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		ResponseHeaderMode: policy.HeaderModeSkip, // the real guardrail skips headers
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// IsFault: true is what a real response guardrail must now set. The status alone no longer
// implies a rejection — a policy relabelling the backend's error sets one too — so a guardrail
// declares the rejection outright.
func (p *responseGuardrailPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	s := p.status
	return policy.DownstreamResponseModifications{
		StatusCode:   &s,
		IsFault:      true,
		Body:         []byte(`{"error":"word count violation"}`),
		HeadersToSet: map[string]string{"content-type": "application/json"},
	}
}

// passthroughBodyPolicy consumes the body phase without touching the status, so the status-override
// gate (a policy must have overridden the status) can be tested as being closed.
type passthroughBodyPolicy struct{}

func (passthroughBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

func (passthroughBodyPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

// faultIntegrationCtx wires a chain with a normal response-body policy plus a fault
// sequence, then walks the response header and body phases as Envoy would.
func faultIntegrationCtx(
	t *testing.T,
	bodyPolicy policy.Policy,
	faultPolicies []policy.Policy,
	upstreamStatus string,
) *PolicyExecutionContext {
	t.Helper()

	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	// These fixtures exist to drive an UPSTREAM response through the engine, which the
	// fault flow only handles when the deployment opts in — so they opt in. The disabled
	// default is covered separately, by TestHandleUpstreamFaults_DisabledKeepsLegacyBehaviour.
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20, nil,
		WithHandleUpstreamFaults(true))

	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
		Policies:             []policy.Policy{bodyPolicy},
		PolicySpecs:          []policy.PolicySpec{{Name: "guardrail", Version: "v1", Enabled: true}},
		FaultPolicies:        faultPolicies,
		FaultPolicySpecs:     specs(len(faultPolicies)),
		HasFaultPolicies:     len(faultPolicies) > 0,
	}

	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/chat/v1/completions")},
				{Key: ":method", RawValue: []byte("POST")},
			},
		},
	}, RouteMetadata{})

	_, err := ec.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte(upstreamStatus)},
				{Key: "content-type", RawValue: []byte("application/json")},
			},
		},
		EndOfStream: false,
	})
	require.NoError(t, err)
	return ec
}

// headerOpsFromBodyResponse collects the :status and header mutations the translator
// emitted, i.e. what Envoy will actually apply.
func headerOpsFromBodyResponse(t *testing.T, resp *extprocv3.ProcessingResponse) map[string]string {
	t.Helper()
	out := map[string]string{}
	body := resp.GetResponseBody()
	require.NotNil(t, body, "expected a ResponseBody reply")
	for _, h := range body.GetResponse().GetHeaderMutation().GetSetHeaders() {
		out[h.GetHeader().GetKey()] = string(h.GetHeader().GetRawValue())
	}
	return out
}

// THE regression test for this fix: a response guardrail rejects, and the fault policies
// runs — through the real phase handler, not a direct call.
func TestFaultPoliciesIntegration_ResponseBody_FiresFromRealPhaseHandler(t *testing.T) {
	var order []string
	ec := faultIntegrationCtx(t,
		&responseGuardrailPolicy{status: 446},
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &order, setHeader: "x-fault-notified"}},
		"200",
	)

	resp, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"completion":"a very long answer"}`),
		EndOfStream: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"notifier"}, order,
		"the fault policy must run when a response-body policy rejects the response")
	assert.True(t, ec.faultPoliciesRan)

	// And its header mutation must reach the wire alongside the guardrail's rejection.
	ops := headerOpsFromBodyResponse(t, resp)
	assert.Equal(t, "notifier", ops["x-fault-notified"],
		"the fault policy's header must be emitted to Envoy, not just applied in memory")
	assert.Equal(t, "446", ops[":status"],
		"the guardrail's rejection status must survive a notify-only fault policies")
}

// A pass-through error status enters the fault flow exactly ONCE, through the real body-phase
// handler.
//
// This test used to assert the opposite. It could, because the fixture sets no provenance and
// an unknown source declined — so "no policy rejected this" and "the engine could not tell who
// produced it" both pointed the same way and the second was doing the work. Now that neither
// is a reason to decline, the property left worth holding is the one that was always the real
// risk here: the pass-through path and the rejection path share this handler, and an error must
// not be reported twice because both looked at it.
func TestFaultPoliciesIntegration_ResponseBody_PassthroughErrorIsReportedExactlyOnce(t *testing.T) {
	var order []string
	ec := faultIntegrationCtx(t,
		passthroughBodyPolicy{},
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &order}},
		"404", // an error status the upstream produced, passed straight through
	)

	_, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"error":"not found"}`),
		EndOfStream: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"notifier"}, order,
		"an upstream error status is a failure the operator may want to hear about")
	assert.True(t, ec.faultPoliciesRan,
		"having run, the guard must be consumed so a later trigger cannot report it again")
}

// A successful response with fault policies configured must cost nothing on the fault
// path, through the real handler.
func TestFaultPoliciesIntegration_ResponseBody_NeverRunsOnSuccess(t *testing.T) {
	var order []string
	ec := faultIntegrationCtx(t,
		passthroughBodyPolicy{},
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &order}},
		"200",
	)

	_, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"ok":true}`),
		EndOfStream: true,
	})
	require.NoError(t, err)

	assert.Empty(t, order, "a successful response must never enter the fault policies")
	assert.False(t, ec.faultPoliciesRan)
}

// A fault policy replacing the response must override the guardrail's own rejection on the
// wire — status and body both.
//
// It arrives as a body mutation with a `:status` header op rather than an ImmediateResponse.
// The status is still mutable at this point because ext_proc holds the response headers until
// the buffered body phase completes, which is the same mechanism the guardrail's own 200→446
// override uses two tests above. What the replacement does NOT do is discard the error's
// other headers, which an ImmediateResponse's fresh header set would do unconditionally.
func TestFaultPoliciesIntegration_ResponseBody_ReplacementReachesTheWire(t *testing.T) {
	var order []string
	ec := faultIntegrationCtx(t,
		&responseGuardrailPolicy{status: 446},
		[]policy.Policy{&faultReplacerPolicy{order: &order}},
		"200",
	)

	resp, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"completion":"a very long answer"}`),
		EndOfStream: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"replacer"}, order)

	body := resp.GetResponseBody()
	require.NotNil(t, body, "expected a ResponseBody reply carrying the replacement")
	assert.JSONEq(t, `{"replaced":"by fault policies"}`,
		string(body.GetResponse().GetBodyMutation().GetBody()))

	ops := headerOpsFromBodyResponse(t, resp)
	assert.Equal(t, "500", ops[":status"],
		"the replacing policy's status must override the guardrail's 446")
}

// ─── pass-through in the body phase, driven through the real handler ────────

// faultIntegrationCtxNoBodyPolicy mirrors the common case the move to the body phase put at risk: an API
// with NO response-body policy of its own, carrying only fault policies. The chain flag
// is set the way the real chain builder sets it (ApplyFaultPoliciesBodyRequirement).
// Provenance values as Envoy reports them, used by the integration tests below.
const (
	routerCodeDetails  = "upstream_reset_before_response_started{remote_connection_failure}"
	backendCodeDetails = "via_upstream"
)

func faultIntegrationCtxNoBodyPolicy(
	t *testing.T,
	faultPolicies []policy.Policy,
	upstreamStatus string,
	applyRequirement bool,
	// codeDetails is Envoy's account of who produced the response, normally extracted
	// from the response-headers ProcessingRequest attributes. Set directly here because
	// these tests drive the phase handlers rather than the ext_proc message loop; the
	// extraction itself is covered by the unit tests.
	codeDetails string,
) *PolicyExecutionContext {
	t.Helper()

	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	// These fixtures exist to drive an UPSTREAM response through the engine, which the
	// fault flow only handles when the deployment opts in — so they opt in. The disabled
	// default is covered separately, by TestHandleUpstreamFaults_DisabledKeepsLegacyBehaviour.
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20, nil,
		WithHandleUpstreamFaults(true))

	chain := &registry.PolicyChain{
		Policies:         nil, // the API has no policies of its own
		PolicySpecs:      nil,
		FaultPolicies:    faultPolicies,
		FaultPolicySpecs: specs(len(faultPolicies)),
		HasFaultPolicies: len(faultPolicies) > 0,
	}
	if applyRequirement {
		ApplyFaultPoliciesBodyRequirement(chain)
	}

	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/orders/v1/list")},
				{Key: ":method", RawValue: []byte("GET")},
			},
		},
	}, RouteMetadata{})

	_, err := ec.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte(upstreamStatus)},
				{Key: "content-type", RawValue: []byte("application/json")},
			},
		},
		EndOfStream: false,
	})
	require.NoError(t, err)
	ec.responseCodeDetails = codeDetails
	return ec
}

// A router-produced 5xx must reach the fault policies after the move to the body phase.
// This is the source-B coverage that previously ran at the header phase.
func TestFaultPoliciesIntegration_Upstream_FiresFromBodyPhase(t *testing.T) {
	var order []string
	ec := faultIntegrationCtxNoBodyPolicy(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &order, setHeader: "x-fault-notified"}},
		"503", true, routerCodeDetails,
	)

	resp, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"error":"upstream exploded"}`),
		EndOfStream: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"notifier"}, order,
		"a router-produced 5xx must fire the fault chain from the body phase")
	ops := headerOpsFromBodyResponse(t, resp)
	assert.Equal(t, "notifier", ops["x-fault-notified"],
		"and the mutation must reach the wire")
}

// THE dependency the move to the body phase introduces. Without ApplyFaultPoliciesBodyRequirement the
// body phase does not execute policies, so the fault chain silently never fires — the exact
// regression this test exists to catch.
func TestFaultPoliciesIntegration_Upstream_RequiresBodyFlag(t *testing.T) {
	var withFlag, withoutFlag []string

	ecOn := faultIntegrationCtxNoBodyPolicy(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &withFlag}}, "503", true, routerCodeDetails)
	_, err := ecOn.processResponseBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"e":1}`), EndOfStream: true})
	require.NoError(t, err)

	ecOff := faultIntegrationCtxNoBodyPolicy(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &withoutFlag}}, "503", false, routerCodeDetails)
	_, err = ecOff.processResponseBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"e":1}`), EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"notifier"}, withFlag, "with the flag the fault chain fires")
	assert.Empty(t, withoutFlag,
		"without the flag the body phase skips policy execution entirely — which is why the chain builder must set it")
}

// A body-hook fault policy (interceptor-service's shape) must receive the BACKEND's error
// body for a pass-through error. This is what the move to the body phase bought.
func TestFaultPoliciesIntegration_Upstream_BodyHookReceivesBackendError(t *testing.T) {
	var order []string
	var saw []byte
	ec := faultIntegrationCtxNoBodyPolicy(t,
		[]policy.Policy{&bodyHookFaultPolicy{name: "interceptor", order: &order, sawBody: &saw}},
		"503", true, routerCodeDetails,
	)

	_, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"error":"upstream exploded"}`),
		EndOfStream: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"interceptor"}, order)
	assert.JSONEq(t, `{"error":"upstream exploded"}`, string(saw),
		"a body-hook fault policy must receive the backend's error body — impossible at the header phase")
}

// An error the BACKEND produced enters the fault flow, and arrives labelled as the backend's —
// exercised through the real body-phase handler.
//
// Both halves matter. The flow running is what makes upstream failures actionable at all; the
// label is what keeps that from being indiscriminate, since it is what an execution condition
// reads to narrow back down.
func TestFaultPoliciesIntegration_Upstream_FiresForABackendErrorAndLabelsIt(t *testing.T) {
	var order []string
	ec := faultIntegrationCtxNoBodyPolicy(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &order}}, "500", true,
		backendCodeDetails)

	_, err := ec.processResponseBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"error":"backend exploded"}`), EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"notifier"}, order,
		"a backend 500 is a failure the gateway's operator may need to act on")
	assert.Equal(t, sourceBackend, ec.faultSource)
	assert.True(t, ec.faultBodyAuthored,
		"the backend wrote this body, so nothing may render over it")
}

// A successful response must not fire the fault chain, even though the body phase now always
// runs for an API carrying one.
func TestFaultPoliciesIntegration_Upstream_NeverFiresOnSuccess(t *testing.T) {
	var order []string
	ec := faultIntegrationCtxNoBodyPolicy(t,
		[]policy.Policy{&faultRecorderPolicy{name: "notifier", order: &order}}, "200", true, routerCodeDetails)

	_, err := ec.processResponseBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"ok":true}`), EndOfStream: true})
	require.NoError(t, err)

	assert.Empty(t, order)
	assert.False(t, ec.faultPoliciesRan)
}

// responsePhaseRecorder records which response hooks the engine called.
type responsePhaseRecorder struct{ calls *[]string }

func (p *responsePhaseRecorder) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

func (p *responsePhaseRecorder) OnResponseHeaders(_ context.Context, _ *policy.ResponseHeaderContext,
	_ map[string]interface{}) policy.ResponseHeaderAction {
	*p.calls = append(*p.calls, "headers")
	return policy.DownstreamResponseHeaderModifications{}
}

func (p *responsePhaseRecorder) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	*p.calls = append(*p.calls, "body")
	return policy.DownstreamResponseModifications{}
}

// upstreamErrorThroughPhases drives a backend 503 through the real response-header and
// response-body handlers, with one response policy and one fault policy on the route.
func upstreamErrorThroughPhases(t *testing.T, handle bool, contentType string) (*PolicyExecutionContext, *[]string, *[]string) {
	t.Helper()
	calls, faultOrder := &[]string{}, &[]string{}

	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20, nil,
		WithHandleUpstreamFaults(handle))

	faultPolicies := []policy.Policy{&faultRecorderPolicy{name: "notifier", order: faultOrder}}
	chain := &registry.PolicyChain{
		RequiresResponseBody:      true,
		SupportsResponseStreaming: true,
		Policies:                  []policy.Policy{&responsePhaseRecorder{calls: calls}},
		PolicySpecs:               []policy.PolicySpec{{Name: "response-mediator", Version: "v1", Enabled: true}},
		FaultPolicies:             faultPolicies,
		FaultPolicySpecs:          specs(len(faultPolicies)),
		HasFaultPolicies:          true,
	}
	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/orders/v1/list")},
				{Key: ":method", RawValue: []byte("GET")},
			},
		},
	}, RouteMetadata{})

	_, err := ec.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("503")},
				{Key: "content-type", RawValue: []byte(contentType)},
			},
		},
	})
	require.NoError(t, err)
	ec.responseCodeDetails = backendCodeDetails
	return ec, calls, faultOrder
}

// With handle_upstream_faults on, an upstream error runs the fault policies and NO response
// policy, in either response phase.
func TestHandleUpstreamFaults_Enabled_SkipsEveryResponsePhasePolicy(t *testing.T) {
	ec, calls, faultOrder := upstreamErrorThroughPhases(t, true, "application/json")

	_, err := ec.processResponseBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"error":"backend exploded"}`), EndOfStream: true})
	require.NoError(t, err)

	assert.Empty(t, *calls, "neither OnResponseHeaders nor OnResponseBody may run")
	assert.Equal(t, []string{"notifier"}, *faultOrder, "the fault policies handle it instead")
}

// A streamed upstream error is buffered, so it reaches the fault policies whole rather than
// running the response policies chunk by chunk.
func TestHandleUpstreamFaults_Enabled_BuffersAStreamedError(t *testing.T) {
	ec, calls, _ := upstreamErrorThroughPhases(t, true, "text/event-stream")

	assert.False(t, ec.isStreamingResponse, "the error must not take the streaming path")
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, ec.getModeOverride().ResponseBodyMode)
	assert.Empty(t, *calls)
}

// With it off, nothing changes: the response policies run over the error in both phases and
// the fault policies do not.
func TestHandleUpstreamFaults_Disabled_RunsTheResponsePolicies(t *testing.T) {
	ec, calls, faultOrder := upstreamErrorThroughPhases(t, false, "application/json")

	_, err := ec.processResponseBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"error":"backend exploded"}`), EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"headers", "body"}, *calls)
	assert.Empty(t, *faultOrder)

	streamed, _, _ := upstreamErrorThroughPhases(t, false, "text/event-stream")
	assert.True(t, streamed.isStreamingResponse, "a streamed error keeps streaming")
}
