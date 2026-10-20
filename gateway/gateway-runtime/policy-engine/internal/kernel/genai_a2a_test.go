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

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
)

const a2aRouteKey = "POST|/weather/1.0.0|example.com"

func a2aStream(body, status string) *mockExtProcStream {
	return newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersWithBodyToFollow(a2aRouteKey, "POST", "/weather/1.0.0"),
		requestBodyReq(body),
		jsonResponseHeadersReq(status),
		responseBodyReq(`{"jsonrpc":"2.0","id":1,"result":{}}`),
	})
}

// Sending a message to an agent is an invoke_agent CLIENT span named for the agent,
// carrying the conversation and the ids the ext_proc spans no longer hold.
func TestProcess_A2AInvokeAgentSpan(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	enableGenAI(server)
	registerA2AJSONRPCRoute(t, k, a2aRouteKey, "SendMessage")

	require.NoError(t, server.Process(a2aStream(`{"jsonrpc":"2.0","id":1,"method":"SendMessage",`+
		`"params":{"message":{"messageId":"msg-1","contextId":"ctx-1","taskId":"task-1"}}}`, "200")))

	ended := sr.Ended()
	span := spanByName(ended, "invoke_agent WeatherAgent")
	require.NotNil(t, span)
	root := spanByName(ended, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, trace.SpanKindClient, span.SpanKind())
	assert.Equal(t, root.SpanContext().SpanID(), span.Parent().SpanID())
	assertSpanAttrs(t, span, map[string]string{
		genai.AttrOperationName:  "invoke_agent",
		genai.AttrProviderName:   "a2a",
		genai.AttrAgentName:      "WeatherAgent",
		genai.AttrConversationID: "ctx-1",
		"a2a.message.id":         "msg-1",
		"a2a.task.id":            "task-1",
		"a2a.operation":          "SendMessage",
	})
	assert.Equal(t, codes.Unset, span.Status().Code)
}

// Reading a task does not invoke the agent.
func TestProcess_A2ANoSpanForGetTask(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	enableGenAI(server)
	registerA2AJSONRPCRoute(t, k, a2aRouteKey, "GetTask")

	require.NoError(t, server.Process(a2aStream(`{"jsonrpc":"2.0","id":1,"method":"GetTask","params":{"id":"task-1"}}`, "200")))
	for _, s := range sr.Ended() {
		assert.NotEqual(t, trace.SpanKindClient, s.SpanKind(), "unexpected client span %q", s.Name())
	}
}

func TestProcess_A2AInvokeAgentSpan_Streaming(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	enableGenAI(server)
	registerA2AJSONRPCRoute(t, k, a2aRouteKey, "SendStreamingMessage")

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersWithBodyToFollow(a2aRouteKey, "POST", "/weather/1.0.0"),
		requestBodyReq(`{"jsonrpc":"2.0","id":1,"method":"SendStreamingMessage","params":{"message":{"messageId":"m","contextId":"c"}}}`),
		sseResponseHeadersReq(),
		responseChunkReq("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"kind\":\"status-update\"}}\n\n", false),
		responseChunkReq("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"kind\":\"status-update\",\"final\":true}}\n\n", true),
	})
	require.NoError(t, server.Process(stream))

	span := spanByName(sr.Ended(), "invoke_agent WeatherAgent")
	require.NotNil(t, span)
	assert.Equal(t, "SendStreamingMessage", attrString(t, span, "a2a.operation"))
	assert.Equal(t, "c", attrString(t, span, genai.AttrConversationID))
}

// An agent that answers with an error status fails the span.
func TestProcess_A2AInvokeAgentSpan_UpstreamError(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	enableGenAI(server)
	registerA2AJSONRPCRoute(t, k, a2aRouteKey, "SendMessage")

	require.NoError(t, server.Process(a2aStream(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"messageId":"m"}}}`, "503")))
	span := spanByName(sr.Ended(), "invoke_agent WeatherAgent")
	require.NotNil(t, span)
	assert.Equal(t, "503", attrString(t, span, genai.AttrErrorType))
	assert.Equal(t, codes.Error, span.Status().Code)
}
