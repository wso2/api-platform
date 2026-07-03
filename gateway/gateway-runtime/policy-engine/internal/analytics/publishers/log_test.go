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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// newLogToFile builds a Log publisher that writes to a temp file, returning the
// publisher and a function that reads back what was written.
func newLogToFile(t *testing.T, cfg *config.TrafficLoggingConfig) (*Log, func() string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.log")
	f, err := os.Create(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	l := NewLog(cfg)
	l.out = f
	return l, func() string {
		require.NoError(t, f.Sync())
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
}

// bothFlows returns a directive that opts in to logging both request and response
// headers and payloads — the "log everything captured" case.
func bothFlows() *dto.TrafficLogDirective {
	return &dto.TrafficLogDirective{
		Request:  &dto.TrafficLogFlow{Payload: true, Headers: true},
		Response: &dto.TrafficLogFlow{Payload: true, Headers: true},
	}
}

// decodeProps runs the single JSON line and returns the decoded event + its properties.
func decodeLine(t *testing.T, out string) (map[string]interface{}, map[string]interface{}) {
	t.Helper()
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	props, _ := decoded["properties"].(map[string]interface{})
	return decoded, props
}

func TestNewLog_NilConfig(t *testing.T) {
	l := NewLog(nil)
	require.NotNil(t, l)
	assert.Empty(t, l.maskedHeaders)
}

// Per-API gating: an event without a traffic-log directive is never emitted.
func TestLog_Publish_SkipsWhenNoDirective(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent() // no TrafficLog set
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	assert.Empty(t, read(), "event without a traffic-log directive must not be logged")
}

func TestLog_Publish_WritesJSONLineWithLatencies(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	out := read()
	// Single line (one trailing newline).
	assert.Equal(t, 1, strings.Count(strings.TrimRight(out, "\n"), "\n")+1)

	decoded, props := decodeLine(t, out)
	api := decoded["api"].(map[string]interface{})
	assert.Equal(t, "test-api", api["apiName"])
	assert.Equal(t, `{"x-foo":"bar"}`, props["requestHeaders"])

	// ALS-derived latencies are always present in the line — the key improvement
	// over the inline log-message policy, which could never see them.
	latencies := decoded["latencies"].(map[string]interface{})
	assert.Equal(t, float64(100), latencies["responseLatency"])
}

// Custom properties from the directive are emitted under properties.custom.
func TestLog_Publish_CustomPropertiesUnderCustom(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: true},
		Properties: map[string]interface{}{
			"who":        "alice",
			"authType":   "jwt",
			"retryCount": float64(3),
		},
	}
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	_, props := decodeLine(t, read())
	custom, ok := props["custom"].(map[string]interface{})
	require.True(t, ok, "expected properties.custom object, got %v", props["custom"])
	assert.Equal(t, "alice", custom["who"])
	assert.Equal(t, "jwt", custom["authType"])
	assert.Equal(t, float64(3), custom["retryCount"])
	// Reserved keys are untouched by the custom namespace.
	assert.Equal(t, `{"x-foo":"bar"}`, props["requestHeaders"])
}

// A directive with no Properties emits no custom key.
func TestLog_Publish_NoCustomWhenAbsent(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()

	l.Publish(event)

	_, props := decodeLine(t, read())
	_, present := props["custom"]
	assert.False(t, present, "no custom key expected when directive has no Properties")
}

// The fields projection can select properties.custom like any other property path.
func TestLog_Publish_CustomProjectableViaFields(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Properties: map[string]interface{}{"who": "alice"},
		Fields:     &dto.TrafficLogFields{Mode: "include", Names: []string{"properties.custom"}},
	}
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	_, props := decodeLine(t, read())
	custom, ok := props["custom"].(map[string]interface{})
	require.True(t, ok, "expected properties.custom retained by include projection")
	assert.Equal(t, "alice", custom["who"])
	// Non-selected property dropped by the include projection.
	_, hasHeaders := props["requestHeaders"]
	assert.False(t, hasHeaders, "requestHeaders should be dropped by include projection")
}

func TestLog_Publish_MasksHeaders(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"Authorization"}})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer secret","x-foo":"bar"}`
	event.Properties["responseHeaders"] = `{"authorization":"Bearer secret2"}`

	l.Publish(event)

	_, props := decodeLine(t, read())

	var reqH map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(props["requestHeaders"].(string)), &reqH))
	assert.Equal(t, "****", reqH["Authorization"]) // masked
	assert.Equal(t, "bar", reqH["x-foo"])          // untouched

	var resH map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(props["responseHeaders"].(string)), &resH))
	assert.Equal(t, "****", resH["authorization"]) // case-insensitive match
}

// Per-API excludeHeaders drops the header entirely (vs global masking which redacts).
func TestLog_Publish_ExcludeHeadersDrops(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: true, ExcludeHeaders: []string{"X-Secret"}},
	}
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Secret":"top","x-foo":"bar"}`

	l.Publish(event)

	_, props := decodeLine(t, read())
	var reqH map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(props["requestHeaders"].(string)), &reqH))

	_, hasSecret := reqH["X-Secret"]
	assert.False(t, hasSecret, "excluded header must be dropped")
	assert.Equal(t, "****", reqH["Authorization"], "masked header still redacted")
	assert.Equal(t, "bar", reqH["x-foo"])
}

