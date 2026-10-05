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

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/faultformat"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// enableFaultFormatter turns on error-body synthesis for the given kinds — what an operator
// does by listing them under [policy_engine.error_response_formatter].enabled_kinds.
//
// Goes through the real faultformat.NewKindSet rather than assigning a map directly, so a
// test cannot enable a kind the config path itself would have rejected.
func enableFaultFormatter(t *testing.T, ec *PolicyExecutionContext, kinds ...policy.APIKind) {
	t.Helper()
	enableFaultFormatterOn(t, ec.server, kinds...)
}

// enableFaultFormatterOn is the same, for a failure that never builds an execution context —
// a route with no policy chain, or a resolver that denied.
func enableFaultFormatterOn(t *testing.T, s *ExternalProcessorServer, kinds ...policy.APIKind) {
	t.Helper()
	names := make([]string, 0, len(kinds))
	for _, k := range kinds {
		names = append(names, string(k))
	}
	// Goes through the real ServerOption rather than assigning the field, so the option is
	// exercised by the tests it exists for. Assigning directly duplicated its body and left
	// the option itself unreferenced — dead code whose own doc said a test needed it.
	WithErrorFormatterKinds(names)(s)
	_, unknown := faultformat.NewKindSet(names)
	require.Empty(t, unknown, "test tried to enable an API kind the config path rejects")
}

// restCtx is an execution context for a plain REST API, whose caller gets the canonical JSON
// error object.
func restCtx(t *testing.T) *PolicyExecutionContext {
	t.Helper()
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)
	return ec
}

// mcpCtx is an execution context for an MCP API — a kind whose clients cannot parse the
// gateway's default JSON error body — with no fault policies configured at all.
func mcpCtx(t *testing.T) *PolicyExecutionContext {
	t.Helper()
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindMCP
	enableFaultFormatter(t, ec, policy.APIKindMCP)
	return ec
}

// The case that justifies moving formatting out of the policy chain: an API with NO fault
// policies still gets a protocol-correct body. As a chain entry this could not run at all,
// because there was no chain to attach it to.
func TestErrorFormat_AppliesWithNoFaultPoliciesConfigured(t *testing.T) {
	ec := mcpCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Fault:      &policy.FaultDetails{Message: "Denied", Code: "900901"},
	})

	require.NotNil(t, out.Body, "an MCP error with no authored body must be formatted")
	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body), "body is not valid JSON: %s", out.Body)
	assert.Equal(t, "2.0", body["jsonrpc"], "MCP errors must be JSON-RPC: %s", out.Body)
	assert.Equal(t, "application/json", out.Headers[contentTypeHeader])
	assert.Equal(t, 401, out.StatusCode, "formatting must never change the status")
}

// The override path, and the whole of it: a policy keeps its body by NOT describing an error.
//
// "Author a body" used to be the override. It cannot be any more, because a policy that must
// also run on a gateway too old to render carries a fallback body alongside its description —
// so a body plus a description is a request to render, not a decision to keep.
func TestErrorFormat_AuthoredBodyIsLeftAloneWhenNothingWasDescribed(t *testing.T) {
	ec := mcpCtx(t)
	custom := []byte(`{"my":"format"}`)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Body:       custom,
	})

	assert.Equal(t, custom, out.Body, "an authored body must survive untouched")
	assert.Empty(t, out.Headers[contentTypeHeader], "and its content type must not be overwritten")
}

// The compatibility case this rule exists for. A policy sets BOTH: the description for a
// gateway that renders, and a body for one that does not. This gateway renders — otherwise the
// fallback would permanently suppress the rendering it stands in for, and an MCP client would
// receive the policy's REST-shaped JSON instead of JSON-RPC.
// The compatibility case this rule exists for. A policy sets BOTH: the description for a
// gateway that renders, and a body for one that does not. This gateway renders — otherwise the
// fallback would permanently suppress the rendering it stands in for, and an MCP client would
// receive the policy's REST-shaped JSON instead of JSON-RPC.
func TestErrorFormat_BodyAccompanyingADescriptionIsAFallbackAndIsReplaced(t *testing.T) {
	ec := mcpCtx(t)
	// Whatever the policy wrote before it was migrated to describe its failures — on an old
	// gateway this is still exactly what the client gets, which is the point of keeping it.
	fallback := []byte(`{"error":"Unauthorized","message":"Denied"}`)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Body:       fallback,
		Fault:      &policy.FaultDetails{Message: "Denied", Code: "900901"},
	})

	require.NotEqual(t, fallback, out.Body, "the fallback must not reach an MCP client")
	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body), "body is not valid JSON: %s", out.Body)
	assert.Equal(t, "2.0", body["jsonrpc"], "the described error must be rendered as JSON-RPC")
	assert.Equal(t, "application/json", out.Headers[contentTypeHeader])
}

