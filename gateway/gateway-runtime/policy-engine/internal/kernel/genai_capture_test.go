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

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

var testGenAIConfig = config.GenAITracingConfig{Enabled: true, MaxContentBytes: 16384}

// enableGenAI turns GenAI tracing on for a test server, as a tracing-enabled config would.
func enableGenAI(server *ExternalProcessorServer) {
	server.genai = testGenAIConfig
}

// genaiExecCtx is an execution context for a route of kind with GenAI tracing on.
func genaiExecCtx(t *testing.T, kind policy.APIKind, chain *registry.PolicyChain) *PolicyExecutionContext {
	t.Helper()
	server, _, _ := newSpanStatusServer(t)
	enableGenAI(server)
	ec := newPolicyExecutionContext(server, "route", chain)
	ec.genai.kind = server.genaiKindFor(kind)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{APIKind: string(kind)})
	return ec
}

func TestGenAIKindFor(t *testing.T) {
	server, _, _ := newSpanStatusServer(t)
	assert.Empty(t, server.genaiKindFor(policy.APIKindLlmProvider), "GenAI tracing off records nothing")

	enableGenAI(server)
	for kind, want := range map[policy.APIKind]policy.APIKind{
		policy.APIKindLlmProvider: policy.APIKindLlmProvider,
		policy.APIKindMCP:         policy.APIKindMCP,
		policy.APIKindAgent:       policy.APIKindAgent,
		policy.APIKindLlmProxy:    "", // its upstream is the provider route, which records the span
		policy.APIKindRestApi:     "",
	} {
		assert.Equal(t, want, server.genaiKindFor(kind), "kind %s", kind)
	}
}

// On an LLM provider route the engine reads both bodies for the span, partially so a
// body over the buffer limit is never rejected on its account.
func TestGetModeOverride_GenAIPartialBodiesOnLLMRoute(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{})
	ec.phase = phaseRequestHeaders

	mode := ec.getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED_PARTIAL, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED_PARTIAL, mode.ResponseBodyMode)

	ec.isStreamingResponse = true
	assert.Equal(t, extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED, ec.getModeOverride().ResponseBodyMode,
		"a streamed response stays streamed")
}

// A streamed request is left to the chain's own mode.
func TestGetModeOverride_GenAILeavesStreamedRequestAlone(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{})
	ec.isStreamingRequest = true
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, ec.getModeOverride().RequestBodyMode)
}

// The MCP POST route reads its response for the JSON-RPC outcome and never streams it.
func TestGetModeOverride_GenAIMCPResponsePartialNeverStreamed(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindMCP, &registry.PolicyChain{})
	ec.resolverName = resolver.MCPResolverName
	ec.buildResponseContexts(sseHeaders())

	assert.False(t, ec.responseStreamingEnabled(false), "MCP responses are buffered")
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED_PARTIAL, ec.getModeOverride().ResponseBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, ec.getModeOverride().RequestBodyMode,
		"the MCP request body is read by the resolver, not on the span's account")

	other := genaiExecCtx(t, policy.APIKindMCP, &registry.PolicyChain{})
	other.resolverName = resolver.RouteKeyResolverName
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, other.getModeOverride().ResponseBodyMode,
		"only the POST route carries JSON-RPC responses")
}

// An A2A response is not read: its span takes the outcome from the status.
func TestGetModeOverride_GenAIAgentReadsNoBody(t *testing.T) {
	mode := genaiExecCtx(t, policy.APIKindAgent, &registry.PolicyChain{}).getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.ResponseBodyMode)
}

func TestGetModeOverride_GenAIOffUnchanged(t *testing.T) {
	server, _, _ := newSpanStatusServer(t)
	ec := newPolicyExecutionContext(server, "route", &registry.PolicyChain{})
	ec.genai.kind = server.genaiKindFor(policy.APIKindLlmProvider)
	mode := ec.getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, mode.ResponseBodyMode)
}

// A chain that needs a body keeps its own mode.
func TestGetModeOverride_GenAIChainBufferedStaysBuffered(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{RequiresRequestBody: true, RequiresResponseBody: true})
	mode := ec.getModeOverride()
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

// A chain whose response policy buffers is not streamed on the span's account.
func TestResponseStreamingEnabled_GenAIOnlyWhenNoPolicyReadsTheResponse(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{})
	ec.buildResponseContexts(sseHeaders())
	assert.True(t, ec.responseStreamingEnabled(false))

	buffered := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{RequiresResponseBody: true})
	buffered.buildResponseContexts(sseHeaders())
	assert.False(t, buffered.responseStreamingEnabled(false))
}

func llmRoute(t *testing.T, k *Kernel, routeName string, chain *registry.PolicyChain) {
	t.Helper()
	k.RegisterRoute(routeName, chain)
	rc := &RouteConfig{Metadata: RouteMetadata{RouteName: routeName, APIKind: string(policy.APIKindLlmProvider)}}
	require.NoError(t, PrepareRoute(resolver.DefaultRegistry(), routeName, rc))
	k.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeName: rc})
}

