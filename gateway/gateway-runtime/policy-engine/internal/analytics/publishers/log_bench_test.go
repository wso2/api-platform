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

package publishers

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// benchmarkHeadersJSON renders a realistic ~15-header set (like a real browser or JMeter
// request), matching the JSON string shape the collector system policy's serializeHeaders
// produces and Log.Publish parses back via parseHeadersFromString.
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

// benchmarkTrafficLogEvent builds a realistic *dto.Event -- the shape Analytics.Process hands
// each publisher -- carrying ~15 request/response headers each (JSON-encoded, as captured),
// application/subscription/auth identity, and generic metadata.
func benchmarkTrafficLogEvent() *dto.Event {
	event := createBaseEvent()
	event.Properties[dto.PropKeyRequestHeaders] = benchmarkHeadersJSON("req")
	event.Properties[dto.PropKeyResponseHeaders] = benchmarkHeadersJSON("resp")
	event.Properties[dto.PropKeyAuthUserID] = "alice"
	event.Properties[dto.PropKeyAuthType] = "jwt"
	event.Properties[dto.PropKeyAuthIssuer] = "https://idp.example.com"
	event.Properties[dto.PropKeyAuthCredentialID] = "client-bench-0001"
	event.Properties[dto.PropKeyAuthTokenID] = "jti-bench-0001"
	event.Properties[dto.PropKeyAuthAudience] = "aud1,aud2"
	event.Properties[dto.PropKeyAuthScopes] = "read write"
	event.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme"}`
	event.Properties[dto.PropKeyAuthAuthorized] = "true"
	event.Properties["userName"] = "alice"
	event.Properties["commonName"] = "N/A"
	event.Properties["apiContext"] = "/petstore/v1"
	event.Properties["responseContentType"] = "application/json"
	event.Properties["requestSize"] = uint64(128)
	event.Properties["responseSize"] = uint64(512)
	event.TrafficLogLatencies = &dto.TrafficLogLatencies{
		DurationUs:                 100_000,
		RequestMediationLatencyUs:  10_000,
		BackendLatencyUs:           25_000,
		ResponseMediationLatencyUs: 1_000,
	}
	return event
}

// benchmarkLogPublisher builds a Log publisher configured the way a representative
// production deployment would: masked_headers redacting the three most sensitive headers,
// and global properties mixing a literal with two "$ctx:" expressions over API identity and
// the response -- crucially NONE over request.header/response.header, which is what lets
// Task 2's CEL variable-binding optimization skip header parsing for this config. Output is
// redirected to io.Discard via useWriterSink (this package's existing test helper) instead of
// a real sink, so the benchmark measures Publish's own cost, not sink I/O.
func benchmarkLogPublisher(b *testing.B) *Log {
	b.Helper()
	cfg := &config.TrafficLoggingConfig{
		Enabled:         true,
		RequestHeaders:  true,
		ResponseHeaders: true,
		MaskedHeaders:   []string{"authorization", "x-api-key", "x-jwt-assertion"},
		Properties: map[string]string{
			"env":     "prod",
			"apiName": "$ctx:api.name",
			"status":  "$ctx:response.status",
		},
	}
	l, err := NewLog(cfg)
	if err != nil {
		b.Fatalf("NewLog: %v", err)
	}
	useWriterSink(l, io.Discard)
	return l
}

// BenchmarkLog_Publish measures Log.Publish (directive resolution, header re-parse/mask,
// JSON marshal, field projection, and the write itself) over a realistic event shape. This is
// what Task 0 baselines before, and Task 2's CEL variable-binding change is measured against
// after.
func BenchmarkLog_Publish(b *testing.B) {
	l := benchmarkLogPublisher(b)
	event := benchmarkTrafficLogEvent()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Publish(event)
	}
}

// benchmarkTrafficLogEventTypedHeaders is identical to benchmarkTrafficLogEvent
// except the header properties are already-typed map[string]string values (the
// real shape a correlation-store hit hands prepareAnalyticEvent -- see
// internal/analytics/correlation and internal/analytics's prepareAnalyticEvent)
// instead of the JSON-encoded strings the pre-Step-4 (and store-miss fallback)
// path produced. headersFromEventProperty accepts both; this measures the typed
// path, which skips parseHeadersFromString's json.Unmarshal entirely.
func benchmarkTrafficLogEventTypedHeaders() *dto.Event {
	event := benchmarkTrafficLogEvent()
	event.Properties[dto.PropKeyRequestHeaders] = mustDecodeHeadersJSON(benchmarkHeadersJSON("req"))
	event.Properties[dto.PropKeyResponseHeaders] = mustDecodeHeadersJSON(benchmarkHeadersJSON("resp"))
	return event
}

func mustDecodeHeadersJSON(raw string) map[string]string {
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		panic(err)
	}
	return m
}

// BenchmarkLog_Publish_TypedHeaders measures the same pipeline as
// BenchmarkLog_Publish but with the Step-4 correlation-store-hit header shape
// (map[string]string, no JSON decode needed) instead of the pre-Step-4 JSON
// string shape. Diff this against BenchmarkLog_Publish to see Step 4's isolated
// effect on the Log publisher.
func BenchmarkLog_Publish_TypedHeaders(b *testing.B) {
	l := benchmarkLogPublisher(b)
	event := benchmarkTrafficLogEventTypedHeaders()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Publish(event)
	}
}
