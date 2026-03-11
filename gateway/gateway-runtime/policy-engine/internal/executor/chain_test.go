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
// Mock CEL Evaluator for Tests
// =============================================================================

type mockCELEvaluator struct {
	reqHeaderResult  bool
	reqBodyResult    bool
	respHeaderResult bool
	respBodyResult   bool
	reqHeaderErr     error
	reqBodyErr       error
	respHeaderErr    error
	respBodyErr      error
}

func (m *mockCELEvaluator) EvaluateRequestHeaderCondition(_ string, _ *policy.RequestHeaderContext) (bool, error) {
	return m.reqHeaderResult, m.reqHeaderErr
}

func (m *mockCELEvaluator) EvaluateRequestBodyCondition(_ string, _ *policy.RequestContext) (bool, error) {
	return m.reqBodyResult, m.reqBodyErr
}

func (m *mockCELEvaluator) EvaluateResponseHeaderCondition(_ string, _ *policy.ResponseHeaderContext) (bool, error) {
	return m.respHeaderResult, m.respHeaderErr
}

func (m *mockCELEvaluator) EvaluateResponseBodyCondition(_ string, _ *policy.ResponseContext) (bool, error) {
	return m.respBodyResult, m.respBodyErr
}

func (m *mockCELEvaluator) EvaluateStreamingRequestCondition(_ string, _ *policy.RequestStreamContext) (bool, error) {
	return m.reqBodyResult, m.reqBodyErr
}

func (m *mockCELEvaluator) EvaluateStreamingResponseCondition(_ string, _ *policy.ResponseStreamContext) (bool, error) {
	return m.respBodyResult, m.respBodyErr
}

// =============================================================================
// Local test policy helpers
// =============================================================================

// noopRequestHeaderPolicy implements RequestHeaderPolicy with no side effects.
type noopRequestHeaderPolicy struct{}

func (p *noopRequestHeaderPolicy) OnRequestHeaders(_ *policy.RequestHeaderContext) policy.RequestHeaderAction {
	return policy.RequestHeaderAction{}
}

// noopResponseHeaderPolicy implements ResponseHeaderPolicy with no side effects.
type noopResponseHeaderPolicy struct{}

func (p *noopResponseHeaderPolicy) OnResponseHeaders(_ *policy.ResponseHeaderContext) policy.ResponseHeaderAction {
	return policy.ResponseHeaderAction{}
}

// trackingPolicyImpl implements ResponseHeaderPolicy and records execution order.
type trackingPolicyImpl struct {
	name           string
	executionOrder *[]string
}

func (p *trackingPolicyImpl) OnResponseHeaders(_ *policy.ResponseHeaderContext) policy.ResponseHeaderAction {
	*p.executionOrder = append(*p.executionOrder, p.name)
	return policy.ResponseHeaderAction{}
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
	celEval := &mockCELEvaluator{reqHeaderResult: true}

	exec := NewChainExecutor(nil, celEval, tracer)

	assert.NotNil(t, exec)
	assert.Nil(t, exec.registry)
	assert.NotNil(t, exec.celEvaluator)
	assert.NotNil(t, exec.tracer)
}

// =============================================================================
// Tests for ExecuteRequestHeaderPolicies
// =============================================================================

func TestExecuteRequestHeaderPolicies_EmptyPolicyList(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, []policy.Policy{}, reqCtx, []policy.PolicySpec{}, "api", "route", false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
	assert.False(t, result.ShortCircuited)
}

func TestExecuteRequestHeaderPolicies_SinglePolicy_NoAction(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	policies := []policy.Policy{&noopRequestHeaderPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, nil)}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "noop", result.Results[0].PolicyName)
	assert.False(t, result.Results[0].Skipped)
	assert.False(t, result.ShortCircuited)
}

