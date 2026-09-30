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
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Each policy below records the span in its context when called, so a test
// can assert it is the executor's per-policy span, not the caller's parent.

type spanCapturingRequestHeaderPolicy struct {
	mode policy.ProcessingMode
	seen trace.SpanContext
}

func (p *spanCapturingRequestHeaderPolicy) Mode() policy.ProcessingMode { return p.mode }
func (p *spanCapturingRequestHeaderPolicy) OnRequestHeaders(ctx context.Context, _ *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	p.seen = trace.SpanFromContext(ctx).SpanContext()
	return policy.UpstreamRequestHeaderModifications{}
}

type spanCapturingResponseHeaderPolicy struct {
	mode policy.ProcessingMode
	seen trace.SpanContext
}

func (p *spanCapturingResponseHeaderPolicy) Mode() policy.ProcessingMode { return p.mode }
func (p *spanCapturingResponseHeaderPolicy) OnResponseHeaders(ctx context.Context, _ *policy.ResponseHeaderContext, _ map[string]interface{}) policy.ResponseHeaderAction {
	p.seen = trace.SpanFromContext(ctx).SpanContext()
	return policy.DownstreamResponseHeaderModifications{}
}

type spanCapturingRequestBodyPolicy struct {
	mode policy.ProcessingMode
	seen trace.SpanContext
}

func (p *spanCapturingRequestBodyPolicy) Mode() policy.ProcessingMode { return p.mode }
func (p *spanCapturingRequestBodyPolicy) OnRequestBody(ctx context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	p.seen = trace.SpanFromContext(ctx).SpanContext()
	return policy.UpstreamRequestModifications{}
}

type spanCapturingResponseBodyPolicy struct {
	mode policy.ProcessingMode
	seen trace.SpanContext
}

func (p *spanCapturingResponseBodyPolicy) Mode() policy.ProcessingMode { return p.mode }
func (p *spanCapturingResponseBodyPolicy) OnResponseBody(ctx context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	p.seen = trace.SpanFromContext(ctx).SpanContext()
	return policy.DownstreamResponseModifications{}
}

type spanCapturingStreamingRequestPolicy struct {
	mode policy.ProcessingMode
	seen trace.SpanContext
}

func (p *spanCapturingStreamingRequestPolicy) Mode() policy.ProcessingMode { return p.mode }
func (p *spanCapturingStreamingRequestPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{}
}
func (p *spanCapturingStreamingRequestPolicy) NeedsMoreRequestData(_ []byte) bool { return false }
func (p *spanCapturingStreamingRequestPolicy) OnRequestBodyChunk(ctx context.Context, _ *policy.RequestStreamContext, _ *policy.StreamBody, _ map[string]interface{}) policy.StreamingRequestAction {
	p.seen = trace.SpanFromContext(ctx).SpanContext()
	return policy.ForwardRequestChunk{}
}

type spanCapturingStreamingResponsePolicy struct {
	mode policy.ProcessingMode
	seen trace.SpanContext
}

func (p *spanCapturingStreamingResponsePolicy) Mode() policy.ProcessingMode { return p.mode }
func (p *spanCapturingStreamingResponsePolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{}
}
func (p *spanCapturingStreamingResponsePolicy) NeedsMoreResponseData(_ []byte) bool { return false }
func (p *spanCapturingStreamingResponsePolicy) OnResponseBodyChunk(ctx context.Context, _ *policy.ResponseStreamContext, _ *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	p.seen = trace.SpanFromContext(ctx).SpanContext()
	return policy.ForwardResponseChunk{}
}

