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
	"strings"
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func runChat(t *testing.T, capture bool, chain *registry.PolicyChain, reqBody, respBody string) sdktrace.ReadOnlySpan {
	t.Helper()
	server, sr := genaiServer(t, policy.APIKindLlmProvider, chain)
	server.genai.CaptureContent = capture
	require.NoError(t, server.Process(newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(reqBody),
		jsonResponseHeadersReq("200"),
		responseBodyReq(respBody),
	})))
	span := spanByName(sr.Ended(), "chat gpt-4o")
	require.NotNil(t, span)
	return span
}

const (
	chatRequest  = `{"model":"gpt-4o","messages":[{"role":"user","content":"my card is 4111 1111 1111 1111"}]}`
	chatResponse = `{"id":"c","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Noted."},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`
)

// Prompts and completions can hold personal data: they are never recorded unless
// content capture is turned on.
func TestContentCaptureOffByDefault(t *testing.T) {
	span := runChat(t, false, passThroughChain(), chatRequest, chatResponse)
	for _, key := range []string{genai.AttrInputMessages, genai.AttrOutputMessages, genai.AttrSystemInstructions} {
		_, ok := attrOf(span, attribute.Key(key))
		assert.False(t, ok, "%s must not be recorded", key)
	}
}

// Content is captured as forwarded: after the masking and guardrail policies ran.
func TestContentCapture_RecordsTheRequestAsPoliciesLeftIt(t *testing.T) {
	masked := `{"model":"gpt-4o","messages":[{"role":"user","content":"my card is [REDACTED]"}]}`
	span := runChat(t, true, buildChainFor([]policy.Policy{&mutatingBodyPolicy{replacement: []byte(masked)}}), chatRequest, chatResponse)

	assert.JSONEq(t, `[{"role":"user","parts":[{"type":"text","content":"my card is [REDACTED]"}]}]`, attrString(t, span, genai.AttrInputMessages))
	assert.JSONEq(t, `[{"role":"assistant","parts":[{"type":"text","content":"Noted."}]}]`, attrString(t, span, genai.AttrOutputMessages))
	_, cut := attrOf(span, attribute.Key(genai.AttrContentTruncated))
	assert.False(t, cut)
}

func TestContentCapture_TruncatesToTheBudget(t *testing.T) {
	server, sr := genaiServer(t, policy.APIKindLlmProvider, passThroughChain())
	server.genai.CaptureContent = true
	server.genai.MaxContentBytes = 64
	require.NoError(t, server.Process(newMockStream([]*extprocv3.ProcessingRequest{
		chatHeaders(),
		requestBodyReq(`{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("a", 500) + `"}]}`),
	})))
	span := spanByName(sr.Ended(), "chat gpt-4o")
	require.NotNil(t, span)
	assert.LessOrEqual(t, len(attrString(t, span, genai.AttrInputMessages)), 64)
	assert.Equal(t, "true", attrString(t, span, genai.AttrContentTruncated))
}

// A tool call's arguments and result are recorded as the JSON they were.
func TestContentCapture_MCPToolArgumentsAndResult(t *testing.T) {
	server, sr := mcpServer(t, true)
	server.genai.CaptureContent = true
	runMCP(t, server, toolCall, `{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"sunny"}]}}`)

	span := spanByName(sr.Ended(), "tools/call get_forecast")
	require.NotNil(t, span)
	assert.JSONEq(t, `{"city":"Colombo"}`, attrString(t, span, genai.AttrToolCallArguments))
	assert.JSONEq(t, `{"content":[{"type":"text","text":"sunny"}]}`, attrString(t, span, genai.AttrToolCallResult))
}

func TestContentCapture_MCPOverBudgetIsLeftOut(t *testing.T) {
	server, sr := mcpServer(t, true)
	server.genai.CaptureContent = true
	server.genai.MaxContentBytes = 8
	runMCP(t, server, toolCall, `{"jsonrpc":"2.0","id":7,"result":{}}`)

	span := spanByName(sr.Ended(), "tools/call get_forecast")
	require.NotNil(t, span)
	_, ok := attrOf(span, attribute.Key(genai.AttrToolCallArguments))
	assert.False(t, ok, "JSON cannot be cut and stay valid, so it is left out")
	assert.Equal(t, "true", attrString(t, span, genai.AttrContentTruncated))
}
