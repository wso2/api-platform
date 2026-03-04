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

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
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

	// Analytics metadata to be shared across request and response phases
	// This is for us to share any analyticd related data internally between phases
	// without currupting the metadata map used by the policies
	analyticsMetadata map[string]interface{}

	// Dynamic metadata to be shared across request and response phases
	dynamicMetadata map[string]map[string]interface{}

	// Reference to server components
	server *ExternalProcessorServer
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

	// TODO: (renuka) Do the optimization to skip response headers if not needed by checking the var hasResponseHeaderProcessing
	_ = hasResponseHeaderProcessing
	mode.ResponseHeaderMode = extprocconfigv3.ProcessingMode_SEND

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
		ctx,
		ec.policyChain.Policies,
		ec.requestContext,
		ec.policyChain.PolicySpecs,
		ec.requestContext.SharedContext.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "request_headers"), nil
	}

	// Translate execution result to ext_proc response (includes analytics metadata)
	return TranslateRequestHeadersActions(execResult, ec.policyChain, ec)
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
			ctx,
			ec.policyChain.Policies,
			ec.requestContext,
			ec.policyChain.PolicySpecs,
			ec.requestContext.SharedContext.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "request_body"), nil
		}

		// Translate execution result to ext_proc response (includes analytics metadata and short-circuit handling)
		return TranslateRequestBodyActions(execResult, ec.policyChain, ec)
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
	ec.buildResponseContext(headers)

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
		ctx,
		ec.policyChain.Policies,
		ec.responseContext,
		ec.policyChain.PolicySpecs,
		ec.responseContext.SharedContext.APIName,
		ec.routeKey,
		ec.policyChain.HasExecutionConditions,
	)
	if err != nil {
		return ec.handlePolicyError(ctx, err, "response_headers"), nil
	}

	// Translate execution result to ext_proc response (includes analytics metadata)
	return TranslateResponseHeadersActions(execResult, ec)
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
			ctx,
			ec.policyChain.Policies,
			ec.responseContext,
			ec.policyChain.PolicySpecs,
			ec.responseContext.SharedContext.APIName,
			ec.routeKey,
			ec.policyChain.HasExecutionConditions,
		)
		if err != nil {
			return ec.handlePolicyError(ctx, err, "response_body"), nil
		}

		// Translate execution result to ext_proc response (includes analytics metadata)
		return TranslateResponseBodyActions(execResult, ec)
	}

	// If policies don't require body, just allow it through unmodified
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{},
		},
	}, nil
}

// buildRequestContext converts Envoy headers to RequestContext
func (ec *PolicyExecutionContext) buildRequestContext(headers *extprocv3.HttpHeaders, routeMetadata RouteMetadata) {
	// Create headers map for internal manipulation
	headersMap := make(map[string][]string)

	// Variables to store pseudo-headers and request ID
	var path, method, authority, scheme, requestID string

	// Extract headers, pseudo-headers, and request ID in a single loop
	if headers.Headers != nil {
		for _, header := range headers.Headers.GetHeaders() {
			key := header.Key
			value := string(header.RawValue)
			headersMap[key] = append(headersMap[key], value)

			// Extract pseudo-headers and request ID
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
				if requestID == "" { // Take first occurrence
					requestID = value
				}
			}
		}
	}

	// Generate request ID if not present in headers
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Create shared context that will persist across request/response phases
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
		AuthContext:   make(map[string]string),
	}
	// Add template handle to metadata for LLM provider/proxy scenarios
	if routeMetadata.TemplateHandle != "" {
		sharedCtx.Metadata["template_handle"] = routeMetadata.TemplateHandle
	}
	// Add provider name to metadata for LLM provider/proxy scenarios
	if routeMetadata.ProviderName != "" {
		sharedCtx.Metadata["provider_name"] = routeMetadata.ProviderName
	}

	// Build context with Headers wrapper and pseudo-headers
	ctx := &policy.RequestContext{
		SharedContext: sharedCtx,
		Headers:       policy.NewHeaders(headersMap),
		Path:          path,
		Method:        method,
		Authority:     authority,
		Scheme:        scheme,
		Vhost:         routeMetadata.Vhost,
	}

	// Set request ID in execution context from SharedContext
	ec.requestID = requestID

	// Initialize Body with EndOfStream from headers (for requests without body)
	// This will be overwritten if processRequestBody is called
	if headers.EndOfStream {
		ctx.Body = &policy.Body{
			Content:     nil,
			EndOfStream: true,
			Present:     false,
		}
	}

	ec.requestContext = ctx
}

// buildResponseContext converts Envoy response headers and stored request context
func (ec *PolicyExecutionContext) buildResponseContext(headers *extprocv3.HttpHeaders) {
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
	ec.responseContext = ctx
}
