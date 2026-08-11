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
	"io"
	"log/slog"
	"net/http"
	"time"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// ExternalProcessorServer implements the Envoy external processor service
// T059: ExternalProcessorServer gRPC service struct
type ExternalProcessorServer struct {
	extprocv3.UnimplementedExternalProcessorServer

	kernel    *Kernel
	executor  *executor.ChainExecutor
	tracer    trace.Tracer
	resolvers resolver.ResolverRegistry

	// Per-direction caps on decompressed bytes buffered per body (buffered mode)
	// or per chunk (streaming), from policy_engine.request_body/.response_body config.
	// Always positive: the constructor falls back to
	// config.DefaultMaxDecompressedBytes for any non-positive input, so a body is
	// never decompressed without a ceiling.
	maxRequestDecompressedBytes  int64
	maxResponseDecompressedBytes int64
}

// NewExternalProcessorServer creates a new ExternalProcessorServer.
// resolvers is the frozen operation-resolver registry; a nil value behaves as an
// identity-only registry, so a route naming any other resolver fails closed.
func NewExternalProcessorServer(kernel *Kernel, chainExecutor *executor.ChainExecutor, tracingConfig config.TracingConfig, tracingServiceName string, maxRequestDecompressedBytes int64, maxResponseDecompressedBytes int64, resolvers resolver.ResolverRegistry) *ExternalProcessorServer {
	// Initialize tracer once - will be NoOp if tracing is disabled
	serviceName := tracingServiceName
	if serviceName == "" {
		serviceName = "policy-engine"
	}

	// Fail closed on a non-positive ceiling rather than decompressing unbounded.
	// Config.Validate already rejects these values, so reaching here means the
	// server was constructed without going through config loading.
	if maxRequestDecompressedBytes <= 0 {
		slog.Warn("Non-positive request body max_decompressed_bytes; falling back to default",
			"provided", maxRequestDecompressedBytes, "default", config.DefaultMaxDecompressedBytes)
		maxRequestDecompressedBytes = config.DefaultMaxDecompressedBytes
	}
	if maxResponseDecompressedBytes <= 0 {
		slog.Warn("Non-positive response body max_decompressed_bytes; falling back to default",
			"provided", maxResponseDecompressedBytes, "default", config.DefaultMaxDecompressedBytes)
		maxResponseDecompressedBytes = config.DefaultMaxDecompressedBytes
	}

	return &ExternalProcessorServer{
		kernel:                       kernel,
		executor:                     chainExecutor,
		tracer:                       otel.Tracer(serviceName),
		resolvers:                    resolvers,
		maxRequestDecompressedBytes:  maxRequestDecompressedBytes,
		maxResponseDecompressedBytes: maxResponseDecompressedBytes,
	}
}

// Process implements the bidirectional streaming RPC handler
// T060: Process(stream) bidirectional streaming RPC handler
func (s *ExternalProcessorServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	// Track active streams
	metrics.ActiveStreams.Inc()
	defer metrics.ActiveStreams.Dec()

	// Extract trace context and create span - NoOp if tracing disabled
	traceCtx := tracing.ExtractTraceContext(stream.Context())
	ctx, span := s.tracer.Start(traceCtx, constants.SpanExternalProcessingProcess,
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	// Execution context for this request-response lifecycle.
	// Initialized lazily on first request headers phase via handleProcessingPhase.
	// Passed by address (&execCtx) to allow initialization (nil -> allocated instance).
	// Lives until response complete, then garbage collected when stream ends.
	// One stream = one HTTP request, so this is allocated once per request.
	var execCtx *PolicyExecutionContext
	// Stamp the request's terminal HTTP status on the root span. Registered AFTER
	// `defer span.End()` above, so LIFO ordering guarantees this runs while the
	// span is still recording. execCtx.terminal holds the last outcome resolved
	// by handleProcessingPhase: the upstream response status for a pass-through,
	// or the denial/fault status for a short-circuit. Nothing is recorded when no
	// phase ever resolved a status (execCtx nil, or the stream ended mid-request);
	// paths that terminate without an execCtx stamp parentSpan inline instead.
	defer func() {
		if execCtx != nil {
			tracing.RecordHTTPOutcome(span, execCtx.terminal)
		}
	}()
	defer func() {
		if execCtx != nil {
			execCtx.closeStreamDecompressors()
		}
	}()

	for {
		// Receive request from Envoy
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Check if this is a normal stream closure due to context cancellation
			// This happens when Envoy closes the stream after completing the request
			if errors.Is(err, context.Canceled) || status.Code(err) == grpccodes.Canceled {
				// Log at debug level for visibility in troubleshooting
				slog.DebugContext(ctx, "Stream closed due to context cancellation")
				return nil
			}
			slog.ErrorContext(ctx, "Error receiving from stream", "error", err)
			metrics.StreamErrorsTotal.WithLabelValues("receive").Inc()
			if span.IsRecording() {
				span.RecordError(err)
				span.SetStatus(codes.Error, "ext_proc stream receive failed")
			}
			return status.Errorf(grpccodes.Unknown, "failed to receive request: %v", err)
		}

		// Handle the request based on phase
		resp, err := s.handleProcessingPhase(ctx, req, &execCtx, span)
		if err != nil {
			slog.ErrorContext(ctx, "Error processing request", "error", err)
			return err
		}

		// Send response back to Envoy
		if err := stream.Send(resp); err != nil {
			slog.ErrorContext(ctx, "Error sending response", "error", err)
			metrics.StreamErrorsTotal.WithLabelValues("send").Inc()
			if span.IsRecording() {
				span.RecordError(err)
				span.SetStatus(codes.Error, "ext_proc stream send failed")
			}
			return status.Errorf(grpccodes.Unknown, "failed to send response: %v", err)
		}
	}
}

