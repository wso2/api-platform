package kernel

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/envoy-policy-engine/sdk/core"
	"github.com/envoy-policy-engine/sdk/policies"
)

// ExternalProcessorServer implements the Envoy external processor service
// T059: ExternalProcessorServer gRPC service struct
type ExternalProcessorServer struct {
	extprocv3.UnimplementedExternalProcessorServer

	kernel *Kernel
	core   *core.Core
}

// NewExternalProcessorServer creates a new ExternalProcessorServer
func NewExternalProcessorServer(kernel *Kernel, coreEngine *core.Core) *ExternalProcessorServer {
	return &ExternalProcessorServer{
		kernel: kernel,
		core:   coreEngine,
	}
}

// Process implements the bidirectional streaming RPC handler
// T060: Process(stream) bidirectional streaming RPC handler
func (s *ExternalProcessorServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()

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

		// Process the request based on phase
		resp, err := s.processRequest(ctx, req)
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

// processRequest routes processing to the appropriate handler based on request phase
func (s *ExternalProcessorServer) processRequest(ctx context.Context, req *extprocv3.ProcessingRequest) (*extprocv3.ProcessingResponse, error) {
	switch v := req.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		// T062: ProcessRequest phase handler (request headers)
		return s.handleRequestHeaders(ctx, v.RequestHeaders)

	case *extprocv3.ProcessingRequest_RequestBody:
		// Handle request body if needed (for body-requiring policies)
		return s.handleRequestBody(ctx, v.RequestBody)

	case *extprocv3.ProcessingRequest_ResponseHeaders:
		// T063: ProcessResponse phase handler (response headers)
		return s.handleResponseHeaders(ctx, v.ResponseHeaders)

	case *extprocv3.ProcessingRequest_ResponseBody:
		// Handle response body if needed
		return s.handleResponseBody(ctx, v.ResponseBody)

	default:
		slog.WarnContext(ctx, "Unknown request type", "type", fmt.Sprintf("%T", v))
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extprocv3.ImmediateResponse{
					Status: &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError},
				},
			},
		}, nil
	}
}

// handleRequestHeaders processes request headers phase
// T062: ProcessRequest phase handler implementation
func (s *ExternalProcessorServer) handleRequestHeaders(ctx context.Context, headers *extprocv3.HttpHeaders) (*extprocv3.ProcessingResponse, error) {
	// T061: Extract metadata key from request
	metadataKey := s.extractMetadataKey(headers)

	// Get policy chain for this route
	chain, err := s.kernel.GetPolicyChainForKey(metadataKey)
	if err != nil {
		slog.WarnContext(ctx, "No policy chain found for route", "route", metadataKey, "error", err)
		// No policy chain = allow request to proceed unmodified
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extprocv3.HeadersResponse{},
			},
		}, nil
	}

	// T064: Generate request ID
	requestID := s.generateRequestID()

	// Build RequestContext from Envoy headers
	reqCtx := s.buildRequestContext(headers, requestID, chain)

	// Execute request policy chain
	execResult, err := s.core.ExecuteRequestPolicies(
		chain.RequestPolicies,
		reqCtx,
		chain.RequestPolicySpecs,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Error executing request policies", "error", err)
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extprocv3.ImmediateResponse{
					Status: &typev3.HttpStatus{Code: typev3.StatusCode_InternalServerError},
					Body:   []byte(fmt.Sprintf("Policy execution error: %v", err)),
				},
			},
		}, nil
	}

	// Store context for response phase
	s.kernel.storeContextForResponse(requestID, reqCtx, chain)

	// Translate execution result to ext_proc response
	return TranslateRequestActions(execResult, chain), nil
}

// handleRequestBody processes request body phase
func (s *ExternalProcessorServer) handleRequestBody(ctx context.Context, body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	// For now, allow body through unmodified
	// Body modification policies would be implemented here
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// handleResponseHeaders processes response headers phase
// T063: ProcessResponse phase handler implementation
func (s *ExternalProcessorServer) handleResponseHeaders(ctx context.Context, headers *extprocv3.HttpHeaders) (*extprocv3.ProcessingResponse, error) {
	// Extract request ID to retrieve stored context
	requestID := s.extractRequestID(headers)

	// Retrieve stored context from request phase
	storedCtx, chain, err := s.kernel.getStoredContext(requestID)
	if err != nil {
		slog.WarnContext(ctx, "No stored context for request", "request_id", requestID, "error", err)
		// No stored context = allow response through unmodified
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extprocv3.HeadersResponse{},
			},
		}, nil
	}

	// Clean up stored context
	defer s.kernel.removeStoredContext(requestID)

	// Build ResponseContext
	respCtx := s.buildResponseContext(headers, storedCtx)

	// Execute response policy chain
	execResult, err := s.core.ExecuteResponsePolicies(
		chain.ResponsePolicies,
		respCtx,
		chain.ResponsePolicySpecs,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Error executing response policies", "error", err)
		// Allow response through unmodified on error
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extprocv3.HeadersResponse{},
			},
		}, nil
	}

	// Translate execution result to ext_proc response
	return TranslateResponseActions(execResult), nil
}

// handleResponseBody processes response body phase
func (s *ExternalProcessorServer) handleResponseBody(ctx context.Context, body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	// For now, allow body through unmodified
	// Body modification policies would be implemented here
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// extractMetadataKey extracts the route identifier from Envoy metadata
// T061: extractMetadataKey implementation
func (s *ExternalProcessorServer) extractMetadataKey(headers *extprocv3.HttpHeaders) string {
	// Look for x-route-key header (set by Envoy via header mutation)
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			if header.Key == "x-route-key" {
				return string(header.RawValue)
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

// extractRequestID extracts request ID from response headers
func (s *ExternalProcessorServer) extractRequestID(headers *extprocv3.HttpHeaders) string {
	// Look for x-request-id header (we'll inject this during request phase)
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			if header.Key == "x-request-id" {
				return string(header.RawValue)
			}
		}
	}
	return ""
}

// buildRequestContext converts Envoy headers to RequestContext
func (s *ExternalProcessorServer) buildRequestContext(headers *extprocv3.HttpHeaders, requestID string, chain *core.PolicyChain) *policies.RequestContext {
	ctx := &policies.RequestContext{
		Headers:   make(map[string][]string),
		RequestID: requestID,
		Metadata:  chain.Metadata, // Share chain metadata
	}

	// Extract headers
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			ctx.Headers[key] = append(ctx.Headers[key], value)

			// Extract path and method from pseudo-headers
			if key == ":path" {
				ctx.Path = value
			} else if key == ":method" {
				ctx.Method = value
			}
		}
	}

	return ctx
}

// buildResponseContext converts Envoy response headers and stored request context
func (s *ExternalProcessorServer) buildResponseContext(headers *extprocv3.HttpHeaders, reqCtx *policies.RequestContext) *policies.ResponseContext {
	ctx := &policies.ResponseContext{
		RequestHeaders: reqCtx.Headers,
		RequestBody:    reqCtx.Body,
		RequestPath:    reqCtx.Path,
		RequestMethod:  reqCtx.Method,
		RequestID:      reqCtx.RequestID,
		ResponseHeaders: make(map[string][]string),
		Metadata:       reqCtx.Metadata, // Share same metadata reference
	}

	// Extract response headers
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			ctx.ResponseHeaders[key] = append(ctx.ResponseHeaders[key], value)

			// Extract status from pseudo-header
			if key == ":status" {
				// Convert status string to int
				var status int
				fmt.Sscanf(value, "%d", &status)
				ctx.ResponseStatus = status
			}
		}
	}

	return ctx
}
