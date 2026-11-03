/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package kernel

import (
	"context"
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A chain bound by a resolver runs from resolution.go rather than from
// processRequestHeaders/processRequestBody, so it is the one execution path that has to
// raise its own fault. It regressed the moment a multiplexed route started deferring:
// every rejection on an MCP or A2A operation reached the client with the fault policies
// never consulted, and nothing failed, because the rejection itself still looked right.
//
// Asserted through the header a fault policy sets, which is the only evidence visible on
// the wire that the chain ran at all.
func TestDeferredBinding_RejectionStillRunsFaultPolicies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reject policy.Policy
	}{
		{"rejected by a header policy", &headerPolicy{statusCode: 401}},
		{"rejected by a body policy", &rejectingBodyPolicy{statusCode: 403}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeOperationResolver{
				name:      "body",
				reqs:      resolver.RequestRequirements{Body: resolver.BodyBuffered},
				bodyField: "method",
			}
			f := newResolutionFixture(t, r)
			f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "body"})

			var order []string
			chain := f.operationChain("SendMessage", tc.reject)
			chain.FaultPolicies = []policy.Policy{
				&faultRecorderPolicy{name: "notify", order: &order, setHeader: "x-fault-handled"},
			}
			chain.FaultPolicySpecs = specs(1)
			chain.HasFaultPolicies = true

			execCtx := f.bindPending(t, "POST|/rpc|example.com")
			resp, err := execCtx.processRequestBody(context.Background(),
				&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
			require.NoError(t, err)

			imm := resp.GetImmediateResponse()
			require.NotNil(t, imm, "the rejection must still be an immediate response")
			assert.Equal(t, []string{"notify"}, order,
				"a rejection on a resolver-bound chain must reach the fault policies")
		})
	}
}

// rejectingBodyPolicy short-circuits at the request-body phase, which on a deferred chain
// is the second of the two places a rejection can originate.
type rejectingBodyPolicy struct {
	statusCode int
}

func (p *rejectingBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer}
}

func (p *rejectingBodyPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext,
	_ map[string]interface{}) policy.RequestAction {
	return policy.ImmediateResponse{
		StatusCode: p.statusCode,
		Body:       []byte(`{"error":"forbidden"}`),
	}
}
