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

// decodeLine parses the single JSON line emitted by the Log publisher.
func decodeLine(t *testing.T, out string) map[string]interface{} {
	t.Helper()
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	return decoded
}

// headerMap returns the JSON-decoded header object as map[string]interface{} for
// easy key/value assertions. Fails the test if v is not the expected type.
func headerMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	m, ok := v.(map[string]interface{})
	require.True(t, ok, "expected header object (map[string]interface{}), got %T", v)
	return m
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

	decoded := decodeLine(t, out)
	api := decoded["api"].(map[string]interface{})
	assert.Equal(t, "test-api", api["name"])
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "bar", reqH["x-foo"])

	// ALS-derived latencies are always present in the line — the key improvement
	// over the inline log-message policy, which could never see them. The traffic
	// log carries microsecond-precision timings, separate from Moesif's ms fields.
	latencies := decoded["latencies"].(map[string]interface{})
	assert.Equal(t, float64(250000), latencies["durationUs"])
}

// Properties from the directive are emitted as a top-level "properties" object.
func TestLog_Publish_PropertiesTopLevel(t *testing.T) {
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

	decoded := decodeLine(t, read())
	props, ok := decoded["properties"].(map[string]interface{})
	require.True(t, ok, "expected top-level properties object, got %T", decoded["properties"])
	assert.Equal(t, "alice", props["who"])
	assert.Equal(t, "jwt", props["authType"])
	assert.Equal(t, float64(3), props["retryCount"])
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "bar", reqH["x-foo"])
}

// A directive with no properties emits no "properties" key.
func TestLog_Publish_NoPropertiesWhenAbsent(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, present := decoded["properties"]
	assert.False(t, present, "no properties key expected when directive has no properties")
}

// The fields projection can select "properties" like any other top-level key.
func TestLog_Publish_PropertiesProjectableViaFields(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Properties: map[string]interface{}{"who": "alice"},
		Fields:     &dto.TrafficLogFields{Only: []string{"properties"}},
	}
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	props, ok := decoded["properties"].(map[string]interface{})
	require.True(t, ok, "expected properties retained by include projection")
	assert.Equal(t, "alice", props["who"])
	_, hasHeaders := decoded["requestHeaders"]
	assert.False(t, hasHeaders, "requestHeaders not in Only list -> dropped")
}

func TestLog_Publish_MasksHeaders(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"Authorization"}})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer secret","x-foo":"bar"}`
	event.Properties["responseHeaders"] = `{"authorization":"Bearer secret2"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "****", reqH["Authorization"]) // masked
	assert.Equal(t, "bar", reqH["x-foo"])          // untouched

	resH := headerMap(t, decoded["responseHeaders"])
	assert.Equal(t, "****", resH["authorization"]) // case-insensitive match
}

// Per-API maskedHeaders are merged with the global config mask; either source alone redacts.
func TestLog_Publish_PerAPIMaskedHeadersMergedWithGlobal(t *testing.T) {
	// Global config masks "authorization"; per-API directive adds "x-secret".
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request:       &dto.TrafficLogFlow{Headers: true},
		MaskedHeaders: []string{"x-secret"},
	}
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Secret":"top","x-foo":"bar"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "****", reqH["Authorization"], "global masked header still redacted")
	assert.Equal(t, "****", reqH["X-Secret"], "per-API masked header redacted")
	assert.Equal(t, "bar", reqH["x-foo"], "unmasked header unchanged")
}

// Per-API maskedHeaders work even when no global headers are configured.
func TestLog_Publish_PerAPIMaskedHeadersNoGlobal(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request:       &dto.TrafficLogFlow{Headers: true},
		MaskedHeaders: []string{"X-Token"},
	}
	event.Properties["requestHeaders"] = `{"X-Token":"secret","x-foo":"bar"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "****", reqH["X-Token"])
	assert.Equal(t, "bar", reqH["x-foo"])
}

// fields.exclude with a dotted path drops a specific header entirely (vs global masking which redacts).
func TestLog_Publish_ExcludeHeadersDrops(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: true},
		Fields:  &dto.TrafficLogFields{Exclude: []string{"requestHeaders.X-Secret"}},
	}
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Secret":"top","x-foo":"bar"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])

	_, hasSecret := reqH["X-Secret"]
	assert.False(t, hasSecret, "excluded header must be dropped")
	assert.Equal(t, "****", reqH["Authorization"], "masked header still redacted")
	assert.Equal(t, "bar", reqH["x-foo"])
}

