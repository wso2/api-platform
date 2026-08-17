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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// maxStreamAccumulatorSize caps the amount of data accumulated before forcing
// a flush, preventing unbounded memory growth from large streaming bodies.
const maxStreamAccumulatorSize = 10 * 1024 * 1024 // 10 MB

// processingPhase identifies the ext_proc phase in which getModeOverride is called.
type processingPhase int

const (
	phaseRequestHeaders processingPhase = iota
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

	// downstreamHeaders is a snapshot of the client request headers, captured at
	// buildRequestContexts before any policy mutation.
	// Exposed to policies via Request*Context.Downstream and, on the response
	// path, via Response*Context.Downstream.
	downstreamHeaders *policy.Headers

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

	// defaultUpstream is this route's own compiled-in upstream (cluster name, URL, base
	// path) — whichever slot it belongs to. Always present; surfaced to policies via the
	// "current_upstream" dynamic metadata object when no dynamic override is in effect.
	defaultUpstream *policyenginev1.UpstreamInfo

	// requestContentEncoding stores the Content-Encoding of the incoming request (e.g. "gzip", "br").
	// The body is decompressed before being passed to policies, and re-compressed using this value
	// before being forwarded to the upstream.
	requestContentEncoding string

	// responseContentEncoding stores the Content-Encoding of the upstream response (e.g. "gzip", "br").
	// The body is decompressed before being passed to policies, and re-compressed using this value
	// before being sent back to the downstream client.
	responseContentEncoding string

	// requestHeadersEndOfStream / responseHeadersEndOfStream record Envoy's
	// EndOfStream flag from the request/response headers message. This is the
	// authoritative framing signal for "does a body follow": Envoy sets it from
	// what is actually on the wire, whereas method/status/Content-Length are only
	// what the peer *should* have sent. A GET carrying a body (RFC 9110 permits it
	// syntactically) arrives with EndOfStream=false and Envoy does deliver a body
	// phase for it — inferring "bodyless" from the method alone would skip the
	// encoding guard and hand those bytes to policies unchecked.
	requestHeadersEndOfStream  bool
	responseHeadersEndOfStream bool

	// requestEncodingUnsupported / responseEncodingUnsupported are set when a
	// non-identity Content-Encoding arrives that the kernel can neither decompress
	// nor re-compress. When the policy chain requires that body, the message is
	// rejected at the header phase rather than forwarded: passing it through would
	// let an unreadable encoding silently skip every body policy — on the request
	// side that is a caller-chosen header disabling guardrails and content
	// moderation, and on the response side it is a masking/redaction policy that
	// never runs on data already on its way to the client.
	requestEncodingUnsupported  bool
	responseEncodingUnsupported bool

	// isStreamingRequest is set when SupportsRequestStreaming is true and the client
	// sends a streaming body — the request body will be processed chunk-by-chunk.
	isStreamingRequest       bool
	requestStreamAccumulator []byte
	requestStreamContext     *policy.RequestStreamContext
	// requestStreamDecomp performs per-chunk decompression for compressed streaming
	// request bodies. Nil when the request is not Content-Encoded.
	requestStreamDecomp *streamDecompressor
	// requestDeflatePending holds the leading compressed bytes of a "deflate"
	// request body until enough have arrived to tell the two wire variants apart
	// (see deflateVariantProbeBytes). Always empty once requestStreamDecomp exists.
	requestDeflatePending []byte
	// requestStreamComp re-compresses streaming request chunks into a SINGLE
	// compressed stream for the whole request, for the same reason
	// responseStreamComp does downstream: one writer per chunk would emit N
	// independent members and the upstream would read only the first.
	requestStreamComp *streamCompressor

	// isStreamingResponse is set to true during response headers processing when
	// streaming indicators are detected AND the policy chain supports streaming.
	isStreamingResponse   bool
	streamAccumulator     []byte
	responseStreamContext *policy.ResponseStreamContext
	// responseStreamDecomp performs per-chunk decompression for compressed streaming
	// response bodies. Nil when the response is not Content-Encoded.
	responseStreamDecomp *streamDecompressor
	// responseDeflatePending is the response-side counterpart of
	// requestDeflatePending.
	responseDeflatePending []byte
	// responseStreamComp re-compresses streaming response chunks into a SINGLE
	// compressed stream for the whole response. It must outlive individual chunks:
	// compressing each chunk independently yields one gzip member (or brotli
	// stream) per chunk, and clients stop reading after the first one — the body
	// then looks truncated to the client even though every byte was sent.
	responseStreamComp *streamCompressor
	// streamTerminated is set when a policy returns TerminateStream=true. Any
	// subsequent upstream chunks that Envoy delivers after EndOfStream was sent
	// downstream are silently suppressed — forwarding more data would be undefined.
	streamTerminated bool

	// Reference to server components
	server *ExternalProcessorServer

	// phase tracks the current ext_proc processing phase and is read by getModeOverride.
	phase processingPhase

	// terminal is the last known terminal HTTP outcome for this request, memoized
	// by resolveTerminalOutcome so Process's root-span defer can stamp it after
	// the final phase. Only non-zero outcomes overwrite it.
	terminal tracing.HTTPOutcome

	// generated describes the ImmediateResponse this execution context built
	// itself (handlePolicyError / handlePayloadTooLarge) rather than one a policy
	// returned. Matched by pointer identity in resolveTerminalOutcome so an
	// engine-generated fault is never mislabelled as a policy denial.
	generated generatedResponse

	// responseStatusOverridden is set by processResponseBody /
	// processResponseBodyForEmptyResponse when a response-body policy sets
	// DownstreamResponseModifications.StatusCode, so resolveTerminalOutcome can
	// distinguish a policy-chosen status from a genuine upstream pass-through
	// (constants.TerminalReasonPolicyStatusOverride vs TerminalReasonUpstream).
	responseStatusOverridden bool
}

