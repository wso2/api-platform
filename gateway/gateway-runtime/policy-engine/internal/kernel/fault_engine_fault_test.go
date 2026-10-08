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
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The gap this closes: a policy chain that FAILED produced a 500 that no fault policy could
// see. It is the most important failure on the API — more so than an auth rejection — and it
// was the one class of gateway error with no fault coverage at all, because it built its
// ext_proc response directly instead of going through an action.
func TestEngineError_PolicyChainFailureReachesFaultPolicies(t *testing.T) {
	var order []string
	var saw policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &saw},
	}, specs(1))

	resp := ec.handlePolicyError(context.Background(), errors.New("policy blew up"), "request_headers")

	require.Equal(t, []string{"notify"}, order, "an engine failure must reach the fault policies")
	require.NotNil(t, saw.Fault, "and must arrive described, not as a bare status")
	assert.Equal(t, codeEngineInternal, saw.Fault.Code)
	assert.Equal(t, policy.FaultTypeInternal, saw.Fault.Type)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, int32(http.StatusInternalServerError), int32(imm.GetStatus().GetCode()))
}

// A body over the decompression ceiling is the gateway's own rejection too, and gets the same
// treatment — with a REQUEST direction, since nothing had reached the upstream.
func TestEngineError_PayloadTooLargeReachesFaultPolicies(t *testing.T) {
	var order []string
	var saw policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &saw},
	}, specs(1))

	resp := ec.handlePayloadTooLarge(context.Background(), errors.New("too big"), "request_body")

	require.Equal(t, []string{"notify"}, order)
	require.NotNil(t, saw.Fault)
	assert.Equal(t, codePayloadTooLarge, saw.Fault.Code)
	assert.Equal(t, policy.DirectionRequest, saw.Fault.Direction)
	assert.Equal(t, int32(http.StatusRequestEntityTooLarge),
		int32(resp.GetImmediateResponse().GetStatus().GetCode()))
}

// The correlation id is why these bodies are left authored rather than re-rendered. It is the
// only thread from a client-visible 500 back to the log line naming the cause, and it appears
// in the body as well as the header because a caller reporting a problem quotes the body.
func TestEngineError_CorrelationIDSurvivesTheFaultFlow(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: new([]string)},
	}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindMCP // would otherwise be re-rendered as JSON-RPC

	resp := ec.handlePolicyError(context.Background(), errors.New("boom"), "request_headers")

	imm := resp.GetImmediateResponse()
	body := string(imm.GetBody())
	assert.Contains(t, body, "error_id", "the correlation id must stay in the body")

	var headerID string
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		if strings.EqualFold(h.GetHeader().GetKey(), "x-error-id") {
			headerID = string(h.GetHeader().GetRawValue())
			if headerID == "" {
				headerID = h.GetHeader().GetValue()
			}
		}
	}
	require.NotEmpty(t, headerID, "x-error-id must be set")
	assert.Contains(t, body, headerID,
		"the id in the body and the header must be the same one, or neither can be traced")
}

// A fault policy may still take the response over — the side effects are not the only thing
// on offer, they are just the part that always works.
func TestEngineError_FaultPolicyCanReplaceTheResponse(t *testing.T) {
	var order []string
	ec := faultExecCtx(t, []policy.Policy{&faultReplacerPolicy{order: &order}}, specs(1))

	resp := ec.handlePolicyError(context.Background(), errors.New("boom"), "response_body")

	require.Equal(t, []string{"replacer"}, order)
	imm := resp.GetImmediateResponse()
	assert.Equal(t, int32(500), int32(imm.GetStatus().GetCode()))
	assert.Contains(t, string(imm.GetBody()), "by fault policies",
		"the replacement must reach the wire, not just the context")
}

// With no fault policies configured — the common case — the engine's own failure keeps the
// body it has always had, even on a kind the formatter supports.
//
// Formatting renders only what a POLICY described (faultformat.Input.PolicyDescribed), and an
// engine failure is described by the engine. Released gateways sent this exact body, and what
// must survive is the status and the correlation id: an id that appears only in a log nobody
// correlated is not an id.
func TestEngineError_KeepsItsOwnBodyEvenWhenTheKindIsEnabled(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)

	resp := ec.handlePolicyError(context.Background(), errors.New("boom"), "request_headers")

	imm := resp.GetImmediateResponse()
	assert.Equal(t, int32(500), int32(imm.GetStatus().GetCode()))
	assert.False(t, ec.faultPoliciesRan)

	var body map[string]any
	require.NoError(t, json.Unmarshal(imm.GetBody(), &body),
		"engine errors must be valid JSON: %s", imm.GetBody())
	assert.Equal(t, "Internal Server Error", body["error"], "the body released gateways sent")
	assert.NotEmpty(t, body["error_id"], "and it still carries the correlation id")
	assert.NotContains(t, body, "code", "an engine-described failure gains no rendered envelope")
}

// The same failure on a gateway that enabled nothing: the engine's own literal body reaches
// the client, unrendered. That is the pre-formatter output, and the correlation id has to
// survive in it — an operator debugging a 500 has only the id to go on, and it must be there
// whether or not anyone turned formatting on.
func TestEngineError_UnrenderedBodyStillCarriesTheCorrelationIDWhenTheKindIsDisabled(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi // deliberately NOT enabled

	resp := ec.handlePolicyError(context.Background(), errors.New("boom"), "request_headers")

	imm := resp.GetImmediateResponse()
	assert.Equal(t, int32(500), int32(imm.GetStatus().GetCode()))

	var body map[string]any
	require.NoError(t, json.Unmarshal(imm.GetBody(), &body),
		"the engine's own error body must be valid JSON: %s", imm.GetBody())
	assert.NotEmpty(t, body["error_id"], "the correlation id must survive without formatting")
	assert.NotContains(t, body, "code",
		"an unenabled kind must not gain the rendered envelope")
}
