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
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
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
	methodMutation *string,
	immediateResp *extprocv3.ProcessingResponse,
	err error) {

	// Check for short-circuit with immediate response
	if result.ShortCircuited && result.FinalAction != nil {
		if immResp, ok := result.FinalAction.(policy.ImmediateResponse); ok {
			// Preserve request-phase analytics metadata from policies executed before
			// the short-circuit action so immediate responses still include it.
			shortCircuitAnalyticsData := make(map[string]any)
			for key, value := range execCtx.analyticsMetadata {
				shortCircuitAnalyticsData[key] = value
			}
			for _, policyResult := range result.Results {
				if policyResult.Skipped || policyResult.Action == nil {
					continue
				}
				mods, ok := policyResult.Action.(policy.UpstreamRequestModifications)
				if !ok {
					continue
				}
				if mods.AnalyticsMetadata != nil {
					for key, value := range mods.AnalyticsMetadata {
						shortCircuitAnalyticsData[key] = value
					}
				}

				dropAction := mods.AnalyticsHeaderFilter
				if dropAction.Action != "" || len(dropAction.Headers) > 0 {
					originalHeaders := execCtx.requestBodyCtx.Headers.GetAll()
					shortCircuitAnalyticsData["request_headers"] = finalizeAnalyticsHeaders(dropAction, originalHeaders)
				}
			}
			if immResp.AnalyticsMetadata != nil {
				for key, value := range immResp.AnalyticsMetadata {
					shortCircuitAnalyticsData[key] = value
				}
			}

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
			analyticsStruct, err := buildAnalyticsStruct(shortCircuitAnalyticsData, execCtx)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
			}
			response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, immResp.DynamicMetadata)
			return nil, nil, nil, nil, nil, nil, response, nil
		}
	}

	// Build final action by resolving conflicting header operations
	headerOps := make(map[string][]*headerOp)
	analyticsData = make(map[string]any)
	dynamicMetadata = make(map[string]map[string]interface{})
	headerMutation = &extprocv3.HeaderMutation{}
	var finalBodyLength int
	bodyModified := false
	var targetUpstreamName *string // Track the target upstream for cluster routing

	path := execCtx.requestBodyCtx.Path

	// Collect all operations in order
	for _, policyResult := range result.Results {
		if policyResult.Skipped {
			continue
		}

		if policyResult.Action != nil {
			if mods, ok := policyResult.Action.(policy.UpstreamRequestModifications); ok {
				// Collect SetHeader operations (deprecated flat field)
				for key, value := range mods.HeadersToSet {
					headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "set", value: value})
				}

	// Collect RemoveHeader operations (deprecated flat field)
				for _, key := range mods.HeadersToRemove {
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

				if mods.QueryParametersToAdd != nil {
					path = utils.AddQueryParametersToPath(path, mods.QueryParametersToAdd)
					pathMutation = &path
				}

				if mods.QueryParametersToRemove != nil {
					path = utils.RemoveQueryParametersFromPath(path, mods.QueryParametersToRemove)
					pathMutation = &path
				}

				if mods.Path != nil {
					pathMutation = mods.Path
				}

				if mods.Method != nil {
					methodMutation = mods.Method
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

				dropAction := mods.AnalyticsHeaderFilter
				if dropAction.Action != "" || len(dropAction.Headers) > 0 {
					slog.Debug("Translator: Found analytics header filter action (REQUEST)",
						"action", dropAction.Action,
						"headers", dropAction.Headers,
						"headers_count", len(dropAction.Headers))

					// Set the finalized headers to the analytics data
					originalHeaders := execCtx.requestBodyCtx.Headers.GetAll()
					finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
					analyticsData["request_headers"] = finalizedHeaders
					execCtx.analyticsMetadata["request_headers"] = finalizedHeaders
				}

				// Handle UpstreamName for dynamic cluster routing (last one wins)
				if mods.UpstreamName != nil && *mods.UpstreamName != "" {
					targetUpstreamName = mods.UpstreamName
				}
			}
		}
	}

	// Handle dynamic cluster routing via header.
	// When a policy sets UpstreamName, we set the x-target-upstream header directly.
	// ClearRouteCache is always enabled so Envoy can re-evaluate routing.
	if targetUpstreamName != nil {
		// Policy explicitly set the upstream - add the prefix, kind, and API ID for scoped cluster name
		// Format: upstream_<kind>_<apiId>_<sanitizedDefName>
		// Sanitize the definition name (replace dots and colons for valid Envoy cluster name)
		apiKind := execCtx.sharedCtx.APIKind
		apiId := execCtx.sharedCtx.APIId
		sanitizedDefName := sanitizeUpstreamDefinitionName(*targetUpstreamName)
		clusterName := constants.UpstreamDefinitionClusterPrefix + apiKind + "_" + apiId + "_" + sanitizedDefName

		// Set the x-target-upstream header directly for cluster_header routing
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{
			opType: "set",
			value:  clusterName,
		})

		// Store in execution context for potential response phase use
		extProcNS := constants.ExtProcFilterName
		if execCtx.dynamicMetadata[extProcNS] == nil {
			execCtx.dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		if dynamicMetadata[extProcNS] == nil {
			dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		execCtx.dynamicMetadata[extProcNS][constants.TargetUpstreamClusterKey] = clusterName
		execCtx.dynamicMetadata[extProcNS][constants.TargetUpstreamNameKey] = *targetUpstreamName

		// Pass api_context and upstream_base_path in dynamic metadata for Lua filter
		// Lua filter's handle:metadata() only works for envoy.filters.http.lua namespace,
		// so we must pass these via dynamic metadata
		dynamicMetadata[extProcNS]["api_context"] = execCtx.apiContext
		dynamicMetadata[extProcNS]["upstream_base_path"] = execCtx.upstreamBasePath

		// When UpstreamName is used, provide the target upstream's base path for Lua filter
		// The Lua filter handles path transformation and needs to know which upstream path to use
		slog.Info("UpstreamName: checking upstreamDefinitionPaths",
			"targetUpstream", *targetUpstreamName,
			"hasUpstreamDefPaths", execCtx.upstreamDefinitionPaths != nil,
			"upstreamDefPaths", execCtx.upstreamDefinitionPaths,
			"apiContext", execCtx.apiContext)
		if execCtx.upstreamDefinitionPaths != nil {
			if targetUpstreamPath, ok := execCtx.upstreamDefinitionPaths[*targetUpstreamName]; ok {
				// Set in both local dynamicMetadata (for response to Envoy) and execCtx (for response phase)
				dynamicMetadata[extProcNS]["target_upstream_base_path"] = targetUpstreamPath
				execCtx.dynamicMetadata[extProcNS]["target_upstream_base_path"] = targetUpstreamPath
				slog.Info("UpstreamName: set target upstream base path",
					"targetUpstream", *targetUpstreamName,
					"targetUpstreamPath", targetUpstreamPath)

				// Set target_path to trigger Lua filter path transformation if not already set
				// If request-rewrite policy set it, use that; otherwise use original request path
				if _, hasTargetPath := dynamicMetadata[extProcNS]["request_transformation.target_path"]; !hasTargetPath {
					dynamicMetadata[extProcNS]["request_transformation.target_path"] = execCtx.requestBodyCtx.Path
					execCtx.dynamicMetadata[extProcNS]["request_transformation.target_path"] = execCtx.requestBodyCtx.Path
					slog.Info("UpstreamName: set target_path", "path", execCtx.requestBodyCtx.Path)
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
		// No policy set upstream, but route uses cluster_header routing
		// Set the default cluster header so Envoy knows which cluster to use
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{
			opType: "set",
			value:  execCtx.defaultUpstreamCluster,
		})
	}

	// Always pass api_context and upstream_base_path in dynamic metadata when path rewrite is requested
	// This allows the Lua filter to properly compute the final upstream path
	if pathMutation != nil {
		extProcNS := constants.ExtProcFilterName
		if dynamicMetadata[extProcNS] == nil {
			dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		// Only set if not already set by UpstreamName handling above
		if _, ok := dynamicMetadata[extProcNS]["api_context"]; !ok {
			dynamicMetadata[extProcNS]["api_context"] = execCtx.apiContext
		}
		if _, ok := dynamicMetadata[extProcNS]["upstream_base_path"]; !ok {
			dynamicMetadata[extProcNS]["upstream_base_path"] = execCtx.upstreamBasePath
		}
	}

	// Re-compress request body if a policy modified it, to preserve the original Content-Encoding.
	// If no policy modified the body, the original compressed bytes are forwarded unchanged.
	if bodyModified && execCtx.requestContentEncoding != "" {
		originalBody := bodyMutation.Mutation.(*extprocv3.BodyMutation_Body).Body
		recompressed, err := recompressBody(originalBody, execCtx.requestContentEncoding)
		if err != nil {
			slog.Warn("Failed to re-compress request body, sending uncompressed",
				"encoding", execCtx.requestContentEncoding,
				"error", err,
			)
			// Remove Content-Encoding so the upstream does not try to decompress an uncompressed body.
			headerOps["content-encoding"] = append(headerOps["content-encoding"], &headerOp{opType: "remove", value: ""})
			finalBodyLength = len(originalBody)
		} else {
			bodyMutation.Mutation.(*extprocv3.BodyMutation_Body).Body = recompressed
			finalBodyLength = len(recompressed)
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

	return headerMutation, bodyMutation, analyticsData, dynamicMetadata, pathMutation, methodMutation, nil, nil
}

// TranslateRequestHeaderActions converts a RequestHeaderExecutionResult (from ExecuteRequestHeaderPolicies)
// to an ext_proc response. The ModeOverride instructs Envoy on how to deliver the remaining phases.
func TranslateRequestHeaderActions(result *executor.RequestHeaderExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
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
			analyticsStruct, err := buildAnalyticsStruct(immResp.AnalyticsMetadata, execCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
			}
			response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, immResp.DynamicMetadata)
			return response, nil
		}
	}

	// Collect header ops, path/method mutations, and analytics from all results
	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}

	path := execCtx.requestBodyCtx.Path
	var pathMutation *string
	var methodMutation *string
	var targetUpstreamName *string

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "set", value: v})
		}
		for _, k := range mods.HeadersToRemove {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "remove", value: ""})
		}
		if mods.QueryParametersToAdd != nil {
			path = utils.AddQueryParametersToPath(path, mods.QueryParametersToAdd)
			pathMutation = &path
		}
		if mods.QueryParametersToRemove != nil {
			path = utils.RemoveQueryParametersFromPath(path, mods.QueryParametersToRemove)
			pathMutation = &path
		}
		if mods.Path != nil {
			pathMutation = mods.Path
		}
		if mods.Method != nil {
			methodMutation = mods.Method
		}
		if mods.UpstreamName != nil && *mods.UpstreamName != "" {
			targetUpstreamName = mods.UpstreamName
		}
		if mods.AnalyticsMetadata != nil {
			for key, value := range mods.AnalyticsMetadata {
				analyticsData[key] = value
				execCtx.analyticsMetadata[key] = value
			}
		}
		if mods.DynamicMetadata != nil {
			mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
			mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
		}
		dropAction := mods.AnalyticsHeaderFilter
		if dropAction.Action != "" || len(dropAction.Headers) > 0 {
			originalHeaders := execCtx.requestBodyCtx.Headers.GetAll()
			finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
			analyticsData["request_headers"] = finalizedHeaders
			execCtx.analyticsMetadata["request_headers"] = finalizedHeaders
		}
	}

	// Handle dynamic cluster routing
	if targetUpstreamName != nil {
		apiKind := execCtx.sharedCtx.APIKind
		apiId := execCtx.sharedCtx.APIId
		sanitizedDefName := sanitizeUpstreamDefinitionName(*targetUpstreamName)
		clusterName := constants.UpstreamDefinitionClusterPrefix + apiKind + "_" + apiId + "_" + sanitizedDefName
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{opType: "set", value: clusterName})
		extProcNS := constants.ExtProcFilterName
		if execCtx.dynamicMetadata[extProcNS] == nil {
			execCtx.dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		execCtx.dynamicMetadata[extProcNS][constants.TargetUpstreamClusterKey] = clusterName
		execCtx.dynamicMetadata[extProcNS][constants.TargetUpstreamNameKey] = *targetUpstreamName
		dynamicMetadata[extProcNS] = map[string]interface{}{
			"api_context":        execCtx.apiContext,
			"upstream_base_path": execCtx.upstreamBasePath,
		}
		if execCtx.upstreamDefinitionPaths != nil {
			if targetUpstreamPath, ok := execCtx.upstreamDefinitionPaths[*targetUpstreamName]; ok {
				if pathMutation == nil {
					computedPath := strings.TrimSuffix(targetUpstreamPath, "/") + execCtx.requestBodyCtx.Path
					pathMutation = &computedPath
					dynamicMetadata[extProcNS]["request_transformation.target_path"] = computedPath
					execCtx.dynamicMetadata[extProcNS]["request_transformation.target_path"] = computedPath
				}
			}
		}
	} else if execCtx.defaultUpstreamCluster != "" {
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{opType: "set", value: execCtx.defaultUpstreamCluster})
		extProcNS := constants.ExtProcFilterName
		if execCtx.dynamicMetadata[extProcNS] == nil {
			execCtx.dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		dynamicMetadata[extProcNS] = map[string]interface{}{
			"api_context":        execCtx.apiContext,
			"upstream_base_path": execCtx.upstreamBasePath,
		}
	}

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
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, pathMutation, methodMutation, dynamicMetadata)

	return response, nil
}

