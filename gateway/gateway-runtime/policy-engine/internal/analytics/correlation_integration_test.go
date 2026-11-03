/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package analytics

import (
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// createLogEntryWithRequestID builds a minimal, valid HTTPAccessLogEntry carrying
// the given request id and no analytics_data metadata at all -- simulating either
// a normal request whose headers now travel via the correlation store instead of
// Envoy metadata (Step 4), or a request that genuinely never had an ext_proc
// stream (a no-route 404, a pre-filter rejection).
func createLogEntryWithRequestID(requestID string) *v3.HTTPAccessLogEntry {
	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{
			DownstreamRemoteAddress: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{Address: "192.168.1.1"},
				},
			},
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
			RequestId:     requestID,
		},
		Response: &v3.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
	}
}

// TestPrepareAnalyticEvent_CorrelationStoreHit covers the steady-state path: the
// ext_proc handler wrote captured headers to the store under this request's id,
// and prepareAnalyticEvent must use them directly (already typed, no JSON decode)
// instead of decoding anything from the (here, empty) access-log metadata.
func TestPrepareAnalyticEvent_CorrelationStoreHit(t *testing.T) {
	cfg := &config.Config{}
	a := NewAnalytics(cfg)

	store := correlation.NewStore(100, time.Minute, 4)
	putCompleted(store, "token-hit-1", correlation.Payload{
		RequestHeaders:  map[string]string{"host": "example.com"},
		ResponseHeaders: map[string]string{"content-type": "application/json"},
	})
	a.SetCorrelationStore(store)

	logEntry := createLogEntryWithToken(t, "req-hit-1", "token-hit-1")
	event := a.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	reqHeaders, ok := event.Properties[dto.PropKeyRequestHeaders].(map[string]string)
	require.True(t, ok, "expected a typed map[string]string from the store hit, got %T", event.Properties[dto.PropKeyRequestHeaders])
	assert.Equal(t, "example.com", reqHeaders["host"])

	respHeaders, ok := event.Properties[dto.PropKeyResponseHeaders].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "application/json", respHeaders["content-type"])
}

// TestPrepareAnalyticEvent_CorrelationStoreMiss_FallsBackToMetadata covers a
// store miss (TTL expiry, capacity eviction, or simply never written) for a
// request that DID have ext_proc-captured headers, stamped into the access-log
// metadata the old way. The line must still be emitted, with headers recovered
// from metadata instead of the store -- never dropped, never left blank when the
// data was actually available.
func TestPrepareAnalyticEvent_CorrelationStoreMiss_FallsBackToMetadata(t *testing.T) {
	cfg := &config.Config{}
	a := NewAnalytics(cfg)

	store := correlation.NewStore(100, time.Minute, 4)
	// Note: nothing is Put for "req-miss-1".
	a.SetCorrelationStore(store)

	logEntry := createLogEntryWithMetadata(map[string]string{
		RequestHeadersKey: `{"host":"example.com"}`,
	})
	logEntry.Request.RequestId = "req-miss-1"

	event := a.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	raw, ok := event.Properties[dto.PropKeyRequestHeaders].(string)
	require.True(t, ok, "expected the metadata-decode fallback (a JSON string), got %T", event.Properties[dto.PropKeyRequestHeaders])
	assert.Contains(t, raw, "example.com")
	// ALS-derived fields the fallback line must still carry.
	assert.Equal(t, 200, event.ProxyResponseCode)
}

// TestPrepareAnalyticEvent_NoCorrelationStore covers every existing caller
// (including every other test in this package) that never wires a store in at
// all -- SetCorrelationStore is never called, so correlationStore stays nil.
// Behavior must be identical to today: decode from metadata, never panic on a
// nil store.
func TestPrepareAnalyticEvent_NoCorrelationStore(t *testing.T) {
	cfg := &config.Config{}
	a := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		RequestHeadersKey: `{"host":"example.com"}`,
	})
	logEntry.Request.RequestId = "req-no-store"

	event := a.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	raw, ok := event.Properties[dto.PropKeyRequestHeaders].(string)
	require.True(t, ok)
	assert.Contains(t, raw, "example.com")
}

