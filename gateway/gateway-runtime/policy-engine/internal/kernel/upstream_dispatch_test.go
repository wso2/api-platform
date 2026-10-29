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
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func upstreamHeaderRequest(pairs ...string) *extprocv3.ProcessingRequest {
	headers := make([]*corev3.HeaderValue, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		headers = append(headers, &corev3.HeaderValue{Key: pairs[i], Value: pairs[i+1]})
	}
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: headers},
			},
		},
	}
}

// A per-attempt policy serves ONE attempt, not the client request. Downstream carries the
// client-request snapshot, so leaving it nil is what stops such a policy making decisions that
// cannot legitimately vary per attempt (FR-037).
func TestUpstreamRequestContext_DownstreamIsNil(t *testing.T) {
	req := upstreamHeaderRequest(":path", "/v1/chat", ":method", "POST")
	reqCtx := upstreamRequestContext(req, req.GetRequestHeaders().GetHeaders())

	if reqCtx.Downstream != nil {
		t.Fatal("Downstream must be nil at the upstream position: it is the client-request " +
			"snapshot, which belongs to the once-per-request chain")
	}
	if reqCtx.Path != "/v1/chat" || reqCtx.Method != "POST" {
		t.Fatalf("pseudo-headers not lifted: path=%q method=%q", reqCtx.Path, reqCtx.Method)
	}
}

func TestUpstreamRequestContext_HeadersAreReadable(t *testing.T) {
	req := upstreamHeaderRequest("X-Custom", "value", ":scheme", "https", ":authority", "api.example")
	reqCtx := upstreamRequestContext(req, req.GetRequestHeaders().GetHeaders())

	// Lower-cased on the way in, so a policy need not guess the casing Envoy used.
	if got := reqCtx.Headers.Get("x-custom"); len(got) != 1 || got[0] != "value" {
		t.Fatalf("header = %v", got)
	}
	if reqCtx.Scheme != "https" || reqCtx.Authority != "api.example" {
		t.Fatalf("scheme=%q authority=%q", reqCtx.Scheme, reqCtx.Authority)
	}
}

// Re-pointing an attempt at another provider is a :path change. It only reaches Envoy because
// the router sets mutation_rules.allow_all_routing on this filter — without that the mutation is
// dropped silently, with no error and no log.
func TestUpstreamHeadersResponse_EmitsPathRewrite(t *testing.T) {
	newPath := "/anthropic-ctx/v1/messages"
	result := &executor.RequestHeaderExecutionResult{
		Results: []executor.RequestHeaderPolicyResult{{
			Action: policy.UpstreamRequestHeaderModifications{
				Path:         &newPath,
				HeadersToSet: map[string]string{"x-api-key": "secret"},
			},
		}},
	}

	resp := upstreamHeadersResponse(result, nil)
	mutation := resp.GetRequestHeaders().GetResponse().GetHeaderMutation()

	var sawPath, sawKey bool
	for _, h := range mutation.GetSetHeaders() {
		switch h.GetHeader().GetKey() {
		case ":path":
			sawPath = true
			if got := string(h.GetHeader().GetRawValue()); got != newPath {
				t.Fatalf(":path = %q, want %q", got, newPath)
			}
		case "x-api-key":
			sawKey = true
		}
	}
	if !sawPath {
		t.Fatal("no :path mutation emitted; the attempt would reach the previous provider's context")
	}
	if !sawKey {
		t.Fatal("ordinary header mutations must still be emitted")
	}
}

// A skipped policy, or one returning no action, contributes nothing.
func TestUpstreamHeadersResponse_IgnoresSkippedPolicies(t *testing.T) {
	result := &executor.RequestHeaderExecutionResult{
		Results: []executor.RequestHeaderPolicyResult{
			{Skipped: true, Action: policy.UpstreamRequestHeaderModifications{
				HeadersToSet: map[string]string{"x-should-not-appear": "1"},
			}},
			{Action: nil},
		},
	}
	mutation := upstreamHeadersResponse(result, nil).GetRequestHeaders().GetResponse().GetHeaderMutation()
	if len(mutation.GetSetHeaders()) != 0 {
		t.Fatalf("expected no mutations, got %v", mutation.GetSetHeaders())
	}
}

// An attempt whose cluster has no chain proceeds exactly as Envoy built it. The ordinary case,
// not an error: an upstream with no per-attempt work generates no chain at all.
func TestPassthroughRequestHeaders_MutatesNothing(t *testing.T) {
	resp := passthroughRequestHeaders()
	common := resp.GetRequestHeaders().GetResponse()
	if common == nil {
		t.Fatal("expected a CommonResponse so Envoy continues the attempt")
	}
	if common.GetHeaderMutation() != nil {
		t.Fatal("passthrough must carry no mutation at all")
	}
}