// handleProcessingPhase routes processing to the appropriate phase handler
func (s *ExternalProcessorServer) handleProcessingPhase(ctx context.Context, req *extprocv3.ProcessingRequest, execCtx **PolicyExecutionContext, parentSpan trace.Span) (*extprocv3.ProcessingResponse, error) {
	switch req.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		startTime := time.Now()

		// Create span for request headers processing - NoOp if tracing disabled
		_, span := s.tracer.Start(ctx, constants.SpanProcessRequestHeaders,
			trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		// Initialize execution context for this request
		rm, outcome, denial := s.initializeExecutionContext(ctx, req, execCtx)
		if parentSpan.IsRecording() {
			parentSpan.SetAttributes(
				attribute.String(constants.AttrRouteName, rm.RouteName),
				attribute.String(constants.AttrAPIName, rm.APIName),
				attribute.String(constants.AttrAPIVersion, rm.APIVersion),
				attribute.String(constants.AttrAPIContext, rm.Context),
				attribute.String(constants.AttrOperationPath, rm.OperationPath),
			)
			// A deferred route has no chain yet, so only the resolver is known here;
			// the chain key is stamped from the request-body branch once binding
			// actually happens (recordResolutionAttributes).
			if *execCtx != nil {
				(*execCtx).recordResolutionAttributes(parentSpan)
			}
		}

		// Track request metrics
		metrics.RequestsTotal.WithLabelValues("request_headers", rm.RouteName, rm.APIName, rm.APIVersion).Inc()

		// Resolution ran and failed. Rendered in the transport's error shape when the
		// resolver supplies a renderer and the failure is one the protocol can
		// describe, sterile-generic otherwise. Never a fallback to the route-level
		// chain: that would silently apply the wrong policies.
		if outcome == bindFailed {
			resp, failureOutcome := renderResolutionFailure(ctx, denial.resolverName, rm.RouteName, "",
				denial.view, denial.protocolState, denial.renderer, denial.failure)
			tracing.RecordHTTPOutcome(span, failureOutcome)
			tracing.RecordHTTPOutcome(parentSpan, failureOutcome)
			metrics.RequestDurationSeconds.WithLabelValues("request_headers", rm.RouteName).Observe(time.Since(startTime).Seconds())
			if slog.Default().Enabled(ctx, slog.LevelDebug) {
				slog.DebugContext(ctx, "ext_proc response", "phase", "request_headers", "resp", prototext.Format(resp))
			}
			return resp, nil
		}

		// The route's resolver needs the request body: retain the request, tell Envoy
		// to buffer, and run nothing until the body arrives.
		if outcome == bindPending {
			resp := pendingResolutionResponse((*execCtx).pending.requirements)
			metrics.RequestDurationSeconds.WithLabelValues("request_headers", rm.RouteName).Observe(time.Since(startTime).Seconds())
			if slog.Default().Enabled(ctx, slog.LevelDebug) {
				slog.DebugContext(ctx, "ext_proc response", "phase", "request_headers", "resp", prototext.Format(resp))
			}
			return resp, nil
		}

		// If no execution context (no policy chain found), return 500
		if *execCtx == nil {
			if span.IsRecording() {
				span.SetAttributes(attribute.Int(constants.AttrPolicyCount, 0))
			}
			outcome := tracing.HTTPOutcome{
				StatusCode: http.StatusInternalServerError,
				Reason:     constants.TerminalReasonNoPolicyChain,
			}
			tracing.RecordHTTPOutcome(span, outcome)
			tracing.RecordHTTPOutcome(parentSpan, outcome)
			metrics.RouteLookupFailuresTotal.Inc()
			metrics.RequestDurationSeconds.WithLabelValues("request_headers", rm.RouteName).Observe(time.Since(startTime).Seconds())
			slog.ErrorContext(ctx, "Policy chain not found for route, returning 500",
				"route", rm.RouteName,
				"api_name", rm.APIName)
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &extprocv3.ImmediateResponse{
						Status: &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError},
						Headers: buildHeaderValueOptions(map[string]string{
							"content-type": "application/json",
						}),
						// TODO: (renuka) handle error codes in a separate issue: https://github.com/wso2/api-platform/issues/1637
						Body: []byte(`{"error":"Internal Server Error"}`),
					},
				},
			}, nil
		}
		if span.IsRecording() {
			span.SetAttributes(attribute.Int(constants.AttrPolicyCount, len((*execCtx).policyChain.Policies)))
		}

		resp, err := (*execCtx).processRequestHeaders(ctx)
		metrics.RequestDurationSeconds.WithLabelValues("request_headers", rm.RouteName).Observe(time.Since(startTime).Seconds())
		if span.IsRecording() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				parentSpan.RecordError(err)
				parentSpan.SetStatus(codes.Error, err.Error())
			}
		}
		if err != nil {
			metrics.RequestErrorsTotal.WithLabelValues("request_headers", "processing_failed", rm.RouteName).Inc()
			// A fatal (stream-ending) error has no resp to derive an outcome from.
			// Overwrite any outcome memoized by an earlier phase so the root-span
			// defer in Process doesn't stamp a stale success status onto a
			// request that actually failed here.
			(*execCtx).terminal = tracing.HTTPOutcome{
				StatusCode: http.StatusInternalServerError,
				Reason:     constants.TerminalReasonProcessingFailed,
			}
		}
		// Stamp the terminal HTTP status on the phase span. Only on the success
		// path: when err != nil, resp is nil and the failure is already recorded
		// above. The root span is stamped once, by the defer in Process.
		if err == nil && resp != nil {
			tracing.RecordHTTPOutcome(span, (*execCtx).resolveTerminalOutcome(resp))
		}
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.DebugContext(ctx, "ext_proc response", "phase", "request_headers", "resp", prototext.Format(resp))
		}
		return resp, err

	case *extprocv3.ProcessingRequest_RequestBody:
		startTime := time.Now()

		// Create span for request body processing - NoOp if tracing disabled
		_, span := s.tracer.Start(ctx, constants.SpanProcessRequestBody,
			trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		if *execCtx == nil {
			slog.WarnContext(ctx, "Request body received before request headers")
			if span.IsRecording() {
				span.SetAttributes(attribute.String(constants.AttrError, constants.AttrErrorReasonNoContext))
			}
			metrics.RequestErrorsTotal.WithLabelValues("request_body", "no_context", "unknown").Inc()
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestBody{
					RequestBody: &extprocv3.BodyResponse{},
				},
			}, nil
		}

		routeName := (*execCtx).routeKey
		metrics.RequestsTotal.WithLabelValues("request_body", routeName, "", "").Inc()

		// Track body bytes
		if body := req.GetRequestBody(); body != nil {
			metrics.BodyBytesProcessed.WithLabelValues("request", "read").Add(float64(len(body.Body)))
		}

		resp, err := (*execCtx).processRequestBody(ctx, req.GetRequestBody())

		// A route whose chain is selected at this callback only learns its chain key
		// here, and it is the attribute that answers "which chain did this operation
		// get?" — the whole point of the resolver observability. Stamped on both the
		// phase span and the request's root span, since the header phase could not.
		if (*execCtx).boundAtBodyPhase {
			(*execCtx).recordResolutionAttributes(span)
			(*execCtx).recordResolutionAttributes(parentSpan)
		}

		metrics.RequestDurationSeconds.WithLabelValues("request_body", routeName).Observe(time.Since(startTime).Seconds())
		if span.IsRecording() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				parentSpan.RecordError(err)
				parentSpan.SetStatus(codes.Error, err.Error())
			}
		}
		if err != nil {
			metrics.RequestErrorsTotal.WithLabelValues("request_body", "processing_failed", routeName).Inc()
			// See the request_headers case above: overwrite any stale outcome
			// memoized by an earlier phase.
			(*execCtx).terminal = tracing.HTTPOutcome{
				StatusCode: http.StatusInternalServerError,
				Reason:     constants.TerminalReasonProcessingFailed,
			}
		}
		// Stamp the terminal HTTP status on the phase span. Only on the success
		// path: when err != nil, resp is nil and the failure is already recorded
		// above. The root span is stamped once, by the defer in Process.
		if err == nil && resp != nil {
			tracing.RecordHTTPOutcome(span, (*execCtx).resolveTerminalOutcome(resp))
		}
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.DebugContext(ctx, "ext_proc response", "phase", "request_body", "resp", prototext.Format(resp))
		}
		return resp, err

	case *extprocv3.ProcessingRequest_ResponseHeaders:
		startTime := time.Now()

		// Create span for response headers processing - NoOp if tracing disabled
		_, span := s.tracer.Start(ctx, constants.SpanProcessResponseHeaders,
			trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		if *execCtx == nil {
			slog.WarnContext(ctx, "Response headers received without execution context")
			if span.IsRecording() {
				span.SetAttributes(attribute.String(constants.AttrError, constants.AttrErrorReasonNoContext))
			}
			metrics.RequestErrorsTotal.WithLabelValues("response_headers", "no_context", "unknown").Inc()
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extprocv3.HeadersResponse{},
				},
			}, nil
		}

		routeName := (*execCtx).routeKey
		metrics.RequestsTotal.WithLabelValues("response_headers", routeName, "", "").Inc()

		resp, err := (*execCtx).processResponseHeaders(ctx, req.GetResponseHeaders())
		metrics.RequestDurationSeconds.WithLabelValues("response_headers", routeName).Observe(time.Since(startTime).Seconds())
		if span.IsRecording() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				parentSpan.RecordError(err)
				parentSpan.SetStatus(codes.Error, err.Error())
			}
		}
		if err != nil {
			metrics.RequestErrorsTotal.WithLabelValues("response_headers", "processing_failed", routeName).Inc()
			// See the request_headers case above: overwrite any stale outcome
			// memoized by an earlier phase.
			(*execCtx).terminal = tracing.HTTPOutcome{
				StatusCode: http.StatusInternalServerError,
				Reason:     constants.TerminalReasonProcessingFailed,
			}
		}
		// Stamp the terminal HTTP status on the phase span. Only on the success
		// path: when err != nil, resp is nil and the failure is already recorded
		// above. The root span is stamped once, by the defer in Process.
		if err == nil && resp != nil {
			tracing.RecordHTTPOutcome(span, (*execCtx).resolveTerminalOutcome(resp))
		}
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.DebugContext(ctx, "ext_proc response", "phase", "response_headers", "resp", prototext.Format(resp))
		}
		return resp, err

	case *extprocv3.ProcessingRequest_ResponseBody:
		startTime := time.Now()

		// Create span for response body processing - NoOp if tracing disabled
		_, span := s.tracer.Start(ctx, constants.SpanProcessResponseBody,
			trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		if *execCtx == nil {
			slog.WarnContext(ctx, "Response body received without execution context")
			if span.IsRecording() {
				span.SetAttributes(attribute.String(constants.AttrError, constants.AttrErrorReasonNoContext))
			}
			metrics.RequestErrorsTotal.WithLabelValues("response_body", "no_context", "unknown").Inc()
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseBody{
					ResponseBody: &extprocv3.BodyResponse{},
				},
			}, nil
		}

		routeName := (*execCtx).routeKey
		metrics.RequestsTotal.WithLabelValues("response_body", routeName, "", "").Inc()

		// Track body bytes
		if body := req.GetResponseBody(); body != nil {
			metrics.BodyBytesProcessed.WithLabelValues("response", "read").Add(float64(len(body.Body)))
		}

		resp, err := (*execCtx).processResponseBody(ctx, req.GetResponseBody())
		metrics.RequestDurationSeconds.WithLabelValues("response_body", routeName).Observe(time.Since(startTime).Seconds())
		if span.IsRecording() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				parentSpan.RecordError(err)
				parentSpan.SetStatus(codes.Error, err.Error())
			}
		}
		if err != nil {
			metrics.RequestErrorsTotal.WithLabelValues("response_body", "processing_failed", routeName).Inc()
			// This is the case the stale-memo bug was found in: a response-headers
			// pass-through memoizes {200, upstream_response}, then the response-body
			// phase fails closed (e.g. responsePayloadTooLargeError) with resp==nil,
			// so resolveTerminalOutcome is never called to refresh the memo. Without
			// this, the root span would end Error-status but still carry the stale
			// 200/upstream_response attributes from the earlier phase.
			(*execCtx).terminal = tracing.HTTPOutcome{
				StatusCode: http.StatusInternalServerError,
				Reason:     constants.TerminalReasonProcessingFailed,
			}
		}
		// Stamp the terminal HTTP status on the phase span. Only on the success
		// path: when err != nil, resp is nil and the failure is already recorded
		// above. The root span is stamped once, by the defer in Process.
		if err == nil && resp != nil {
			tracing.RecordHTTPOutcome(span, (*execCtx).resolveTerminalOutcome(resp))
		}
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.DebugContext(ctx, "ext_proc response", "phase", "response_body", "resp", prototext.Format(resp))
		}
		return resp, err

	default:
		slog.WarnContext(ctx, "Unknown request type", "type", fmt.Sprintf("%T", req.Request))
		metrics.RequestErrorsTotal.WithLabelValues("unknown", "unknown_type", "unknown").Inc()
		// No phase span exists on this path; parentSpan is the only span in scope.
		outcome := tracing.HTTPOutcome{
			StatusCode: http.StatusInternalServerError,
			Reason:     constants.TerminalReasonUnknownMessageType,
		}
		tracing.RecordHTTPOutcome(parentSpan, outcome)
		// If execCtx already exists (a stray unknown message mid-stream), also
		// update its memoized terminal outcome — otherwise Process's root-span
		// defer runs after this and overwrites what was just stamped above with
		// whatever an earlier phase memoized.
		if *execCtx != nil {
			(*execCtx).terminal = outcome
		}
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extprocv3.ImmediateResponse{
					Status: &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError},
				},
			},
		}, nil
	}
}

