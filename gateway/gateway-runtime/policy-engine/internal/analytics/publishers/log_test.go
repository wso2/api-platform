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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// bothFlowsConfig returns a TrafficLoggingConfig that opts in to logging both
// request and response headers and payloads — the "log everything captured" case.
// Callers needing additional fields (MaskedHeaders, MaxPayloadSize, ...) can set
// them on the returned pointer before passing it to newLogToFile/NewLog.
func bothFlowsConfig() *config.TrafficLoggingConfig {
	return &config.TrafficLoggingConfig{
		Enabled:         true,
		RequestHeaders:  true,
		RequestBody:     true,
		ResponseHeaders: true,
		ResponseBody:    true,
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

// Traffic logging disabled (the default) -> every event is skipped.
func TestLog_Publish_SkipsWhenGlobalDisabled(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	assert.Empty(t, read(), "traffic logging disabled -> event must not be logged")
}

func TestLog_Publish_WritesJSONLineWithLatencies(t *testing.T) {
	l, read := newLogToFile(t, bothFlowsConfig())
	event := createBaseEvent()
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

	// ALS-derived latencies are always present in the line, including for
	// requests an auth policy short-circuited (denied) before any backend
	// response existed. The traffic log carries microsecond-precision timings,
	// separate from Moesif's ms fields.
	latencies := decoded["latencies"].(map[string]interface{})
	assert.Equal(t, float64(250000), latencies["durationUs"])
}

func TestLog_Publish_MasksHeaders(t *testing.T) {
	cfg := bothFlowsConfig()
	cfg.MaskedHeaders = []string{"Authorization"}
	l, read := newLogToFile(t, cfg)
	event := createBaseEvent()
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

// fields.exclude with a dotted path drops a specific header entirely (vs masking which redacts).
func TestLog_Publish_ExcludeHeadersDrops(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		MaskedHeaders:  []string{"authorization"},
		Enabled:        true,
		RequestHeaders: true,
		ExcludeFields:  []string{"requestHeaders.X-Secret"},
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer s","X-Secret":"top","x-foo":"bar"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])

	_, hasSecret := reqH["X-Secret"]
	assert.False(t, hasSecret, "excluded header must be dropped")
	assert.Equal(t, "****", reqH["Authorization"], "masked header still redacted")
	assert.Equal(t, "bar", reqH["x-foo"])
}

// headers:false omits the headers property; disabled response flags omit the whole side.
func TestLog_Publish_DisabledFieldsOmitted(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:        true,
		RequestHeaders: false,
		RequestBody:    true,
		// ResponseHeaders/ResponseBody left false -> both response props dropped.
	})
	event := createBaseEvent()
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
	assert.False(t, hasRespHeaders, "response headers disabled -> omitted")
	assert.False(t, hasRespPayload, "response payload disabled -> omitted")
}

func TestLog_Publish_DoesNotMutateSharedEvent(t *testing.T) {
	cfg := bothFlowsConfig()
	cfg.MaskedHeaders = []string{"authorization"}
	l, _ := newLogToFile(t, cfg)
	event := createBaseEvent()
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

// Field selection (exclude): named keys are dropped, everything else remains,
// even a key ("requestBody") that would otherwise be present.
func TestLog_Publish_FieldsExclude(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:        true,
		RequestHeaders: true,
		RequestBody:    true,
		ExcludeFields:  []string{"operation", "requestBody"},
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "secret-body"

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasOp := decoded["operation"]
	assert.False(t, hasOp, "operation excluded")
	assert.Contains(t, decoded, "api", "api kept")
	assert.Contains(t, decoded, "latencies", "latencies kept")
	_, hasPayload := decoded["requestBody"]
	assert.False(t, hasPayload, "requestBody excluded even though request_body:true")
	assert.Contains(t, decoded, "requestHeaders", "requestHeaders kept (not excluded)")
}

// An unrelated fields.exclude entry (dropping one header sub-key) must not defeat
// an explicitly configured flow's payload:false/headers:true booleans. Regression
// test for a bug where any fields.exclude entry made the per-flow booleans
// globally inert, silently re-enabling payload logging that request_body:false
// was supposed to suppress.
func TestLog_Publish_FieldsExcludeDoesNotOverrideExplicitFlowBooleans(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:         true,
		RequestHeaders:  true,
		RequestBody:     false,
		ResponseHeaders: true,
		ResponseBody:    false,
		ExcludeFields:   []string{"requestHeaders.Cookie"},
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Cookie":"sid=1","x-foo":"bar"}`
	event.Properties["request_payload"] = "secret-request-body"
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.Properties["response_payload"] = "secret-response-body"

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasReqBody := decoded["requestBody"]
	assert.False(t, hasReqBody, "request_body:false must still suppress the request body")
	_, hasRespBody := decoded["responseBody"]
	assert.False(t, hasRespBody, "response_body:false must still suppress the response body")

	reqH := headerMap(t, decoded["requestHeaders"])
	_, hasCookie := reqH["Cookie"]
	assert.False(t, hasCookie, "excluded header sub-key still dropped")
	assert.Equal(t, "bar", reqH["x-foo"], "other request headers still present")

	respH := headerMap(t, decoded["responseHeaders"])
	assert.Equal(t, "baz", respH["x-bar"], "response headers still present per response_headers:true")
}

// A fields.exclude header sub-key must match regardless of casing on either side
// (config or the header the upstream actually returned), since HTTP header names
// are case-insensitive and an upstream may return e.g. "Set-Cookie" while the
// operator wrote "responseHeaders.set-cookie" in config.
func TestLog_Publish_FieldsExcludeHeaderSubKeyCaseInsensitive(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:         true,
		ResponseHeaders: true,
		ExcludeFields:   []string{"responseHeaders.set-cookie"},
	})
	event := createBaseEvent()
	event.Properties["responseHeaders"] = `{"Set-Cookie":"sid=1","x-bar":"baz"}`

	l.Publish(event)
	decoded := decodeLine(t, read())

	respH := headerMap(t, decoded["responseHeaders"])
	_, hasCookie := respH["Set-Cookie"]
	assert.False(t, hasCookie, "excluded header sub-key dropped despite case mismatch")
	assert.Equal(t, "baz", respH["x-bar"], "other response headers still present")
}

