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
	"time"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
)

func newTestServerWithStore(t *testing.T, store *correlation.Store) *ExternalProcessorServer {
	t.Helper()
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	return NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, store)
}

// correlatedExecCtx returns an execution context for one ext_proc stream.
func correlatedExecCtx(server *ExternalProcessorServer, requestID string) *PolicyExecutionContext {
	execCtx := newPolicyExecutionContext(server, "test-route", nil)
	execCtx.requestID = requestID
	return execCtx
}

// A captured field must already be in the store when buildAnalyticsStruct returns
// the struct it was left out of: that struct goes into the ext_proc response, and
// Envoy can emit the access-log entry as soon as it has that response.
func TestBuildAnalyticsStruct_StoresCapturedHeadersBeforeResponse(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")

	st, err := buildAnalyticsStruct(map[string]any{
		"request_headers": map[string]string{"host": "example.com"},
		"source":          "policy",
	}, execCtx)
	require.NoError(t, err)

	_, inMetadata := st.GetFields()["request_headers"]
	assert.False(t, inMetadata, "accepted by the store, so left out of Envoy metadata")
	assert.Equal(t, "policy", st.GetFields()["source"].GetStringValue(), "unrelated fields still go to Envoy")
	token := st.GetFields()[analytics.CorrelationTokenKey].GetStringValue()
	require.NotEmpty(t, token, "the struct tells the ALS side where the fields went")
	assert.Equal(t, execCtx.correlationToken, token)
	payload, ok := store.Take(token)
	require.True(t, ok, "stored synchronously, before the response is sent")
	assert.Equal(t, "example.com", payload.RequestHeaders["host"])
}

// Each phase merges its own fields into the request's single entry.
func TestBuildAnalyticsStruct_MergesPhasesIntoOneEntry(t *testing.T) {
	store := correlation.NewStoreWithBodyLimits(100, time.Minute, 4, 1024, 4096)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")

	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "1"}}, execCtx)
	require.NoError(t, err)
	_, err = buildAnalyticsStruct(map[string]any{"request_payload": "body"}, execCtx)
	require.NoError(t, err)
	_, err = buildAnalyticsStruct(map[string]any{"response_headers": map[string]string{"b": "2"}}, execCtx)
	require.NoError(t, err)

	payload, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok)
	assert.Equal(t, "1", payload.RequestHeaders["a"])
	assert.Equal(t, "body", payload.RequestBody)
	assert.Equal(t, "2", payload.ResponseHeaders["b"])
}

// Whenever the store does not take a field, it must stay in Envoy metadata.
func TestBuildAnalyticsStruct_KeepsFieldsInMetadataWhenNotStored(t *testing.T) {
	headers := map[string]string{"host": "example.com"}

	t.Run("store full of in-flight requests", func(t *testing.T) {
		store := correlation.NewStore(1, time.Nanosecond, 1)
		server := newTestServerWithStore(t, store)
		inFlight := correlatedExecCtx(server, "in-flight")
		_, err := buildAnalyticsStruct(map[string]any{"request_headers": headers}, inFlight)
		require.NoError(t, err)

		st, err := buildAnalyticsStruct(map[string]any{"request_headers": headers}, correlatedExecCtx(server, "next"))
		require.NoError(t, err)
		assert.Contains(t, st.GetFields(), "request_headers", "no slot, so the field stays in metadata")
		_, ok := store.Take(inFlight.correlationToken)
		assert.True(t, ok, "an unread, in-flight entry is never evicted")
	})

	t.Run("no store", func(t *testing.T) {
		execCtx := correlatedExecCtx(newTestServerWithStore(t, nil), "req-1")
		st, err := buildAnalyticsStruct(map[string]any{"request_headers": headers}, execCtx)
		require.NoError(t, err)
		assert.Contains(t, st.GetFields(), "request_headers")
	})
}

// Envoy keeps a client-supplied x-request-id, so concurrent requests can share
// one. Each stream must still get its own entry, or one request's line would
// receive the other's fields and the other's would miss.
func TestCorrelation_DuplicateRequestIDsGetSeparateEntries(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	server := newTestServerWithStore(t, store)
	a := correlatedExecCtx(server, "client-chosen-id")
	b := correlatedExecCtx(server, "client-chosen-id")

	stA, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"who": "a"}}, a)
	require.NoError(t, err)
	stB, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"who": "b"}}, b)
	require.NoError(t, err)

	tokenA := stA.GetFields()[analytics.CorrelationTokenKey].GetStringValue()
	tokenB := stB.GetFields()[analytics.CorrelationTokenKey].GetStringValue()
	require.NotEqual(t, tokenA, tokenB)
	gotA, ok := store.Take(tokenA)
	require.True(t, ok)
	gotB, ok := store.Take(tokenB)
	require.True(t, ok)
	assert.Equal(t, "a", gotA.RequestHeaders["who"])
	assert.Equal(t, "b", gotB.RequestHeaders["who"])
}