// generatedResponse ties a policy-engine-generated ImmediateResponse to the span
// outcome that describes it.
type generatedResponse struct {
	resp    *extprocv3.ProcessingResponse
	outcome tracing.HTTPOutcome
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

// closeStreamDecompressors releases decoder goroutines when the ext_proc stream
// ends before an encoded body reaches EOS (client cancellation, send/receive
// failure, or a policy short-circuit).
func (ec *PolicyExecutionContext) closeStreamDecompressors() {
	if ec.requestStreamDecomp != nil {
		ec.requestStreamDecomp.Close()
		ec.requestStreamDecomp = nil
	}
	if ec.responseStreamDecomp != nil {
		ec.responseStreamDecomp.Close()
		ec.responseStreamDecomp = nil
	}
	if ec.requestStreamComp != nil {
		ec.requestStreamComp.Close()
		ec.requestStreamComp = nil
	}
	if ec.responseStreamComp != nil {
		ec.responseStreamComp.Close()
		ec.responseStreamComp = nil
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

	resp := &extprocv3.ProcessingResponse{
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

	// Tag this response as engine-generated so the span carries
	// reason=policy_error and the same correlation id the client and the error
	// log see.
	ec.generated = generatedResponse{
		resp: resp,
		outcome: tracing.HTTPOutcome{
			StatusCode: http.StatusInternalServerError,
			Reason:     constants.TerminalReasonPolicyError,
			ErrorID:    errorID,
		},
	}
	return resp
}

// handlePayloadTooLarge builds an HTTP 413 immediate response for a buffered
// request body that exceeded the decompression ceiling. The client payload stays
// generic (no limit value, no internals); specifics are logged under the
// correlation id. Streaming bodies fail the ext_proc stream closed instead,
// because an HTTP response cannot be guaranteed after full-duplex forwarding starts.
func (ec *PolicyExecutionContext) handlePayloadTooLarge(
	ctx context.Context,
	err error,
	phase string,
) *extprocv3.ProcessingResponse {
	errorID := uuid.New().String()

	slog.WarnContext(ctx, "Rejecting body: decompressed payload exceeds configured limit",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"error", err,
	)

	errorBody := fmt.Sprintf(`{"error":"Payload Too Large","error_id":"%s"}`, errorID)

	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &typev3.HttpStatus{
					Code: typev3.StatusCode_PayloadTooLarge,
				},
				Headers: buildHeaderValueOptions(map[string]string{
					"content-type": "application/json",
					"x-error-id":   errorID,
				}),
				Body: []byte(errorBody),
			},
		},
	}

	// Tag this response as engine-generated so the span carries
	// reason=payload_too_large and the same correlation id the client and the
	// warning log see.
	ec.generated = generatedResponse{
		resp: resp,
		outcome: tracing.HTTPOutcome{
			StatusCode: http.StatusRequestEntityTooLarge,
			Reason:     constants.TerminalReasonPayloadTooLarge,
			ErrorID:    errorID,
		},
	}
	return resp
}

// rejectUnsupportedEncoding builds an immediate response for a message whose
// Content-Encoding the kernel cannot round-trip, on a body the policy chain
// requires.
//
// This fails closed by design. The alternative — forward the body untouched —
// means every body policy on the route silently does not run while the message
// still completes with a success status. On the request side the encoding is
// chosen by the caller, so that would be a bypass primitive: send
// "Content-Encoding: <anything unrecognised>" and guardrails, content moderation
// and schema validation all stop applying to the payload. On the response side
// it means a masking or redaction policy never runs on data already on its way
// to the client. Neither is a decision the kernel can make on the operator's
// behalf, so the message is rejected instead.
//
// The client payload stays generic — no encoding name, no policy names, nothing
// about which side failed (per error-handling.md directive 1); specifics go to
// the log under the correlation id.
func (ec *PolicyExecutionContext) rejectUnsupportedEncoding(
	ctx context.Context,
	encoding []string,
	phase string,
	statusCode typev3.StatusCode,
	httpStatus int,
	clientError string,
) *extprocv3.ProcessingResponse {
	errorID := uuid.New().String()

	slog.WarnContext(ctx, "Rejecting message: Content-Encoding cannot be decoded, and body policies are attached to this route",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"encoding", encoding,
	)

	errorBody := fmt.Sprintf(`{"error":%q,"error_id":"%s"}`, clientError, errorID)

	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &typev3.HttpStatus{
					Code: statusCode,
				},
				Headers: buildHeaderValueOptions(map[string]string{
					"content-type": "application/json",
					"x-error-id":   errorID,
				}),
				Body: []byte(errorBody),
			},
		},
	}

	ec.generated = generatedResponse{
		resp: resp,
		outcome: tracing.HTTPOutcome{
			StatusCode: httpStatus,
			Reason:     constants.TerminalReasonUnsupportedEncoding,
			ErrorID:    errorID,
		},
	}
	return resp
}

