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
	store.Put("req-hit-1", correlation.Payload{
		RequestHeaders:  map[string]string{"host": "example.com"},
		ResponseHeaders: map[string]string{"content-type": "application/json"},
	})
	a.SetCorrelationStore(store)

	logEntry := createLogEntryWithRequestID("req-hit-1")
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

// TestPrepareAnalyticEvent_NoXRequestID covers a request whose ext_proc side
// never saw an x-request-id header at all: buildRequestContexts falls back to a
// generated uuid there, which correlatesInProcess (internal/kernel/analytics.go)
// never writes to the store under -- so the ALS side must never get a spurious
// hit either. Here that's modeled directly: the access-log entry itself carries
// no RequestId (empty string), which lookupCorrelationPayload must treat as an
// automatic miss without even querying the store.
func TestPrepareAnalyticEvent_NoXRequestID(t *testing.T) {
	cfg := &config.Config{}
	a := NewAnalytics(cfg)

	store := correlation.NewStore(100, time.Minute, 4)
	// Simulate a store that (incorrectly, hypothetically) held an entry under the
	// empty key -- lookupCorrelationPayload must still refuse to match it, since
	// Merge itself never allows this in production (see correlatesInProcess's
	// requestIDFromHeader gate).
	store.Put("", correlation.Payload{RequestHeaders: map[string]string{"host": "should-never-be-used"}})
	a.SetCorrelationStore(store)

	logEntry := createLogEntryWithMetadata(nil) // no analytics_data metadata either
	logEntry.Request.RequestId = ""

	event := a.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	_, ok := event.Properties[dto.PropKeyRequestHeaders]
	assert.False(t, ok, "a request with no x-request-id must never surface a spurious store hit")
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

// withAnalyticsData attaches an analytics_data struct to the entry's ext_proc
// filter metadata, as Envoy echoes it back in the access-log entry.
func withAnalyticsData(t *testing.T, entry *v3.HTTPAccessLogEntry, data map[string]any) *v3.HTTPAccessLogEntry {
	t.Helper()
	inner, err := structpb.NewStruct(data)
	require.NoError(t, err)
	entry.CommonProperties.Metadata = &corev3.Metadata{FilterMetadata: map[string]*structpb.Struct{
		constants.ExtProcFilterName: {Fields: map[string]*structpb.Value{"analytics_data": structpb.NewStructValue(inner)}},
	}}
	return entry
}

// A stored body is preferred over metadata, and the hit consumes the entry so its
// body is released as soon as the line is built.
func TestPrepareAnalyticEvent_StoredBodyUsedAndEntryTaken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Collector.RequestBody = true
	cfg.Collector.ResponseBody = true
	a := NewAnalytics(cfg)
	store := correlation.NewStoreWithBodyLimits(100, time.Minute, 1, 1024, 4096)
	store.Put("req-body-1", correlation.Payload{RequestBody: "from-store"})
	a.SetCorrelationStore(store)

	entry := withAnalyticsData(t, createLogEntryWithRequestID("req-body-1"),
		map[string]any{"response_payload": "large-from-metadata"})
	event := a.prepareAnalyticEvent(entry)

	assert.Equal(t, "from-store", event.Properties[dto.PropKeyRequestPayload])
	assert.Equal(t, "large-from-metadata", event.Properties[dto.PropKeyResponsePayload], "metadata still serves bodies the store did not take")
	_, stillThere := store.Get("req-body-1")
	assert.False(t, stillThere, "entry consumed by the ALS read")
}

// The LLM proxy's internal loopback hop can share the outer call's request id;
// it must only peek so the outer call's own line still finds the entry.
func TestPrepareAnalyticEvent_LoopbackHopDoesNotConsumeEntry(t *testing.T) {
	a := NewAnalytics(&config.Config{})
	store := correlation.NewStore(100, time.Minute, 1)
	store.Put("req-shared", correlation.Payload{RequestHeaders: map[string]string{"h": "v"}})
	a.SetCorrelationStore(store)

	entry := withAnalyticsData(t, createLogEntryWithRequestID("req-shared"),
		map[string]any{InternalLoopbackMetadataKey: "true"})
	a.prepareAnalyticEvent(entry)

	_, stillThere := store.Get("req-shared")
	assert.True(t, stillThere)
}