func TestExecuteRequestHeaderPolicies_DisabledPolicy_Skipped(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	// NoopPolicy satisfies Policy but not RequestHeaderPolicy; disabled check fires first.
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", false, nil)}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteRequestHeaderPolicies_ConditionFalse_Skipped(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{reqHeaderResult: false}
	exec := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	// Condition check fires before RequestHeaderPolicy type assertion.
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("request.method == 'POST'"))}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", true)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteRequestHeaderPolicies_ConditionTrue_Executed(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{reqHeaderResult: true}
	exec := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	policies := []policy.Policy{&noopRequestHeaderPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("request.method == 'GET'"))}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", true)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Skipped)
}

func TestExecuteRequestHeaderPolicies_ConditionEvaluationError(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{reqHeaderErr: errors.New("CEL evaluation failed")}
	exec := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	// Error is returned before the RequestHeaderPolicy type assertion.
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("invalid.expression"))}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", true)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "condition evaluation failed")
}

func TestExecuteRequestHeaderPolicies_ShortCircuit(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	policies := []policy.Policy{
		&testutils.ShortCircuitingPolicy{StatusCode: 401, Body: []byte("unauthorized")},
		&noopRequestHeaderPolicy{}, // This should NOT be executed
	}
	specs := []policy.PolicySpec{
		newPolicySpec("auth", "v1.0.0", true, nil),
		newPolicySpec("noop", "v1.0.0", true, nil),
	}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.True(t, result.ShortCircuited)
	assert.Len(t, result.Results, 1) // Only the first policy executed
	assert.NotNil(t, result.FinalAction)
	assert.NotNil(t, result.FinalAction.ImmediateResponse)
}

func TestExecuteRequestHeaderPolicies_HeaderModification(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	policies := []policy.Policy{
		&testutils.HeaderModifyingPolicy{Key: "x-custom-header", Value: "test-value"},
	}
	specs := []policy.PolicySpec{newPolicySpec("header-mod", "v1.0.0", true, nil)}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.False(t, result.ShortCircuited)
	require.NotNil(t, result.FinalAction)
	assert.Equal(t, "test-value", result.FinalAction.Set["x-custom-header"])
}

func TestExecuteRequestHeaderPolicies_MultiplePolicies(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	reqCtx := testutils.NewTestRequestHeaderContext()
	policies := []policy.Policy{
		&testutils.HeaderModifyingPolicy{Key: "x-header-1", Value: "value-1"},
		&testutils.HeaderModifyingPolicy{Key: "x-header-2", Value: "value-2"},
		&noopRequestHeaderPolicy{},
	}
	specs := []policy.PolicySpec{
		newPolicySpec("header-1", "v1.0.0", true, nil),
		newPolicySpec("header-2", "v1.0.0", true, nil),
		newPolicySpec("noop", "v1.0.0", true, nil),
	}

	result, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, reqCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 3)
	assert.False(t, result.ShortCircuited)

	// Verify actions were produced for each policy
	assert.Equal(t, "value-1", result.Results[0].Action.Set["x-header-1"])
	assert.Equal(t, "value-2", result.Results[1].Action.Set["x-header-2"])
}

// =============================================================================
// Tests for ExecuteResponseHeaderPolicies
// =============================================================================

func TestExecuteResponseHeaderPolicies_EmptyPolicyList(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, []policy.Policy{}, respCtx, []policy.PolicySpec{}, "api", "route", false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
}

func TestExecuteResponseHeaderPolicies_SinglePolicy(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()
	policies := []policy.Policy{&noopResponseHeaderPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, nil)}

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "noop", result.Results[0].PolicyName)
	assert.False(t, result.Results[0].Skipped)
}

func TestExecuteResponseHeaderPolicies_DisabledPolicy(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()
	// Disabled check fires before ResponseHeaderPolicy type assertion.
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", false, nil)}

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteResponseHeaderPolicies_ConditionFalse(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{respHeaderResult: false}
	exec := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()
	// Condition check fires before ResponseHeaderPolicy type assertion.
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("response.status == 404"))}

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, policies, respCtx, specs, "api", "route", true)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
}

