/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package steps

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"gopkg.in/yaml.v3"
)

func TestSetRequestHostExpandsContextValues(t *testing.T) {
	local := tcontext.NewLocal("runner")
	local.Set("host", "main-unique.local")
	ctx := tcontext.WithLocal(context.Background(), local)
	base := &Base{}

	require.NoError(t, base.setRequestHost(ctx, "${CTX:host}"))
	value, ok := tcontext.Get(ctx, keyRequestHost)
	require.True(t, ok)
	require.Equal(t, "main-unique.local", value)

	err := base.setRequestHost(ctx, "${CTX:missing}")
	require.ErrorContains(t, err, `no value in context for key "missing"`)
}

func TestDefinitionMetadataUsesYAMLParsing(t *testing.T) {
	definition := `apiVersion: v1
kind: RestApi
metadata: {name: "api:with-special-format"}
`
	require.Equal(t, "api:with-special-format", apiNameFrom(definition))
	require.Equal(t, "RestApi", kindFromDefinition(definition))
	require.Equal(t, "api:with-special-format", handleFromDefinition(definition))
	require.Empty(t, apiNameFrom("metadata: [invalid"))
}

func TestParseSecondsRejectsInvalidValues(t *testing.T) {
	seconds, err := parseSeconds(" 1.25 ")
	require.NoError(t, err)
	require.Equal(t, 1.25, seconds)

	for _, value := range []string{"", "-1", "NaN", "+Inf"} {
		_, err := parseSeconds(value)
		require.Error(t, err, "value %q must be rejected", value)
	}
}

func TestParseHTTPStatusLine(t *testing.T) {
	for _, test := range []struct {
		line   string
		status int
	}{
		{line: "HTTP/1.1 408 Request Timeout\r\n", status: 408},
		{line: "HTTP/2 503", status: 503},
	} {
		got, err := parseHTTPStatusLine(test.line)
		require.NoError(t, err)
		require.Equal(t, test.status, got)
	}
	for _, line := range []string{"", "not HTTP", "HTTP/1.1 nope", "HTTP/1.1 99"} {
		_, err := parseHTTPStatusLine(line)
		require.Error(t, err, "status line %q should be rejected", line)
	}
}

func TestMCPJSONPayloadExtractsSSEData(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":2}`)
	require.Equal(t, payload, mcpJSONPayload([]byte("event: message\ndata: "+string(payload)+"\n\n")))
	require.Equal(t, payload, mcpJSONPayload(payload))
	require.Equal(t, []byte("event: message\ndata: not-json\n"),
		mcpJSONPayload([]byte("event: message\ndata: not-json\n")))
}

func TestLazyDisplayNameMatchesOnlyTheRequestedTemplate(t *testing.T) {
	body := []byte(`{"lazy_resources":{"resources_by_type":{"LlmProviderTemplate":[{"id":"template-a","resource":{"spec":{"displayName":"Original"}}},{"id":"template-b","resource":{"spec":{"displayName":"Updated"}}}]}}}`)
	require.True(t, lazyDisplayNameMatches(body, "template-b", "Updated"))
	require.False(t, lazyDisplayNameMatches(body, "template-b", "Original"))
	require.False(t, lazyDisplayNameMatches(body, "missing", "Updated"))
}

func TestProviderTemplateMappingMatchesOnlyTheRequestedProvider(t *testing.T) {
	body := []byte(`{"lazy_resources":{"resources_by_type":{"ProviderTemplateMapping":[{"id":"provider-a","resource":{"template_handle":"openai"}},{"id":"provider-b","resource":{"template_handle":"custom"}}]}}}`)
	require.True(t, providerTemplateMappingMatches(body, "provider-b", "custom"))
	require.False(t, providerTemplateMappingMatches(body, "provider-b", "openai"))
	require.False(t, providerTemplateMappingMatches(body, "missing", "custom"))
}

func TestLazyResourceAbsentMatchesOnlyTheRequestedTypeAndID(t *testing.T) {
	body := []byte(`{"lazy_resources":{"resources_by_type":{"ProviderTemplateMapping":[{"id":"provider-a","resource":{}}],"LlmProvider":[{"id":"provider-a","resource":{}}]}}}`)
	require.True(t, lazyResourceAbsent(body, "provider-a", "LlmProviderTemplate"))
	require.False(t, lazyResourceAbsent(body, "provider-a", "ProviderTemplateMapping"))
	require.True(t, lazyResourceAbsent(body, "missing", "ProviderTemplateMapping"))
}

func TestElapsedToleranceProducesExpectedBounds(t *testing.T) {
	want := 2.0
	floor := time.Duration(want * (1 - elapsedTolerance) * float64(time.Second))
	ceiling := time.Duration(want * (1 + elapsedTolerance) * float64(time.Second))
	require.Equal(t, 1900*time.Millisecond, floor)
	require.Equal(t, 2100*time.Millisecond, ceiling)
}

func TestAnalyticsHeaderValueMatchesNamesWithoutCase(t *testing.T) {
	headers := map[string][]string{"Content-Type": {"application/json"}, "X-Trace": {"abc"}}

	value, present := analyticsHeaderValue(headers, "content-type")
	require.True(t, present)
	require.Equal(t, "application/json", value)

	_, present = analyticsHeaderValue(headers, "authorization")
	require.False(t, present)
}

