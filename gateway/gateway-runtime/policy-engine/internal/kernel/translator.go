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
	"fmt"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/utils"
	"google.golang.org/protobuf/types/known/structpb"
	"log/slog"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
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
func translateRequestActionsCore(result *executor.RequestExecutionResult, execCtx *PolicyExecutionContext) (
	headerMutation *extprocv3.HeaderMutation,
	bodyMutation *extprocv3.BodyMutation,
	analyticsData map[string]any,
	dynamicMetadata map[string]map[string]interface{},
	pathMutation *string,
	immediateResp *extprocv3.ProcessingResponse,
	err error) {

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
			analyticsStruct, err := buildAnalyticsStruct(immResp.AnalyticsMetadata, execCtx)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
			}
			response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, immResp.DynamicMetadata)
			return nil, nil, nil, nil, nil, response, nil
		}
	}

	// Build final action by resolving conflicting header operations
	headerOps := make(map[string][]*headerOp)
	analyticsData = make(map[string]any)
	dynamicMetadata = make(map[string]map[string]interface{})
	headerMutation = &extprocv3.HeaderMutation{}
	var finalBodyLength int
	bodyModified := false

	path := execCtx.requestContext.Path

	// Collect all operations in order
	for _, policyResult := range result.Results {
		if policyResult.Skipped {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamRequestModifications); ok {
				// Collect SetHeader operations
				for key, value := range mods.SetHeaders {
					headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "set", value: value})
				}

				// Collect AppendHeader operations
				for key, values := range mods.AppendHeaders {
					for _, value := range values {
						headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "append", value: value})
					}
				}

				// Collect RemoveHeader operations
				for _, key := range mods.RemoveHeaders {
					headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "remove", value: ""})
				}

				// Handle body modifications (last one wins)
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
					finalBodyLength = len(mods.Body)
					bodyModified = true
				}

				if mods.AddQueryParameters != nil {
					path = utils.AddQueryParametersToPath(path, mods.AddQueryParameters)
					pathMutation = &path
				}

				if mods.RemoveQueryParameters != nil {
					path = utils.RemoveQueryParametersFromPath(path, mods.RemoveQueryParameters)
					pathMutation = &path
				}

				if mods.Path != nil {
					pathMutation = mods.Path
				}

				// Collect analytics metadata from policies
				if mods.AnalyticsMetadata != nil {
					for key, value := range mods.AnalyticsMetadata {
						analyticsData[key] = value
						// Store in execution context for preservation across phases
						execCtx.analyticsMetadata[key] = value
					}
				}

				// Collect dynamic metadata from policies
				if mods.DynamicMetadata != nil {
					mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
					mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
				}

				dropAction := mods.DropHeadersFromAnalytics
				if dropAction.Action != "" || len(dropAction.Headers) > 0 {
					slog.Debug("Translator: Found DropHeadersFromAnalytics action (REQUEST)",
						"action", dropAction.Action,
						"headers", dropAction.Headers,
						"headers_count", len(dropAction.Headers))

					// Set the finalized headers to the analytics data
					originalHeaders := execCtx.requestContext.Headers.GetAll()
					finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
					analyticsData["request_headers"] = finalizedHeaders
					execCtx.analyticsMetadata["request_headers"] = finalizedHeaders
				}
			}
		}
	}

	// Remove any content-length headers from policy operations if we're managing it ourselves
	if bodyModified {
		delete(headerOps, "content-length")
	}

	// Build HeaderMutation with conflict resolution and merge with existing mutations
	mergeHeaderMutations(headerMutation, headerOps)

	// Set Content-Length header once after all policies have been processed
	if bodyModified {
		setContentLengthHeader(headerMutation, finalBodyLength)
	}

	return headerMutation, bodyMutation, analyticsData, dynamicMetadata, pathMutation, nil, nil
}

// TranslateRequestHeadersActions converts request headers execution result to ext_proc response
func TranslateRequestHeadersActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, path, immediateResp, err := translateRequestActionsCore(result, execCtx)
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

	// Add analytics metadata
	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, path, dynamicMetadata)

	return response, nil
}

// TranslateRequestBodyActions converts request body execution result to ext_proc response
func TranslateRequestBodyActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, _, immediateResp, err := translateRequestActionsCore(result, execCtx)
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

	// Add analytics metadata
	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)

	return response, nil
}

