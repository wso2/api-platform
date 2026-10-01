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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
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

func TestNewRejectsInvalidPlatformAPICA(t *testing.T) {
	_, err := newSuite(nil, []byte("not a PEM certificate"))
	require.ErrorContains(t, err, "loading the generated Platform API CA certificate")
}

func TestJSONFieldNotEqual(t *testing.T) {
	const oldToken = "previous-subscription-token"
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "regenerated token", body: `{"subscriptionToken":"new-subscription-token"}`},
		{name: "unchanged token", body: `{"subscriptionToken":"previous-subscription-token"}`, wantErr: true},
		{name: "empty token", body: `{"subscriptionToken":""}`, wantErr: true},
		{name: "non-string token", body: `{"subscriptionToken":1}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := tcontext.NewLocal("runner")
			local.Set("oldToken", oldToken)
			ctx := tcontext.WithLocal(context.Background(), local)
			require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{Body: []byte(tt.body)}))

			err := (&Base{}).jsonFieldNotEqual(ctx, "subscriptionToken", "${CTX:oldToken}")
			if tt.wantErr {
				require.Error(t, err)
				require.NotContains(t, err.Error(), oldToken)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestJSONFieldContainsBefore(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		first   string
		second  string
		wantErr bool
	}{
		{name: "ordered values", body: `{"prompt":"instruction original prompt"}`, first: "instruction", second: "original"},
		{name: "reversed values", body: `{"prompt":"original prompt instruction"}`, first: "instruction", second: "original", wantErr: true},
		{name: "missing first value", body: `{"prompt":"original prompt"}`, first: "instruction", second: "original", wantErr: true},
		{name: "missing second value", body: `{"prompt":"instruction"}`, first: "instruction", second: "original", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
			require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{Body: []byte(tt.body)}))

			err := (&Base{}).jsonFieldContainsBefore(ctx, "prompt", tt.first, tt.second)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
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

func TestCanonicalResourceTemplates(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)

	root := filepath.Join(filepath.Dir(source), "..", "resources", "templates")
	want := map[string]string{
		"agent.yaml":                 "Agent",
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

func TestEchoedHeaderAssertions(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantExact    bool
		wantContains bool
	}{
		{name: "string value", body: `{"headers":{"X-Forwarded-Client-Cert":"Subject=\"CN=client\";URI=urn:a"}}`, wantContains: true},
		{name: "single element array", body: `{"headers":{"x-forwarded-client-cert":["URI=urn:a"]}}`, wantExact: true, wantContains: true},
		{name: "expected value first of several", body: `{"headers":{"x-forwarded-client-cert":["URI=urn:a","other"]}}`, wantContains: true},
		{name: "expected value after a forged one", body: `{"headers":{"x-forwarded-client-cert":["forged","URI=urn:a"]}}`, wantContains: true},
		{name: "expected value in no element", body: `{"headers":{"x-forwarded-client-cert":["forged","other"]}}`},
		{name: "other value", body: `{"headers":{"x-forwarded-client-cert":"URI=urn:b"}}`},
		{name: "absent header", body: `{"headers":{}}`},
		{name: "empty array", body: `{"headers":{"x-forwarded-client-cert":[]}}`},
		{name: "not an echo", body: `{"other":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := tcontext.NewLocal("runner")
			local.Set("uri", "URI=urn:a")
			ctx := tcontext.WithLocal(context.Background(), local)
			require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{Body: []byte(tt.body)}))

			exactErr := (&Base{}).echoedHeaderEquals(ctx, "X-Forwarded-Client-Cert", "${CTX:uri}")
			containsErr := (&Base{}).echoedHeaderContains(ctx, "X-Forwarded-Client-Cert", "${CTX:uri}")
			require.Equal(t, tt.wantExact, exactErr == nil, "exact: %v", exactErr)
			require.Equal(t, tt.wantContains, containsErr == nil, "contains: %v", containsErr)
		})
	}
}

func TestJSONFieldIsBool(t *testing.T) {
	tests := []struct {
		name, body, field, want string
		ok                      bool
	}{
		{name: "true", body: `{"isLeaf":true}`, field: "isLeaf", want: "true", ok: true},
		{name: "nested false", body: `{"a":{"isLeaf":false}}`, field: "a.isLeaf", want: "false", ok: true},
		{name: "other value", body: `{"isLeaf":true}`, field: "isLeaf", want: "false"},
		{name: "string spelling", body: `{"isLeaf":"true"}`, field: "isLeaf", want: "true"},
		{name: "absent", body: `{}`, field: "isLeaf", want: "true"},
		{name: "not JSON", body: `nope`, field: "isLeaf", want: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
			require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{Body: []byte(tt.body)}))
			err := (&Base{}).jsonFieldIsBool(ctx, tt.field, tt.want)
			require.Equal(t, tt.ok, err == nil, "%v", err)
		})
	}
}

// mtlsFeatureFiles lists every feature that exercises mutual TLS: each mtls_*.feature and
// certificates.feature.
func mtlsFeatureFiles(t *testing.T, featureRoot string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(featureRoot, "features", "mtls_*.feature"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	certificates := filepath.Join(featureRoot, "features", "certificates.feature")
	_, err = os.Stat(certificates)
	require.NoError(t, err)
	return append(paths, certificates)
}

// unresolvedSteps reads godog's event stream of a run and returns the location of every step
// that started without a matching definition. godog announces the definition it resolved
// before it runs a step, and announces none for a step that is undefined or, in strict mode,
// matches two patterns. Counting per location keeps the rows of a scenario outline apart from
// the step they share.
func unresolvedSteps(t *testing.T, events []byte) []string {
	t.Helper()
	started := map[string]int{}
	found := map[string]int{}
	decoder := json.NewDecoder(bytes.NewReader(events))
	for decoder.More() {
		var event struct {
			Event    string `json:"event"`
			Location string `json:"location"`
		}
		require.NoError(t, decoder.Decode(&event))
		switch event.Event {
		case "TestStepStarted":
			started[event.Location]++
		case "StepDefinitionFound":
			found[event.Location]++
		}
	}
	var unresolved []string
	for location, count := range started {
		if found[location] < count {
			unresolved = append(unresolved, location)
		}
	}
	sort.Strings(unresolved)
	return unresolved
}

// TestMTLSFeatureStepsAreDefined matches every step of every mutual TLS feature against the
// registered steps without running one: a hook that fails every scenario skips the steps, and
// godog still resolves each one first. A step that matches no pattern, or two, is reported.
func TestMTLSFeatureStepsAreDefined(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	featureRoot := filepath.Join(filepath.Dir(source), "..")
	for _, path := range mtlsFeatureFiles(t, featureRoot) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			suite, err := New(&frameworkruntime.Topology{}, featureRoot)
			require.NoError(t, err)

			var events bytes.Buffer
			godog.TestSuite{
				ScenarioInitializer: func(sc *godog.ScenarioContext) {
					suite.Register(sc)
					sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
						return ctx, errors.New("steps are matched, not run")
					})
				},
				Options: &godog.Options{Format: "events", Output: &events, NoColors: true, Strict: true, Paths: []string{path}},
			}.Run()

			require.NotZero(t, events.Len(), "godog wrote no events")
			require.Empty(t, unresolvedSteps(t, events.Bytes()), "steps that match no pattern or more than one")
		})
	}
}
