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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"strconv"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
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
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

const mcpRouteKey = "POST|/weather/mcp|example.com"

// mcpServer is a server with an MCP POST route bound by the real MCP resolver.
func mcpServer(t *testing.T, genaiOn bool) (*ExternalProcessorServer, *tracetest.SpanRecorder) {
	t.Helper()
	server, k, sr := newSpanStatusServer(t)
	if genaiOn {
		enableGenAI(server)
	}
	rc := &RouteConfig{
		Metadata: RouteMetadata{
			RouteName:       mcpRouteKey,
			APIId:           "mcp-1",
			APIName:         "weather-mcp",
			APIKind:         string(policy.APIKindMCP),
			Vhost:           "example.com",
			DefaultUpstream: &policyenginev1.UpstreamInfo{URL: "http://mcp-backend:3001/mcp"},
		},
		RouteResolution: resolver.RouteResolution{
			ResolverName:   resolver.MCPResolverName,
			ResolverConfig: json.RawMessage(`{"specVersion":"2025-06-18"}`),
		},
	}
	require.NoError(t, PrepareRoute(resolver.DefaultRegistry(), mcpRouteKey, rc))
	k.RegisterRoute(resolver.ChainKeyFor("mcp-1", "example.com", "mcp"), buildChainFor([]policy.Policy{&testutils.NoopPolicy{}}))
	k.ApplyWholeRouteConfigs(map[string]*RouteConfig{mcpRouteKey: rc})
	return server, sr
}

func mcpHeaders(extra ...*corev3.HeaderValue) *extprocv3.ProcessingRequest {
	req := requestHeadersReq(mcpRouteKey, "POST", "/weather/mcp")
	req.GetRequestHeaders().EndOfStream = false
	headers := req.GetRequestHeaders().GetHeaders()
	headers.Headers = append(headers.Headers,
		&corev3.HeaderValue{Key: "content-type", RawValue: []byte("application/json")},
		&corev3.HeaderValue{Key: "mcp-session-id", RawValue: []byte("sess-1")},
	)
	headers.Headers = append(headers.Headers, extra...)
	return req
}

const toolCall = `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_forecast","arguments":{"city":"Colombo"}}}`

func runMCP(t *testing.T, server *ExternalProcessorServer, reqBody, respBody string, extra ...*corev3.HeaderValue) *mockExtProcStream {
	t.Helper()
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		mcpHeaders(extra...),
		requestBodyReq(reqBody),
		jsonResponseHeadersReq("200"),
		responseBodyReq(respBody),
	})
	require.NoError(t, server.Process(stream))
	require.Len(t, stream.responses, 4)
	return stream
}

// forwardedBody is the request body the engine sent upstream, with its Content-Length.
func forwardedBody(t *testing.T, resp *extprocv3.ProcessingResponse) ([]byte, int) {
	t.Helper()
	common := resp.GetRequestBody().GetResponse()
	body := common.GetBodyMutation().GetBody()
	require.NotNil(t, body, "the body must be rewritten")
	length := -1
	for _, h := range common.GetHeaderMutation().GetSetHeaders() {
		if h.GetHeader().GetKey() == "content-length" {
			length, _ = strconv.Atoi(string(h.GetHeader().GetRawValue()))
		}
	}
	return body, length
}

func metaTraceparent(t *testing.T, body []byte) string {
	t.Helper()
	return genai.MCPMetaContext(body).Get("traceparent")
}

func traceparentOf(span sdktrace.ReadOnlySpan) string {
	sc := span.SpanContext()
	return "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-" + sc.TraceFlags().String()
}

func TestProcess_MCPToolCallSpan(t *testing.T) {
	server, sr := mcpServer(t, true)
	stream := runMCP(t, server, toolCall, `{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"sunny"}]}}`)

	ended := sr.Ended()
	span := spanByName(ended, "tools/call get_forecast")
	require.NotNil(t, span)
	root := spanByName(ended, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, trace.SpanKindClient, span.SpanKind())
	assert.Equal(t, root.SpanContext().SpanID(), span.Parent().SpanID())
	assert.Empty(t, span.Links())
	assert.Equal(t, codes.Unset, span.Status().Code)
	assertSpanAttrs(t, span, map[string]string{
		genai.AttrMCPMethodName:       "tools/call",
		genai.AttrOperationName:       "execute_tool",
		genai.AttrToolName:            "get_forecast",
		genai.AttrJSONRPCRequestID:    "7",
		genai.AttrMCPSessionID:        "sess-1",
		genai.AttrNetworkTransport:    "tcp",
		genai.AttrNetworkProtocolName: "http",
		genai.AttrServerAddress:       "mcp-backend",
	})

	// The server receives the gateway's MCP span as its parent, in _meta.
	body, length := forwardedBody(t, stream.responses[1])
	assert.Equal(t, traceparentOf(span), metaTraceparent(t, body))
	assert.Equal(t, len(body), length, "Content-Length must match the rewritten body")
	var req map[string]any
	require.NoError(t, json.Unmarshal(body, &req))
	assert.Equal(t, "get_forecast", req["params"].(map[string]any)["name"])

	// The tool name is on the MCP span only, never on the ext_proc spans (#3737).
	_, onRoot := attrOf(root, attribute.Key(resolver.AttrMCPBodyCapabilityName))
	assert.False(t, onRoot)
}

