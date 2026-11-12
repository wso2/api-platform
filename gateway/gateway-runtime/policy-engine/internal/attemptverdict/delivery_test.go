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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// completionSpy implements AttemptCompletionPolicy and records what it was handed.
type completionSpy struct {
	calls  []*policy.AttemptCompletionContext
	params []map[string]interface{}
	panics bool
}

func (s *completionSpy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

func (s *completionSpy) OnAttemptComplete(_ context.Context, c *policy.AttemptCompletionContext,
	params map[string]interface{}) {
	if s.panics {
		panic("policy blew up")
	}
	s.calls = append(s.calls, c)
	s.params = append(s.params, params)
}

// plainPolicy implements no completion hook — the common case on nearly every route.
type plainPolicy struct{}

func (plainPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

func chainWith(policies ...policy.Policy) *registry.PolicyChain {
	specs := make([]policy.PolicySpec, len(policies))
	for i := range policies {
		specs[i] = policy.PolicySpec{
			Parameters: policy.PolicyParameters{
				Raw: map[string]interface{}{"index": i},
			},
		}
	}
	return &registry.PolicyChain{Policies: policies, PolicySpecs: specs}
}

func upeVerdict(route string) Verdict {
	return Verdict{
		RequestID:       "req-1",
		RouteName:       route,
		UpstreamCluster: "cluster-a",
		Flags:           []string{"UPE"},
		ResponseCode:    200,
	}
}

func TestPolicyDeliverySink_DeliversToTheRoutesPolicy(t *testing.T) {
	spy := &completionSpy{}
	sink := NewPolicyDeliverySink(func(route string) *registry.PolicyChain {
		if route == "GET:/orders" {
			return chainWith(plainPolicy{}, spy)
		}
		return nil
	}, &LoggingSink{})

	sink.RecordAttemptVerdict(upeVerdict("GET:/orders"))

	require.Len(t, spy.calls, 1)
	got := spy.calls[0]
	assert.Equal(t, "req-1", got.RequestID,
		"the request id is the only way a policy can tie this back to what it saw in flight")
	assert.Equal(t, "GET:/orders", got.RouteName)
	assert.Equal(t, "cluster-a", got.UpstreamCluster)
	assert.Equal(t, []string{"UPE"}, got.ResponseFlags)
	assert.True(t, got.CommittedBeforeFailing(),
		"200 means the client was told it succeeded — the case no in-flight hook can see")

	require.Len(t, spy.params, 1)
	assert.Equal(t, map[string]interface{}{"index": 1}, spy.params[0],
		"params must be the spec aligned with THIS policy's position, not the first one's")
}

// TestPolicyDeliverySink_AttributesByRouteNotCluster is the point of structural delivery.
//
// An LLM proxy forwards through the provider's own route, so one client call produces two flagged
// entries under a single request id — observed at runtime, see poc-results.md. Attribution by
// cluster name would record two failures for one request and would try to grade a loopback hop
// that is not a failover target at all. Keying on the route gives one delivery, to the one route
// that actually carries the policy.
func TestPolicyDeliverySink_AttributesByRouteNotCluster(t *testing.T) {
	spy := &completionSpy{}
	sink := NewPolicyDeliverySink(func(route string) *registry.PolicyChain {
		if route == "POST:/proxy" {
			return chainWith(spy)
		}
		return nil // the provider-route hop carries no chain of its own
	}, &LoggingSink{})

	// Both hops of the same client call, same request id, different routes and clusters.
	proxyHop := upeVerdict("POST:/proxy")
	loopbackHop := upeVerdict("POST:/provider")
	loopbackHop.UpstreamCluster = "upstream_main_host_9099"

	sink.RecordAttemptVerdict(proxyHop)
	sink.RecordAttemptVerdict(loopbackHop)

	assert.Len(t, spy.calls, 1, "one client call must produce exactly one graded failure")
	assert.Equal(t, "cluster-a", spy.calls[0].UpstreamCluster)
}

func TestPolicyDeliverySink_FallsBackWhenNothingConsumes(t *testing.T) {
	// A verdict that reaches no policy must still be visible: silence would hide a
	// misconfiguration that leaves failover grading dead.
	cases := map[string]struct {
		lookup ChainLookup
		route  string
	}{
		"no route name on the entry":  {lookup: func(string) *registry.PolicyChain { return nil }, route: ""},
		"route has no chain":          {lookup: func(string) *registry.PolicyChain { return nil }, route: "GET:/none"},
		"chain has no such policy":    {lookup: func(string) *registry.PolicyChain { return chainWith(plainPolicy{}) }, route: "GET:/plain"},
		"no lookup configured at all": {lookup: nil, route: "GET:/orders"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fallback := &recordingSink{}
			sink := NewPolicyDeliverySink(tc.lookup, fallback)
			sink.RecordAttemptVerdict(upeVerdict(tc.route))
			assert.Len(t, fallback.got, 1, "the verdict must surface somewhere")
		})
	}
}

// TestPolicyDeliverySink_ContainsAPanickingPolicy — delivery runs inline on the access-log
// stream, so an unrecovered panic in one policy would take down verdict delivery for every route
// on the gateway. The request is already over, so there is nothing to escalate to.
func TestPolicyDeliverySink_ContainsAPanickingPolicy(t *testing.T) {
	bad := &completionSpy{panics: true}
	good := &completionSpy{}
	sink := NewPolicyDeliverySink(func(string) *registry.PolicyChain {
		return chainWith(bad, good)
	}, &LoggingSink{})

	require.NotPanics(t, func() { sink.RecordAttemptVerdict(upeVerdict("GET:/orders")) })
	assert.Len(t, good.calls, 1, "a later policy must still receive the verdict")
}