// A properties $ctx: expression can expose a whole CEL map wholesale (e.g.
// "$ctx:auth.property" for a token's full, dynamically-shaped claim set) rather
// than flattening it into one property per claim. exclude_fields must be able to
// reach one specific key inside that nested object via a multi-level dotted path.
func TestLog_Publish_FieldsExcludeMultiLevelPath(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled: true,
		Properties: map[string]string{
			"claims": "$ctx:auth.property",
		},
		ExcludeFields: []string{"properties.claims.internal_debug"},
	})
	event := createBaseEvent()
	event.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme","internal_debug":"xyz"}`

	l.Publish(event)
	decoded := decodeLine(t, read())

	props, ok := decoded["properties"].(map[string]interface{})
	require.True(t, ok, "properties present")
	claims, ok := props["claims"].(map[string]interface{})
	require.True(t, ok, "nested claims object still present")
	_, hasDebug := claims["internal_debug"]
	assert.False(t, hasDebug, "excluded nested key dropped")
	assert.Equal(t, "acme", claims["tenant"], "sibling key under the nested object still present")
}

// Emptying a nested object via a multi-level exclude must collapse every level
// that becomes empty as a result, all the way up — not leave "claims": {} or
// "properties": {} lingering in the output.
func TestLog_Publish_FieldsExcludeMultiLevelPath_CollapsesEmptyParents(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled: true,
		Properties: map[string]string{
			"claims": "$ctx:auth.property",
		},
		ExcludeFields: []string{"properties.claims.internal_debug"},
	})
	event := createBaseEvent()
	// The only claim present is the one being excluded.
	event.Properties[dto.PropKeyAuthProperties] = `{"internal_debug":"xyz"}`

	l.Publish(event)
	decoded := decodeLine(t, read())

	_, hasProperties := decoded["properties"]
	assert.False(t, hasProperties, "emptied properties object removed entirely, not left as {}")
}

// A dotted path that reaches past a header's value (always a string, never an
// object) must be a graceful no-op, not a panic or data corruption.
func TestLog_Publish_FieldsExcludePastHeaderValueIsNoOp(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:        true,
		RequestHeaders: true,
		ExcludeFields:  []string{"requestHeaders.x-foo.bar"},
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar-value"}`

	l.Publish(event)
	decoded := decodeLine(t, read())

	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "bar-value", reqH["x-foo"], "path deeper than a header's string value is a no-op")
}

// Output-side payload truncation (0 = no limit).
func TestLog_Publish_TruncatesPayload(t *testing.T) {
	cfg := bothFlowsConfig()
	cfg.MaxPayloadSize = 5
	l, read := newLogToFile(t, cfg)
	event := createBaseEvent()
	event.Properties["request_payload"] = "hello world"
	event.Properties["response_payload"] = "goodbye world"

	l.Publish(event)
	decoded := decodeLine(t, read())
	assert.Equal(t, "hello", decoded["requestBody"])
	assert.Equal(t, "goodb", decoded["responseBody"])
}

