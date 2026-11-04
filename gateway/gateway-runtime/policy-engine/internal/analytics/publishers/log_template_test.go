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
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// captureLogs redirects the package-level slog default into a buffer at debug level for the
// duration of the test, restoring the previous logger afterwards.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// testMaskedHeaders is the default traffic_logging.masked_headers list.
var testMaskedHeaders = []string{"authorization", "x-api-key", "x-jwt-assertion"}

// ordersEvent returns a GET to the API "Orders" that returned 200, with captured request
// headers including a credential.
func ordersEvent() *dto.Event {
	event := createBaseEvent()
	event.RequestTimestamp = fixedRequestTime
	event.API.APIName = "Orders"
	event.Properties[dto.PropKeyRequestHeaders] = `{"authorization":"Bearer secret","x-foo":"bar"}`
	return event
}

// newTemplateLog builds an enabled Log from cfg, with the default masked headers when cfg sets
// none and nothing captured by the collector, writing to the returned buffer.
func newTemplateLog(t *testing.T, cfg config.TrafficLoggingConfig) (*Log, *bytes.Buffer) {
	t.Helper()
	return newTemplateLogWithCollector(t, cfg, nil)
}

// newTemplateLogWithCollector is newTemplateLog with the given collector capture settings.
func newTemplateLogWithCollector(t *testing.T, cfg config.TrafficLoggingConfig, collector *config.CollectorConfig) (*Log, *bytes.Buffer) {
	t.Helper()
	cfg.Enabled = true
	if cfg.MaskedHeaders == nil {
		cfg.MaskedHeaders = testMaskedHeaders
	}
	l, err := NewLog(&cfg, collector)
	require.NoError(t, err)
	var buf bytes.Buffer
	useWriterSink(l, &buf)
	return l, &buf
}

// templateLine publishes event through a Log built from template and returns the line it
// writes, without the trailing newline.
func templateLine(t *testing.T, template string, event *dto.Event) string {
	t.Helper()
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{Template: template})
	l.Publish(event)
	return strings.TrimSuffix(buf.String(), "\n")
}

func TestLogTemplate_WritesTheTemplateLayout(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "fixed values, nested object and array",
			template: `{"service":"orders","http":{"method":"$ctx:request.method","status":"$ctx:response.status"},"tags":["gw","$ctx:api.name"]}`,
			want:     `{"service":"orders","http":{"method":"GET","status":200},"tags":["gw","Orders"]}`,
		},
		{
			name:     "key order kept at every level",
			template: `{"c":1,"a":{"z":"$ctx:api.name","b":2,"y":3},"b":"x"}`,
			want:     `{"c":1,"a":{"z":"Orders","b":2,"y":3},"b":"x"}`,
		},
		{
			name:     "keys are never evaluated",
			template: `{"$ctx:api.name":"$ctx:api.name"}`,
			want:     `{"$ctx:api.name":"Orders"}`,
		},
		{
			name:     "masked header read alone or as the whole map",
			template: `{"auth":"$ctx:request.header['authorization']","hdrs":"$ctx:request.header"}`,
			want:     `{"auth":"****","hdrs":{"authorization":"****","x-foo":"bar"}}`,
		},
		{
			name: "template over several lines gives one line",
			template: `{
  "service": "orders",
  "http": {
    "method": "$ctx:request.method"
  }
}`,
			want: `{"service":"orders","http":{"method":"GET"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, templateLine(t, tt.template, ordersEvent()))
		})
	}
}

func TestLogTemplate_AnyValueStaysOneLineOfValidJSON(t *testing.T) {
	event := ordersEvent()
	event.Properties[dto.PropKeyRequestHeaders] = map[string]string{"x-note": "say \"hi\"\n<b>caf\u00e9 \xff"}

	line := templateLine(t, `{"note":"$ctx:request.header['x-note']","fixed":"a<b"}`, event)

	require.True(t, json.Valid([]byte(line)), line)
	assert.NotContains(t, line, "\n")
	assert.NotContains(t, line, "<")
	assert.Equal(t, `{"note":"say \"hi\"\n\u003cb\u003ecafé \ufffd","fixed":"a\u003cb"}`, line)
}

func TestLogTemplate_OutputRules(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "fixed values are written exactly as given",
			template: `{"n":1.50,"t":true,"f":false,"z":null,"arr":[1,[2,"x"],{"k":0}]}`,
			want:     `{"n":1.50,"t":true,"f":false,"z":null,"arr":[1,[2,"x"],{"k":0}]}`,
		},
		{
			name:     "an expression can return text starting with the prefix",
			template: `{"v":"$ctx:'$ctx:example'"}`,
			want:     `{"v":"$ctx:example"}`,
		},
		{
			name:     "results keep their type and maps are written with sorted keys",
			template: `{"s":"$ctx:api.name","i":"$ctx:response.status","d":"$ctx:1.5","b":"$ctx:true","l":"$ctx:['b','a']","m":"$ctx:{'z':1,'a':'x'}"}`,
			want:     `{"s":"Orders","i":200,"d":1.5,"b":true,"l":["b","a"],"m":{"a":"x","z":1}}`,
		},
		{
			name:     "no value is written as null at any depth, keeping array positions",
			template: `{"a":"$ctx:metadata['absent']","o":{"b":"$ctx:metadata['absent']"},"arr":["x","$ctx:metadata['absent']","y"]}`,
			want:     `{"a":null,"o":{"b":null},"arr":["x",null,"y"]}`,
		},
		{
			name:     "empty values are written",
			template: `{"e":"$ctx:''","zero":"$ctx:0","no":"$ctx:false","l":"$ctx:[]","m":"$ctx:{}"}`,
			want:     `{"e":"","zero":0,"no":false,"l":[],"m":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, templateLine(t, tt.template, ordersEvent()))
		})
	}
}