// initializeExecutionContext sets up the execution context for a request by
// selecting its policy chain. Route metadata is pre-loaded via xDS RouteConfigs —
// no request-time parsing needed.
//
// The returned outcome tells the caller which of four things happened; see
// routeBindOutcome. The failure value is non-nil only for bindFailed.
func (s *ExternalProcessorServer) initializeExecutionContext(
	ctx context.Context,
	req *extprocv3.ProcessingRequest,
	execCtx **PolicyExecutionContext,
) (*RouteMetadata, routeBindOutcome, *resolutionDenial) {
	// Extract route key from Envoy attributes (just xds.route_name, lightweight)
	routeKey := s.extractRouteKey(req)

	slog.DebugContext(ctx, "initializeExecutionContext: looking up route",
		"route_key", routeKey)

	rc := s.kernel.GetRouteConfig(routeKey)
	if rc == nil {
		// No RouteConfig found for this route key — empty metadata, nil exec context
		slog.DebugContext(ctx, "initializeExecutionContext: RouteConfig not found",
			"route_key", routeKey)
		*execCtx = nil
		return &RouteMetadata{RouteName: routeKey}, bindNoChain, nil
	}

	// Metadata is pre-populated from xDS — no request-time parsing needed
	routeMetadata := rc.Metadata
	routeMetadata.RouteName = routeKey

	// Identity: every API kind shipping today. The chain key is read from the field
	// the controller populated, never reconstructed, and this returns before any
	// resolver code runs — no request view is built, no map is consulted.
	if rc.IsIdentity() {
		// canonical_chain_key is populated on every route by a current controller.
		// The fallback covers a RouteConfig that reached the kernel without it (an
		// older controller, or a non-xDS load path) — for an identity route the
		// route key *is* the chain key, which is what the pre-resolution engine
		// assumed unconditionally.
		chainKey := rc.CanonicalChainKey
		if chainKey == "" {
			chainKey = routeKey
		}
		chain := s.kernel.GetPolicyChain(chainKey)
		if chain == nil {
			slog.DebugContext(ctx, "No policy chain found for route (new path)",
				"route", routeKey,
				"api_name", routeMetadata.APIName)
			*execCtx = nil
			return &routeMetadata, bindNoChain, nil
		}
		ec := s.newBoundExecutionContext(routeKey, rc, chainKey, chain, req, routeMetadata)
		// An identity route can still be an operation route (one A2A HTTP+JSON path per
		// operation), in which case the controller stamped how its response is delivered.
		// Empty for every kind shipping today, which resolves to Auto.
		ec.responseKind = effectiveResponseKind(rc, resolver.Resolution{})
		*execCtx = ec
		return &routeMetadata, bindReady, nil
	}

	res, ok := s.lookupResolver(rc.ResolverName)
	if !ok {
		// The route names a resolver this binary does not have. Deny — resolving by
		// identity instead would select the route-level chain for every logical
		// operation the route multiplexes.
		slog.ErrorContext(ctx, "Route names an unknown operation resolver",
			"route", routeKey, "resolver", rc.ResolverName)
		*execCtx = nil
		// No renderer: the protocol is exactly what is unknown here, so the response
		// is sterile-generic.
		return &routeMetadata, bindFailed, &resolutionDenial{
			resolverName: rc.ResolverName,
			failure:      &resolver.ResolutionError{Kind: resolver.FailureUnknownResolver},
		}
	}

	view := buildRequestView(routeKey, rc, req.GetRequestHeaders())
	reqs := res.Requirements()

	// A body-reading resolver normally defers to the request-body callback. It must
	// not defer when the request headers are end-of-stream: Envoy sends no
	// request-body callback for a bodyless request, so a pending request would wait
	// for a callback that cannot occur. Resolve (or deny) here instead — for a
	// JSON-RPC route an empty body is an invalid request anyway.
	if reqs.BufferBody && !req.GetRequestHeaders().GetEndOfStream() {
		ec := s.newBoundExecutionContext(routeKey, rc, "", nil, req, routeMetadata)
		ec.pending = &pendingResolution{route: rc, res: res, requirements: reqs, view: view}
		ec.requestView = view
		// Attached now, not after binding: a resolution failure at the body callback
		// still has to render in the transport's error shape, and by then there is no
		// resolver left to ask.
		ec.attachRenderers(res)
		*execCtx = ec
		slog.DebugContext(ctx, "[resolution] deferring chain selection to the request-body phase",
			"route", routeKey, "resolver", rc.ResolverName)
		return &routeMetadata, bindPending, nil
	}

	key, resolution, err := resolver.ResolveChainKey(s.resolvers, &rc.RouteResolution, view, s.hasPolicyChain)
	if err != nil {
		*execCtx = nil
		return &routeMetadata, bindFailed, newResolutionDenial(rc.ResolverName, res, view, resolution.ProtocolState,
			resolver.NormalizeResolutionError(err, resolution.ProtocolState))
	}

	// ResolveChainKey already probed for this chain; a nil here means it was removed
	// between the probe and this read, which renders the same as any other skew.
	chain := s.kernel.GetPolicyChain(key)
	if chain == nil {
		*execCtx = nil
		return &routeMetadata, bindFailed, newResolutionDenial(rc.ResolverName, res, view, resolution.ProtocolState,
			&resolver.ResolutionError{
				Kind:          resolver.FailureChainMissing,
				ProtocolState: resolution.ProtocolState,
				Cause:         errors.New("no policy chain for resolved key"),
			})
	}

	ec := s.newBoundExecutionContext(routeKey, rc, key, chain, req, routeMetadata)
	ec.requestView = view
	ec.protocolState = resolution.ProtocolState
	ec.responseKind = effectiveResponseKind(rc, resolution)
	ec.attachRenderers(res)
	*execCtx = ec
	return &routeMetadata, bindReady, nil
}

