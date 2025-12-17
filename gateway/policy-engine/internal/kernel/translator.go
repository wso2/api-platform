package kernel

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/policy-engine/policy-engine/internal/constants"
	"github.com/policy-engine/policy-engine/internal/executor"
	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// headerOp represents a single header operation (set, append, or remove)
type headerOp struct {
	opType string // "set", "append", or "remove"
	value  string // for set and append operations
}

// TranslateRequestActions converts policy execution result to ext_proc response
// The execCtx parameter is optional - if provided, uses its computed mode override
func TranslateRequestActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) *extprocv3.ProcessingResponse {
	if result.ShortCircuited && result.FinalAction != nil {
		// Short-circuited with ImmediateResponse
		if immediateResp, ok := result.FinalAction.(policy.ImmediateResponse); ok {
			// T066: Handle ImmediateResponse
			response := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &extprocv3.ImmediateResponse{
						Status: &typev3.HttpStatus{
							Code: typev3.StatusCode(immediateResp.StatusCode),
						},
						Headers: buildHeaderValueOptions(immediateResp.Headers),
						Body:    immediateResp.Body,
					},
				},
			}

			// Handle analytics metadata for immediate response
			if len(immediateResp.AnalyticsMetadata) > 0 {
				analyticsStruct, err := buildAnalyticsStruct(immediateResp.AnalyticsMetadata, nil)
				if err == nil {
					response.DynamicMetadata = &structpb.Struct{
						Fields: map[string]*structpb.Value{
							constants.ExtProcFilterName: {
								Kind: &structpb.Value_StructValue{
									StructValue: &structpb.Struct{
										Fields: map[string]*structpb.Value{
											"analytics_data": {
												Kind: &structpb.Value_StructValue{
													StructValue: analyticsStruct,
												},
											},
										},
									},
								},
							},
						},
					}
				}
			}

			return response
		}
	}

	// Build final action by resolving conflicting header operations
	// Track sequence of operations per header to handle conflicts correctly
	headerOps := make(map[string][]*headerOp)
	var bodyMutation *extprocv3.BodyMutation
	analyticsData := make(map[string]any)

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
				}

				// Handle analytics metadata - merge all metadata from policies
				if mods.AnalyticsMetadata != nil {
					for key, value := range mods.AnalyticsMetadata {
						analyticsData[key] = value
					}
				}
			}
		}
	}

	// Build HeaderMutation with conflict resolution
	headerMutation := buildHeaderMutationFromOps(headerOps)

	// T070: Implement mode override configuration
	// Determine if we need to override body processing mode
	modeOverride := execCtx.getModeOverride()

	// Build ProcessingResponse
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
					BodyMutation:   bodyMutation,
					// Set mode override based on chain requirements
					// This tells Envoy whether to buffer body or not
					// T070: mode override implementation
					// If chain doesn't need request body, use SKIP mode
					// If chain needs request body, use BUFFERED mode
				},
			},
		},
		ModeOverride: modeOverride,
	}

	// Build DynamicMetadata for analytics
	// Include system-level metadata from execution context
	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err == nil {
		response.DynamicMetadata = &structpb.Struct{
			Fields: map[string]*structpb.Value{
				constants.ExtProcFilterName: {
					Kind: &structpb.Value_StructValue{
						StructValue: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"analytics_data": {
									Kind: &structpb.Value_StructValue{
										StructValue: analyticsStruct,
									},
								},
							},
						},
					},
				},
			},
		}
	}

	return response
}

// TranslateResponseActions converts response policy execution result to ext_proc response
// T067: TranslateResponseActions for UpstreamResponseModifications
func TranslateResponseActions(result *executor.ResponseExecutionResult) *extprocv3.ProcessingResponse {
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamResponseModifications); ok {
				// T069: Build response mutations
				applyResponseModifications(headerMutation, &mods)

				// Handle body modifications if present
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
				}
			}
		}
	}

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
					BodyMutation:   bodyMutation,
				},
			},
		},
	}
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

