/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"fmt"
	"log/slog"
	"strings"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// maxStreamAccumulatorSize is the maximum allowed size for stream accumulators.
// When exceeded, the accumulated data is flushed regardless of policy readiness.
const maxStreamAccumulatorSize = 10 * 1024 * 1024 // 10MB default

// PolicyExecutionContext manages the lifecycle of a single request through the policy chain.
// This context is created when a request arrives and lives until the response is completed.
// It encapsulates all state needed for processing both request and response phases.
type PolicyExecutionContext struct {
	// Per-phase contexts — built lazily as each phase is processed.
	requestHeaderCtx  *policy.RequestHeaderContext
	requestBodyCtx    *policy.RequestContext
	responseHeaderCtx *policy.ResponseHeaderContext
	responseBodyCtx   *policy.ResponseContext

	// Shared context that spans the entire request/response lifecycle.
	// Pointed to by each per-phase context's SharedContext field.
	sharedCtx *policy.SharedContext

	// Policy chain for this request
	policyChain *registry.PolicyChain

	// Route key (metadata key) for this request
	routeKey string

	// Request ID for correlation
	requestID string

	// Analytics metadata to be shared across request and response phases.
	// Used internally to propagate analytics data between phases without
	// contaminating the policy-visible metadata map.
	analyticsMetadata map[string]interface{}

	// Dynamic metadata to be shared across request and response phases
	dynamicMetadata map[string]map[string]interface{}

	// Default upstream cluster for dynamic cluster routing.
	// Set from route metadata when the route uses cluster_header routing.
	defaultUpstreamCluster string

	// Upstream base path for the main upstream (e.g., /anything)
	upstreamBasePath string

	// API context path (e.g., /weather/v1.0).
	// Used for computing path transformations when UpstreamName changes the upstream.
	apiContext string

	// Maps upstream definition names to their URL paths.
	// Used when UpstreamName is set to compute the correct path transformation.
	upstreamDefinitionPaths map[string]string

	// isStreamingRequest is set when SupportsRequestStreaming is true — the request
	// body will be processed chunk-by-chunk via processStreamingRequestBody.
	isStreamingRequest bool

	// requestStreamAccumulator collects request body chunks while policies are
	// waiting for enough data. Flushed when all policies are ready or EOS.
	requestStreamAccumulator []byte

	// requestStreamContext is built once at request headers time and reused for
	// every streaming request chunk, avoiding repeated allocation per chunk.
	requestStreamContext *policy.RequestStreamContext

	// isStreamingResponse is set to true during response headers processing when
	// streaming indicators are detected (Transfer-Encoding: chunked or
	// Content-Type: text/event-stream) AND the policy chain supports streaming.
	isStreamingResponse bool

	// streamAccumulator collects response body chunks while ChunkBuffering policies
	// are waiting for enough data. Flushed when all policies are ready or EOS.
	streamAccumulator []byte

	// responseStreamContext is built once at response headers time and reused for
	// every streaming response chunk, avoiding repeated allocation per chunk.
	responseStreamContext *policy.ResponseStreamContext

	// Reference to server components
	server *ExternalProcessorServer
}

// newPolicyExecutionContext creates a new execution context for a request
func newPolicyExecutionContext(
	server *ExternalProcessorServer,
	routeKey string,
	chain *registry.PolicyChain,
) *PolicyExecutionContext {
	return &PolicyExecutionContext{
		server:            server,
		routeKey:          routeKey,
		policyChain:       chain,
		analyticsMetadata: make(map[string]interface{}),
		dynamicMetadata:   make(map[string]map[string]interface{}),
	}
}