// headers:false omits the headers property; a nil flow omits its whole side.
func TestLog_Publish_DisabledFieldsOmitted(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: false, Payload: true},
		// Response flow nil -> both response props dropped.
	}
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "req-body"
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.Properties["response_payload"] = "resp-body"

	l.Publish(event)

	_, props := decodeLine(t, read())
	_, hasReqHeaders := props["requestHeaders"]
	assert.False(t, hasReqHeaders, "request headers disabled -> omitted")
	assert.Equal(t, "req-body", props["request_payload"], "request payload enabled -> kept")

	_, hasRespHeaders := props["responseHeaders"]
	_, hasRespPayload := props["response_payload"]
	assert.False(t, hasRespHeaders, "nil response flow -> response headers omitted")
	assert.False(t, hasRespPayload, "nil response flow -> response payload omitted")
}

func TestLog_Publish_DoesNotMutateSharedEvent(t *testing.T) {
	l, _ := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	original := `{"authorization":"Bearer secret"}`
	event.Properties["requestHeaders"] = original

	l.Publish(event)

	// The shared event (read by other publishers) must be untouched.
	assert.Equal(t, original, event.Properties["requestHeaders"])
}

func TestLog_Publish_NilEvent(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	assert.NotPanics(t, func() { l.Publish(nil) })
	assert.Empty(t, read())
}

// Field selection (include): only named top-level keys and properties.* survive,
// and it is authoritative over presence (request.headers boolean is ignored, but
// excludeHeaders + masking still apply).
func TestLog_Publish_FieldsInclude(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Drop":"d","X-Keep":"k"}`
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: false, ExcludeHeaders: []string{"X-Drop"}}, // boolean ignored
		Fields:  &dto.TrafficLogFields{Mode: "include", Names: []string{"latencies", "properties.requestHeaders"}},
	}

	l.Publish(event)
	decoded, props := decodeLine(t, read())

	_, hasAPI := decoded["api"]
	_, hasOp := decoded["operation"]
	assert.False(t, hasAPI, "api not listed -> dropped")
	assert.False(t, hasOp, "operation not listed -> dropped")
	assert.Contains(t, decoded, "latencies", "latencies listed -> kept")
	require.NotNil(t, props, "properties kept (a properties.* path was listed)")

	_, hasResp := props["responseHeaders"]
	assert.False(t, hasResp, "responseHeaders not listed -> dropped")

	reqRaw, ok := props["requestHeaders"].(string)
	require.True(t, ok, "requestHeaders present (fields authoritative, boolean ignored)")
	var reqH map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(reqRaw), &reqH))
	assert.Equal(t, "****", reqH["Authorization"], "masking still applies")
	_, hasDrop := reqH["X-Drop"]
	assert.False(t, hasDrop, "excludeHeaders still applies")
	assert.Equal(t, "k", reqH["X-Keep"])
}

// Field selection (exclude): named keys are dropped, everything else remains.
func TestLog_Publish_FieldsExclude(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "secret-body"
	event.TrafficLog = &dto.TrafficLogDirective{
		Fields: &dto.TrafficLogFields{Mode: "exclude", Names: []string{"operation", "properties.request_payload"}},
	}

	l.Publish(event)
	decoded, props := decodeLine(t, read())

	_, hasOp := decoded["operation"]
	assert.False(t, hasOp, "operation excluded")
	assert.Contains(t, decoded, "api", "api kept")
	assert.Contains(t, decoded, "latencies", "latencies kept")
	require.NotNil(t, props)
	_, hasPayload := props["request_payload"]
	assert.False(t, hasPayload, "request_payload excluded")
	assert.Contains(t, props, "requestHeaders", "requestHeaders kept (not excluded)")
}

// Naming the whole "properties" key keeps all of its subkeys.
func TestLog_Publish_FieldsIncludeWholeProperties(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.TrafficLog = &dto.TrafficLogDirective{
		Fields: &dto.TrafficLogFields{Mode: "include", Names: []string{"properties"}},
	}

	l.Publish(event)
	decoded, props := decodeLine(t, read())

	_, hasAPI := decoded["api"]
	assert.False(t, hasAPI, "api not listed -> dropped")
	require.NotNil(t, props)
	assert.Contains(t, props, "requestHeaders")
	assert.Contains(t, props, "responseHeaders")
}

// Output-side payload truncation (0 = no limit).
func TestLog_Publish_TruncatesPayload(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaxPayloadSize: 5})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["request_payload"] = "hello world"
	event.Properties["response_payload"] = "goodbye world"

	l.Publish(event)
	_, props := decodeLine(t, read())
	assert.Equal(t, "hello", props["request_payload"])
	assert.Equal(t, "goodb", props["response_payload"])
}

func TestLog_Publish_NoTruncationWhenZero(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaxPayloadSize: 0})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["request_payload"] = "hello world"

	l.Publish(event)
	_, props := decodeLine(t, read())
	assert.Equal(t, "hello world", props["request_payload"])
}

func TestLog_Publish_InvalidHeaderJSONLeftAsIs(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["requestHeaders"] = "not-json"

	l.Publish(event)

	_, props := decodeLine(t, read())
	assert.Equal(t, "not-json", props["requestHeaders"])
}
