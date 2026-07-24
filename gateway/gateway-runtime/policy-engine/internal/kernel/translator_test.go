/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// =============================================================================
// buildHeaderMutationFromOps Tests
// =============================================================================

func TestBuildHeaderMutationFromOps_Empty(t *testing.T) {
	ops := map[string][]*headerOp{}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	assert.Empty(t, result.SetHeaders)
	assert.Empty(t, result.RemoveHeaders)
}

func TestBuildHeaderMutationFromOps_SingleSet(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-custom": {{opType: "set", value: "value1"}},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	require.Len(t, result.SetHeaders, 1)
	assert.Equal(t, "x-custom", result.SetHeaders[0].Header.Key)
	assert.Equal(t, []byte("value1"), result.SetHeaders[0].Header.RawValue)
}

func TestBuildHeaderMutationFromOps_SingleRemove(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-remove-me": {{opType: "remove", value: ""}},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	require.Len(t, result.RemoveHeaders, 1)
	assert.Equal(t, "x-remove-me", result.RemoveHeaders[0])
}

func TestBuildHeaderMutationFromOps_SingleAppend(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-multi": {{opType: "append", value: "val1"}},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	require.Len(t, result.SetHeaders, 1)
	assert.Equal(t, "x-multi", result.SetHeaders[0].Header.Key)
}

func TestAddAppendHeaderOps_ProducesAppendOps(t *testing.T) {
	headerOps := map[string][]*headerOp{}
	addAppendHeaderOps(headerOps, map[string][]string{
		"X-Header-Add": {"add-appends-values"},
	})

	// Header name is lower-cased and the op is "append" (preserves existing values).
	require.Len(t, headerOps["x-header-add"], 1)
	assert.Equal(t, "append", headerOps["x-header-add"][0].opType)
	assert.Equal(t, "add-appends-values", headerOps["x-header-add"][0].value)

	// End-to-end: an append op next to a pre-existing client value must use
	// APPEND_IF_EXISTS_OR_ADD so Envoy keeps the original value.
	result := buildHeaderMutationFromOps(headerOps)
	require.Len(t, result.SetHeaders, 1)
	assert.Equal(t, corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD, result.SetHeaders[0].AppendAction)
	// Envoy's ext_proc (through at least v1.38) only honors the deprecated `append`
	// BoolValue, not append_action — without it the mutation is applied as overwrite.
	require.NotNil(t, result.SetHeaders[0].Append)
	assert.True(t, result.SetHeaders[0].Append.GetValue())
}

func TestAddAppendHeaderOps_MultipleValuesAndNilNoop(t *testing.T) {
	headerOps := map[string][]*headerOp{}
	// Multiple values for one header → one append op each.
	addAppendHeaderOps(headerOps, map[string][]string{"X-Multi": {"a", "b"}})
	require.Len(t, headerOps["x-multi"], 2)

	// nil/empty map is a no-op (set/remove-only policies are unaffected).
	before := len(headerOps)
	addAppendHeaderOps(headerOps, nil)
	assert.Equal(t, before, len(headerOps))
}

func TestBuildHeaderMutationFromOps_SetThenAppend(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-header": {
			{opType: "set", value: "initial"},
			{opType: "append", value: "extra1"},
			{opType: "append", value: "extra2"},
		},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	// Should have set + 2 appends = 3 entries
	require.Len(t, result.SetHeaders, 3)
	// The set entry must replace (deprecated `append` unset/false); the append
	// entries must carry append=true for ext_proc to preserve prior values.
	assert.False(t, result.SetHeaders[0].Append.GetValue())
	assert.True(t, result.SetHeaders[1].Append.GetValue())
	assert.True(t, result.SetHeaders[2].Append.GetValue())
}

func TestBuildHeaderMutationFromOps_SetAfterSet(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-header": {
			{opType: "set", value: "first"},
			{opType: "set", value: "second"},
		},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	// Last set wins
	require.Len(t, result.SetHeaders, 1)
	assert.Equal(t, []byte("second"), result.SetHeaders[0].Header.RawValue)
}

func TestBuildHeaderMutationFromOps_RemoveAfterSet(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-header": {
			{opType: "set", value: "value"},
			{opType: "remove", value: ""},
		},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	// Last op is remove, so only remove should be sent
	assert.Empty(t, result.SetHeaders)
	require.Len(t, result.RemoveHeaders, 1)
}

