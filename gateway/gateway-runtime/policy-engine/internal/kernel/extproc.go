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

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
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
		routeMetadata := s.extractRouteMetadata(req)
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

		// If no execution context (no policy chain), skip processing
		if *execCtx == nil {
			if span.IsRecording() {
				span.SetAttributes(attribute.Int(constants.AttrPolicyCount, 0))
			}
			metrics.RouteLookupFailuresTotal.Inc()
			metrics.RequestDurationSeconds.WithLabelValues("request_headers", rm.RouteName).Observe(time.Since(startTime).Seconds())
			return s.skipAllProcessing(routeMetadata), nil
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
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extprocv3.ImmediateResponse{
					Status: &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError},
				},
			},
		}, nil
	}
}

// initializeExecutionContext sets up the execution context for a request by retrieving the policy chain
// T061: Extract metadata key from request and get policy chain
// T064: Generate request ID
func (s *ExternalProcessorServer) initializeExecutionContext(ctx context.Context, req *extprocv3.ProcessingRequest, execCtx **PolicyExecutionContext) *RouteMetadata {
	// Extract route metadata from request
	routeMetadata := s.extractRouteMetadata(req)

	// Get policy chain for this route using route name
	chain := s.kernel.GetPolicyChainForKey(routeMetadata.RouteName)
	if chain == nil {
		slog.InfoContext(ctx, "No policy chain found for route, skipping all processing",
			"route", routeMetadata.RouteName,
			"api_id", routeMetadata.APIId,
			"api_name", routeMetadata.APIName,
			"api_version", routeMetadata.APIVersion,
			"context", routeMetadata.Context)
		*execCtx = nil
		return &routeMetadata
	}

	// Create execution context for this request-response lifecycle
	*execCtx = newPolicyExecutionContext(s, routeMetadata.RouteName, chain)

	// Build request context from Envoy headers with route metadata
	// Request ID will be extracted from x-request-id header or generated if not present
	(*execCtx).buildRequestContext(req.GetRequestHeaders(), routeMetadata)
	return &routeMetadata
}

// skipAllProcessing returns a response that skips all processing phases
func (s *ExternalProcessorServer) skipAllProcessing(routeMetadata RouteMetadata) *extprocv3.ProcessingResponse {
	// Build analytics metadata using route metadataeven when skipping policy processing
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
	RouteName      string
	APIId          string
	APIName        string
	APIVersion     string
	Context        string
	OperationPath  string
	Vhost          string
	APIKind        string
	TemplateHandle string
	ProviderName   string
	ProjectID      string
}

// extractRouteMetadata extracts the route metadata from Envoy metadata
func (s *ExternalProcessorServer) extractRouteMetadata(req *extprocv3.ProcessingRequest) RouteMetadata {
	metadata := RouteMetadata{}

	if req.Attributes == nil {
		return metadata
	}

	extProcAttrs, ok := req.Attributes[constants.ExtProcFilter]
	if !ok || extProcAttrs.Fields == nil {
		return metadata
	}

	// Extract route name from xds.route_name
	if routeNameValue, ok := extProcAttrs.Fields["xds.route_name"]; ok {
		if stringValue := routeNameValue.GetStringValue(); stringValue != "" {
			metadata.RouteName = stringValue
		}
	}

	// Extract API metadata from xds.route_metadata
	if routeMetadataValue, ok := extProcAttrs.Fields["xds.route_metadata"]; ok {
		if metadataStr := routeMetadataValue.GetStringValue(); metadataStr != "" {
			// Parse the protobuf text format string using prototext
			var envoyMetadata core.Metadata
			if err := prototext.Unmarshal([]byte(metadataStr), &envoyMetadata); err != nil {
				slog.Warn("Failed to unmarshal route metadata", "error", err)
			} else {
				// Extract fields from "wso2.route" filter metadata
				if routeStruct, ok := envoyMetadata.FilterMetadata["wso2.route"]; ok && routeStruct.Fields != nil {
					if apiIdValue, ok := routeStruct.Fields["api_id"]; ok {
						metadata.APIId = apiIdValue.GetStringValue()
					}
					if apiNameValue, ok := routeStruct.Fields["api_name"]; ok {
						metadata.APIName = apiNameValue.GetStringValue()
					}
					if apiVersionValue, ok := routeStruct.Fields["api_version"]; ok {
						metadata.APIVersion = apiVersionValue.GetStringValue()
					}
					if apiContextValue, ok := routeStruct.Fields["api_context"]; ok {
						metadata.Context = apiContextValue.GetStringValue()
					}
					if operationPath, ok := routeStruct.Fields["path"]; ok {
						metadata.OperationPath = operationPath.GetStringValue()
					}
					if vhostValue, ok := routeStruct.Fields["vhost"]; ok {
						metadata.Vhost = vhostValue.GetStringValue()
					}
					if originalAPIKindValue, ok := routeStruct.Fields["api_kind"]; ok {
						metadata.APIKind = originalAPIKindValue.GetStringValue()
					}
					if templateHandleValue, ok := routeStruct.Fields["template_handle"]; ok {
						metadata.TemplateHandle = templateHandleValue.GetStringValue()
					}
					if providerNameValue, ok := routeStruct.Fields["provider_name"]; ok {
						metadata.ProviderName = providerNameValue.GetStringValue()
					}
					if projectIDValue, ok := routeStruct.Fields["project_id"]; ok {
						metadata.ProjectID = projectIDValue.GetStringValue()
					}
				}
			}
		}
	}

	// If no route name found, use default
	if metadata.RouteName == "" {
		metadata.RouteName = "default"
	}

	return metadata
}

// generateRequestID generates a unique request identifier
func (s *ExternalProcessorServer) generateRequestID() string {
	return uuid.New().String()
}