func TestChainExecutor_PolicySeesItsOwnSpan_AllPhases(t *testing.T) {
	t.Run("request header", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, nil)
		pol := &spanCapturingRequestHeaderPolicy{mode: policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}}
		reqCtx := &policy.RequestHeaderContext{
			SharedContext: testutils.NewTestSharedContext(),
			Headers:       policy.NewHeaders(map[string][]string{}),
			Path:          "/test",
			Method:        "GET",
		}

		_, err := executor.ExecuteRequestHeaderPolicies(context.Background(), []policy.Policy{pol}, reqCtx,
			[]policy.PolicySpec{newPolicySpec("hdr", "v1.0.0", true, nil)}, "api", "route", false)
		require.NoError(t, err)

		span := findSpanByName(sr.Ended(), "policy.request.hdr")
		require.NotNil(t, span)
		require.True(t, pol.seen.IsValid())
		assert.True(t, pol.seen.Equal(span.SpanContext()))
	})

	t.Run("response header", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, nil)
		pol := &spanCapturingResponseHeaderPolicy{mode: policy.ProcessingMode{ResponseHeaderMode: policy.HeaderModeProcess}}
		respCtx := &policy.ResponseHeaderContext{
			SharedContext:   testutils.NewTestSharedContext(),
			RequestHeaders:  policy.NewHeaders(map[string][]string{}),
			RequestPath:     "/test",
			RequestMethod:   "GET",
			ResponseHeaders: policy.NewHeaders(map[string][]string{}),
			ResponseStatus:  200,
		}

		_, err := executor.ExecuteResponseHeaderPolicies(context.Background(), []policy.Policy{pol}, respCtx,
			[]policy.PolicySpec{newPolicySpec("resphdr", "v1.0.0", true, nil)}, "api", "route", false)
		require.NoError(t, err)

		span := findSpanByName(sr.Ended(), "policy.response.resphdr")
		require.NotNil(t, span)
		require.True(t, pol.seen.IsValid())
		assert.True(t, pol.seen.Equal(span.SpanContext()))
	})

	t.Run("request body", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, nil)
		pol := &spanCapturingRequestBodyPolicy{}
		reqCtx := testutils.NewTestRequestContext()

		_, err := executor.ExecuteRequestPolicies(context.Background(), []policy.Policy{pol}, reqCtx,
			[]policy.PolicySpec{newPolicySpec("body", "v1.0.0", true, nil)}, "api", "route", false)
		require.NoError(t, err)

		span := findSpanByName(sr.Ended(), "policy.request.body")
		require.NotNil(t, span)
		require.True(t, pol.seen.IsValid())
		assert.True(t, pol.seen.Equal(span.SpanContext()))
	})

	t.Run("response body", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, nil)
		pol := &spanCapturingResponseBodyPolicy{}
		respCtx := testutils.NewTestResponseContext()

		_, err := executor.ExecuteResponsePolicies(context.Background(), []policy.Policy{pol}, respCtx,
			[]policy.PolicySpec{newPolicySpec("respbody", "v1.0.0", true, nil)}, "api", "route", false)
		require.NoError(t, err)

		span := findSpanByName(sr.Ended(), "policy.response.respbody")
		require.NotNil(t, span)
		require.True(t, pol.seen.IsValid())
		assert.True(t, pol.seen.Equal(span.SpanContext()))
	})

	t.Run("streaming request", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, nil)
		pol := &spanCapturingStreamingRequestPolicy{mode: policy.ProcessingMode{RequestBodyMode: policy.BodyModeStream}}
		reqCtx := &policy.RequestStreamContext{
			SharedContext: testutils.NewTestSharedContext(),
			Headers:       policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
			Path:          "/test",
			Method:        "POST",
		}
		chunk := &policy.StreamBody{Chunk: []byte("hello"), EndOfStream: true}

		_, err := executor.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{pol}, reqCtx, chunk,
			[]policy.PolicySpec{newPolicySpec("stream", "v1.0.0", true, nil)}, "api", "route", false)
		require.NoError(t, err)

		span := findSpanByName(sr.Ended(), "policy.request.stream")
		require.NotNil(t, span)
		require.True(t, pol.seen.IsValid())
		assert.True(t, pol.seen.Equal(span.SpanContext()))
	})

	t.Run("streaming response", func(t *testing.T) {
		executor, sr := newRecordedChainExecutor(t, nil)
		pol := &spanCapturingStreamingResponsePolicy{mode: policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}}
		respCtx := &policy.ResponseStreamContext{
			SharedContext:   testutils.NewTestSharedContext(),
			RequestHeaders:  policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
			RequestPath:     "/test",
			RequestMethod:   "POST",
			ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
			ResponseStatus:  200,
		}
		chunk := &policy.StreamBody{Chunk: []byte("hello"), EndOfStream: true}

		_, err := executor.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, respCtx, chunk,
			[]policy.PolicySpec{newPolicySpec("respstream", "v1.0.0", true, nil)}, "api", "route", false)
		require.NoError(t, err)

		span := findSpanByName(sr.Ended(), "policy.response.respstream")
		require.NotNil(t, span)
		require.True(t, pol.seen.IsValid())
		assert.True(t, pol.seen.Equal(span.SpanContext()))
	})
}