func TestBuildHeaderMutationFromOps_AppendAfterRemove(t *testing.T) {
	ops := map[string][]*headerOp{
		"x-header": {
			{opType: "set", value: "initial"},
			{opType: "remove", value: ""},
			{opType: "append", value: "new-value"},
		},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	// Should only have the append (remove discarded since we're appending after)
	require.Len(t, result.SetHeaders, 1)
	assert.Equal(t, []byte("new-value"), result.SetHeaders[0].Header.RawValue)
}

func TestBuildHeaderMutationFromOps_MultipleHeaders(t *testing.T) {
	ops := map[string][]*headerOp{
		"header-a": {{opType: "set", value: "a"}},
		"header-b": {{opType: "remove", value: ""}},
		"header-c": {{opType: "append", value: "c1"}, {opType: "append", value: "c2"}},
	}

	result := buildHeaderMutationFromOps(ops)

	require.NotNil(t, result)
	// header-a: 1 set, header-c: 2 appends = 3 total
	assert.Len(t, result.SetHeaders, 3)
	assert.Len(t, result.RemoveHeaders, 1)
}

// =============================================================================
// buildHeaderValueOptions Tests
// =============================================================================

func TestBuildHeaderValueOptions_Empty(t *testing.T) {
	result := buildHeaderValueOptions(map[string]string{})

	assert.Nil(t, result)
}

func TestBuildHeaderValueOptions_SingleHeader(t *testing.T) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	result := buildHeaderValueOptions(headers)

	require.NotNil(t, result)
	require.Len(t, result.SetHeaders, 1)
	assert.Equal(t, "content-type", result.SetHeaders[0].Header.Key) // lowercase
}

func TestBuildHeaderValueOptions_MultipleHeaders(t *testing.T) {
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Custom":     "value",
	}

	result := buildHeaderValueOptions(headers)

	require.NotNil(t, result)
	assert.Len(t, result.SetHeaders, 2)
}

// =============================================================================
// setContentLengthHeader Tests
// =============================================================================

func TestSetContentLengthHeader_NilSetHeaders(t *testing.T) {
	mutation := &extprocv3.HeaderMutation{}

	setContentLengthHeader(mutation, 100)

	require.NotNil(t, mutation.SetHeaders)
	require.Len(t, mutation.SetHeaders, 1)
	assert.Equal(t, "content-length", mutation.SetHeaders[0].Header.Key)
	assert.Equal(t, []byte("100"), mutation.SetHeaders[0].Header.RawValue)
}

func TestSetContentLengthHeader_ExistingHeaders(t *testing.T) {
	mutation := &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			{Header: &corev3.HeaderValue{Key: "x-other"}},
		},
	}

	setContentLengthHeader(mutation, 50)

	require.Len(t, mutation.SetHeaders, 2)
}

// =============================================================================
// finalizeAnalyticsHeaders Tests
// =============================================================================

func TestFinalizeAnalyticsHeaders_NoAction(t *testing.T) {
	original := map[string][]string{
		"content-type":  {"application/json"},
		"authorization": {"Bearer token"},
		"x-custom":      {"value"},
	}

	dropAction := policy.DropHeaderAction{
		Action:  "",
		Headers: nil,
	}

	result := finalizeAnalyticsHeaders(dropAction, original)

	// Should return all headers
	assert.Equal(t, original, result)
}

func TestFinalizeAnalyticsHeaders_AllowMode(t *testing.T) {
	original := map[string][]string{
		"content-type":  {"application/json"},
		"authorization": {"Bearer token"},
		"x-custom":      {"value"},
	}

	dropAction := policy.DropHeaderAction{
		Action:  "allow",
		Headers: []string{"content-type", "x-custom"},
	}

	result := finalizeAnalyticsHeaders(dropAction, original)

	// Should only include allowed headers
	assert.Len(t, result, 2)
	assert.Contains(t, result, "content-type")
	assert.Contains(t, result, "x-custom")
	assert.NotContains(t, result, "authorization")
}