// Per-flow excludeHeaders drops a header entirely (case-insensitive), in both the
// request and response flows, while masking still redacts other headers. This is
// the traffic-logging counterpart of the inline excludeHeaders param.
func TestLog_Publish_PerFlowExcludeHeadersDrops(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		// Directive carries lower-cased names (as the policy stamps them); the
		// captured header keys use mixed case to prove case-insensitive matching.
		Request:  &dto.TrafficLogFlow{Headers: true, ExcludeHeaders: []string{"x-secret"}},
		Response: &dto.TrafficLogFlow{Headers: true, ExcludeHeaders: []string{"set-cookie"}},
	}
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Secret":"top","x-foo":"bar"}`
	event.Properties["responseHeaders"] = `{"Set-Cookie":"sid=1","x-bar":"baz"}`

	l.Publish(event)

	decoded := decodeLine(t, read())

	reqH := headerMap(t, decoded["requestHeaders"])
	_, hasSecret := reqH["X-Secret"]
	assert.False(t, hasSecret, "excluded request header must be dropped entirely")
	assert.Equal(t, "****", reqH["Authorization"], "masked header still redacted")
	assert.Equal(t, "bar", reqH["x-foo"], "untouched header retained")

	resH := headerMap(t, decoded["responseHeaders"])
	_, hasCookie := resH["Set-Cookie"]
	assert.False(t, hasCookie, "excluded response header must be dropped entirely")
	assert.Equal(t, "baz", resH["x-bar"], "untouched response header retained")
}

// Per-flow excludeHeaders must not mutate the shared event, so other publishers
// (e.g. Moesif) still see the full captured header set.
func TestLog_Publish_PerFlowExcludeHeadersDoesNotMutateSharedEvent(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: true, ExcludeHeaders: []string{"x-secret"}},
	}
	const raw = `{"X-Secret":"top","x-foo":"bar"}`
	event.Properties["requestHeaders"] = raw

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])
	_, hasSecret := reqH["X-Secret"]
	assert.False(t, hasSecret, "header dropped from the emitted line")
	assert.Equal(t, raw, event.Properties["requestHeaders"], "shared event.Properties must be untouched")
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

	decoded := decodeLine(t, read())
	_, hasReqHeaders := decoded["requestHeaders"]
	assert.False(t, hasReqHeaders, "request headers disabled -> omitted")
	assert.Equal(t, "req-body", decoded["requestBody"], "request payload enabled -> kept")

	_, hasRespHeaders := decoded["responseHeaders"]
	_, hasRespPayload := decoded["responseBody"]
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

// Field selection (include): only named top-level keys survive; fields.only is
// authoritative over presence (request.headers boolean is ignored, masking still applies).
func TestLog_Publish_FieldsInclude(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Keep":"k"}`
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.TrafficLog = &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{Headers: false}, // boolean ignored when fields set
		Fields:  &dto.TrafficLogFields{Only: []string{"latencies", "requestHeaders"}},
	}

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasAPI := decoded["api"]
	_, hasOp := decoded["operation"]
	assert.False(t, hasAPI, "api not listed -> dropped")
	assert.False(t, hasOp, "operation not listed -> dropped")
	assert.Contains(t, decoded, "latencies", "latencies listed -> kept")

	_, hasResp := decoded["responseHeaders"]
	assert.False(t, hasResp, "responseHeaders not listed -> dropped")

	require.NotNil(t, decoded["requestHeaders"], "requestHeaders present (fields authoritative, boolean ignored)")
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "****", reqH["Authorization"], "masking still applies")
	assert.Equal(t, "k", reqH["X-Keep"])
}

// Field selection (exclude): named keys are dropped, everything else remains.
func TestLog_Publish_FieldsExclude(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "secret-body"
	event.TrafficLog = &dto.TrafficLogDirective{
		Fields: &dto.TrafficLogFields{Exclude: []string{"operation", "requestBody"}},
	}

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasOp := decoded["operation"]
	assert.False(t, hasOp, "operation excluded")
	assert.Contains(t, decoded, "api", "api kept")
	assert.Contains(t, decoded, "latencies", "latencies kept")
	_, hasPayload := decoded["requestBody"]
	assert.False(t, hasPayload, "requestBody excluded")
	assert.Contains(t, decoded, "requestHeaders", "requestHeaders kept (not excluded)")
}

