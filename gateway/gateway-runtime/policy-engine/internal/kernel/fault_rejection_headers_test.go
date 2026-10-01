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

	"github.com/stretchr/testify/assert"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// headerChangingFaultPolicy returns a fixed set of header changes.
type headerChangingFaultPolicy struct{ resp policy.FaultResponse }

func (p *headerChangingFaultPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (p *headerChangingFaultPolicy) OnFault(_ context.Context, _ *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	r := p.resp
	return &r
}

// A rejection's headers are rebuilt from the fault chain's view alone, so every header
// operation a fault policy returns reaches the client — including the two a rejection used
// to lose.
func TestFaultPolicies_Rejection_AppliesEveryHeaderOperation(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{
		&headerChangingFaultPolicy{resp: policy.FaultResponse{
			HeadersToSet:    map[string]string{"X-Error-Code": "900902"},
			HeadersToAppend: map[string][]string{"x-trace": {"first", "second"}},
			HeadersToRemove: []string{"X-Internal"},
		}},
	}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Headers: map[string]string{
			"content-type": "application/json",
			"x-error-code": "stale",
			"x-internal":   "secret",
		},
	})

	assert.Equal(t, "first, second", out.Headers["x-trace"],
		"appended values reach the client, combined into one field")
	assert.NotContains(t, out.Headers, "x-internal", "a removed header must not come back")
	assert.Equal(t, "900902", out.Headers["x-error-code"],
		"a mixed-case set replaces the header rather than sitting beside it")
	assert.NotContains(t, out.Headers, "X-Error-Code")
	assert.Equal(t, "application/json", out.Headers["content-type"], "untouched headers survive")
}

// A later entry sees what an earlier one appended, as it does for every other change.
func TestFaultPolicies_Rejection_AppendIsVisibleToTheNextEntry(t *testing.T) {
	var seen policy.FaultContext
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&headerChangingFaultPolicy{resp: policy.FaultResponse{
			HeadersToAppend: map[string][]string{"x-trace": {"first"}},
		}},
		&faultRecorderPolicy{name: "observer", order: &order, sawErrCtx: &seen},
	}, specs(2))

	ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 401})

	assert.Equal(t, []string{"first"}, seen.ResponseHeaders.Get("x-trace"))
}
