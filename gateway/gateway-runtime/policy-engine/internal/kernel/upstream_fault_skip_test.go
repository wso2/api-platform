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
	"testing"

	"github.com/stretchr/testify/assert"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A response that arrived already failing runs none of the response policies — operator's or
// platform's. The fault policies handle it instead, from the response-body phase.
func TestUpstreamFault_SkipsEveryResponsePolicy(t *testing.T) {
	chainOf := func() ([]policy.Policy, []policy.PolicySpec) {
		return []policy.Policy{
				&faultRecorderPolicy{name: "transformer", order: new([]string)},
				&faultRecorderPolicy{name: "analytics", order: new([]string)},
				&faultRecorderPolicy{name: "guardrail", order: new([]string)},
			}, []policy.PolicySpec{
				{Name: "json-xml-mediator", Version: "v1", Enabled: true},
				{Name: "wso2_apip_sys_analytics", Version: "v1", Enabled: true},
				{Name: "word-count-guardrail", Version: "v1", Enabled: true},
			}
	}

	t.Run("a healthy response runs every policy", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		ec.policyChain.Policies, ec.policyChain.PolicySpecs = chainOf()
		ec.noteUpstreamFault(200)

		pols, specs := ec.responsePolicies()

		assert.False(t, ec.upstreamFault, "200 is not an upstream fault")
		assert.Len(t, pols, 3, "nothing is skipped on a healthy response")
		assert.Len(t, specs, 3, "and the parallel slices stay in step")
	})

	t.Run("an upstream error runs none of them", func(t *testing.T) {
		ec := faultExecCtx(t, nil, nil)
		ec.policyChain.Policies, ec.policyChain.PolicySpecs = chainOf()
		ec.noteUpstreamFault(503)

		pols, specs := ec.responsePolicies()

		assert.True(t, ec.upstreamFault)
		assert.Empty(t, pols, "the fault policies replace the response policies here")
		assert.Empty(t, specs)
	})

	// The boundary, because the skip and the fault gate share faultMinStatus and an
	// off-by-one in either would be invisible to a test that only checked 200 and 503.
	t.Run("the boundary is 400", func(t *testing.T) {
		for _, tc := range []struct {
			status int
			skip   bool
		}{{200, false}, {399, false}, {400, true}, {503, true}} {
			ec := faultExecCtx(t, nil, nil)
			ec.noteUpstreamFault(tc.status)
			assert.Equal(t, tc.skip, ec.upstreamFault, "status %d", tc.status)
		}
	})
}
