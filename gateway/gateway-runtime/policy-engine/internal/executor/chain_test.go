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

package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"go.opentelemetry.io/otel/trace/noop"
)

// =============================================================================
// Mock CEL Evaluators for Tests
// =============================================================================

// mockCELEvaluator is a configurable mock for testing
type mockCELEvaluator struct {
	requestResult  bool
	responseResult bool
	requestErr     error
	responseErr    error
}

func (m *mockCELEvaluator) EvaluateRequestCondition(expression string, ctx *policy.RequestContext) (bool, error) {
	if m.requestErr != nil {
		return false, m.requestErr
	}
	return m.requestResult, nil
}

func (m *mockCELEvaluator) EvaluateResponseCondition(expression string, ctx *policy.ResponseContext) (bool, error) {
	if m.responseErr != nil {
		return false, m.responseErr
	}
	return m.responseResult, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func newPolicySpec(name, version string, enabled bool, condition *string) policy.PolicySpec {
	return policy.PolicySpec{
		Name:               name,
		Version:            version,
		Enabled:            enabled,
		Parameters:         policy.PolicyParameters{Raw: map[string]interface{}{}},
		ExecutionCondition: condition,
	}
}

// =============================================================================
// Tests for NewChainExecutor
// =============================================================================

func TestNewChainExecutor(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{requestResult: true, responseResult: true}

	executor := NewChainExecutor(nil, celEval, tracer)

	assert.NotNil(t, executor)
	assert.Nil(t, executor.registry)
	assert.NotNil(t, executor.celEvaluator)
	assert.NotNil(t, executor.tracer)
}

// =============================================================================
// Tests for ExecuteRequestPolicies
// =============================================================================

func TestExecuteRequestPolicies_EmptyPolicyList(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()

	result, err := executor.ExecuteRequestPolicies(ctx, []policy.Policy{}, reqCtx, []policy.PolicySpec{}, "api", "route", false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
	assert.False(t, result.ShortCircuited)
}

func TestExecuteRequestPolicies_SinglePolicy_NoAction(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, nil)}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "noop", result.Results[0].PolicyName)
	assert.False(t, result.Results[0].Skipped)
	assert.False(t, result.ShortCircuited)
}

func TestExecuteRequestPolicies_DisabledPolicy_Skipped(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", false, nil)}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteRequestPolicies_ConditionFalse_Skipped(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{requestResult: false}
	executor := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("request.method == 'POST'"))}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", true)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteRequestPolicies_ConditionTrue_Executed(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{requestResult: true}
	executor := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("request.method == 'GET'"))}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", true)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Skipped)
}

func TestExecuteRequestPolicies_ConditionEvaluationError(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{requestErr: errors.New("CEL evaluation failed")}
	executor := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("invalid.expression"))}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", true)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "condition evaluation failed")
}

func TestExecuteRequestPolicies_ShortCircuit(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{
		&testutils.ShortCircuitingPolicy{StatusCode: 401, Body: []byte("unauthorized")},
		&testutils.NoopPolicy{}, // This should NOT be executed
	}
	specs := []policy.PolicySpec{
		newPolicySpec("auth", "v1.0.0", true, nil),
		newPolicySpec("noop", "v1.0.0", true, nil),
	}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.True(t, result.ShortCircuited)
	assert.Len(t, result.Results, 1) // Only first policy executed
	assert.NotNil(t, result.FinalAction)
}

func TestExecuteRequestPolicies_HeaderModification(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{
		&testutils.HeaderModifyingPolicy{Key: "x-custom-header", Value: "test-value"},
	}
	specs := []policy.PolicySpec{newPolicySpec("header-mod", "v1.0.0", true, nil)}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.False(t, result.ShortCircuited)

	// Verify header was modified
	vals := reqCtx.Headers.Get("x-custom-header")
	assert.NotNil(t, vals)
	assert.Equal(t, "test-value", vals[0])
}

func TestExecuteRequestPolicies_MultiplePolicies(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestContext()
	policies := []policy.Policy{
		&testutils.HeaderModifyingPolicy{Key: "x-header-1", Value: "value-1"},
		&testutils.HeaderModifyingPolicy{Key: "x-header-2", Value: "value-2"},
		&testutils.NoopPolicy{},
	}
	specs := []policy.PolicySpec{
		newPolicySpec("header-1", "v1.0.0", true, nil),
		newPolicySpec("header-2", "v1.0.0", true, nil),
		newPolicySpec("noop", "v1.0.0", true, nil),
	}

	result, err := executor.ExecuteRequestPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 3)
	assert.False(t, result.ShortCircuited)

	// Verify both headers were added
	vals1 := reqCtx.Headers.Get("x-header-1")
	vals2 := reqCtx.Headers.Get("x-header-2")
	assert.NotNil(t, vals1)
	assert.NotNil(t, vals2)
	assert.Equal(t, "value-1", vals1[0])
	assert.Equal(t, "value-2", vals2[0])
}

// =============================================================================
// Tests for ExecuteResponsePolicies
// =============================================================================

