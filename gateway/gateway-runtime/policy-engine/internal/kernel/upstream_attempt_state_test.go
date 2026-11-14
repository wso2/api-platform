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
	"strconv"
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A RequestBody message at the upstream position carries NO request_attributes: Envoy attaches
// them to the first message of a stream only. Deriving the cluster from the body message itself
// therefore yields "", the chain lookup misses, and a transformer in the per-attempt chain is
// silently skipped — the fallback provider receives the primary's payload format while its
// credential (applied in the headers phase) is correct. See spec FR-037b, poc-results Probe I.
func TestUpstreamBodyMessage_CarriesNoClusterAttribute(t *testing.T) {
	body := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{
			RequestBody: &extprocv3.HttpBody{Body: []byte(`{"model":"gpt-4o"}`)},
		},
	}

	if got := clusterFromRequest(body); got != "" {
		t.Fatalf("body message resolved a cluster %q; the test's premise no longer holds", got)
	}
}

// The chain must still be reachable for that body message, via the cluster its own headers phase
// resolved and stored.
func TestUpstreamChainForCluster_ResolvesWhatTheHeadersPhaseStored(t *testing.T) {
	const cluster = "upstream_LlmProxy_abc_anthropic"

	k := NewKernel()
	want := &registry.PolicyChain{RequiresRequestBody: true}
	k.RegisterRoute(UpstreamChainKey(cluster), want)

	// What the headers phase does with the attribute-bearing message.
	state := &upstreamAttemptState{}
	state.cluster = clusterFromRequest(requestWithClusterAttribute(cluster))
	if state.cluster != cluster {
		t.Fatalf("headers phase stored cluster %q, want %q", state.cluster, cluster)
	}

	// What the body phase must then be able to do without any attributes of its own.
	if got := k.UpstreamChainForCluster(state.cluster); got != want {
		t.Fatalf("body phase resolved chain %v, want the registered chain", got)
	}

	// And the regression itself: resolving from the body message instead finds nothing.
	if got := k.UpstreamChainForCluster(clusterFromRequest(&extprocv3.ProcessingRequest{})); got != nil {
		t.Fatalf("expected no chain when the cluster is unknown, got %v", got)
	}
}

// An unknown cluster is "no per-attempt work", not an error — an upstream with nothing to do
// generates no chain at all.
func TestUpstreamChainForCluster_EmptyClusterIsNil(t *testing.T) {
	if got := NewKernel().UpstreamChainForCluster(""); got != nil {
		t.Fatalf("empty cluster resolved a chain: %v", got)
	}
}

// Every body-rewriting policy in this repo opens with "if Body == nil || !Body.Present ||
// len(Content) == 0 { nothing to do }". A per-attempt body handed over with Present at its zero
// value is therefore silently ignored — the chain runs, the transformer returns no modification,
// and the fallback provider receives the primary's payload format with no error raised anywhere.
func TestUpstreamBodyContext_MarksBodyPresentAndComplete(t *testing.T) {
	payload := []byte(`{"model":"gpt-4o"}`)
	ctx := upstreamBodyContext(&extprocv3.HttpBody{Body: payload, EndOfStream: true}, &upstreamAttemptState{})

	if ctx.Body == nil {
		t.Fatal("body context has no Body")
	}
	if !ctx.Body.Present {
		t.Fatal("Body.Present is false — every body-rewriting policy will no-op")
	}
	if !ctx.Body.EndOfStream {
		t.Fatal("Body.EndOfStream is false for a complete BUFFERED body")
	}
	if string(ctx.Body.Content) != string(payload) {
		t.Fatalf("Content = %q, want %q", ctx.Body.Content, payload)
	}
}

// An empty body is still "present": the policy's own len(Content)==0 check decides what that
// means, and it must not be pre-empted by a Present flag that merely says "no bytes".
func TestUpstreamBodyContext_EmptyBodyIsStillPresent(t *testing.T) {
	ctx := upstreamBodyContext(&extprocv3.HttpBody{EndOfStream: true}, &upstreamAttemptState{})
	if !ctx.Body.Present {
		t.Fatal("Body.Present is false for an empty body")
	}
}

