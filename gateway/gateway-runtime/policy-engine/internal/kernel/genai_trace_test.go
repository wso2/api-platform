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

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	"github.com/wso2/api-platform/sdk/ai/llmusage"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// storeOpenAITemplate installs a minimal OpenAI provider template under the handle the
// route names, the way the controller's lazy resources do.
func storeOpenAITemplate(t *testing.T) {
	t.Helper()
	store := policy.GetLazyResourceStoreInstance()
	require.NoError(t, store.StoreResource(&policy.LazyResource{
		ID:           "openai",
		ResourceType: llmusage.ResourceTypeLLMProviderTemplate,
		Resource: map[string]interface{}{"spec": map[string]interface{}{
			"promptTokens":     map[string]interface{}{"location": "payload", "identifier": "$.usage.prompt_tokens"},
			"completionTokens": map[string]interface{}{"location": "payload", "identifier": "$.usage.completion_tokens"},
			"totalTokens":      map[string]interface{}{"location": "payload", "identifier": "$.usage.total_tokens"},
			"cachedTokens":     map[string]interface{}{"location": "payload", "identifier": "$.usage.prompt_tokens_details.cached_tokens"},
			"requestModel":     map[string]interface{}{"location": "payload", "identifier": "$.model"},
			"responseModel":    map[string]interface{}{"location": "payload", "identifier": "$.model"},
		}},
	}))
	t.Cleanup(func() { _ = store.RemoveResourceByIDAndType("openai", llmusage.ResourceTypeLLMProviderTemplate) })
}

// registerProviderRoute registers an OpenAI provider route served at /openai.
func registerProviderRoute(t *testing.T, k *Kernel, routeName string, kind policy.APIKind, chain *registry.PolicyChain) {
	t.Helper()
	k.RegisterRoute(routeName, chain)
	rc := &RouteConfig{Metadata: RouteMetadata{
		RouteName:       routeName,
		APIName:         "openai-provider",
		APIVersion:      "v1",
		Context:         "/openai",
		APIKind:         string(kind),
		TemplateHandle:  "openai",
		DefaultUpstream: &policyenginev1.UpstreamInfo{URL: "https://api.openai.com/v1"},
	}}
	require.NoError(t, PrepareRoute(resolver.DefaultRegistry(), routeName, rc))
	k.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeName: rc})
}

func genaiServer(t *testing.T, kind policy.APIKind, chain *registry.PolicyChain) (*ExternalProcessorServer, *tracetest.SpanRecorder) {
	t.Helper()
	server, k, sr := newSpanStatusServer(t)
	enableGenAI(server)
	storeOpenAITemplate(t)
	registerProviderRoute(t, k, "llm", kind, chain)
	return server, sr
}

func chatHeaders() *extprocv3.ProcessingRequest {
	req := requestHeadersReq("llm", "POST", "/openai/chat/completions")
	req.GetRequestHeaders().EndOfStream = false
	return req
}

func jsonResponseHeadersReq(status string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseHeaders{
		ResponseHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":status", RawValue: []byte(status)},
			{Key: "content-type", RawValue: []byte("application/json")},
		}}},
	}}
}

func passThroughChain() *registry.PolicyChain {
	return buildChainFor([]policy.Policy{&testutils.NoopPolicy{}})
}

func attrString(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()
	v, ok := attrOf(span, attribute.Key(key))
	require.True(t, ok, "span %q has no %s", span.Name(), key)
	return v.Emit()
}

func TestRequestForwarded(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{})
	none := &extprocv3.ProcessingResponse{ModeOverride: &extprocconfigv3.ProcessingMode{RequestBodyMode: extprocconfigv3.ProcessingMode_NONE}}
	partial := &extprocv3.ProcessingResponse{ModeOverride: &extprocconfigv3.ProcessingMode{RequestBodyMode: extprocconfigv3.ProcessingMode_BUFFERED_PARTIAL}}
	denied := &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: &extprocv3.ImmediateResponse{}}}
	headersEOS := requestHeadersReq("llm", "GET", "/models")
	headersMore := chatHeaders()
	body := requestBodyReq("{}")
	chunk := &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestBody{RequestBody: &extprocv3.HttpBody{Body: []byte("a")}}}

	assert.True(t, ec.requestForwarded(headersEOS, partial), "a bodyless request leaves with its headers")
	assert.True(t, ec.requestForwarded(headersMore, none), "a body the engine does not read follows the headers upstream")
	assert.True(t, ec.requestForwarded(headersMore, &extprocv3.ProcessingResponse{}), "no override leaves the static mode, which sends no body")
	assert.False(t, ec.requestForwarded(headersMore, partial), "the body is still to come")
	assert.True(t, ec.requestForwarded(body, &extprocv3.ProcessingResponse{}))
	assert.False(t, ec.requestForwarded(body, denied), "a denied request makes no call")
	assert.False(t, ec.requestForwarded(headersEOS, denied))
	assert.False(t, ec.requestForwarded(responseHeadersReq("200"), &extprocv3.ProcessingResponse{}))

	ec.isStreamingRequest = true
	assert.False(t, ec.requestForwarded(chunk, &extprocv3.ProcessingResponse{}), "a streamed request leaves at its last chunk")
	assert.True(t, ec.requestForwarded(body, &extprocv3.ProcessingResponse{}))

	ec.isStreamingRequest = false
	ec.pending = &pendingResolution{}
	assert.False(t, ec.requestForwarded(headersMore, none), "a deferred route binds at the body")
}