// newBoundExecutionContext allocates the execution context for a request and copies
// the route's pre-resolved metadata into it. chain may be nil for a route whose
// chain is selected later, at the request-body callback.
func (s *ExternalProcessorServer) newBoundExecutionContext(
	routeKey string,
	rc *RouteConfig,
	chainKey string,
	chain *registry.PolicyChain,
	req *extprocv3.ProcessingRequest,
	routeMetadata RouteMetadata,
) *PolicyExecutionContext {
	ec := newPolicyExecutionContext(s, routeKey, chain)
	ec.chainKey = chainKey
	ec.resolverName = rc.ResolverName
	ec.defaultUpstreamCluster = routeMetadata.DefaultUpstreamCluster
	ec.upstreamBasePath = routeMetadata.UpstreamBasePath
	ec.apiContext = routeMetadata.Context
	ec.upstreamDefinitionPaths = routeMetadata.UpstreamDefinitionPaths
	ec.defaultUpstream = routeMetadata.DefaultUpstream
	ec.buildRequestContexts(req.GetRequestHeaders(), routeMetadata)
	return ec
}

// lookupResolver reads the injected registry, treating a nil registry as
// identity-only so an incompletely wired server fails closed rather than panicking.
// hasPolicyChain is the chain-existence probe handed to resolver.ResolveChainKey, so
// the candidate ladder can ask whether a composed key has a chain without the resolver
// package reaching into the kernel's map or its lock.
func (s *ExternalProcessorServer) hasPolicyChain(chainKey string) bool {
	return s.kernel.GetPolicyChain(chainKey) != nil
}

