package kernel

import (
	"context"
	"fmt"
	"log/slog"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/policy-engine/sdk/core"
	"github.com/policy-engine/sdk/policies"
)

// PolicyExecutionContext manages the lifecycle of a single request through the policy chain.
// This context is created when a request arrives and lives until the response is completed.
// It encapsulates all state needed for processing both request and response phases.
type PolicyExecutionContext struct {
	// Request context that carries request data and metadata
	requestContext *policies.RequestContext

	// Response context that carries response data and metadata
	responseContext *policies.ResponseContext

	// Policy chain for this request
	policyChain *core.PolicyChain

	// Request ID for correlation
	requestID string

	// Reference to server components
	server *ExternalProcessorServer
}

// newPolicyExecutionContext creates a new execution context for a request
func newPolicyExecutionContext(
	server *ExternalProcessorServer,
	requestID string,
	chain *core.PolicyChain,
) *PolicyExecutionContext {
	return &PolicyExecutionContext{
		server:      server,
		requestID:   requestID,
		policyChain: chain,
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

	// Set response header mode based on whether any response policies exist
	// (all response policies process headers)
	if len(ec.policyChain.ResponsePolicies) > 0 {
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
	// If policy chain requires request body, skip processing headers separately
	// Headers and body will be processed together in processRequestBody phase
	if ec.policyChain.RequiresRequestBody {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extprocv3.HeadersResponse{},
			},
			ModeOverride: ec.getModeOverride(),
		}, nil
	}

	// Execute request policy chain with headers only
	execResult, err := ec.server.core.ExecuteRequestPolicies(
		ec.policyChain.RequestPolicies,
		ec.requestContext,
		ec.policyChain.RequestPolicySpecs,
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
		ec.requestContext.Body = &policies.Body{
			Content:     body.Body,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		// Execute request policy chain with headers and body
		execResult, err := ec.server.core.ExecuteRequestPolicies(
			ec.policyChain.RequestPolicies,
			ec.requestContext,
			ec.policyChain.RequestPolicySpecs,
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

		// Check if policies short-circuited with immediate response
		if execResult.ShortCircuited && execResult.FinalAction != nil {
			if immediateResp, ok := execResult.FinalAction.Action.(policies.ImmediateResponse); ok {
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

	// If policy chain requires response body, skip processing headers separately
	// Headers and body will be processed together in processResponseBody phase
	if ec.policyChain.RequiresResponseBody {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extprocv3.HeadersResponse{},
			},
		}, nil
	}

	// Execute response policy chain with headers only
	execResult, err := ec.server.core.ExecuteResponsePolicies(
		ec.policyChain.ResponsePolicies,
		ec.responseContext,
		ec.policyChain.ResponsePolicySpecs,
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

// processResponseBody processes response body phase
func (ec *PolicyExecutionContext) processResponseBody(
	ctx context.Context,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	// If policy chain requires response body, execute policies with both headers and body
	if ec.policyChain.RequiresResponseBody {
		// Update response context with body data
		ec.responseContext.ResponseBody = &policies.Body{
			Content:     body.Body,
			EndOfStream: body.EndOfStream,
			Present:     true,
		}

		// Execute response policy chain with headers and body
		execResult, err := ec.server.core.ExecuteResponsePolicies(
			ec.policyChain.ResponsePolicies,
			ec.responseContext,
			ec.policyChain.ResponsePolicySpecs,
		)
		if err != nil {
			slog.ErrorContext(ctx, "Error executing response policies", "error", err)
			// Allow response through unmodified on error
			return &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseBody{
					ResponseBody: &extprocv3.BodyResponse{},
				},
			}, nil
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
func (ec *PolicyExecutionContext) buildRequestContext(headers *extprocv3.HttpHeaders) *policies.RequestContext {
	ctx := &policies.RequestContext{
		Headers:   make(map[string][]string),
		RequestID: ec.requestID,
		Metadata:  ec.policyChain.Metadata, // Share chain metadata
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
func (ec *PolicyExecutionContext) buildResponseContext(headers *extprocv3.HttpHeaders) *policies.ResponseContext {
	ctx := &policies.ResponseContext{
		RequestHeaders:  ec.requestContext.Headers,
		RequestBody:     ec.requestContext.Body,
		RequestPath:     ec.requestContext.Path,
		RequestMethod:   ec.requestContext.Method,
		RequestID:       ec.requestContext.RequestID,
		ResponseHeaders: make(map[string][]string),
		Metadata:        ec.requestContext.Metadata, // Share same metadata reference
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
