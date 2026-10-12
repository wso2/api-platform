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

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// llmCtx is an LlmProxy execution context with policy_engine.llm_openai_compatible_errors.enabled
// switched on through the real ServerOption.
func llmCtx(t *testing.T, faultPolicies []policy.Policy, faultSpecs []policy.PolicySpec) *PolicyExecutionContext {
	t.Helper()
	ec := faultExecCtx(t, faultPolicies, faultSpecs)
	ec.sharedCtx.APIKind = policy.APIKindLlmProxy
	WithLLMOpenAIErrors(true)(ec.server)
	return ec
}

// openAIMessageOf asserts body is the OpenAI envelope and returns its inner object.
func openAIErrorOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var envelope map[string]map[string]any
	require.NoError(t, json.Unmarshal(body, &envelope), "body is not the OpenAI envelope: %s", body)
	inner, ok := envelope["error"]
	require.True(t, ok, "missing `error` object: %s", body)
	return inner
}

// formattedResult returns the body the formatter appended in the response phase, or nil.
func formattedResult(execResult *executor.ResponseExecutionResult) []byte {
	for _, r := range execResult.Results {
		if r.PolicyName == errorFormatResultName {
			return r.Action.(policy.DownstreamResponseModifications).Body
		}
	}
	return nil
}

func TestLLMOpenAIErrors_OffLeavesLLMErrorsAsTheyWere(t *testing.T) {
	ec := faultExecCtx(t, nil, nil)
	ec.sharedCtx.APIKind = policy.APIKindLlmProxy
	legacy := []byte(`{"error":"Unauthorized","message":"Invalid API key"}`)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Body:       legacy,
		Fault:      nil,
	})
	assert.Equal(t, legacy, out.Body, "without the option an LLM error is untouched")

	described := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Fault:      &policy.FaultDetails{Message: "Denied"},
	})
	assert.Nil(t, described.Body, "LLM kinds are not in the shipped formatter set")
}

func TestLLMOpenAIErrors_OptionDoesNotDisturbTheShippedKinds(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	assert.True(t, ec.server.errorFormatterKinds.Enabled(policy.APIKindAgent))
	assert.True(t, ec.server.errorFormatterKinds.Enabled(policy.APIKindLlmProvider))
	assert.False(t, ec.server.errorFormatterKinds.Enabled(policy.APIKindRestApi))
}

// The case option B exists for: a policy written for REST APIs still writes its own JSON on an
// LLM route, and the OpenAI SDK on the other end must get its envelope.
func TestLLMOpenAIErrors_ReshapesARejectingPolicysOwnBody(t *testing.T) {
	ec := llmCtx(t, nil, nil)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    map[string]string{contentTypeHeader: "text/plain"},
		Body:       []byte(`{"error":"Unauthorized","message":"Invalid API key"}`),
	})

	inner := openAIErrorOf(t, out.Body)
	assert.Equal(t, "Invalid API key", inner["message"])
	assert.Equal(t, "authentication_error", inner["type"])
	assert.Equal(t, "application/json", out.Headers[contentTypeHeader])
	assert.Equal(t, 401, out.StatusCode, "the status is never changed")
}

func TestLLMOpenAIErrors_RendersADescribedRejection(t *testing.T) {
	ec := llmCtx(t, nil, nil)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 429,
		Fault: &policy.FaultDetails{
			Code: "900802", Type: policy.FaultTypeThrottling, Message: "Rate limit exceeded",
		},
	})

	inner := openAIErrorOf(t, out.Body)
	assert.Equal(t, "Rate limit exceeded", inner["message"])
	assert.Equal(t, "rate_limit_error", inner["type"])
	assert.Equal(t, "900802", inner["code"])
}

// An operator's own fault policy writing a body is a deliberate decision, and it still wins —
// including over a producer body that would otherwise have been reshaped.
func TestLLMOpenAIErrors_FaultPolicyBodyStillWins(t *testing.T) {
	custom := []byte(`{"operator":"format"}`)
	ec := llmCtx(t, []policy.Policy{&bodyAuthoringFaultPolicy{body: custom}}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 401,
		Body:       []byte(`{"error":"Unauthorized"}`),
	})

	assert.Equal(t, custom, out.Body)
}