// TranslateRequestHeaderActionsWithBodyMerge merges results from both the request-headers
// phase and the request-body phase into a single RequestHeaders ext_proc response.
// This is used when a request carries no body (GET, Content-Length: 0, EndOfStream in headers)
// so body policies are executed inline during the headers phase.
//
// The caller must set execCtx.requestBodyProcessedInline = true before calling this function
// so that getModeOverride() instructs Envoy to skip the RequestBody phase (mode = NONE).
func TranslateRequestHeaderActionsWithBodyMerge(
	headerResult *executor.RequestHeaderExecutionResult,
	bodyResult *executor.RequestExecutionResult,
	execCtx *PolicyExecutionContext,
) (*extprocv3.ProcessingResponse, error) {
	// Only body policies can short-circuit here: this function is called exclusively
	// when header policies did NOT short-circuit (see processRequestBodyForEmptyRequest).
	if bodyResult.ShortCircuited && bodyResult.FinalAction != nil {
		if immResp, ok := bodyResult.FinalAction.(policy.ImmediateResponse); ok {
			response := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &extprocv3.ImmediateResponse{
						Status:  &typev3.HttpStatus{Code: typev3.StatusCode(immResp.StatusCode)},
						Headers: buildHeaderValueOptions(immResp.Headers),
						Body:    immResp.Body,
					},
				},
			}
			analyticsStruct, err := buildAnalyticsStruct(immResp.AnalyticsMetadata, execCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
			}
			response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, immResp.DynamicMetadata)
			return response, nil
		}
	}

	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}
	var pathMutation *string
	var methodMutation *string
	var bodyMutation *extprocv3.BodyMutation
	var finalBodyLength int
	bodyModified := false
	var targetUpstreamName *string

	path := execCtx.requestBodyCtx.Path

	// Collect mutations from header-phase policies (UpstreamRequestHeaderModifications).
	for _, pr := range headerResult.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "set", value: v})
		}
		for _, k := range mods.HeadersToRemove {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "remove", value: ""})
		}
		if mods.QueryParametersToAdd != nil {
			path = utils.AddQueryParametersToPath(path, mods.QueryParametersToAdd)
			pathMutation = &path
		}
		if mods.QueryParametersToRemove != nil {
			path = utils.RemoveQueryParametersFromPath(path, mods.QueryParametersToRemove)
			pathMutation = &path
		}
		if mods.Path != nil {
			pathMutation = mods.Path
		}
		if mods.Method != nil {
			methodMutation = mods.Method
		}
		if mods.UpstreamName != nil && *mods.UpstreamName != "" {
			targetUpstreamName = mods.UpstreamName
		}
		if mods.AnalyticsMetadata != nil {
			for key, value := range mods.AnalyticsMetadata {
				analyticsData[key] = value
				execCtx.analyticsMetadata[key] = value
			}
		}
		if mods.DynamicMetadata != nil {
			mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
			mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
		}
		dropAction := mods.AnalyticsHeaderFilter
		if dropAction.Action != "" || len(dropAction.Headers) > 0 {
			originalHeaders := execCtx.requestBodyCtx.Headers.GetAll()
			finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
			analyticsData["request_headers"] = finalizedHeaders
			execCtx.analyticsMetadata["request_headers"] = finalizedHeaders
		}
	}

	// Collect mutations from body-phase policies (UpstreamRequestModifications).
	for _, pr := range bodyResult.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.UpstreamRequestModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "set", value: v})
		}
		for _, k := range mods.HeadersToRemove {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "remove", value: ""})
		}
		if mods.Body != nil {
			bodyMutation = &extprocv3.BodyMutation{
				Mutation: &extprocv3.BodyMutation_Body{Body: mods.Body},
			}
			finalBodyLength = len(mods.Body)
			bodyModified = true
		}
		if mods.QueryParametersToAdd != nil {
			path = utils.AddQueryParametersToPath(path, mods.QueryParametersToAdd)
			pathMutation = &path
		}
		if mods.QueryParametersToRemove != nil {
			path = utils.RemoveQueryParametersFromPath(path, mods.QueryParametersToRemove)
			pathMutation = &path
		}
		if mods.Path != nil {
			pathMutation = mods.Path
		}
		if mods.Method != nil {
			methodMutation = mods.Method
		}
		if mods.UpstreamName != nil && *mods.UpstreamName != "" {
			targetUpstreamName = mods.UpstreamName
		}
		if mods.AnalyticsMetadata != nil {
			for key, value := range mods.AnalyticsMetadata {
				analyticsData[key] = value
				execCtx.analyticsMetadata[key] = value
			}
		}
		if mods.DynamicMetadata != nil {
			mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
			mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
		}
		dropAction := mods.AnalyticsHeaderFilter
		if dropAction.Action != "" || len(dropAction.Headers) > 0 {
			originalHeaders := execCtx.requestBodyCtx.Headers.GetAll()
			finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
			analyticsData["request_headers"] = finalizedHeaders
			execCtx.analyticsMetadata["request_headers"] = finalizedHeaders
		}
	}

	// Handle dynamic cluster routing (last UpstreamName wins across both phases).
	if targetUpstreamName != nil {
		apiKind := execCtx.sharedCtx.APIKind
		apiId := execCtx.sharedCtx.APIId
		sanitizedDefName := sanitizeUpstreamDefinitionName(*targetUpstreamName)
		clusterName := constants.UpstreamDefinitionClusterPrefix + apiKind + "_" + apiId + "_" + sanitizedDefName
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{opType: "set", value: clusterName})
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
		if execCtx.upstreamDefinitionPaths != nil {
			if targetUpstreamPath, ok := execCtx.upstreamDefinitionPaths[*targetUpstreamName]; ok {
				if pathMutation == nil {
					computedPath := strings.TrimSuffix(targetUpstreamPath, "/") + execCtx.requestBodyCtx.Path
					pathMutation = &computedPath
					dynamicMetadata[extProcNS]["request_transformation.target_path"] = computedPath
					execCtx.dynamicMetadata[extProcNS]["request_transformation.target_path"] = computedPath
				}
			}
		}
	} else if execCtx.defaultUpstreamCluster != "" {
		headerOps[constants.TargetUpstreamHeader] = append(headerOps[constants.TargetUpstreamHeader], &headerOp{opType: "set", value: execCtx.defaultUpstreamCluster})
		extProcNS := constants.ExtProcFilterName
		if execCtx.dynamicMetadata[extProcNS] == nil {
			execCtx.dynamicMetadata[extProcNS] = make(map[string]interface{})
		}
		dynamicMetadata[extProcNS] = map[string]interface{}{
			"api_context":        execCtx.apiContext,
			"upstream_base_path": execCtx.upstreamBasePath,
		}
	}

	if bodyModified {
		delete(headerOps, "content-length")
	}
	mergeHeaderMutations(headerMutation, headerOps)
	if bodyModified {
		setContentLengthHeader(headerMutation, finalBodyLength)
	}

	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation:  headerMutation,
					BodyMutation:    bodyMutation,
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
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, pathMutation, methodMutation, dynamicMetadata)

	return response, nil
}

