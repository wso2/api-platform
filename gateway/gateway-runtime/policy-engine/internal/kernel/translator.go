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
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/utils"
	"google.golang.org/protobuf/types/known/structpb"

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

// ─── Request header phase translation ────────────────────────────────────────

// TranslateRequestHeaderActions converts a request-headers execution result to an ext_proc response.
func TranslateRequestHeaderActions(result *executor.RequestHeaderExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	// Short-circuit: ImmediateResponse from any policy
	if result.ShortCircuited {
		if imm, ok := result.FinalAction.(policy.ImmediateResponse); ok {
			return buildImmediateResponse(&imm, execCtx)
		}
	}

	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}
	var pathMutation *string
	var targetUpstreamName *string

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			continue
		}
		collectHeaderOps(headerOps, mods.Set, mods.Remove, mods.Append)

		mergeAnalytics(analyticsData, execCtx, mods.AnalyticsMetadata, mods.AnalyticsHeaderFilter, execCtx.requestHeaderCtx.Headers.GetAll())
		mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
		mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
	}

	handleUpstreamRouting(headerOps, targetUpstreamName, pathMutation, dynamicMetadata, execCtx)
	mergeHeaderMutations(headerMutation, headerOps)

	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation:  headerMutation,
					ClearRouteCache: true,
				},
			},
		},
		ModeOverride: execCtx.getModeOverride(),
	}

	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, pathMutation, dynamicMetadata)

	return response, nil
}

// ─── Request body phase translation ──────────────────────────────────────────

// TranslateRequestBodyActions converts a request-body execution result to an ext_proc response.
func TranslateRequestBodyActions(result *executor.RequestBodyExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	if result.ShortCircuited {
		if imm, ok := result.FinalAction.(policy.ImmediateResponse); ok {
			return buildImmediateResponse(&imm, execCtx)
		}
	}

	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation
	finalBodyLength := 0
	bodyModified := false
	var pathMutation *string
	var methodMutation *string
	var targetUpstreamName *string

	path := execCtx.requestBodyCtx.Path

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		a, ok := pr.Action.(policy.UpstreamRequestModifications)
		if !ok {
			continue
		}

		if a.Body != nil {
			bodyMutation = &extprocv3.BodyMutation{
				Mutation: &extprocv3.BodyMutation_Body{Body: a.Body},
			}
			finalBodyLength = len(a.Body)
			bodyModified = true
		}

		// Header mutations from body phase
		if a.Header != nil {
			collectHeaderOps(headerOps, a.Header.Set, a.Header.Remove, a.Header.Append)
		}

		if a.Path != nil {
			pathMutation = a.Path
			path = *a.Path
		}
		if a.QueryParametersToAdd != nil {
			path = utils.AddQueryParametersToPath(path, a.QueryParametersToAdd)
			pathMutation = &path
		}
		if a.QueryParametersToRemove != nil {
			path = utils.RemoveQueryParametersFromPath(path, a.QueryParametersToRemove)
			pathMutation = &path
		}
		if a.Method != nil {
			methodMutation = a.Method
			headerOps[":method"] = append(headerOps[":method"], &headerOp{opType: "set", value: *a.Method})
		}
		if a.UpstreamName != nil && *a.UpstreamName != "" {
			targetUpstreamName = a.UpstreamName
		}

		mergeAnalytics(analyticsData, execCtx, a.AnalyticsMetadata, a.AnalyticsHeaderFilter, execCtx.requestBodyCtx.Headers.GetAll())
		mergeDynamicMetadata(dynamicMetadata, a.DynamicMetadata)
		mergeDynamicMetadata(execCtx.dynamicMetadata, a.DynamicMetadata)
	}

	handleUpstreamRouting(headerOps, targetUpstreamName, pathMutation, dynamicMetadata, execCtx)

	if bodyModified {
		delete(headerOps, "content-length")
	}
	mergeHeaderMutations(headerMutation, headerOps)
	if bodyModified {
		setContentLengthHeader(headerMutation, finalBodyLength)
	}

	// ClearRouteCache is needed only when a routing-relevant mutation occurred: path,
	// query parameters, HTTP method, or upstream target. Sending it unconditionally
	// forces an unnecessary route re-evaluation on every body response.
	clearRouteCache := pathMutation != nil || methodMutation != nil || targetUpstreamName != nil

	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation:  headerMutation,
					BodyMutation:    bodyMutation,
					ClearRouteCache: clearRouteCache,
				},
			},
		},
		ModeOverride: execCtx.getModeOverride(),
	}

	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)

	return response, nil
}