// The call to the provider is a CLIENT span under the root, named for the operation
// and model, with the usage and response metadata the conventions ask for.
func TestProcess_InferenceSpan_Buffered(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, passThroughChain())
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`),
		jsonResponseHeadersReq("200"),
		responseBodyReq(`{"id":"chatcmpl-1","model":"gpt-4o-2024-08-06","choices":[{"index":0,"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17,"prompt_tokens_details":{"cached_tokens":4}}}`),
	})
	require.NoError(t, server.Process(stream))

	ended := sr.Ended()
	span := spanByName(ended, "chat gpt-4o")
	require.NotNil(t, span)
	root := spanByName(ended, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, trace.SpanKindClient, span.SpanKind())
	assert.Equal(t, root.SpanContext().SpanID(), span.Parent().SpanID())
	assert.False(t, span.EndTime().After(root.EndTime()))
	assert.Equal(t, codes.Unset, span.Status().Code)

	assert.Equal(t, "chat", attrString(t, span, genai.AttrOperationName))
	assert.Equal(t, "openai", attrString(t, span, genai.AttrProviderName))
	assert.Equal(t, "gpt-4o", attrString(t, span, genai.AttrRequestModel))
	assert.Equal(t, "gpt-4o-2024-08-06", attrString(t, span, genai.AttrResponseModel))
	assert.Equal(t, "chatcmpl-1", attrString(t, span, genai.AttrResponseID))
	assert.Equal(t, `["stop"]`, attrString(t, span, genai.AttrResponseFinishReasons))
	assert.Equal(t, "12", attrString(t, span, genai.AttrUsageInputTokens))
	assert.Equal(t, "5", attrString(t, span, genai.AttrUsageOutputTokens))
	assert.Equal(t, "4", attrString(t, span, genai.AttrUsageCacheReadTokens))
	assert.Equal(t, "api.openai.com", attrString(t, span, genai.AttrServerAddress))
	assert.Equal(t, "443", attrString(t, span, genai.AttrServerPort))
	_, streamed := attrOf(span, attribute.Key(genai.AttrRequestStream))
	assert.False(t, streamed)
	_, ttfc := attrOf(span, attribute.Key(genai.AttrResponseTTFC))
	assert.False(t, ttfc, "time to first chunk is a streaming attribute")
}

func TestProcess_InferenceSpan_SSE(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, passThroughChain())
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o-mini","stream":true,"stream_options":{"include_usage":true}}`),
		sseResponseHeadersReq(),
		responseChunkReq("data: {\"id\":\"c2\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":null}]}\n\n", false),
		responseChunkReq("data: {\"id\":\"c2\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n", false),
		responseChunkReq("data: {\"id\":\"c2\",\"model\":\"gpt-4o-mini\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":21,\"total_tokens\":31}}\n\ndata: [DONE]\n\n", true),
	})
	require.NoError(t, server.Process(stream))

	span := spanByName(sr.Ended(), "chat gpt-4o-mini")
	require.NotNil(t, span)
	assert.Equal(t, "true", attrString(t, span, genai.AttrRequestStream))
	assert.Equal(t, "10", attrString(t, span, genai.AttrUsageInputTokens))
	assert.Equal(t, "21", attrString(t, span, genai.AttrUsageOutputTokens))
	assert.Equal(t, `["stop"]`, attrString(t, span, genai.AttrResponseFinishReasons))
	assert.Equal(t, "c2", attrString(t, span, genai.AttrResponseID))
	ttfc, ok := attrOf(span, attribute.Key(genai.AttrResponseTTFC))
	require.True(t, ok)
	assert.GreaterOrEqual(t, ttfc.AsFloat64(), 0.0)
}

type denyRequestBodyPolicy struct{}

func (denyRequestBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer}
}

func (denyRequestBodyPolicy) OnRequestBody(context.Context, *policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return policy.ImmediateResponse{StatusCode: 422}
}

