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
