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

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonerrors "github.com/wso2/api-platform/common/errors"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
)

// errNoPolicyChain is returned when no policy chain is found for a route key.
var errNoPolicyChain = errors.New("no policy chain found for route")

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
		rm, err := s.initializeExecutionContext(req, execCtx)
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

		// If no policy chain found, reject the request with 500
		if err != nil {
			slog.ErrorContext(ctx, "No policy chain found for route, rejecting request",
				"route", rm.RouteName,
				"api_name", rm.APIName,
				"api_version", rm.APIVersion)
			if span.IsRecording() {
				span.SetAttributes(attribute.Int(constants.AttrPolicyCount, 0))
			}
			metrics.RouteLookupFailuresTotal.Inc()
			metrics.RequestDurationSeconds.WithLabelValues("request_headers", rm.RouteName).Observe(time.Since(startTime).Seconds())
			// TODO: pass correlation ID once correlation ID propagation is implemented
			return buildImmediateErrorResponse(commonerrors.ErrCodeRouteNotFound, ""), nil
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
		return resp, err

	default:
		slog.WarnContext(ctx, "Unknown request type", "type", fmt.Sprintf("%T", req.Request))
		metrics.RequestErrorsTotal.WithLabelValues("unknown", "unknown_type", "unknown").Inc()
		return buildImmediateErrorResponse(commonerrors.ErrCodeUnknownRequestType, ""), nil
	}
}

// initializeExecutionContext sets up the execution context for a request by retrieving the policy chain.
// Uses RouteConfigs (metadata pre-loaded via xDS) and PolicyChains.
func (s *ExternalProcessorServer) initializeExecutionContext(req *extprocv3.ProcessingRequest, execCtx **PolicyExecutionContext) (*RouteMetadata, error) {
	// Extract route key from Envoy attributes (just xds.route_name, lightweight)
	routeKey := s.extractRouteKey(req)

	rc := s.kernel.GetRouteConfig(routeKey)
	if rc == nil {
		return &RouteMetadata{RouteName: routeKey}, errNoPolicyChain
	}

	// Metadata is pre-populated from xDS — no request-time parsing needed
	routeMetadata := rc.Metadata
	routeMetadata.RouteName = routeKey
	routeMetadata.DefaultUpstreamCluster = rc.DefaultUpstreamCluster
	routeMetadata.UpstreamBasePath = rc.UpstreamBasePath
	routeMetadata.UpstreamDefinitionPaths = rc.UpstreamDefinitionPaths

	chain := s.kernel.GetPolicyChain(routeKey)
	if chain == nil {
		return &routeMetadata, errNoPolicyChain
	}

	*execCtx = newPolicyExecutionContext(s, routeKey, chain)
	(*execCtx).defaultUpstreamCluster = rc.DefaultUpstreamCluster
	(*execCtx).upstreamBasePath = rc.UpstreamBasePath
	(*execCtx).apiContext = routeMetadata.Context
	(*execCtx).upstreamDefinitionPaths = rc.UpstreamDefinitionPaths
	(*execCtx).buildRequestContext(req.GetRequestHeaders(), routeMetadata)
	return &routeMetadata, nil
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

// buildImmediateErrorResponse constructs a ProcessingResponse that immediately
// terminates the request with the HTTP status and JSON body from the given ErrorCode.
// If correlationID is empty it is omitted from the response body.
func buildImmediateErrorResponse(errCode commonerrors.ErrorCode, correlationID string) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status:  &typev3.HttpStatus{Code: typev3.StatusCode(errCode.HTTPStatus)},
				Headers: buildHeaderValueOptions(map[string]string{"content-type": "application/json"}),
				Body:    commonerrors.BuildErrorBody(errCode, correlationID),
			},
		},
	}
}
