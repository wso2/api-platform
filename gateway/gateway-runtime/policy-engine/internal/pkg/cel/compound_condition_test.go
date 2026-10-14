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

package cel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The demo's compound conditions, evaluated against the two FaultContexts the demo
// actually produces. Pins that && and || traverse BOTH dimensions rather than one.
func TestCompoundConditions_CrossDimension(t *testing.T) {
	ev, err := NewCELEvaluator()
	require.NoError(t, err)

	shared := func() *policy.SharedContext {
		return &policy.SharedContext{
			APIName: "faultdemo-cond-compound", APIVersion: "v1.0",
			Metadata: map[string]interface{}{},
		}
	}
	backend503 := &policy.FaultContext{
		SharedContext: shared(),
		Source:        string(policy.FaultSourceBackend), ResponseStatus: 503,
	}
	backend404 := &policy.FaultContext{
		SharedContext: shared(),
		Source:        string(policy.FaultSourceBackend), ResponseStatus: 404,
	}
	gateway503 := &policy.FaultContext{
		SharedContext:  shared(),
		Source:         string(policy.FaultSourceGateway),
		ResponseStatus: 503,
		Fault:          &policy.FaultDetails{Type: policy.FaultTypeUpstream, Code: "101503"},
	}

	conds := map[string]string{
		"backend-5xx":     `fault.Source == "backend" && fault.Status >= 500`,
		"backend-4xx":     `fault.Source == "backend" && fault.Status >= 400 && fault.Status < 500`,
		"gateway-5xx":     `fault.Source == "gateway" && fault.Status >= 500`,
		"any-5xx":         `fault.Source == "router" || fault.Status >= 500`,
		"not-backend-4xx": `!(fault.Source == "backend") && fault.Status >= 400`,
		"three":           `fault.Source == "gateway" && fault.Type == "upstream" && fault.Status == 503`,
	}

	for _, tc := range []struct {
		name string
		ctx  *policy.FaultContext
		want []string
	}{
		{"backend 503", backend503, []string{"any-5xx", "backend-5xx"}},
		{"backend 404", backend404, []string{"backend-4xx"}},
		{"gateway 503", gateway503, []string{"any-5xx", "gateway-5xx", "not-backend-4xx", "three"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fired []string
			for name, expr := range conds {
				ok, err := ev.EvaluateFaultCondition(expr, tc.ctx)
				require.NoError(t, err, "%s must evaluate, not error", name)
				if ok {
					fired = append(fired, name)
				}
			}
			assert.ElementsMatch(t, tc.want, fired)
		})
	}
}

// A FaultContext with no SharedContext must evaluate, not panic.
//
// The kernel guards ec.sharedCtx before reading APIName, so nil is a shape it already
// treats as reachable — and this activation is built for a request that is ALREADY
// failing, so a panic here converts a served error into a dropped one for every request
// on the route. Cheaper to be nil-safe than to prove the shape unreachable.
func TestFaultCondition_NilSharedContextDoesNotPanic(t *testing.T) {
	ev, err := NewCELEvaluator()
	require.NoError(t, err)

	got, err := ev.EvaluateFaultCondition(
		`fault.Source == "backend" && fault.Status >= 500`,
		&policy.FaultContext{Source: string(policy.FaultSourceBackend), ResponseStatus: 503},
	)
	require.NoError(t, err)
	assert.True(t, got, "the fault.* terms must still evaluate with no SharedContext")

	// And a condition reaching the fields that LIVE on SharedContext reads empty.
	got, err = ev.EvaluateFaultCondition(`request.RequestID == ""`,
		&policy.FaultContext{Source: string(policy.FaultSourceGateway), ResponseStatus: 500})
	require.NoError(t, err)
	assert.True(t, got, "an absent SharedContext must read as empty, not error")
}
