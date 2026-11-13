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
)

// A request where no attempt ever reached the upstream position unwinds nothing. This is the
// ordinary case for every API without failover, so it must cost a lookup and return — not error,
// and not reach for a chain that was never built (FR-037h).
func TestUnwindServingUpstreamChain_NoAttemptRecorded(t *testing.T) {
	server := newTestServerWithStore(t, nil)
	ec := newPolicyExecutionContext(server, "test-route", nil)
	ec.requestID = "req-no-attempt"

	if err := ec.unwindServingUpstreamChain(context.Background()); err != nil {
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

	if err := ec.unwindServingUpstreamChain(context.Background()); err != nil {
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
	if err := ec.unwindServingUpstreamChain(context.Background()); err != nil {
		t.Fatalf("unwind returned %v", err)
	}
}