// A response guardrail's rejection body lives in the response chain's results. On an LLM route
// it is reshaped like any other policy-authored body; elsewhere it still stands
// (TestErrorFormat_RespectsARejectingPolicysBodyFromTheResponseChain).
func TestLLMOpenAIErrors_ReshapesAResponseGuardrailsBody(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	formatted := formattedResult(execResult)
	require.NotNil(t, formatted, "the guardrail's REST-shaped body must not reach an OpenAI SDK")
	assert.Equal(t, "word count violation", openAIErrorOf(t, formatted)["message"])
}

// A fault entry runs BEFORE the response-phase formatter, so a re-description of its is the
// final account of the failure. The producing guardrail's legacy body must not replace it.
func TestLLMOpenAIErrors_FaultRedescriptionWinsOverAReshapedProducerBody(t *testing.T) {
	ec := llmCtx(t, []policy.Policy{&faultRedescriberPolicy{fault: &policy.FaultDetails{
		Message: "Blocked by policy", Code: "906201", Type: policy.FaultTypeGuardrail,
	}}}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	formatted := formattedResult(execResult)
	require.NotNil(t, formatted)
	inner := openAIErrorOf(t, formatted)
	assert.Equal(t, "Blocked by policy", inner["message"], "not the producer body's \"word count violation\"")
	assert.Equal(t, "906201", inner["code"])
	assert.Equal(t, "invalid_request_error", inner["type"])
}

// A re-description without a message keeps every field it set; the producing policy's legacy
// body only fills the message in.
func TestLLMOpenAIErrors_ProducerBodyMessageFillsAMessagelessRedescription(t *testing.T) {
	redescription := &policy.FaultDetails{
		Code: "906201", Type: policy.FaultTypeGuardrail, Direction: "response",
		Guardrail: &policy.GuardrailDetails{
			InterveningGuardrail: "word-count-guardrail",
			Action:               policy.GuardrailActionIntervened,
			ActionReason:         "too long",
		},
	}
	ec := llmCtx(t, []policy.Policy{&faultRedescriberPolicy{fault: redescription}}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	formatted := formattedResult(execResult)
	require.NotNil(t, formatted)
	inner := openAIErrorOf(t, formatted)
	assert.Equal(t, "word count violation", inner["message"], "lifted from the producer body")
	assert.Equal(t, "906201", inner["code"])
	assert.Equal(t, "invalid_request_error", inner["type"])
	guardrail, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok, "the re-description's guardrail block survives: %s", formatted)
	assert.Equal(t, "word-count-guardrail", guardrail["interveningGuardrail"])
	assert.Equal(t, "too long", guardrail["actionReason"])
	assert.Equal(t, "RESPONSE", guardrail["direction"])

	assert.Empty(t, redescription.Message, "the fault entry's own description is not mutated")
	assert.Equal(t, "word count violation", ec.faultDeclared.Message)
	assert.Equal(t, "906201", ec.faultDeclared.Code)
}

// A fault entry that writes a body still decides what the client receives on the response
// path, over the producing guardrail's reshaped body.
func TestLLMOpenAIErrors_FaultPolicyBodyStillWinsOnTheResponsePath(t *testing.T) {
	custom := []byte(`{"operator":"format"}`)
	ec := llmCtx(t, []policy.Policy{&bodyAuthoringFaultPolicy{body: custom}}, specs(1))
	withRejectedBody(ec, 446)

	execResult := bodyRejection(446)
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	assert.Nil(t, formattedResult(execResult), "the formatter must not overwrite the fault policy's body")
	last := execResult.Results[len(execResult.Results)-1].Action.(policy.DownstreamResponseModifications)
	assert.Equal(t, custom, last.Body)
}

// An explicitly empty body is the policy's decision that the client gets nothing — on either
// path, the same as a provider's bodyless error.
func TestLLMOpenAIErrors_ExplicitlyEmptyPolicyBodyStaysEmpty(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 403, Body: []byte{},
	})
	assert.Equal(t, []byte{}, out.Body, "rejection path")

	ec = llmCtx(t, nil, nil)
	withRejectedBody(ec, 446)
	s := 446
	execResult := &executor.ResponseExecutionResult{Results: []executor.ResponsePolicyResult{{
		PolicyName: "word-count-guardrail", PolicyVersion: "v1",
		Action: policy.DownstreamResponseModifications{StatusCode: &s, Body: []byte{}},
	}}}
	ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway)
	assert.Nil(t, formattedResult(execResult), "response path")
}

