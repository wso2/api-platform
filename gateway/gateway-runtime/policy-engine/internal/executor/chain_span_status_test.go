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

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// countingRequestBodyPolicy and countingResponseBodyPolicy round out the
// counting-policy family declared in chain_test.go (which only covers header
// and streaming phases) with buffered request/response body participants.
type countingRequestBodyPolicy struct {
	mode  policy.ProcessingMode
	calls int
}

func (p *countingRequestBodyPolicy) Mode() policy.ProcessingMode { return p.mode }

func (p *countingRequestBodyPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	p.calls++
	return policy.UpstreamRequestModifications{}
}

type countingResponseBodyPolicy struct {
	mode  policy.ProcessingMode
	calls int
}

func (p *countingResponseBodyPolicy) Mode() policy.ProcessingMode { return p.mode }

func (p *countingResponseBodyPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	p.calls++
	return policy.DownstreamResponseModifications{}
}

// newRecordedChainExecutor builds a ChainExecutor backed by a real SDK
// TracerProvider (recorded by sr), so per-policy span status/attributes can be
// asserted on rather than exercising the noop tracer used elsewhere in this package.
func newRecordedChainExecutor(t *testing.T, celEval CELEvaluator) (*ChainExecutor, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return NewChainExecutor(nil, celEval, tp.Tracer("test")), sr
}

func findSpanByName(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func conditionStr() *string {
	s := "some.condition == true"
	return &s
}

// =============================================================================
// Condition-evaluation failure: span status across all six chain entry points.
//
// Site :449 (ExecuteResponseHeaderPolicies) previously did a bare span.End() on
// this failure path, unlike its five siblings — this both fixes that gap and
// locks in that all six now agree on the exact same description string.
// =============================================================================

func TestConditionEvalFailure_SpanStatus_AllPhases(t *testing.T) {
	failingEval := &mockCELEvaluator{requestErr: assert.AnError, responseErr: assert.AnError}

	t.Run("request header", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, failingEval)
		pol := &countingRequestHeaderPolicy{mode: policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}}
		reqCtx := &policy.RequestHeaderContext{
			SharedContext: testutils.NewTestSharedContext(),
			Headers:       policy.NewHeaders(map[string][]string{}),
			Path:          "/test",
			Method:        "GET",
		}

		_, err := executor.ExecuteRequestHeaderPolicies(context.Background(), []policy.Policy{pol}, reqCtx,
			[]policy.PolicySpec{newPolicySpec("hdr", "v1.0.0", true, conditionStr())}, "api", "route", true)

		require.Error(t, err)
		span := findSpanByName(sr.Ended(), "policy.request.hdr")
		require.NotNil(t, span)
		assertConditionEvalFailure(t, span)
	})

	t.Run("request body", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, failingEval)
		reqCtx := testutils.NewTestRequestContext()

		_, err := executor.ExecuteRequestPolicies(context.Background(), []policy.Policy{&testutils.NoopPolicy{}}, reqCtx,
			[]policy.PolicySpec{newPolicySpec("body", "v1.0.0", true, conditionStr())}, "api", "route", true)

		require.Error(t, err)
		span := findSpanByName(sr.Ended(), "policy.request.body")
		require.NotNil(t, span)
		assertConditionEvalFailure(t, span)
	})

	t.Run("response header", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, failingEval)
		pol := &countingResponseHeaderPolicy{mode: policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}}
		respCtx := &policy.ResponseHeaderContext{
			SharedContext:   testutils.NewTestSharedContext(),
			RequestHeaders:  policy.NewHeaders(map[string][]string{}),
			RequestPath:     "/test",
			RequestMethod:   "GET",
			ResponseHeaders: policy.NewHeaders(map[string][]string{}),
			ResponseStatus:  200,
		}

		_, err := executor.ExecuteResponseHeaderPolicies(context.Background(), []policy.Policy{pol}, respCtx,
			[]policy.PolicySpec{newPolicySpec("resphdr", "v1.0.0", true, conditionStr())}, "api", "route", true)

		require.Error(t, err)
		span := findSpanByName(sr.Ended(), "policy.response.resphdr")
		require.NotNil(t, span)
		assertConditionEvalFailure(t, span)
	})

	t.Run("response body", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, failingEval)
		respCtx := testutils.NewTestResponseContext()

		_, err := executor.ExecuteResponsePolicies(context.Background(), []policy.Policy{&testutils.NoopPolicy{}}, respCtx,
			[]policy.PolicySpec{newPolicySpec("respbody", "v1.0.0", true, conditionStr())}, "api", "route", true)

		require.Error(t, err)
		span := findSpanByName(sr.Ended(), "policy.response.respbody")
		require.NotNil(t, span)
		assertConditionEvalFailure(t, span)
	})

	t.Run("streaming request", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, failingEval)
		pol := &countingStreamingRequestPolicy{mode: policy.ProcessingMode{RequestBodyMode: policy.BodyModeStream}}
		reqCtx := &policy.RequestStreamContext{
			SharedContext: testutils.NewTestSharedContext(),
			Headers:       policy.NewHeaders(map[string][]string{}),
			Path:          "/test",
			Method:        "POST",
		}
		chunk := &policy.StreamBody{Chunk: []byte("hello"), EndOfStream: true}

		_, err := executor.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, reqCtx, chunk,
			[]policy.PolicySpec{newPolicySpec("streq", "v1.0.0", true, conditionStr())}, "api", "route", true)

		require.Error(t, err)
		span := findSpanByName(sr.Ended(), "policy.request.streq")
		require.NotNil(t, span)
		assertConditionEvalFailure(t, span)
	})

	t.Run("streaming response", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, failingEval)
		pol := &countingStreamingResponsePolicy{mode: policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}}
		respCtx := &policy.ResponseStreamContext{
			SharedContext:   testutils.NewTestSharedContext(),
			RequestHeaders:  policy.NewHeaders(map[string][]string{}),
			RequestPath:     "/test",
			RequestMethod:   "POST",
			ResponseHeaders: policy.NewHeaders(map[string][]string{}),
			ResponseStatus:  200,
		}
		chunk := &policy.StreamBody{Chunk: []byte("hello"), EndOfStream: true}

		_, err := executor.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, respCtx, chunk,
			[]policy.PolicySpec{newPolicySpec("stresp", "v1.0.0", true, conditionStr())}, "api", "route", true)

		require.Error(t, err)
		span := findSpanByName(sr.Ended(), "policy.response.stresp")
		require.NotNil(t, span)
		assertConditionEvalFailure(t, span)
	})
}

func assertConditionEvalFailure(t *testing.T, span sdktrace.ReadOnlySpan) {
	t.Helper()
	status := span.Status()
	assert.Equal(t, codes.Error, status.Code)
	assert.Equal(t, "condition evaluation failed", status.Description)
	require.Len(t, span.Events(), 1)
	assert.Equal(t, "exception", span.Events()[0].Name)
}