// An unrelated fields.exclude entry (dropping one header sub-key) must not defeat
// an explicitly configured flow's payload:false/headers:true booleans. Regression
// test for a bug where any fields.exclude entry made the per-flow booleans
// globally inert, silently re-enabling payload logging that request.payload:false
// was supposed to suppress.
func TestLog_Publish_FieldsExcludeDoesNotOverrideExplicitFlowBooleans(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Cookie":"sid=1","x-foo":"bar"}`
	event.Properties["request_payload"] = "secret-request-body"
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.Properties["response_payload"] = "secret-response-body"
	event.TrafficLog = &dto.TrafficLogDirective{
		Request:  &dto.TrafficLogFlow{Headers: true, Payload: false},
		Response: &dto.TrafficLogFlow{Headers: true, Payload: false},
		Fields:   &dto.TrafficLogFields{Exclude: []string{"requestHeaders.Cookie"}},
	}

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasReqBody := decoded["requestBody"]
	assert.False(t, hasReqBody, "request.payload:false must still suppress the request body")
	_, hasRespBody := decoded["responseBody"]
	assert.False(t, hasRespBody, "response.payload:false must still suppress the response body")

	reqH := headerMap(t, decoded["requestHeaders"])
	_, hasCookie := reqH["Cookie"]
	assert.False(t, hasCookie, "excluded header sub-key still dropped")
	assert.Equal(t, "bar", reqH["x-foo"], "other request headers still present")

	respH := headerMap(t, decoded["responseHeaders"])
	assert.Equal(t, "baz", respH["x-bar"], "response headers still present per headers:true")
}

// requestBody and properties are top-level keys like any other and can be selected
// explicitly via fields.only.
func TestLog_Publish_FieldsIncludeRequestBodyAndProperties(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "body-data"
	event.TrafficLog = &dto.TrafficLogDirective{
		Properties: map[string]interface{}{"env": "prod"},
		Fields:     &dto.TrafficLogFields{Only: []string{"requestBody", "properties"}},
	}

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasAPI := decoded["api"]
	assert.False(t, hasAPI, "api not listed -> dropped")
	_, hasHeaders := decoded["requestHeaders"]
	assert.False(t, hasHeaders, "requestHeaders not in Only list -> dropped")
	assert.Equal(t, "body-data", decoded["requestBody"])
	props, ok := decoded["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "prod", props["env"])
}

// Output-side payload truncation (0 = no limit).
func TestLog_Publish_TruncatesPayload(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaxPayloadSize: 5})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["request_payload"] = "hello world"
	event.Properties["response_payload"] = "goodbye world"

	l.Publish(event)
	decoded := decodeLine(t, read())
	assert.Equal(t, "hello", decoded["requestBody"])
	assert.Equal(t, "goodb", decoded["responseBody"])
}

func TestLog_Publish_NoTruncationWhenZero(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaxPayloadSize: 0})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["request_payload"] = "hello world"

	l.Publish(event)
	decoded := decodeLine(t, read())
	assert.Equal(t, "hello world", decoded["requestBody"])
}

func TestLog_Publish_UnparseableHeadersDropped(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()
	event.Properties["requestHeaders"] = "not-json"

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, hasHeaders := decoded["requestHeaders"]
	assert.False(t, hasHeaders, "unparseable header value must be silently dropped")
}

// Application is omitted entirely for unauthenticated requests (all fields empty).
func TestLog_Publish_UnauthenticatedRequestOmitsApplication(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Application = &dto.Application{} // all fields are ""
	event.TrafficLog = bothFlows()

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, hasApp := decoded["application"]
	assert.False(t, hasApp, "application with all-empty fields must be absent")
}

// TrafficLogAPI uses clean field names (id/name/kind) not the Moesif apiId/apiName/apiType.
func TestLog_Publish_APIFieldNames(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()

	l.Publish(event)

	decoded := decodeLine(t, read())
	api, ok := decoded["api"].(map[string]interface{})
	require.True(t, ok, "api object must be present")
	assert.Equal(t, "api-123", api["id"])
	assert.Equal(t, "test-api", api["name"])
	assert.Equal(t, "v1.0", api["version"])
	assert.Equal(t, "Rest", api["kind"])
	assert.Equal(t, "/test", api["context"])
	assert.Equal(t, "project-123", api["projectId"])
	// Moesif-specific keys must not bleed into traffic logs.
	for _, moesifKey := range []string{"apiId", "apiName", "apiCreator", "apiCreatorTenantDomain", "organizationId"} {
		_, present := api[moesifKey]
		assert.False(t, present, "Moesif field %q must not appear in traffic log", moesifKey)
	}
}

// Top-level fields — status, correlationId, client — are always present when set.
func TestLog_Publish_TopLevelFieldsPresent(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.TrafficLog = bothFlows()

	l.Publish(event)

	decoded := decodeLine(t, read())
	assert.Equal(t, float64(200), decoded["status"], "proxy response code must appear as status")
	assert.Equal(t, "corr-123", decoded["correlationId"])
	client, ok := decoded["client"].(map[string]interface{})
	require.True(t, ok, "client object must be present")
	assert.Equal(t, "192.168.1.1", client["ip"])
	assert.Equal(t, "test-agent", client["userAgent"])
}
