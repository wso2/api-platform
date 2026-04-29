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
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
)

// ExternalProcessorServer implements the Envoy external processor service
// T059: ExternalProcessorServer gRPC service struct
type ExternalProcessorServer struct {
	extprocv3.UnimplementedExternalProcessorServer

	kernel   *Kernel
	executor *executor.ChainExecutor
	tracer   trace.Tracer
}

// NewExternalProcessorServer creates a new ExternalProcessorServer
func NewExternalProcessorServer(kernel *Kernel, chainExecutor *executor.ChainExecutor, tracingConfig config.TracingConfig, tracingServiceName string) *ExternalProcessorServer {
	// Initialize tracer once - will be NoOp if tracing is disabled
	serviceName := tracingServiceName
	if serviceName == "" {
		serviceName = "policy-engine"
	}

	return &ExternalProcessorServer{
		kernel:   kernel,
		executor: chainExecutor,
		tracer:   otel.Tracer(serviceName),
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
		rm := s.initializeExecutionContext(ctx, req, execCtx)
		if parentSpan.IsRecording() {
			parentSpan.SetAttributes(
				attribute.String(constants.AttrRouteName, rm.RouteName),
				attribute.String(constants.AttrAPIName, rm.APIName),
				attribute.String(constants.AttrAPIVersion, rm.APIVersion),
				attribute.String(constants.AttrAPIContext, rm.Context),
				attribute.String(constants.AttrOperationPath, rm.OperationPath),
			)
		}

		// Track request metrics
		metrics.RequestsTotal.WithLabelValues("request_headers", rm.RouteName, rm.APIName, rm.APIVersion).Inc()

		// If no execution context (no policy chain found), return 500
		if *execCtx == nil {
			if span.IsRecording() {
				span.SetAttributes(attribute.Int(constants.AttrPolicyCount, 0))
			}
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
		}
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.DebugContext(ctx, "ext_proc response", "phase", "response_body", "resp", prototext.Format(resp))
		}
		return resp, err

	default:
		slog.WarnContext(ctx, "Unknown request type", "type", fmt.Sprintf("%T", req.Request))
		metrics.RequestErrorsTotal.WithLabelValues("unknown", "unknown_type", "unknown").Inc()
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extprocv3.ImmediateResponse{
					Status: &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError},
				},
			},
		}, nil
	}
}

// initializeExecutionContext sets up the execution context for a request by retrieving the policy chain.
// Route metadata is pre-loaded via xDS RouteConfigs — no request-time parsing needed.
func (s *ExternalProcessorServer) initializeExecutionContext(ctx context.Context, req *extprocv3.ProcessingRequest, execCtx **PolicyExecutionContext) *RouteMetadata {
	// Extract route key from Envoy attributes (just xds.route_name, lightweight)
	routeKey := s.extractRouteKey(req)

	slog.DebugContext(ctx, "initializeExecutionContext: looking up route",
		"route_key", routeKey)

	// Try new path: RouteConfigs + PolicyChains
	if rc := s.kernel.GetRouteConfig(routeKey); rc != nil {
		// Metadata is pre-populated from xDS — no request-time parsing needed
		routeMetadata := rc.Metadata
		routeMetadata.RouteName = routeKey

		// Resolve policy chain key (route-key resolver: policyChainKey = routeKey)
		policyChainKey := routeKey // For route-key resolver, this is always the same

		chain := s.kernel.GetPolicyChain(policyChainKey)
		if chain == nil {
			slog.DebugContext(ctx, "No policy chain found for route (new path)",
				"route", routeKey,
				"api_name", routeMetadata.APIName)
			*execCtx = nil
			return &routeMetadata
		}

		*execCtx = newPolicyExecutionContext(s, routeKey, chain)
		(*execCtx).defaultUpstreamCluster = routeMetadata.DefaultUpstreamCluster
		(*execCtx).upstreamBasePath = routeMetadata.UpstreamBasePath
		(*execCtx).apiContext = routeMetadata.Context
		(*execCtx).upstreamDefinitionPaths = routeMetadata.UpstreamDefinitionPaths
		(*execCtx).buildRequestContexts(req.GetRequestHeaders(), routeMetadata)
		return &routeMetadata
	}

	// No RouteConfig found for this route key — return empty metadata with nil exec context
	slog.DebugContext(ctx, "initializeExecutionContext: RouteConfig not found",
		"route_key", routeKey)
	routeMetadata := RouteMetadata{RouteName: routeKey}
	*execCtx = nil
	return &routeMetadata
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
	UpstreamDefinitionPaths map[string]string // Maps upstream definition names to their URL paths
}

// generateRequestID generates a unique request identifier
func (s *ExternalProcessorServer) generateRequestID() string {
	return uuid.New().String()
}