// translateResponseActionsCore is the shared implementation for response translation
func translateResponseActionsCore(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (
	headerMutation *extprocv3.HeaderMutation,
	bodyMutation *extprocv3.BodyMutation,
	analyticsData map[string]any,
	dynamicMetadata map[string]map[string]interface{},
	err error) {

	// Build final action by resolving conflicting header operations
	headerOps := make(map[string][]*headerOp)
	analyticsData = make(map[string]any)
	dynamicMetadata = make(map[string]map[string]interface{})
	headerMutation = &extprocv3.HeaderMutation{}

	// Merge analytics data from request phase stored in execution context
	for key, value := range execCtx.analyticsMetadata {
		// Skip request_headers as it's handled separately below
		if key != "request_headers" {
			analyticsData[key] = value
		}
	}
	mergeDynamicMetadata(dynamicMetadata, execCtx.dynamicMetadata)
	var finalBodyLength int
	bodyModified := false

	// Collect all operations in order
	for _, policyResult := range result.Results {
		if policyResult.Skipped || policyResult.Error != nil {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamResponseModifications); ok {
				// Collect SetHeader operations
				for key, value := range mods.SetHeaders {
					headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "set", value: value})
				}

				// Collect AppendHeader operations
				for key, values := range mods.AppendHeaders {
					for _, value := range values {
						headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "append", value: value})
					}
				}

				// Collect RemoveHeader operations
				for _, key := range mods.RemoveHeaders {
					headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "remove", value: ""})
				}

				// Handle body modifications (last one wins)
				if mods.Body != nil {
					bodyMutation = &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{
							Body: mods.Body,
						},
					}
					finalBodyLength = len(mods.Body)
					bodyModified = true
				}

				// Collect analytics metadata from policies
				if mods.AnalyticsMetadata != nil {
					for key, value := range mods.AnalyticsMetadata {
						analyticsData[key] = value
					}
				}

				// Collect dynamic metadata from policies
				if mods.DynamicMetadata != nil {
					mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
					mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
				}

				dropAction := mods.DropHeadersFromAnalytics
				if dropAction.Action != "" || len(dropAction.Headers) > 0 {
					slog.Debug("Translator: Found DropHeadersFromAnalytics action (RESPONSE)",
						"action", dropAction.Action,
						"headers", dropAction.Headers,
						"headers_count", len(dropAction.Headers))

					// Set the finalized headers to the analytics data
					originalHeaders := execCtx.responseContext.ResponseHeaders.GetAll()
					finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
					analyticsData["response_headers"] = finalizedHeaders

					// Include request_headers from execution context if it was set in a previous phase
					if _, exists := execCtx.analyticsMetadata["request_headers"]; exists {
						slog.Debug("Translator: Including request_headers from execution context")
						analyticsData["request_headers"] = execCtx.analyticsMetadata["request_headers"]
					}

				}
			}
		}
	}

	// Remove any content-length headers from policy operations if we're managing it ourselves
	if bodyModified {
		delete(headerOps, "content-length")
	}

	// Build HeaderMutation with conflict resolution and merge with existing mutations
	mergeHeaderMutations(headerMutation, headerOps)

	// Set Content-Length header once after all policies have been processed
	if bodyModified {
		setContentLengthHeader(headerMutation, finalBodyLength)
	}

	return headerMutation, bodyMutation, analyticsData, dynamicMetadata, nil
}

// finalizeAnalyticsHeaders finalizes the analytics headers based on the drop action
// If action is "allow", only the specified headers that exist in originalHeaders are returned
// If action is "deny", all headers except the specified ones are returned
func finalizeAnalyticsHeaders(dropAction policy.DropHeaderAction, originalHeaders map[string][]string) map[string][]string {
	finalizedHeaders := make(map[string][]string)

	// If no action specified or no headers to filter, return all original headers
	if dropAction.Action == "" || len(dropAction.Headers) == 0 {
		return originalHeaders
	}

	// Create a map of specified headers (normalized to lowercase) for quick lookup
	specifiedHeaders := make(map[string]bool)
	for _, header := range dropAction.Headers {
		specifiedHeaders[strings.ToLower(header)] = true
	}
	switch dropAction.Action {
	case "allow":
		// Allow mode: only include headers that are in the specified list AND exist in original headers
		for headerName, headerValues := range originalHeaders {
			normalizedName := strings.ToLower(headerName)
			if specifiedHeaders[normalizedName] {
				// Include this header in the finalized headers
				finalizedHeaders[headerName] = headerValues
			}
		}
		slog.Debug("Analytics headers filtered (allow mode)",
			"original_count", len(originalHeaders),
			"specified_count", len(dropAction.Headers),
			"finalized_count", len(finalizedHeaders))
	case "deny":
		// Deny mode: include all headers except those in the specified list
		for headerName, headerValues := range originalHeaders {
			normalizedName := strings.ToLower(headerName)
			if !specifiedHeaders[normalizedName] {
				// Include this header (it's not in the deny list)
				finalizedHeaders[headerName] = headerValues
			}
		}
		slog.Debug("Analytics headers filtered (deny mode)",
			"original_count", len(originalHeaders),
			"specified_count", len(dropAction.Headers),
			"finalized_count", len(finalizedHeaders))
	default:
		// Unknown action, log warning and return all original headers
		slog.Warn("Unknown drop action, returning all headers",
			"action", dropAction.Action)
		return originalHeaders
	}

	return finalizedHeaders
}