func TestExecuteResponseHeaderPolicies_ConditionError(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	celEval := &mockCELEvaluator{respHeaderErr: errors.New("CEL error")}
	exec := NewChainExecutor(nil, celEval, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()
	policies := []policy.Policy{&testutils.NoopPolicy{}}
	specs := []policy.PolicySpec{newPolicySpec("noop", "v1.0.0", true, testutils.PtrString("invalid"))}

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, policies, respCtx, specs, "api", "route", true)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "condition evaluation failed")
}

func TestExecuteResponseHeaderPolicies_HeaderModification(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()
	policies := []policy.Policy{
		&testutils.HeaderModifyingPolicy{Key: "x-response-header", Value: "response-value"},
	}
	specs := []policy.PolicySpec{newPolicySpec("header-mod", "v1.0.0", true, nil)}

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	require.NotNil(t, result.Results[0].Action)
	assert.Equal(t, "response-value", result.Results[0].Action.Set["x-response-header"])
}

func TestExecuteResponseHeaderPolicies_ReverseOrder(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec := NewChainExecutor(nil, nil, tracer)

	ctx := context.Background()
	respCtx := testutils.NewTestResponseHeaderContext()

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

	result, err := exec.ExecuteResponseHeaderPolicies(ctx, policies, respCtx, specs, "api", "route", false)

	require.NoError(t, err)
	assert.Len(t, result.Results, 3)

	// Response policies execute in reverse order
	assert.Equal(t, []string{"third", "second", "first"}, executionOrder)
}

// =============================================================================
// Tests for Result Structs
// =============================================================================

func TestRequestHeaderPolicyResult(t *testing.T) {
	result := RequestHeaderPolicyResult{
		PolicyName:    "jwt-auth",
		PolicyVersion: "v1.0.0",
		Action:        nil,
		Skipped:       false,
	}

	assert.Equal(t, "jwt-auth", result.PolicyName)
	assert.Equal(t, "v1.0.0", result.PolicyVersion)
	assert.False(t, result.Skipped)
}

func TestRequestHeaderExecutionResult(t *testing.T) {
	result := RequestHeaderExecutionResult{
		Results:        []RequestHeaderPolicyResult{},
		ShortCircuited: false,
		FinalAction:    nil,
	}

	assert.Empty(t, result.Results)
	assert.False(t, result.ShortCircuited)
	assert.Nil(t, result.FinalAction)
}

func TestResponseHeaderPolicyResult(t *testing.T) {
	result := ResponseHeaderPolicyResult{
		PolicyName:    "cors",
		PolicyVersion: "v1.0.0",
		Action:        nil,
		Skipped:       true,
	}

	assert.Equal(t, "cors", result.PolicyName)
	assert.True(t, result.Skipped)
}

func TestResponseHeaderExecutionResult(t *testing.T) {
	result := ResponseHeaderExecutionResult{
		Results: []ResponseHeaderPolicyResult{},
	}

	assert.Empty(t, result.Results)
}

// =============================================================================
// Streaming policy helpers (local to this file)
// =============================================================================

// noopStreamingRequestPolicy implements StreamingRequestPolicy with no mutations.
type noopStreamingRequestPolicy struct{}

func (p *noopStreamingRequestPolicy) OnRequestBody(_ *policy.RequestContext) policy.RequestAction {
	return policy.RequestAction{}
}
func (p *noopStreamingRequestPolicy) OnRequestBodyChunk(_ *policy.RequestStreamContext, _ *policy.StreamBody) policy.RequestChunkAction {
	return policy.RequestChunkAction{}
}
func (p *noopStreamingRequestPolicy) NeedsMoreRequestData(_ []byte) bool { return false }

// mutatingStreamingRequestPolicy replaces every chunk with a fixed payload.
type mutatingStreamingRequestPolicy struct{ out []byte }

func (p *mutatingStreamingRequestPolicy) OnRequestBody(_ *policy.RequestContext) policy.RequestAction {
	return policy.RequestAction{}
}
func (p *mutatingStreamingRequestPolicy) OnRequestBodyChunk(_ *policy.RequestStreamContext, _ *policy.StreamBody) policy.RequestChunkAction {
	return policy.RequestChunkAction{BodyMutation: p.out}
}
func (p *mutatingStreamingRequestPolicy) NeedsMoreRequestData(_ []byte) bool { return false }

