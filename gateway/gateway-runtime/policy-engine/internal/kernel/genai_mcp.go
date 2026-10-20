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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// startMCPSpan starts the client span for a JSON-RPC request on the MCP POST route and
// rewrites params._meta so the server's own spans join it. It runs at the end of
// bindPendingChainAndProcess, after the chain's policies and only when none answered
// the request, so the span starts as the request is forwarded and the rewrite applies
// to the body as forwarded.
//
// The span follows the MCP conventions on context: when _meta carries a context of
// another trace (a client whose HTTP layer did not propagate its trace), the span is
// that context's child and links to the gateway's root; otherwise it is the root's
// child. The rewrite is appended to the body results so the merge that emits them
// re-encodes, recompresses and re-measures the body as for any policy's rewrite.
func (ec *PolicyExecutionContext) startMCPSpan(bodyResult *executor.RequestExecutionResult) {
	if ec.genai.kind != policy.APIKindMCP || ec.resolverName != resolver.MCPResolverName ||
		ec.genai.span != nil || ec.rootSpan == nil {
		return
	}
	facts := ec.resolutionAttributes
	method := facts[resolver.AttrMCPBodyMethod]
	if method == "" || facts[resolver.AttrMCPBodyUnusable] != "" {
		return // not a JSON-RPC request the span could describe
	}

	name := method
	attrs := []attribute.KeyValue{
		attribute.String(genai.AttrMCPMethodName, method),
		attribute.String(genai.AttrNetworkTransport, "tcp"),
		attribute.String(genai.AttrNetworkProtocolName, "http"),
	}
	target := facts[resolver.AttrMCPBodyCapabilityName]
	switch facts[resolver.AttrMCPBodyCapabilityType] {
	case "tool":
		if method == "tools/call" {
			attrs = append(attrs, attribute.String(genai.AttrOperationName, genai.OperationExecuteTool))
		}
		if target != "" {
			attrs = append(attrs, attribute.String(genai.AttrToolName, target))
			name += " " + target
		}
	case "prompt":
		if target != "" {
			attrs = append(attrs, attribute.String(genai.AttrPromptName, target))
			name += " " + target
		}
	case "resource":
		if target != "" {
			attrs = append(attrs, attribute.String(genai.AttrMCPResourceURI, target))
		}
	}
	if id := jsonRPCID(facts[resolver.AttrMCPBodyJSONRPCID]); id != "" {
		attrs = append(attrs, attribute.String(genai.AttrJSONRPCRequestID, id))
	}
	if v := facts[resolver.AttrMCPBodyProtocolVersion]; v != "" {
		attrs = append(attrs, attribute.String(genai.AttrMCPProtocolVersion, v))
	} else if v := ec.requestHeader("mcp-protocol-version"); v != "" {
		attrs = append(attrs, attribute.String(genai.AttrMCPProtocolVersion, v))
	}
	if v := ec.requestHeader("mcp-session-id"); v != "" {
		attrs = append(attrs, attribute.String(genai.AttrMCPSessionID, v))
	}
	attrs = append(attrs, ec.serverAttributes()...)

	body := ec.genaiRequestBody()
	parent := trace.ContextWithSpan(context.Background(), ec.rootSpan)
	opts := []trace.SpanStartOption{trace.WithAttributes(attrs...)}
	carried := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), genai.MCPMetaContext(body)))
	if carried.IsValid() && carried.TraceID() != ec.rootSpan.SpanContext().TraceID() {
		parent = trace.ContextWithRemoteSpanContext(context.Background(), carried)
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: ec.rootSpan.SpanContext()}))
	}
	ctx := ec.startClientSpanUnder(parent, name, opts...)

	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	if rewritten, ok := genai.RewriteMCPMeta(body, carrier); ok {
		bodyResult.Results = append(bodyResult.Results, executor.RequestPolicyResult{
			Action: policy.UpstreamRequestModifications{Body: rewritten},
		})
	}
}

// finishMCPSpan records the JSON-RPC outcome. A JSON-RPC error is an error on a client
// span whatever its code; a tool result with isError is a failed tool call.
func (ec *PolicyExecutionContext) finishMCPSpan(span trace.Span) {
	if ec.genai.response == nil {
		return
	}
	outcome, ok := genai.ParseMCPResponse(ec.genai.response.Bytes())
	switch {
	case !ok:
	case outcome.ErrorCode != "":
		span.SetAttributes(
			attribute.String(genai.AttrErrorType, outcome.ErrorCode),
			attribute.String(genai.AttrRPCStatusCode, outcome.ErrorCode),
		)
		span.SetStatus(codes.Error, "")
	case outcome.ToolError:
		span.SetAttributes(attribute.String(genai.AttrErrorType, genai.ErrorTypeToolError))
		span.SetStatus(codes.Error, "")
	}
}

// jsonRPCID turns the resolver's id token (7, or "7" with its quotes) into the id's
// text.
func jsonRPCID(token string) string {
	if token == "" {
		return ""
	}
	var s string
	if json.Unmarshal([]byte(token), &s) == nil {
		return s
	}
	return token
}
