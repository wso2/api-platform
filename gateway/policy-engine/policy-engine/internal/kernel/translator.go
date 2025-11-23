package kernel

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/policy-engine/policy-engine/internal/executor"
	"github.com/policy-engine/policy-engine/internal/registry"
	"github.com/policy-engine/sdk/policies"
)

// TranslateRequestActions converts policy execution result to ext_proc response
// T065: TranslateRequestActions for UpstreamRequestModifications
// T066: TranslateRequestActions for ImmediateResponse
// The execCtx parameter is optional - if provided, uses its computed mode override
func TranslateRequestActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) *extprocv3.ProcessingResponse {
	if result.ShortCircuited && result.FinalAction != nil {
		// Short-circuited with ImmediateResponse
		if immediateResp, ok := result.FinalAction.(policies.ImmediateResponse); ok {
			// T066: Handle ImmediateResponse
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
			}
		}
	}

	// Normal case: apply accumulated modifications
	// T065: Handle UpstreamRequestModifications
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policies.UpstreamRequestModifications); ok {
				// T068: Build header mutations
				applyRequestModifications(headerMutation, &mods)

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

	// T070: Implement mode override configuration
	// Determine if we need to override body processing mode
	modeOverride := execCtx.getModeOverride()

	return &extprocv3.ProcessingResponse{
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
			if mods, ok := policyResult.Action.(policies.UpstreamResponseModifications); ok {
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

// applyRequestModifications applies request modifications to header mutation
// T068: buildHeaderMutations helper implementation
func applyRequestModifications(mutation *extprocv3.HeaderMutation, mods *policies.UpstreamRequestModifications) {
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
func applyResponseModifications(mutation *extprocv3.HeaderMutation, mods *policies.UpstreamResponseModifications) {
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

// buildRequestMutations extracts header and body mutations from request execution result
func buildRequestMutations(result *executor.RequestExecutionResult) (*extprocv3.HeaderMutation, *extprocv3.BodyMutation) {
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policies.UpstreamRequestModifications); ok {
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

	return headerMutation, bodyMutation
}

// buildResponseMutations extracts header and body mutations from response execution result
func buildResponseMutations(result *executor.ResponseExecutionResult) (*extprocv3.HeaderMutation, *extprocv3.BodyMutation) {
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation

	// Accumulate modifications from all executed policies
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policies.UpstreamResponseModifications); ok {
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

	return headerMutation, bodyMutation
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