func TestFinalizeAnalyticsHeaders_DenyMode(t *testing.T) {
	original := map[string][]string{
		"content-type":  {"application/json"},
		"authorization": {"Bearer token"},
		"x-custom":      {"value"},
	}

	dropAction := policy.DropHeaderAction{
		Action:  "deny",
		Headers: []string{"authorization"},
	}

	result := finalizeAnalyticsHeaders(dropAction, original)

	// Should exclude denied headers
	assert.Len(t, result, 2)
	assert.Contains(t, result, "content-type")
	assert.Contains(t, result, "x-custom")
	assert.NotContains(t, result, "authorization")
}

func TestFinalizeAnalyticsHeaders_CaseInsensitive(t *testing.T) {
	original := map[string][]string{
		"Content-Type": {"application/json"},
	}

	dropAction := policy.DropHeaderAction{
		Action:  "allow",
		Headers: []string{"CONTENT-TYPE"},
	}

	result := finalizeAnalyticsHeaders(dropAction, original)

	// Should match case-insensitively
	assert.Len(t, result, 1)
	assert.Contains(t, result, "Content-Type")
}

func TestFinalizeAnalyticsHeaders_UnknownAction(t *testing.T) {
	original := map[string][]string{
		"content-type": {"application/json"},
	}

	dropAction := policy.DropHeaderAction{
		Action:  "unknown",
		Headers: []string{"content-type"},
	}

	result := finalizeAnalyticsHeaders(dropAction, original)

	// Unknown action should return all headers
	assert.Equal(t, original, result)
}

func TestFinalizeAnalyticsHeaders_EmptyHeaders(t *testing.T) {
	original := map[string][]string{
		"content-type": {"application/json"},
	}

	dropAction := policy.DropHeaderAction{
		Action:  "allow",
		Headers: []string{}, // Empty headers list
	}

	result := finalizeAnalyticsHeaders(dropAction, original)

	// With empty headers list, should return original
	assert.Equal(t, original, result)
}

// =============================================================================
// buildDynamicMetadata Tests
// =============================================================================

func TestBuildDynamicMetadata_WithoutPath(t *testing.T) {
	analyticsStruct, _ := structpb.NewStruct(map[string]interface{}{
		"key": "value",
	})

	result := buildDynamicMetadata(analyticsStruct, nil, nil)

	require.NotNil(t, result)
	require.NotNil(t, result.Fields)
	// Should have ext_proc filter (using the constant value)
	assert.Contains(t, result.Fields, "api_platform.policy_engine.envoy.filters.http.ext_proc")
}

func TestBuildDynamicMetadata_WithPath(t *testing.T) {
	analyticsStruct, _ := structpb.NewStruct(map[string]interface{}{
		"key": "value",
	})
	path := "/new/path"

	result := buildDynamicMetadata(analyticsStruct, &RequestMutations{Path: &path}, nil)

	require.NotNil(t, result)
	// Should include path in metadata
	extProc := result.Fields["api_platform.policy_engine.envoy.filters.http.ext_proc"].GetStructValue()
	require.NotNil(t, extProc)
	assert.Contains(t, extProc.Fields, "path")
	assert.Equal(t, "/new/path", extProc.Fields["path"].GetStringValue())
}

// =============================================================================
// Per-op upstream + dynamic-endpoint precedence (regression: no double base prefix)
// =============================================================================

// TestTranslateRequestHeaderActions_DynamicEndpointDoesNotBakeBasePath guards the
// per-op-upstream-ref behavior. When a dynamic-endpoint policy redirects a request to an
// upstream definition that has a base path, the kernel must pass the original request path
// plus target_upstream_base_path so Lua prepends the base exactly once.
func TestTranslateRequestHeaderActions_DynamicEndpointDoesNotBakeBasePath(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/per-op/v1.0/override",
		SharedContext: execCtx.sharedCtx,
	}
	execCtx.apiContext = "/per-op/v1.0"
	execCtx.upstreamBasePath = "/ref-svc" // the per-op route's default base path
	execCtx.upstreamDefinitionPaths = map[string]string{
		"op-policy-svc": "/op-policy-svc",
	}

	targetUpstream := "op-policy-svc"
	result := &executor.RequestHeaderExecutionResult{
		Results: []executor.RequestHeaderPolicyResult{
			{
				Action: policy.UpstreamRequestHeaderModifications{
					UpstreamName: &targetUpstream,
				},
			},
		},
	}

	resp, err := TranslateRequestHeaderActions(result, chain, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.DynamicMetadata)

	extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
	require.NotNil(t, extProc)

	// The target upstream's base path is advertised so the Lua prepends it exactly once.
	assert.Equal(t, "/op-policy-svc", extProc.Fields["target_upstream_base_path"].GetStringValue())
	// The ORIGINAL request path is handed to Lua via the single path metadata channel,
	// not a pre-computed base-prefixed path.
	assert.Equal(t, "/per-op/v1.0/override", extProc.Fields["path"].GetStringValue())
	assert.NotContains(t, extProc.Fields, "request_transformation.target_path")
}