// handlePolicyError creates a generic error response for policy execution failures.
// It logs the full error details internally while returning only a generic message to the client.
// This prevents information disclosure of internal policy configuration and implementation details.
func (ec *PolicyExecutionContext) handlePolicyError(
	ctx context.Context,
	err error,
	phase string,
) *extprocv3.ProcessingResponse {
	// Generate unique error ID for correlation between client response and server logs
	errorID := uuid.New().String()

	// Log full error details with structured logging for troubleshooting
	slog.ErrorContext(ctx, "Policy execution failed",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"error", err,
	)

	// Build generic error response body (JSON format)
	errorBody := fmt.Sprintf(`{"error":"Internal Server Error","error_id":"%s"}`, errorID)

	// Return generic 500 response with correlation ID
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &typev3.HttpStatus{
					Code: typev3.StatusCode_InternalServerError,
				},
				Headers: buildHeaderValueOptions(map[string]string{
					"content-type": "application/json",
					"x-error-id":   errorID,
				}),
				Body: []byte(errorBody),
			},
		},
	}
}

// getModeOverride returns the ProcessingMode override for this execution context.
func (ec *PolicyExecutionContext) getModeOverride() *extprocconfigv3.ProcessingMode {
	mode := &extprocconfigv3.ProcessingMode{}

	switch {
	case !ec.policyChain.RequiresRequestBody:
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_NONE
	case ec.isStreamingRequest:
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED
	default:
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
	}

	mode.ResponseHeaderMode = extprocconfigv3.ProcessingMode_SEND

	if ec.policyChain.RequiresResponseBody {
		mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
	} else {
		mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_NONE
	}

	// Always skip trailers (not used by policies)
	mode.RequestTrailerMode = extprocconfigv3.ProcessingMode_SKIP
	mode.ResponseTrailerMode = extprocconfigv3.ProcessingMode_SKIP

	return mode
}

// getStreamingResponseModeOverride returns a ModeOverride that upgrades the response
// body processing to FULL_DUPLEX_STREAMED.
func (ec *PolicyExecutionContext) getStreamingResponseModeOverride() *extprocconfigv3.ProcessingMode {
	return &extprocconfigv3.ProcessingMode{
		ResponseBodyMode:    extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED,
		RequestTrailerMode:  extprocconfigv3.ProcessingMode_SKIP,
		ResponseTrailerMode: extprocconfigv3.ProcessingMode_SKIP,
	}
}

// isStreamingClientRequest reports whether the client request headers indicate a
// streaming body. Detection covers chunked transfer encoding and SSE content type.
func isStreamingClientRequest(headers *policy.Headers) bool {
	if teValues := headers.Get("transfer-encoding"); len(teValues) > 0 {
		if strings.Contains(strings.ToLower(teValues[0]), "chunked") {
			return true
		}
	}
	if ctValues := headers.Get("content-type"); len(ctValues) > 0 {
		if strings.HasPrefix(strings.ToLower(ctValues[0]), "text/event-stream") {
			return true
		}
	}
	return false
}

// isStreamingUpstreamResponse reports whether the upstream response headers indicate
// a streaming body. Detection covers chunked transfer encoding and SSE content type.
func isStreamingUpstreamResponse(headers *policy.Headers) bool {
	if teValues := headers.Get("transfer-encoding"); len(teValues) > 0 {
		if strings.Contains(strings.ToLower(teValues[0]), "chunked") {
			return true
		}
	}
	if ctValues := headers.Get("content-type"); len(ctValues) > 0 {
		if strings.HasPrefix(strings.ToLower(ctValues[0]), "text/event-stream") {
			return true
		}
	}
	return false
}

// processRequestHeaders processes request headers phase.
// Header policies (OnRequestHeaders) are always executed here regardless of whether
// body is required. Body policies (OnRequestBody) execute separately at body phase
// with headers already available in RequestContext.
func (ec *PolicyExecutionContext) processRequestHeaders(
	ctx context.Context,
) (*extprocv3.ProcessingResponse, error) {
	execResult, err := ec.server.executor.ExecuteRequestHeaderPolicies(
		ctx,
		ec.policyChain.Policies,
		ec.requestHeaderCtx,
		ec.policyChain.PolicySpecs,
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "request_headers"), nil
	}

	// Propagate header mutations into the shared in-memory context so that body-phase
	// policies (OnRequestBody / OnRequestBodyChunk) observe the post-mutation headers.
	if !execResult.ShortCircuited {
		applyRequestHeaderMutations(ec.requestHeaderCtx.Headers, execResult.Results)
		ec.syncRequestPseudoHeaders()
	}

	return TranslateRequestHeaderActions(execResult, ec.policyChain, ec)
}

