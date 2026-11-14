/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package kernel

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// A request where no attempt ever reached the upstream position unwinds nothing. This is the
// ordinary case for every API without failover, so it must cost a lookup and return — not error,
// and not reach for a chain that was never built (FR-037h).
func TestUnwindServingUpstreamChain_NoAttemptRecorded(t *testing.T) {
	server := newTestServerWithStore(t, nil)
	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-no-attempt"
	ec.attemptKey = "req-no-attempt"

	if _, err := ec.unwindServingUpstreamChain(context.Background()); err != nil {
		t.Fatalf("unwinding a request with no recorded attempt must be a no-op, got %v", err)
	}
}

// A recorded serving cluster whose chain was never published also unwinds nothing. Reachable
// when a config push retires the cluster between the request and its response.
func TestUnwindServingUpstreamChain_RecordedClusterWithNoChain(t *testing.T) {
	server := newTestServerWithStore(t, nil)
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", "cluster_gone")

	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1" // the stores key on the attempt key, not the request id

	if _, err := ec.unwindServingUpstreamChain(context.Background()); err != nil {
		t.Fatalf("a cluster with no published chain must unwind nothing, got %v", err)
	}
}

// The cluster unwound is the one that SERVED, not the one that was tried first. A failover
// records every attempt; the last write is the attempt whose response the client receives, and
// unwinding the first would reverse a translation that never ran.
func TestUnwindServingUpstreamChain_UsesTheServingAttemptNotTheFirst(t *testing.T) {
	server := newTestServerWithStore(t, nil)
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", "cluster_openai")    // attempt 1, failed
	server.baselines.RememberServingCluster("req-1", "cluster_anthropic") // attempt 2, served

	got, ok := server.baselines.ServingCluster("req-1")
	if !ok || got != "cluster_anthropic" {
		t.Fatalf("serving cluster = %q (ok=%v), want the attempt that answered", got, ok)
	}

	// And unwinding resolves against that cluster: neither chain is published here, so the
	// assertion is that it looks up the serving one and returns cleanly rather than the first.
	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1" // the stores key on the attempt key, not the request id
	if _, err := ec.unwindServingUpstreamChain(context.Background()); err != nil {
		t.Fatalf("unwind returned %v", err)
	}
}

// unwoundHeaderPolicy sets one response header, so a test can tell whether the unwind's work
// survived as far as the caller.
type unwoundHeaderPolicy struct{}

func (unwoundHeaderPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (unwoundHeaderPolicy) OnResponseHeaders(_ context.Context, _ *policy.ResponseHeaderContext,
	_ map[string]interface{}) policy.ResponseHeaderAction {
	return policy.DownstreamResponseHeaderModifications{
		HeadersToSet: map[string]string{"x-unwound-by": "live-vendor"},
	}
}

// The unwind must RETURN what it executed. A policy's header mutation only reaches the client
// through TranslateResponseHeaderActions, which builds its mutation list from these results and
// never re-reads responseHeaderCtx — so a discarded result is a mutation silently dropped. This
// shipped once: the chain ran, the debug log said so, and the header never appeared on the
// response. Asserting "no error" cannot catch it; only the results can.
func TestUnwindServingUpstreamChain_ReturnsTheExecutedResults(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_live-vendor"

	// newTestServerWithStore builds its executor with a nil tracer, which panics as soon as
	// a policy actually executes. This test runs one, so it needs a real no-op tracer.
	server := newTestServerWithStore(t, nil)
	server.executor = executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer("test"))
	server.kernel.RegisterRoute(UpstreamChainKey(cluster), &registry.PolicyChain{
		Policies: []policy.Policy{unwoundHeaderPolicy{}},
		// Enabled MUST be set. It defaults to false, and the executor emits a result with
		// Skipped=true for a disabled policy without ever calling OnResponseHeaders — so a
		// test that only counts results passes on a chain that executed nothing.
		PolicySpecs: []policy.PolicySpec{{Name: "set-headers", Version: "v1", Enabled: true}},
	})
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", cluster)

	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1" // the stores key on the attempt key, not the request id
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
	}

	result, err := ec.unwindServingUpstreamChain(context.Background())
	if err != nil {
		t.Fatalf("unwind returned %v", err)
	}
	if result == nil {
		t.Fatal("unwind returned no result: the executed chain's mutations cannot reach the client")
	}
	if len(result.Results) != 1 {
		t.Fatalf("unwind returned %d results, want 1 — the chain's single policy ran", len(result.Results))
	}
	// Counting results is not enough: a skipped policy produces one too.
	if result.Results[0].Skipped {
		t.Fatal("the policy was skipped, so the chain executed nothing and the result is empty")
	}
	mods, ok := result.Results[0].Action.(policy.DownstreamResponseHeaderModifications)
	if !ok {
		t.Fatalf("action is %T, want the policy's header modifications", result.Results[0].Action)
	}
	if got := mods.HeadersToSet["x-unwound-by"]; got != "live-vendor" {
		t.Fatalf("unwound header = %q, want %q — the mutation the client must receive",
			got, "live-vendor")
	}
}

