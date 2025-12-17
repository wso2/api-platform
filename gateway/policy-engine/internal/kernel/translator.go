package kernel

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/policy-engine/policy-engine/internal/executor"
	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// headerOp represents a single header operation (set, append, or remove)
type headerOp struct {
	opType string // "set", "append", or "remove"
	value  string // for set and append operations
}

// Mutations holds header and body mutations for request/response processing
type Mutations struct {
	HeaderMutation *extprocv3.HeaderMutation
	BodyMutation   *extprocv3.BodyMutation
}

// translateRequestActionsCore is the shared implementation for request translation
func translateRequestActionsCore(result *executor.RequestExecutionResult, execCtx *PolicyExecutionContext) (headerMutation *extprocv3.HeaderMutation, bodyMutation *extprocv3.BodyMutation, analyticsData map[string]any, immediateResp *extprocv3.ProcessingResponse, err error) {
	// Check for short-circuit with immediate response
	if result.ShortCircuited && result.FinalAction != nil {
		if immResp, ok := result.FinalAction.(policy.ImmediateResponse); ok {
			response := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &extprocv3.ImmediateResponse{
						Status: &typev3.HttpStatus{
							Code: typev3.StatusCode(immResp.StatusCode),
						},
						Headers: buildHeaderValueOptions(immResp.Headers),
						Body:    immResp.Body,
					},
				},
			}

			// Handle analytics metadata for immediate response
			if len(immResp.AnalyticsMetadata) > 0 {
				analyticsStruct, err := buildAnalyticsStruct(immResp.AnalyticsMetadata, nil)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
				}
				response.DynamicMetadata = buildDynamicMetadata(analyticsStruct)
			}
			return nil, nil, nil, response, nil
		}
	}

	// Build final action by resolving conflicting header operations
	headerOps := make(map[string][]*headerOp)
	analyticsData = make(map[string]any)

	// Collect all operations in order
	for _, policyResult := range result.Results {
		if policyResult.Skipped {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamRequestModifications); ok {
				// Collect SetHeader operations
				for key, value := range mods.SetHeaders {
					headerOps[key] = append(headerOps[key], &headerOp{opType: "set", value: value})
				}

				// Collect AppendHeader operations
				for key, values := range mods.AppendHeaders {
					for _, value := range values {
						headerOps[key] = append(headerOps[key], &headerOp{opType: "append", value: value})
					}
				}

				// Collect RemoveHeader operations
				for _, key := range mods.RemoveHeaders {
					headerOps[key] = append(headerOps[key], &headerOp{opType: "remove", value: ""})
				}

				// Handle body modifications (last one wins)
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
					// Update Content-Length header to match new body size
					setContentLengthHeader(headerMutation, len(mods.Body))
				}

				// Collect analytics metadata from policies
				if mods.AnalyticsMetadata != nil {
					for key, value := range mods.AnalyticsMetadata {
						analyticsData[key] = value
					}
				}
			}
		}
	}

	// Build HeaderMutation with conflict resolution
	headerMutation = buildHeaderMutationFromOps(headerOps)
	return headerMutation, bodyMutation, analyticsData, nil, nil
}

// TranslateRequestHeadersActions converts request headers execution result to ext_proc response
func TranslateRequestHeadersActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, immediateResp, err := translateRequestActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}
	if immediateResp != nil {
		return immediateResp, nil
	}

	// Build ProcessingResponse for request headers
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
					BodyMutation:   bodyMutation,
				},
			},
		},
		ModeOverride: execCtx.getModeOverride(),
	}

	// Add analytics metadata if present
	if len(analyticsData) > 0 {
		analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
		}
		response.DynamicMetadata = buildDynamicMetadata(analyticsStruct)
	}

	return response, nil
}

// TranslateRequestBodyActions converts request body execution result to ext_proc response
func TranslateRequestBodyActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, immediateResp, err := translateRequestActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}
	if immediateResp != nil {
		return immediateResp, nil
	}

	// Build ProcessingResponse for request body
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
					BodyMutation:   bodyMutation,
				},
			},
		},
		ModeOverride: execCtx.getModeOverride(),
	}

	// Add analytics metadata if present
	if len(analyticsData) > 0 {
		analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
		}
		response.DynamicMetadata = buildDynamicMetadata(analyticsStruct)
	}

	return response, nil
}

// translateResponseActionsCore is the shared implementation for response translation
func translateResponseActionsCore(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (headerMutation *extprocv3.HeaderMutation, bodyMutation *extprocv3.BodyMutation, analyticsData map[string]any, err error) {
	headerMutation = &extprocv3.HeaderMutation{}
	analyticsData = make(map[string]any)

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamResponseModifications); ok {
				// Build response mutations
				applyResponseModifications(headerMutation, &mods)

				// Handle body modifications (last one wins)
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
					// Update Content-Length header to match new body size
					setContentLengthHeader(headerMutation, len(mods.Body))
				}

				// Collect analytics metadata from policies
				if mods.AnalyticsMetadata != nil {
					for key, value := range mods.AnalyticsMetadata {
						analyticsData[key] = value
					}
				}
			}
		}
	}

	return headerMutation, bodyMutation, analyticsData, nil
}

// TranslateResponseHeadersActions converts response headers execution result to ext_proc response
func TranslateResponseHeadersActions(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, err := translateResponseActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}

	// Build ProcessingResponse for response headers
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
					BodyMutation:   bodyMutation,
				},
			},
		},
	}

	// Add analytics metadata if present
	if len(analyticsData) > 0 {
		analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
		}
		response.DynamicMetadata = buildDynamicMetadata(analyticsStruct)
	}

	return response, nil
}

