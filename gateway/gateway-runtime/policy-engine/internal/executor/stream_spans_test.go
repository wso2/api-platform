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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func countSpans(spans []sdktrace.ReadOnlySpan, name string) int {
	n := 0
	for _, s := range spans {
		if s.Name() == name {
			n++
		}
	}
	return n
}

func streamRespCtx() *policy.ResponseStreamContext {
	return &policy.ResponseStreamContext{
		SharedContext:   testutils.NewTestSharedContext(),
		RequestHeaders:  policy.NewHeaders(map[string][]string{}),
		ResponseHeaders: policy.NewHeaders(map[string][]string{}),
		ResponseStatus:  200,
	}
}

func streamReqCtx() *policy.RequestStreamContext {
	return &policy.RequestStreamContext{
		SharedContext: testutils.NewTestSharedContext(),
		Headers:       policy.NewHeaders(map[string][]string{}),
	}
}

// A policy that does not take part in the stream used to get an empty span on every
// flush (#3736).
func TestExecuteStreamingResponsePolicies_NoSpanForNonParticipatingPolicy(t *testing.T) {
	exec, sr := newRecordedChainExecutor(t, nil)
	streaming := &countingStreamingResponsePolicy{mode: policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}}
	buffered := &countingStreamingResponsePolicy{mode: policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}}
	specs := []policy.PolicySpec{newPolicySpec("streams", "v1", true, nil), newPolicySpec("buffers", "v1", true, nil), newPolicySpec("headers", "v1", true, nil)}

	_, err := exec.ExecuteStreamingResponsePolicies(context.Background(),
		[]policy.Policy{streaming, buffered, &testutils.NoopPolicy{}}, streamRespCtx(),
		&policy.StreamBody{Chunk: []byte("a")}, specs, "api", "route", false, nil)
	require.NoError(t, err)

	ended := sr.Ended()
	assert.Equal(t, 1, countSpans(ended, "policy.response.streams"))
	assert.Zero(t, countSpans(ended, "policy.response.buffers"), "a buffering policy does not run per chunk")
	assert.Zero(t, countSpans(ended, "policy.response.headers"), "a policy with no streaming callback does not run per chunk")
}

func TestExecuteStreamingRequestPolicies_NoSpanForNonParticipatingPolicy(t *testing.T) {
	exec, sr := newRecordedChainExecutor(t, nil)
	streaming := &countingStreamingRequestPolicy{mode: policy.ProcessingMode{RequestBodyMode: policy.BodyModeStream}}
	specs := []policy.PolicySpec{newPolicySpec("streams", "v1", true, nil), newPolicySpec("headers", "v1", true, nil)}

	_, err := exec.ExecuteStreamingRequestPolicies(context.Background(),
		[]policy.Policy{streaming, &testutils.NoopPolicy{}}, streamReqCtx(),
		&policy.StreamBody{Chunk: []byte("a")}, specs, "api", "route", false, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, countSpans(sr.Ended(), "policy.request.streams"))
	assert.Zero(t, countSpans(sr.Ended(), "policy.request.headers"))
}

// With StreamSpans, every flush of a stream lands on one span per policy, which stays
// open until End and sums the policy's execution time.
func TestStreamSpans_OneSpanPerPolicyAcrossFlushes(t *testing.T) {
	exec, sr := newRecordedChainExecutor(t, nil)
	reqPol := &countingStreamingRequestPolicy{mode: policy.ProcessingMode{RequestBodyMode: policy.BodyModeStream}}
	respPol := &countingStreamingResponsePolicy{mode: policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}}
	reqSpans, respSpans := NewStreamSpans(), NewStreamSpans()

	for i := 0; i < 3; i++ {
		_, err := exec.ExecuteStreamingRequestPolicies(context.Background(), []policy.Policy{reqPol}, streamReqCtx(),
			&policy.StreamBody{Chunk: []byte("a"), EndOfStream: i == 2}, []policy.PolicySpec{newPolicySpec("req", "v1", true, nil)},
			"api", "route", false, reqSpans)
		require.NoError(t, err)
		_, err = exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{respPol}, streamRespCtx(),
			&policy.StreamBody{Chunk: []byte("a"), EndOfStream: i == 2}, []policy.PolicySpec{newPolicySpec("resp", "v1", true, nil)},
			"api", "route", false, respSpans)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, reqPol.calls)
	assert.Equal(t, 3, respPol.calls)
	assert.Empty(t, sr.Ended(), "a stream's policy spans stay open until the stream ends")

	reqSpans.End()
	respSpans.End()
	reqSpans.End() // idempotent

	ended := sr.Ended()
	assert.Equal(t, 1, countSpans(ended, "policy.request.req"))
	assert.Equal(t, 1, countSpans(ended, "policy.response.resp"))
	span := findSpanByName(ended, "policy.response.resp")
	for _, kv := range span.Attributes() {
		if string(kv.Key) == constants.AttrPolicyExecutionTimeNS {
			assert.Positive(t, kv.Value.AsInt64())
		}
	}
}