func TestLog_Publish_NoTruncationWhenZero(t *testing.T) {
	cfg := bothFlowsConfig()
	cfg.MaxPayloadSize = 0
	l, read := newLogToFile(t, cfg)
	event := createBaseEvent()
	event.Properties["request_payload"] = "hello world"

	l.Publish(event)
	decoded := decodeLine(t, read())
	assert.Equal(t, "hello world", decoded["requestBody"])
}

func TestLog_Publish_UnparseableHeadersDropped(t *testing.T) {
	cfg := bothFlowsConfig()
	cfg.MaskedHeaders = []string{"authorization"}
	l, read := newLogToFile(t, cfg)
	event := createBaseEvent()
	event.Properties["requestHeaders"] = "not-json"

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, hasHeaders := decoded["requestHeaders"]
	assert.False(t, hasHeaders, "unparseable header value must be silently dropped")
}

// Application is omitted entirely for unauthenticated requests (all fields empty).
func TestLog_Publish_UnauthenticatedRequestOmitsApplication(t *testing.T) {
	l, read := newLogToFile(t, bothFlowsConfig())
	event := createBaseEvent()
	event.Application = &dto.Application{} // all fields are ""

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, hasApp := decoded["application"]
	assert.False(t, hasApp, "application with all-empty fields must be absent")
}

// TrafficLogAPI uses clean field names (id/name/kind) not the Moesif apiId/apiName/apiType.
func TestLog_Publish_APIFieldNames(t *testing.T) {
	l, read := newLogToFile(t, bothFlowsConfig())
	event := createBaseEvent()

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

// Traffic logging emits a line for every request, including ones an auth policy
// short-circuited (denied) before any backend response existed — the key win of
// the config-only, no-policy design.
func TestLog_Publish_GlobalFallback_UsedWhenNoDirective(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:        true,
		RequestHeaders: true,
	})
	event := createBaseEvent() // e.g. a denied/401 request
	event.ProxyResponseCode = 401
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	decoded := decodeLine(t, read())
	assert.Equal(t, float64(401), decoded["status"])
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "bar", reqH["x-foo"])
}

// When traffic logging is disabled (the default), every event is skipped —
// regression guard for the base case.
func TestLog_Publish_GlobalFallbackDisabled_StillSkipsWhenNoDirective(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	assert.Empty(t, read(), "traffic logging disabled -> skip")
}

// The directive's request/response body and header toggles select fields
// exactly like an explicit flow configuration.
func TestLog_Publish_GlobalFallback_FlowToggles(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:         true,
		RequestHeaders:  true,
		RequestBody:     false,
		ResponseHeaders: false,
		ResponseBody:    true,
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "req-body"
	event.Properties["responseHeaders"] = `{"x-bar":"baz"}`
	event.Properties["response_payload"] = "resp-body"

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, hasReqHeaders := decoded["requestHeaders"]
	assert.True(t, hasReqHeaders, "request_headers:true -> included")
	_, hasReqBody := decoded["requestBody"]
	assert.False(t, hasReqBody, "request_body:false -> omitted")
	_, hasRespHeaders := decoded["responseHeaders"]
	assert.False(t, hasRespHeaders, "response_headers:false -> omitted")
	assert.Equal(t, "resp-body", decoded["responseBody"], "response_body:true -> included")
}

// masked_headers and max_payload_size apply to every line.
func TestLog_Publish_GlobalFallback_MaskingAndTruncationApply(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		MaskedHeaders:  []string{"authorization"},
		MaxPayloadSize: 5,
		Enabled:        true,
		RequestHeaders: true,
		RequestBody:    true,
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer secret","x-foo":"bar"}`
	event.Properties["request_payload"] = "hello world"

	l.Publish(event)

	decoded := decodeLine(t, read())
	reqH := headerMap(t, decoded["requestHeaders"])
	assert.Equal(t, "****", reqH["Authorization"])
	assert.Equal(t, "hello", decoded["requestBody"])
}