// The LLM proxy's internal loopback hop keeps its data in Envoy metadata (its own
// access-log event is suppressed) and never writes or completes another entry.
func TestCorrelation_LoopbackHopDoesNotTouchOuterEntry(t *testing.T) {
	store := correlation.NewStore(1, time.Nanosecond, 1)
	server := newTestServerWithStore(t, store)

	outer := correlatedExecCtx(server, "shared-id")
	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"who": "outer"}}, outer)
	require.NoError(t, err)

	loopback := correlatedExecCtx(server, "shared-id")
	st, err := buildAnalyticsStruct(map[string]any{
		analyticsInternalLoopbackKey: "true",
		"request_headers":            map[string]string{"who": "loopback"},
	}, loopback)
	require.NoError(t, err)
	assert.Contains(t, st.GetFields(), "request_headers", "loopback hop keeps its data in metadata")
	assert.NotContains(t, st.GetFields(), analytics.CorrelationTokenKey)

	loopback.analyticsMetadata[analyticsInternalLoopbackKey] = "true"
	loopback.responseFinished = true
	server.completeCorrelationEntry(loopback)

	assert.False(t, store.Merge("other", correlation.Payload{RequestHeaders: map[string]string{"x": "y"}}),
		"outer entry is still in flight, so its slot is not reclaimable")
	payload, ok := store.Take(outer.correlationToken)
	require.True(t, ok)
	assert.Equal(t, "outer", payload.RequestHeaders["who"], "outer entry untouched")
}

// Completing a finished request makes its unread entry reclaimable after the TTL.
func TestCompleteCorrelationEntry_MakesEntryReclaimable(t *testing.T) {
	store := correlation.NewStore(1, time.Nanosecond, 1)
	server := newTestServerWithStore(t, store)
	execCtx := correlatedExecCtx(server, "done")
	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, execCtx)
	require.NoError(t, err)

	execCtx.correlationTokenSent = true
	execCtx.responseFinished = true
	server.completeCorrelationEntry(execCtx)
	time.Sleep(time.Millisecond)

	assert.True(t, store.Merge("next", correlation.Payload{RequestHeaders: map[string]string{"x": "y"}}))
}

// A stream can close before the response ends (response body processing skipped:
// Envoy ends the stream after the response headers while the body still streams).
// Such an entry must not be completed, so the TTL cannot reclaim it before its
// access-log entry arrives.
func TestCompleteCorrelationEntry_WaitsForResponseEnd(t *testing.T) {
	store := correlation.NewStore(1, time.Nanosecond, 1)
	server := newTestServerWithStore(t, store)
	execCtx := correlatedExecCtx(server, "streaming")
	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, execCtx)
	require.NoError(t, err)

	execCtx.correlationTokenSent = true
	server.completeCorrelationEntry(execCtx) // stream closed, response not seen to end
	time.Sleep(time.Millisecond)

	assert.False(t, store.Merge("next", correlation.Payload{RequestHeaders: map[string]string{"x": "y"}}),
		"an entry whose response may still be streaming is not reclaimable")
	payload, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok, "the ALS handler still finds it")
	assert.Equal(t, "b", payload.RequestHeaders["a"])
}

func TestEndsResponse(t *testing.T) {
	plain := &extprocv3.ProcessingResponse{}
	immediate := &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: &extprocv3.ImmediateResponse{}}}
	cases := []struct {
		name string
		req  *extprocv3.ProcessingRequest
		resp *extprocv3.ProcessingResponse
		want bool
	}{
		{"request headers", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestHeaders{RequestHeaders: &extprocv3.HttpHeaders{}}}, plain, false},
		{"immediate response", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestHeaders{RequestHeaders: &extprocv3.HttpHeaders{}}}, immediate, true},
		{"response headers, body follows", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseHeaders{ResponseHeaders: &extprocv3.HttpHeaders{}}}, plain, false},
		{"response headers, end of stream", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseHeaders{ResponseHeaders: &extprocv3.HttpHeaders{EndOfStream: true}}}, plain, true},
		{"response body chunk", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseBody{ResponseBody: &extprocv3.HttpBody{}}}, plain, false},
		{"last response body chunk", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseBody{ResponseBody: &extprocv3.HttpBody{EndOfStream: true}}}, plain, true},
		{"response trailers", &extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseTrailers{ResponseTrailers: &extprocv3.HttpTrailers{}}}, plain, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, endsResponse(tc.req, tc.resp))
		})
	}
}

// Requests on collector.ignore_path_prefixes never get an access-log entry, so
// their fields are not stored (they stay in metadata and are dropped with it).
func TestStoreInProcess_SkipsIgnoredPaths(t *testing.T) {
	store := correlation.NewStoreFromConfig(config.CollectorConfig{
		CorrelationStore:   config.CorrelationStoreConfig{Capacity: 100, TTL: time.Minute, Shards: 4},
		IgnorePathPrefixes: []string{"/health"},
	})
	server := newTestServerWithStore(t, store)

	ignored := correlatedExecCtx(server, "ignored")
	ignored.clientPath = "/health/live"
	st, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, ignored)
	require.NoError(t, err)
	assert.Contains(t, st.GetFields(), "request_headers", "ignored path: kept in metadata")
	assert.Empty(t, ignored.correlationToken, "nothing stored")

	logged := correlatedExecCtx(server, "logged")
	logged.clientPath = "/api/v1/health"
	st, err = buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, logged)
	require.NoError(t, err)
	assert.NotContains(t, st.GetFields(), "request_headers", "prefix match only: stored")
}

// A repeated header from the analytics-header-filter path is stored with every
// value, joined with ", ".
func TestStoreInProcess_KeepsRepeatedHeaderValues(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "multi")
	_, err := buildAnalyticsStruct(map[string]any{
		"request_headers": map[string][]string{"set-cookie": {"a=1", "b=2"}},
	}, execCtx)
	require.NoError(t, err)
	payload, ok := store.Take(execCtx.correlationToken)
	require.True(t, ok)
	assert.Equal(t, "a=1, b=2", payload.RequestHeaders["set-cookie"])
}

func TestCompleteCorrelationEntry_NilStoreIsNoop(t *testing.T) {
	server := newTestServerWithStore(t, nil)
	assert.NotPanics(t, func() { server.completeCorrelationEntry(correlatedExecCtx(server, "req-1")) })
}