// A field with no value for one request is written as null without a WARNING or ERROR.
func TestLogTemplate_NoValueIsNotReportedAboveDebug(t *testing.T) {
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{Template: `{"a":"$ctx:metadata['absent']","k":1}`})
	logs := captureLogs(t)

	l.Publish(ordersEvent())

	assert.Equal(t, `{"a":null,"k":1}`+"\n", buf.String())
	assert.NotContains(t, logs.String(), "level=WARN")
	assert.NotContains(t, logs.String(), "level=ERROR")
}

// Headers in a template need only collector capture; the traffic_logging header switches shape
// only the default line.
func TestLogTemplate_HeadersIgnoreTheTrafficLoggingSwitches(t *testing.T) {
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{
		Template:       `{"h":"$ctx:request.header['x-foo']"}`,
		RequestHeaders: false,
	})

	l.Publish(ordersEvent())

	assert.Equal(t, `{"h":"bar"}`+"\n", buf.String())
}

// noLinesSuffix ends every template-error message.
const noLinesSuffix = "; no traffic log lines will be written until the template is fixed and the gateway restarted"

// logged reports whether logs holds a message at level with exactly the text msg.
func logged(logs, level, msg string) bool {
	return strings.Contains(logs, "level="+level+" msg="+strconv.Quote(msg))
}