// TranslateResponseHeaderActions converts a ResponseHeaderExecutionResult (from ExecuteResponseHeaderPolicies)
// to an ext_proc response. The ModeOverride instructs Envoy on how to deliver the response body.
// Callers may further override ModeOverride for streaming (FULL_DUPLEX_STREAMED).
func TranslateResponseHeaderActions(result *executor.ResponseHeaderExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
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
			analyticsStruct, err := buildAnalyticsStruct(immResp.AnalyticsMetadata, execCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
			}
			response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, immResp.DynamicMetadata)
			return response, nil
		}
	}

	// Collect header ops and analytics from all results
	headerOps := make(map[string][]*headerOp)
	analyticsData := make(map[string]any)
	dynamicMetadata := make(map[string]map[string]interface{})
	headerMutation := &extprocv3.HeaderMutation{}

	for _, pr := range result.Results {
		if pr.Skipped || pr.Action == nil {
			continue
		}
		mods, ok := pr.Action.(policy.DownstreamResponseHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "set", value: v})
		}
		for _, k := range mods.HeadersToRemove {
			headerOps[strings.ToLower(k)] = append(headerOps[strings.ToLower(k)], &headerOp{opType: "remove", value: ""})
		}
		if mods.AnalyticsMetadata != nil {
			for key, value := range mods.AnalyticsMetadata {
				analyticsData[key] = value
				execCtx.analyticsMetadata[key] = value
			}
		}
		if mods.DynamicMetadata != nil {
			mergeDynamicMetadata(dynamicMetadata, mods.DynamicMetadata)
			mergeDynamicMetadata(execCtx.dynamicMetadata, mods.DynamicMetadata)
		}
		dropAction := mods.AnalyticsHeaderFilter
		if dropAction.Action != "" || len(dropAction.Headers) > 0 {
			originalHeaders := execCtx.responseBodyCtx.ResponseHeaders.GetAll()
			finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
			analyticsData["response_headers"] = finalizedHeaders
			execCtx.analyticsMetadata["response_headers"] = finalizedHeaders
		}
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
		ModeOverride: execCtx.getModeOverride(),
	}

	analyticsStruct, err := buildAnalyticsStruct(analyticsData, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics metadata: %w", err)
	}
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, dynamicMetadata)

	return response, nil
}