// rejectUnsupportedRequestEncoding rejects an undecodable request body with 415.
// The caller picked the encoding, so this is a client error and is safe to
// surface as one — no part of the request has been forwarded upstream yet.
func (ec *PolicyExecutionContext) rejectUnsupportedRequestEncoding(
	ctx context.Context,
	phase string,
) *extprocv3.ProcessingResponse {
	return ec.rejectUnsupportedEncoding(
		ctx,
		ec.requestHeaderCtx.Headers.Get("content-encoding"),
		phase,
		typev3.StatusCode_UnsupportedMediaType,
		http.StatusUnsupportedMediaType,
		"Unsupported Media Type",
	)
}

// rejectUnsupportedResponseEncoding rejects an undecodable upstream response with
// 502. The client did nothing wrong — the upstream answered in a coding this
// gateway cannot inspect — and response headers have not been committed
// downstream yet at the point this runs, so a clean status is still possible.
func (ec *PolicyExecutionContext) rejectUnsupportedResponseEncoding(ctx context.Context) *extprocv3.ProcessingResponse {
	return ec.rejectUnsupportedEncoding(
		ctx,
		ec.responseHeaderCtx.ResponseHeaders.Get("content-encoding"),
		"response_headers",
		typev3.StatusCode_BadGateway,
		http.StatusBadGateway,
		"Bad Gateway",
	)
}

// rejectUndecodableBody builds an immediate response for a body that declared a
// supported Content-Encoding but failed to decode as it. The reasoning is the
// same as rejectUnsupportedEncoding: forwarding the raw bytes would leave every
// body policy on the route silently inapplicable while the message still
// succeeds. The decoder's error goes to the log only — it can describe stream
// internals, and the client learns nothing beyond the status.
func (ec *PolicyExecutionContext) rejectUndecodableBody(
	ctx context.Context,
	err error,
	phase string,
	statusCode typev3.StatusCode,
	httpStatus int,
	clientError string,
) *extprocv3.ProcessingResponse {
	errorID := uuid.New().String()

	slog.WarnContext(ctx, "Rejecting body: does not decode as its declared Content-Encoding",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"error", err,
	)

	errorBody := fmt.Sprintf(`{"error":%q,"error_id":"%s"}`, clientError, errorID)

	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &typev3.HttpStatus{
					Code: statusCode,
				},
				Headers: buildHeaderValueOptions(map[string]string{
					"content-type": "application/json",
					"x-error-id":   errorID,
				}),
				Body: []byte(errorBody),
			},
		},
	}

	ec.generated = generatedResponse{
		resp: resp,
		outcome: tracing.HTTPOutcome{
			StatusCode: httpStatus,
			Reason:     constants.TerminalReasonUnsupportedEncoding,
			ErrorID:    errorID,
		},
	}
	return resp
}

// resolveEncodingFromBody pins "deflate" to the concrete variant the peer
// actually sent, using the first bytes of the body. Every other encoding is
// self-describing and passes through unchanged.
//
// This runs at the first body bytes rather than at the header phase because the
// header alone cannot distinguish the two: both arrive as
// "Content-Encoding: deflate".
func resolveEncodingFromBody(encoding string, firstBytes []byte) string {
	if encoding != encodingDeflate {
		return encoding
	}
	return resolveDeflateVariant(firstBytes)
}

// finalResponseStatus returns the HTTP status the downstream client will
// actually see for a pass-through (non-short-circuited) response.
//
// responseBodyCtx.ResponseStatus starts out equal to the upstream ":status"
// (both are seeded from the same value in buildResponseContexts) and is
// overwritten in place by applyResponseModifications when a response-body policy
// sets StatusCode — the same value the translator emits as a ":status" header op
// for Envoy. So it must be read only AFTER chain execution has returned.
// Response-HEADER policies cannot change the status
// (DownstreamResponseHeaderModifications has no StatusCode field), so the header
// context is only a fallback for a bodyless response.
func (ec *PolicyExecutionContext) finalResponseStatus() int {
	if ec.responseBodyCtx != nil && ec.responseBodyCtx.ResponseStatus != 0 {
		return ec.responseBodyCtx.ResponseStatus
	}
	if ec.responseHeaderCtx != nil {
		return ec.responseHeaderCtx.ResponseStatus
	}
	return 0 // request phase — no response status exists yet
}

// resolveTerminalOutcome derives the terminal HTTP outcome of the phase that is
// about to return resp, and memoizes it on ec for the root-span stamp in
// Process. Reading the outgoing ext_proc response rather than tracking state at
// every generator means every terminating status — a policy deny, a generated
// 500 or 413, a python-bridge fault — is classified from what actually goes on
// the wire and cannot drift.
//
// Must be called only after chain execution has returned, because a
// response-body policy can still change the status at that point
// (applyResponseModifications).
func (ec *PolicyExecutionContext) resolveTerminalOutcome(resp *extprocv3.ProcessingResponse) tracing.HTTPOutcome {
	var out tracing.HTTPOutcome

	if imm := resp.GetImmediateResponse(); imm != nil {
		if ec.generated.resp == resp {
			// Built by handlePolicyError / handlePayloadTooLarge.
			out = ec.generated.outcome
		} else {
			// Every other ImmediateResponse originates from a policy returning
			// policy.ImmediateResponse (auth denial, rate limit, guardrail, or a
			// python-bridge fault).
			out = tracing.HTTPOutcome{
				StatusCode: int(imm.GetStatus().GetCode()),
				Reason:     constants.TerminalReasonPolicyDenied,
			}
		}
	} else if status := ec.finalResponseStatus(); status != 0 {
		// Pass-through: the client sees the upstream status, post policy override.
		// finalResponseStatus() can't tell those two cases apart on its own (a
		// response-body policy overwrites ResponseStatus in place), so
		// responseStatusOverridden — set when a policy actually supplied
		// DownstreamResponseModifications.StatusCode — picks the reason.
		reason := constants.TerminalReasonUpstream
		if ec.responseStatusOverridden {
			reason = constants.TerminalReasonPolicyStatusOverride
		}
		out = tracing.HTTPOutcome{
			StatusCode: status,
			Reason:     reason,
		}
	}

	// A request-phase pass-through has no status yet; never let it erase a status
	// resolved by an earlier phase.
	if out.StatusCode != 0 {
		ec.terminal = out
	}
	return out
}