// ─── Response header phase translation ───────────────────────────────────────

// TranslateResponseHeaderActions converts a response-headers execution result to an ext_proc response.
func TranslateResponseHeaderActions(result *executor.ResponseHeaderExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}

	// Carry over analytics from request phase
	for key, value := range execCtx.analyticsMetadata {
		if key != "request_headers" {
			analyticsData[key] = value
		}
	}
	mergeDynamicMetadata(dynamicMetadata, execCtx.dynamicMetadata)

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.DownstreamResponseHeaderModifications)
		if !ok {
			continue
		}
		collectHeaderOps(headerOps, mods.Set, mods.Remove, mods.Append)

		mergeAnalytics(analyticsData, execCtx, mods.AnalyticsMetadata, mods.AnalyticsHeaderFilter, execCtx.responseHeaderCtx.ResponseHeaders.GetAll())
		mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
		mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
	}

	mergeHeaderMutations(headerMutation, headerOps)

	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: headerMutation,
				},
			},
		},
	}

	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)

	return response, nil
}

// ─── Response body phase translation ─────────────────────────────────────────

// TranslateResponseBodyActions converts a response-body execution result to an ext_proc response.
func TranslateResponseBodyActions(result *executor.ResponseBodyExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	if result.ShortCircuited {
		if imm, ok := result.FinalAction.(policy.ImmediateResponse); ok {
			return buildImmediateResponse(&imm, execCtx)
		}
	}

	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}
	var bodyMutation *extprocv3.BodyMutation
	finalBodyLength := 0
	bodyModified := false

	// Carry over analytics from request phase
	for key, value := range execCtx.analyticsMetadata {
		if key != "request_headers" {
			analyticsData[key] = value
		}
	}
	mergeDynamicMetadata(dynamicMetadata, execCtx.dynamicMetadata)

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		a, ok := pr.Action.(policy.DownstreamResponseModifications)
		if !ok {
			continue
		}

		if a.Body != nil {
			bodyMutation = &extprocv3.BodyMutation{
				Mutation: &extprocv3.BodyMutation_Body{Body: a.Body},
			}
			finalBodyLength = len(a.Body)
			bodyModified = true
		}

		if a.StatusCode != nil {
			// Status codes are mutated via the :status pseudo-header — the only mechanism
			// ext_proc exposes for changing the response status in a BodyResponse.
			headerOps[":status"] = append(headerOps[":status"], &headerOp{
				opType: "set",
				value:  fmt.Sprintf("%d", *a.StatusCode),
			})
		}

		// Header mutations from body phase
		if a.Header != nil {
			collectHeaderOps(headerOps, a.Header.Set, a.Header.Remove, a.Header.Append)
		}

		mergeAnalytics(analyticsData, execCtx, a.AnalyticsMetadata, a.AnalyticsHeaderFilter, execCtx.responseBodyCtx.ResponseHeaders.GetAll())
		mergeDynamicMetadata(dynamicMetadata, a.DynamicMetadata)
		mergeDynamicMetadata(execCtx.dynamicMetadata, a.DynamicMetadata)
	}

	if bodyModified {
		delete(headerOps, "content-length")
	}
	mergeHeaderMutations(headerMutation, headerOps)
	if bodyModified {
		setContentLengthHeader(headerMutation, finalBodyLength)
	}

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

// ─── Streaming response chunk translation ─────────────────────────────────────

