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
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// TestMain suppresses log output during benchmarks (Info/Warn lines from NewAnalytics /
// NewLog would otherwise print on every calibration run) so `go test -bench` output stays
// clean, matching the pattern in internal/executor/chain_bench_test.go and
// internal/kernel/extproc_bench_test.go. It also initializes the metrics registry: the
// correlation store (internal/analytics/correlation) records counters on every Put/Get,
// and the metric variables are nil until Init() is called.
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
	metrics.Enabled = true
	metrics.Init()
	os.Exit(m.Run())
}

// benchmarkHeadersJSON renders a realistic ~15-header set (like a real browser or JMeter
// request) as the JSON string the collector system policy's serializeHeaders would produce.
func benchmarkHeadersJSON(idPrefix string) string {
	headers := map[string]string{
		"host":             "api.example.com",
		"user-agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36",
		"accept":           "application/json, text/plain, */*",
		"accept-encoding":  "gzip, deflate, br",
		"accept-language":  "en-US,en;q=0.9",
		"connection":       "keep-alive",
		"content-type":     "application/json",
		"content-length":   "128",
		"authorization":    "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test-payload.test-signature",
		"x-api-key":        "test-api-key-not-a-secret",
		"x-jwt-assertion":  "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.signature",
		"x-forwarded-for":  "203.0.113.5",
		"x-request-id":     idPrefix + "-req-0001",
		"cache-control":    "no-cache",
		"x-correlation-id": idPrefix + "-corr-0001",
	}
	data, _ := json.Marshal(headers)
	return string(data)
}

// benchmarkLogEntry builds a realistic *v3.HTTPAccessLogEntry: the ext_proc filter metadata
// namespace carries an analytics_data struct (API/application/subscription/auth identity,
// ~15 request/response headers each, JSON-encoded exactly as the collector system policy
// emits them) alongside the latency timepoints and socket addresses Envoy itself supplies
// (see analytics.go's prepareAnalyticEvent, which reads from both).
func benchmarkLogEntry() *v3.HTTPAccessLogEntry {
	metadata := map[string]string{
		APITypeKey:                "RestApi",
		APIIDKey:                  "01977e3a-petstore-api-0001",
		APICreatorKey:             "admin",
		APINameKey:                "PetStoreAPI",
		APIVersionKey:             "v1",
		APICreatorTenantDomainKey: "carbon.super",
		APIOrganizationIDKey:      "org-bench-0001",
		APIContextKey:             "/petstore/v1",
		APIEnvironmentKey:         "Production",
		ProjectIDKey:              "project-bench-0001",
		AppIDKey:                  "app-bench-0001",
		AppKeyTypeKey:             "PRODUCTION",
		AppNameKey:                "DefaultApplication",
		AppOwnerKey:               "admin",
		RegionKey:                 "us-east-1",
		RequestHeadersKey:         benchmarkHeadersJSON("req"),
		ResponseHeadersKey:        benchmarkHeadersJSON("resp"),
		"response_content_type":   "application/json",

		dto.PropKeyAuthUserID:       "alice",
		dto.PropKeyAuthType:         "jwt",
		dto.PropKeyAuthIssuer:       "https://idp.example.com",
		dto.PropKeyAuthCredentialID: "client-bench-0001",
		dto.PropKeyAuthTokenID:      "jti-bench-0001",
		dto.PropKeyAuthAudience:     "aud1,aud2",
		dto.PropKeyAuthScopes:       "read write",
		dto.PropKeyAuthProperties:   `{"tenant":"acme"}`,
		dto.PropKeyAuthAuthorized:   "true",

		BillingCustomerIDKey:     "cust-bench-0001",
		BillingSubscriptionIDKey: "sub-bench-0001",
		SubscriptionStatusKey:    "ACTIVE",
		SubscriptionPlanNameKey:  "Gold",
	}
	fields := make(map[string]*structpb.Value, len(metadata))
	for k, v := range metadata {
		fields[k] = structpb.NewStringValue(v)
	}

	addr := func(ip string) *corev3.Address {
		return &corev3.Address{Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{Address: ip},
		}}
	}
	dur := func(nanos int32) *durationpb.Duration { return &durationpb.Duration{Nanos: nanos} }

	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{
			StartTime:                     timestamppb.Now(),
			TimeToLastRxByte:              dur(50_000_000),
			TimeToFirstUpstreamTxByte:     dur(60_000_000),
			TimeToLastUpstreamTxByte:      dur(65_000_000),
			TimeToFirstUpstreamRxByte:     dur(90_000_000),
			TimeToLastUpstreamRxByte:      dur(95_000_000),
			TimeToFirstDownstreamTxByte:   dur(96_000_000),
			TimeToLastDownstreamTxByte:    dur(100_000_000),
			DownstreamRemoteAddress:       addr("203.0.113.5"),
			DownstreamDirectRemoteAddress: addr("203.0.113.5"),
			DownstreamLocalAddress:        addr("10.0.0.10"),
			StreamId:                      "stream-bench-0001",
			Metadata: &corev3.Metadata{
				FilterMetadata: map[string]*structpb.Struct{
					constants.ExtProcFilterName: {
						Fields: map[string]*structpb.Value{
							"analytics_data": structpb.NewStructValue(&structpb.Struct{Fields: fields}),
						},
					},
				},
			},
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod:    corev3.RequestMethod_GET,
			Authority:        "api.example.com",
			Path:             "/petstore/v1/pets/123",
			OriginalPath:     "/pets/{petId}",
			UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			RequestId:        "req-bench-0001",
			RequestBodyBytes: 128,
		},
		Response: &v3.HTTPResponseProperties{
			ResponseCode:        wrapperspb.UInt32(200),
			ResponseCodeDetails: UpstreamSuccessResponseDetail,
			ResponseBodyBytes:   512,
			ResponseHeaders:     map[string]string{"content-type": "application/json"},
		},
	}
}