// The directive's exclude_fields projection layers on top of the flow toggles.
func TestLog_Publish_GlobalFallback_FieldsProjection(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled:        true,
		RequestHeaders: true,
		RequestBody:    true,
		ExcludeFields:  []string{"requestBody"},
	})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`
	event.Properties["request_payload"] = "secret-body"

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, hasBody := decoded["requestBody"]
	assert.False(t, hasBody, "fields.exclude=[requestBody] must drop it from the line")
	assert.Contains(t, decoded, "requestHeaders")
}

func TestBuildGlobalDirective_DisabledReturnsNil(t *testing.T) {
	dir := buildGlobalDirective(config.TrafficLoggingConfig{Enabled: false})
	assert.Nil(t, dir)
}

func TestBuildGlobalDirective_NoFieldsSelectionLeavesFieldsNil(t *testing.T) {
	dir := buildGlobalDirective(config.TrafficLoggingConfig{Enabled: true})
	require.NotNil(t, dir)
	assert.Nil(t, dir.Fields)
}

// End-to-end: traffic_logging.properties resolves "$ctx:" expressions against
// the event and emits them under the top-level "properties" object.
func TestLog_Publish_GlobalFallback_PropertiesResolveCtx(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{
		Enabled: true,
		Properties: map[string]string{
			"env":     "prod",
			"apiName": "$ctx:api.name",
			"status":  "$ctx:response.status",
		},
	})
	event := createBaseEvent()
	event.ProxyResponseCode = 201

	l.Publish(event)

	decoded := decodeLine(t, read())
	props, ok := decoded["properties"].(map[string]interface{})
	require.True(t, ok, "expected top-level properties object")
	assert.Equal(t, "prod", props["env"])
	assert.Equal(t, "test-api", props["apiName"])
	assert.Equal(t, float64(201), props["status"])
}

// No properties configured -> no "properties" key on the line.
func TestLog_Publish_GlobalFallback_NoPropertiesConfiguredOmitsKey(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{Enabled: true})
	event := createBaseEvent()

	l.Publish(event)

	decoded := decodeLine(t, read())
	_, present := decoded["properties"]
	assert.False(t, present)
}

// resolveGlobalDirective must never mutate the shared l.globalDir: successive
// requests for different APIs must not leak each other's resolved properties.
// Each Publish call is directed to its own temp file so the two lines can be
// decoded independently.
func TestLog_Publish_GlobalFallback_PropertiesDoNotLeakAcrossRequests(t *testing.T) {
	l := NewLog(&config.TrafficLoggingConfig{
		Enabled:    true,
		Properties: map[string]string{"apiName": "$ctx:api.name"},
	})
	require.Nil(t, l.globalDir.Properties, "globalDir must never carry baked-in properties")

	readOnce := func(event *dto.Event) map[string]interface{} {
		path := filepath.Join(t.TempDir(), "out.log")
		f, err := os.Create(path)
		require.NoError(t, err)
		defer f.Close()
		l.out = f

		l.Publish(event)

		require.NoError(t, f.Sync())
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return decodeLine(t, string(data))
	}

	first := createBaseEvent()
	first.API.APIName = "api-one"
	firstLine := readOnce(first)
	assert.Equal(t, "api-one", firstLine["properties"].(map[string]interface{})["apiName"])

	require.Nil(t, l.globalDir.Properties, "globalDir must remain unmutated after a Publish call")

	second := createBaseEvent()
	second.API.APIName = "api-two"
	secondLine := readOnce(second)
	assert.Equal(t, "api-two", secondLine["properties"].(map[string]interface{})["apiName"])
}

// Stress the concurrency-safety claim in resolveGlobalDirective's doc comment:
// many goroutines (standing in for concurrent ALS streams) calling Publish at
// once, each with a different API name, must never race on the shared
// l.globalDir. Run with -race to make this meaningful.
func TestLog_Publish_GlobalFallback_ConcurrentPropertiesNoRace(t *testing.T) {
	l := NewLog(&config.TrafficLoggingConfig{
		Enabled:    true,
		Properties: map[string]string{"apiName": "$ctx:api.name"},
	})
	path := filepath.Join(t.TempDir(), "out.log")
	f, err := os.Create(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	l.out = f

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			event := createBaseEvent()
			event.API.APIName = fmt.Sprintf("api-%d", i)
			l.Publish(event)
		}(i)
	}
	wg.Wait()

	require.Nil(t, l.globalDir.Properties, "globalDir must remain unmutated after concurrent Publish calls")
}

// Top-level fields — status, correlationId, client — are always present when set.
func TestLog_Publish_TopLevelFieldsPresent(t *testing.T) {
	l, read := newLogToFile(t, bothFlowsConfig())
	event := createBaseEvent()

	l.Publish(event)

	decoded := decodeLine(t, read())
	assert.Equal(t, float64(200), decoded["status"], "proxy response code must appear as status")
	assert.Equal(t, "corr-123", decoded["correlationId"])
	client, ok := decoded["client"].(map[string]interface{})
	require.True(t, ok, "client object must be present")
	assert.Equal(t, "192.168.1.1", client["ip"])
	assert.Equal(t, "test-agent", client["userAgent"])
}