// processRequestBody processes request body phase
func (ec *PolicyExecutionContext) processRequestBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	if ec.isStreamingRequest {
		return ec.processStreamingRequestBody(ctx, body)
	}

	if ec.policyChain.RequiresRequestBody {
		// Update the request body context with incoming body data
		ec.requestBodyCtx.Body = &policy.Body{
			Content:     body.Body,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		execResult, err := ec.server.executor.ExecuteRequestBodyPolicies(
			ctx,
			ec.policyChain.Policies,
			ec.requestBodyCtx,
			ec.policyChain.PolicySpecs,
			ec.sharedCtx.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "request_body"), nil
		}

		return TranslateRequestBodyActions(execResult, ec.policyChain, ec)
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// processResponseHeaders processes response headers phase.
func (ec *PolicyExecutionContext) processResponseHeaders(
	ctx context.Context,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, error) {
	ec.buildResponseContexts(headers)

	if ec.policyChain.SupportsResponseStreaming && !headers.EndOfStream &&
		isStreamingUpstreamResponse(ec.responseHeaderCtx.ResponseHeaders) {
		ec.isStreamingResponse = true
	}

	// Response header policies (OnResponseHeaders) always execute here.
	// Body policies (OnResponseBody) execute separately at body phase with
	// response headers already available in ResponseContext.
	execResult, err := ec.server.executor.ExecuteResponseHeaderPolicies(
		ctx,
		ec.policyChain.Policies,
		ec.responseHeaderCtx,
		ec.policyChain.PolicySpecs,
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "response_headers"), nil
	}

	// Propagate header mutations into the shared in-memory context so that body-phase
	// policies (OnResponseBody / OnResponseBodyChunk) observe the post-mutation headers.
	applyResponseHeaderMutations(ec.responseHeaderCtx.ResponseHeaders, execResult.Results)

	resp, err := TranslateResponseHeaderActions(execResult, ec)
	if err != nil {
		return nil, err
	}

	if ec.isStreamingResponse {
		resp.ModeOverride = ec.getStreamingResponseModeOverride()
	}

	return resp, nil
}

