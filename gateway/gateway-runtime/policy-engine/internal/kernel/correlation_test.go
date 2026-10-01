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

// TestWriteCorrelationEntry_StoresWhenRequestIDFromHeaderAndHeadersCaptured
// covers the write side of Step 4's steady-state path: a real x-request-id and
// captured headers must land in the store, keyed by that request id.
func TestWriteCorrelationEntry_StoresWhenRequestIDFromHeaderAndHeadersCaptured(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	server := newTestServerWithStore(t, store)

	execCtx := newPolicyExecutionContext(server, "test-route", nil)
	execCtx.requestID = "req-1"
	execCtx.requestIDFromHeader = true
	execCtx.analyticsMetadata["request_headers"] = map[string]string{"host": "example.com"}

	server.writeCorrelationEntry(execCtx)

	payload, ok := store.Get("req-1")
	require.True(t, ok, "expected the entry to be stored")
	assert.Equal(t, "example.com", payload.RequestHeaders["host"])
}

// TestWriteCorrelationEntry_SkipsWhenNoRealRequestID covers the ext_proc uuid
// fallback: when x-request-id was absent, execCtx.requestID is a locally
// generated id the ALS side can never look up, so the write must be skipped
// entirely rather than wasting a store slot no one will ever read.
func TestWriteCorrelationEntry_SkipsWhenNoRealRequestID(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	server := newTestServerWithStore(t, store)

	execCtx := newPolicyExecutionContext(server, "test-route", nil)
	execCtx.requestID = "generated-uuid-fallback"
	execCtx.requestIDFromHeader = false
	execCtx.analyticsMetadata["request_headers"] = map[string]string{"host": "example.com"}

	server.writeCorrelationEntry(execCtx)

	_, ok := store.Get("generated-uuid-fallback")
	assert.False(t, ok, "must never store an entry keyed by a locally generated request id")
}

// TestWriteCorrelationEntry_SkipsWhenNothingCaptured covers a request where
// header capture wasn't enabled (or no policy contributed anything): nothing
// worth correlating, so the store must stay empty.
func TestWriteCorrelationEntry_SkipsWhenNothingCaptured(t *testing.T) {
	store := correlation.NewStore(100, time.Minute, 4)
	server := newTestServerWithStore(t, store)

	execCtx := newPolicyExecutionContext(server, "test-route", nil)
	execCtx.requestID = "req-empty"
	execCtx.requestIDFromHeader = true
	// analyticsMetadata has no request_headers/response_headers entries at all.

	server.writeCorrelationEntry(execCtx)

	_, ok := store.Get("req-empty")
	assert.False(t, ok)
}

// TestWriteCorrelationEntry_NilStoreIsNoop covers the collector-disabled
// deployment: ExternalProcessorServer.correlationStore is nil, and
// writeCorrelationEntry must not panic or otherwise misbehave.
func TestWriteCorrelationEntry_NilStoreIsNoop(t *testing.T) {
	server := newTestServerWithStore(t, nil)

	execCtx := newPolicyExecutionContext(server, "test-route", nil)
	execCtx.requestID = "req-1"
	execCtx.requestIDFromHeader = true
	execCtx.analyticsMetadata["request_headers"] = map[string]string{"host": "example.com"}

	assert.NotPanics(t, func() { server.writeCorrelationEntry(execCtx) })
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

func TestSnapshotHeaderPayload(t *testing.T) {
	t.Run("builds a payload from both keys", func(t *testing.T) {
		p := snapshotHeaderPayload(map[string]interface{}{
			"request_headers":  map[string]string{"host": "example.com"},
			"response_headers": map[string]string{"content-type": "application/json"},
			"source":           "immediate-response", // unrelated key, ignored
		})
		assert.Equal(t, map[string]string{"host": "example.com"}, p.RequestHeaders)
		assert.Equal(t, map[string]string{"content-type": "application/json"}, p.ResponseHeaders)
		assert.False(t, p.IsEmpty())
	})

	t.Run("empty when neither key present", func(t *testing.T) {
		p := snapshotHeaderPayload(map[string]interface{}{"source": "immediate-response"})
		assert.True(t, p.IsEmpty())
	})
}