func streamedChunkOf(t *testing.T, resp *extprocv3.ProcessingResponse) *extprocv3.StreamedBodyResponse {
	t.Helper()
	streamed := resp.GetResponseBody().GetResponse().GetBodyMutation().GetStreamedResponse()
	require.NotNil(t, streamed, "a streamed response body must be answered with a streamed mutation")
	return streamed
}

// The SSE stream of an LLM route no policy reads reaches the client byte for byte while
// the engine reads it for the span, and the request body the span needs is kept.
func TestStreamingResponse_GenAIOnlySSEByteExact(t *testing.T) {
	server, k, _ := newSpanStatusServer(t)
	enableGenAI(server)
	llmRoute(t, k, "llm", buildChainFor([]policy.Policy{&testutils.NoopPolicy{}}))

	headers := requestHeadersReq("llm", "POST", "/chat/completions")
	headers.GetRequestHeaders().EndOfStream = false
	chunks := []string{
		"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n",
		"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n",
		"data: [DONE]\n\n",
	}
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		headers,
		requestBodyReq(`{"model":"gpt-4o","stream":true}`),
		sseResponseHeadersReq(),
		responseChunkReq(chunks[0], false),
		responseChunkReq(chunks[1], false),
		responseChunkReq(chunks[2], true),
	})
	require.NoError(t, server.Process(stream))
	require.Len(t, stream.responses, 6)

	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED_PARTIAL, stream.responses[0].GetModeOverride().GetRequestBodyMode())
	assert.Equal(t, extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED, stream.responses[2].GetModeOverride().GetResponseBodyMode())
	assert.Nil(t, stream.responses[1].GetRequestBody().GetResponse().GetBodyMutation(), "the request is forwarded untouched")
	for i, chunk := range chunks {
		streamed := streamedChunkOf(t, stream.responses[3+i])
		assert.Equal(t, chunk, string(streamed.GetBody()), "chunk %d", i)
		assert.Equal(t, i == len(chunks)-1, streamed.GetEndOfStream())
	}
}

// The capture keeps the forwarded request and the upstream response, decoded.
func TestGenAICapture_KeepsRequestAndResponse(t *testing.T) {
	ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{})

	_, err := ec.processRequestBody(t.Context(), &extprocv3.HttpBody{Body: []byte(`{"model":"gpt-4o"}`), EndOfStream: true})
	require.NoError(t, err)
	assert.Equal(t, `{"model":"gpt-4o"}`, string(ec.genaiRequestBody()))

	ec.buildResponseContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("application/json")},
	}}})
	_, err = ec.processResponseBody(t.Context(), &extprocv3.HttpBody{Body: []byte(`{"id":"r1"}`), EndOfStream: true})
	require.NoError(t, err)
	require.NotNil(t, ec.genai.response)
	assert.Equal(t, `{"id":"r1"}`, string(ec.genai.response.Bytes()))
	assert.True(t, ec.genai.responseComplete)
	assert.False(t, ec.genai.firstChunkAt.IsZero())
}

// With GenAI off nothing is kept.
func TestGenAICapture_OffKeepsNothing(t *testing.T) {
	server, _, _ := newSpanStatusServer(t)
	ec := newPolicyExecutionContext(server, "route", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}, RouteMetadata{})
	ec.buildResponseContexts(sseHeaders())
	_, err := ec.processResponseBody(t.Context(), &extprocv3.HttpBody{Body: []byte("data: x\n\n"), EndOfStream: true})
	require.NoError(t, err)
	assert.Nil(t, ec.genai.response)
}

// A compressed stream read only for the span still reaches the client as one valid
// compressed stream, and the span sees it decoded.
func TestStreamingResponse_GenAIOnlyCompressedStreamDecodes(t *testing.T) {
	for _, encoding := range []string{"gzip", "br"} {
		t.Run(encoding, func(t *testing.T) {
			ec := genaiExecCtx(t, policy.APIKindLlmProvider, &registry.PolicyChain{})
			ec.buildResponseContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
				{Key: ":status", RawValue: []byte("200")},
				{Key: "content-type", RawValue: []byte("text/event-stream")},
				{Key: "content-encoding", RawValue: []byte(encoding)},
			}}})
			ec.isStreamingResponse = ec.responseStreamingEnabled(false)
			require.True(t, ec.isStreamingResponse)

			chunks := []string{"data: {\"id\":\"a\"}\n\n", "data: {\"id\":\"b\"}\n\n", "data: [DONE]\n\n"}
			var wire []byte
			for i, c := range encodeStreamChunks(t, chunks, encoding) {
				resp, err := ec.processResponseBody(t.Context(), &extprocv3.HttpBody{Body: c, EndOfStream: i == len(chunks)-1})
				require.NoError(t, err)
				wire = append(wire, streamedChunkOf(t, resp).GetBody()...)
			}
			assert.Equal(t, "data: {\"id\":\"a\"}\n\ndata: {\"id\":\"b\"}\n\ndata: [DONE]\n\n", string(decodeWire(t, wire, encoding)))
			assert.Equal(t, "data: {\"id\":\"a\"}\n\ndata: {\"id\":\"b\"}\n\ndata: [DONE]\n\n", string(ec.genai.response.Bytes()))
		})
	}
}
