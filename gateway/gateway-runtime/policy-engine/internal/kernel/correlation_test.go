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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	token := st.GetFields()[CorrelationTokenKey].GetStringValue()
	require.NotEmpty(t, token, "the struct tells the ALS side where the fields went")
	assert.Equal(t, execCtx.correlationToken, token)
	payload, ok := store.Get(token)
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

	payload, ok := store.Get(execCtx.correlationToken)
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
		_, ok := store.Get(inFlight.correlationToken)
		assert.True(t, ok, "an unread, in-flight entry is never evicted")
	})

	t.Run("unrecognised header shape", func(t *testing.T) {
		store := correlation.NewStore(100, time.Minute, 4)
		execCtx := correlatedExecCtx(newTestServerWithStore(t, store), "req-1")
		st, err := buildAnalyticsStruct(map[string]any{"request_headers": 12345}, execCtx)
		require.NoError(t, err)
		assert.Contains(t, st.GetFields(), "request_headers")
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

	tokenA := stA.GetFields()[CorrelationTokenKey].GetStringValue()
	tokenB := stB.GetFields()[CorrelationTokenKey].GetStringValue()
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
	assert.NotContains(t, st.GetFields(), CorrelationTokenKey)

	loopback.analyticsMetadata[analyticsInternalLoopbackKey] = "true"
	server.completeCorrelationEntry(loopback)

	payload, ok := store.Get(outer.correlationToken)
	require.True(t, ok)
	assert.Equal(t, "outer", payload.RequestHeaders["who"], "outer entry untouched")
	assert.False(t, store.Merge("other", correlation.Payload{RequestHeaders: map[string]string{"x": "y"}}),
		"outer entry is still in flight, so its slot is not reclaimable")
}

// Completing a finished request makes its unread entry reclaimable after the TTL.
func TestCompleteCorrelationEntry_MakesEntryReclaimable(t *testing.T) {
	store := correlation.NewStore(1, time.Nanosecond, 1)
	server := newTestServerWithStore(t, store)
	execCtx := correlatedExecCtx(server, "done")
	_, err := buildAnalyticsStruct(map[string]any{"request_headers": map[string]string{"a": "b"}}, execCtx)
	require.NoError(t, err)

	server.completeCorrelationEntry(execCtx)
	time.Sleep(time.Millisecond)

	assert.True(t, store.Merge("next", correlation.Payload{RequestHeaders: map[string]string{"x": "y"}}))
}

func TestCorrelationTokenKey(t *testing.T) {
	// The ALS side (internal/analytics) spells out the same key.
	assert.Equal(t, "x-wso2-correlation-token", CorrelationTokenKey)
}

func TestCompleteCorrelationEntry_NilStoreIsNoop(t *testing.T) {
	server := newTestServerWithStore(t, nil)
	assert.NotPanics(t, func() { server.completeCorrelationEntry(correlatedExecCtx(server, "req-1")) })
}

// TestNormalizeAnalyticsHeaderValue covers every shape captured headers can
// arrive in inside execCtx.analyticsMetadata (see its doc comment in
// analytics.go), asserting the flattening matches exactly what the old
// JSON-encode-then-decode-back round trip used to produce for each shape.
func TestNormalizeAnalyticsHeaderValue(t *testing.T) {
	t.Run("map[string]string is used as-is", func(t *testing.T) {
		got := normalizeAnalyticsHeaderValue(map[string]string{"host": "example.com"})
		assert.Equal(t, map[string]string{"host": "example.com"}, got)
	})

	t.Run("map[string][]string flattens to the first value", func(t *testing.T) {
		got := normalizeAnalyticsHeaderValue(map[string][]string{"x-foo": {"a", "b"}})
		assert.Equal(t, map[string]string{"x-foo": "a"}, got)
	})

	t.Run("JSON string (single-value map) is decoded", func(t *testing.T) {
		got := normalizeAnalyticsHeaderValue(`{"host":"example.com"}`)
		assert.Equal(t, map[string]string{"host": "example.com"}, got)
	})

	t.Run("JSON string (multi-value map) flattens to the first value", func(t *testing.T) {
		got := normalizeAnalyticsHeaderValue(`{"x-foo":["a","b"]}`)
		assert.Equal(t, map[string]string{"x-foo": "a"}, got)
	})

	t.Run("nil is nil", func(t *testing.T) {
		assert.Nil(t, normalizeAnalyticsHeaderValue(nil))
	})

	t.Run("unrecognized shape is nil", func(t *testing.T) {
		assert.Nil(t, normalizeAnalyticsHeaderValue(12345))
	})
}