// TranslateRequestHeadersActions converts request headers execution result to ext_proc response
func TranslateRequestHeadersActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, path, method, immediateResp, err := translateRequestActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}
	if immediateResp != nil {
		return immediateResp, nil
	}

	// Build ProcessingResponse for request headers
	// Always set ClearRouteCache to true so Envoy re-evaluates routing after ext_proc modifications
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation:  headerMutation,
					BodyMutation:    bodyMutation,
					ClearRouteCache: true,
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
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, path, method, dynamicMetadata)

	return response, nil
}

// TranslateRequestBodyActions converts request body execution result to ext_proc response
func TranslateRequestBodyActions(result *executor.RequestExecutionResult, chain *registry.PolicyChain, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, path, method, immediateResp, err := translateRequestActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}
	if immediateResp != nil {
		return immediateResp, nil
	}

	// Build ProcessingResponse for request body
	// Always set ClearRouteCache to true so Envoy re-evaluates routing after ext_proc modifications
	response := &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation:  headerMutation,
					BodyMutation:    bodyMutation,
					ClearRouteCache: true,
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
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, path, method, dynamicMetadata)

	return response, nil
}

// translateResponseActionsCore is the shared implementation for response translation
func translateResponseActionsCore(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (
	headerMutation *extprocv3.HeaderMutation,
	bodyMutation *extprocv3.BodyMutation,
	analyticsData map[string]any,
	dynamicMetadata map[string]map[string]interface{},
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
				return nil, nil, nil, nil, nil, fmt.Errorf("failed to build analytics metadata for immediate response: %w", err)
			}
			response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, immResp.DynamicMetadata)
			return nil, nil, nil, nil, response, nil
		}
	}

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
			if mods, ok := policyResult.Action.(policy.DownstreamResponseModifications); ok {
				// Collect SetHeader operations (deprecated flat field)
				for key, value := range mods.HeadersToSet {
					headerOps[strings.ToLower(key)] = append(headerOps[strings.ToLower(key)], &headerOp{opType: "set", value: value})
				}

	// Collect RemoveHeader operations (deprecated flat field)
				for _, key := range mods.HeadersToRemove {
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

				// Handle status code modification via :status pseudo-header (last one wins)
				if mods.StatusCode != nil {
					headerOps[":status"] = append(headerOps[":status"], &headerOp{opType: "set", value: fmt.Sprintf("%d", *mods.StatusCode)})
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

				dropAction := mods.AnalyticsHeaderFilter
				if dropAction.Action != "" || len(dropAction.Headers) > 0 {
					slog.Debug("Translator: Found analytics header filter action (RESPONSE)",
						"action", dropAction.Action,
						"headers", dropAction.Headers,
						"headers_count", len(dropAction.Headers))

					originalHeaders := execCtx.responseBodyCtx.ResponseHeaders.GetAll()
					finalizedHeaders := finalizeAnalyticsHeaders(dropAction, originalHeaders)
					analyticsData["response_headers"] = finalizedHeaders

					// Include request_headers from execution context if it was set in a previous phase
					if _, exists := execCtx.analyticsMetadata["request_headers"]; exists {
						analyticsData["request_headers"] = execCtx.analyticsMetadata["request_headers"]
					}
				}
			}
		}
	}

	// Re-compress body if a policy modified it and the original response was compressed.
	// Policies receive decompressed bodies; the downstream client expects the original encoding.
	if bodyModified && execCtx.responseContentEncoding != "" {
		originalBody := bodyMutation.Mutation.(*extprocv3.BodyMutation_Body).Body
		recompressed, err := recompressBody(originalBody, execCtx.responseContentEncoding)
		if err != nil {
			slog.Warn("Failed to re-compress response body, sending uncompressed",
				"encoding", execCtx.responseContentEncoding,
				"error", err,
			)
			// Remove Content-Encoding so the client does not try to decompress an uncompressed body.
			headerOps["content-encoding"] = append(headerOps["content-encoding"], &headerOp{opType: "remove", value: ""})
			finalBodyLength = len(originalBody)
		} else {
			bodyMutation.Mutation.(*extprocv3.BodyMutation_Body).Body = recompressed
			finalBodyLength = len(recompressed)
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

	return headerMutation, bodyMutation, analyticsData, dynamicMetadata, nil, nil
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
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, immediateResp, err := translateResponseActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}
	if immediateResp != nil {
		return immediateResp, nil
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
	response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, dynamicMetadata)

	return response, nil
}