func TestAnalyticsHeaderValueHandlesNilHeaders(t *testing.T) {
	value, present := analyticsHeaderValue(nil, "content-type")
	require.False(t, present)
	require.Empty(t, value)
}

func TestAnalyticsEventMatchesOperationPath(t *testing.T) {
	for _, test := range []struct {
		eventURI, requestedPath string
		want                    bool
	}{
		{eventURI: "/test", requestedPath: "/analytics/v1.0/test", want: true},
		{eventURI: "/analytics/v1.0/test", requestedPath: "/analytics/v1.0/test", want: true},
		{eventURI: "/test", requestedPath: "/analytics/v1.0/other", want: false},
		{eventURI: "", requestedPath: "/analytics/v1.0/test", want: false},
	} {
		require.Equal(t, test.want, analyticsEventMatchesPath(test.eventURI, test.requestedPath),
			"event URI %q and requested path %q", test.eventURI, test.requestedPath)
	}
}

func TestJSONStringField(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		want    string
		wantErr bool
	}{
		{name: "nested string", body: `{"apiKey":{"apiKey":"secret"}}`, field: "apiKey.apiKey", want: "secret"},
		{name: "missing field", body: `{"status":"success"}`, field: "apiKey.apiKey", wantErr: true},
		{name: "non-string field", body: `{"totalCount":1}`, field: "totalCount", wantErr: true},
		{name: "empty string", body: `{"apiKey":" "}`, field: "apiKey", wantErr: true},
		{name: "invalid JSON", body: `{`, field: "apiKey", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonStringField([]byte(tt.body), tt.field)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResponseHeaderPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "exact dynamic redirect", pattern: `^http://example\.org:[0-9]+/api-[^/]+/v1\.0/go$`, value: "http://example.org:39859/api-abc/v1.0/go", want: true},
		{name: "wrong path", pattern: `^http://example\.org:[0-9]+/api-[^/]+/v1\.0/go$`, value: "http://example.org:39859/api-abc/v1.0/other", want: false},
		{name: "invalid pattern", pattern: `[`, value: "anything", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := regexp.Compile(tt.pattern)
			if err != nil {
				require.False(t, tt.want)
				return
			}
			require.Equal(t, tt.want, re.MatchString(tt.value))
		})
	}
}

func TestPrometheusAssertions(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		valid      bool
		metric     string
		metricSeen bool
	}{
		{
			name:       "metadata and sample",
			body:       "# HELP requests_total Requests\n# TYPE requests_total counter\nrequests_total 2\n",
			valid:      true,
			metric:     "requests_total",
			metricSeen: true,
		},
		{
			name:       "labelled sample",
			body:       "# TYPE requests_total counter\nrequests_total{method=\"GET\"} 1\n",
			valid:      true,
			metric:     "requests_total",
			metricSeen: true,
		},
		{
			name:       "comments without sample",
			body:       "# HELP requests_total Requests\n# TYPE requests_total counter\n",
			valid:      false,
			metric:     "requests_total",
			metricSeen: true,
		},
		{
			name:       "name only appears in another metric",
			body:       "# TYPE other_requests_total counter\nother_requests_total 1\n",
			valid:      true,
			metric:     "requests_total",
			metricSeen: false,
		},
		{
			name:       "empty",
			body:       "",
			valid:      false,
			metric:     "requests_total",
			metricSeen: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.valid, validPrometheusExposition(tt.body))
			require.Equal(t, tt.metricSeen, prometheusMetricPresent(tt.body, tt.metric))
		})
	}
}

func TestTemplatePathIsRestrictedToFeatureRoot(t *testing.T) {
	root := t.TempDir()
	gateway := &Gateway{Base: &Base{featureRoot: root}}
	path, err := gateway.templatePath("resources/templates/rest-api.yaml")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(gateway.featureRoot, "resources/templates/rest-api.yaml"), path)

	for _, name := range []string{"", "/tmp/api.yaml", "../api.yaml", "resources/templates/../../../api.yaml"} {
		_, err := gateway.templatePath(name)
		require.Error(t, err, "path %q should be rejected", name)
	}

	outside := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("kind: RestApi\n"), 0o600))
	link := filepath.Join(root, "resources", "templates", "link.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.Symlink(outside, link))
	_, err = gateway.templatePath("resources/templates/link.yaml")
	require.ErrorContains(t, err, "escapes")
}

func TestCanonicalResourceTemplates(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)

	root := filepath.Join(filepath.Dir(source), "..", "resources", "templates")
	want := map[string]string{
		"llm-provider-template.yaml": "LlmProviderTemplate",
		"llm-provider.yaml":          "LlmProvider",
		"llm-proxy.yaml":             "LlmProxy",
		"mcp.yaml":                   "Mcp",
		"rest-api.yaml":              "RestApi",
	}

	paths, err := filepath.Glob(filepath.Join(root, "*.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, len(want))
	for _, path := range paths {
		name := filepath.Base(path)
		expectedKind, expected := want[name]
		require.Truef(t, expected, "unexpected canonical template %q", name)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		var document map[string]any
		require.NoError(t, yaml.Unmarshal(content, &document))
		require.Equal(t, expectedKind, document["kind"], "template %q", name)
		require.NotEmpty(t, document["apiVersion"], "template %q", name)
		require.NotEmpty(t, document["metadata"], "template %q", name)
		spec, ok := document["spec"].(map[string]any)
		require.True(t, ok, "template %q must define a spec mapping", name)
		require.NotNil(t, spec, "template %q", name)
	}
}