// TestPrepareAnalyticEvent_KeyedByTokenNotRequestID covers an access-log entry
// without a correlation token: the ext_proc side stored nothing for it, so the
// ALS side must miss even if some entry happens to be stored under its request id
// (a client-chosen x-request-id is not a safe key).
func TestPrepareAnalyticEvent_KeyedByTokenNotRequestID(t *testing.T) {
	a := NewAnalytics(&config.Config{})
	store := correlation.NewStore(100, time.Minute, 4)
	putCompleted(store, "req-1", correlation.Payload{RequestHeaders: map[string]string{"host": "should-never-be-used"}})
	a.SetCorrelationStore(store)

	event := a.prepareAnalyticEvent(createLogEntryWithRequestID("req-1"))

	require.NotNil(t, event)
	_, ok := event.Properties[dto.PropKeyRequestHeaders]
	assert.False(t, ok, "no token, so no store lookup")
}

// Two access-log entries with the same (client-supplied) request id but
// different tokens each get their own stream's fields.
func TestPrepareAnalyticEvent_SameRequestIDDifferentTokens(t *testing.T) {
	a := NewAnalytics(&config.Config{})
	store := correlation.NewStore(100, time.Minute, 4)
	putCompleted(store, "token-a", correlation.Payload{RequestHeaders: map[string]string{"who": "a"}})
	putCompleted(store, "token-b", correlation.Payload{RequestHeaders: map[string]string{"who": "b"}})
	a.SetCorrelationStore(store)

	eventB := a.prepareAnalyticEvent(createLogEntryWithToken(t, "dup", "token-b"))
	eventA := a.prepareAnalyticEvent(createLogEntryWithToken(t, "dup", "token-a"))

	assert.Equal(t, map[string]string{"who": "a"}, eventA.Properties[dto.PropKeyRequestHeaders])
	assert.Equal(t, map[string]string{"who": "b"}, eventB.Properties[dto.PropKeyRequestHeaders])
}

// TestPrepareAnalyticEvent_NoExtProcStream covers an access-log entry for a
// request that never had an ext_proc stream at all (a no-route 404, a
// pre-filter rejection): no store entry (nothing ever wrote one) and no
// analytics_data metadata (skipAllProcessing never populated any). The event
// must still be built from whatever the access-log entry itself carries --
// never dropped -- just without API/application/header enrichment.
func TestPrepareAnalyticEvent_NoExtProcStream(t *testing.T) {
	cfg := &config.Config{}
	a := NewAnalytics(cfg)
	a.SetCorrelationStore(correlation.NewStore(100, time.Minute, 4))

	logEntry := createLogEntryWithRequestID("req-404-1")
	event := a.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	assert.Equal(t, 200, event.ProxyResponseCode) // ALS-derived field always present
	_, ok := event.Properties[dto.PropKeyRequestHeaders]
	assert.False(t, ok, "no headers should be present when neither the store nor metadata has any")
}

// withAnalyticsData adds fields to the entry's analytics_data in its ext_proc
// filter metadata, as Envoy echoes it back in the access-log entry.
func withAnalyticsData(t *testing.T, entry *v3.HTTPAccessLogEntry, data map[string]any) *v3.HTTPAccessLogEntry {
	t.Helper()
	if entry.CommonProperties.Metadata == nil {
		entry.CommonProperties.Metadata = &corev3.Metadata{FilterMetadata: map[string]*structpb.Struct{
			constants.ExtProcFilterName: {Fields: map[string]*structpb.Value{
				"analytics_data": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{}}),
			}},
		}}
	}
	inner := entry.CommonProperties.Metadata.FilterMetadata[constants.ExtProcFilterName].Fields["analytics_data"].GetStructValue()
	for k, v := range data {
		val, err := structpb.NewValue(v)
		require.NoError(t, err)
		inner.Fields[k] = val
	}
	return entry
}

// createLogEntryWithToken is createLogEntryWithRequestID for a request whose
// ext_proc stream stored fields under token.
func createLogEntryWithToken(t *testing.T, requestID, token string) *v3.HTTPAccessLogEntry {
	t.Helper()
	return withAnalyticsData(t, createLogEntryWithRequestID(requestID), map[string]any{CorrelationTokenKey: token})
}

