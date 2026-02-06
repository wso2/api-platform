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

	"github.com/wso2/api-platform/gateway/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
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
		"content-type":   {"application/json"},
		"authorization":  {"Bearer token"},
		"x-custom":       {"value"},
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
		"content-type":   {"application/json"},
		"authorization":  {"Bearer token"},
		"x-custom":       {"value"},
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
		"content-type":   {"application/json"},
		"authorization":  {"Bearer token"},
		"x-custom":       {"value"},
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

	result := buildDynamicMetadata(analyticsStruct, &path, nil)

	require.NotNil(t, result)
	// Should include path in metadata
	extProc := result.Fields["api_platform.policy_engine.envoy.filters.http.ext_proc"].GetStructValue()
	require.NotNil(t, extProc)
	assert.Contains(t, extProc.Fields, "path")
	assert.Equal(t, "/new/path", extProc.Fields["path"].GetStringValue())
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
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{},
	}

	headerMutation, bodyMutation, analyticsData, _, pathMutation, immResp, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, immResp)
	assert.NotNil(t, headerMutation)
	assert.Nil(t, bodyMutation)
	assert.NotNil(t, analyticsData)
	assert.Nil(t, pathMutation)
}

func TestTranslateRequestActionsCore_WithSetHeaders(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					SetHeaders: map[string]string{
						"x-custom": "value",
					},
				},
			},
		},
	}

	headerMutation, _, _, _, _, immResp, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, immResp)
	require.NotNil(t, headerMutation)
	assert.Len(t, headerMutation.SetHeaders, 1)
}

func TestTranslateRequestActionsCore_WithBodyModification(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
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

	headerMutation, bodyMutation, _, _, _, immResp, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	assert.Nil(t, immResp)
	require.NotNil(t, bodyMutation)
	assert.Equal(t, []byte("modified body"), bodyMutation.GetBody())
	// Content-Length should be set
	require.NotNil(t, headerMutation)
	var foundContentLength bool
	for _, h := range headerMutation.SetHeaders {
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
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
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

	_, _, _, _, _, immResp, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, immResp)

	immediate := immResp.GetImmediateResponse()
	require.NotNil(t, immediate)
	assert.Equal(t, uint32(403), uint32(immediate.Status.Code))
}

func TestTranslateRequestActionsCore_SkippedPolicy(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: true, // This policy was skipped
				Action: policy.UpstreamRequestModifications{
					SetHeaders: map[string]string{
						"should-not-appear": "value",
					},
				},
			},
		},
	}

	headerMutation, _, _, _, _, _, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	// Skipped policy actions should not be applied
	assert.Empty(t, headerMutation.SetHeaders)
}

func TestTranslateRequestActionsCore_WithQueryParams(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
	}

	result := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{
			{
				Skipped: false,
				Action: policy.UpstreamRequestModifications{
					AddQueryParameters: map[string][]string{
						"added": {"param"},
					},
				},
			},
		},
	}

	_, _, _, _, pathMutation, _, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, pathMutation)
	assert.Contains(t, pathMutation, "added=param")
}

func TestTranslateRequestActionsCore_WithPathOverride(t *testing.T) {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")

	chain := &registry.PolicyChain{}
	execCtx := newPolicyExecutionContext(server, "test-route", chain)
	execCtx.requestContext = &policy.RequestContext{
		Path: "/api/test",
		SharedContext: &policy.SharedContext{},
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

	_, _, _, _, pathMutation, _, err := translateRequestActionsCore(result, execCtx)

	assert.NoError(t, err)
	require.NotNil(t, pathMutation)
	assert.Equal(t, "/new/path", pathMutation)
}
