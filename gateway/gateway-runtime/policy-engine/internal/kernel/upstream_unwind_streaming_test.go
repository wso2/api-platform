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
	"strings"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// translatingStreamPolicy stands in for a provider transformer's streaming half: it rewrites
// every chunk, the way openai-to-anthropic-transformer rewrites Anthropic SSE events into
// OpenAI ones in OnResponseBodyChunk.
type translatingStreamPolicy struct{ holdUntil string }

func (translatingStreamPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}
}

func (translatingStreamPolicy) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext,
	chunk *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	return policy.ForwardResponseChunk{Body: []byte("openai:" + string(chunk.Chunk))}
}

// NeedsMoreResponseData holds a PARTIAL event back, which is the behaviour that makes the
// serving chain's answer load-bearing in the flush decision.
func (p translatingStreamPolicy) NeedsMoreResponseData(accumulated []byte) bool {
	return p.holdUntil != "" && !strings.Contains(string(accumulated), p.holdUntil)
}

// StreamingResponsePolicy embeds ResponsePolicy as the buffered fallback, so a streaming policy
// must offer both halves — exactly as the real transformer does.
func (translatingStreamPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{Body: []byte(`{"choices":[]}`)}
}

// routeChunkRecorder is the ROUTE chain, recording what it was handed.
type routeChunkRecorder struct{ saw []string }

func (*routeChunkRecorder) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}
}

func (r *routeChunkRecorder) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext,
	chunk *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	r.saw = append(r.saw, string(chunk.Chunk))
	return policy.ForwardResponseChunk{}
}

func (*routeChunkRecorder) NeedsMoreResponseData([]byte) bool { return false }

func (*routeChunkRecorder) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

// bufferedOnlyStreamPolicy declares it needs the response body but implements no streaming
// half, which is what forces a chain to be buffered.
type bufferedOnlyStreamPolicy struct{}

func (bufferedOnlyStreamPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

func (bufferedOnlyStreamPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext,
	_ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}

// streamingUnwindCtx wires a route chain and a SERVING upstream chain onto one streaming
// response, the shape a cross-vendor LLM answer actually arrives in.
func streamingUnwindCtx(t *testing.T, route policy.Policy, serving *registry.PolicyChain) *PolicyExecutionContext {
	t.Helper()
	const cluster = "upstream_LlmProxy_abc_vendor-y"

	kernel := NewKernel()
	server := NewExternalProcessorServer(kernel, newTestExecutor(), config.TracingConfig{}, "",
		testMaxDecompressedBytes, testMaxDecompressedBytes, nil)
	if serving != nil {
		kernel.RegisterRoute(UpstreamChainKey(cluster), serving)
	}

	routeChain := &registry.PolicyChain{SupportsResponseStreaming: true}
	if route != nil {
		routeChain.RequiresResponseBody = true
		routeChain.Policies = []policy.Policy{route}
		routeChain.PolicySpecs = []policy.PolicySpec{{Name: "route-policy", Enabled: true}}
	}

	ec := newPolicyExecutionContext(server, "test-route", routeChain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	ec.requestID = "req-1"
	ec.attemptKey = "req-1"
	server.baselines.Remember("req-1", requestBaseline{Path: "/v1/chat"})
	if serving != nil {
		server.baselines.RememberServingCluster("req-1", cluster)
	}
	ec.buildResponseContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("text/event-stream")},
	}}})
	ec.isStreamingResponse = true
	return ec
}

func servingStreamChain(p policy.Policy) *registry.PolicyChain {
	return &registry.PolicyChain{
		Policies:                  []policy.Policy{p},
		PolicySpecs:               []policy.PolicySpec{{Name: "openai-to-anthropic-transformer", Version: "v0", Enabled: true}},
		RequiresResponseBody:      true,
		SupportsResponseStreaming: p.Mode().ResponseBodyMode == policy.BodyModeStream,
	}
}

// FR-037i. THE test for the streaming half: the serving chain runs on each chunk, and the route
// chain is handed what IT produced.
//
// Before this existed the streaming path called the route chain alone. A cross-vendor streamed
// answer reached the client as raw upstream SSE while every attempt-level log said the provider
// chain had been installed — the request half had run, and only the response half was missing.
func TestProcessStreamingResponseBody_UnwindsTheServingChainAndFeedsItsOutputOnward(t *testing.T) {
	route := &routeChunkRecorder{}
	ec := streamingUnwindCtx(t, route, servingStreamChain(translatingStreamPolicy{}))

	resp, err := ec.processStreamingResponseBody(context.Background(), &extprocv3.HttpBody{
		Body: []byte("anthropic-event"), EndOfStream: true,
	})
	if err != nil {
		t.Fatalf("processStreamingResponseBody returned %v", err)
	}
	if resp == nil {
		t.Fatal("no processing response")
	}

	if len(route.saw) != 1 {
		t.Fatalf("the route chain ran %d times, want 1", len(route.saw))
	}
	// The ONLY assertion that separates "the unwind ran" from "the unwind ran and its output
	// was thrown away" — the same failure the header half already had once, where every
	// policy executed, every log line agreed, and the mutation never reached the client.
	if route.saw[0] != "openai:anthropic-event" {
		t.Fatalf("the route chain was handed %q; it must see the chunk the serving chain "+
			"translated, not the bytes the provider sent", route.saw[0])
	}
}

