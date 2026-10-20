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
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
	"github.com/wso2/api-platform/sdk/ai/llmusage"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/sdk/core/utils"
)

// The GenAI client span of an exchange is the call the gateway makes to an LLM provider,
// an MCP server or an A2A agent on the caller's behalf. Process owns it: it starts once
// the request has left the policy chain for the upstream (startGenAISpan, after each
// message) and ends with the response (endGenAISpan, at endsResponse and in the
// teardown defer, so every way out of the stream ends it). Its duration is therefore
// the upstream's own time, beside the gateway's time in the policy spans.

// startGenAISpan starts the client span once the request has been forwarded. A request
// a policy answered, or one that failed resolution, made no call and gets no span.
func (ec *PolicyExecutionContext) startGenAISpan(req *extprocv3.ProcessingRequest, resp *extprocv3.ProcessingResponse) {
	if ec.genai.kind == "" || ec.genai.span != nil || ec.rootSpan == nil || !ec.requestForwarded(req, resp) {
		return
	}
	switch ec.genai.kind {
	case policy.APIKindLlmProvider:
		ec.startInferenceSpan()
	case policy.APIKindAgent:
		ec.startAgentSpan()
	}
}

// requestForwarded reports whether, with resp, the request leaves the engine for the
// upstream: its last request message was answered with anything but an immediate
// response.
func (ec *PolicyExecutionContext) requestForwarded(req *extprocv3.ProcessingRequest, resp *extprocv3.ProcessingResponse) bool {
	if resp == nil || resp.GetImmediateResponse() != nil {
		return false
	}
	switch r := req.GetRequest().(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		if ec.pending != nil {
			return false // the chain is bound at the body, which is still to come
		}
		if r.RequestHeaders.GetEndOfStream() {
			return true
		}
		// No override leaves the filter's static mode, which sends no request body.
		mode := resp.GetModeOverride()
		return mode == nil || mode.GetRequestBodyMode() == extprocconfigv3.ProcessingMode_NONE
	case *extprocv3.ProcessingRequest_RequestBody:
		return !ec.isStreamingRequest || r.RequestBody.GetEndOfStream()
	}
	return false
}

// startClientSpan starts a GenAI client span as a child of the root span.
func (ec *PolicyExecutionContext) startClientSpan(name string, attrs ...attribute.KeyValue) {
	ec.startClientSpanUnder(trace.ContextWithSpan(context.Background(), ec.rootSpan), name, trace.WithAttributes(attrs...))
}

// startClientSpanUnder starts the GenAI client span under parent and returns its context.
func (ec *PolicyExecutionContext) startClientSpanUnder(parent context.Context, name string, opts ...trace.SpanStartOption) context.Context {
	ctx, span := ec.server.tracer.Start(parent, name, append(opts, trace.WithSpanKind(trace.SpanKindClient))...)
	ec.genai.span = span
	ec.genai.startedAt = time.Now()
	return ctx
}

// endGenAISpan finishes and ends the client span. Safe to call more than once and when
// no span started.
func (ec *PolicyExecutionContext) endGenAISpan() {
	span := ec.genai.span
	if span == nil || ec.genai.ended {
		return
	}
	ec.genai.ended = true
	if span.IsRecording() {
		ec.recordUpstreamOutcome(span)
		switch ec.genai.kind {
		case policy.APIKindLlmProvider:
			ec.finishInferenceSpan(span)
		case policy.APIKindMCP:
			ec.finishMCPSpan(span)
		case policy.APIKindAgent:
			ec.finishAgentSpan(span)
		}
	}
	span.End()
}

// recordUpstreamOutcome marks the span failed when the upstream answered with an error
// status, or never answered at all. A client span treats every 4xx as an error: the
// call it made failed.
func (ec *PolicyExecutionContext) recordUpstreamOutcome(span trace.Span) {
	status := ec.upstreamStatus()
	switch {
	case status == 0:
		span.SetAttributes(attribute.String(genai.AttrErrorType, genai.ErrorTypeOther))
		span.SetStatus(codes.Error, "no response from the upstream")
	case status >= 400:
		span.SetAttributes(attribute.String(genai.AttrErrorType, strconv.Itoa(status)))
		span.SetStatus(codes.Error, "")
	}
}

// upstreamStatus is the status the upstream answered with, before any policy changed
// it, or 0 when no response arrived.
func (ec *PolicyExecutionContext) upstreamStatus() int {
	if ec.responseHeaderCtx == nil {
		return 0
	}
	if up := ec.responseHeaderCtx.Upstream; up != nil && up.Response != nil && up.Response.StatusCode != 0 {
		return up.Response.StatusCode
	}
	return ec.responseHeaderCtx.ResponseStatus
}

// serverAttributes are the upstream's address and port, from the route's upstream URL.
func (ec *PolicyExecutionContext) serverAttributes() []attribute.KeyValue {
	if ec.defaultUpstream == nil || ec.defaultUpstream.URL == "" {
		return nil
	}
	u, err := url.Parse(ec.defaultUpstream.URL)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[u.Scheme]
	}
	attrs := []attribute.KeyValue{attribute.String(genai.AttrServerAddress, u.Hostname())}
	if n, err := strconv.Atoi(port); err == nil {
		attrs = append(attrs, attribute.Int(genai.AttrServerPort, n))
	}
	return attrs
}

// ─── LLM inference ───────────────────────────────────────────────────────────