// A request a policy answered made no call, so it has no client span.
func TestProcess_InferenceSpan_NoSpanWhenPolicyDenies(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, buildChainFor([]policy.Policy{denyRequestBodyPolicy{}}))
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o"}`),
	})
	require.NoError(t, server.Process(stream))
	require.NotNil(t, stream.responses[1].GetImmediateResponse())
	for _, s := range sr.Ended() {
		assert.NotEqual(t, trace.SpanKindClient, s.SpanKind(), "unexpected client span %q", s.Name())
	}
}

// A stream that closes before the response still ends the span, as a failed call.
func TestProcess_InferenceSpan_EndsOnCancel(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, passThroughChain())
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o"}`),
	})
	require.NoError(t, server.Process(stream))

	span := spanByName(sr.Ended(), "chat gpt-4o")
	require.NotNil(t, span, "the span must end with the stream")
	assert.Equal(t, genai.ErrorTypeOther, attrString(t, span, genai.AttrErrorType))
	assert.Equal(t, codes.Error, span.Status().Code)
}

// An upstream error fails the client span, with the status as its error type.
func TestProcess_InferenceSpan_UpstreamError(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, passThroughChain())
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o"}`),
		jsonResponseHeadersReq("429"),
		responseBodyReq(`{"error":{"message":"rate limited"}}`),
	})
	require.NoError(t, server.Process(stream))

	span := spanByName(sr.Ended(), "chat gpt-4o")
	require.NotNil(t, span)
	assert.Equal(t, "429", attrString(t, span, genai.AttrErrorType))
	assert.Equal(t, codes.Error, span.Status().Code)
	_, hasUsage := attrOf(span, attribute.Key(genai.AttrUsageInputTokens))
	assert.False(t, hasUsage)
}

// An LLM proxy route's upstream is its provider route, which records the call; the
// proxy route only names itself on its root, so tokens are counted once.
func TestProcess_InferenceSpan_ProxyRouteHasNoInferenceSpan(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProxy, passThroughChain())
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o"}`),
		jsonResponseHeadersReq("200"),
		responseBodyReq(`{"usage":{"prompt_tokens":1,"completion_tokens":1}}`),
	})
	require.NoError(t, server.Process(stream))

	ended := sr.Ended()
	assert.Nil(t, spanByName(ended, "chat gpt-4o"))
	root := spanByName(ended, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, "openai-provider", attrString(t, root, genai.AttrAIProxy))
}

// A path that is not an inference endpoint gets no span.
func TestProcess_InferenceSpan_NoSpanForModelListing(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, passThroughChain())
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("llm", "GET", "/openai/models"),
		jsonResponseHeadersReq("200"),
		responseBodyReq(`{"data":[]}`),
	})
	require.NoError(t, server.Process(stream))
	for _, s := range sr.Ended() {
		assert.NotEqual(t, trace.SpanKindClient, s.SpanKind())
	}
}

// The usage the span reports is the one llm-cost prices: the span reads it through the
// same memoised llmusage.Get, after the response policies ran.
func TestInferenceSpan_UsageMatchesLLMCostCache(t *testing.T) {
	storeOpenAITemplate(t)
	server, _, sr := newSpanStatusServer(t)
	enableGenAI(server)
	ec := newPolicyExecutionContext(server, "llm", &registry.PolicyChain{})
	ec.genai.kind = policy.APIKindLlmProvider
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":path", RawValue: []byte("/openai/chat/completions")},
		{Key: ":method", RawValue: []byte("POST")},
	}}}, RouteMetadata{APIKind: string(policy.APIKindLlmProvider), TemplateHandle: "openai", Context: "/openai"})
	_, ec.rootSpan = server.tracer.Start(context.Background(), "root")
	ec.genai.requestBody = []byte(`{"model":"gpt-4o"}`)

	ec.startInferenceSpan()
	ec.buildResponseContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":status", RawValue: []byte("200")}}}})
	ec.captureResponse([]byte(`{"usage":{"prompt_tokens":7,"completion_tokens":3}}`), true)
	ec.endGenAISpan()
	ec.endGenAISpan() // idempotent

	cached, ok := ec.sharedCtx.Metadata[llmusage.UsageKey].(llmusage.Usage)
	require.True(t, ok, "the span's read is the memoised one llm-cost reuses")
	span := spanByName(sr.Ended(), "chat gpt-4o")
	require.NotNil(t, span)
	assert.Equal(t, "7", attrString(t, span, genai.AttrUsageInputTokens))
	assert.Equal(t, int64(7), cached.TotalInputTokens)
	assert.Equal(t, int64(3), cached.OutputTokens)
	assert.Len(t, spansNamed(sr.Ended(), "chat gpt-4o"), 1)
}