func TestUpstreamBodyContext_CarriesMetadata(t *testing.T) {
	meta := map[string]interface{}{"ns": map[string]interface{}{"k": "v"}}
	ctx := upstreamBodyContext(&extprocv3.HttpBody{}, &upstreamAttemptState{metadata: meta})
	if ctx.SharedContext == nil || ctx.SharedContext.Metadata == nil {
		t.Fatal("metadata not carried into the body phase")
	}
	if _, ok := ctx.SharedContext.Metadata["ns"]; !ok {
		t.Fatalf("metadata = %v, want the headers phase's map", ctx.SharedContext.Metadata)
	}
}

// The executor writes a policy's HeadersToSet straight into the map behind ctx.Headers. A body
// context built without them panics with "assignment to entry in nil map" the moment any
// per-attempt policy sets a header — which a body-rewriting policy routinely does (content-length,
// a vendor API version).
func TestUpstreamBodyContext_HeadersAreWritable(t *testing.T) {
	state := &upstreamAttemptState{headers: map[string][]string{"x-api-key": {"secret"}}}
	ctx := upstreamBodyContext(&extprocv3.HttpBody{}, state)

	if ctx.Headers == nil {
		t.Fatal("body context has no Headers")
	}
	values := ctx.Headers.UnsafeInternalValues()
	if values == nil {
		t.Fatal("header map is nil — the executor will panic writing into it")
	}
	values["anthropic-version"] = []string{"2023-06-01"} // what the executor does

	// The headers phase's own map must not see that write.
	if _, leaked := state.headers["anthropic-version"]; leaked {
		t.Fatal("body-phase header mutation leaked back into the headers phase's map")
	}
	if got := values["x-api-key"]; len(got) != 1 || got[0] != "secret" {
		t.Fatalf("x-api-key = %v, want the headers phase's value", got)
	}
}

// Even with no headers recorded, the map must exist.
func TestUpstreamBodyContext_HeadersWritableWhenNoneRecorded(t *testing.T) {
	ctx := upstreamBodyContext(&extprocv3.HttpBody{}, &upstreamAttemptState{})
	if ctx.Headers == nil || ctx.Headers.UnsafeInternalValues() == nil {
		t.Fatal("header map is nil for an attempt with no recorded headers")
	}
}

// A body-phase header change has to reach Envoy, or the provider gets a payload its headers
// contradict.
func TestPassthroughRequestBody_EmitsHeaderMutations(t *testing.T) {
	resp := passthroughRequestBody([]byte(`{"x":1}`), map[string]string{"anthropic-version": "2023-06-01"}, []string{"x-drop"})

	common := resp.GetRequestBody().GetResponse()
	if common.GetBodyMutation() == nil {
		t.Fatal("body mutation missing")
	}
	mutation := common.GetHeaderMutation()
	if mutation == nil {
		t.Fatal("header mutation missing")
	}
	set := map[string]string{}
	for _, h := range mutation.GetSetHeaders() {
		set[h.GetHeader().GetKey()] = string(h.GetHeader().GetRawValue())
	}
	if set["anthropic-version"] != "2023-06-01" {
		t.Fatalf("set headers = %v", set)
	}
	// The rewrite also corrects content-length — see TestPassthroughRequestBody_CorrectsContentLength.
	if _, ok := set[contentLengthHeader]; !ok {
		t.Fatalf("content-length not corrected alongside the body rewrite: %v", set)
	}
	if len(mutation.GetRemoveHeaders()) != 1 || mutation.GetRemoveHeaders()[0] != "x-drop" {
		t.Fatalf("remove headers = %v", mutation.GetRemoveHeaders())
	}
}

// No changes means no mutation block at all — not an empty one.
func TestPassthroughRequestBody_NoChangesMeansNoMutations(t *testing.T) {
	common := passthroughRequestBody(nil, nil, nil).GetRequestBody().GetResponse()
	if common.GetBodyMutation() != nil || common.GetHeaderMutation() != nil {
		t.Fatalf("expected an inert response, got %v", common)
	}
}

