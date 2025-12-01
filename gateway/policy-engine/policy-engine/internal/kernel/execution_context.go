package kernel

import (
	"context"
	"fmt"
	"log/slog"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"

	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/policy-engine/sdk/policy/v1alpha"
)

// PolicyExecutionContext manages the lifecycle of a single request through the policy chain.
// This context is created when a request arrives and lives until the response is completed.
// It encapsulates all state needed for processing both request and response phases.
type PolicyExecutionContext struct {
	// Request context that carries request data and metadata
	requestContext *policy.RequestContext

	// Response context that carries response data and metadata
	responseContext *policy.ResponseContext

	// Policy chain for this request
	policyChain *registry.PolicyChain

	// Route key (metadata key) for this request
	routeKey string

	// Request ID for correlation
	requestID string

	// Reference to server components
	server *ExternalProcessorServer
}

// newPolicyExecutionContext creates a new execution context for a request
func newPolicyExecutionContext(
	server *ExternalProcessorServer,
	requestID string,
	routeKey string,
	chain *registry.PolicyChain,
) *PolicyExecutionContext {
	return &PolicyExecutionContext{
		server:      server,
		requestID:   requestID,
		routeKey:    routeKey,
		policyChain: chain,
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

// getModeOverride returns the ProcessingMode override for this execution context
// This tells Envoy which phases to process based on policy chain requirements
func (ec *PolicyExecutionContext) getModeOverride() *extprocconfigv3.ProcessingMode {
	mode := &extprocconfigv3.ProcessingMode{}

	// Set request body mode based on policy chain requirements
	if ec.policyChain.RequiresRequestBody {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_BUFFERED
	} else {
		mode.RequestBodyMode = extprocconfigv3.ProcessingMode_NONE
	}

	// Set response header mode based on whether any policies process response headers
	hasResponseHeaderProcessing := false
	for _, pol := range ec.policyChain.Policies {
		if pol.Mode().ResponseHeaderMode == policy.HeaderModeProcess {
			hasResponseHeaderProcessing = true
			break
		}
	}

	if hasResponseHeaderProcessing {
		mode.ResponseHeaderMode = extprocconfigv3.ProcessingMode_SEND
	} else {
		mode.ResponseHeaderMode = extprocconfigv3.ProcessingMode_SKIP
	}

	// Set response body mode based on policy chain requirements
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

// processRequestHeaders processes request headers phase
func (ec *PolicyExecutionContext) processRequestHeaders(
	ctx context.Context,
) (*extprocv3.ProcessingResponse, error) {
	// Check if this is end of stream (no body coming)
	endOfStream := ec.requestContext.Body != nil && ec.requestContext.Body.EndOfStream

	// If policy chain requires request body AND body is coming, skip processing headers separately
	// Headers and body will be processed together in processRequestBody phase
	// However, if EndOfStream is true, there's no body coming, so process immediately
	if ec.policyChain.RequiresRequestBody && !endOfStream {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extprocv3.HeadersResponse{},
			},
			ModeOverride: ec.getModeOverride(),
		}, nil
	}

	// Execute request policy chain with headers only
	execResult, err := ec.server.executor.ExecuteRequestPolicies(
		ec.policyChain.Policies,
		ec.requestContext,
		ec.policyChain.PolicySpecs,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "request_headers"), nil
	}

	// Translate execution result to ext_proc response
	return TranslateRequestActions(execResult, ec.policyChain, ec), nil
}

// processRequestBody processes request body phase
func (ec *PolicyExecutionContext) processRequestBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	// If policy chain requires request body, execute policies with both headers and body
	if ec.policyChain.RequiresRequestBody {
		// Update request context with body data
		ec.requestContext.Body = &policy.Body{
			Content:     body.Body,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		// Execute request policy chain with headers and body
		execResult, err := ec.server.executor.ExecuteRequestPolicies(
			ec.policyChain.Policies,
			ec.requestContext,
			ec.policyChain.PolicySpecs,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "request_body"), nil
		}

		// Check if policies short-circuited with immediate response
		if execResult.ShortCircuited && execResult.FinalAction != nil {
			if immediateResp, ok := execResult.FinalAction.(policy.ImmediateResponse); ok {
				return &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_ImmediateResponse{
						ImmediateResponse: &extprocv3.ImmediateResponse{
							Status: &typev3.HttpStatus{
								Code: typev3.StatusCode(immediateResp.StatusCode),
							},
							Headers: buildHeaderValueOptions(immediateResp.Headers),
							Body:    immediateResp.Body,
						},
					},
				}, nil
			}
		}

		// Normal case: translate modifications to body response
		headerMutation, bodyMutation := buildRequestMutations(execResult)
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestBody{
				RequestBody: &extprocv3.BodyResponse{
					Response: &extprocv3.CommonResponse{
						HeaderMutation: headerMutation,
						BodyMutation:   bodyMutation,
					},
				},
			},
		}, nil
	}

	// If policies don't require body, just allow it through unmodified
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// processResponseHeaders processes response headers phase
func (ec *PolicyExecutionContext) processResponseHeaders(
	ctx context.Context,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, error) {
	// Build ResponseContext from stored request context and response headers
	ec.responseContext = ec.buildResponseContext(headers)

	// If policy chain requires response body AND body is coming, skip processing headers separately
	// Headers and body will be processed together in processResponseBody phase
	// However, if EndOfStream is true, there's no body coming, so process immediately
	if ec.policyChain.RequiresResponseBody && !headers.EndOfStream {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extprocv3.HeadersResponse{},
			},
		}, nil
	}

	// Execute response policy chain with headers only
	execResult, err := ec.server.executor.ExecuteResponsePolicies(
		ec.policyChain.Policies,
		ec.responseContext,
		ec.policyChain.PolicySpecs,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "response_headers"), nil
	}

	// Translate execution result to ext_proc response
	return TranslateResponseActions(execResult), nil
}

