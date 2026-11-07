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
	"fmt"
	"net/http"
	"strings"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// llmCtx is an LlmProxy execution context with the OpenAI envelope switched on through the real
// ServerOption, as it is when policy_engine.llm_errors.enable_openai_format is left on.
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
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20, nil,
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
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20, nil,
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

// A fault entry is handed what it was before the OpenAI format existed: a producer that wrote a
// body without describing the failure declared nothing. The body is reshaped only when the
// error is formatted, so an entry that only adds a code still yields the guardrail's message and
// block, with its code.
func TestLLMOpenAIErrors_FaultEntryIsNotHandedTheReshapedRejectionBody(t *testing.T) {
	observer := &faultObserverPolicy{code: "999004"}
	ec := llmCtx(t, []policy.Policy{observer}, specs(1))

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Headers:    map[string]string{contentTypeHeader: "application/json"},
		Body:       legacyGuardrailBody("REQUEST"),
	})

	require.True(t, observer.ran, "the fault entry ran")
	assert.Nil(t, observer.seen, "the entry is handed no description, as before the OpenAI format")

	inner := openAIErrorOf(t, out.Body)
	assert.Equal(t, "999004", inner["code"], "the entry's code is kept")
	assert.Equal(t, "Violation of applied word count constraints detected", inner["message"])
	assert.Equal(t, "invalid_request_error", inner["type"])
	g, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok, "the guardrail block is filled in from the body: %s", out.Body)
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
	assert.Equal(t, "REQUEST", g["direction"])
}

// The same rejection with the OpenAI format off: the entry is handed the same (no description),
// and the guardrail's own body is sent as written.
func TestLLMOpenAIErrors_FaultEntryIsHandedTheSameWithTheFormatOff(t *testing.T) {
	observer := &faultObserverPolicy{code: "999004"}
	ec := faultExecCtx(t, []policy.Policy{observer}, specs(1))
	ec.sharedCtx.APIKind = policy.APIKindLlmProxy

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Headers:    map[string]string{contentTypeHeader: "application/json"},
		Body:       legacyGuardrailBody("REQUEST"),
	})

	require.True(t, observer.ran)
	assert.Nil(t, observer.seen)
	assert.Equal(t, legacyGuardrailBody("REQUEST"), out.Body)
}

// faultObserverPolicy records the description it was handed, if any, and re-describes the
// failure with a code — what an operator's "add our code" fault policy does.
type faultObserverPolicy struct {
	ran  bool
	seen *policy.FaultDetails
	code string
}

func (p *faultObserverPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

// OnFault records the description it was handed and re-describes the failure with p.code.
func (p *faultObserverPolicy) OnFault(_ context.Context, fc *policy.FaultContext, _ map[string]interface{}) *policy.FaultResponse {
	p.ran = true
	redescribed := policy.FaultDetails{}
	if fc.Fault != nil {
		seen := *fc.Fault
		p.seen = &seen
		redescribed = seen
	}
	redescribed.Code = p.code
	return &policy.FaultResponse{Fault: &redescribed}
}

// A policy that set "Content-Type" in its own casing still leaves exactly one content-type:
// Envoy lowercases header names, so the two would otherwise reach it as duplicates.
func TestLLMOpenAIErrors_OneContentTypeWhateverThePolicysCasing(t *testing.T) {
	ec := llmCtx(t, nil, nil)

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 403,
		Headers:    map[string]string{"Content-Type": "text/plain", "X-Kept": "yes"},
		Body:       []byte("Caller is not allowed to use this model"),
	})

	var contentTypes []string
	for name, value := range out.Headers {
		if strings.EqualFold(name, contentTypeHeader) {
			contentTypes = append(contentTypes, name+"="+value)
		}
	}
	assert.Equal(t, []string{contentTypeHeader + "=application/json"}, contentTypes)
	assert.Equal(t, "yes", out.Headers["X-Kept"], "other headers are left alone")
	assert.Equal(t, "Caller is not allowed to use this model", openAIErrorOf(t, out.Body)["message"])
}