// TranslateStreamingResponseChunkAction converts a streaming chunk execution result
// into an ext_proc ProcessingResponse for FULL_DUPLEX_STREAMED mode.
// Must use BodyMutation_StreamedResponse (not BodyMutation_Body) in streaming mode.
// BodyMutation nil = forward the original chunk unchanged.
// BodyMutation non-nil = forward the mutated bytes.
func TranslateStreamingResponseChunkAction(result *executor.StreamingResponseExecutionResult, originalChunk *policy.StreamBody, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	var outputBody []byte

	if result.FinalAction != nil && result.FinalAction.Body != nil {
		outputBody = result.FinalAction.Body
	} else {
		outputBody = originalChunk.Chunk
	}

	// Collect analytics and dynamic metadata from all policy results for this chunk.
	// Chunk actions have no AnalyticsHeaderFilter, so metadata is merged directly.
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		for key, value := range pr.Action.AnalyticsMetadata {
			analyticsData[key] = value
			execCtx.analyticsMetadata[key] = value
		}
		mergeDynamicMetadata(dynamicMetadata, pr.Action.DynamicMetadata)
		mergeDynamicMetadata(execCtx.dynamicMetadata, pr.Action.DynamicMetadata)
	}

	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					BodyMutation: &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_StreamedResponse{
							StreamedResponse: &extprocv3.StreamedBodyResponse{
								Body:        outputBody,
								EndOfStream: originalChunk.EndOfStream,
							},
						},
					},
				},
			},
		},
	}

	if len(analyticsData) > 0 {
		analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
		}
		resp.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)
		return resp, nil
	}
	if len(dynamicMetadata) > 0 {
		resp.DynamicMetadata = buildDynamicMetadata(nil, nil, dynamicMetadata)
	}

	return resp, nil
}

// ─── Streaming request chunk translation ──────────────────────────────────────

// TranslateStreamingRequestChunkAction converts a streaming chunk execution result
// into an ext_proc ProcessingResponse for the request body in FULL_DUPLEX_STREAMED mode.
// Must use BodyMutation_StreamedResponse (not BodyMutation_Body) in streaming mode.
// BodyMutation nil = forward the original chunk unchanged.
// BodyMutation non-nil = forward the mutated bytes.
func TranslateStreamingRequestChunkAction(result *executor.StreamingRequestExecutionResult, originalChunk *policy.StreamBody, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	var outputBody []byte

	if result.FinalAction != nil && result.FinalAction.Body != nil {
		outputBody = result.FinalAction.Body
	} else {
		outputBody = originalChunk.Chunk
	}

	// Collect analytics and dynamic metadata from all policy results for this chunk.
	// Chunk actions have no AnalyticsHeaderFilter, so metadata is merged directly.
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		for key, value := range pr.Action.AnalyticsMetadata {
			analyticsData[key] = value
			execCtx.analyticsMetadata[key] = value
		}
		mergeDynamicMetadata(dynamicMetadata, pr.Action.DynamicMetadata)
		mergeDynamicMetadata(execCtx.dynamicMetadata, pr.Action.DynamicMetadata)
	}

	resp := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					BodyMutation: &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_StreamedResponse{
							StreamedResponse: &extprocv3.StreamedBodyResponse{
								Body:        outputBody,
								EndOfStream: originalChunk.EndOfStream,
							},
						},
					},
				},
			},
		},
	}

	if len(analyticsData) > 0 {
		analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
		}
		resp.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, dynamicMetadata)
		return resp, nil
	}
	if len(dynamicMetadata) > 0 {
		resp.DynamicMetadata = buildDynamicMetadata(nil, nil, dynamicMetadata)
	}

	return resp, nil
}

// ─── Shared helpers ───────────────────────────────────────────────────────────

// buildImmediateResponse converts an ImmediateResponse action to an ext_proc ProcessingResponse.
func buildImmediateResponse(immResp *policy.ImmediateResponse, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
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

	analyticsStruct, err := buildAnalyticsStruct(immResp.AnalyticsMetadata, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, immResp.DynamicMetadata)
	return response, nil
}

// collectHeaderOps appends set/remove/append operations to the headerOps accumulator.
func collectHeaderOps(
	headerOps map[string][]*headerOp,
	setHeaders map[string]string,
	removeHeaders []string,
	appendHeaders map[string][]string,
) {
	for key, value := range setHeaders {
		headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "set", value: value})
	}
	for key, values := range appendHeaders {
		for _, value := range values {
			headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "append", value: value})
		}
	}
	for _, key := range removeHeaders {
		headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "remove", value: ""})
	}
}