// A client whose HTTP layer did not propagate its trace still sends it in _meta: the
// MCP span joins that trace and links to the gateway's.
func TestProcess_MCPSpanParentsOnAnotherTracesMeta(t *testing.T) {
	server, sr := mcpServer(t, true)
	const carried = "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01"
	runMCP(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"traceparent":"`+carried+`"}}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`)

	span := spanByName(sr.Ended(), "tools/list")
	require.NotNil(t, span)
	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", span.SpanContext().TraceID().String())
	assert.Equal(t, "bbbbbbbbbbbbbbbb", span.Parent().SpanID().String())
	require.Len(t, span.Links(), 1)
	assert.Equal(t, root.SpanContext(), span.Links()[0].SpanContext)
	_, isTool := attrOf(span, attribute.Key(genai.AttrOperationName))
	assert.False(t, isTool, "execute_tool is for tool calls only")
}

// When _meta carries the gateway's own trace, the span nests under the root.
func TestProcess_MCPSpanNestsWhenMetaIsTheSameTrace(t *testing.T) {
	installW3CPropagator(t)
	server, sr := mcpServer(t, true)
	server.tracingEnabled = true
	const incoming = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	runMCP(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"traceparent":"`+incoming+`"}}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`,
		&corev3.HeaderValue{Key: "traceparent", RawValue: []byte(incoming)})

	span := spanByName(sr.Ended(), "tools/list")
	require.NotNil(t, span)
	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	assert.Equal(t, root.SpanContext().SpanID(), span.Parent().SpanID())
	assert.Empty(t, span.Links())
}

func TestProcess_MCPSpanRecordsTheServersAnswer(t *testing.T) {
	cases := map[string]struct {
		response  string
		errorType string
		rpcStatus string
	}{
		"tool error":     {`{"jsonrpc":"2.0","id":7,"result":{"isError":true,"content":[]}}`, genai.ErrorTypeToolError, ""},
		"jsonrpc error":  {`{"jsonrpc":"2.0","id":7,"error":{"code":-32602,"message":"unknown tool"}}`, "-32602", "-32602"},
		"sse with error": {"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":7,\"error\":{\"code\":-32603}}\n\n", "-32603", "-32603"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server, sr := mcpServer(t, true)
			runMCP(t, server, toolCall, tc.response)
			span := spanByName(sr.Ended(), "tools/call get_forecast")
			require.NotNil(t, span)
			assert.Equal(t, codes.Error, span.Status().Code)
			assert.Equal(t, tc.errorType, attrString(t, span, genai.AttrErrorType))
			status, ok := attrOf(span, attribute.Key(genai.AttrRPCStatusCode))
			assert.Equal(t, tc.rpcStatus != "", ok)
			if ok {
				assert.Equal(t, tc.rpcStatus, status.AsString())
			}
		})
	}
}

// A compressed request is rewritten and recompressed, like any policy rewrite.
func TestProcess_MCPGzipRequestRecompressed(t *testing.T) {
	server, sr := mcpServer(t, true)
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(toolCall))
	require.NoError(t, w.Close())

	stream := runMCP(t, server, gz.String(), `{"jsonrpc":"2.0","id":7,"result":{}}`,
		&corev3.HeaderValue{Key: "content-encoding", RawValue: []byte("gzip")})

	body, length := forwardedBody(t, stream.responses[1])
	assert.Equal(t, len(body), length)
	zr, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	plain, err := io.ReadAll(zr)
	require.NoError(t, err)
	span := spanByName(sr.Ended(), "tools/call get_forecast")
	require.NotNil(t, span)
	assert.Equal(t, traceparentOf(span), metaTraceparent(t, plain))
}

// With GenAI tracing off the request is forwarded exactly as before.
func TestProcess_MCPGenAIOffLeavesTheBodyAlone(t *testing.T) {
	server, sr := mcpServer(t, false)
	stream := runMCP(t, server, toolCall, `{"jsonrpc":"2.0","id":7,"result":{}}`)
	assert.Nil(t, stream.responses[1].GetRequestBody().GetResponse().GetBodyMutation())
	assert.Nil(t, spanByName(sr.Ended(), "tools/call get_forecast"))
}