// bodyUnwindPolicy rewrites the response body, which is what a provider transformer does.
type bodyUnwindPolicy struct{}

func (bodyUnwindPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

func (bodyUnwindPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{Body: []byte(`{"choices":[]}`)}
}

// The BODY half of the unwind is the half that matters for a cross-vendor failover.
//
// A provider transformer converts the vendor's JSON payload; headers cannot turn `content_block`
// into `choices[]`. The upstream filter is request-only and the downstream body phase runs the
// ROUTE chain, so without this the serving provider's reverse translation never runs and the
// client receives the fallback vendor's shape.
func TestUnwindServingUpstreamChainBody_RunsTheServingChainsBodyHooks(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_live-vendor"

	server := newTestServerWithStore(t, nil)
	server.executor = executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer("test"))
	server.kernel.RegisterRoute(UpstreamChainKey(cluster), &registry.PolicyChain{
		Policies:    []policy.Policy{bodyUnwindPolicy{}},
		PolicySpecs: []policy.PolicySpec{{Name: "openai-to-anthropic-transformer", Version: "v0", Enabled: true}},
	})
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", cluster)

	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1"
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:  ec.sharedCtx,
		ResponseStatus: 200,
		ResponseBody:   &policy.Body{Content: []byte(`{"content_block":{}}`), Present: true},
	}

	result, err := ec.unwindServingUpstreamChainBody(context.Background())
	if err != nil {
		t.Fatalf("body unwind returned %v", err)
	}
	if result == nil || len(result.Results) != 1 {
		t.Fatal("the serving chain's body hooks did not run; a cross-vendor response would " +
			"reach the client in the fallback vendor's shape")
	}
	if result.Results[0].Skipped {
		t.Fatal("the transformer was skipped, so no conversion happened")
	}
}

// refusingCELEvaluator answers every execution condition with "do not run".
//
// A nil evaluator is not usable for this: the executor only consults the condition when it HAS
// an evaluator, so a chain with a nil one runs every policy and a test built on it would pass
// whatever flag the unwind passes.
type refusingCELEvaluator struct{}

func (refusingCELEvaluator) EvaluateRequestHeaderCondition(string, *policy.RequestHeaderContext) (bool, error) {
	return false, nil
}
func (refusingCELEvaluator) EvaluateRequestBodyCondition(string, *policy.RequestContext) (bool, error) {
	return false, nil
}
func (refusingCELEvaluator) EvaluateResponseHeaderCondition(string, *policy.ResponseHeaderContext) (bool, error) {
	return false, nil
}
func (refusingCELEvaluator) EvaluateResponseBodyCondition(string, *policy.ResponseContext) (bool, error) {
	return false, nil
}
func (refusingCELEvaluator) EvaluateFaultCondition(string, *policy.FaultContext) (bool, error) {
	return false, nil
}
func (refusingCELEvaluator) EvaluateStreamingRequestCondition(string, *policy.RequestStreamContext) (bool, error) {
	return false, nil
}
func (refusingCELEvaluator) EvaluateStreamingResponseCondition(string, *policy.ResponseStreamContext) (bool, error) {
	return false, nil
}

