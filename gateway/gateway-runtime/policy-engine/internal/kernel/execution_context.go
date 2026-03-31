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
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// maxStreamAccumulatorSize caps the amount of data accumulated before forcing
// a flush, preventing unbounded memory growth from large streaming bodies.
const maxStreamAccumulatorSize = 10 * 1024 * 1024 // 10 MB

// processingPhase identifies the ext_proc phase in which getModeOverride is called.
type processingPhase int

const (
	phaseRequestHeaders  processingPhase = iota
	phaseRequestBody
	phaseResponseHeaders
	phaseResponseBody
)

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

	// requestContentEncoding stores the Content-Encoding of the incoming request (e.g. "gzip", "br").
	// The body is decompressed before being passed to policies, and re-compressed using this value
	// before being forwarded to the upstream.
	requestContentEncoding string

	// responseContentEncoding stores the Content-Encoding of the upstream response (e.g. "gzip", "br").
	// The body is decompressed before being passed to policies, and re-compressed using this value
	// before being sent back to the downstream client.
	responseContentEncoding string

	// isStreamingRequest is set when SupportsRequestStreaming is true and the client
	// sends a streaming body — the request body will be processed chunk-by-chunk.
	isStreamingRequest       bool
	requestStreamAccumulator []byte
	requestStreamContext     *policy.RequestStreamContext
	// requestStreamDecomp performs per-chunk decompression for compressed streaming
	// request bodies. Nil when the request is not Content-Encoded.
	requestStreamDecomp *streamDecompressor

	// isStreamingResponse is set to true during response headers processing when
	// streaming indicators are detected AND the policy chain supports streaming.
	isStreamingResponse   bool
	streamAccumulator     []byte
	responseStreamContext *policy.ResponseStreamContext
	// responseStreamDecomp performs per-chunk decompression for compressed streaming
	// response bodies. Nil when the response is not Content-Encoded.
	responseStreamDecomp *streamDecompressor
	// streamTerminated is set when a policy returns TerminateStream=true. Any
	// subsequent upstream chunks that Envoy delivers after we have already sent
	// EndOfStream downstream are silently suppressed — the downstream connection
	// is already closed and forwarding more data would be undefined behaviour.
	streamTerminated bool

	// Reference to server components
	server *ExternalProcessorServer

	// phase tracks the current ext_proc processing phase and is read by getModeOverride.
	phase processingPhase
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
func (ec *PolicyExecutionContext) handlePolicyError(
	ctx context.Context,
	err error,
	phase string,
) *extprocv3.ProcessingResponse {
	errorID := uuid.New().String()

	slog.ErrorContext(ctx, "Policy execution failed",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"error", err,
	)

	errorBody := fmt.Sprintf(`{"error":"Internal Server Error","error_id":"%s"}`, errorID)

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
// Response body is always set to BUFFERED here (never FULL_DUPLEX_STREAMED).
// The upgrade to streaming happens at response-headers phase via
// getStreamingResponseModeOverride when a streaming upstream response is detected.
func (ec *PolicyExecutionContext) getModeOverride() *extprocconfigv3.ProcessingMode {
	mode := &extprocconfigv3.ProcessingMode{}

	if ec.policyChain.RequiresRequestBody {
		if ec.isStreamingRequest {
			mode.RequestBodyMode = extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED
		} else {
			mode.RequestBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
		}
	} else {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_NONE
	}

	if ec.policyChain.RequiresResponseBody {
		mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
		if ec.isStreamingResponse {
			mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED
			slog.Debug("[mode] upgraded response body mode to FULL_DUPLEX_STREAMED",
				"route", ec.routeKey,
			)
		}
	} else {
		mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_NONE
	}

	// At response-headers phase we know whether a body will follow. If not, skip it
	// even if the policy chain declared RequiresResponseBody, to avoid Envoy buffering
	// a body that will never arrive.
	if ec.phase == phaseResponseHeaders && ec.responseHasNoBody() {
		mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_NONE
	}

	// Always skip trailers (not used by policies)
	mode.RequestTrailerMode = extprocconfigv3.ProcessingMode_SKIP
	mode.ResponseTrailerMode = extprocconfigv3.ProcessingMode_SKIP

	slog.Debug("[mode] getModeOverride",
		"phase", ec.phase,
		"route", ec.routeKey,
		"requires_request_body", ec.policyChain.RequiresRequestBody,
		"requires_response_body", ec.policyChain.RequiresResponseBody,
		"supports_response_streaming", ec.policyChain.SupportsResponseStreaming,
		"is_streaming_request", ec.isStreamingRequest,
		"request_body_mode", mode.RequestBodyMode.String(),
		"response_header_mode", mode.ResponseHeaderMode.String(),
		"response_body_mode", mode.ResponseBodyMode.String(),
	)

	return mode
}

// ─── Phase processing methods ────────────────────────────────────────────────

// processRequestHeaders processes request headers phase.
// Header policies (OnRequestHeaders) always execute here regardless of whether
// body is required. Body policies (OnRequestBody) execute separately at body phase
// with headers already available in RequestContext.
//
// For bodyless requests (GET, Content-Length: 0, etc.) Envoy never sends a RequestBody
// phase. If the policy chain requires body processing, body policies are executed inline
// here with Body=nil so they still run on every request regardless of HTTP method.
func (ec *PolicyExecutionContext) processRequestHeaders(
	ctx context.Context,
) (*extprocv3.ProcessingResponse, error) {
	ec.phase = phaseRequestHeaders
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

	// For bodyless requests Envoy skips the RequestBody ext_proc phase entirely.
	// Execute body policies inline now so they run on every request, receiving a nil body.
	if !execResult.ShortCircuited && ec.policyChain.RequiresRequestBody && ec.requestHasNoBody() {
		return ec.processRequestBodyForEmptyRequest(ctx, execResult)
	}

	return TranslateRequestHeaderActions(execResult, ec.policyChain, ec)
}

// responseHasNoBody returns true when the response carries no body and Envoy will not
// deliver a ResponseBody ext_proc phase for this response.
// Note: this is called during the response-headers phase so responseBodyCtx is not yet populated.
func (ec *PolicyExecutionContext) responseHasNoBody() bool {
	// 1xx, 204, and 304 responses must not carry a body (RFC 9110).
	status := ec.responseHeaderCtx.ResponseStatus
	if status == 204 || status == 304 || (status >= 100 && status < 200) {
		return true
	}
	// Responses to HEAD requests have headers only — no body (RFC 9110).
	if strings.ToUpper(ec.requestHeaderCtx.Method) == "HEAD" {
		return true
	}
	// Content-Length: 0 explicitly signals an empty body.
	if cl := ec.responseHeaderCtx.ResponseHeaders.Get("content-length"); len(cl) > 0 && cl[0] == "0" {
		return true
	}
	return false
}

// requestHasNoBody returns true when the request carries no body and Envoy will not
// deliver a RequestBody ext_proc phase for this request.
func (ec *PolicyExecutionContext) requestHasNoBody() bool {
	// Envoy sets EndOfStream=true in the request-headers message when no body follows.
	if ec.requestBodyCtx.Body != nil && ec.requestBodyCtx.Body.EndOfStream {
		return true
	}
	// GET and HEAD requests must not carry a body (RFC 9110).
	switch strings.ToUpper(ec.requestHeaderCtx.Method) {
	case "GET", "HEAD":
		return true
	}
	// Content-Length: 0 explicitly signals an empty body.
	if cl := ec.requestHeaderCtx.Headers.Get("content-length"); len(cl) > 0 && cl[0] == "0" {
		return true
	}
	return false
}

// processRequestBodyForEmptyRequest executes body policies inline during the headers phase
// for requests that carry no body. The body context is set to Present=false / EndOfStream=true
// so policies can inspect headers-only state and short-circuit if necessary.
func (ec *PolicyExecutionContext) processRequestBodyForEmptyRequest(
	ctx context.Context,
	headerResult *executor.RequestHeaderExecutionResult,
) (*extprocv3.ProcessingResponse, error) {
	// Ensure the body context reflects a nil/absent body.
	if ec.requestBodyCtx.Body == nil {
		ec.requestBodyCtx.Body = &policy.Body{Content: nil, EndOfStream: true, Present: false}
	}

	slog.DebugContext(ctx, "[no-body] executing request body policies inline during headers phase",
		"route", ec.routeKey,
		"method", ec.requestHeaderCtx.Method,
	)

	bodyResult, err := ec.server.executor.ExecuteRequestPolicies(
		ctx,
		ec.policyChain.Policies,
		ec.requestBodyCtx,
		ec.policyChain.PolicySpecs,
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "request_body_no_body"), nil
	}

	return TranslateRequestHeaderActionsWithBodyMerge(headerResult, bodyResult, ec)
}