// mergeAnalytics merges policy analytics into the accumulator and handles header dropping.
func mergeAnalytics(
	analyticsData map[string]any,
	execCtx *PolicyExecutionContext,
	metadata map[string]any,
	dropAction policy.DropHeaderAction,
	originalHeaders map[string][]string,
) {
	for key, value := range metadata {
		analyticsData[key] = value
		execCtx.analyticsMetadata[key] = value
	}
	if dropAction.Action != "" || len(dropAction.Headers) > 0 {
		slog.Debug("Translator: Found AnalyticsHeaderFilter action",
			"action", dropAction.Action,
			"headers", dropAction.Headers)
		finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
		analyticsData["request_headers"] = finalizedHeaders
		execCtx.analyticsMetadata["request_headers"] = finalizedHeaders
	}
}

// handleUpstreamRouting applies dynamic cluster routing when a policy sets UpstreamName.
func handleUpstreamRouting(
	headerOps map[string][]*headerOp,
	targetUpstreamName *string,
	pathMutation *string,
	dynamicMetadata map[string]map[string]interface{},
	execCtx *PolicyExecutionContext,
) {
	if targetUpstreamName != nil {
		apiKind := execCtx.sharedCtx.APIKind
		apiId := execCtx.sharedCtx.APIId
		sanitizedDefName := sanitizeUpstreamDefinitionName(*targetUpstreamName)
		clusterName := constants.UpstreamDefinitionClusterPrefix + apiKind + "_" + apiId + "_" + sanitizedDefName

		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{
			opType: "set",
			value:  clusterName,
		})

		extProcNS := constants.ExtProcFilterName
		if execCtx.dynamicMetadata[extProcNS] == nil {
			execCtx.dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		if dynamicMetadata[extProcNS] == nil {
			dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		execCtx.dynamicMetadata[extProcNS][constants.TargetUpstreamClusterKey] = clusterName
		execCtx.dynamicMetadata[extProcNS][constants.TargetUpstreamNameKey] = *targetUpstreamName

		dynamicMetadata[extProcNS]["api_context"] = execCtx.apiContext
		dynamicMetadata[extProcNS]["upstream_base_path"] = execCtx.upstreamBasePath

		currentPath := execCtx.requestHeaderCtx.Path
		slog.Info("UpstreamName: checking upstreamDefinitionPaths",
			"targetUpstream", *targetUpstreamName,
			"hasUpstreamDefPaths", execCtx.upstreamDefinitionPaths != nil,
			"upstreamDefPaths", execCtx.upstreamDefinitionPaths,
			"apiContext", execCtx.apiContext)

		if execCtx.upstreamDefinitionPaths != nil {
			if targetUpstreamPath, ok := execCtx.upstreamDefinitionPaths[*targetUpstreamName]; ok {
				dynamicMetadata[extProcNS]["target_upstream_base_path"] = targetUpstreamPath
				execCtx.dynamicMetadata[extProcNS]["target_upstream_base_path"] = targetUpstreamPath
				slog.Info("UpstreamName: set target upstream base path",
					"targetUpstream", *targetUpstreamName,
					"targetUpstreamPath", targetUpstreamPath)

				if _, hasTargetPath := dynamicMetadata[extProcNS]["request_transformation.target_path"]; !hasTargetPath {
					dynamicMetadata[extProcNS]["request_transformation.target_path"] = currentPath
					execCtx.dynamicMetadata[extProcNS]["request_transformation.target_path"] = currentPath
					slog.Info("UpstreamName: set target_path", "path", currentPath)
				} else {
					slog.Info("UpstreamName: target_path already set by policy")
				}
			} else {
				slog.Warn("UpstreamName: target upstream not found in upstreamDefinitionPaths",
					"targetUpstream", *targetUpstreamName,
					"availableUpstreams", execCtx.upstreamDefinitionPaths)
			}
		}
	} else if execCtx.defaultUpstreamCluster != "" {
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{
			opType: "set",
			value:  execCtx.defaultUpstreamCluster,
		})
	}
}

