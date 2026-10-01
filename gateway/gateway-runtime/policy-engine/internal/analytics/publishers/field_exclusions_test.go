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
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// projectedReferenceLine renders the line the way Publish did before
// fieldExclusions existed: marshal the full event, then apply every exclusion
// through the JSON projection.
func projectedReferenceLine(t *testing.T, l *Log, event *dto.Event, exclude []string) []byte {
	t.Helper()
	ref := *l
	ref.exclusions = nil
	data, err := json.Marshal(ref.toTrafficLogEvent(event, ref.resolveGlobalDirective(event)))
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &m))
	applyFieldsProjection(m, &dto.TrafficLogFields{Exclude: exclude})
	out, err := json.Marshal(m)
	require.NoError(t, err)
	return out
}

func TestLog_Publish_StructExclusionsMatchJSONProjection(t *testing.T) {
	cases := map[string][]string{
		"customer config": {"api", "target", "client", "requestHeaders.:authority", "requestHeaders.:method",
			"requestHeaders.:path", "requestHeaders.:scheme", "responseHeaders.:status"},
		"header casing":           {"requestHeaders.X-Foo", "responseHeaders.SET-COOKIE"},
		"every request header":    {"requestHeaders.x-foo", "requestHeaders.authorization", "requestHeaders.:path"},
		"whole header maps":       {"requestHeaders", "responseHeaders"},
		"scalars and bodies":      {"component", "timestamp", "correlationId", "status", "requestBody", "responseBody"},
		"nested residual":         {"latencies.durationUs", "api.name", "operation"},
		"residual collapses":      {"target.statusCode", "target.destination"},
		"unknown and past leaves": {"nope", "requestHeaders.x-foo.deeper", "requestBody.x"},
		"mixed":                   {"api", "latencies.durationUs", "requestHeaders.authorization", "properties"},
	}
	for name, exclude := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := bothFlowsConfig()
			cfg.MaskedHeaders = []string{"authorization"}
			cfg.ExcludeFields = exclude
			l, err := NewLog(cfg)
			require.NoError(t, err)
			var buf bytes.Buffer
			useWriterSink(l, &buf)

			event := createBaseEvent()
			event.Properties["requestHeaders"] = `{":path":"/books","X-Foo":"bar","authorization":"Bearer t"}`
			event.Properties["responseHeaders"] = `{":status":"201","Set-Cookie":"a=b","content-type":"application/json"}`
			event.Properties["request_payload"] = "req-body"
			event.Properties["response_payload"] = "resp-body"

			want := projectedReferenceLine(t, l, event, exclude)
			l.Publish(event)
			assert.JSONEq(t, string(want), strings.TrimSpace(buf.String()))
		})
	}
}

func TestCompileFieldExclusions_SplitsStructAndResidual(t *testing.T) {
	fe := compileFieldExclusions([]string{"api", "requestHeaders.X-Foo", "responseHeaders.:status",
		"latencies.durationUs", "properties.claims.internal", "nope"})
	require.NotNil(t, fe)
	assert.Equal(t, map[string]bool{"api": true}, fe.topLevel)
	assert.Equal(t, map[string]bool{"x-foo": true}, fe.requestHeaders)
	assert.Equal(t, map[string]bool{":status": true}, fe.responseHeaders)
	require.NotNil(t, fe.residualFields())
	assert.Equal(t, []string{"latencies.durationUs", "properties.claims.internal", "nope"}, fe.residualFields().Exclude)

	assert.Nil(t, compileFieldExclusions(nil))
	assert.Nil(t, compileFieldExclusions([]string{"api"}).residualFields(), "no residual when every entry is struct-level")
}

func BenchmarkLog_Publish_CustomerExcludeFields(b *testing.B) {
	cfg := &config.TrafficLoggingConfig{
		Enabled: true, RequestHeaders: true, RequestBody: true, ResponseHeaders: true,
		MaskedHeaders: []string{"authorization", "x-api-key", "x-jwt-assertion"},
		ExcludeFields: []string{"api", "target", "client", "requestHeaders.:authority", "requestHeaders.:method",
			"requestHeaders.:path", "requestHeaders.:scheme", "responseHeaders.:status"},
	}
	l, err := NewLog(cfg)
	require.NoError(b, err)
	l.sinks = []Sink{newWriterSink(discard{}, "bench", nil)}
	event := createBaseEvent()
	event.Properties["requestHeaders"] = map[string]string{":authority": "api.wso2.com", ":method": "POST",
		":path": "/readinglistapi/1.0/books", ":scheme": "https", "content-type": "application/json",
		"user-agent": "Apache-HttpClient/4.5.14", "x-request-id": "45f34f0b-06bc-41fb-adc5-1fcbf26a2139",
		"x-forwarded-proto": "https", "content-length": "187"}
	event.Properties["responseHeaders"] = map[string]string{":status": "201", "content-type": "application/json",
		"server": "nginx", "date": "Thu, 01 Oct 2026 06:24:08 GMT", "content-length": "124"}
	event.Properties["request_payload"] = `{"uuid":"00f1e4a4-abb2-4be7-b390-5b05ea4d74fe","title":"The Lord of the Rings"}`
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Publish(event)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