// An encoded rejection body is kept as written: replacing it would leave its Content-Encoding
// describing bytes that are gone. Identity is not an encoding.
func TestLLMOpenAIErrors_EncodedPolicyBodyIsKept(t *testing.T) {
	gzipped := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}
	for _, name := range []string{"content-encoding", "Content-Encoding"} {
		ec := llmCtx(t, nil, nil)
		out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
			StatusCode: 403,
			Headers:    map[string]string{name: "gzip", contentTypeHeader: "application/json"},
			Body:       gzipped,
		})
		assert.Equal(t, gzipped, out.Body, "%s: gzip body kept", name)
		assert.Equal(t, "gzip", out.Headers[name], "%s: header kept with its body", name)
	}

	ec := llmCtx(t, nil, nil)
	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 403,
		Headers:    map[string]string{"Content-Encoding": "identity"},
		Body:       []byte(`{"error":"Forbidden","message":"Caller is not allowed to use this model"}`),
	})
	assert.Equal(t, "permission_error", openAIErrorOf(t, out.Body)["type"], "identity is reshaped like no encoding")
}

func TestEncodedBody(t *testing.T) {
	assert.False(t, encodedBody(nil))
	assert.False(t, encodedBody(map[string]string{"content-type": "application/json"}))
	assert.False(t, encodedBody(map[string]string{"Content-Encoding": "identity"}))
	assert.False(t, encodedBody(map[string]string{"content-encoding": " "}))
	assert.True(t, encodedBody(map[string]string{"content-encoding": "gzip"}))
	assert.True(t, encodedBody(map[string]string{"CONTENT-ENCODING": "br"}))
	// Both casings present: the answer must not depend on map iteration order.
	for i := 0; i < 50; i++ {
		assert.True(t, encodedBody(map[string]string{"content-encoding": "identity", "Content-Encoding": "gzip"}))
	}
}

// When two response policies write a body, the client receives the last one, so that is the
// one the envelope describes.
func TestLLMOpenAIErrors_LastPolicyBodyDescribesTheFailure(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	withRejectedBody(ec, 422)

	execResult := bodyRejection(422)
	first := execResult.Results[0]
	first.Action = policy.DownstreamResponseModifications{
		StatusCode: first.Action.(policy.DownstreamResponseModifications).StatusCode,
		Body:       []byte(`{"error":"Earlier","message":"an earlier policy's body"}`),
	}
	last := first
	last.PolicyName = "word-count-guardrail-2"
	last.Action = policy.DownstreamResponseModifications{
		StatusCode: first.Action.(policy.DownstreamResponseModifications).StatusCode,
		Body:       legacyGuardrailBody("RESPONSE"),
	}
	execResult.Results = []executor.ResponsePolicyResult{first, last}
	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway))

	inner := openAIErrorOf(t, formattedResult(execResult))
	assert.Equal(t, "Violation of applied word count constraints detected", inner["message"])
	g, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok, "the last body's guardrail block is carried")
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
}

// A router error with no body still gets the envelope on a route that reads response bodies:
// the body phase runs inline there. (On a route that reads none, Envoy gives the engine no
// body phase for it, which is a known limitation.)
func TestLLMOpenAIErrors_BodylessRouterErrorIsRenderedOnABodyReadingRoute(t *testing.T) {
	common := llmRouteBodylessResponse(t, true, "503", codeDetailsNoHealthyUpstream)
	require.NotNil(t, common.GetBodyMutation(), "a bodyless router error is rendered")
	inner := openAIErrorOf(t, common.GetBodyMutation().GetBody())
	assert.Equal(t, "server_error", inner["type"])
}

// A response policy that encoded the body it wrote keeps it: reshaping it would leave its
// Content-Encoding describing bytes that are gone. Covers a set and an appended header.
func TestLLMOpenAIErrors_EncodedResponsePolicyBodyIsKept(t *testing.T) {
	gzipped := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}
	for name, mods := range map[string]policy.DownstreamResponseModifications{
		"set":      {HeadersToSet: map[string]string{"Content-Encoding": "gzip"}},
		"appended": {HeadersToAppend: map[string][]string{"content-encoding": {"br"}}},
	} {
		ec := llmCtx(t, nil, nil)
		withRejectedBody(ec, 422)

		execResult := bodyRejection(422)
		mods.StatusCode = execResult.Results[0].Action.(policy.DownstreamResponseModifications).StatusCode
		mods.Body = gzipped
		execResult.Results[0].Action = mods
		assert.False(t, ec.runFaultPoliciesOnResponse(context.Background(), execResult, originGateway),
			"%s: nothing is formatted", name)
		assert.Nil(t, formattedResult(execResult), "%s: no envelope replaces the encoded body", name)
		assert.Equal(t, gzipped, execResult.Results[0].Action.(policy.DownstreamResponseModifications).Body,
			"%s: the policy's body is untouched", name)
	}
}