// =============================================================================
// translateRequestActionsCore Tests
// =============================================================================

func TestTranslateRequestActionsCore_EmptyResult(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, rsl.ImmediateResp)
	assert.NotNil(t, rsl.HeaderMutation)
	assert.Nil(t, rsl.BodyMutation)
	assert.NotNil(t, rsl.AnalyticsData)
	assert.Nil(t, rsl.Mutations.Path)
}

func TestTranslateRequestActionsCore_WithSetHeaders(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					HeadersToSet: map[string]string{
						"x-custom": "value",
					},
				},
			},
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, rsl.ImmediateResp)
	require.NotNil(t, rsl.HeaderMutation)
	assert.Len(t, rsl.HeaderMutation.SetHeaders, 1)
}

func TestTranslateRequestActionsCore_WithBodyModification(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					Body: []byte("modified body"),
				},
			},
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, rsl.ImmediateResp)
	require.NotNil(t, rsl.BodyMutation)
	assert.Equal(t, []byte("modified body"), rsl.BodyMutation.GetBody())
	// Content-Length should be set
	require.NotNil(t, rsl.HeaderMutation)
	var foundContentLength bool
	for _, h := range rsl.HeaderMutation.SetHeaders {
		if h.Header.Key == "content-length" {
			foundContentLength = true
			assert.Equal(t, []byte("13"), h.Header.RawValue)
		}
	}
	assert.True(t, foundContentLength)
}

func TestTranslateRequestActionsCore_ShortCircuit(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	result := &executor.RequestExecutionResult{
		ShortCircuited: true,
		FinalAction: policy.ImmediateResponse{
			StatusCode: 403,
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: []byte(`{"error":"forbidden"}`),
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, rsl.ImmediateResp)

	immediate := rsl.ImmediateResp.GetImmediateResponse()
	require.NotNil(t, immediate)
	assert.Equal(t, uint32(403), uint32(immediate.Status.Code))
}

func TestTranslateRequestActionsCore_ShortCircuit_PreservesPriorRequestAnalyticsMetadata(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{
			APIId: "api-1",
		},
	}

	result := &executor.RequestExecutionResult{
		ShortCircuited: true,
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					AnalyticsMetadata: map[string]any{
						"x-wso2-application-id":   "app-1",
						"x-wso2-application-name": "app-one",
						"source":                  "request-policy",
					},
				},
			},
		},
		FinalAction: policy.ImmediateResponse{
			StatusCode: 403,
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: []byte(`{"error":"forbidden"}`),
			AnalyticsMetadata: map[string]any{
				"source": "immediate-response",
			},
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, rsl.ImmediateResp)

	extProcNamespace := rsl.ImmediateResp.DynamicMetadata.GetFields()[constants.ExtProcFilterName].GetStructValue()
	require.NotNil(t, extProcNamespace)

	analyticsData := extProcNamespace.GetFields()["analytics_data"].GetStructValue()
	require.NotNil(t, analyticsData)

	assert.Equal(t, "app-1", analyticsData.GetFields()["x-wso2-application-id"].GetStringValue())
	assert.Equal(t, "app-one", analyticsData.GetFields()["x-wso2-application-name"].GetStringValue())
	assert.Equal(t, "immediate-response", analyticsData.GetFields()["source"].GetStringValue())
}

