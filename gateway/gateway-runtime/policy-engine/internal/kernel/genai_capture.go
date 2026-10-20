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
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// genaiCapture is what the engine keeps of an AI route's exchange for its GenAI client
// span: the request as forwarded and the response as the upstream sent it.
//
// The bodies are read even when no policy needs them, which is the cost of GenAI
// tracing: token usage and the response metadata are only in the body. The modes are
// chosen so the read never changes the exchange: a request or a buffered response is
// read with BUFFERED_PARTIAL, which never rejects a body over the buffer limit (Envoy
// sends what fits and streams the rest past the engine), and a streamed response stays
// streamed.
type genaiCapture struct {
	// kind is the API kind a client span is recorded for, or "" when none is.
	kind policy.APIKind
	// requestBody is the request body as forwarded, kept only when no request policy
	// read it; otherwise the request context holds it.
	requestBody []byte
	// response is the upstream's response body, before any response policy ran.
	response *genai.Window
	// responseComplete reports that the whole response body was read.
	responseComplete bool
	// firstChunkAt is when the first response body bytes arrived.
	firstChunkAt time.Time

	// span is the client span (genai_trace.go), started when the request is
	// forwarded, at startedAt, and ended once.
	span      trace.Span
	startedAt time.Time
	ended     bool
	// streaming reports that the request asked for a streamed response.
	streaming bool
}

// genaiKindFor returns the API kind a client span is recorded for on a route of kind,
// or "" for none. An LLM proxy route records none: its upstream is the provider route,
// which does, so tokens are counted once.
func (s *ExternalProcessorServer) genaiKindFor(kind policy.APIKind) policy.APIKind {
	if !s.genai.Enabled {
		return ""
	}
	switch kind {
	case policy.APIKindLlmProvider, policy.APIKindMCP, policy.APIKindAgent:
		return kind
	}
	return ""
}

// genaiReadsRequestBody reports whether the engine reads the request body for the span:
// on an LLM provider route, where it names the model and says whether the call streams.
// A streamed request is left alone — its mode follows the chain's streaming policies.
func (ec *PolicyExecutionContext) genaiReadsRequestBody() bool {
	return ec.genai.kind == policy.APIKindLlmProvider && !ec.isStreamingRequest
}

// genaiReadsResponseBody reports whether the engine reads the response body for the span:
// on an LLM provider route for the usage and response metadata, and on the MCP POST
// route for the JSON-RPC outcome. An A2A response is not read; its span takes the
// outcome from the status.
func (ec *PolicyExecutionContext) genaiReadsResponseBody() bool {
	switch ec.genai.kind {
	case policy.APIKindLlmProvider:
		return true
	case policy.APIKindMCP:
		return ec.resolverName == resolver.MCPResolverName
	}
	return false
}

// genaiStreamsResponse reports whether a streamed LLM response is processed as a stream
// for the span alone: no response policy reads the body, so nothing else would.
func (ec *PolicyExecutionContext) genaiStreamsResponse() bool {
	return ec.genai.kind == policy.APIKindLlmProvider && ec.policyChain != nil && !ec.policyChain.RequiresResponseBody
}

// captureRequestBody keeps the forwarded request body, decoded when it is compressed. A
// body that does not decode is kept as it is; the span then simply finds no model in it.
func (ec *PolicyExecutionContext) captureRequestBody(body []byte) {
	if ec.requestContentEncoding != "" {
		if decoded, err := decompressBody(body, ec.requestContentEncoding, ec.server.maxRequestDecompressedBytes); err == nil {
			body = decoded
		}
	}
	ec.genai.requestBody = append([]byte(nil), body...)
}

// genaiRequestBody returns the request body as forwarded: the request context's, which
// reflects every request policy's rewrite, or the one kept when no policy read it.
func (ec *PolicyExecutionContext) genaiRequestBody() []byte {
	if ec.requestBodyCtx != nil && ec.requestBodyCtx.Body != nil && ec.requestBodyCtx.Body.Present {
		return ec.requestBodyCtx.Body.Content
	}
	return ec.genai.requestBody
}

// captureResponse appends decoded upstream response bytes for the span.
func (ec *PolicyExecutionContext) captureResponse(chunk []byte, endOfStream bool) {
	if ec.genai.kind == "" {
		return
	}
	if ec.genai.response == nil {
		ec.genai.response = genai.NewWindow(genai.DefaultWindowBytes)
	}
	if len(chunk) > 0 {
		if ec.genai.firstChunkAt.IsZero() {
			ec.genai.firstChunkAt = time.Now()
		}
		ec.genai.response.Write(chunk)
	}
	if endOfStream {
		ec.genai.responseComplete = true
	}
}