// processResponseBody processes response body phase.
func (ec *PolicyExecutionContext) processResponseBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	if ec.isStreamingResponse {
		return ec.processStreamingResponseBody(ctx, body)
	}

	if ec.policyChain.RequiresResponseBody {
		ec.responseBodyCtx.ResponseBody = &policy.Body{
			Content:     body.Body,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		execResult, err := ec.server.executor.ExecuteResponseBodyPolicies(
			ctx,
			ec.policyChain.Policies,
			ec.responseBodyCtx,
			ec.policyChain.PolicySpecs,
			ec.sharedCtx.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "response_body"), nil
		}

		return TranslateResponseBodyActions(execResult, ec)
	}

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// processStreamingResponseBody handles a single response body chunk during streaming.
func (ec *PolicyExecutionContext) processStreamingResponseBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	chunk := &policy.StreamBody{
		Chunk:       body.Body,
		EndOfStream: body.EndOfStream,
	}

	if len(chunk.Chunk) > 0 {
		ec.streamAccumulator = append(ec.streamAccumulator, chunk.Chunk...)
	}

	slog.Debug("[streaming] response chunk received",
		"route", ec.routeKey,
		"chunk_bytes", len(chunk.Chunk),
		"accumulated_bytes", len(ec.streamAccumulator),
		"end_of_stream", chunk.EndOfStream,
	)

	// Check if accumulator has grown too large and force flush to prevent unbounded memory growth
	shouldForceFlush := len(ec.streamAccumulator) > maxStreamAccumulatorSize

	if shouldForceFlush {
		slog.Warn("[streaming] response accumulator size limit exceeded, forcing flush",
			"route", ec.routeKey,
			"accumulated_bytes", len(ec.streamAccumulator),
			"max_size", maxStreamAccumulatorSize,
		)
	}

	// Consult ChunkBuffering policies to decide whether to flush now.
	// In FULL_DUPLEX_STREAMED mode an empty BodyResponse passes the chunk through unchanged,
	// so we must explicitly suppress it with an empty StreamedBodyResponse while accumulating.
	if !chunk.EndOfStream && !shouldForceFlush && ec.anyPolicyNeedsMoreData(ec.streamAccumulator) {
		slog.Debug("[streaming] accumulating — waiting for more response data",
			"route", ec.routeKey,
			"accumulated_bytes", len(ec.streamAccumulator),
		)
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseBody{
				ResponseBody: &extprocv3.BodyResponse{
					Response: &extprocv3.CommonResponse{
						BodyMutation: &extprocv3.BodyMutation{
							Mutation: &extprocv3.BodyMutation_StreamedResponse{
								StreamedResponse: &extprocv3.StreamedBodyResponse{},
							},
						},
					},
				},
			},
		}, nil
	}

	flushChunk := &policy.StreamBody{
		Chunk:       ec.streamAccumulator,
		EndOfStream: chunk.EndOfStream,
	}
	slog.Debug("[streaming] flushing accumulated response data to policies",
		"route", ec.routeKey,
		"flush_bytes", len(flushChunk.Chunk),
		"end_of_stream", flushChunk.EndOfStream,
	)
	ec.streamAccumulator = nil

	execResult, err := ec.server.executor.ExecuteStreamingResponsePolicies(
		ctx,
		ec.policyChain.Policies,
		ec.responseStreamContext,
		flushChunk,
		ec.policyChain.PolicySpecs,
		ec.responseStreamContext.SharedContext.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "response_body_streaming"), nil
	}

	return TranslateStreamingResponseChunkAction(execResult, flushChunk, ec)
}

// processStreamingRequestBody handles a single request body chunk during streaming.
func (ec *PolicyExecutionContext) processStreamingRequestBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	chunk := &policy.StreamBody{
		Chunk:       body.Body,
		EndOfStream: body.EndOfStream,
	}

	if len(chunk.Chunk) > 0 {
		ec.requestStreamAccumulator = append(ec.requestStreamAccumulator, chunk.Chunk...)
	}

	slog.Debug("[streaming] request chunk received",
		"route", ec.routeKey,
		"chunk_bytes", len(chunk.Chunk),
		"accumulated_bytes", len(ec.requestStreamAccumulator),
		"end_of_stream", chunk.EndOfStream,
	)

	// Check if accumulator has grown too large and force flush to prevent unbounded memory growth
	shouldForceFlush := len(ec.requestStreamAccumulator) > maxStreamAccumulatorSize

	if shouldForceFlush {
		slog.Warn("[streaming] request accumulator size limit exceeded, forcing flush",
			"route", ec.routeKey,
			"accumulated_bytes", len(ec.requestStreamAccumulator),
			"max_size", maxStreamAccumulatorSize,
		)
	}

	// Consult streaming policies to decide whether to flush now.
	// In FULL_DUPLEX_STREAMED mode an empty BodyResponse passes the chunk through unchanged,
	// so we must explicitly suppress it with an empty StreamedBodyResponse while accumulating.
	if !chunk.EndOfStream && !shouldForceFlush && ec.anyPolicyNeedsMoreRequestData(ec.requestStreamAccumulator) {
		slog.Debug("[streaming] accumulating — waiting for more request data",
			"route", ec.routeKey,
			"accumulated_bytes", len(ec.requestStreamAccumulator),
		)
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestBody{
				RequestBody: &extprocv3.BodyResponse{
					Response: &extprocv3.CommonResponse{
						BodyMutation: &extprocv3.BodyMutation{
							Mutation: &extprocv3.BodyMutation_StreamedResponse{
								StreamedResponse: &extprocv3.StreamedBodyResponse{},
							},
						},
					},
				},
			},
		}, nil
	}

	flushChunk := &policy.StreamBody{
		Chunk:       ec.requestStreamAccumulator,
		EndOfStream: chunk.EndOfStream,
	}
	slog.Debug("[streaming] flushing accumulated request data to policies",
		"route", ec.routeKey,
		"flush_bytes", len(flushChunk.Chunk),
		"end_of_stream", flushChunk.EndOfStream,
	)
	ec.requestStreamAccumulator = nil

	// Populate ec.requestBodyCtx.Body on the EOS flush so that buildResponseContexts
	// (called during processResponseHeaders) exposes the accumulated request payload
	// to response-phase policies via ResponseHeaderContext/ResponseContext/ResponseStreamContext.
	// In non-streaming mode processRequestBody always sets this field; the streaming
	// path must do the same so response phases never see a nil RequestBody.
	if flushChunk.EndOfStream {
		ec.requestBodyCtx.Body = &policy.Body{
			Content:     flushChunk.Chunk,
			EndOfStream: true,
			Present:     true,
		}
	}

	execResult, err := ec.server.executor.ExecuteStreamingRequestPolicies(
		ctx,
		ec.policyChain.Policies,
		ec.requestStreamContext,
		flushChunk,
		ec.policyChain.PolicySpecs,
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "request_body_streaming"), nil
	}

	return TranslateStreamingRequestChunkAction(execResult, flushChunk, ec)
}