// noopStreamingResponsePolicy implements StreamingResponsePolicy with no mutations.
type noopStreamingResponsePolicy struct{}

func (p *noopStreamingResponsePolicy) OnResponseBody(_ *policy.ResponseContext) policy.ResponseAction {
	return policy.ResponseAction{}
}
func (p *noopStreamingResponsePolicy) OnResponseBodyChunk(_ *policy.ResponseStreamContext, _ *policy.StreamBody) policy.ResponseChunkAction {
	return policy.ResponseChunkAction{}
}
func (p *noopStreamingResponsePolicy) NeedsMoreResponseData(_ []byte) bool { return false }

// mutatingStreamingResponsePolicy replaces every chunk with a fixed payload.
type mutatingStreamingResponsePolicy struct{ out []byte }

func (p *mutatingStreamingResponsePolicy) OnResponseBody(_ *policy.ResponseContext) policy.ResponseAction {
	return policy.ResponseAction{}
}
func (p *mutatingStreamingResponsePolicy) OnResponseBodyChunk(_ *policy.ResponseStreamContext, _ *policy.StreamBody) policy.ResponseChunkAction {
	return policy.ResponseChunkAction{BodyMutation: p.out}
}
func (p *mutatingStreamingResponsePolicy) NeedsMoreResponseData(_ []byte) bool { return false }

// trackingStreamingResponsePolicy records execution order.
type trackingStreamingResponsePolicy struct {
	name           string
	executionOrder *[]string
}

func (p *trackingStreamingResponsePolicy) OnResponseBody(_ *policy.ResponseContext) policy.ResponseAction {
	return policy.ResponseAction{}
}
func (p *trackingStreamingResponsePolicy) OnResponseBodyChunk(_ *policy.ResponseStreamContext, _ *policy.StreamBody) policy.ResponseChunkAction {
	*p.executionOrder = append(*p.executionOrder, p.name)
	return policy.ResponseChunkAction{}
}
func (p *trackingStreamingResponsePolicy) NeedsMoreResponseData(_ []byte) bool { return false }

// =============================================================================
// ExecuteStreamingRequestPolicies Tests
// =============================================================================