func TestActionEncoded(t *testing.T) {
	assert.False(t, actionEncoded(nil))
	assert.False(t, actionEncoded(policy.ImmediateResponse{Headers: map[string]string{"content-type": "application/json"}}))
	assert.True(t, actionEncoded(policy.ImmediateResponse{Headers: map[string]string{"Content-Encoding": "gzip"}}))
	assert.False(t, actionEncoded(policy.DownstreamResponseModifications{HeadersToSet: map[string]string{"content-encoding": "identity"}}))
	assert.True(t, actionEncoded(policy.DownstreamResponseModifications{HeadersToSet: map[string]string{"content-encoding": "gzip"}}))
	assert.True(t, actionEncoded(policy.DownstreamResponseModifications{HeadersToAppend: map[string][]string{"Content-Encoding": {"identity", "br"}}}))
	assert.False(t, actionEncoded(policy.DownstreamResponseModifications{HeadersToAppend: map[string][]string{"x-other": {"gzip"}}}))
}

// When the route declares fault policies but none can run, the rejection is still formatted:
// formatting is a default, not a chain step.
func TestLLMOpenAIErrors_RejectionIsFormattedWhenTheFaultChainCannotRun(t *testing.T) {
	ec := llmCtx(t, nil, nil)
	ec.policyChain.HasFaultPolicies = true
	ec.policyChain.FaultPolicies = nil

	out := ec.runFaultPoliciesOnRejection(context.Background(), policy.ImmediateResponse{
		StatusCode: 422,
		Headers:    map[string]string{contentTypeHeader: "application/json"},
		Body:       legacyGuardrailBody("REQUEST"),
	})

	inner := openAIErrorOf(t, out.Body)
	assert.Equal(t, "Violation of applied word count constraints detected", inner["message"])
	_, ok := inner["guardrail"].(map[string]any)
	assert.True(t, ok, "the guardrail block is carried: %s", out.Body)
}

// Only errors the gateway constructed get the envelope. A response policy that writes its own
// body over a provider error without rejecting it leaves the failure the provider's, so what the
// pipeline produced is sent as written, whether or not the provider sent a body. A policy that
// rejects the response (sets the status) constructed the error, so its body is reshaped.
func TestLLMOpenAIErrors_PolicyBodyOverAProviderErrorIsFormattedOnlyWhenThePolicyRejects(t *testing.T) {
	policyBody := []byte(`{"error":"Too Many Requests","message":"Slow down"}`)
	for _, tc := range []struct {
		providerBody string
		rejects      bool
	}{
		{providerBody: "", rejects: false},
		{providerBody: `{"error":{"message":"provider said no","type":"requests","param":null,"code":"x"}}`, rejects: false},
		{providerBody: "", rejects: true},
		{providerBody: `{"error":{"message":"provider said no","type":"requests","param":null,"code":"x"}}`, rejects: true},
	} {
		name := fmt.Sprintf("provider body=%v, policy rejects=%v", tc.providerBody != "", tc.rejects)
		ec := llmCtx(t, nil, nil)
		withRejectedBody(ec, 429)
		ec.responseHeaderCtx.ResponseStatus = 429
		ec.responseBodyCtx.ResponseStatus = 429
		ec.responseCodeDetails = codeDetailsViaUpstream
		ec.responseBodyCtx.ResponseBody = &policy.Body{Content: []byte(tc.providerBody), Present: tc.providerBody != ""}

		action := policy.DownstreamResponseModifications{Body: policyBody}
		if tc.rejects {
			s := 429
			action.StatusCode = &s
		}
		execResult := &executor.ResponseExecutionResult{Results: []executor.ResponsePolicyResult{{
			PolicyName: "response-policy", PolicyVersion: "v1", Action: action,
		}}}
		// As processResponseBody does.
		ec.handleFault(context.Background(), faultFromResponseBody(execResult, responseRejectedByPolicy(execResult.Results)))

		formatted := formattedResult(execResult)
		if !tc.rejects {
			assert.Nil(t, formatted, "%s: passed through", name)
			assert.Equal(t, policyBody, execResult.Results[0].Action.(policy.DownstreamResponseModifications).Body,
				"%s: the policy's body is sent as written", name)
			continue
		}
		require.NotNil(t, formatted, "%s: formatted", name)
		assert.Equal(t, "Slow down", openAIErrorOf(t, formatted)["message"], name)
		assert.Equal(t, "rate_limit_error", openAIErrorOf(t, formatted)["type"], name)
	}
}