// A template that cannot be used is reported once, at startup, as an ERROR; startup goes on and
// no line is written, not even the default line.
func TestLogTemplate_TemplateErrors(t *testing.T) {
	tooDeep := strings.Repeat(`{"a":`, maxTemplateDepth+1) + "1" + strings.Repeat("}", maxTemplateDepth+1)
	tests := []struct {
		name string
		cfg  config.TrafficLoggingConfig
		want string
	}{
		{
			name: "both sources",
			cfg:  config.TrafficLoggingConfig{Template: `{"a":1}`, TemplateFile: "/etc/gateway/template.json"},
			want: "traffic_logging.template, traffic_logging.template_file: both are set; set only one",
		},
		{
			name: "invalid JSON",
			cfg:  config.TrafficLoggingConfig{Template: "{\n  \"a\": x\n}"},
			want: "traffic_logging.template: invalid JSON at line 2, column 8: invalid character 'x' looking for beginning of value",
		},
		{
			name: "top level not an object",
			cfg:  config.TrafficLoggingConfig{Template: `["a"]`},
			want: "traffic_logging.template: the template must be a JSON object, found an array",
		},
		{
			name: "content after the object",
			cfg:  config.TrafficLoggingConfig{Template: `{"a":1} {"b":2}`},
			want: "traffic_logging.template: unexpected content after the template object at line 1, column 9",
		},
		{
			name: "repeated key",
			cfg:  config.TrafficLoggingConfig{Template: `{"http":{"m":1,"m":2}}`},
			want: `traffic_logging.template: key "m" appears more than once in "http"`,
		},
		{
			name: "repeated top-level key",
			cfg:  config.TrafficLoggingConfig{Template: `{"a":1,"a":2}`},
			want: `traffic_logging.template: key "a" appears more than once at the top level`,
		},
		{
			name: "too deep",
			cfg:  config.TrafficLoggingConfig{Template: tooDeep},
			want: `traffic_logging.template: nesting deeper than 64 levels at "` + strings.TrimSuffix(strings.Repeat("a.", maxTemplateDepth), ".") + `"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			l, buf := newTemplateLog(t, tt.cfg)

			for range 100 {
				l.Publish(ordersEvent())
			}

			assert.Empty(t, buf.String(), "no line, not even the default line")
			assert.Equal(t, 1, strings.Count(logs.String(), "level="), "exactly one message: %s", logs.String())
			assert.True(t, logged(logs.String(), "ERROR", tt.want+noLinesSuffix), logs.String())
		})
	}
}

// traffic_logging.properties applies only to the default line: with a template it is reported
// once and never compiled or written.
func TestLogTemplate_PropertiesIgnoredWithATemplate(t *testing.T) {
	logs := captureLogs(t)
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{
		Template:   `{"api":"$ctx:api.name"}`,
		Properties: map[string]string{"env": "prod", "broken": "$ctx:no.such.variable"},
	})

	for range 100 {
		l.Publish(ordersEvent())
	}

	assert.Equal(t, strings.Repeat(`{"api":"Orders"}`+"\n", 100), buf.String())
	assert.Equal(t, 1, strings.Count(logs.String(), "level="), "exactly one message: %s", logs.String())
	assert.True(t, logged(logs.String(), "WARN",
		"traffic_logging.properties is set but traffic_logging.template is also set; properties are ignored while a template is used"), logs.String())
}

// A value that does not compile is written as null on every line and reported once at startup,
// with its full path and the reason; the rest of the template is used.
func TestLogTemplate_ValueThatDoesNotCompileIsNull(t *testing.T) {
	logs := captureLogs(t)
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{
		Template: `{"method":"$ctx:request.methd","http":{"request":{"method":"$ctx:request.method +"}},"tags":["gw","$ctx:request.method + 1"],"api":"$ctx:api.name"}`,
	})
	startup := logs.String()

	for range 100 {
		l.Publish(ordersEvent())
	}

	want := `{"method":null,"http":{"request":{"method":null}},"tags":["gw",null],"api":"Orders"}` + "\n"
	assert.Equal(t, strings.Repeat(want, 100), buf.String())
	assert.Equal(t, startup, logs.String(), "nothing is logged per request")
	assert.Equal(t, 3, strings.Count(startup, "level=WARN"), startup)
	assert.NotContains(t, startup, "level=ERROR")
	assert.Contains(t, startup, `field \"method\" is written as null: undeclared reference to 'request.methd'`)
	assert.Contains(t, startup, `field \"http.request.method\" is written as null: Syntax error`)
	assert.Contains(t, startup, `field \"tags[1]\" is written as null: found no matching overload for '_+_' applied to '(string, int)'`)
}

// A field that reads headers the collector does not capture is kept, with one startup WARNING
// listing every reason. A field written as null gets only its own WARNING.
func TestLogTemplate_FieldReadingUncapturedHeadersIsWarned(t *testing.T) {
	const template = `{"req":"$ctx:request.header","res":{"type":"$ctx:response.header['content-type']"},` +
		`"all":"$ctx:{'q': request.header, 's': response.header}","api":"$ctx:api.name","bad":"$ctx:request.header +"}`
	const (
		requestReason  = "reads request.header but collector.request_headers is false; no request headers will be logged"
		responseReason = "reads response.header but collector.response_headers is false; no response headers will be logged"
	)

	t.Run("capture off", func(t *testing.T) {
		logs := captureLogs(t)
		l, buf := newTemplateLogWithCollector(t, config.TrafficLoggingConfig{Template: template}, &config.CollectorConfig{})

		l.Publish(ordersEvent())

		assert.True(t, logged(logs.String(), "WARN", `traffic_logging.template: field "req" `+requestReason), logs.String())
		assert.True(t, logged(logs.String(), "WARN", `traffic_logging.template: field "res.type" `+responseReason), logs.String())
		assert.True(t, logged(logs.String(), "WARN", `traffic_logging.template: field "all" `+requestReason+"; "+responseReason), logs.String())
		assert.Equal(t, 1, strings.Count(logs.String(), `field \"bad\"`), "only the compile WARNING for bad")
		assert.Equal(t, 4, strings.Count(logs.String(), "level=WARN"), logs.String())
		assert.Contains(t, buf.String(), `"req":{"authorization":"****","x-foo":"bar"}`, "the field is kept")
	})

	t.Run("capture on", func(t *testing.T) {
		logs := captureLogs(t)
		newTemplateLogWithCollector(t, config.TrafficLoggingConfig{Template: template},
			&config.CollectorConfig{RequestHeaders: true, ResponseHeaders: true})

		assert.NotContains(t, logs.String(), "no request headers will be logged")
		assert.NotContains(t, logs.String(), "no response headers will be logged")
		assert.Equal(t, 1, strings.Count(logs.String(), "level=WARN"), "only the compile WARNING for bad")
	})
}

// Startup messages never carry a header value, a claim value or a body from a request.
func TestLogTemplate_StartupMessagesCarryNoRequestValues(t *testing.T) {
	event := ordersEvent()
	event.Properties[dto.PropKeyAuthProperties] = `{"email":"alice@example.com"}`
	event.Properties[dto.PropKeyRequestPayload] = `{"card":"4111"}`
	logs := captureLogs(t)
	l, _ := newTemplateLog(t, config.TrafficLoggingConfig{
		Template:   `{"h":"$ctx:request.header","c":"$ctx:auth.property","bad":"$ctx:request.header['authorization'] +"}`,
		Properties: map[string]string{"env": "prod"},
	})

	for range 10 {
		l.Publish(event)
	}

	for _, value := range []string{"Bearer secret", "alice@example.com", "4111"} {
		assert.NotContains(t, logs.String(), value)
	}
}

// exclusionTemplate writes fixed values, nested objects, a flat key with a dot, a claims map, the
// request header map and an array.
const exclusionTemplate = `{"service":"orders","http":{"method":"$ctx:request.method","status":"$ctx:response.status"},"http.route":"$ctx:request.pathTemplate","client":{"ip":"$ctx:client.ip"},"auth_prop":"$ctx:auth.property","hdrs":"$ctx:request.header","tags":["gw","$ctx:api.name"]}`

// exclusionEvent is ordersEvent with a route, a client IP, two claims and more request headers.
func exclusionEvent() *dto.Event {
	event := ordersEvent()
	event.UserIP = "10.0.0.1"
	event.Properties[dto.PropKeyOperationPath] = "/orders/{id}"
	event.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme","region":"eu"}`
	event.Properties[dto.PropKeyRequestHeaders] = `{"authorization":"Bearer secret","x-foo":"bar","x-user":"alice"}`
	return event
}

// excludedLine publishes event through a Log built from template and exclude, with headers
// captured, and returns the line and everything logged while building and publishing.
func excludedLine(t *testing.T, template string, exclude []string, event *dto.Event) (line, logs string) {
	t.Helper()
	logBuf := captureLogs(t)
	l, buf := newTemplateLogWithCollector(t, config.TrafficLoggingConfig{Template: template, ExcludeFields: exclude},
		&config.CollectorConfig{RequestHeaders: true, ResponseHeaders: true})
	l.Publish(event)
	return strings.TrimSuffix(buf.String(), "\n"), logBuf.String()
}

func TestLogTemplate_ExcludeFields(t *testing.T) {
	const (
		service  = `"service":"orders"`
		http     = `"http":{"method":"GET","status":200}`
		route    = `"http.route":"/orders/{id}"`
		client   = `"client":{"ip":"10.0.0.1"}`
		authProp = `"auth_prop":{"region":"eu","tenant":"acme"}`
		hdrs     = `"hdrs":{"authorization":"****","x-foo":"bar","x-user":"alice"}`
		tags     = `"tags":["gw","Orders"]`
	)
	line := func(fields ...string) string { return "{" + strings.Join(fields, ",") + "}" }
	full := line(service, http, route, client, authProp, hdrs, tags)

	tests := []struct {
		name    string
		exclude []string
		want    string
	}{
		{"nothing excluded", nil, full},
		{"a whole field", []string{"client"}, line(service, http, route, authProp, hdrs, tags)},
		{"the only key of an object removes the object", []string{"client.ip"}, line(service, http, route, authProp, hdrs, tags)},
		{"a nested key", []string{"http.method"}, line(service, `"http":{"status":200}`, route, client, authProp, hdrs, tags)},
		{"every key of an object", []string{"http.method", "http.status"}, line(service, route, client, authProp, hdrs, tags)},
		{"a flat key with a dot", []string{"http.route"}, line(service, http, client, authProp, hdrs, tags)},
		{"a key inside an expression's value", []string{"auth_prop.tenant"}, line(service, http, route, client, `"auth_prop":{"region":"eu"}`, hdrs, tags)},
		{"claim names match exactly", []string{"auth_prop.Tenant"}, full},
		{"header names match in any case", []string{"hdrs.X-User"}, line(service, http, route, client, authProp, `"hdrs":{"authorization":"****","x-foo":"bar"}`, tags)},
		{"a masked header can be removed", []string{"hdrs.authorization"}, line(service, http, route, client, authProp, `"hdrs":{"x-foo":"bar","x-user":"alice"}`, tags)},
		{"arrays are not reached", []string{"tags.0"}, full},
		{"fixed values are not reached", []string{"service.name"}, full},
		{"header values are not reached", []string{"hdrs.x-foo.bar"}, full},
		{"no such field", []string{"nosuch"}, full},
		{"every top-level field", []string{"service", "http", "http.route", "client", "auth_prop", "hdrs", "tags"}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, logs := excludedLine(t, exclusionTemplate, tt.exclude, exclusionEvent())
			assert.Equal(t, tt.want, got)
			assert.Empty(t, logs, "an exclusion is never logged")
		})
	}
}

func TestLogTemplate_ExclusionThatEmptiesAValueRemovesIt(t *testing.T) {
	onlyTenant := func() *dto.Event {
		event := exclusionEvent()
		event.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme"}`
		return event
	}
	noClaims := func() *dto.Event {
		event := exclusionEvent()
		delete(event.Properties, dto.PropKeyAuthProperties)
		return event
	}
	metadata := func() *dto.Event {
		event := exclusionEvent()
		event.Properties[dto.PropKeyMetadata] = `{"x.y":1,"x":{"y":2},"deep":{"k":{"v":3}},"list":[{"k":1}]}`
		return event
	}

	tests := []struct {
		name     string
		template string
		exclude  []string
		event    *dto.Event
		want     string
	}{
		{"first field", `{"a":"$ctx:auth.property","b":1}`, []string{"a.tenant"}, onlyTenant(), `{"b":1}`},
		{"middle field", `{"x":1,"a":"$ctx:auth.property","b":2}`, []string{"a.tenant"}, onlyTenant(), `{"x":1,"b":2}`},
		{"last field", `{"x":1,"a":"$ctx:auth.property"}`, []string{"a.tenant"}, onlyTenant(), `{"x":1}`},
		{"emptied objects above it are removed too", `{"o":{"p":{"a":"$ctx:auth.property"}},"b":1}`, []string{"o.p.a.tenant"}, onlyTenant(), `{"b":1}`},
		{"an object above it with other fields stays", `{"o":{"a":"$ctx:auth.property","n":"$ctx:api.name"}}`, []string{"o.a.tenant"}, onlyTenant(), `{"o":{"n":"Orders"}}`},
		{"every field emptied gives an empty line object", `{"a":"$ctx:auth.property"}`, []string{"a.tenant"}, onlyTenant(), `{}`},
		{"a value that is already empty stays", `{"a":"$ctx:auth.property"}`, []string{"a.tenant"}, noClaims(), `{"a":{}}`},
		{"an object written empty stays", `{"e":{},"b":1}`, []string{"e.x"}, onlyTenant(), `{"e":{},"b":1}`},
		{"a header map emptied by exclusion is removed", `{"h":"$ctx:request.header","b":1}`, []string{"h.authorization", "h.x-foo"}, ordersEvent(), `{"b":1}`},
		{"a header map with no captured headers stays", `{"h":"$ctx:request.header"}`, []string{"h.x-foo"}, createBaseEvent(), `{"h":{}}`},
		{"a flat key inside a value", `{"m":"$ctx:metadata"}`, []string{"m.x.y"}, metadata(), `{"m":{"deep":{"k":{"v":3}},"list":[{"k":1}],"x":{"y":2}}}`},
		{"emptied maps inside a value are removed", `{"m":"$ctx:metadata"}`, []string{"m.deep.k.v"}, metadata(), `{"m":{"list":[{"k":1}],"x":{"y":2},"x.y":1}}`},
		{"lists inside a value are not reached", `{"m":"$ctx:metadata"}`, []string{"m.list.0.k"}, metadata(), `{"m":{"deep":{"k":{"v":3}},"list":[{"k":1}],"x":{"y":2},"x.y":1}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, logs := excludedLine(t, tt.template, tt.exclude, tt.event)
			assert.Equal(t, tt.want, got)
			assert.Empty(t, logs)
		})
	}
}

func TestLogTemplate_ExclusionDoesNotCarryOverToTheNextRequest(t *testing.T) {
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{
		Template:      `{"a":"$ctx:auth.property","b":1}`,
		ExcludeFields: []string{"a.tenant"},
	})
	onlyTenant := exclusionEvent()
	onlyTenant.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme"}`

	l.Publish(onlyTenant)
	l.Publish(exclusionEvent())
	l.Publish(onlyTenant)

	assert.Equal(t, `{"b":1}`+"\n"+`{"a":{"region":"eu"},"b":1}`+"\n"+`{"b":1}`+"\n", buf.String())
}

func TestLogTemplate_ExcludedAndMaskedHeaders(t *testing.T) {
	event := func() *dto.Event {
		event := ordersEvent()
		event.Properties[dto.PropKeyResponseHeaders] = `{"Set-Cookie":"sid=1","content-type":"application/json"}`
		return event
	}
	const singleHeader = `{"auth":"$ctx:request.header['authorization']","n":"$ctx:api.name"}`

	tests := []struct {
		name     string
		template string
		exclude  []string
		want     string
	}{
		{"a single masked header is masked", singleHeader, nil, `{"auth":"****","n":"Orders"}`},
		{"a single masked header is masked whatever else is excluded", singleHeader, []string{"n", "auth.x"}, `{"auth":"****"}`},
		{"a single header field can be removed", singleHeader, []string{"auth"}, `{"n":"Orders"}`},
		{"the header map stays masked when another header is removed", `{"hdrs":"$ctx:request.header"}`, []string{"hdrs.X-Foo"}, `{"hdrs":{"authorization":"****"}}`},
		{"a masked header can be removed from the header map", `{"hdrs":"$ctx:request.header"}`, []string{"hdrs.Authorization"}, `{"hdrs":{"x-foo":"bar"}}`},
		{"two fields read the same headers, one is filtered", `{"h1":"$ctx:request.header","h2":"$ctx:request.header"}`, []string{"h1.x-foo"}, `{"h1":{"authorization":"****"},"h2":{"authorization":"****","x-foo":"bar"}}`},
		{"response header names match in any case", `{"rh":"$ctx:response.header"}`, []string{"rh.SET-COOKIE"}, `{"rh":{"content-type":"application/json"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, logs := excludedLine(t, tt.template, tt.exclude, event())
			assert.Equal(t, tt.want, got)
			assert.NotContains(t, got, "secret")
			assert.Empty(t, logs)
		})
	}
}

func TestLogTemplate_ExcludedFieldIsNeverCompiled(t *testing.T) {
	logs := captureLogs(t)
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{
		Template:      `{"bad":"$ctx:nosuch.var","hdrs":"$ctx:request.header","n":1}`,
		ExcludeFields: []string{"bad", "hdrs"},
	})
	l.Publish(ordersEvent())

	assert.Equal(t, `{"n":1}`+"\n", buf.String())
	assert.Empty(t, logs.String(), "an excluded field that does not compile, or reads uncaptured headers, gets no WARNING")
}

func TestLogTemplate_PathInsideAFieldThatDoesNotCompile(t *testing.T) {
	line, logs := excludedLine(t, `{"bad":"$ctx:nosuch.var","n":1}`, []string{"bad.x"}, ordersEvent())

	assert.Equal(t, `{"bad":null,"n":1}`, line)
	assert.Equal(t, 1, strings.Count(logs, "level=WARN"), logs)
	assert.Contains(t, logs, `field \"bad\" is written as null`)
}

// latencyEvent is ordersEvent with the given backend latency in microseconds; its total
// duration is 250,000 µs.
func latencyEvent(backendUs int64) *dto.Event {
	event := ordersEvent()
	event.TrafficLogLatencies.BackendLatencyUs = backendUs
	return event
}

func TestLogTemplate_LatenciesInAnyUnit(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		backendUs int64
		want      string
	}{
		{
			name:      "whole microseconds and converted",
			template:  `{"backend":"$ctx:latencies.backendLatency","backend_ms":"$ctx:convertUnit(latencies.backendLatency, 'ms')","backend_s":"$ctx:convertUnit(latencies.backendLatency, 's')"}`,
			backendUs: 12345,
			want:      `{"backend":12345,"backend_ms":12.345,"backend_s":0.012345}`,
		},
		{"nanoseconds", `{"v":"$ctx:convertUnit(latencies.backendLatency, 'ns')"}`, 12345, `{"v":12345000}`},
		{"microseconds", `{"v":"$ctx:convertUnit(latencies.backendLatency, 'us')"}`, 12345, `{"v":12345}`},
		{"milliseconds", `{"v":"$ctx:convertUnit(latencies.backendLatency, 'ms')"}`, 12345, `{"v":12.345}`},
		{"seconds", `{"v":"$ctx:convertUnit(latencies.backendLatency, 's')"}`, 12345, `{"v":0.012345}`},
		{"no decimals lost", `{"v":"$ctx:convertUnit(latencies.backendLatency, 'ms')"}`, 420, `{"v":0.42}`},
		{"any int expression", `{"v":"$ctx:convertUnit(latencies.duration - latencies.backendLatency, 'ms')"}`, 12345, `{"v":237.655}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, templateLine(t, tt.template, latencyEvent(tt.backendUs)))
		})
	}
}

func TestLogTemplate_ConvertUnitMistakeIsNull(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		message    string
	}{
		{"unsupported unit", "$ctx:convertUnit(latencies.duration, 'mins')", `convertUnit: unsupported unit "mins" (supported: ns, us, ms, s)`},
		{"unit not a constant", "$ctx:convertUnit(latencies.duration, request.header['unit'])", "convertUnit: the unit must be a constant: 'ns', 'us', 'ms' or 's'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			l, buf := newTemplateLog(t, config.TrafficLoggingConfig{Template: `{"dur":"` + tt.expression + `","n":1}`})
			for i := 0; i < 3; i++ {
				l.Publish(ordersEvent())
			}

			assert.Equal(t, strings.Repeat(`{"dur":null,"n":1}`+"\n", 3), buf.String())
			assert.Equal(t, 1, strings.Count(logs.String(), "level=WARN"), logs.String())
			assert.True(t, logged(logs.String(), "WARN", `traffic_logging.template: field "dur" is written as null: `+tt.message), logs.String())
		})
	}
}

// A request rejected before it reached the backend has no latencies.
func TestLogTemplate_NoLatenciesWritesNull(t *testing.T) {
	event := ordersEvent()
	event.TrafficLogLatencies = nil

	line := templateLine(t, `{"d":"$ctx:latencies.duration","rq":"$ctx:latencies.requestMediationLatency","rs":"$ctx:latencies.responseMediationLatency","b":"$ctx:latencies.backendLatency","ms":"$ctx:convertUnit(latencies.duration, 'ms')"}`, event)

	assert.Equal(t, `{"d":null,"rq":null,"rs":null,"b":null,"ms":null}`, line)
}

// dropEmpty removes "", 0 and objects left empty, so a template line, which writes every
// field, can be compared with the default line, which leaves empty fields out.
func dropEmpty(m map[string]interface{}) {
	for k, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			dropEmpty(nested)
			if len(nested) == 0 {
				delete(m, k)
			}
			continue
		}
		if v == "" || v == float64(0) {
			delete(m, k)
		}
	}
}

func TestLogTemplate_TimestampClientAndErrorMatchTheDefaultLine(t *testing.T) {
	const template = `{"timestamp":"$ctx:request.timestamp",` +
		`"client":{"ip":"$ctx:client.ip","userAgent":"$ctx:client.userAgent"},` +
		`"errorType":"$ctx:error.category",` +
		`"error":{"errorCode":"$ctx:error.errorCode","errorMessage":"$ctx:error.errorMessage","wso2ErrorCode":"$ctx:error.wso2ErrorCode",` +
		`"type":"$ctx:error.type","direction":"$ctx:error.direction","summary":"$ctx:error.summary","policy":"$ctx:error.policy",` +
		`"policyPhase":"$ctx:error.policyPhase","source":"$ctx:error.source","originalStatus":"$ctx:error.originalStatus",` +
		`"jsonRpcCode":"$ctx:error.jsonRpcCode",` +
		`"guardrail":{"name":"$ctx:error.guardrail.name","action":"$ctx:error.guardrail.action","reason":"$ctx:error.guardrail.reason"}}}`

	events := defaultLineEvents()
	guardrail := defaultLineEvents()["auth failure"]
	guardrail.ErrorType = "OTHER"
	guardrail.Error = &dto.Error{
		ErrorCode: 446, ErrorMessage: dto.OtherMediationError, Wso2ErrorCode: 906201, Type: "guardrail",
		Direction: "response", Summary: "Response blocked", Policy: "pii-guardrail", PolicyPhase: "response_body",
		Source: "gateway", OriginalStatus: 200, JSONRPCCode: -32603,
		Guardrail: &dto.ErrorGuardrail{Name: "pii", Action: "block", Reason: "email address found"},
	}
	events["guardrail"] = guardrail

	for name, event := range events {
		t.Run(name, func(t *testing.T) {
			defaultLog, err := NewLog(defaultLineConfigs()["plain"], nil)
			require.NoError(t, err)
			var defaultBuf bytes.Buffer
			useWriterSink(defaultLog, &defaultBuf)
			defaultLog.Publish(event)

			var defaultLine, line map[string]interface{}
			require.NoError(t, json.Unmarshal(defaultBuf.Bytes(), &defaultLine))
			require.NoError(t, json.Unmarshal([]byte(templateLine(t, template, event)), &line))
			for key := range defaultLine {
				if key != "timestamp" && key != "client" && key != "errorType" && key != "error" {
					delete(defaultLine, key)
				}
			}
			dropEmpty(line)
			assert.Equal(t, defaultLine, line)
		})
	}
}

func TestLogTemplate_Bodies(t *testing.T) {
	const template = `{"req":{"body":"$ctx:request.body"},"resp":"$ctx:response.body","n":1}`
	withBodies := func() *dto.Event {
		event := ordersEvent()
		event.Properties[dto.PropKeyRequestPayload] = `{"order":1}`
		event.Properties[dto.PropKeyResponsePayload] = `{"id":7}`
		return event
	}
	const requestBodyWarning = `traffic_logging.template: field "req.body" reads request.body but collector.request_body is false; no request body will be logged`
	const responseBodyWarning = `traffic_logging.template: field "resp" reads response.body but collector.response_body is false; no response body will be logged`

	t.Run("captured bodies are cut to max_payload_size whatever the traffic_logging switches say", func(t *testing.T) {
		logs := captureLogs(t)
		l, buf := newTemplateLogWithCollector(t,
			config.TrafficLoggingConfig{Template: template, MaxPayloadSize: 5, RequestBody: false, ResponseBody: false},
			&config.CollectorConfig{RequestBody: true, ResponseBody: true})
		l.Publish(withBodies())

		assert.Equal(t, `{"req":{"body":"{\"ord"},"resp":"{\"id\"","n":1}`+"\n", buf.String())
		assert.Empty(t, logs.String())
	})

	t.Run("bodies the collector does not capture are warned about and written as null", func(t *testing.T) {
		logs := captureLogs(t)
		l, buf := newTemplateLog(t, config.TrafficLoggingConfig{Template: template})
		l.Publish(ordersEvent())

		assert.Equal(t, `{"req":{"body":null},"resp":null,"n":1}`+"\n", buf.String())
		assert.True(t, logged(logs.String(), "WARN", requestBodyWarning), logs.String())
		assert.True(t, logged(logs.String(), "WARN", responseBodyWarning), logs.String())
		assert.Equal(t, 2, strings.Count(logs.String(), "level=WARN"), logs.String())
	})

	t.Run("an excluded body field is removed and not warned about", func(t *testing.T) {
		logs := captureLogs(t)
		l, buf := newTemplateLog(t, config.TrafficLoggingConfig{Template: template, ExcludeFields: []string{"req.body", "resp"}})
		l.Publish(withBodies())

		assert.Equal(t, `{"n":1}`+"\n", buf.String())
		assert.Empty(t, logs.String())
	})
}