func (s *ExternalProcessorServer) lookupResolver(name string) (resolver.OperationResolver, bool) {
	if s.resolvers == nil {
		return nil, false
	}
	return s.resolvers.Get(name)
}

// extractRouteKey extracts just the route key (xds.route_name) from the request attributes.
// This is a lightweight extraction that avoids parsing route metadata.
func (s *ExternalProcessorServer) extractRouteKey(req *extprocv3.ProcessingRequest) string {
	if req.Attributes == nil {
		return "default"
	}
	extProcAttrs, ok := req.Attributes[constants.ExtProcFilter]
	if !ok || extProcAttrs.Fields == nil {
		return "default"
	}
	if routeNameValue, ok := extProcAttrs.Fields["xds.route_name"]; ok {
		if stringValue := routeNameValue.GetStringValue(); stringValue != "" {
			return stringValue
		}
	}
	return "default"
}

// skipAllProcessing returns a response that skips all processing phases
func (s *ExternalProcessorServer) skipAllProcessing(routeMetadata RouteMetadata) *extprocv3.ProcessingResponse {
	// Build analytics metadata using route metadata even when skipping policy processing
	analyticsData := extractMetadataFromRouteMetadata(routeMetadata)

	// Build the analytics struct
	analyticsStruct, err := structpb.NewStruct(analyticsData)
	if err != nil {
		// Log error but continue
		slog.Warn("Failed to build analytics struct for skip processing", "error", err)
		analyticsStruct = &structpb.Struct{Fields: make(map[string]*structpb.Value)}
	}

	// Build dynamic metadata structure
	dynamicMetadata := buildDynamicMetadata(analyticsStruct, nil, nil)

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{},
		},
		ModeOverride: &extprocconfigv3.ProcessingMode{
			ResponseHeaderMode:  extprocconfigv3.ProcessingMode_SKIP,
			RequestTrailerMode:  extprocconfigv3.ProcessingMode_SKIP,
			ResponseTrailerMode: extprocconfigv3.ProcessingMode_SKIP,
			RequestBodyMode:     extprocconfigv3.ProcessingMode_NONE,
			ResponseBodyMode:    extprocconfigv3.ProcessingMode_NONE,
		},
		DynamicMetadata: dynamicMetadata,
	}
}

// RouteMetadata contains metadata about the route
type RouteMetadata struct {
	RouteName               string
	APIId                   string
	APIName                 string
	APIVersion              string
	Context                 string
	OperationPath           string
	Vhost                   string
	APIKind                 string
	TemplateHandle          string
	ProviderName            string
	ProjectID               string
	DefaultUpstreamCluster  string            // Default cluster for dynamic cluster routing
	UpstreamBasePath        string            // Base path for the upstream (e.g., /anything)
	UpstreamDefinitionPaths map[string]string // Maps upstream definition names to their URL base paths

	// DefaultUpstream is this route's own compiled-in upstream (cluster name, URL, base
	// path) — whichever slot it belongs to (main or sandbox). Always present; surfaced
	// to the policy engine as the route's single default upstream.
	DefaultUpstream *policyenginev1.UpstreamInfo
}

// generateRequestID generates a unique request identifier
func (s *ExternalProcessorServer) generateRequestID() string {
	return uuid.New().String()
}