// THE gap that made cross-provider failover return 503 while every other part worked: the
// route's regex_rewrite is compiled once and points EVERY attempt at the primary provider's
// context, so an attempt routed to a fallback cluster still arrived on the primary's context.
func TestRepointToProvider(t *testing.T) {
	cases := []struct {
		name                                      string
		baseline, routeBasePath, providerBasePath string
		want                                      string
	}{
		{
			name:             "primary context swapped for the fallback's",
			baseline:         "/openai-ctx/v1/chat/completions",
			routeBasePath:    "/openai-ctx",
			providerBasePath: "/anthropic-ctx",
			want:             "/anthropic-ctx/v1/chat/completions",
		},
		{
			name:             "provider at the root keeps only the remainder",
			baseline:         "/openai-ctx/v1/chat",
			routeBasePath:    "/openai-ctx",
			providerBasePath: "/",
			want:             "/v1/chat",
		},
		{
			name:             "route at the root prepends the provider context",
			baseline:         "/v1/chat",
			routeBasePath:    "/",
			providerBasePath: "/azure-ctx",
			want:             "/azure-ctx/v1/chat",
		},
		{
			name:             "trailing slash on the provider context is not doubled",
			baseline:         "/openai-ctx/v1/chat",
			routeBasePath:    "/openai-ctx",
			providerBasePath: "/anthropic-ctx/",
			want:             "/anthropic-ctx/v1/chat",
		},
		{
			name:             "nothing left after the context still yields a valid path",
			baseline:         "/openai-ctx",
			routeBasePath:    "/openai-ctx",
			providerBasePath: "/anthropic-ctx",
			want:             "/anthropic-ctx/",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := repointToProvider(tc.baseline, tc.routeBasePath, tc.providerBasePath); got != tc.want {
				t.Fatalf("path = %q, want %q", got, tc.want)
			}
		})
	}
}

// Built from the BASELINE, never the incoming path — the incoming path already carries the
// previous attempt's rewrite, so deriving from it compounds (FR-038). Feeding an
// already-rewritten path in shows the difference the baseline makes.
func TestRepointToProvider_BaselinePreventsCompounding(t *testing.T) {
	baseline := "/openai-ctx/v1/chat"

	first := repointToProvider(baseline, "/openai-ctx", "/anthropic-ctx")
	// Attempt 3 resets to the same baseline rather than building on attempt 2's result.
	third := repointToProvider(baseline, "/openai-ctx", "/azure-ctx")

	if first != "/anthropic-ctx/v1/chat" || third != "/azure-ctx/v1/chat" {
		t.Fatalf("first=%q third=%q", first, third)
	}
	// What compounding would have produced, had the incoming path been used instead.
	compounded := repointToProvider(first, "/openai-ctx", "/azure-ctx")
	if compounded == third {
		t.Fatal("expected deriving from the previous attempt's path to differ from deriving " +
			"from the baseline; if they match, this test no longer demonstrates the hazard")
	}
}

// An attempt whose provider cannot be resolved is left alone rather than re-pointed at a guess.
func TestUpstreamPathOnlyResponse_NoRepointMeansPassthrough(t *testing.T) {
	if mutation := upstreamPathOnlyResponse(nil).GetRequestHeaders().GetResponse().GetHeaderMutation(); mutation != nil {
		t.Fatalf("expected a pure passthrough, got %v", mutation)
	}

	repointed := "/anthropic-ctx/v1/chat"
	mutation := upstreamPathOnlyResponse(&repointed).GetRequestHeaders().GetResponse().GetHeaderMutation()
	if len(mutation.GetSetHeaders()) != 1 {
		t.Fatalf("expected exactly the :path mutation, got %v", mutation.GetSetHeaders())
	}
	if got := string(mutation.GetSetHeaders()[0].GetHeader().GetRawValue()); got != repointed {
		t.Fatalf(":path = %q", got)
	}
}

// A transformer rewrites the PAYLOAD and implements that as OnRequestBody, so the body has to be
// asked for — otherwise the transformer is never called and the fallback provider receives a
// request in the primary's format.
//
// Asked for per attempt, and only by chains that rewrite it: the filter is generated with
// RequestBodyMode NONE, so a header-only chain pays no buffering cost.
func TestUpstreamModeOverride_RequestsBodyOnlyWhenNeeded(t *testing.T) {
	needsBody := &registry.PolicyChain{RequiresRequestBody: true}
	override := upstreamModeOverride(needsBody)
	if override == nil {
		t.Fatal("a chain that rewrites the body must ask Envoy for it, or its transformer never runs")
	}
	if override.GetRequestBodyMode() != extprocconfigv3.ProcessingMode_BUFFERED {
		t.Fatalf("body mode = %v, want BUFFERED", override.GetRequestBodyMode())
	}

	for name, chain := range map[string]*registry.PolicyChain{
		"header-only chain": {RequiresRequestBody: false},
		"no chain":          nil,
	} {
		t.Run(name, func(t *testing.T) {
			if upstreamModeOverride(chain) != nil {
				t.Fatal("must not request a body that nothing will read")
			}
		})
	}
}

// A body phase that rewrote nothing forwards the original untouched.
func TestPassthroughRequestBody(t *testing.T) {
	if m := passthroughRequestBody(nil, nil, nil).GetRequestBody().GetResponse().GetBodyMutation(); m != nil {
		t.Fatalf("expected no mutation, got %v", m)
	}

	rewritten := []byte(`{"model":"claude-sonnet-4"}`)
	mutation := passthroughRequestBody(rewritten, nil, nil).GetRequestBody().GetResponse().GetBodyMutation()
	if string(mutation.GetBody()) != string(rewritten) {
		t.Fatalf("body = %q, want the rewritten payload", mutation.GetBody())
	}
}