// TestTranslateRequestHeaderActions_ShortCircuit_PreservesPriorAnalyticsMetadata
// covers the request-header phase short-circuit path: an earlier policy (e.g. the
// collector system policy capturing request headers) stamps request-header-phase
// analytics metadata, then an auth policy rejects the request with 401 and
// short-circuits the chain. The prior policy's metadata must survive onto the
// immediate response's dynamic metadata, otherwise the ALS access-log entry is
// missing it and the global traffic-logging publisher's line for that denied
// request would be incomplete.
func TestTranslateRequestHeaderActions_ShortCircuit_PreservesPriorAnalyticsMetadata(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{
			APIId: "api-1",
		},
	}

	result := &executor.RequestHeaderExecutionResult{
		ShortCircuited: true,
		Results: []executor.RequestHeaderPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestHeaderModifications{
					AnalyticsMetadata: map[string]any{
						"request_headers": `{"x-request-id":"req-1"}`,
					},
				},
			},
		},
		FinalAction: policy.ImmediateResponse{
			StatusCode: 401,
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: []byte(`{"error":"unauthorized"}`),
			AnalyticsMetadata: map[string]any{
				"source": "immediate-response",
			},
		},
	}

	resp, err := TranslateRequestHeaderActions(result, chain, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, resp)

	immediate := resp.GetImmediateResponse()
	require.NotNil(t, immediate)
	assert.Equal(t, uint32(401), uint32(immediate.Status.Code))

	extProcNamespace := resp.DynamicMetadata.GetFields()[constants.ExtProcFilterName].GetStructValue()
	require.NotNil(t, extProcNamespace)

	analyticsData := extProcNamespace.GetFields()["analytics_data"].GetStructValue()
	require.NotNil(t, analyticsData)

	// The metadata stamped by the earlier policy survives the short-circuit...
	assert.Equal(t, `{"x-request-id":"req-1"}`, analyticsData.GetFields()["request_headers"].GetStringValue())
	// ...and the immediate response's own analytics metadata is still present.
	assert.Equal(t, "immediate-response", analyticsData.GetFields()["source"].GetStringValue())
}

func TestTranslateRequestActionsCore_SkippedPolicy(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: true, // This policy was skipped
				Action: policy.UpstreamRequestModifications{
					HeadersToSet: map[string]string{
						"should-not-appear": "value",
					},
				},
			},
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	// Skipped policy actions should not be applied
	assert.Empty(t, rsl.HeaderMutation.SetHeaders)
}

func TestTranslateRequestActionsCore_WithQueryParams(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					QueryParametersToAdd: map[string][]string{
						"added": {"param"},
					},
				},
			},
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, rsl.Mutations.Path)
	assert.Contains(t, *rsl.Mutations.Path, "added=param")
}

func TestTranslateRequestActionsCore_WithPathOverride(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}

	newPath := "/new/path"
	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					Path: &newPath,
				},
			},
		},
	}

	rsl, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, rsl.Mutations.Path)
	assert.Equal(t, "/new/path", *rsl.Mutations.Path)
}

// =============================================================================
// translateResponseActionsCore Tests
// =============================================================================

func TestTranslateResponseActionsCore_ShortCircuit(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}
	execCtx.responseBodyCtx = &policy.ResponseContext{
		SharedContext: execCtx.sharedCtx,
		ResponseHeaders: policy.NewHeaders(map[string][]string{
			"content-type": {"application/json"},
		}),
		ResponseStatus: 200,
	}

	result := &executor.ResponseExecutionResult{
		ShortCircuited: true,
		FinalAction: policy.ImmediateResponse{
			StatusCode: 503,
			Headers:    map[string]string{"x-error": "upstream-fault"},
			Body:       []byte(`{"error":"service unavailable"}`),
		},
	}

	_, _, _, _, immResp, err := translateResponseActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, immResp, "short-circuit response must be returned")

	immediate := immResp.GetImmediateResponse()
	require.NotNil(t, immediate)
	assert.Equal(t, uint32(503), uint32(immediate.Status.Code))
	assert.Equal(t, []byte(`{"error":"service unavailable"}`), immediate.Body)
}

func TestTranslateResponseActionsCore_NoShortCircuit(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.sharedCtx = &policy.SharedContext{}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/test",
		SharedContext: execCtx.sharedCtx,
	}
	execCtx.responseBodyCtx = &policy.ResponseContext{
		SharedContext: execCtx.sharedCtx,
		ResponseHeaders: policy.NewHeaders(map[string][]string{
			"content-type": {"application/json"},
		}),
		ResponseStatus: 200,
	}

	result := &executor.ResponseExecutionResult{
		Results: []executor.ResponsePolicyResult{
			{
				Skipped: false,
				Action: policy.DownstreamResponseModifications{
					HeadersToSet: map[string]string{"x-response": "modified"},
				},
			},
		},
	}

	headerMutation, _, _, _, immResp, err := translateResponseActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, immResp, "no immediate response expected for normal flow")
	require.NotNil(t, headerMutation)
	assert.Len(t, headerMutation.SetHeaders, 1)
}