// A stored body is preferred over metadata, and the hit consumes the entry so its
// body is released as soon as the line is built.
func TestPrepareAnalyticEvent_StoredBodyUsedAndEntryTaken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Collector.RequestBody = true
	cfg.Collector.ResponseBody = true
	a := NewAnalytics(cfg)
	store := correlation.NewStoreWithBodyLimits(100, time.Minute, 1, 1024, 4096)
	putCompleted(store, "token-body-1", correlation.Payload{RequestBody: "from-store"})
	a.SetCorrelationStore(store)

	entry := withAnalyticsData(t, createLogEntryWithToken(t, "req-body-1", "token-body-1"),
		map[string]any{"response_payload": "large-from-metadata"})
	event := a.prepareAnalyticEvent(entry)

	assert.Equal(t, "from-store", event.Properties[dto.PropKeyRequestPayload])
	assert.Equal(t, "large-from-metadata", event.Properties[dto.PropKeyResponsePayload], "metadata still serves bodies the store did not take")
	_, stillThere := store.Take("token-body-1")
	assert.False(t, stillThere, "entry consumed by the ALS read")
}

// A stored empty header set means every header was filtered out in a later phase:
// the event carries no headers and does not fall back to whatever metadata holds.
func TestPrepareAnalyticEvent_StoredEmptyHeadersDoNotFallBackToMetadata(t *testing.T) {
	a := NewAnalytics(&config.Config{})
	store := correlation.NewStore(100, time.Minute, 1)
	putCompleted(store, "token-filtered", correlation.Payload{RequestHeaders: map[string]string{}})
	a.SetCorrelationStore(store)

	entry := withAnalyticsData(t, createLogEntryWithToken(t, "req-filtered", "token-filtered"),
		map[string]any{RequestHeadersKey: `{"authorization":"Bearer secret"}`})
	event := a.prepareAnalyticEvent(entry)

	assert.NotContains(t, event.Properties, dto.PropKeyRequestHeaders)
}

// putCompleted stores payload under key and marks its response finished, the way a
// request whose response ended is handed over.
func putCompleted(s *correlation.Store, key string, payload correlation.Payload) bool {
	if !s.Merge(key, payload) {
		return false
	}
	s.Complete(key)
	return true
}

// The token is read only from analytics_data: a same-named top-level ext_proc
// metadata key, which a policy can set, must not select another request's entry.
func TestPrepareAnalyticEvent_IgnoresTopLevelTokenKey(t *testing.T) {
	a := NewAnalytics(&config.Config{})
	store := correlation.NewStore(100, time.Minute, 1)
	putCompleted(store, "victim-token", correlation.Payload{RequestHeaders: map[string]string{"x-secret": "victim"}})
	a.SetCorrelationStore(store)

	entry := createLogEntryWithRequestID("attacker")
	entry = withAnalyticsData(t, entry, map[string]any{"source": "policy"})
	entry.CommonProperties.Metadata.FilterMetadata[constants.ExtProcFilterName].Fields[CorrelationTokenKey] =
		structpb.NewStringValue("victim-token")
	event := a.prepareAnalyticEvent(entry)

	assert.NotContains(t, event.Properties, dto.PropKeyRequestHeaders)
	assert.True(t, store.Has("victim-token"), "the victim's entry is untouched")
}

// Repeated values kept separate in the store reach the event unchanged.
func TestPrepareAnalyticEvent_StoredMultiValueHeaders(t *testing.T) {
	a := NewAnalytics(&config.Config{})
	store := correlation.NewStore(100, time.Minute, 1)
	putCompleted(store, "token-multi", correlation.Payload{RequestHeaders: map[string][]string{"x-multi": {"a", "b"}}})
	a.SetCorrelationStore(store)

	event := a.prepareAnalyticEvent(createLogEntryWithToken(t, "req-multi", "token-multi"))
	assert.Equal(t, map[string][]string{"x-multi": {"a", "b"}}, event.Properties[dto.PropKeyRequestHeaders])
}
