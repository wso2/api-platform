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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
)

// faultRedescriberPolicy re-describes the failure: it returns a Fault and nothing else.
type faultRedescriberPolicy struct {
	fault *policy.FaultDetails
}

func (p *faultRedescriberPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}
}

func (p *faultRedescriberPolicy) OnFault(_ context.Context, _ *policy.FaultContext,
	_ map[string]interface{}) *policy.FaultResponse {
	return &policy.FaultResponse{Fault: p.fault}
}

// producerDenied is the producing policy's own account of a rejection.
func producerDenied() *policy.FaultDetails {
	return &policy.FaultDetails{Code: "900901", Type: policy.FaultTypeAuthentication, Message: "Denied"}
}

// renamed is a fault entry's re-description. Policy is set to show that the engine, not the
// entry, decides it.
func renamed() *policy.FaultDetails {
	return &policy.FaultDetails{
		Code: "965001", Type: policy.FaultTypeValidation, Message: "Renamed", Policy: "spoofed",
	}
}

// recordRejection does what handleFault does before the chain runs: attribute the rejection
// to the policy that produced it.
func recordRejection(ec *PolicyExecutionContext, declared *policy.FaultDetails) {
	ec.record(faultAttribution{
		policyName: "api-key-auth", version: "v1",
		phase: policy.PolicyPhaseRequestHeaders, declared: declared,
	})
}

// A later entry reads the re-description as FaultContext.Fault — that is what lets the
// analytics collector, appended last, record the account the client receives.
func TestFaultRedescription_ALaterEntrySeesIt(t *testing.T) {
	var order []string
	var seen policy.FaultContext
	redescription := renamed()
	ec := faultExecCtx(t, []policy.Policy{
		&faultRedescriberPolicy{fault: redescription},
		&faultRecorderPolicy{name: "after", order: &order, sawErrCtx: &seen},
	}, specs(2))
	imm := policy.ImmediateResponse{StatusCode: 401, Fault: producerDenied()}
	recordRejection(ec, imm.Fault)

	ec.runFaultPoliciesOnRejection(context.Background(), imm)

	require.NotNil(t, seen.Fault, "the later entry must see a description")
	assert.Equal(t, "965001", seen.Fault.Code)
	assert.Equal(t, policy.FaultTypeValidation, seen.Fault.Type)
	assert.Equal(t, "Renamed", seen.Fault.Message)
	assert.Equal(t, "api-key-auth", seen.Fault.Policy,
		"Policy names who caused the failure; re-describing it does not change that")
	assert.Equal(t, "spoofed", redescription.Policy,
		"the entry's own struct must not be written through")
}

// On a rejection the formatter renders from immResp.Fault, so the re-description must be
// carried there — otherwise the body would describe the producer's account while the
// analytics event described the entry's.
func TestFaultRedescription_TheRejectionFormatterRendersIt(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{&faultRedescriberPolicy{fault: renamed()}}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)
	imm := policy.ImmediateResponse{StatusCode: 401, Fault: producerDenied()}
	recordRejection(ec, imm.Fault)

	out := ec.runFaultPoliciesOnRejection(context.Background(), imm)

	require.NotNil(t, out.Body, "nothing authored a body, so the formatter must render one")
	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body), "body is not valid JSON: %s", out.Body)
	assert.Equal(t, "965001", body["code"], "the re-described code must be rendered")
	assert.Equal(t, "Renamed", body["message"])
	require.NotNil(t, out.Fault)
	assert.Equal(t, "965001", out.Fault.Code, "and the rejection carries it onward")
	assert.Equal(t, 401, out.StatusCode, "re-describing does not change the status")
}

// On the response path the formatter renders from faultDeclared. A router failure is the case
// where it renders at all: no policy produced it and nothing authored a body.
func TestFaultRedescription_TheResponseFormatterRendersIt(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{&faultRedescriberPolicy{fault: renamed()}}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)
	responseHeaderCtxWithStatus(ec, 503)
	routerError(ec)

	execResult := &executor.ResponseExecutionResult{}
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream))

	var rendered []byte
	for _, r := range execResult.Results {
		if r.PolicyName == errorFormatResultName {
			rendered = r.Action.(policy.DownstreamResponseModifications).Body
		}
	}
	require.NotNil(t, rendered, "the formatter must render the router failure")
	var body map[string]any
	require.NoError(t, json.Unmarshal(rendered, &body), "body is not valid JSON: %s", rendered)
	assert.Equal(t, "965001", body["code"],
		"the entry's description, not the engine's router-failure one, must be rendered")
	assert.Equal(t, "Renamed", body["message"])
}

// An entry that returns no Fault leaves the producer's account exactly as it was — the
// common case, and the one this change must not disturb.
func TestFaultRedescription_WithoutOneTheProducersAccountStands(t *testing.T) {
	var seen policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "annotate", order: new([]string), setHeader: "x-annotated"},
		&faultRecorderPolicy{name: "after", order: new([]string), sawErrCtx: &seen},
	}, specs(2))
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)
	imm := policy.ImmediateResponse{StatusCode: 401, Fault: producerDenied()}
	recordRejection(ec, imm.Fault)

	out := ec.runFaultPoliciesOnRejection(context.Background(), imm)

	require.NotNil(t, seen.Fault)
	assert.Equal(t, "900901", seen.Fault.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body), "body is not valid JSON: %s", out.Body)
	assert.Equal(t, "900901", body["code"])
	assert.Equal(t, "Denied", body["message"])
}