// =============================================================================
// Dynamic-endpoint request-header translation
// =============================================================================

// A dynamic-endpoint policy sets UpstreamName: the header phase must advertise the target
// upstream base path in dynamic metadata and surface the RAW request path as metadata["path"]
// (via the mutation struct). The Lua filter strips api_context and prepends the target base
// exactly once, so the value must be the raw path, never a pre-baked one. A path-changing
// policy that already set mutations.Path must win (not be clobbered).
func TestTranslateRequestHeaderActions_DynamicEndpoint(t *testing.T) {
	newExecCtx := func() *PolicyExecutionContext {
		kernel := NewKernel()
		chainExecutor := executor.NewChainExecutor(nil, nil, nil)
		server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")
		execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
		execCtx.sharedCtx = &policy.SharedContext{APIKind: "API", APIId: "api-123"}
		execCtx.requestBodyCtx = &policy.RequestContext{
			Path:          "/api/whoami",
			SharedContext: execCtx.sharedCtx,
		}
		execCtx.apiContext = "/api"
		execCtx.upstreamBasePath = "/sandbox"
		execCtx.upstreamDefinitionPaths = map[string]string{"alt-upstream": "/alternate"}
		return execCtx
	}

	targetUpstream := "alt-upstream"
	chain := &registry.PolicyChain{}

	t.Run("surfaces the raw request path and target upstream base path", func(t *testing.T) {
		execCtx := newExecCtx()
		result := &executor.RequestHeaderExecutionResult{
			Results: []executor.RequestHeaderPolicyResult{
				{Action: policy.UpstreamRequestHeaderModifications{UpstreamName: &targetUpstream}},
			},
		}

		resp, err := TranslateRequestHeaderActions(result, chain, execCtx)
		require.NoError(t, err)
		require.NotNil(t, resp)

		hm := resp.GetRequestHeaders().GetResponse().GetHeaderMutation()
		require.NotNil(t, hm)
		expectedCluster := constants.UpstreamDefinitionClusterPrefix + "API_api-123_" + sanitizeUpstreamDefinitionName(targetUpstream)
		var headerValue string
		for _, h := range hm.SetHeaders {
			if h.Header.Key == constants.TargetUpstreamHeader {
				headerValue = string(h.Header.RawValue)
			}
		}
		assert.Equal(t, expectedCluster, headerValue, "x-target-upstream must select the alt-upstream cluster")

		extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
		require.NotNil(t, extProc)
		assert.Equal(t, "/alternate", extProc.Fields["target_upstream_base_path"].GetStringValue())
		assert.Equal(t, "/api/whoami", extProc.Fields["path"].GetStringValue())
		assert.NotContains(t, extProc.Fields, "request_transformation.target_path")
	})

	t.Run("does not clobber a path set by an earlier policy", func(t *testing.T) {
		execCtx := newExecCtx()
		rewrittenPath := "/rewritten/path"
		result := &executor.RequestHeaderExecutionResult{
			Results: []executor.RequestHeaderPolicyResult{
				{Action: policy.UpstreamRequestHeaderModifications{
					UpstreamName: &targetUpstream,
					Path:         &rewrittenPath,
				}},
			},
		}

		resp, err := TranslateRequestHeaderActions(result, chain, execCtx)
		require.NoError(t, err)
		extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
		require.NotNil(t, extProc)
		assert.Equal(t, "/rewritten/path", extProc.Fields["path"].GetStringValue())
		assert.Equal(t, "/alternate", extProc.Fields["target_upstream_base_path"].GetStringValue())
		assert.NotContains(t, extProc.Fields, "request_transformation.target_path")
	})
}