// The kind gate, which is what makes this shippable in a minor release: an API kind the
// operator did not enable is left exactly as it would be with no formatter at all.
func TestErrorFormat_DisabledKindIsLeftAlone(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindMCP // enabled for NOTHING — no option applied

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Fault:      &policy.FaultDetails{Message: "Denied", Code: "900901"},
	})

	assert.Nil(t, out.Body,
		"a kind the operator did not enable must not gain a synthesized body")
	assert.Equal(t, 401, out.StatusCode)
}

// Enabling one kind must not enable another. A gateway serving both REST and MCP has to be
// able to turn formatting on for the clients that need it without touching the others.
func TestErrorFormat_EnablingOneKindDoesNotEnableAnother(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindMCP)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Fault:      &policy.FaultDetails{Message: "Denied", Code: "900901"},
	})

	assert.Nil(t, out.Body, "REST is not enabled, so nothing may be synthesized for it")
}

// An explicitly EMPTY body is a decision, not an absence. The SDK defines nil as "no
// opinion" and []byte{} as "clear the body"; formatting the latter would override a policy
// that deliberately chose to return nothing.
func TestErrorFormat_ExplicitlyEmptyBodyIsAnAuthoredDecision(t *testing.T) {
	ec := mcpCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Body:       []byte{},
	})

	assert.NotNil(t, out.Body)
	assert.Empty(t, out.Body, "an explicitly empty body must stay empty")
}

// The guardrail block is rendered when the policy attached one, and its PRESENCE is the
// opt-in: a guardrail attaches it only when its own showAssessment parameter says the
// assessment may be returned. So the operator's existing switch decides, and the formatter
// makes no safety judgement of its own.
func TestErrorFormat_GuardrailBlockIsRenderedWhenThePolicyAttachedOne(t *testing.T) {
	ec := restCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Fault: &policy.FaultDetails{
			Code:        "906000",
			Type:        "guardrail",
			Message:     "Violation of applied word count constraints detected",
			Description: "the blocked content itself",
			Guardrail: &policy.GuardrailDetails{
				InterveningGuardrail: "word-count-guardrail",
				Action:               policy.GuardrailActionIntervened,
				ActionReason:         "Violation of applied word count constraints detected",
				Assessments: map[string]any{
					"assessments": "Expected word count to be between 10 and 500 words.",
				},
			},
		},
	})

	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body), "body is not valid JSON: %s", out.Body)
	guardrail, ok := body["guardrail"].(map[string]any)
	require.True(t, ok, "the guardrail block must be rendered: %s", out.Body)
	assert.Equal(t, "word-count-guardrail", guardrail["interveningGuardrail"])
	assert.Equal(t, policy.GuardrailActionIntervened, guardrail["action"])
	assert.NotNil(t, guardrail["assessments"], "the opted-in assessment must reach the client")

	assert.NotContains(t, string(out.Body), "the blocked content itself",
		"Description must never be rendered, whatever else is")
}

// No block attached means the operator did not opt in, and the formatter must not invent one.
// An empty `guardrail: {}` would be worse than its absence: it says there is an assessment and
// then carries none.
func TestErrorFormat_NoGuardrailBlockWhenThePolicyAttachedNone(t *testing.T) {
	ec := restCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Fault: &policy.FaultDetails{
			Code:    "906000",
			Type:    "guardrail",
			Message: "Violation of applied word count constraints detected",
		},
	})

	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body), "body is not valid JSON: %s", out.Body)
	assert.NotContains(t, body, "guardrail")
	assert.Equal(t, "906000", body["code"], "the rest of the envelope is unaffected")
}

// A plain REST caller now gets canonical JSON rather than nothing.
//
// This reverses the earlier default. It is safe precisely because of the authorship gate: the
// formatter is only consulted when no policy authored a body, so the alternative for this
// caller is an EMPTY body — worse than a generic one, since a client parsing JSON gets a parse
// failure instead of a message. A policy that still writes its own body is untouched.
func TestErrorFormat_PlainRestGetsCanonicalJSON(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	enableFaultFormatter(t, ec, policy.APIKindRestApi)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Fault:      &policy.FaultDetails{Code: "900901", Message: "Denied"},
	})

	require.NotNil(t, out.Body, "an unauthored REST error must not reach the client empty")
	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body))
	assert.Equal(t, "Denied", body["message"])
	assert.Equal(t, "900901", body["code"])
	assert.Equal(t, "application/json", out.Headers[contentTypeHeader])
}