// Option B: the backend's own error document is passed through. OpenAI-compatible providers
// already answer in this shape, and rewriting another vendor's would lose its detail.
func TestLLMOpenAIErrors_BackendErrorBodyIsPassedThrough(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	withRejectedBody(ec, 400)
	ec.responseCodeDetails = codeDetailsViaUpstream
	ec.responseBodyCtx.ResponseBody = &policy.Body{
		Content: []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`), Present: true,
	}

	execResult := &executor.ResponseExecutionResult{}
	ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream)

	assert.Nil(t, formattedResult(execResult))
}

func TestLLMOpenAIErrors_RouterFailureIsRendered(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	withRejectedBody(ec, 503)
	ec.responseCodeDetails = codeDetailsNoHealthyUpstream
	ec.responseBodyCtx.ResponseBody = &policy.Body{Content: []byte("no healthy upstream"), Present: true}

	execResult := &executor.ResponseExecutionResult{}
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream))

	formatted := formattedResult(execResult)
	require.NotNil(t, formatted)
	inner := openAIErrorOf(t, formatted)
	assert.Equal(t, "server_error", inner["type"])
	assert.Equal(t, codeUpstreamUnavailable, inner["code"])
	assert.NotContains(t, string(formatted), "no healthy upstream", "Envoy's own text is not forwarded")
}

// ─── LLM routes whose policies never read the response body ──────────────────

// llmRouteNoBodyPolicy drives a response through the real header phase on an LLM route whose
// chain does NOT require the response body — the route the formatter could not reach before,
// because the body phase only exists when some policy asks for it.
func llmRouteNoBodyPolicy(t *testing.T, enabled bool, status, codeDetails string) *PolicyExecutionContext {
	t.Helper()
	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20,
		WithLLMOpenAIErrors(enabled))

	ec := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":path", RawValue: []byte("/openai/v1/chat/completions")},
			{Key: ":method", RawValue: []byte("POST")},
		}},
	}, RouteMetadata{APIKind: string(policy.APIKindLlmProxy)})
	ec.responseCodeDetails = codeDetails

	_, err := ec.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":status", RawValue: []byte(status)},
			{Key: "content-type", RawValue: []byte("text/plain")},
		}},
		EndOfStream: false,
	})
	require.NoError(t, err)
	return ec
}

func TestLLMOpenAIErrors_BuffersOnlyAnErrorBodyOnARouteWithNoBodyPolicy(t *testing.T) {
	ec := llmRouteNoBodyPolicy(t, true, "503", codeDetailsNoHealthyUpstream)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, ec.getModeOverride().GetResponseBodyMode(),
		"the router's 503 body has to come back to the engine to be replaced")

	ok := llmRouteNoBodyPolicy(t, true, "200", codeDetailsViaUpstream)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, ok.getModeOverride().GetResponseBodyMode(),
		"a successful completion must not pay for buffering")

	off := llmRouteNoBodyPolicy(t, false, "503", codeDetailsNoHealthyUpstream)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, off.getModeOverride().GetResponseBodyMode(),
		"with the option off nothing about the route changes")

	backend := llmRouteNoBodyPolicy(t, true, "429", codeDetailsViaUpstream)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, backend.getModeOverride().GetResponseBodyMode(),
		"a backend's own error passes through, so buffering it would cost a buffer for nothing")

	unknown := llmRouteNoBodyPolicy(t, true, "500", "")
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, unknown.getModeOverride().GetResponseBodyMode(),
		"an error of unreported provenance may be the provider's, so it is not buffered to be rewritten")
}

// Without code_details there is no proof the router produced an error, and it may be the
// provider's own document — which an OpenAI route never rewrites.
func TestLLMOpenAIErrors_UnattributedUpstreamErrorPassesThrough(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	withRejectedBody(ec, 418)
	ec.responseCodeDetails = ""
	ec.responseBodyCtx.ResponseBody = &policy.Body{
		Content: []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`), Present: true,
	}

	execResult := &executor.ResponseExecutionResult{}
	ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream)

	assert.Nil(t, formattedResult(execResult))
}