// TranslateResponseHeadersActions converts response headers execution result to ext_proc response
func TranslateResponseHeadersActions(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, err := translateResponseActionsCore(result, execCtx)
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

	// Add analytics metadata
	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)

	return response, nil
}

// TranslateResponseBodyActions converts response body execution result to ext_proc response
func TranslateResponseBodyActions(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, err := translateResponseActionsCore(result, execCtx)
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
		response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)
		return response, nil
	}

	if len(dynamicMetadata) > 0 {
		response.DynamicMetadata = buildDynamicMetadata(nil, nil, dynamicMetadata)
	}

	return response, nil
}

// buildDynamicMetadata creates the dynamic metadata structure for analytics and path rewrite
func buildDynamicMetadata(analyticsStruct *structpb.Struct, path *string, extra map[string]map[string]interface{}) *structpb.Struct {
	namespaces := make(map[string]*structpb.Struct)

	baseFields := make(map[string]*structpb.Value)
	if analyticsStruct != nil {
		baseFields["analytics_data"] = structpb.NewStructValue(analyticsStruct)
	}
	if path != nil {
		baseFields["path"] = structpb.NewStringValue(*path)
	}
	if len(baseFields) > 0 {
		namespaces[constants.ExtProcFilterName] = &structpb.Struct{Fields: baseFields}
	}

	for namespace, metadata := range extra {
		if metadata == nil {
			continue
		}
		metaStruct, err := structpb.NewStruct(metadata)
		if err != nil {
			slog.Warn("Failed to build dynamic metadata struct", "namespace", namespace, "error", err)
			continue
		}
		if namespace == constants.ExtProcFilterName {
			// Prevent policies from overwriting reserved keys managed by the engine.
			delete(metaStruct.Fields, "analytics_data")
			delete(metaStruct.Fields, "path")
		}
		if existing, ok := namespaces[namespace]; ok {
			for key, value := range metaStruct.Fields {
				existing.Fields[key] = value
			}
			continue
		}
		namespaces[namespace] = metaStruct
	}

	if len(namespaces) == 0 {
		return nil
	}

	metadataStruct := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
	for namespace, metadata := range namespaces {
		metadataStruct.Fields[namespace] = structpb.NewStructValue(metadata)
	}

	return metadataStruct
}

func mergeDynamicMetadata(dest map[string]map[string]interface{}, src map[string]map[string]interface{}) {
	for namespace, metadata := range src {
		if metadata == nil {
			continue
		}
		target, exists := dest[namespace]
		if !exists {
			target = make(map[string]interface{})
			dest[namespace] = target
		}
		for key, value := range metadata {
			target[key] = value
		}
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

// mergeHeaderMutations builds HeaderMutation from operations and merges with existing mutations
func mergeHeaderMutations(headerMutation *extprocv3.HeaderMutation, headerOps map[string][]*headerOp) {
	opsMutation := buildHeaderMutationFromOps(headerOps)

	// Merge SetHeaders from ops-based mutation
	if len(opsMutation.SetHeaders) > 0 {
		headerMutation.SetHeaders = append(headerMutation.SetHeaders, opsMutation.SetHeaders...)
	}

	// Merge RemoveHeaders from ops-based mutation
	if len(opsMutation.RemoveHeaders) > 0 {
		headerMutation.RemoveHeaders = append(headerMutation.RemoveHeaders, opsMutation.RemoveHeaders...)
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
				Key:      strings.ToLower(key),
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