// anyPolicyNeedsMoreRequestData returns true if any StreamingRequestPolicy that would
// actually execute (enabled and condition met) is not yet ready to process the accumulated bytes.
func (ec *PolicyExecutionContext) anyPolicyNeedsMoreRequestData(accumulated []byte) bool {
	specs := ec.policyChain.PolicySpecs
	celEval := ec.server.executor.GetCELEvaluator()
	for i, pol := range ec.policyChain.Policies {
		spec := specs[i]
		if !spec.Enabled {
			continue
		}
		if ec.policyChain.HasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if celEval != nil {
				conditionMet, err := celEval.EvaluateStreamingRequestCondition(*spec.ExecutionCondition, ec.requestStreamContext)
				if err == nil && !conditionMet {
					continue
				}
				// On error: fall through and treat as condition met (conservative)
			}
		}
		if streamPol, ok := pol.(policy.StreamingRequestPolicy); ok {
			if streamPol.NeedsMoreRequestData(accumulated) {
				return true
			}
		}
	}
	return false
}

// anyPolicyNeedsMoreData returns true if any StreamingResponsePolicy that would
// actually execute (enabled and condition met) is not yet ready to process the accumulated bytes.
func (ec *PolicyExecutionContext) anyPolicyNeedsMoreData(accumulated []byte) bool {
	specs := ec.policyChain.PolicySpecs
	celEval := ec.server.executor.GetCELEvaluator()
	for i, pol := range ec.policyChain.Policies {
		spec := specs[i]
		if !spec.Enabled {
			continue
		}
		if ec.policyChain.HasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if celEval != nil {
				conditionMet, err := celEval.EvaluateStreamingResponseCondition(*spec.ExecutionCondition, ec.responseStreamContext)
				if err == nil && !conditionMet {
					continue
				}
				// On error: fall through and treat as condition met (conservative)
			}
		}
		if streamPol, ok := pol.(policy.StreamingResponsePolicy); ok {
			if streamPol.NeedsMoreResponseData(accumulated) {
				return true
			}
		}
	}
	return false
}