// TranslateResponseBodyActions converts response body execution result to ext_proc response
func TranslateResponseBodyActions(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, err := translateResponseActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}

	// Build ProcessingResponse for response body
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
					BodyMutation:   bodyMutation,
				},
			},
		},
	}

	// Add analytics metadata if present
	if len(analyticsData) > 0 {
		analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
		}
		response.DynamicMetadata = buildDynamicMetadata(analyticsStruct)
	}

	return response, nil
}

// buildHeaderMutationFromOps builds HeaderMutation from header operations with conflict resolution
// Rules:
// - If last operation is Remove: only send Remove
// - If last operation is Set: only send that Set
// - If last operation is Append: send last Set (if any) + all subsequent Appends
func buildHeaderMutationFromOps(headerOps map[string][]*headerOp) *extprocv3.HeaderMutation {
	headerMutation := &extprocv3.HeaderMutation{}

	for key, ops := range headerOps {
		if len(ops) == 0 {
			continue
		}

		// Check the last operation for this header
		lastOp := ops[len(ops)-1]

		if lastOp.opType == "remove" {
			// If last operation is remove, only send remove (ignore all previous operations)
			if headerMutation.RemoveHeaders == nil {
				headerMutation.RemoveHeaders = make([]string, 0)
			}
			headerMutation.RemoveHeaders = append(headerMutation.RemoveHeaders, key)
		} else if lastOp.opType == "set" {
			// If last operation is set, only send that set (ignore all previous operations)
			if headerMutation.SetHeaders == nil {
				headerMutation.SetHeaders = make([]*corev3.HeaderValueOption, 0)
			}
			headerMutation.SetHeaders = append(headerMutation.SetHeaders, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{
					Key:      key,
					RawValue: []byte(lastOp.value),
				},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			})
		} else if lastOp.opType == "append" {
			// If last operation is append, find the last set or remove
			lastBreakIdx := -1
			lastBreakType := ""
			for i := len(ops) - 1; i >= 0; i-- {
				if ops[i].opType == "set" || ops[i].opType == "remove" {
					lastBreakIdx = i
					lastBreakType = ops[i].opType
					break
				}
			}

			if headerMutation.SetHeaders == nil {
				headerMutation.SetHeaders = make([]*corev3.HeaderValueOption, 0)
			}

			// If last break is a Set, send it with OVERWRITE
			if lastBreakType == "set" {
				headerMutation.SetHeaders = append(headerMutation.SetHeaders, &corev3.HeaderValueOption{
					Header: &corev3.HeaderValue{
						Key:      key,
						RawValue: []byte(ops[lastBreakIdx].value),
					},
					AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
				})
			}
			// If last break is a Remove, we discard it (don't send Remove)

			// Send all appends after the last break (or all appends if no break found)
			startIdx := lastBreakIdx + 1
			for i := startIdx; i < len(ops); i++ {
				if ops[i].opType == "append" {
					headerMutation.SetHeaders = append(headerMutation.SetHeaders, &corev3.HeaderValueOption{
						Header: &corev3.HeaderValue{
							Key:      key,
							RawValue: []byte(ops[i].value),
						},
						AppendAction: corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
					})
				}
			}
		}
	}

	return headerMutation
}

// applyResponseModifications applies response modifications to header mutation
func applyResponseModifications(mutation *extprocv3.HeaderMutation, mods *policy.UpstreamResponseModifications) {
	// Set/Replace headers
	if len(mods.SetHeaders) > 0 {
		if mutation.SetHeaders == nil {
			mutation.SetHeaders = make([]*corev3.HeaderValueOption, 0, len(mods.SetHeaders))
		}
		for key, value := range mods.SetHeaders {
			mutation.SetHeaders = append(mutation.SetHeaders, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{
					Key:      key,
					RawValue: []byte(value),
				},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			})
		}
	}

	// Append headers
	if len(mods.AppendHeaders) > 0 {
		if mutation.SetHeaders == nil {
			mutation.SetHeaders = make([]*corev3.HeaderValueOption, 0)
		}
		for key, values := range mods.AppendHeaders {
			for _, value := range values {
				mutation.SetHeaders = append(mutation.SetHeaders, &corev3.HeaderValueOption{
					Header: &corev3.HeaderValue{
						Key:      key,
						RawValue: []byte(value),
					},
					AppendAction: corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
				})
			}
		}
	}

	// Remove headers
	if len(mods.RemoveHeaders) > 0 {
		if mutation.RemoveHeaders == nil {
			mutation.RemoveHeaders = make([]string, 0, len(mods.RemoveHeaders))
		}
		mutation.RemoveHeaders = append(mutation.RemoveHeaders, mods.RemoveHeaders...)
	}
}

// buildHeaderValueOptions converts map of headers to HeaderValueOption array
func buildHeaderValueOptions(headers map[string]string) *extprocv3.HeaderMutation {
	if len(headers) == 0 {
		return nil
	}

	mutation := &extprocv3.HeaderMutation{
		SetHeaders: make([]*corev3.HeaderValueOption, 0, len(headers)),
	}

	for key, value := range headers {
		mutation.SetHeaders = append(mutation.SetHeaders, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{
				Key:      key,
				RawValue: []byte(value),
			},
			AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
		})
	}

	return mutation
}

// setContentLengthHeader sets the Content-Length header to match the body size
func setContentLengthHeader(mutation *extprocv3.HeaderMutation, bodyLength int) {
	if mutation.SetHeaders == nil {
		mutation.SetHeaders = make([]*corev3.HeaderValueOption, 0, 1)
	}

	mutation.SetHeaders = append(mutation.SetHeaders, &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{
			Key:      "content-length",
			RawValue: []byte(fmt.Sprintf("%d", bodyLength)),
		},
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	})
}