// applyRequestModifications applies request modifications to header mutation
func applyRequestModifications(mutation *extprocv3.HeaderMutation, mods *policy.UpstreamRequestModifications) {
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

// applyResponseModifications applies response modifications to header mutation
// T069: buildResponseMutations helper implementation
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

// Mutations holds header and body mutations for request/response processing
type Mutations struct {
	HeaderMutation *extprocv3.HeaderMutation
	BodyMutation   *extprocv3.BodyMutation
	// AnalyticsData
}

// buildRequestMutations extracts header and body mutations from request execution result
func buildRequestMutations(result *executor.RequestExecutionResult) Mutations {
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamRequestModifications); ok {
				// Build header mutations
				applyRequestModifications(headerMutation, &mods)

				// Handle body modifications if present
				// mods.Body is []byte from the action
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
					// Update Content-Length header to match new body size
					setContentLengthHeader(headerMutation, len(mods.Body))
				}
			}
		}
	}

	return Mutations{
		HeaderMutation: headerMutation,
		BodyMutation:   bodyMutation,
	}
}

// buildResponseMutations extracts header and body mutations from response execution result
func buildResponseMutations(result *executor.ResponseExecutionResult) Mutations {
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamResponseModifications); ok {
				// Build header mutations
				applyResponseModifications(headerMutation, &mods)

				// Handle body modifications if present
				// mods.Body is []byte from the action
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
					// Update Content-Length header to match new body size
					setContentLengthHeader(headerMutation, len(mods.Body))
				}
			}
		}
	}

	return Mutations{
		HeaderMutation: headerMutation,
		BodyMutation:   bodyMutation,
	}
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

// determineModeOverride determines body processing mode based on chain requirements
// T070: mode override configuration implementation
func determineModeOverride(chain *registry.PolicyChain, isRequest bool) *extprocconfigv3.ProcessingMode {
	if isRequest && !chain.RequiresRequestBody {
		// Chain doesn't need request body - use NONE mode for performance
		return &extprocconfigv3.ProcessingMode{
			RequestBodyMode: extprocconfigv3.ProcessingMode_NONE,
		}
	}

	if isRequest && chain.RequiresRequestBody {
		// Chain needs request body - use BUFFERED mode
		return &extprocconfigv3.ProcessingMode{
			RequestBodyMode: extprocconfigv3.ProcessingMode_BUFFERED,
		}
	}

	if !isRequest && !chain.RequiresResponseBody {
		// Chain doesn't need response body - use NONE mode
		return &extprocconfigv3.ProcessingMode{
			ResponseBodyMode: extprocconfigv3.ProcessingMode_NONE,
		}
	}

	if !isRequest && chain.RequiresResponseBody {
		// Chain needs response body - use BUFFERED mode
		return &extprocconfigv3.ProcessingMode{
			ResponseBodyMode: extprocconfigv3.ProcessingMode_BUFFERED,
		}
	}

	return nil
}

// buildAnalyticsStruct converts analytics metadata map to structpb.Struct
// If execCtx is provided, adds system-level metadata (API name, version, etc.) to analytics_data.metadata
func buildAnalyticsStruct(analyticsData map[string]any, execCtx *PolicyExecutionContext) (*structpb.Struct, error) {
	// Start with the analytics data from policies
	fields := make(map[string]*structpb.Value)

	// Add policy-provided analytics data
	for key, value := range analyticsData {
		val, err := structpb.NewValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert analytics value for key %s: %w", key, err)
		}
		fields[key] = val
	}

	// Add system-level metadata if context is provided
	if execCtx != nil && execCtx.requestContext != nil && execCtx.requestContext.SharedContext != nil {
		metadata := make(map[string]interface{})

		sharedCtx := execCtx.requestContext.SharedContext
		if sharedCtx.APIName != "" {
			metadata["api_name"] = sharedCtx.APIName
		}
		if sharedCtx.APIVersion != "" {
			metadata["api_version"] = sharedCtx.APIVersion
		}
		if sharedCtx.APIContext != "" {
			metadata["api_context"] = sharedCtx.APIContext
		}
		if sharedCtx.OperationPath != "" {
			metadata["operation_path"] = sharedCtx.OperationPath
		}
		if sharedCtx.RequestID != "" {
			metadata["request_id"] = sharedCtx.RequestID
		}

		if len(metadata) > 0 {
			metadataVal, err := structpb.NewValue(metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to convert system metadata: %w", err)
			}
			fields["metadata"] = metadataVal
		}
	}

	return &structpb.Struct{Fields: fields}, nil
}