func refusedSpec(name, version string) policy.PolicySpec {
	condition := "false"
	return policy.PolicySpec{Name: name, Version: version, Enabled: true, ExecutionCondition: &condition}
}

// An upstream policy's execution condition governs BOTH halves of its work.
//
// The executor skips every condition when the chain flag it is handed is false, so passing a
// literal false ran a provider policy whose condition said not to — unwrapping a response the
// request half never wrapped, because the request half honoured the same condition.
func TestUnwindServingUpstreamChain_HonoursTheChainsExecutionConditions(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_live-vendor"

	server := newTestServerWithStore(t, nil)
	server.executor = executor.NewChainExecutor(nil, refusingCELEvaluator{},
		noop.NewTracerProvider().Tracer("test"))
	server.kernel.RegisterRoute(UpstreamChainKey(cluster), &registry.PolicyChain{
		Policies:               []policy.Policy{unwoundHeaderPolicy{}},
		PolicySpecs:            []policy.PolicySpec{refusedSpec("set-headers", "v1")},
		HasExecutionConditions: true,
	})
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", cluster)

	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1"
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
	}

	result, err := ec.unwindServingUpstreamChain(context.Background())
	if err != nil {
		t.Fatalf("unwind returned %v", err)
	}
	if result == nil || len(result.Results) != 1 {
		t.Fatalf("expected one result for the one policy in the chain, got %+v", result)
	}
	if !result.Results[0].Skipped {
		t.Fatal("the policy ran although its execution condition refused it: the unwind is " +
			"ignoring the chain's conditions and undoing work that was never done")
	}
}

// The body half, same rule. Listed separately because the two halves pass the flag at two
// different call sites and the first fix shipped without the second.
func TestUnwindServingUpstreamChainBody_HonoursTheChainsExecutionConditions(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_live-vendor"

	server := newTestServerWithStore(t, nil)
	server.executor = executor.NewChainExecutor(nil, refusingCELEvaluator{},
		noop.NewTracerProvider().Tracer("test"))
	server.kernel.RegisterRoute(UpstreamChainKey(cluster), &registry.PolicyChain{
		Policies:               []policy.Policy{bodyUnwindPolicy{}},
		PolicySpecs:            []policy.PolicySpec{refusedSpec("openai-to-anthropic-transformer", "v0")},
		HasExecutionConditions: true,
	})
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", cluster)

	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1"
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:  ec.sharedCtx,
		ResponseStatus: 200,
		ResponseBody:   &policy.Body{Content: []byte(`{"content_block":{}}`), Present: true},
	}

	result, err := ec.unwindServingUpstreamChainBody(context.Background())
	if err != nil {
		t.Fatalf("body unwind returned %v", err)
	}
	if result == nil || len(result.Results) != 1 {
		t.Fatalf("expected one result for the one policy in the chain, got %+v", result)
	}
	if !result.Results[0].Skipped {
		t.Fatal("the transformer ran although its execution condition refused it")
	}
}