// resourcePath is the request path within the API, the form provider templates
// declare their resource mappings in: no query, no API context.
func (ec *PolicyExecutionContext) resourcePath() string {
	path := ec.requestHeaderCtx.Path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if sc := ec.sharedCtx; sc != nil {
		if apiCtx := strings.ReplaceAll(sc.APIContext, "$version", sc.APIVersion); apiCtx != "" && apiCtx != "/" {
			path = strings.TrimPrefix(path, apiCtx)
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// templateName is the route's provider template, which names the provider.
func (ec *PolicyExecutionContext) templateName() string {
	name, _ := ec.sharedCtx.Metadata[llmusage.MetadataTemplateHandle].(string)
	return name
}

// startInferenceSpan starts "{operation} {model}". A path that is not an inference
// endpoint (a model listing, say) gets no span.
func (ec *PolicyExecutionContext) startInferenceSpan() {
	path := ec.resourcePath()
	operation := genai.OperationName(path)
	if operation == "" {
		return
	}
	body := ec.genaiRequestBody()
	attrs := []attribute.KeyValue{attribute.String(genai.AttrOperationName, operation)}
	if provider := genai.ProviderName(ec.templateName()); provider != "" {
		attrs = append(attrs, attribute.String(genai.AttrProviderName, provider))
	}
	name := operation
	if model := ec.requestModel(body, path); model != "" {
		attrs = append(attrs, attribute.String(genai.AttrRequestModel, model))
		name += " " + model
	}
	if requestStreams(body, path) {
		ec.genai.streaming = true
		attrs = append(attrs, attribute.Bool(genai.AttrRequestStream, true))
	}
	attrs = append(attrs, ec.serverAttributes()...)
	ec.startClientSpan(name, attrs...)
}

// requestModel is the model the request asks for: where the route's template declares
// it in the body, or else in the path, for the providers that put it there.
func (ec *PolicyExecutionContext) requestModel(body []byte, path string) string {
	if jsonPath := llmusage.FieldPaths(ec.sharedCtx, path)["requestModel"]; jsonPath != "" && len(body) > 0 {
		if model, err := utils.ExtractStringValueFromJsonpath(body, jsonPath); err == nil && model != "" {
			return model
		}
	}
	return genai.ModelFromPath(path)
}

// requestStreams reports whether the request asks for a streamed response: a
// "stream": true body (OpenAI, Anthropic, Mistral and the OpenAI-compatible APIs) or a
// streaming endpoint (Gemini, Bedrock).
func requestStreams(body []byte, path string) bool {
	p := strings.ToLower(path)
	if strings.Contains(p, "streamgeneratecontent") || strings.Contains(p, "/converse-stream") ||
		strings.Contains(p, "/invoke-with-response-stream") {
		return true
	}
	if len(body) == 0 || !bytes.Contains(body, []byte(`"stream"`)) {
		return false
	}
	var req struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &req) == nil && req.Stream
}

// finishInferenceSpan records what the response says: usage, response id and model,
// finish reasons and, for a stream, the time to the first chunk.
func (ec *PolicyExecutionContext) finishInferenceSpan(span trace.Span) {
	if ec.genai.streaming && !ec.genai.firstChunkAt.IsZero() {
		span.SetAttributes(attribute.Float64(genai.AttrResponseTTFC, ec.genai.firstChunkAt.Sub(ec.genai.startedAt).Seconds()))
	}
	if ec.genai.response == nil {
		return
	}
	if ec.genai.response.Truncated() || !ec.genai.responseComplete {
		span.SetAttributes(attribute.Bool(genai.AttrResponseTruncated, true))
	}
	body := ec.genai.response.Bytes()
	meta := genai.ParseResponse(ec.templateName(), body)
	if meta.ID != "" {
		span.SetAttributes(attribute.String(genai.AttrResponseID, meta.ID))
	}
	if meta.Model != "" {
		span.SetAttributes(attribute.String(genai.AttrResponseModel, meta.Model))
	}
	if len(meta.FinishReasons) > 0 {
		span.SetAttributes(attribute.StringSlice(genai.AttrResponseFinishReasons, meta.FinishReasons))
	}
	if ec.upstreamStatus() >= 400 {
		return // an error body carries no usage
	}
	// Called only now, after every response policy ran: Get memoises its result in the
	// shared context, so an earlier call would decide what llm-cost reads.
	usage, err := llmusage.Get(ec.sharedCtx, body, ec.genaiRequestBody(), ec.resourcePath())
	if err != nil || (usage.TotalInputTokens == 0 && usage.OutputTokens == 0) {
		return
	}
	span.SetAttributes(
		attribute.Int64(genai.AttrUsageInputTokens, usage.TotalInputTokens),
		attribute.Int64(genai.AttrUsageOutputTokens, usage.OutputTokens),
	)
	if usage.CachedReadTokens > 0 {
		span.SetAttributes(attribute.Int64(genai.AttrUsageCacheReadTokens, usage.CachedReadTokens))
	}
	if write := usage.CacheWriteTokens + usage.CacheWrite1hTokens; write > 0 {
		span.SetAttributes(attribute.Int64(genai.AttrUsageCacheWriteTokens, write))
	}
	if usage.ReasoningTokens > 0 {
		span.SetAttributes(attribute.Int64(genai.AttrUsageReasoningTokens, usage.ReasoningTokens))
	}
}
