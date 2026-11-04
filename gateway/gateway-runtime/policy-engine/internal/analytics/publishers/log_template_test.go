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
// none, writing to the returned buffer.
func newTemplateLog(t *testing.T, cfg config.TrafficLoggingConfig) (*Log, *bytes.Buffer) {
	t.Helper()
	cfg.Enabled = true
	if cfg.MaskedHeaders == nil {
		cfg.MaskedHeaders = testMaskedHeaders
	}
	l, err := NewLog(&cfg, nil)
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
			name:     "no value leaves the key out and shortens the array, empty values are written",
			template: `{"a":"$ctx:metadata['absent']","arr":["x","$ctx:metadata['absent']","y"],"e":"$ctx:''","zero":"$ctx:0","no":"$ctx:false"}`,
			want:     `{"arr":["x","y"],"e":"","zero":0,"no":false}`,
		},
		{
			name:     "objects and arrays are kept when every expression in them has no value",
			template: `{"o":{"a":"$ctx:metadata['x']"},"l":["$ctx:metadata['x']"],"k":"fixed"}`,
			want:     `{"o":{},"l":[],"k":"fixed"}`,
		},
		{
			name:     "a line whose only field has no value is an empty object",
			template: `{"a":"$ctx:metadata['x']"}`,
			want:     `{}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, templateLine(t, tt.template, ordersEvent()))
		})
	}
}

// A field with no value for one request is left out of that line without a WARNING or ERROR.
func TestLogTemplate_NoValueIsNotReportedAboveDebug(t *testing.T) {
	l, buf := newTemplateLog(t, config.TrafficLoggingConfig{Template: `{"a":"$ctx:metadata['absent']","k":1}`})
	logs := captureLogs(t)

	l.Publish(ordersEvent())

	assert.Equal(t, `{"k":1}`+"\n", buf.String())
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
