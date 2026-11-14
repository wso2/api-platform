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
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
)

// unwindServingUpstreamChain runs the serving attempt's upstream chain in REVERSE at the head of
// the downstream response phase, before the route's own response policies (FR-037h).
//
// WHY THIS EXISTS. A provider's transformer is ONE bidirectional policy: it converts the request
// into that vendor's shape and converts the response back. Per-attempt placement puts it in the
// upstream chain, which Envoy re-enters per attempt — but that position is generated
// response-SKIP, and the engine's upstream switch bare-acks anything that is not a request
// message. So only its request half ever runs, and a cross-vendor fallback would answer the
// client in the vendor's own shape.
//
// WHY HERE AND NOT AT THE UPSTREAM POSITION. Probes T4 and T5 measured that the upstream position
// CAN deliver responses — including for attempts that were retried away from:
//
//	cluster_name = "provider_a"   RESPONSE HEADERS at upstream position: status=500  <- discarded
//	cluster_name = "provider_b"   RESPONSE HEADERS at upstream position: status=200  <- served
//
// That is exactly the problem: when the filter sees a response there, the router has not yet
// decided whether to retry it, so a policy cannot tell a response that will be delivered from one
// about to be discarded. Here the question does not arise — the downstream response phase runs
// once, on the committed response, so "which response is final" is settled by construction.
//
// T5 also measured the cost of moving the work there rather than unwinding: 4.0 ext_proc messages
// per failover request instead of 2.0, since every attempt pays a response round trip, and the
// position does not work at all under the 200ms default message timeout — it needs 5s, which
// would weaken that bound for every message. The unwind adds no round trip, because it executes
// inside the downstream response message that happens anyway.
//
// WHY THE WHOLE CHAIN RATHER THAN THE TRANSFORMER. Unwinding everything makes a transformer's
// reverse translation and any future authored per-provider response policy the same mechanism,
// with no further change. Policies with no response half — the credential, the failover
// per-attempt instance — simply contribute nothing.
//
// Reverse order is what the executor already does; this only has to run it first, so the chain
// unwraps in the order opposite to the one that wrapped it.
func (ec *PolicyExecutionContext) unwindServingUpstreamChain(
	ctx context.Context,
) (*executor.ResponseHeaderExecutionResult, error) {
	// Keyed by the engine-minted attempt key, NOT the request id — the upstream position keys
	// its writes the same way. A client-supplied x-request-id survives into the gateway, so
	// keying on it let two concurrent requests share an entry and unwind one response with the
	// other's provider chain.
	cluster, ok := ec.server.baselines.ServingCluster(ec.attemptKey)
	if !ok {
		// No attempt reached the upstream position for this request: the API attaches no
		// per-attempt work, or the entry expired. Nothing wrapped the request, so nothing
		// needs unwrapping — the ordinary case for every API without failover.
		return nil, nil
	}

	chain := ec.server.kernel.UpstreamChainForCluster(cluster)
	if chain == nil || len(chain.Policies) == 0 {
		return nil, nil
	}

	slog.DebugContext(ctx, "unwinding the serving attempt's upstream chain",
		"cluster", cluster, "policies", len(chain.Policies), "route", ec.routeKey)

	// Executed with the UPSTREAM chain's own key, not the route's. These instances were built
	// for that chain and resolve their own per-chain state through it; handing them the route
	// key would silently point them at a different one.
	// The result is RETURNED, not discarded. A policy's header mutation only reaches the
	// client through TranslateResponseHeaderActions, which builds its mutation list from
	// these Results alone — it never re-reads responseHeaderCtx. Dropping the result here
	// ran set-headers (and a transformer's reverse half) and then silently threw the
	// outcome away, which is how an unwound header went missing from the response while
	// every log line said the chain had executed.
	return ec.server.executor.ExecuteResponseHeaderPolicies(
		ctx,
		chain.Policies,
		ec.responseHeaderCtx,
		chain.PolicySpecs,
		"",
		UpstreamChainKey(cluster),
		// The CHAIN's own flag, not a literal false. The executor skips every
		// ExecutionCondition when this is false, so hard-coding it ran a provider policy
		// whose condition said not to — on the response half only, while the request half
		// honoured it. A condition that gates a transformer would then have wrapped
		// nothing and unwrapped the response anyway.
		chain.HasExecutionConditions,
	)
}

// unwindServingUpstreamChainBody is the body-phase half of the unwind.
//
// The header half alone is not enough, and for the case this feature exists for it is the less
// important half. A provider transformer converts the vendor's JSON payload, which is a
// response-BODY concern: headers cannot turn `content_block` into `choices[]`. The upstream
// filter is request-only, and the downstream body phase runs ec.responsePolicies(), which is the
// ROUTE chain — so without this the serving provider's reverse translation never runs and a
// cross-vendor failover answers the client in the fallback vendor's shape.
//
// Runs on the same chain, resolved the same way, before the route's own body policies, so the
// payload is back in the API's shape by the time they see it.
func (ec *PolicyExecutionContext) unwindServingUpstreamChainBody(ctx context.Context) (*executor.ResponseExecutionResult, error) {
	cluster, ok := ec.server.baselines.ServingCluster(ec.attemptKey)
	if !ok {
		return nil, nil
	}
	chain := ec.server.kernel.UpstreamChainForCluster(cluster)
	if chain == nil || len(chain.Policies) == 0 {
		return nil, nil
	}

	slog.DebugContext(ctx, "unwinding the serving attempt's upstream chain (body)",
		"cluster", cluster, "policies", len(chain.Policies), "route", ec.routeKey)

	return ec.server.executor.ExecuteResponsePolicies(
		ctx,
		chain.Policies,
		ec.responseBodyCtx,
		chain.PolicySpecs,
		"",
		UpstreamChainKey(cluster),
		chain.HasExecutionConditions, // see the header half
	)
}

// ServingUpstreamChainRequiresResponseBody reports whether the chain that wrapped this response
// has a response-BODY policy, so the body has to be delivered even when the route's own chain
// wants none.
//
// Without this the gate that decides body delivery read the route chain alone. An API whose only
// body work is a provider transformer — the shape this feature exists for, since the route chain
// often has no body policy at all — told Envoy to skip the response body, and the body unwind
// then had nothing to run on: the provider's reverse translation never happened and the client
// received the fallback vendor's payload shape.
//
// Streaming is excluded deliberately rather than overlooked. A reverse transform is a
// whole-document conversion, so it cannot run chunk by chunk, and the streaming body path does
// not unwind at all; asking Envoy for a streamed body here would buffer or forward chunks for a
// chain that has nothing to do with them. A streamed cross-vendor response therefore still
// reaches the client in the serving provider's shape, which is the documented streaming limit of
// this policy, not a new one.
func (ec *PolicyExecutionContext) ServingUpstreamChainRequiresResponseBody() bool {
	if ec.isStreamingResponse || ec.server == nil || ec.server.baselines == nil || ec.server.kernel == nil {
		return false
	}
	cluster, ok := ec.server.baselines.ServingCluster(ec.attemptKey)
	if !ok {
		// Before an attempt reaches the upstream position there is no serving cluster, so
		// the request-phase mode decision never sees this; the response-header callback is
		// where it takes effect, and Envoy honours a ModeOverride returned from there.
		return false
	}
	chain := ec.server.kernel.UpstreamChainForCluster(cluster)
	return chain != nil && chain.RequiresResponseBody
}