// benchmarkTrafficLoggingConfig mirrors a representative production traffic-logging setup:
// masked_headers redacting the three most sensitive headers, and global properties mixing a
// literal with two "$ctx:" expressions -- one over API identity, one over the response, and
// crucially NONE over request.header/response.header, which is what lets Task 2's CEL
// variable-binding optimization skip header parsing entirely for this config. Output goes to
// a temp file (a real Sink implementation, exercised end-to-end) that nothing reads.
func benchmarkTrafficLoggingConfig(b *testing.B) *config.Config {
	b.Helper()
	return &config.Config{
		TrafficLogging: config.TrafficLoggingConfig{
			Enabled:         true,
			Outputs:         []string{"file"},
			File:            config.TrafficLogFileConfig{Path: filepath.Join(b.TempDir(), "bench-traffic.log")},
			RequestHeaders:  true,
			ResponseHeaders: true,
			MaskedHeaders:   []string{"authorization", "x-api-key", "x-jwt-assertion"},
			Properties: map[string]string{
				"env":     "prod",
				"apiName": "$ctx:api.name",
				"status":  "$ctx:response.status",
			},
		},
	}
}

// BenchmarkAnalytics_Process measures the full ALS-entry-to-published-event pipeline
// (prepareAnalyticEvent + publisher fan-out, i.e. the traffic-logging Log publisher's
// Publish) with traffic logging configured as described above. This is what Task 0
// baselines before, and Tasks 1-2 are measured against after, the CPU-reduction changes.
func BenchmarkAnalytics_Process(b *testing.B) {
	cfg := benchmarkTrafficLoggingConfig(b)
	a := NewAnalytics(cfg)
	entry := benchmarkLogEntry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Process(entry)
	}
}

// benchmarkLogEntryNoHeaderMetadata is identical to benchmarkLogEntry except the
// access-log entry's analytics_data metadata carries no request_headers/
// response_headers keys at all -- the real production shape once Step 4 lands
// (see buildAnalyticsStruct's doc comment in internal/kernel/analytics.go): those
// two keys are no longer sent to Envoy, so they're no longer echoed back here
// either. Every other metadata field is unchanged, unaffected by Step 4.
func benchmarkLogEntryNoHeaderMetadata() *v3.HTTPAccessLogEntry {
	entry := benchmarkLogEntry()
	analyticsData := entry.CommonProperties.Metadata.FilterMetadata[constants.ExtProcFilterName].
		Fields["analytics_data"].GetStructValue()
	delete(analyticsData.Fields, RequestHeadersKey)
	delete(analyticsData.Fields, ResponseHeadersKey)
	return entry
}

// BenchmarkAnalytics_Process_CorrelationStoreHit measures the real Step-4
// steady-state pipeline: the ext_proc handler already wrote captured headers to
// the correlation store (see internal/analytics/correlation) under this
// request's id, so prepareAnalyticEvent picks them up directly -- no JSON decode,
// no oversized metadata struct to walk -- instead of decoding
// RequestHeadersKey/ResponseHeadersKey out of the access-log entry's metadata as
// BenchmarkAnalytics_Process (the pre-Step-4 shape, still exercised above for a
// direct before/after comparison of everything Step 4 did NOT touch) does. Diff
// this against BenchmarkAnalytics_Process to see Step 4's isolated effect.
func BenchmarkAnalytics_Process_CorrelationStoreHit(b *testing.B) {
	cfg := benchmarkTrafficLoggingConfig(b)
	a := NewAnalytics(cfg)
	store := correlation.NewStore(1000, time.Minute, 16)
	putCompleted(store, "req-bench-0001", correlation.Payload{
		RequestHeaders:  headersFromJSON(b, benchmarkHeadersJSON("req")),
		ResponseHeaders: headersFromJSON(b, benchmarkHeadersJSON("resp")),
	})
	a.SetCorrelationStore(store)
	entry := benchmarkLogEntryNoHeaderMetadata()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Process(entry)
	}
}

// headersFromJSON decodes a JSON header blob (as produced by benchmarkHeadersJSON)
// into the map[string]string shape correlation.Payload carries, so the store-hit
// benchmark seeds the store with the same logical header content as the
// pre-Step-4 benchmark's metadata carries.
func headersFromJSON(b *testing.B, raw string) map[string]string {
	b.Helper()
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		b.Fatalf("headersFromJSON: %v", err)
	}
	return m
}
