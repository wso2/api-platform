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
// WHY HERE AND NOT AT THE UPSTREAM POSITION. Probe T4 measured that the upstream position CAN
// deliver responses — including for attempts that were retried away from. That is exactly the
// problem: when the filter sees a response there, the router has not yet decided whether to retry
// it, so a policy cannot tell a response that will be delivered from one about to be discarded.
// Here the question does not arise. The downstream response phase runs once, on the committed
// response, so "which response is final" is settled by construction.
//
// WHY THE WHOLE CHAIN RATHER THAN THE TRANSFORMER. Unwinding everything makes a transformer's
// reverse translation and any future authored per-provider response policy the same mechanism,
// with no further change. Policies with no response half — the credential, the failover
// per-attempt instance — simply contribute nothing.
//
// Reverse order is what the executor already does; this only has to run it first, so the chain
// unwraps in the order opposite to the one that wrapped it.
func (ec *PolicyExecutionContext) unwindServingUpstreamChain(ctx context.Context) error {
	cluster, ok := ec.server.baselines.ServingCluster(ec.requestID)
	if !ok {
		// No attempt reached the upstream position for this request: the API attaches no
		// per-attempt work, or the entry expired. Nothing wrapped the request, so nothing
		// needs unwrapping — the ordinary case for every API without failover.
		return nil
	}

	chain := ec.server.kernel.UpstreamChainForCluster(cluster)
	if chain == nil || len(chain.Policies) == 0 {
		return nil
	}

	slog.DebugContext(ctx, "unwinding the serving attempt's upstream chain",
		"cluster", cluster, "policies", len(chain.Policies), "route", ec.routeKey)

	// Executed with the UPSTREAM chain's own key, not the route's. These instances were built
	// for that chain and resolve their own per-chain state through it; handing them the route
	// key would silently point them at a different one.
	_, err := ec.server.executor.ExecuteResponseHeaderPolicies(
		ctx,
		chain.Policies,
		ec.responseHeaderCtx,
		chain.PolicySpecs,
		"",
		UpstreamChainKey(cluster),
		false,
	)
	return err
}
