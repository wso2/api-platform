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
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// faultTelemetryPolicy is a fault entry that only reports telemetry — the shape the
// analytics collector has on this path.
type faultTelemetryPolicy struct {
	analytics map[string]interface{}
	dynamic   map[string]map[string]interface{}
}

func (p *faultTelemetryPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (p *faultTelemetryPolicy) OnFault(_ context.Context, _ *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	return &policy.FaultResponse{AnalyticsMetadata: p.analytics, DynamicMetadata: p.dynamic}
}

// A fault entry's telemetry has to survive the REJECTION path, not only the response path.
//
// The response path reaches the wire through faultResponseAsModifications, which the
// response translator already merges. A rejection is translated straight from the
// ImmediateResponse, so before foldFaultTelemetry an entry's AnalyticsMetadata was built,
// handed back and silently discarded — and a request-phase rejection is where most policy
// faults happen, so that was nearly all of them.
func TestFaultTelemetry_SurvivesARejection(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{
		&faultTelemetryPolicy{
			analytics: map[string]interface{}{"x-wso2-fault-code": "900902", "x-wso2-fault-status": 401},
			dynamic:   map[string]map[string]interface{}{"envoy.test": {"k": "v"}},
		},
	}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Body:       []byte(`{"error":"unauthorized"}`),
	})

	require.NotNil(t, out.AnalyticsMetadata, "the entry's telemetry must reach the response")
	assert.Equal(t, "900902", out.AnalyticsMetadata["x-wso2-fault-code"])
	assert.Equal(t, 401, out.AnalyticsMetadata["x-wso2-fault-status"])

	// Also written to the accumulator, because the two translators disagree about where to
	// look: the request short-circuit seeds from execCtx, the response one reads only the
	// response object. Both are covered only if both are written.
	assert.Equal(t, "900902", ec.analyticsMetadata["x-wso2-fault-code"],
		"the request-phase translator seeds from the accumulator")

	require.Contains(t, out.DynamicMetadata, "envoy.test")
	assert.Equal(t, "v", out.DynamicMetadata["envoy.test"]["k"])
	assert.Equal(t, "v", ec.dynamicMetadata["envoy.test"]["k"])
}

// Telemetry must not come at the cost of the response. The collector returns metadata and
// nothing else, so the status and body the client receives have to be exactly what they
// would have been without it.
func TestFaultTelemetry_DoesNotDisturbTheResponse(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{
		&faultTelemetryPolicy{analytics: map[string]interface{}{"x-wso2-fault-source": "gateway"}},
	}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 429,
		Body:       []byte(`{"error":"throttled"}`),
	})

	assert.Equal(t, 429, out.StatusCode, "a telemetry-only entry must not change the status")
	assert.JSONEq(t, `{"error":"throttled"}`, string(out.Body),
		"nor the body the producing policy authored")
}

// An entry that changed nothing at all (nil return) must not leave an empty metadata map
// behind: buildAnalyticsStruct is called with whatever is there, and an empty map is a
// different thing from absent for a consumer reading the event.
func TestFaultTelemetry_NilReturnAddsNothing(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order},
	}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 500,
	})

	assert.Equal(t, []string{"notify"}, order, "the entry did run")
	assert.Nil(t, out.AnalyticsMetadata, "a notify-only entry contributes no telemetry")
}