// processResponseBody processes response body phase
func (ec *PolicyExecutionContext) processResponseBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	// If policy chain requires response body, execute policies with both headers and body
	if ec.policyChain.RequiresResponseBody {
		// Update response context with body data
		ec.responseContext.ResponseBody = &policy.Body{
			Content:     body.Body,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		// Execute response policy chain with headers and body
		execResult, err := ec.server.executor.ExecuteResponsePolicies(
			ec.policyChain.Policies,
			ec.responseContext,
			ec.policyChain.PolicySpecs,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "response_body"), nil
		}

		// Normal case: translate modifications to body response
		headerMutation, bodyMutation := buildResponseMutations(execResult)
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseBody{
				ResponseBody: &extprocv3.BodyResponse{
					Response: &extprocv3.CommonResponse{
						HeaderMutation: headerMutation,
						BodyMutation:   bodyMutation,
					},
				},
			},
		}, nil
	}

	// If policies don't require body, just allow it through unmodified
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// buildRequestContext converts Envoy headers to RequestContext
func (ec *PolicyExecutionContext) buildRequestContext(headers *extprocv3.HttpHeaders) *policy.RequestContext {
	// Create shared context that will persist across request/response phases
	sharedCtx := &policy.SharedContext{
		RequestID: ec.requestID,
		Metadata:  ec.policyChain.Metadata, // Share chain metadata
	}

	// Create headers map for internal manipulation
	headersMap := make(map[string][]string)

	// Extract headers
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			headersMap[key] = append(headersMap[key], value)
		}
	}

	// Build context with Headers wrapper
	ctx := &policy.RequestContext{
		SharedContext: sharedCtx,
		Headers:       policy.NewHeaders(headersMap),
	}

	// Extract path and method from pseudo-headers
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)

			if key == ":path" {
				ctx.Path = value
			} else if key == ":method" {
				ctx.Method = value
			}
		}
	}

	// Initialize Body with EndOfStream from headers (for requests without body)
	// This will be overwritten if processRequestBody is called
	if headers.EndOfStream {
		ctx.Body = &policy.Body{
			Content:     nil,
			EndOfStream: true,
			Present:     false,
		}
	}

	return ctx
}

// buildResponseContext converts Envoy response headers and stored request context
func (ec *PolicyExecutionContext) buildResponseContext(headers *extprocv3.HttpHeaders) *policy.ResponseContext {
	// Create response headers map for internal manipulation
	responseHeadersMap := make(map[string][]string)
	var responseStatus int

	// Extract response headers
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			responseHeadersMap[key] = append(responseHeadersMap[key], value)

			// Extract status from pseudo-header
			if key == ":status" {
				// Convert status string to int
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

	ctx := &policy.ResponseContext{
		SharedContext:   ec.requestContext.SharedContext, // Reuse same shared context from request phase
		RequestHeaders:  ec.requestContext.Headers,
		RequestBody:     ec.requestContext.Body,
		RequestPath:     ec.requestContext.Path,
		RequestMethod:   ec.requestContext.Method,
		ResponseHeaders: policy.NewHeaders(responseHeadersMap),
		ResponseStatus:  responseStatus,
	}

	// Initialize ResponseBody with EndOfStream from headers (for responses without body)
	// This will be overwritten if processResponseBody is called
	if headers.EndOfStream {
		ctx.ResponseBody = &policy.Body{
			Content:     nil,
			EndOfStream: true,
			Present:     false,
		}
	}

	return ctx
}