// TranslateResponseBodyActions converts response body execution result to ext_proc response
func TranslateResponseBodyActions(result *executor.ResponseExecutionResult, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	headerMutation, bodyMutation, analyticsData, dynamicMetadata, immediateResp, err := translateResponseActionsCore(result, execCtx)
	if err != nil {
		return nil, err
	}
	if immediateResp != nil {
		return immediateResp, nil
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
		response.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, dynamicMetadata)
		return response, nil
	}

	if len(dynamicMetadata) > 0 {
		response.DynamicMetadata = buildDynamicMetadata(nil, nil, nil, dynamicMetadata)
	}

	return response, nil
}

// buildDynamicMetadata creates the dynamic metadata structure for analytics and path/method rewrite
func buildDynamicMetadata(analyticsStruct *structpb.Struct, path *string, method *string, extra map[string]map[string]interface{}) *structpb.Struct {
	namespaces := make(map[string]*structpb.Struct)

	baseFields := make(map[string]*structpb.Value)
	if analyticsStruct != nil {
		baseFields["analytics_data"] = structpb.NewStructValue(analyticsStruct)
	}
	if path != nil {
		baseFields["path"] = structpb.NewStringValue(*path)
	}
	if method != nil {
		baseFields["method"] = structpb.NewStringValue(*method)
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
			delete(metaStruct.Fields, "method")
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

// ─── Streaming chunk translators ──────────────────────────────────────────────

// TranslateStreamingRequestChunkAction converts a streaming request execution result
// into an ext_proc StreamedBodyResponse.
func TranslateStreamingRequestChunkAction(result *executor.StreamingRequestExecutionResult, originalChunk *policy.StreamBody, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	var outputBody []byte
	if result.FinalChunk != nil {
		outputBody = result.FinalChunk.Chunk
	} else {
		outputBody = originalChunk.Chunk
	}

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
		resp.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, dynamicMetadata)
		return resp, nil
	}
	if len(dynamicMetadata) > 0 {
		resp.DynamicMetadata = buildDynamicMetadata(nil, nil, nil, dynamicMetadata)
	}

	return resp, nil
}

