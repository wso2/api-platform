package kernel

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/policy-engine/policy-engine/internal/executor"
)

// ExternalProcessorServer implements the Envoy external processor service
// T059: ExternalProcessorServer gRPC service struct
type ExternalProcessorServer struct {
	extprocv3.UnimplementedExternalProcessorServer

	kernel   *Kernel
	executor *executor.ChainExecutor
}

// NewExternalProcessorServer creates a new ExternalProcessorServer
func NewExternalProcessorServer(kernel *Kernel, chainExecutor *executor.ChainExecutor) *ExternalProcessorServer {
	return &ExternalProcessorServer{
		kernel:   kernel,
		executor: chainExecutor,
	}
}

// Process implements the bidirectional streaming RPC handler
// T060: Process(stream) bidirectional streaming RPC handler
func (s *ExternalProcessorServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()

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
			slog.ErrorContext(ctx, "Error receiving from stream", "error", err)
			return status.Errorf(codes.Unknown, "failed to receive request: %v", err)
		}

		// Handle the request based on phase
		resp, err := s.handleProcessingPhase(ctx, req, &execCtx)
		if err != nil {
			slog.ErrorContext(ctx, "Error processing request", "error", err)
			return err
		}

		// Send response back to Envoy
		if err := stream.Send(resp); err != nil {
			slog.ErrorContext(ctx, "Error sending response", "error", err)
			return status.Errorf(codes.Unknown, "failed to send response: %v", err)
		}
	}
}

// handleProcessingPhase routes processing to the appropriate phase handler
func (s *ExternalProcessorServer) handleProcessingPhase(ctx context.Context, req *extprocv3.ProcessingRequest, execCtx **PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	switch req.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		// Initialize execution context for this request
		s.initializeExecutionContext(ctx, req, execCtx)

		// If no execution context (no policy chain), skip processing
		if *execCtx == nil {
			return s.skipAllProcessing(), nil
		}

		// Process request headers
		return (*execCtx).processRequestHeaders(ctx)

	case *extprocv3.ProcessingRequest_RequestBody:
		if *execCtx == nil {
			slog.WarnContext(ctx, "Request body received before request headers")
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestBody{
					RequestBody: &extprocv3.BodyResponse{},
				},
			}, nil
		}
		return (*execCtx).processRequestBody(ctx, req.GetRequestBody())

	case *extprocv3.ProcessingRequest_ResponseHeaders:
		if *execCtx == nil {
			slog.WarnContext(ctx, "Response headers received without execution context")
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extprocv3.HeadersResponse{},
				},
			}, nil
		}
		return (*execCtx).processResponseHeaders(ctx, req.GetResponseHeaders())

	case *extprocv3.ProcessingRequest_ResponseBody:
		if *execCtx == nil {
			slog.WarnContext(ctx, "Response body received without execution context")
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseBody{
					ResponseBody: &extprocv3.BodyResponse{},
				},
			}, nil
		}
		return (*execCtx).processResponseBody(ctx, req.GetResponseBody())

	default:
		slog.WarnContext(ctx, "Unknown request type", "type", fmt.Sprintf("%T", req.Request))
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
func (s *ExternalProcessorServer) initializeExecutionContext(ctx context.Context, req *extprocv3.ProcessingRequest, execCtx **PolicyExecutionContext) {
	// Extract metadata key from request
	metadataKey := s.extractMetadataKey(req)

	// Get policy chain for this route
	chain := s.kernel.GetPolicyChainForKey(metadataKey)
	if chain == nil {
		slog.InfoContext(ctx, "No policy chain found for route, skipping all processing", "route", metadataKey)
		*execCtx = nil
		return
	}

	// Generate request ID
	requestID := s.generateRequestID()

	// Create execution context for this request-response lifecycle
	*execCtx = newPolicyExecutionContext(s, requestID, metadataKey, chain)

	// Build request context from Envoy headers
	(*execCtx).requestContext = (*execCtx).buildRequestContext(req.GetRequestHeaders())
}

// skipAllProcessing returns a response that skips all processing phases
func (s *ExternalProcessorServer) skipAllProcessing() *extprocv3.ProcessingResponse {
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
	}
}

// extractMetadataKey extracts the route identifier from Envoy metadata
// T061: extractMetadataKey implementation
func (s *ExternalProcessorServer) extractMetadataKey(req *extprocv3.ProcessingRequest) string {
	// Extract route name from Envoy attributes
	// Path: req.Attributes["envoy.filters.http.ext_proc"].Fields["xds.route_name"]
	if req.Attributes != nil {
		if extProcAttrs, ok := req.Attributes["envoy.filters.http.ext_proc"]; ok {
			if extProcAttrs.Fields != nil {
				if routeNameValue, ok := extProcAttrs.Fields["xds.route_name"]; ok {
					if stringValue := routeNameValue.GetStringValue(); stringValue != "" {
						return stringValue
					}
				}
			}
		}
	}

	// Default route if no metadata key found
	return "default"
}

// generateRequestID generates a unique request identifier
// T064: Request ID generation implementation
func (s *ExternalProcessorServer) generateRequestID() string {
	return uuid.New().String()
}