// The engine's own rejections of a body it cannot decode are errors the gateway constructed, so
// an LLM route gets them in the OpenAI envelope, keeping the generic message, the status and the
// correlation id. A REST route keeps the body it always had.
func TestLLMOpenAIErrors_EngineBodyRejectionsAreFormatted(t *testing.T) {
	type rejection func(ec *PolicyExecutionContext) *extprocv3.ProcessingResponse
	unsupported := func(ec *PolicyExecutionContext) *extprocv3.ProcessingResponse {
		return ec.rejectUnsupportedEncoding(context.Background(), []string{"zstd"}, "request_body",
			typev3.StatusCode_UnsupportedMediaType, http.StatusUnsupportedMediaType, "Unsupported Media Type")
	}
	undecodable := func(ec *PolicyExecutionContext) *extprocv3.ProcessingResponse {
		return ec.rejectUndecodableBody(context.Background(), errors.New("gzip: invalid header"), "response_body",
			typev3.StatusCode_BadGateway, http.StatusBadGateway, "Bad Gateway")
	}
	for name, tc := range map[string]struct {
		reject   rejection
		message  string
		wantType string
	}{
		"unsupported encoding": {unsupported, "Unsupported Media Type", "invalid_request_error"},
		"undecodable body":     {undecodable, "Bad Gateway", "server_error"},
	} {
		imm := tc.reject(llmCtx(t, nil, nil)).GetImmediateResponse()
		errorID := immediateHeader(imm, "x-error-id")
		require.NotEmpty(t, errorID, name)
		inner := openAIErrorOf(t, imm.GetBody())
		assert.Equal(t, tc.message, inner["message"], name)
		assert.Equal(t, tc.wantType, inner["type"], name)
		assert.Equal(t, errorID, inner["error_id"], "%s: the envelope keeps the correlation id", name)
		assert.Equal(t, "application/json", immediateHeader(imm, "content-type"), name)

		rest := faultExecCtx(t, nil, nil)
		rest.sharedCtx.APIKind = policy.APIKindRestApi
		restImm := tc.reject(rest).GetImmediateResponse()
		assert.JSONEq(t, fmt.Sprintf(`{"error":%q,"error_id":%q}`, tc.message, immediateHeader(restImm, "x-error-id")),
			string(restImm.GetBody()), "%s: a REST route keeps its body", name)
	}
}

// immediateHeader returns the value of header name on an immediate response, or "".
func immediateHeader(imm *extprocv3.ImmediateResponse, name string) string {
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		if strings.EqualFold(h.GetHeader().GetKey(), name) {
			if v := h.GetHeader().GetValue(); v != "" {
				return v
			}
			return string(h.GetHeader().GetRawValue())
		}
	}
	return ""
}

// Without a server there is nothing to shape against, so the body is returned as it was.
func TestFormatEngineRejection_WithoutAServerKeepsTheBody(t *testing.T) {
	body := []byte(`{"error":"Bad Gateway","error_id":"id"}`)
	got, contentType := (&PolicyExecutionContext{}).formatEngineRejection(context.Background(), http.StatusBadGateway, "id", "Bad Gateway", body)
	assert.Equal(t, body, got)
	assert.Equal(t, "application/json", contentType)
}