func TestExecuteResponsePolicies_EmptyPolicyList(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()

	result, err := executor.ExecuteResponsePolicies(ctx, []policy.Policy{}, respCtx, []policy.PolicySpec{}, "api", "route", false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
}

func TestExecuteResponsePolicies_SinglePolicy(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, nil)}

	result, err := executor.ExecuteResponsePolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "noop", result.Results[0].PolicyName)
	assert.False(t, result.Results[0].Skipped)
}

func TestExecuteResponsePolicies_DisabledPolicy(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", false, nil)}

	result, err := executor.ExecuteResponsePolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteResponsePolicies_ConditionFalse(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{responseResult: false}
	executor := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("response.status == 404"))}

	result, err := executor.ExecuteResponsePolicies(ctx, policies, respCtx, specs, "api", "route", true)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteResponsePolicies_ConditionError(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{responseErr: errors.New("CEL error")}
	executor := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("invalid"))}

	result, err := executor.ExecuteResponsePolicies(ctx, policies, respCtx, specs, "api", "route", true)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "condition evaluation failed")
}

func TestExecuteResponsePolicies_HeaderModification(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()
	policies := []policy.Policy{
		&testutils.HeaderModifyingPolicy{Key: "x-response-header", Value: "response-value"},
	}
	specs := []policy.PolicySpec{newPolicySpec("header-mod", "v1.0.0", true, nil)}

	result, err := executor.ExecuteResponsePolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)

	// Verify header was modified
	vals := respCtx.ResponseHeaders.Get("x-response-header")
	assert.NotNil(t, vals)
	assert.Equal(t, "response-value", vals[0])
}

func TestExecuteResponsePolicies_ReverseOrder(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseContext()

	// Track execution order
	var executionOrder []string
	trackingPolicy := func(name string) policy.Policy {
		return &trackingPolicyImpl{name: name, executionOrder: &executionOrder}
	}

	policies := []policy.Policy{
		trackingPolicy("first"),
		trackingPolicy("second"),
		trackingPolicy("third"),
	}
	specs := []policy.PolicySpec{
		newPolicySpec("first", "v1.0.0", true, nil),
		newPolicySpec("second", "v1.0.0", true, nil),
		newPolicySpec("third", "v1.0.0", true, nil),
	}

	result, err := executor.ExecuteResponsePolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 3)

	// Response policies execute in reverse order
	assert.Equal(t, []string{"third", "second", "first"}, executionOrder)
}

// trackingPolicyImpl tracks execution order
type trackingPolicyImpl struct {
	name           string
	executionOrder *[]string
}

func (p *trackingPolicyImpl) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{}
}

func (p *trackingPolicyImpl) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	*p.executionOrder = append(*p.executionOrder, p.name)
	return nil
}

func (p *trackingPolicyImpl) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	*p.executionOrder = append(*p.executionOrder, p.name)
	return nil
}

// =============================================================================
// Tests for applyRequestModifications
// =============================================================================

func TestApplyRequestModifications_SetHeaders(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	mods := policy.UpstreamRequestModifications{
		SetHeaders: map[string]string{
			"x-new-header": "new-value",
			"content-type": "text/plain", // Override existing
		},
	}

	applyRequestModifications(ctx, &mods)

	vals := ctx.Headers.Get("x-new-header")
	assert.NotNil(t, vals)
	assert.Equal(t, "new-value", vals[0])

	ct := ctx.Headers.Get("content-type")
	assert.NotNil(t, ct)
	assert.Equal(t, "text/plain", ct[0])
}

func TestApplyRequestModifications_RemoveHeaders(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	mods := policy.UpstreamRequestModifications{
		RemoveHeaders: []string{"content-type"},
	}

	applyRequestModifications(ctx, &mods)

	vals := ctx.Headers.Get("content-type")
	assert.Nil(t, vals)
}

func TestApplyRequestModifications_AppendHeaders(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	mods := policy.UpstreamRequestModifications{
		AppendHeaders: map[string][]string{
			"x-multi": {"value1", "value2"},
		},
	}

	applyRequestModifications(ctx, &mods)

	vals := ctx.Headers.Get("x-multi")
	assert.Equal(t, []string{"value1", "value2"}, vals)
}

func TestApplyRequestModifications_UpdateBody(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	newBody := []byte(`{"modified": true}`)
	mods := policy.UpstreamRequestModifications{
		Body: newBody,
	}

	applyRequestModifications(ctx, &mods)

	assert.NotNil(t, ctx.Body)
	assert.Equal(t, newBody, ctx.Body.Content)
	assert.True(t, ctx.Body.EndOfStream)
	assert.True(t, ctx.Body.Present)
}

func TestApplyRequestModifications_UpdatePath(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	newPath := "/new/path"
	mods := policy.UpstreamRequestModifications{
		Path: &newPath,
	}

	applyRequestModifications(ctx, &mods)

	assert.Equal(t, "/new/path", ctx.Path)
}