// processRequestBody processes request body phase
func (ec *PolicyExecutionContext) processRequestBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	ec.phase = phaseRequestBody
	if ec.isStreamingRequest {
		return ec.processStreamingRequestBody(ctx, body)
	}

	if ec.policyChain.RequiresRequestBody {
		// Decompress body if Content-Encoding was set, so policies receive plain bytes.
		bodyContent := body.Body
		if ec.requestContentEncoding != "" {
			decompressed, err := decompressBody(body.Body, ec.requestContentEncoding)
			if err != nil {
				slog.Warn("Failed to decompress request body, passing raw bytes to policies",
					"request_id", ec.requestID,
					"encoding", ec.requestContentEncoding,
					"error", err,
				)
				// Clear encoding so translator doesn't attempt to recompress raw compressed bytes
				ec.requestContentEncoding = ""
			} else {
				bodyContent = decompressed
			}
		}

		// Update request context with body data
		ec.requestBodyCtx.Body = &policy.Body{
			Content:     bodyContent,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		execResult, err := ec.server.executor.ExecuteRequestPolicies(
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

// processStreamingRequestBody handles streaming request body chunks
func (ec *PolicyExecutionContext) processStreamingRequestBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	chunk := &policy.StreamBody{
		Chunk:       body.Body,
		EndOfStream: body.EndOfStream,
	}

	// Compressed request: decompress this chunk, pass directly to policies,
	// recompress the output. No kernel accumulation — policy implementations
	// handle their own internal state across chunks.
	if ec.requestContentEncoding != "" {
		if ec.requestStreamDecomp == nil {
			ec.requestStreamDecomp = newStreamDecompressor(ec.requestContentEncoding)
		}
		decompressed, err := ec.requestStreamDecomp.FeedChunk(chunk.Chunk, chunk.EndOfStream)
		if err != nil {
			slog.Warn("[streaming] per-chunk request decompression error; disabling decompression",
				"request_id", ec.requestID,
				"encoding", ec.requestContentEncoding,
				"error", err,
			)
			ec.requestStreamDecomp.Close()
			ec.requestStreamDecomp = nil
			ec.requestContentEncoding = ""
		} else {
			chunk.Chunk = decompressed
		}

		// Suppress empty intermediate chunks.
		if len(chunk.Chunk) == 0 && !chunk.EndOfStream {
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

		slog.Debug("[streaming] request chunk decompressed",
			"route", ec.routeKey,
			"decompressed_bytes", len(chunk.Chunk),
			"end_of_stream", chunk.EndOfStream,
		)

		if chunk.EndOfStream {
			ec.requestBodyCtx.Body = &policy.Body{
				Content:     chunk.Chunk,
				EndOfStream: true,
				Present:     true,
			}
		}

		execResult, err := ec.server.executor.ExecuteStreamingRequestPolicies(
			ctx,
			ec.policyChain.Policies,
			ec.requestStreamContext,
			chunk,
			ec.policyChain.PolicySpecs,
			ec.sharedCtx.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "request_body_streaming"), nil
		}
		return TranslateStreamingRequestChunkAction(execResult, chunk, ec)
	}

	// Uncompressed (chunked): use the existing accumulation path.
	if len(chunk.Chunk) > 0 {
		ec.requestStreamAccumulator = append(ec.requestStreamAccumulator, chunk.Chunk...)
	}

	slog.Debug("[streaming] request chunk received",
		"route", ec.routeKey,
		"chunk_bytes", len(chunk.Chunk),
		"accumulated_bytes", len(ec.requestStreamAccumulator),
		"end_of_stream", chunk.EndOfStream,
	)

	shouldForceFlush := len(ec.requestStreamAccumulator) >= maxStreamAccumulatorSize
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

	// Populate requestBodyCtx.Body on the EOS flush so that buildResponseContexts
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
		ec.requestStreamAccumulator = nil
		return ec.handlePolicyError(ctx, err, "request_body_streaming"), nil
	}

	return TranslateStreamingRequestChunkAction(execResult, flushChunk, ec)
}

// processResponseHeaders processes response headers phase.
// Header policies (OnResponseHeaders) always execute here regardless of whether
// body is required. Body policies (OnResponseBody) execute separately at body phase.
func (ec *PolicyExecutionContext) processResponseHeaders(
	ctx context.Context,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, error) {
	ec.phase = phaseResponseHeaders
	ec.buildResponseContexts(headers)

	// Detect streaming response: upgrade when chain supports streaming AND
	// upstream signals chunked/SSE AND body is coming (not EndOfStream).
	hasStreamingHeaders := isStreamingUpstreamResponse(ec.responseHeaderCtx.ResponseHeaders)
	slog.Debug("[mode] response headers received — streaming detection",
		"route", ec.routeKey,
		"supports_response_streaming", ec.policyChain.SupportsResponseStreaming,
		"headers_end_of_stream", headers.EndOfStream,
		"streaming_headers_detected", hasStreamingHeaders,
		"content_type", ec.responseHeaderCtx.ResponseHeaders.Get("content-type"),
		"transfer_encoding", ec.responseHeaderCtx.ResponseHeaders.Get("transfer-encoding"),
	)
	if ec.policyChain.SupportsResponseStreaming && !headers.EndOfStream && hasStreamingHeaders {
		ec.isStreamingResponse = true
	}
	slog.Debug("[mode] streaming response decision",
		"route", ec.routeKey,
		"is_streaming_response", ec.isStreamingResponse,
	)

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

	return resp, nil
}

// processResponseBody processes response body phase
func (ec *PolicyExecutionContext) processResponseBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	ec.phase = phaseResponseBody
	if ec.isStreamingResponse {
		slog.Debug("[body] routing to streaming response body handler",
			"route", ec.routeKey,
			"chunk_bytes", len(body.Body),
			"end_of_stream", body.EndOfStream,
		)
		return ec.processStreamingResponseBody(ctx, body)
	}
	slog.Debug("[body] routing to buffered response body handler",
		"route", ec.routeKey,
		"body_bytes", len(body.Body),
		"end_of_stream", body.EndOfStream,
	)

	if ec.policyChain.RequiresResponseBody {
		// Decompress body if Content-Encoding was set, so policies receive plain JSON.
		bodyContent := body.Body
		if ec.responseContentEncoding != "" {
			decompressed, err := decompressBody(body.Body, ec.responseContentEncoding)
			if err != nil {
				slog.Warn("Failed to decompress response body, passing raw bytes to policies",
					"request_id", ec.requestID,
					"encoding", ec.responseContentEncoding,
					"error", err,
				)
				// Clear encoding so translator doesn't attempt to recompress raw compressed bytes
				ec.responseContentEncoding = ""
			} else {
				bodyContent = decompressed
			}
		}

		// Update response context with body data
		ec.responseBodyCtx.ResponseBody = &policy.Body{
			Content:     bodyContent,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		execResult, err := ec.server.executor.ExecuteResponsePolicies(
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

// processStreamingResponseBody handles streaming response body chunks
func (ec *PolicyExecutionContext) processStreamingResponseBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	// A policy previously terminated the stream (TerminateStream=true). Envoy may
	// still deliver buffered upstream chunks after we have already sent EndOfStream
	// downstream — suppress them with an empty streamed response so we do not attempt
	// to write to a closed downstream connection.
	if ec.streamTerminated {
		slog.Warn("[streaming] received upstream chunk after stream was already terminated; suppressing",
			"route", ec.routeKey,
			"chunk_bytes", len(body.Body),
			"end_of_stream", body.EndOfStream,
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

	chunk := &policy.StreamBody{
		Chunk:       body.Body,
		EndOfStream: body.EndOfStream,
	}

	// Compressed response: decompress this chunk, pass directly to policies,
	// recompress the output. No kernel accumulation — policy implementations
	// handle their own internal state across chunks.
	if ec.responseContentEncoding != "" {
		if ec.responseStreamDecomp == nil {
			ec.responseStreamDecomp = newStreamDecompressor(ec.responseContentEncoding)
		}
		decompressed, err := ec.responseStreamDecomp.FeedChunk(chunk.Chunk, chunk.EndOfStream)
		if err != nil {
			slog.Warn("[streaming] per-chunk response decompression error; disabling decompression",
				"request_id", ec.requestID,
				"encoding", ec.responseContentEncoding,
				"error", err,
			)
			ec.responseStreamDecomp.Close()
			ec.responseStreamDecomp = nil
			ec.responseContentEncoding = ""
		} else {
			chunk.Chunk = decompressed
		}

		// Suppress empty intermediate chunks — the decoder needed more input to
		// produce a full block. The client already expects compressed data so
		// sending nothing is correct here.
		if len(chunk.Chunk) == 0 && !chunk.EndOfStream {
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

		slog.Debug("[streaming] response chunk decompressed",
			"route", ec.routeKey,
			"decompressed_bytes", len(chunk.Chunk),
			"end_of_stream", chunk.EndOfStream,
		)

		execResult, err := ec.server.executor.ExecuteStreamingResponsePolicies(
			ctx,
			ec.policyChain.Policies,
			ec.responseStreamContext,
			chunk,
			ec.policyChain.PolicySpecs,
			ec.sharedCtx.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "response_body_streaming"), nil
		}
		if execResult.StreamTerminated {
			ec.streamTerminated = true
		}
		return TranslateStreamingResponseChunkAction(execResult, chunk, ec)
	}

	// Uncompressed (SSE / plain chunked): use the existing accumulation path so
	// policies that need multiple chunks (e.g. waiting for a full SSE event) still work.
	if len(chunk.Chunk) > 0 {
		ec.streamAccumulator = append(ec.streamAccumulator, chunk.Chunk...)
	}

	slog.Debug("[streaming] response chunk received",
		"route", ec.routeKey,
		"chunk_bytes", len(chunk.Chunk),
		"accumulated_bytes", len(ec.streamAccumulator),
		"end_of_stream", chunk.EndOfStream,
	)

	shouldForceFlush := len(ec.streamAccumulator) >= maxStreamAccumulatorSize
	if shouldForceFlush {
		slog.Warn("[streaming] response accumulator size limit exceeded, forcing flush",
			"route", ec.routeKey,
			"accumulated_bytes", len(ec.streamAccumulator),
			"max_size", maxStreamAccumulatorSize,
		)
	}

	// Consult streaming policies to decide whether to flush now.
	if !chunk.EndOfStream && !shouldForceFlush && ec.anyPolicyNeedsMoreResponseData(ec.streamAccumulator) {
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
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		ec.streamAccumulator = nil
		// NOTE: Mid-stream error — response headers and any previously flushed chunks
		// are already committed to the downstream client. The ImmediateResponse
		// returned by handlePolicyError is silently ignored by Envoy in
		// FULL_DUPLEX_STREAMED mode; Envoy will abort the HTTP/2 stream with a
		// RESET_STREAM. The client sees an abrupt connection close, not a structured
		// HTTP error response. There is no recovery path once streaming has started.
		return ec.handlePolicyError(ctx, err, "response_body_streaming"), nil
	}

	if execResult.StreamTerminated {
		ec.streamTerminated = true
	}
	return TranslateStreamingResponseChunkAction(execResult, flushChunk, ec)
}

// ─── Context builders ────────────────────────────────────────────────────────

// buildRequestContexts converts Envoy request headers into per-phase context objects.
// Both requestHeaderCtx and requestBodyCtx are initialized here; requestBodyCtx.Body
// is populated later in processRequestBody when body data arrives.
func (ec *PolicyExecutionContext) buildRequestContexts(headers *extprocv3.HttpHeaders, routeMetadata RouteMetadata) {
	headersMap := make(map[string][]string)
	var path, method, authority, scheme, requestID string

	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			headersMap[key] = append(headersMap[key], value)

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
				if requestID == "" {
					requestID = value
				}
			case "content-encoding":
				ec.requestContentEncoding = value
			}
		}
	}

	if requestID == "" {
		requestID = uuid.New().String()
	}

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

	// Detect request streaming at context-build time while headers are available.
	// Only enable streaming when the client actually sends a streaming body
	// (chunked transfer encoding or SSE content type).
	// Compressed requests are allowed into the streaming path — the body is
	// decompressed before policies run and recompressed before forwarding to
	// the upstream, preserving the original Content-Encoding header.
	if ec.policyChain.SupportsRequestStreaming && isStreamingClientRequest(wrappedHeaders) {
		ec.isStreamingRequest = true
	}
}

// buildResponseContexts converts Envoy response headers and stored request state into
// per-phase response context objects. All three response contexts share the same
// ResponseHeaders instance so that mutations applied by header-phase policies are
// immediately visible to body-phase policies.
func (ec *PolicyExecutionContext) buildResponseContexts(headers *extprocv3.HttpHeaders) {
	responseHeadersMap := make(map[string][]string)
	var responseStatus int

	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			responseHeadersMap[key] = append(responseHeadersMap[key], value)

			switch key {
			case ":status":
				// Convert status string to int
				_, err := fmt.Sscanf(value, "%d", &responseStatus)
				if err != nil {
					slog.Warn("Failed to parse response status code",
						"request_id", ec.requestID,
						"status_value", value,
						"error", err,
					)
				}
			case "content-encoding":
				ec.responseContentEncoding = value
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

// ─── Helpers ─────────────────────────────────────────────────────────────────

// isStreamingClientRequest detects if the client request indicates a streaming
// body based on transfer-encoding: chunked or content-type: text/event-stream.
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

// isStreamingUpstreamResponse detects if the upstream response is a streaming
// response based on transfer-encoding: chunked or content-type: text/event-stream.
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
		for k, v := range mods.HeadersToSet {
			values[strings.ToLower(k)] = []string{v}
		}
		for _, k := range mods.HeadersToRemove {
			delete(values, strings.ToLower(k))
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
		for k, v := range mods.HeadersToSet {
			values[strings.ToLower(k)] = []string{v}
		}
		for _, k := range mods.HeadersToRemove {
			delete(values, strings.ToLower(k))
		}
	}
}

// anyPolicyNeedsMoreRequestData returns true if any streaming request policy that
// would actually execute (enabled and condition met) is not yet ready to process
// the accumulated bytes.
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
		if sp, ok := pol.(policy.StreamingRequestPolicy); ok {
			if sp.NeedsMoreRequestData(accumulated) {
				return true
			}
		}
	}
	return false
}

// anyPolicyNeedsMoreResponseData returns true if any streaming response policy that
// would actually execute (enabled and condition met) is not yet ready to process
// the accumulated bytes.
func (ec *PolicyExecutionContext) anyPolicyNeedsMoreResponseData(accumulated []byte) bool {
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
		if sp, ok := pol.(policy.StreamingResponsePolicy); ok {
			needs := sp.NeedsMoreResponseData(accumulated)
			slog.Debug("[streaming] NeedsMoreResponseData",
				"route", ec.routeKey,
				"policy", spec.Name,
				"accumulated_bytes", len(accumulated),
				"needs_more", needs,
			)
			if needs {
				return true
			}
		}
	}
	return false
}