// The gate that decides whether Envoy delivers the response body read the ROUTE chain alone.
// For the case this feature exists for the route chain often has no body policy at all — the
// only body work is the provider's transformer, which lives on the upstream chain — so the gate
// said "no body", Envoy skipped the callback, and the body unwind had nothing to run on.
func TestNeedsResponseBody_CountsTheServingUpstreamChain(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_live-vendor"

	server := newTestServerWithStore(t, nil)
	server.kernel.RegisterRoute(UpstreamChainKey(cluster), &registry.PolicyChain{
		Policies:             []policy.Policy{bodyUnwindPolicy{}},
		PolicySpecs:          []policy.PolicySpec{{Name: "openai-to-anthropic-transformer", Version: "v0", Enabled: true}},
		RequiresResponseBody: true,
	})
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})

	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-1"
	ec.attemptKey = "req-1"
	// The route's own chain wants nothing: this is the configuration the bug hid in.
	ec.policyChain = &registry.PolicyChain{}

	// Control: with no attempt recorded there is no serving cluster, so nothing claims the
	// body. This is also the request-phase answer, before any attempt has run.
	if ec.needsResponseBody() {
		t.Fatal("a request with no serving cluster must not ask for the response body")
	}

	server.baselines.RememberServingCluster("req-1", cluster)

	if !ec.needsResponseBody() {
		t.Fatal("the serving attempt's chain needs the body and the route's does not; " +
			"skipping it leaves the provider's reverse translation with nothing to convert")
	}

	// Streaming is excluded deliberately: a reverse transform is a whole-document conversion
	// and the streaming body path does not unwind at all, so asking for the body there would
	// buffer chunks for a chain that cannot use them.
	ec.isStreamingResponse = true
	if ec.needsResponseBody() {
		t.Fatal("a streamed response must not be buffered for an unwind that cannot run on it")
	}
}

// shortCircuitingBodyPolicy replaces the response outright, as a guardrail on a provider's own
// chain would.
type shortCircuitingBodyPolicy struct{}

func (shortCircuitingBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

func (shortCircuitingBodyPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	return policy.ImmediateResponse{
		StatusCode: 451,
		Body:       []byte(`{"error":"blocked by the provider chain"}`),
	}
}

// A short circuit in the unwound BODY chain has to carry, exactly as the header half already
// carries one.
//
// The body merge took the unwound chain's Results and nothing else, so an upstream body policy
// that returned an ImmediateResponse had its replacement discarded and the route's own body
// result translated in its place: the policy ran, decided to replace the response, and the
// decision went nowhere.
func TestProcessResponseBody_CarriesTheUpstreamChainsShortCircuit(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_live-vendor"

	server := newTestServerWithStore(t, nil)
	server.executor = executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer("test"))
	server.kernel.RegisterRoute(UpstreamChainKey(cluster), &registry.PolicyChain{
		Policies:             []policy.Policy{shortCircuitingBodyPolicy{}},
		PolicySpecs:          []policy.PolicySpec{{Name: "guardrail", Version: "v1", Enabled: true}},
		RequiresResponseBody: true,
	})
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	server.baselines.RememberServingCluster("req-1", cluster)

	// The ROUTE chain runs and does not short-circuit, which is the case the merge got wrong:
	// with the route short-circuiting too, its own decision would have been translated and the
	// bug would be invisible.
	ec := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{
		RequiresResponseBody: true,
	})
	// The real phase handler reads the shared context, so the request phase has to have run.
	// It also assigns the request id and attempt key, so they are set AFTER it, not before.
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: ":path", RawValue: []byte("/v1/chat")},
				{Key: ":method", RawValue: []byte("POST")},
			},
		},
	}, RouteMetadata{})
	ec.requestID = "req-1"
	ec.attemptKey = "req-1"
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
	}
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:   ec.sharedCtx,
		ResponseStatus:  200,
		ResponseHeaders: ec.responseHeaderCtx.ResponseHeaders,
	}

	resp, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body:        []byte(`{"content_block":{}}`),
		EndOfStream: true,
	})
	if err != nil {
		t.Fatalf("processResponseBody returned %v", err)
	}
	immediate := resp.GetImmediateResponse()
	if immediate == nil {
		t.Fatalf("the upstream chain replaced the response and the reply carries no immediate "+
			"response: the short circuit was dropped in the merge (reply = %T)", resp.GetResponse())
	}
	if got := immediate.GetStatus().GetCode(); int(got) != 451 {
		t.Fatalf("immediate response status = %d, want 451 — the provider chain's own replacement", got)
	}
}