func TestExecuteStreamingRequestPolicies_EmptyPolicyList(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("hello")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), nil, ctx, chunk, nil, "", "", false)

	require.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingRequestPolicies_NonStreamingPolicy_Skipped(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	// noopRequestHeaderPolicy does NOT implement StreamingRequestPolicy
	pol := &noopRequestHeaderPolicy{}
	spec := newPolicySpec("auth", "v1", true, nil)
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	assert.Empty(t, result.Results) // not appended — silently skipped
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingRequestPolicies_DisabledPolicy_Skipped(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingRequestPolicy{}
	spec := newPolicySpec("body", "v1", false, nil) // disabled
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingRequestPolicies_ConditionFalse_Skipped(t *testing.T) {
	cel := &mockCELEvaluator{reqBodyResult: false}
	exec := NewChainExecutor(nil, cel, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingRequestPolicy{}
	cond := "false"
	spec := newPolicySpec("body", "v1", true, &cond)
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", true)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingRequestPolicies_ConditionError(t *testing.T) {
	cel := &mockCELEvaluator{reqBodyErr: errors.New("cel error")}
	exec := NewChainExecutor(nil, cel, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingRequestPolicy{}
	cond := "bad"
	spec := newPolicySpec("body", "v1", true, &cond)
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	_, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cel error")
}

func TestExecuteStreamingRequestPolicies_Passthrough_FinalActionSet(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingRequestPolicy{} // no mutation
	spec := newPolicySpec("body", "v1", true, nil)
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("original")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.NotNil(t, result.FinalAction)
	assert.Nil(t, result.FinalAction.BodyMutation) // passthrough — translator falls back to original
}

func TestExecuteStreamingRequestPolicies_Mutation_FinalActionHasBytes(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &mutatingStreamingRequestPolicy{out: []byte("replaced")}
	spec := newPolicySpec("body", "v1", true, nil)
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("original")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	require.NotNil(t, result.FinalAction)
	assert.Equal(t, []byte("replaced"), result.FinalAction.BodyMutation)
}

func TestExecuteStreamingRequestPolicies_ChainedMutations(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol1 := &mutatingStreamingRequestPolicy{out: []byte("after-pol1")}
	pol2 := &noopStreamingRequestPolicy{} // no further mutation
	specs := []policy.PolicySpec{
		newPolicySpec("pol1", "v1", true, nil),
		newPolicySpec("pol2", "v1", true, nil),
	}
	ctx := &policy.RequestStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("original")}

	result, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol1, pol2}, ctx, chunk, specs, "", "", false)

	require.NoError(t, err)
	require.Len(t, result.Results, 2)
	assert.Equal(t, []byte("after-pol1"), result.Results[0].Action.BodyMutation)
	// FinalAction is pol2's (last executed), which has no mutation
	assert.Nil(t, result.FinalAction.BodyMutation)
}

// =============================================================================
// ExecuteStreamingResponsePolicies Tests
// =============================================================================

func TestExecuteStreamingResponsePolicies_EmptyPolicyList(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("hello")}

	result, err := exec.ExecuteStreamingResponsePolicies(context.Background(), nil, ctx, chunk, nil, "", "", false)

	require.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingResponsePolicies_NonStreamingPolicy_Skipped(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &noopResponseHeaderPolicy{} // does NOT implement StreamingResponsePolicy
	spec := newPolicySpec("logger", "v1", true, nil)
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	result, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingResponsePolicies_DisabledPolicy_Skipped(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingResponsePolicy{}
	spec := newPolicySpec("body", "v1", false, nil)
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	result, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingResponsePolicies_ConditionFalse_Skipped(t *testing.T) {
	cel := &mockCELEvaluator{respBodyResult: false}
	exec := NewChainExecutor(nil, cel, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingResponsePolicy{}
	cond := "false"
	spec := newPolicySpec("body", "v1", true, &cond)
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	result, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", true)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
	assert.Nil(t, result.FinalAction)
}

func TestExecuteStreamingResponsePolicies_Passthrough_FinalActionSet(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &noopStreamingResponsePolicy{}
	spec := newPolicySpec("body", "v1", true, nil)
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("original")}

	result, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.NotNil(t, result.FinalAction)
	assert.Nil(t, result.FinalAction.BodyMutation)
}

func TestExecuteStreamingResponsePolicies_Mutation_FinalActionHasBytes(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	pol := &mutatingStreamingResponsePolicy{out: []byte("replaced")}
	spec := newPolicySpec("body", "v1", true, nil)
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("original")}

	result, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, ctx, chunk, []policy.PolicySpec{spec}, "", "", false)

	require.NoError(t, err)
	require.NotNil(t, result.FinalAction)
	assert.Equal(t, []byte("replaced"), result.FinalAction.BodyMutation)
}

func TestExecuteStreamingResponsePolicies_ReverseOrder(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	order := []string{}
	pol1 := &trackingStreamingResponsePolicy{name: "first", executionOrder: &order}
	pol2 := &trackingStreamingResponsePolicy{name: "second", executionOrder: &order}
	specs := []policy.PolicySpec{
		newPolicySpec("first", "v1", true, nil),
		newPolicySpec("second", "v1", true, nil),
	}
	ctx := &policy.ResponseStreamContext{}
	chunk := &policy.StreamBody{Chunk: []byte("data")}

	_, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol1, pol2}, ctx, chunk, specs, "", "", false)

	require.NoError(t, err)
	// Response policies execute in reverse order
	assert.Equal(t, []string{"second", "first"}, order)
}
