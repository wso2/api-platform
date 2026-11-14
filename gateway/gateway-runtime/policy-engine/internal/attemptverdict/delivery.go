/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
 *
 */

package attemptverdict

import (
	"context"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ChainLookup resolves a route name to the policy chain configured for it.
//
// Taken as a function rather than the kernel itself so this package does not depend on the
// kernel — the kernel already depends on a great deal, and the dependency would be circular in
// spirit if not yet in fact.
type ChainLookup func(routeName string) *registry.PolicyChain

// PolicyDeliverySink hands a verdict to the policies configured on the route that produced it.
//
// Delivery is STRUCTURAL: the route name on the access-log entry is the same key the engine
// files chains under, so the chain is looked up, not inferred. That matters beyond tidiness.
// An LLM proxy forwards through the provider's own route, so one client call produces two
// flagged access-log entries under a single request id; attributing by cluster name would record
// two failures for one, and would try to grade a loopback hop that is not a failover target at
// all. Keying on the route resolves both: the hops are different routes, and only one has the
// policy on it.
type PolicyDeliverySink struct {
	lookup ChainLookup

	// fallback receives verdicts that no policy consumed, so a misconfiguration surfaces as a
	// log line rather than silence.
	fallback Sink
}

// NewPolicyDeliverySink builds a sink delivering through lookup.
func NewPolicyDeliverySink(lookup ChainLookup, fallback Sink) *PolicyDeliverySink {
	if fallback == nil {
		fallback = &LoggingSink{}
	}
	return &PolicyDeliverySink{lookup: lookup, fallback: fallback}
}

// RecordAttemptVerdict delivers v to every policy on its route that implements
// policy.AttemptCompletionPolicy.
//
// Runs on the access-log stream's goroutine, inline with the entries behind it, so it does the
// least work that is correct: a route with no such policy — the overwhelmingly common case — is
// a map lookup and a type assertion per policy.
func (s *PolicyDeliverySink) RecordAttemptVerdict(v Verdict) {
	if s.lookup == nil || v.RouteName == "" {
		s.fallback.RecordAttemptVerdict(v)
		return
	}

	chain := s.lookup(v.RouteName)
	if chain == nil {
		// Not an error: most routes have no chain at all, and a config push can retire a route
		// between the request and its access-log entry arriving a second later.
		s.fallback.RecordAttemptVerdict(v)
		return
	}

	attemptCtx := &policy.AttemptCompletionContext{
		RequestID:       v.RequestID,
		RouteName:       v.RouteName,
		UpstreamCluster: v.UpstreamCluster,
		ResponseFlags:   v.Flags,
		ResponseCode:    v.ResponseCode,
		Duration:        v.Duration,
	}

	delivered := 0
	for i, p := range chain.Policies {
		completion, ok := p.(policy.AttemptCompletionPolicy)
		if !ok {
			continue
		}
		// Parameters.Raw is the merged (init + runtime) parameter map the chain builder wrote
		// back at configuration time — the same value every other hook is handed, see
		// internal/executor/chain.go.
		var params map[string]interface{}
		if i < len(chain.PolicySpecs) {
			params = chain.PolicySpecs[i].Parameters.Raw
		}
		deliverOne(completion, attemptCtx, params, v)
		delivered++
	}

	if delivered == 0 {
		s.fallback.RecordAttemptVerdict(v)
	}
}

// deliverOne calls a single policy, containing a panic so one bad implementation cannot take
// down the access-log stream and with it every other route's delivery.
//
// The request is already finished, so there is nothing a failure here can be escalated to — the
// only choice is between losing this verdict and losing the stream.
func deliverOne(p policy.AttemptCompletionPolicy, attemptCtx *policy.AttemptCompletionContext,
	params map[string]interface{}, v Verdict) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic delivering attempt verdict to policy",
				"error", r,
				"route", v.RouteName,
				"request_id", v.RequestID,
				"upstream_cluster", v.UpstreamCluster)
		}
	}()
	p.OnAttemptComplete(context.Background(), attemptCtx, params)
}