// finalizeAnalyticsHeaders finalizes the analytics headers based on the drop action.
func finalizeAnalyticsHeaders(dropAction policy.DropHeaderAction, originalHeaders map[string][]string) map[string][]string {
	finalizedHeaders := make(map[string][]string)

	if dropAction.Action == "" || len(dropAction.Headers) == 0 {
		return originalHeaders
	}

	specifiedHeaders := make(map[string]bool)
	for _, header := range dropAction.Headers {
		specifiedHeaders[strings.ToLower(header)] = true
	}
	switch dropAction.Action {
	case "allow":
		for headerName, headerValues := range originalHeaders {
			if specifiedHeaders[strings.ToLower(headerName)] {
				finalizedHeaders[headerName] = headerValues
			}
		}
		slog.Debug("Analytics headers filtered (allow mode)",
			"original_count", len(originalHeaders),
			"specified_count", len(dropAction.Headers),
			"finalized_count", len(finalizedHeaders))
	case "deny":
		for headerName, headerValues := range originalHeaders {
			if !specifiedHeaders[strings.ToLower(headerName)] {
				finalizedHeaders[headerName] = headerValues
			}
		}
		slog.Debug("Analytics headers filtered (deny mode)",
			"original_count", len(originalHeaders),
			"specified_count", len(dropAction.Headers),
			"finalized_count", len(finalizedHeaders))
	default:
		slog.Warn("Unknown drop action, returning all headers", "action", dropAction.Action)
		return originalHeaders
	}

	return finalizedHeaders
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

// buildHeaderMutationFromOps builds HeaderMutation from header operations with conflict resolution.
func buildHeaderMutationFromOps(headerOps map[string][]*headerOp) *extprocv3.HeaderMutation {
	headerMutation := &extprocv3.HeaderMutation{}

	for key, ops := range headerOps {
		if len(ops) == 0 {
			continue
		}

		lastOp := ops[len(ops)-1]

		if lastOp.opType == "remove" {
			if headerMutation.RemoveHeaders == nil {
				headerMutation.RemoveHeaders = make([]string, 0)
			}
			headerMutation.RemoveHeaders = append(headerMutation.RemoveHeaders, key)
		} else if lastOp.opType == "set" {
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

			if lastBreakType == "set" {
				headerMutation.SetHeaders = append(headerMutation.SetHeaders, &corev3.HeaderValueOption{
					Header: &corev3.HeaderValue{
						Key:      key,
						RawValue: []byte(ops[lastBreakIdx].value),
					},
					AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
				})
			}

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

func mergeHeaderMutations(headerMutation *extprocv3.HeaderMutation, headerOps map[string][]*headerOp) {
	opsMutation := buildHeaderMutationFromOps(headerOps)

	if len(opsMutation.SetHeaders) > 0 {
		headerMutation.SetHeaders = append(headerMutation.SetHeaders, opsMutation.SetHeaders...)
	}
	if len(opsMutation.RemoveHeaders) > 0 {
		headerMutation.RemoveHeaders = append(headerMutation.RemoveHeaders, opsMutation.RemoveHeaders...)
	}
}

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

func sanitizeUpstreamDefinitionName(name string) string {
	sanitized := strings.ReplaceAll(name, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	return sanitized
}

func computeUpstreamPath(currentPath, apiContext, upstreamPath string) string {
	if currentPath == "" {
		return ""
	}

	if apiContext == "" {
		apiContext = "/"
	}

	relativePath := currentPath
	if apiContext != "/" {
		if strings.HasPrefix(currentPath, apiContext) {
			relativePath = strings.TrimPrefix(currentPath, apiContext)
			if relativePath == "" {
				relativePath = "/"
			}
		}
	}

	if upstreamPath == "" || upstreamPath == "/" {
		return relativePath
	}

	if strings.HasSuffix(upstreamPath, "/") && strings.HasPrefix(relativePath, "/") {
		return upstreamPath + relativePath[1:]
	}
	if !strings.HasSuffix(upstreamPath, "/") && !strings.HasPrefix(relativePath, "/") {
		return upstreamPath + "/" + relativePath
	}
	return upstreamPath + relativePath
}