// buildRequestContexts converts Envoy request headers into per-phase context objects.
// Both requestHeaderCtx and requestBodyCtx are initialized here; requestBodyCtx.Body
// is populated later in processRequestBody when body data arrives.
func (ec *PolicyExecutionContext) buildRequestContexts(headers *extprocv3.HttpHeaders, routeMetadata RouteMetadata) {
	headersMap := make(map[string][]string)

	// Variables to store pseudo-headers and request ID
	var path, method, authority, scheme, requestID string

	// Extract headers, pseudo-headers, and request ID in a single loop
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			headersMap[key] = append(headersMap[key], value)

			// Extract pseudo-headers and request ID
			switch key {
			case ":path":
				path = value
			case ":method":
				method = value
			case ":authority":
				authority = value
			case ":scheme":
				scheme = value
			case "x-request-id":
				if requestID == "" { // Take first occurrence
					requestID = value
				}
			}
		}
	}

	// Generate request ID if not present in headers
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Create shared context that will persist across request/response phases
	sharedCtx := &policy.SharedContext{
		RequestID:     requestID,
		ProjectID:     routeMetadata.ProjectID,
		APIId:         routeMetadata.APIId,
		APIName:       routeMetadata.APIName,
		APIVersion:    routeMetadata.APIVersion,
		APIKind:       routeMetadata.APIKind,
		APIContext:    routeMetadata.Context,
		OperationPath: routeMetadata.OperationPath,
		Metadata:      make(map[string]interface{}),
	}
	if routeMetadata.TemplateHandle != "" {
		sharedCtx.Metadata["template_handle"] = routeMetadata.TemplateHandle
	}
	if routeMetadata.ProviderName != "" {
		sharedCtx.Metadata["provider_name"] = routeMetadata.ProviderName
	}

	ec.sharedCtx = sharedCtx
	ec.requestID = requestID

	wrappedHeaders := policy.NewHeaders(headersMap)

	ec.requestHeaderCtx = &policy.RequestHeaderContext{
		SharedContext: sharedCtx,
		Headers:       wrappedHeaders,
		Path:          path,
		Method:        method,
		Authority:     authority,
		Scheme:        scheme,
		Vhost:         routeMetadata.Vhost,
	}

	// requestBodyCtx shares the same shared context and headers; Body is set later.
	var bodyEOS *policy.Body
	if headers.EndOfStream {
		bodyEOS = &policy.Body{EndOfStream: true}
	}
	ec.requestBodyCtx = &policy.RequestContext{
		SharedContext: sharedCtx,
		Headers:       wrappedHeaders,
		Body:          bodyEOS,
		Path:          path,
		Method:        method,
		Authority:     authority,
		Scheme:        scheme,
		Vhost:         routeMetadata.Vhost,
	}

	// Build the streaming context once; reused across all chunks for this request.
	ec.requestStreamContext = &policy.RequestStreamContext{
		SharedContext: sharedCtx,
		Headers:       wrappedHeaders,
		Path:          path,
		Method:        method,
		Authority:     authority,
		Scheme:        scheme,
		Vhost:         routeMetadata.Vhost,
	}

	if ec.policyChain.SupportsRequestStreaming && isStreamingClientRequest(wrappedHeaders) {
		ec.isStreamingRequest = true
	}
}

// buildResponseContexts converts Envoy response headers and stored request state into
// per-phase response context objects.
func (ec *PolicyExecutionContext) buildResponseContexts(headers *extprocv3.HttpHeaders) {
	responseHeadersMap := make(map[string][]string)
	var responseStatus int

	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			responseHeadersMap[key] = append(responseHeadersMap[key], value)

			if key == ":status" {
				_, err := fmt.Sscanf(value, "%d", &responseStatus)
				if err != nil {
					slog.Warn("Failed to parse response status code",
						"request_id", ec.requestID,
						"status_value", value,
						"error", err,
					)
				}
			}
		}
	}

	responseHeaders := policy.NewHeaders(responseHeadersMap)

	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		RequestBody:     ec.requestBodyCtx.Body,
		RequestPath:     ec.requestHeaderCtx.Path,
		RequestMethod:   ec.requestHeaderCtx.Method,
		ResponseHeaders: responseHeaders,
		ResponseStatus:  responseStatus,
	}

	var responseBodyEOS *policy.Body
	if headers.EndOfStream {
		responseBodyEOS = &policy.Body{EndOfStream: true}
	}
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		RequestBody:     ec.requestBodyCtx.Body,
		RequestPath:     ec.requestHeaderCtx.Path,
		RequestMethod:   ec.requestHeaderCtx.Method,
		ResponseHeaders: responseHeaders,
		ResponseBody:    responseBodyEOS,
		ResponseStatus:  responseStatus,
	}

	// Build the streaming context once; reused across all chunks for this response.
	ec.responseStreamContext = &policy.ResponseStreamContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		RequestBody:     ec.requestBodyCtx.Body,
		RequestPath:     ec.requestHeaderCtx.Path,
		RequestMethod:   ec.requestHeaderCtx.Method,
		ResponseHeaders: responseHeaders,
		ResponseStatus:  responseStatus,
	}
}