func TestLLMOpenAIErrors_RouterFailureReachesTheWireOnARouteWithNoBodyPolicy(t *testing.T) {
	ec := llmRouteNoBodyPolicy(t, true, "503", codeDetailsNoHealthyUpstream)

	resp, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body: []byte("no healthy upstream"), EndOfStream: true,
	})
	require.NoError(t, err)

	mutation := resp.GetResponseBody().GetResponse().GetBodyMutation().GetBody()
	require.NotNil(t, mutation, "the router's text must be replaced")
	assert.Equal(t, "server_error", openAIErrorOf(t, mutation)["type"])
	assert.Equal(t, "application/json", headerOpsFromBodyResponse(t, resp)["content-type"])
}

func TestLLMOpenAIErrors_BackendErrorPassesThroughOnARouteWithNoBodyPolicy(t *testing.T) {
	ec := llmRouteNoBodyPolicy(t, true, "429", codeDetailsViaUpstream)

	resp, err := ec.processResponseBody(context.Background(), &extprocv3.HttpBody{
		Body: []byte(`{"error":{"message":"Rate limit reached","type":"requests"}}`), EndOfStream: true,
	})
	require.NoError(t, err)
	assert.Nil(t, resp.GetResponseBody().GetResponse().GetBodyMutation(),
		"the provider's own error document is not rewritten")
}

// llmRouteBodylessResponse drives a bodyless response through the header phase of an LLM route,
// with or without a policy that reads response bodies. Envoy sends no body phase for it, so
// anything the formatter adds is added here.
func llmRouteBodylessResponse(t *testing.T, requiresBody bool, status, codeDetails string) *extprocv3.CommonResponse {
	t.Helper()
	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20,
		WithLLMOpenAIErrors(true))
	chain := &registry.PolicyChain{RequiresResponseBody: requiresBody}

	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":path", RawValue: []byte("/openai/v1/chat/completions")},
			{Key: ":method", RawValue: []byte("POST")},
		}},
	}, RouteMetadata{APIKind: string(policy.APIKindLlmProvider)})
	ec.responseCodeDetails = codeDetails

	resp, err := ec.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":status", RawValue: []byte(status)},
			{Key: "content-length", RawValue: []byte("0")},
		}},
		EndOfStream: true,
	})
	require.NoError(t, err)
	return resp.GetResponseHeaders().GetResponse()
}

// A provider owns its complete error response, including the decision to send no body.
func TestLLMOpenAIErrors_BodylessBackendErrorStaysEmpty(t *testing.T) {
	for _, requiresBody := range []bool{true, false} {
		for _, status := range []string{"429", "500"} {
			common := llmRouteBodylessResponse(t, requiresBody, status, codeDetailsViaUpstream)
			assert.Nil(t, common.GetBodyMutation(),
				"a backend %s with no body is passed through as it is (route reads bodies: %v)", status, requiresBody)
		}
	}
}

func TestActionBody_ReadsTheBodyOfEitherResponseAction(t *testing.T) {
	assert.Equal(t, []byte("rejected"), actionBody(policy.ImmediateResponse{StatusCode: 403, Body: []byte("rejected")}))
	assert.Equal(t, []byte("modified"), actionBody(policy.DownstreamResponseModifications{Body: []byte("modified")}))
	assert.Nil(t, actionBody(nil), "no action carries no body")
}

// The body the shipped guardrails write, as word-count-guardrail v1.0.2 sends it.
func legacyGuardrailBody(direction string) []byte {
	return []byte(`{"message":{"action":"GUARDRAIL_INTERVENED",` +
		`"actionReason":"Violation of applied word count constraints detected",` +
		`"assessments":"Expected between 1 and 5 words.",` +
		`"direction":"` + direction + `","interveningGuardrail":"word-count-guardrail"},"type":"WORD_COUNT_GUARDRAIL"}`)
}

func TestLLMOpenAIErrors_RequestGuardrailBodyKeepsTheGuardrailBlock(t *testing.T) {
	ec := llmCtx(t, nil, nil)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Headers:    map[string]string{contentTypeHeader: "application/json"},
		Body:       legacyGuardrailBody("REQUEST"),
	})

	inner := openAIErrorOf(t, out.Body)
	assert.Equal(t, "Violation of applied word count constraints detected", inner["message"])
	assert.Equal(t, "invalid_request_error", inner["type"])
	g, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok, "the guardrail block is carried: %s", out.Body)
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
	assert.Equal(t, "REQUEST", g["direction"])
	assert.Equal(t, map[string]any{"details": "Expected between 1 and 5 words."}, g["assessments"])
	assert.Equal(t, 422, out.StatusCode)
}