// A transformer rewrites the vendor path alongside the payload — Anthropic's /v1/messages. Emitted
// as-is that path lands at the gateway root instead of inside the provider's own context, so the
// body phase must re-point it exactly as the headers phase re-pointed the request.
func TestUpstreamBodyPath_IsRepointedIntoTheProviderContext(t *testing.T) {
	got := repointToProvider("/v1/messages", "/tx-proxy", "/anthropic-upstream")
	if got != "/anthropic-upstream/v1/messages" {
		t.Fatalf("path = %q, want /anthropic-upstream/v1/messages", got)
	}

	// A provider with no base path of its own keeps the vendor path unchanged.
	if got := repointToProvider("/v1/messages", "/tx-proxy", ""); got != "/v1/messages" {
		t.Fatalf("path = %q, want /v1/messages", got)
	}
}

// The platform half of per-attempt policy support: a policy whose decision varies per attempt
// can derive neither the attempt number nor the target on its own. The downstream chain ran once,
// before any target existed, and Envoy reports the selection only as an attribute on the attempt's
// first message.
func TestWithAttemptContext_PublishesNumberAndCluster(t *testing.T) {
	shared := &policy.SharedContext{}
	withAttemptContext(shared, 2, "upstream_LlmProxy_abc_anthropic")

	ns, ok := shared.Metadata[constants.AttemptMetadataNamespace].(map[string]interface{})
	if !ok {
		t.Fatalf("reserved namespace missing: %v", shared.Metadata)
	}
	if ns[constants.AttemptMetadataNumber] != 2 {
		t.Fatalf("attempt number = %v, want 2", ns[constants.AttemptMetadataNumber])
	}
	if ns[constants.AttemptMetadataCluster] != "upstream_LlmProxy_abc_anthropic" {
		t.Fatalf("cluster = %v", ns[constants.AttemptMetadataCluster])
	}
}

// It must not clobber metadata Envoy forwarded — a per-attempt policy reads the downstream
// chain's published decisions from there.
func TestWithAttemptContext_PreservesForwardedMetadata(t *testing.T) {
	shared := &policy.SharedContext{Metadata: map[string]interface{}{"wso2.failover": map[string]interface{}{"chain": "x"}}}
	withAttemptContext(shared, 1, "c")

	if _, ok := shared.Metadata["wso2.failover"]; !ok {
		t.Fatal("forwarded metadata was clobbered")
	}
	if _, ok := shared.Metadata[constants.AttemptMetadataNamespace]; !ok {
		t.Fatal("attempt namespace missing")
	}
}

func TestWithAttemptContext_NilSafe(t *testing.T) {
	withAttemptContext(nil, 1, "c") // must not panic
}

// Envoy rejects a rewritten body whose content-length still describes the old one —
// "mismatch_between_content_length_and_the_length_of_the_mutated_body", surfaced to the client as
// a 500. The policy that rewrote the body has no reason to know its own output length matters
// here, so the dispatcher owns the correction.
func TestPassthroughRequestBody_CorrectsContentLength(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5-20250929"}`)
	common := passthroughRequestBody(body, nil, nil).GetRequestBody().GetResponse()

	var got string
	for _, h := range common.GetHeaderMutation().GetSetHeaders() {
		if h.GetHeader().GetKey() == contentLengthHeader {
			got = string(h.GetHeader().GetRawValue())
		}
	}
	if got != strconv.Itoa(len(body)) {
		t.Fatalf("content-length = %q, want %d", got, len(body))
	}
}

// A policy that set content-length itself was explicit about it and wins.
func TestPassthroughRequestBody_PolicyContentLengthWins(t *testing.T) {
	common := passthroughRequestBody([]byte(`{"a":1}`),
		map[string]string{contentLengthHeader: "999"}, nil).GetRequestBody().GetResponse()

	for _, h := range common.GetHeaderMutation().GetSetHeaders() {
		if h.GetHeader().GetKey() == contentLengthHeader && string(h.GetHeader().GetRawValue()) != "999" {
			t.Fatalf("content-length = %q, want the policy's own 999", h.GetHeader().GetRawValue())
		}
	}
}

// An untouched body must not gain a content-length mutation it never needed.
func TestPassthroughRequestBody_NoContentLengthWithoutARewrite(t *testing.T) {
	common := passthroughRequestBody(nil, nil, nil).GetRequestBody().GetResponse()
	if common.GetHeaderMutation() != nil {
		t.Fatalf("expected no header mutation, got %v", common.GetHeaderMutation())
	}
}