func TestApplyRequestModifications_UpdateMethod(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	newMethod := "POST"
	mods := policy.UpstreamRequestModifications{
		Method: &newMethod,
	}

	applyRequestModifications(ctx, &mods)

	assert.Equal(t, "POST", ctx.Method)
}

func TestApplyRequestModifications_AddQueryParameters(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	ctx.Path = "/test/path"
	mods := policy.UpstreamRequestModifications{
		AddQueryParameters: map[string][]string{
			"foo": {"bar"},
			"baz": {"qux"},
		},
	}

	applyRequestModifications(ctx, &mods)

	assert.Contains(t, ctx.Path, "foo=bar")
	assert.Contains(t, ctx.Path, "baz=qux")
}

func TestApplyRequestModifications_RemoveQueryParameters(t *testing.T) {
	ctx := testutils.NewTestRequestContext()
	ctx.Path = "/test/path?foo=bar&baz=qux&keep=me"
	mods := policy.UpstreamRequestModifications{
		RemoveQueryParameters: []string{"foo", "baz"},
	}

	applyRequestModifications(ctx, &mods)

	assert.NotContains(t, ctx.Path, "foo=bar")
	assert.NotContains(t, ctx.Path, "baz=qux")
	assert.Contains(t, ctx.Path, "keep=me")
}

// =============================================================================
// Tests for applyResponseModifications
// =============================================================================

func TestApplyResponseModifications_SetHeaders(t *testing.T) {
	ctx := testutils.NewTestResponseContext()
	mods := policy.UpstreamResponseModifications{
		SetHeaders: map[string]string{
			"x-response-header": "response-value",
		},
	}

	applyResponseModifications(ctx, &mods)

	vals := ctx.ResponseHeaders.Get("x-response-header")
	assert.NotNil(t, vals)
	assert.Equal(t, "response-value", vals[0])
}

func TestApplyResponseModifications_RemoveHeaders(t *testing.T) {
	ctx := testutils.NewTestResponseContext()
	mods := policy.UpstreamResponseModifications{
		RemoveHeaders: []string{"content-type"},
	}

	applyResponseModifications(ctx, &mods)

	vals := ctx.ResponseHeaders.Get("content-type")
	assert.Nil(t, vals)
}

func TestApplyResponseModifications_AppendHeaders(t *testing.T) {
	ctx := testutils.NewTestResponseContext()
	mods := policy.UpstreamResponseModifications{
		AppendHeaders: map[string][]string{
			"x-multi-resp": {"val1", "val2"},
		},
	}

	applyResponseModifications(ctx, &mods)

	vals := ctx.ResponseHeaders.Get("x-multi-resp")
	assert.Equal(t, []string{"val1", "val2"}, vals)
}

func TestApplyResponseModifications_UpdateBody(t *testing.T) {
	ctx := testutils.NewTestResponseContext()
	newBody := []byte(`{"response": "modified"}`)
	mods := policy.UpstreamResponseModifications{
		Body: newBody,
	}

	applyResponseModifications(ctx, &mods)

	assert.NotNil(t, ctx.ResponseBody)
	assert.Equal(t, newBody, ctx.ResponseBody.Content)
	assert.True(t, ctx.ResponseBody.EndOfStream)
	assert.True(t, ctx.ResponseBody.Present)
}

func TestApplyResponseModifications_UpdateStatusCode(t *testing.T) {
	ctx := testutils.NewTestResponseContext()
	newStatus := 404
	mods := policy.UpstreamResponseModifications{
		StatusCode: &newStatus,
	}

	applyResponseModifications(ctx, &mods)

	assert.Equal(t, 404, ctx.ResponseStatus)
}

// =============================================================================
// Tests for Result Structs
// =============================================================================

func TestRequestPolicyResult(t *testing.T) {
	result := RequestPolicyResult{
		PolicyName:    "jwt-auth",
		PolicyVersion: "v1.0.0",
		Action:        nil,
		Skipped:       false,
	}

	assert.Equal(t, "jwt-auth", result.PolicyName)
	assert.Equal(t, "v1.0.0", result.PolicyVersion)
	assert.False(t, result.Skipped)
}

func TestRequestExecutionResult(t *testing.T) {
	result := RequestExecutionResult{
		Results:        []RequestPolicyResult{},
		ShortCircuited: false,
		FinalAction:    nil,
	}

	assert.Empty(t, result.Results)
	assert.False(t, result.ShortCircuited)
	assert.Nil(t, result.FinalAction)
}

func TestResponsePolicyResult(t *testing.T) {
	result := ResponsePolicyResult{
		PolicyName:    "cors",
		PolicyVersion: "v1.0.0",
		Action:        nil,
		Error:         nil,
		Skipped:       true,
	}

	assert.Equal(t, "cors", result.PolicyName)
	assert.True(t, result.Skipped)
	assert.Nil(t, result.Error)
}

func TestResponseExecutionResult(t *testing.T) {
	result := ResponseExecutionResult{
		Results:     []ResponsePolicyResult{},
		FinalAction: nil,
	}

	assert.Empty(t, result.Results)
	assert.Nil(t, result.FinalAction)
}