func TestLLMOpenAIErrors_ResponseGuardrailBodyKeepsTheGuardrailBlock(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	withRejectedBody(ec, 422)

	execResult := bodyRejection(422)
	execResult.Results[0].Action = policy.DownstreamResponseModifications{
		StatusCode:   execResult.Results[0].Action.(policy.DownstreamResponseModifications).StatusCode,
		Body:         legacyGuardrailBody("RESPONSE"),
		HeadersToSet: map[string]string{"Content-Type": "application/json"},
	}
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	formatted := formattedResult(execResult)
	require.NotNil(t, formatted)
	inner := openAIErrorOf(t, formatted)
	assert.Equal(t, "Violation of applied word count constraints detected", inner["message"])
	g, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok, "the guardrail block is carried: %s", formatted)
	assert.Equal(t, "RESPONSE", g["direction"])
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
}

// A fault entry's re-description that names no guardrail gets the body's block; everything it
// did set stays.
func TestLLMOpenAIErrors_GuardrailBodyFillsARedescriptionWithoutABlock(t *testing.T) {
	redescription := &policy.FaultDetails{Code: "906201", Message: "Blocked by policy"}
	ec := llmCtx(t, []policy.Policy{&faultRedescriberPolicy{fault: redescription}}, specs(1))
	withRejectedBody(ec, 422)

	execResult := bodyRejection(422)
	execResult.Results[0].Action = policy.DownstreamResponseModifications{
		StatusCode: execResult.Results[0].Action.(policy.DownstreamResponseModifications).StatusCode,
		Body:       legacyGuardrailBody("RESPONSE"),
	}
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	inner := openAIErrorOf(t, formattedResult(execResult))
	assert.Equal(t, "Blocked by policy", inner["message"], "the re-description's message stands")
	assert.Equal(t, "906201", inner["code"])
	g, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
	assert.Nil(t, redescription.Guardrail, "the fault entry's own description is not mutated")
}

func TestWithFallbackDetails(t *testing.T) {
	fallback := policy.FaultDetails{
		Message: "from body", Type: policy.FaultTypeGuardrail, Direction: policy.DirectionResponse,
		Guardrail: &policy.GuardrailDetails{InterveningGuardrail: "body-guardrail"},
	}

	assert.Equal(t, &fallback, withFallbackDetails(nil, fallback), "no description: the body's stands")

	declared := &policy.FaultDetails{
		Message: "declared", Type: policy.FaultTypeAuthorization, Direction: policy.DirectionRequest,
		Guardrail: &policy.GuardrailDetails{InterveningGuardrail: "declared-guardrail"},
	}
	assert.Equal(t, declared, withFallbackDetails(declared, fallback), "nothing declared is replaced")
}

// A fault entry is handed what the gateway read out of the producer's body, so one that only
// adds a code keeps the guardrail block and message.
func TestLLMOpenAIErrors_FaultEntrySeesTheDescriptionLiftedFromARejectionBody(t *testing.T) {
	seen := &policy.FaultDetails{}
	ec := llmCtx(t, []policy.Policy{&faultObserverPolicy{seen: seen, code: "999004"}}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Headers:    map[string]string{contentTypeHeader: "application/json"},
		Body:       legacyGuardrailBody("REQUEST"),
	})

	require.NotNil(t, seen.Guardrail, "the entry saw the lifted description")
	assert.Equal(t, "word-count-guardrail", seen.Guardrail.InterveningGuardrail)

	inner := openAIErrorOf(t, out.Body)
	assert.Equal(t, "999004", inner["code"])
	assert.Equal(t, "Violation of applied word count constraints detected", inner["message"])
	g, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok, "the guardrail block survives the re-description: %s", out.Body)
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
}

// faultObserverPolicy records the description it was handed and re-describes it with a code,
// keeping everything else — what an operator's "add our code" fault policy does.
type faultObserverPolicy struct {
	seen *policy.FaultDetails
	code string
}

func (p *faultObserverPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

func (p *faultObserverPolicy) OnFault(_ context.Context, fc *policy.FaultContext, _ map[string]interface{}) *policy.FaultResponse {
	if fc.Fault == nil {
		return nil
	}
	*p.seen = *fc.Fault
	redescribed := *fc.Fault
	redescribed.Code = p.code
	return &policy.FaultResponse{Fault: &redescribed}
}