// responseStatusOverriddenByPolicy reports whether any non-skipped, non-errored
// response-body policy result set DownstreamResponseModifications.StatusCode.
// Mirrors the same Results scan translator.go performs when building the
// ":status" header op, so the two can never disagree about whether a policy
// touched the status.
func responseStatusOverriddenByPolicy(results []executor.ResponsePolicyResult) bool {
	for _, r := range results {
		if r.Skipped || r.Error != nil {
			continue
		}
		if mods, ok := r.Action.(policy.DownstreamResponseModifications); ok && mods.StatusCode != nil {
			return true
		}
	}
	return false
}

// responsePayloadTooLargeError fails the ext_proc stream closed when an upstream
// response exceeds the decompression ceiling. A response-body ImmediateResponse
// cannot reliably replace an upstream response because its headers may already
// have reached the downstream codec. Returning an error lets Envoy's fail-closed
// ext_proc configuration terminate the response instead of promising a late 413.
func (ec *PolicyExecutionContext) responsePayloadTooLargeError(
	ctx context.Context,
	err error,
	phase string,
) error {
	errorID := uuid.New().String()

	slog.WarnContext(ctx, "Rejecting upstream response: decompressed payload exceeds configured limit",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"error", err,
	)

	return fmt.Errorf("upstream response decompression limit exceeded (error_id=%s): %w", errorID, err)
}

// requestPayloadTooLargeError fails a full-duplex request closed when its
// decompressed output exceeds the ceiling. Earlier request chunks may already be
// upstream, so an ImmediateResponse cannot reliably promise an HTTP 413.
func (ec *PolicyExecutionContext) requestPayloadTooLargeError(
	ctx context.Context,
	err error,
	phase string,
) error {
	errorID := uuid.New().String()

	slog.WarnContext(ctx, "Rejecting streaming request: decompressed payload exceeds configured limit",
		"error_id", errorID,
		"request_id", ec.requestID,
		"phase", phase,
		"route_key", ec.routeKey,
		"error", err,
	)

	return fmt.Errorf("streaming request decompression limit exceeded (error_id=%s): %w", errorID, err)
}