// The body-merge variant (body-less requests) shares the same dynamic-endpoint contract:
// the policy runs in the header phase, surfaces the raw request path as metadata["path"],
// and must not clobber a path already set by an earlier policy.
func TestTranslateRequestHeaderActionsWithBodyMerge_DynamicEndpoint(t *testing.T) {
	newExecCtx := func() *PolicyExecutionContext {
		kernel := NewKernel()
		chainExecutor := executor.NewChainExecutor(nil, nil, nil)
		server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")
		execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
		execCtx.sharedCtx = &policy.SharedContext{APIKind: "API", APIId: "api-123"}
		execCtx.requestBodyCtx = &policy.RequestContext{
			Path:          "/api/whoami",
			SharedContext: execCtx.sharedCtx,
		}
		execCtx.apiContext = "/api"
		execCtx.upstreamBasePath = "/sandbox"
		execCtx.upstreamDefinitionPaths = map[string]string{"alt-upstream": "/alternate"}
		return execCtx
	}
	targetUpstream := "alt-upstream"
	emptyBody := func() *executor.RequestExecutionResult {
		return &executor.RequestExecutionResult{Results: []executor.RequestPolicyResult{}}
	}

	t.Run("surfaces the raw request path and target upstream base path", func(t *testing.T) {
		execCtx := newExecCtx()
		headerResult := &executor.RequestHeaderExecutionResult{
			Results: []executor.RequestHeaderPolicyResult{
				{Action: policy.UpstreamRequestHeaderModifications{UpstreamName: &targetUpstream}},
			},
		}
		resp, err := TranslateRequestHeaderActionsWithBodyMerge(headerResult, emptyBody(), execCtx)
		require.NoError(t, err)
		require.NotNil(t, resp)
		extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
		require.NotNil(t, extProc)
		assert.Equal(t, "/alternate", extProc.Fields["target_upstream_base_path"].GetStringValue())
		assert.Equal(t, "/api/whoami", extProc.Fields["path"].GetStringValue())
		assert.NotContains(t, extProc.Fields, "request_transformation.target_path")
	})

	t.Run("does not clobber a path set by an earlier policy", func(t *testing.T) {
		execCtx := newExecCtx()
		rewrittenPath := "/rewritten/path"
		headerResult := &executor.RequestHeaderExecutionResult{
			Results: []executor.RequestHeaderPolicyResult{
				{Action: policy.UpstreamRequestHeaderModifications{
					UpstreamName: &targetUpstream,
					Path:         &rewrittenPath,
				}},
			},
		}
		resp, err := TranslateRequestHeaderActionsWithBodyMerge(headerResult, emptyBody(), execCtx)
		require.NoError(t, err)
		extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
		require.NotNil(t, extProc)
		assert.Equal(t, "/rewritten/path", extProc.Fields["path"].GetStringValue())
		assert.Equal(t, "/alternate", extProc.Fields["target_upstream_base_path"].GetStringValue())
		assert.NotContains(t, extProc.Fields, "request_transformation.target_path")
	})
}

// TestTranslateRequestHeaderActions_DynamicEndpointSanitizesClusterName pins the exact
// x-target-upstream name for a definition name containing dots or colons, locking it byte-for-byte to
// the controller's clusterkey.DefinitionName so the two modules' cluster names cannot drift.
func TestTranslateRequestHeaderActions_DynamicEndpointSanitizesClusterName(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")
	execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
	execCtx.sharedCtx = &policy.SharedContext{APIKind: "RestApi", APIId: "api-123"}
	execCtx.requestBodyCtx = &policy.RequestContext{
		Path:          "/api/whoami",
		SharedContext: execCtx.sharedCtx,
	}
	execCtx.apiContext = "/api"
	execCtx.upstreamBasePath = "/sandbox"
	execCtx.upstreamDefinitionPaths = map[string]string{"host.example.com:8080": "/alternate"}

	targetUpstream := "host.example.com:8080"
	result := &executor.RequestHeaderExecutionResult{
		Results: []executor.RequestHeaderPolicyResult{
			{Action: policy.UpstreamRequestHeaderModifications{UpstreamName: &targetUpstream}},
		},
	}

	resp, err := TranslateRequestHeaderActions(result, &registry.PolicyChain{}, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)

	hm := resp.GetRequestHeaders().GetResponse().GetHeaderMutation()
	require.NotNil(t, hm)
	var headerValue string
	for _, h := range hm.SetHeaders {
		if h.Header.Key == constants.TargetUpstreamHeader {
			headerValue = string(h.Header.RawValue)
		}
	}
	assert.Equal(t, "upstream_RestApi_api-123_host_example_com_8080", headerValue,
		"dots and colons in the definition name must be replaced with underscores to match the controller's cluster name")
}