// The serving chain's NeedsMoreResponseData decides the flush too. A transformer holds back a
// partial SSE event precisely so OnResponseBodyChunk never sees a fragment of one; asking only
// the route chain hands it exactly that fragment.
func TestAnyPolicyNeedsMoreResponseData_AsksTheServingChain(t *testing.T) {
	// Route chain wants nothing held back — the common case, and what made this invisible.
	ec := streamingUnwindCtx(t, &routeChunkRecorder{},
		servingStreamChain(translatingStreamPolicy{holdUntil: "\n\n"}))

	if !ec.anyPolicyNeedsMoreResponseData([]byte("data: par")) {
		t.Fatal("a partial event was flushed: the serving chain's NeedsMoreResponseData was " +
			"never consulted, so the transformer receives a fragment of an SSE event")
	}
	if ec.anyPolicyNeedsMoreResponseData([]byte("data: whole\n\n")) {
		t.Fatal("a complete event was held back")
	}
}

// A serving chain that CANNOT run chunk by chunk must force the response to be buffered, exactly
// as the same policy would have done from the route chain. Otherwise the engine streams and that
// policy never runs at all.
func TestResponseStreamingEnabled_ABufferedOnlyServingChainForcesBuffering(t *testing.T) {
	ec := streamingUnwindCtx(t, nil, servingStreamChain(bufferedOnlyStreamPolicy{}))

	if ec.responseStreamingEnabled(false) {
		t.Fatal("the response streamed past a serving chain whose body policy is buffered-only; " +
			"that policy would never run")
	}

	// And a streaming-capable serving chain must NOT force buffering, or every cross-vendor
	// SSE response is buffered and the point of the streaming unwind is lost.
	ok := streamingUnwindCtx(t, nil, servingStreamChain(translatingStreamPolicy{}))
	if !ok.responseStreamingEnabled(false) {
		t.Fatal("a streaming-capable serving chain must not force the response to buffer")
	}
}

// No serving chain at all must not make anything buffer — that is nearly every API.
func TestResponseStreamingEnabled_NoServingChainIsNeutral(t *testing.T) {
	ec := streamingUnwindCtx(t, nil, nil)
	if !ec.responseStreamingEnabled(false) {
		t.Fatal("an API with no per-attempt work was forced to buffer its streamed response")
	}
}

// headerOnlyUpstreamPolicy is a provider CREDENTIAL: it touches headers and never the body.
type headerOnlyUpstreamPolicy struct{}

func (headerOnlyUpstreamPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (headerOnlyUpstreamPolicy) OnResponseHeaders(_ context.Context, _ *policy.ResponseHeaderContext,
	_ map[string]interface{}) policy.ResponseHeaderAction {
	return policy.DownstreamResponseHeaderModifications{}
}

// A serving chain that never reads the response body must not force the response to buffer.
//
// Both chain builders clear SupportsResponseStreaming when a chain has NO response-body policy
// — "there is nothing to stream" — so a credential-only chain reports false. Reading that flag
// as an objection buffers every streamed response on a proxy whose providers carry only a
// credential, which is what same-vendor failover looks like.
func TestResponseStreamingEnabled_ACredentialOnlyServingChainIsNeutral(t *testing.T) {
	serving := &registry.PolicyChain{
		Policies:    []policy.Policy{headerOnlyUpstreamPolicy{}},
		PolicySpecs: []policy.PolicySpec{{Name: "set-headers", Enabled: true}},
		// Exactly what BuildPolicyChain produces for a chain with no response-body policy.
		RequiresResponseBody:      false,
		SupportsResponseStreaming: false,
	}
	ec := streamingUnwindCtx(t, nil, serving)

	if !ec.responseStreamingEnabled(false) {
		t.Fatal("a provider chain that only sets a credential forced the response to buffer; " +
			"every same-vendor failover would lose incremental delivery")
	}
}

// And the ROUTE chain is read through the same predicate, for the same reason.
//
// Once a provider's transformer moves to the per-attempt position (FR-012i) the route chain is
// often left with no response-body policy at all, so its flag goes false. Treating that as an
// objection buffers the response — the translation still happens, but the client receives the
// whole answer at once instead of token by token, for a reason that has nothing to do with it.
func TestResponseStreamingEnabled_ARouteChainWithNoBodyPolicyIsNeutral(t *testing.T) {
	ec := streamingUnwindCtx(t, nil, servingStreamChain(translatingStreamPolicy{}))
	// What the builders produce for a route chain left with nothing to do on the body.
	ec.policyChain.SupportsResponseStreaming = false
	ec.policyChain.RequiresResponseBody = false

	if !ec.responseStreamingEnabled(false) {
		t.Fatal("an empty route chain vetoed streaming, so a cross-vendor answer the serving " +
			"chain can translate chunk by chunk is buffered instead")
	}
}

// A chain that reads the body only ON A FAULT still objects. An error framed as SSE must be
// buffered whole, because fault policies cannot act on a fragment of one.
func TestResponseStreamingEnabled_AFaultOnlyChainStillBlocks(t *testing.T) {
	ec := streamingUnwindCtx(t, nil, nil)
	ec.policyChain.SupportsResponseStreaming = false
	ec.policyChain.RequiresResponseBody = false
	ec.policyChain.RequiresResponseBodyOnFault = true

	if ec.responseStreamingEnabled(false) {
		t.Fatal("a fault-only chain let an SSE-framed error stream past it")
	}
}