// getModeOverride returns the ProcessingMode override for this execution context.
// ec.isStreamingResponse is the single source of truth for whether the response body is
// processed in streaming mode — it is set once in processResponseHeaders via
// responseStreamingEnabled(), and this function must not re-derive that decision, or the
// ModeOverride sent to Envoy could disagree with which body-phase handler actually runs
// (see processResponseBody), which Envoy rejects as a content-length/body mismatch.
func (ec *PolicyExecutionContext) getModeOverride() *extprocconfigv3.ProcessingMode {
	mode := &extprocconfigv3.ProcessingMode{
		ResponseHeaderMode: extprocconfigv3.ProcessingMode_SEND,
	}

	if ec.policyChain.RequiresRequestBody {
		if ec.isStreamingRequest {
			mode.RequestBodyMode = extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED
			slog.Debug("[mode] upgraded request body mode to FULL_DUPLEX_STREAMED",
				"route", ec.routeKey,
			)
		} else {
			mode.RequestBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
		}
	} else {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_NONE
	}

	if ec.policyChain.RequiresResponseBody {
		if ec.isStreamingResponse {
			mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_FULL_DUPLEX_STREAMED
			slog.Debug("[mode] upgraded response body mode to FULL_DUPLEX_STREAMED",
				"route", ec.routeKey,
			)
		} else {
			mode.ResponseBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
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

	// Reject before any policy runs and before a single body byte is forwarded
	// upstream. Only matters when the chain actually inspects the request body —
	// with no body policy attached there is nothing to bypass, so an encoding the
	// kernel cannot read is simply none of its business and passes through.
	if ec.requestEncodingBlocksBodyPolicies() {
		return ec.rejectUnsupportedRequestEncoding(ctx, "request_headers"), nil
	}

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
	// Envoy's response-headers EndOfStream flag is the authoritative signal: it
	// reflects the actual framing, so a header-only response is recognised as
	// bodyless even when status/method/Content-Length say nothing about it.
	if ec.responseHeadersEndOfStream {
		return true
	}
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
// The decision rests solely on Envoy's EndOfStream flag from the request-headers
// message, which is set from the framing actually observed on the wire — for a
// bodyless GET, a HEAD, or Content-Length: 0 that flag is already true.
// Method and Content-Length are deliberately NOT consulted: a GET may carry a
// body, Envoy then reports EndOfStream=false and does deliver a body phase for
// it, and treating such a request as bodyless would skip
// requestEncodingBlocksBodyPolicies (an unreadable Content-Encoding reaching
// policies unchecked) and run body policies twice — once inline here with a nil
// body and again when the body arrives.
func (ec *PolicyExecutionContext) requestHasNoBody() bool {
	return ec.requestHeadersEndOfStream
}

// requestEncodingBlocksBodyPolicies reports whether the request carries a Content-Encoding
// the kernel cannot decode while the chain needs to inspect the request body. With no body
// policy attached there is nothing to bypass, and a bodyless request has nothing to decode,
// so both cases pass through untouched.
func (ec *PolicyExecutionContext) requestEncodingBlocksBodyPolicies() bool {
	return ec.requestEncodingUnsupported && ec.policyChain.RequiresRequestBody && !ec.requestHasNoBody()
}

// responseEncodingBlocksBodyPolicies is the response-side counterpart of
// requestEncodingBlocksBodyPolicies. Header and body phases must agree on it: a response the
// header phase let through must not be failed later at the body phase.
func (ec *PolicyExecutionContext) responseEncodingBlocksBodyPolicies() bool {
	return ec.responseEncodingUnsupported && ec.policyChain.RequiresResponseBody && !ec.responseHasNoBody()
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

// processResponseBodyForEmptyResponse executes body policies inline during the response-headers
// phase for responses that carry no body. The body context is set to Present=false / EndOfStream=true
// so policies can inspect headers-only state and still run on every response.
func (ec *PolicyExecutionContext) processResponseBodyForEmptyResponse(
	ctx context.Context,
	headerResult *executor.ResponseHeaderExecutionResult,
) (*extprocv3.ProcessingResponse, error) {
	// Ensure the body context reflects a nil/absent body.
	ec.responseBodyCtx.ResponseBody = &policy.Body{Content: nil, EndOfStream: true, Present: false}

	slog.DebugContext(ctx, "[no-body] executing response body policies inline during response-headers phase",
		"route", ec.routeKey,
		"status", ec.responseHeaderCtx.ResponseStatus,
	)

	bodyResult, err := ec.server.executor.ExecuteResponsePolicies(
		ctx,
		ec.policyChain.Policies,
		ec.responseBodyCtx,
		ec.policyChain.PolicySpecs,
		ec.sharedCtx.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "response_body_no_body"), nil
	}
	if responseStatusOverriddenByPolicy(bodyResult.Results) {
		ec.responseStatusOverridden = true
	}

	return TranslateResponseHeaderActionsWithBodyMerge(headerResult, bodyResult, ec)
}

// processRequestBody processes request body phase
func (ec *PolicyExecutionContext) processRequestBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	ec.phase = phaseRequestBody

	// Mirror of the response-body guard below: if a body arrives under a
	// Content-Encoding the kernel cannot decode while the chain inspects request
	// bodies, reject it here too. The header phase normally catches this, but it
	// can only act on what the headers message said — a request whose framing
	// promised no body and then delivered one must not slip past on that basis.
	// Nothing has been forwarded upstream yet, so a clean 415 is still possible.
	if ec.requestEncodingUnsupported && ec.policyChain.RequiresRequestBody {
		return ec.rejectUnsupportedRequestEncoding(ctx, "request_body"), nil
	}

	if ec.isStreamingRequest {
		return ec.processStreamingRequestBody(ctx, body)
	}

	if ec.policyChain.RequiresRequestBody {
		// Decompress body if Content-Encoding was set, so policies receive plain bytes.
		bodyContent := body.Body
		if ec.requestContentEncoding != "" {
			ec.requestContentEncoding = resolveEncodingFromBody(ec.requestContentEncoding, body.Body)
			decompressed, err := decompressBody(body.Body, ec.requestContentEncoding, ec.server.maxRequestDecompressedBytes)
			if err != nil {
				// Over-limit bodies must be rejected, never forwarded raw.
				if errors.Is(err, ErrDecompressedTooLarge) {
					return ec.handlePayloadTooLarge(ctx, err, "request_body"), nil
				}
				// A body that does not decode as its declared encoding is rejected,
				// not handed to policies raw. Falling through would let any caller
				// disable every request-body policy by labelling arbitrary bytes
				// "Content-Encoding: gzip" — the policies see bytes they cannot
				// parse, match nothing, and the body is forwarded upstream anyway.
				return ec.rejectUndecodableBody(ctx, err, "request_body",
					typev3.StatusCode_BadRequest, http.StatusBadRequest, "Bad Request"), nil
			}
			bodyContent = decompressed
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
	// An empty NON-TERMINAL chunk carries no encoding evidence, so the decoder is
	// not built from it: the deflate variant would be guessed from zero bytes and
	// cannot be corrected once the decoder is running. Nothing is lost by waiting
	// — feeding an empty chunk produces no output either.
	// An empty chunk that IS terminal is different: it means the whole encoded body
	// was zero bytes, which no codec produces. Building the decoder and feeding it
	// the end-of-stream is what turns that into a rejection, matching the buffered
	// path (decompressBody fails on empty input) — skipping it would forward an
	// unvalidated body under a Content-Encoding no policy ever read.
	if ec.requestContentEncoding != "" &&
		(len(chunk.Chunk) > 0 || chunk.EndOfStream || ec.requestStreamDecomp != nil || len(ec.requestDeflatePending) > 0) {
		if ec.requestStreamDecomp == nil {
			// Pin the deflate variant before the decoder for it is built — the
			// decoder cannot be swapped once running. A first chunk may legally
			// carry a single byte, which is not enough to tell the two variants
			// apart, so hold the leading bytes back until it is.
			ec.requestDeflatePending = append(ec.requestDeflatePending, chunk.Chunk...)
			if needsMoreDeflateVariantEvidence(ec.requestContentEncoding, ec.requestDeflatePending, chunk.EndOfStream) {
				return suppressedRequestChunk(), nil
			}
			chunk.Chunk = ec.requestDeflatePending
			ec.requestDeflatePending = nil
			ec.requestContentEncoding = resolveEncodingFromBody(ec.requestContentEncoding, chunk.Chunk)
			ec.requestStreamDecomp = newStreamDecompressor(ec.requestContentEncoding, ec.server.maxRequestDecompressedBytes)
		}
		decompressed, err := ec.requestStreamDecomp.FeedChunk(chunk.Chunk, chunk.EndOfStream)
		if err != nil {
			ec.requestStreamDecomp.Close()
			ec.requestStreamDecomp = nil
			if errors.Is(err, ErrDecompressedTooLarge) {
				return nil, ec.requestPayloadTooLargeError(ctx, err, "request_body_streaming")
			}
			slog.Warn("[streaming] per-chunk request decompression error; failing stream closed",
				"request_id", ec.requestID,
				"encoding", ec.requestContentEncoding,
				"error", err,
			)
			return nil, fmt.Errorf("streaming request decompression failed: %w", err)
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

// suppressedRequestChunk emits nothing for this request chunk while keeping
// Envoy's FULL_DUPLEX_STREAMED accounting intact. In that mode an empty
// BodyResponse would pass the chunk through unchanged, so withholding data has
// to be spelled out as an empty StreamedBodyResponse.
func suppressedRequestChunk() *extprocv3.ProcessingResponse {
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
	}
}

// suppressedResponseChunk is the response-side counterpart of suppressedRequestChunk.
func suppressedResponseChunk() *extprocv3.ProcessingResponse {
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
	}
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

	// Reject here, at the last point an HTTP status can still be chosen: response
	// headers have not been forwarded downstream yet, so this becomes a clean 502
	// rather than a mid-stream reset. As on the request side, this only applies
	// when the chain actually inspects the response body.
	if ec.responseEncodingBlocksBodyPolicies() {
		return ec.rejectUnsupportedResponseEncoding(ctx), nil
	}

	// Detect streaming response: upgrade when chain supports streaming AND
	// upstream signals chunked/SSE AND body is coming (not EndOfStream).
	slog.Debug("[mode] response headers received — streaming detection",
		"route", ec.routeKey,
		"supports_response_streaming", ec.policyChain.SupportsResponseStreaming,
		"headers_end_of_stream", headers.EndOfStream,
		"streaming_headers_detected", isStreamingUpstreamResponse(ec.responseHeaderCtx.ResponseHeaders),
		"content_type", ec.responseHeaderCtx.ResponseHeaders.Get("content-type"),
		"transfer_encoding", ec.responseHeaderCtx.ResponseHeaders.Get("transfer-encoding"),
	)
	ec.isStreamingResponse = ec.responseStreamingEnabled(headers.EndOfStream)
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

	// For bodyless responses Envoy skips the ResponseBody ext_proc phase entirely.
	// Execute body policies inline now so they run on every response, receiving a nil body.
	if !execResult.ShortCircuited && ec.policyChain.RequiresResponseBody && ec.responseHasNoBody() {
		return ec.processResponseBodyForEmptyResponse(ctx, execResult)
	}

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

	// Defence in depth. processResponseHeaders already rejected this response, so
	// reaching the body phase with an undecodable encoding means the header-phase
	// guard was bypassed or removed. Fail the stream rather than fall through to
	// policies that would receive compressed bytes as if they were plaintext. Uses the same
	// predicate as the header phase so a response that phase deliberately let through (no
	// body to decode) is not failed here instead.
	if ec.responseEncodingBlocksBodyPolicies() {
		return nil, fmt.Errorf("response body phase reached with undecodable Content-Encoding; refusing to run body policies")
	}

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
			ec.responseContentEncoding = resolveEncodingFromBody(ec.responseContentEncoding, body.Body)
			decompressed, err := decompressBody(body.Body, ec.responseContentEncoding, ec.server.maxResponseDecompressedBytes)
			if err != nil {
				if errors.Is(err, ErrDecompressedTooLarge) {
					return nil, ec.responsePayloadTooLargeError(ctx, err, "response_body")
				}
				// As on the request side: a response that does not decode as its
				// declared encoding is rejected rather than handed to policies raw,
				// so a malformed upstream body cannot silently skip masking or
				// redaction on its way to the client.
				return ec.rejectUndecodableBody(ctx, err, "response_body",
					typev3.StatusCode_BadGateway, http.StatusBadGateway, "Bad Gateway"), nil
			}
			bodyContent = decompressed
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
		if responseStatusOverriddenByPolicy(execResult.Results) {
			ec.responseStatusOverridden = true
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
	// A policy previously terminated the stream. Envoy may still deliver buffered
	// upstream chunks after processing has been terminated. Keep suppressing them
	// while mirroring the upstream EndOfStream flag; Envoy's StreamedBodyResponse
	// contract does not permit us to invent an early EOS.
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
								StreamedResponse: &extprocv3.StreamedBodyResponse{
									EndOfStream: body.EndOfStream,
								},
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

	// Decompression is a transform applied BEFORE the shared accumulation logic
	// below, not a separate processing path. Policies must observe exactly the same
	// contract — same accumulation, same NeedsMoreResponseData consultation —
	// whether or not the upstream compressed the response. When this was a second
	// path that fed chunks straight to policies, NeedsMoreResponseData was never
	// called on a compressed response, so any policy relying on it (the documented
	// SDK hook for cross-chunk buffering) worked on plaintext and silently degraded
	// on gzip: word-count/sentence-count guardrails evaluated isolated fragments
	// instead of assembled content, and content-rewriting policies never saw a
	// placeholder that straddled a chunk boundary.
	// As on the request path: an empty non-terminal chunk is no evidence of the
	// deflate variant, and the decoder cannot be swapped once running — so defer
	// building it until a chunk actually carries bytes. A terminal empty chunk is
	// still decoded, so a zero-byte body under a declared Content-Encoding is
	// rejected here rather than forwarded as one no policy could read.
	if ec.responseContentEncoding != "" &&
		(len(chunk.Chunk) > 0 || chunk.EndOfStream || ec.responseStreamDecomp != nil || len(ec.responseDeflatePending) > 0) {
		if ec.responseStreamDecomp == nil {
			// Pin the deflate variant before the decoder for it is built — the
			// decoder cannot be swapped once running. As on the request path, a
			// one-byte first chunk carries too little evidence, so hold the leading
			// bytes back until deflateVariantProbeBytes have arrived.
			ec.responseDeflatePending = append(ec.responseDeflatePending, chunk.Chunk...)
			if needsMoreDeflateVariantEvidence(ec.responseContentEncoding, ec.responseDeflatePending, chunk.EndOfStream) {
				return suppressedResponseChunk(), nil
			}
			chunk.Chunk = ec.responseDeflatePending
			ec.responseDeflatePending = nil
			ec.responseContentEncoding = resolveEncodingFromBody(ec.responseContentEncoding, chunk.Chunk)
			ec.responseStreamDecomp = newStreamDecompressor(ec.responseContentEncoding, ec.server.maxResponseDecompressedBytes)
		}
		decompressed, err := ec.responseStreamDecomp.FeedChunk(chunk.Chunk, chunk.EndOfStream)
		if err != nil {
			// Failing the ext_proc stream makes Envoy reset the response; suppressing
			// until upstream EOS would turn the failure into a successful truncated body.
			slog.Warn("[streaming] per-chunk response decompression error; failing stream closed",
				"request_id", ec.requestID,
				"encoding", ec.responseContentEncoding,
				"error", err,
			)
			ec.responseStreamDecomp.Close()
			ec.responseStreamDecomp = nil
			if errors.Is(err, ErrDecompressedTooLarge) {
				return nil, ec.responsePayloadTooLargeError(ctx, err, "response_body_streaming")
			}
			return nil, fmt.Errorf("streaming upstream response decompression failed: %w", err)
		}
		chunk.Chunk = decompressed

		slog.Debug("[streaming] response chunk decompressed",
			"route", ec.routeKey,
			"decompressed_bytes", len(chunk.Chunk),
			"end_of_stream", chunk.EndOfStream,
		)
	}

	if len(chunk.Chunk) > 0 {
		ec.streamAccumulator = append(ec.streamAccumulator, chunk.Chunk...)
	}

	// Nothing to hand policies yet. For a compressed stream this is the common
	// case: the decoder needs more input before it can emit a block. Forwarding an
	// empty streamed response keeps Envoy's chunk accounting intact.
	if len(ec.streamAccumulator) == 0 && !chunk.EndOfStream {
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

	slog.Debug("[streaming] response chunk received",
		"route", ec.routeKey,
		"chunk_bytes", len(chunk.Chunk),
		"accumulated_bytes", len(ec.streamAccumulator),
		"encoding", ec.responseContentEncoding,
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
				// Only encodings the kernel can round-trip are recorded; anything
				// else is flagged so the request is rejected outright before any
				// body policy runs (see rejectUnsupportedRequestEncoding).
				//
				// This value is chosen by the caller, so treating an unrecognised
				// one as "no encoding" would be a policy bypass primitive: the body
				// would reach policies as opaque compressed bytes, match nothing,
				// and be forwarded to the upstream unchanged — guardrails, content
				// moderation and schema validation all silently skipped by setting
				// a header. Content codings are case-insensitive tokens
				// (RFC 9110 §8.4.1), so normalise before matching or "GZIP" alone
				// would take that path.
				encoding := strings.ToLower(strings.TrimSpace(value))
				if isRecompressibleEncoding(encoding) {
					ec.requestContentEncoding = encoding
				} else if encoding != "" && encoding != encodingIdentity {
					ec.requestEncodingUnsupported = true
				}
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
		APIKind:       policy.APIKind(routeMetadata.APIKind),
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

	// Capture a snapshot of the downstream (client) headers before any policy
	// mutation. Header-phase policies mutate wrappedHeaders in
	// place, so body/stream-phase validators need this pristine copy to inspect
	// what the client actually sent.
	ec.downstreamHeaders = cloneHeaders(wrappedHeaders)
	downstream := &policy.DownstreamContext{Request: &policy.DownstreamRequest{
		Headers:   ec.downstreamHeaders,
		Path:      path,
		Method:    method,
		Authority: authority,
		Scheme:    scheme,
	}}

	ec.requestHeaderCtx = &policy.RequestHeaderContext{
		SharedContext: sharedCtx,
		Headers:       wrappedHeaders,
		Path:          path,
		Method:        method,
		Authority:     authority,
		Scheme:        scheme,
		Vhost:         routeMetadata.Vhost,
		Downstream:    downstream,
		Upstream:      toRequestUpstream(ec.defaultUpstream),
	}

	// requestBodyCtx shares the same shared context and headers; Body is set later.
	ec.requestHeadersEndOfStream = headers.EndOfStream
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
		UpstreamInfo:  ec.defaultUpstream,
		Downstream:    downstream,
		Upstream:      toRequestUpstream(ec.defaultUpstream),
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
		Downstream:    downstream,
		Upstream:      toRequestUpstream(ec.defaultUpstream),
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
				// Only encodings the kernel can actually decompress AND re-compress
				// are recorded. Anything else is flagged and the response is rejected
				// at the header phase when the chain requires the body — the
				// decompressor would otherwise hand policies raw compressed bytes, so
				// a content-rewriting policy silently matches nothing (pii-masking-regex
				// delivering "[EMAIL_0000]" to the client instead of restoring it, with
				// no error anywhere).
				//
				// Content codings are case-insensitive tokens (RFC 9110 §8.4.1), so
				// normalise before matching — the decompressor/compressor switches are
				// lowercase-only and would otherwise miss a "GZIP" response.
				encoding := strings.ToLower(strings.TrimSpace(value))
				if isRecompressibleEncoding(encoding) {
					ec.responseContentEncoding = encoding
				} else if encoding != "" && encoding != encodingIdentity {
					ec.responseEncodingUnsupported = true
				}
			}
		}
	}

	responseHeaders := policy.NewHeaders(responseHeadersMap)

	// Downstream snapshot: reuse the request-time snapshot built in
	// buildRequestContexts, which already carries the pristine client headers
	// (ec.downstreamHeaders) plus path/method/authority/scheme. Reusing it —
	// rather than rebuilding from headers alone — keeps the response path's
	// scalar fields populated and guarantees no drift from the request phase.
	downstream := ec.requestHeaderCtx.Downstream

	// Upstream: the route's resolved upstream target plus a snapshot of the
	// original upstream response headers, captured before any response-header
	// policy mutation.
	upstream := toResponseUpstream(ec.defaultUpstream, cloneHeaders(responseHeaders), responseStatus)

	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		SharedContext:   ec.sharedCtx,
		RequestHeaders:  ec.requestHeaderCtx.Headers,
		RequestBody:     ec.requestBodyCtx.Body,
		RequestPath:     ec.requestHeaderCtx.Path,
		RequestMethod:   ec.requestHeaderCtx.Method,
		ResponseHeaders: responseHeaders,
		ResponseStatus:  responseStatus,
		Downstream:      downstream,
		Upstream:        upstream,
	}

	ec.responseHeadersEndOfStream = headers.EndOfStream
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
		Downstream:      downstream,
		Upstream:        upstream,
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
		Downstream:      downstream,
		Upstream:        upstream,
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

// responseStreamingEnabled reports whether the response body should be processed in
// streaming (FULL_DUPLEX_STREAMED) mode. This is the single source of truth: it decides
// both the ModeOverride sent to Envoy (getModeOverride) and which body-phase handler runs
// (processResponseBody), so the two can never disagree. Callable only once
// responseHeaderCtx has been populated (i.e. from processResponseHeaders).
func (ec *PolicyExecutionContext) responseStreamingEnabled(endOfStream bool) bool {
	if !ec.policyChain.SupportsResponseStreaming || endOfStream {
		return false
	}
	if !isStreamingUpstreamResponse(ec.responseHeaderCtx.ResponseHeaders) {
		return false
	}
	// MCP is kept buffered: when an upstream MCP server sends a Transfer-Encoding: chunked
	// response with an empty body, Envoy does not deliver the body phase to the policy
	// engine in FULL_DUPLEX_STREAMED mode.
	if ec.sharedCtx != nil && ec.sharedCtx.APIKind == policy.APIKindMCP {
		return false
	}
	return true
}

// cloneHeaders returns an independent copy of h — both the map and every
// value slice — so it survives in-place mutation of the source headers. Built on
// the SDK's public GetAll() (which already deep-copies) so no policy-facing
// surface is added for this kernel-only concern. Used to snapshot the original
// downstream/upstream headers before policy mutation.
func cloneHeaders(h *policy.Headers) *policy.Headers {
	return policy.NewHeaders(h.GetAll())
}

// toRequestUpstream maps the internal wire UpstreamInfo to the request-phase SDK
// type surfaced to policies. Returns nil when no upstream is resolved so the
// context field is left unset (older gateways / no route upstream).
//
// Name is intentionally left unset: the internal Envoy cluster name must
// not be exposed here, and the wire UpstreamInfo carries no user-facing name.
// Policies that still need the raw cluster name can read the deprecated
// RequestContext.UpstreamInfo field.
func toRequestUpstream(info *policyenginev1.UpstreamInfo) *policy.UpstreamRequestContext {
	if info == nil {
		return nil
	}
	return &policy.UpstreamRequestContext{
		URL:      info.URL,
		BasePath: info.BasePath,
	}
}

// toResponseUpstream maps the internal wire UpstreamInfo plus the upstream
// response header snapshot to the response-phase SDK type. The UpstreamResponseContext
// is always built (the response came from an upstream), with the identity fields
// filled only when info is available. Name is left unset for the same
// reason as toRequestUpstream — the cluster name is not exposed here.
func toResponseUpstream(info *policyenginev1.UpstreamInfo, respHeaders *policy.Headers, statusCode int) *policy.UpstreamResponseContext {
	us := &policy.UpstreamResponseContext{
		Response: &policy.UpstreamResponse{Headers: respHeaders, StatusCode: statusCode},
	}
	if info != nil {
		us.URL = info.URL
		us.BasePath = info.BasePath
	}
	return us
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