// TranslateStreamingResponseChunkAction converts a streaming response execution result
// into an ext_proc StreamedBodyResponse.
func TranslateStreamingResponseChunkAction(result *executor.StreamingResponseExecutionResult, originalChunk *policy.StreamBody, execCtx *PolicyExecutionContext) (*extprocv3.ProcessingResponse, error) {
	var outputBody []byte
	if result.FinalChunk != nil {
		outputBody = result.FinalChunk.Chunk
	} else {
		outputBody = originalChunk.Chunk
	}

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
		resp.DynamicMetadata = buildDynamicMetadata(analyticsStruct, nil, nil, dynamicMetadata)
		return resp, nil
	}
	if len(dynamicMetadata) > 0 {
		resp.DynamicMetadata = buildDynamicMetadata(nil, nil, nil, dynamicMetadata)
	}

	return resp, nil
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

// sanitizeUpstreamDefinitionName sanitizes an upstream definition name for use in Envoy cluster names.
// Envoy cluster names cannot contain dots or colons.
func sanitizeUpstreamDefinitionName(name string) string {
	sanitized := strings.ReplaceAll(name, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	return sanitized
}

// computeUpstreamPath computes the final upstream path by stripping the API context
// from the current path and prepending the target upstream's path.
// Example: currentPath="/weather/v1.0/pets", apiContext="/weather/v1.0", upstreamPath="/alternate-backend"
// Result: "/alternate-backend/pets"
func computeUpstreamPath(currentPath, apiContext, upstreamPath string) string {
	if currentPath == "" {
		return ""
	}

	// Default values
	if apiContext == "" {
		apiContext = "/"
	}
	if upstreamPath == "" {
		upstreamPath = ""
	}

	// Strip the API context prefix from the current path
	relativePath := currentPath
	if apiContext != "/" {
		if strings.HasPrefix(currentPath, apiContext) {
			relativePath = strings.TrimPrefix(currentPath, apiContext)
			if relativePath == "" {
				relativePath = "/"
			}
		}
	}

	// Prepend the upstream path
	if upstreamPath == "" || upstreamPath == "/" {
		return relativePath
	}

	// Handle trailing slash in upstreamPath and leading slash in relativePath
	if strings.HasSuffix(upstreamPath, "/") && strings.HasPrefix(relativePath, "/") {
		return upstreamPath + relativePath[1:]
	}
	if !strings.HasSuffix(upstreamPath, "/") && !strings.HasPrefix(relativePath, "/") {
		return upstreamPath + "/" + relativePath
	}
	return upstreamPath + relativePath
}