// A HEAD response has no body by definition, so none may be synthesized — Envoy recalculates
// content-length from the mutation, so a body here contradicts the method.
func TestErrorFormat_HeadResponseGetsNoBody(t *testing.T) {
	ec := mcpCtx(t)
	ec.requestHeaderCtx.Method = "HEAD"

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Fault:      &policy.FaultDetails{Message: "Denied"},
	})

	assert.Nil(t, out.Body, "a HEAD response must not gain a body")
	assert.Empty(t, out.Headers[contentTypeHeader])
}

// A rejection that no policy described is left exactly as it was, even on an enabled kind:
// a policy that predates the fault contract describes nothing, and released gateways sent
// its errors unformatted. Rendering them would change what an existing client receives.
func TestErrorFormat_UndescribedErrorIsLeftAlone(t *testing.T) {
	ec := mcpCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: 503})

	assert.Nil(t, out.Body, "an undescribed rejection must not gain a body")
	assert.Empty(t, out.Headers[contentTypeHeader])
	assert.Equal(t, 503, out.StatusCode)
}

// Described, but sparsely: a policy that declared a Fault with no fields set still asked for
// rendering, and a sparse description must still produce a valid document with a usable
// message — the case an MCP client cannot otherwise parse.
func TestErrorFormat_SparseDescriptionStillRenders(t *testing.T) {
	ec := mcpCtx(t)

	out := ec.runFaultPoliciesOnRejection(context.Background(),
		policy.ImmediateResponse{StatusCode: 503, Fault: &policy.FaultDetails{}})

	require.NotNil(t, out.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(out.Body, &body))
	errObj, _ := body["error"].(map[string]any)
	require.NotNil(t, errObj, "sparse error must still produce a JSON-RPC error object: %s", out.Body)
	assert.NotEmpty(t, errObj["message"], "and a usable message: %s", out.Body)
}

// A fault policy that authors a body switches formatting off — this is how a customer ships
// a custom error format. Ordering is irrelevant: the signal is that a body exists.
func TestErrorFormat_FaultPolicyAuthoringABodyDisablesFormatting(t *testing.T) {
	custom := []byte(`<custom/>`)
	ec := faultExecCtx(t, []policy.Policy{
		&bodyAuthoringFaultPolicy{body: custom},
	}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindMCP

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Fault:      &policy.FaultDetails{Message: "Rejected"},
	})

	assert.Equal(t, custom, out.Body,
		"the fault policy's body must win; the built-in formatter must not overwrite it")
}

// The status belongs to the response, not the body. A SOAP-conformant 500 in place of a 401
// would destroy the signal analytics and client retry logic depend on.
func TestErrorFormat_NeverChangesTheStatus(t *testing.T) {
	for _, status := range []int{401, 403, 422, 429, 503} {
		ec := mcpCtx(t)
		out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{StatusCode: status})
		assert.Equal(t, status, out.StatusCode)
	}
}

// bodyAuthoringFaultPolicy is a custom formatter: it decides the body in OnFault, which is
// the contract a customer-supplied formatter policy implements.
type bodyAuthoringFaultPolicy struct {
	body []byte
}

func (p *bodyAuthoringFaultPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *bodyAuthoringFaultPolicy) OnFault(
	_ context.Context, _ *policy.FaultContext, _ map[string]interface{},
) *policy.FaultResponse {
	return &policy.FaultResponse{
		Body:         p.body,
		HeadersToSet: map[string]string{contentTypeHeader: "application/xml"},
	}
}

// A REJECTING policy's body lives in the response chain's results, not the fault chain's — a
// response guardrail is the usual case. The formatter must respect it.
//
// Regression: the first version of the authorship gate scanned only the fault chain, so the
// formatter overwrote a guardrail's own rejection body with a generic one.
func TestErrorFormat_RespectsARejectingPolicysBodyFromTheResponseChain(t *testing.T) {
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notifier", order: new([]string), setHeader: "x-fault-notified"},
	}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindMCP
	withRejectedBody(ec, 446)

	// bodyRejection carries the guardrail's own authored body in the response results.
	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	for _, r := range execResult.Results {
		if r.PolicyName == errorFormatResultName {
			t.Fatalf("formatter overwrote a rejecting policy's body: %s",
				r.Action.(policy.DownstreamResponseModifications).Body)
		}
	}
}