// applyRequestHeaderMutations applies RequestHeaderAction mutations from all policy
// results into the shared in-memory Headers object so that body-phase policies see
// the post-mutation state of the request headers.
//
// requestHeaderCtx, requestBodyCtx, and requestStreamContext all point to the same
// *Headers instance, so one in-place update covers all three.
func applyRequestHeaderMutations(headers *policy.Headers, results []executor.RequestHeaderPolicyResult) {
	values := headers.UnsafeInternalValues()
	for _, pr := range results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.Set {
			values[strings.ToLower(k)] = []string{v}
		}
		for _, k := range mods.Remove {
			delete(values, strings.ToLower(k))
		}
		for k, vs := range mods.Append {
			lk := strings.ToLower(k)
			values[lk] = append(values[lk], vs...)
		}
	}
}

// syncRequestPseudoHeaders reads :path, :method, :authority, and :scheme from the
// shared request Headers (which may have been mutated by header-phase policies) and
// writes the updated values back into the explicit fields of requestHeaderCtx,
// requestBodyCtx, and requestStreamContext. This keeps the separate struct fields
// in sync with the Headers map so that body/stream-phase policies observe a
// consistent view of the request.
func (ec *PolicyExecutionContext) syncRequestPseudoHeaders() {
	values := ec.requestHeaderCtx.Headers.UnsafeInternalValues()
	if v := values[":path"]; len(v) > 0 {
		ec.requestHeaderCtx.Path = v[0]
		ec.requestBodyCtx.Path = v[0]
		ec.requestStreamContext.Path = v[0]
	}
	if v := values[":method"]; len(v) > 0 {
		ec.requestHeaderCtx.Method = v[0]
		ec.requestBodyCtx.Method = v[0]
		ec.requestStreamContext.Method = v[0]
	}
	if v := values[":authority"]; len(v) > 0 {
		ec.requestHeaderCtx.Authority = v[0]
		ec.requestBodyCtx.Authority = v[0]
		ec.requestStreamContext.Authority = v[0]
	}
	if v := values[":scheme"]; len(v) > 0 {
		ec.requestHeaderCtx.Scheme = v[0]
		ec.requestBodyCtx.Scheme = v[0]
		ec.requestStreamContext.Scheme = v[0]
	}
}

// applyResponseHeaderMutations applies ResponseHeaderAction mutations from all policy
// results into the shared in-memory Headers object so that body-phase policies see
// the post-mutation state of the response headers.
//
// responseHeaderCtx, responseBodyCtx, and responseStreamContext all point to the same
// *Headers instance, so one in-place update covers all three.
func applyResponseHeaderMutations(headers *policy.Headers, results []executor.ResponseHeaderPolicyResult) {
	values := headers.UnsafeInternalValues()
	for _, pr := range results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.DownstreamResponseHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.Set {
			values[strings.ToLower(k)] = []string{v}
		}
		for _, k := range mods.Remove {
			delete(values, strings.ToLower(k))
		}
		for k, vs := range mods.Append {
			lk := strings.ToLower(k)
			values[lk] = append(values[lk], vs...)
		}
	}
}
