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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
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
	_, err := newSuite(nil, []byte("not a PEM certificate"), nil)
	require.ErrorContains(t, err, "loading the generated Platform API CA certificate")
}

func TestGatewayListenerTLSConfig(t *testing.T) {
	t.Run("trusts the checked-in listener certificate under its own name", func(t *testing.T) {
		pem, err := gatewayListenerCertificate()
		require.NoError(t, err)

		cfg, err := gatewayListenerTLSConfig(pem)
		require.NoError(t, err)
		require.Equal(t, "localhost", cfg.ServerName)
		require.NotNil(t, cfg.RootCAs)
	})
	t.Run("rejects malformed PEM", func(t *testing.T) {
		_, err := gatewayListenerTLSConfig([]byte("not a PEM certificate"))
		require.ErrorContains(t, err, "loading the gateway listener certificate")
	})
	t.Run("rejects empty input", func(t *testing.T) {
		_, err := gatewayListenerTLSConfig(nil)
		require.ErrorContains(t, err, "loading the gateway listener certificate")
	})
	t.Run("newSuite fails on an invalid listener certificate", func(t *testing.T) {
		_, err := newSuite(nil, shared.ControlPlaneCrypto()["certs/cert.pem"], []byte("bad"))
		require.ErrorContains(t, err, "loading the gateway listener certificate")
	})
}

func TestSendRequestOnListenerRejectsUnknownContextValue(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	base := &Base{}

	err := base.sendRequestOnListener(ctx, "GET", " over HTTPS", "${CTX:missing}/card")
	require.Error(t, err)
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
	// Platform Gateway templates own the gateway resource envelope.
	gatewayKinds := map[string]string{
		"agent.yaml":                 "Agent",
		"llm-provider-template.yaml": "LlmProviderTemplate",
		"llm-provider.yaml":          "LlmProvider",
		"llm-proxy.yaml":             "LlmProxy",
		"mcp.yaml":                   "Mcp",
		"rest-api.yaml":              "RestApi",
	}
	// Control-plane templates are publisher-API payloads, which carry no gateway envelope.
	controlPlaneProtocols := map[string]string{
		"agent-proxy.yaml": "a2a",
	}

	paths, err := filepath.Glob(filepath.Join(root, "*.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, len(gatewayKinds)+len(controlPlaneProtocols))
	for _, path := range paths {
		name := filepath.Base(path)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		var document map[string]any
		require.NoError(t, yaml.Unmarshal(content, &document))

		if protocol, isControlPlane := controlPlaneProtocols[name]; isControlPlane {
			require.Equal(t, protocol, document["protocol"], "template %q", name)
			for _, envelope := range []string{"apiVersion", "kind", "metadata", "spec"} {
				require.NotContains(t, document, envelope, "control-plane template %q must not carry a gateway envelope", name)
			}
			block, ok := document[protocol].(map[string]any)
			require.True(t, ok, "template %q must define its %q protocol block", name, protocol)
			require.NotEmpty(t, block, "template %q", name)
			continue
		}

		expectedKind, expected := gatewayKinds[name]
		require.Truef(t, expected, "unexpected canonical template %q", name)
		require.Equal(t, expectedKind, document["kind"], "template %q", name)
		require.NotEmpty(t, document["apiVersion"], "template %q", name)
		require.NotEmpty(t, document["metadata"], "template %q", name)
		spec, ok := document["spec"].(map[string]any)
		require.True(t, ok, "template %q must define a spec mapping", name)
		require.NotNil(t, spec, "template %q", name)
	}
}

func TestJSONArrayItemSteps(t *testing.T) {
	base := &Base{}
	publish := func(t *testing.T, body string) context.Context {
		t.Helper()
		ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
		require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{Body: []byte(body)}))
		return ctx
	}
	const listBody = `{"list":[{"id":"a","status":"active","nested":{"kind":"x"}},{"id":"b","status":"revoked"},"not-an-object",{"id":7}]}`

	t.Run("presence", func(t *testing.T) {
		ctx := publish(t, listBody)
		require.NoError(t, base.jsonArrayItemPresence(ctx, "list", "contain", "id", "a"))
		require.NoError(t, base.jsonArrayItemPresence(ctx, "list", "contain", "id", "7"), "numbers match their rendered value")
		require.NoError(t, base.jsonArrayItemPresence(ctx, "list", "not contain", "id", "c"))
		require.Error(t, base.jsonArrayItemPresence(ctx, "list", "contain", "id", "c"))
		require.Error(t, base.jsonArrayItemPresence(ctx, "list", "not contain", "id", "b"))
		require.Error(t, base.jsonArrayItemPresence(ctx, "list", "sometimes contain", "id", "a"))
	})
	t.Run("root array", func(t *testing.T) {
		ctx := publish(t, `[{"name":"k1","artifactUuid":"u1"},{"name":"k2"}]`)
		require.NoError(t, base.jsonArrayItemPresence(ctx, "", "contain", "name", "k2"))
		require.NoError(t, base.jsonArrayItemFieldIs(ctx, "", "name", "k1", "artifactUuid", "u1"))
	})
	t.Run("selected item field", func(t *testing.T) {
		ctx := publish(t, listBody)
		require.NoError(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "a", "status", "active"))
		require.NoError(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "a", "nested.kind", "x"))
		require.ErrorContains(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "a", "status", "revoked"), `expected "revoked"`)
		require.ErrorContains(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "b", "nested", "x"), "has no field")
		require.ErrorContains(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "z", "status", "active"), "has 0 items")
		require.NoError(t, base.jsonArrayItemFieldAbsent(ctx, "list", "id", "b", "nested"))
		require.ErrorContains(t, base.jsonArrayItemFieldAbsent(ctx, "list", "id", "a", "nested"), "should not have field")
	})
	t.Run("ambiguous selection", func(t *testing.T) {
		ctx := publish(t, `{"list":[{"id":"dup","v":1},{"id":"dup","v":2}]}`)
		require.ErrorContains(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "dup", "v", "1"), "want exactly one")
	})
	t.Run("expands context values", func(t *testing.T) {
		ctx := publish(t, listBody)
		local, ok := tcontext.LocalOf(ctx)
		require.True(t, ok)
		local.Set("keyId", "a")
		require.NoError(t, base.jsonArrayItemFieldIs(ctx, "list", "id", "${CTX:keyId}", "status", "active"))
	})
	t.Run("malformed input", func(t *testing.T) {
		require.ErrorContains(t, base.jsonArrayItemPresence(publish(t, `{`), "list", "contain", "id", "a"), "not JSON")
		require.ErrorContains(t, base.jsonArrayItemPresence(publish(t, `{"list":{}}`), "list", "contain", "id", "a"), "not an array")
		require.ErrorContains(t, base.jsonArrayItemPresence(publish(t, `{}`), "list", "contain", "id", "a"), "absent")
	})
}

func headerTable(t *testing.T, rows ...[]string) *godog.Table {
	t.Helper()
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []row `json:"rows"`
	}{}
	for _, values := range rows {
		r := row{}
		for _, v := range values {
			r.Cells = append(r.Cells, cell{Value: v})
		}
		payload.Rows = append(payload.Rows, r)
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	table := &godog.Table{}
	require.NoError(t, json.Unmarshal(raw, table))
	return table
}

func TestPerRequestHeaders(t *testing.T) {
	local := tcontext.NewLocal("runner")
	local.Set("uid", "run-1")
	ctx := tcontext.WithLocal(context.Background(), local)

	headers, err := perRequestHeaders(ctx, headerTable(t,
		[]string{"x-correlation-id", "${CTX:uid}"}, []string{" X-Secret ", "secret"}))
	require.NoError(t, err)
	require.Equal(t, []perRequestHeader{
		{name: "X-Correlation-Id", prefix: "run-1"}, {name: "X-Secret", prefix: "secret"},
	}, headers)

	tests := []struct {
		name  string
		table *godog.Table
		want  string
	}{
		{name: "nil table", want: "table is required"},
		{name: "empty table", table: &godog.Table{}, want: "table is required"},
		{name: "wrong cell count", table: headerTable(t, []string{"X-A"}), want: "exactly two cells"},
		{name: "empty name", table: headerTable(t, []string{" ", "a"}), want: "empty header name"},
		{name: "empty prefix", table: headerTable(t, []string{"X-A", " "}), want: "empty value prefix"},
		{name: "duplicate", table: headerTable(t, []string{"X-A", "a"}, []string{"x-a", "b"}), want: "duplicate"},
		{name: "missing context", table: headerTable(t, []string{"X-A", "${CTX:missing}"}), want: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := perRequestHeaders(ctx, tt.table)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	shared := map[string]string{"Authorization": "Basic x"}
	requests := concurrentRequests(3, "GET", "http://gw/p", "host.local", shared,
		[]perRequestHeader{{name: "X-Correlation-Id", prefix: "run"}})

	require.Len(t, requests, 3)
	for i, req := range requests {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "http://gw/p", req.URL)
		require.Equal(t, "host.local", req.Host)
		require.Equal(t, "Basic x", req.Headers["Authorization"])
		require.Equal(t, fmt.Sprintf("run-%d", i+1), req.Headers["X-Correlation-Id"])
	}
	require.Len(t, shared, 1, "shared headers must not be modified")
	require.NotNil(t, concurrentRequests(1, "GET", "u", "", nil, nil)[0].Headers)
}

func TestSendAllConcurrently(t *testing.T) {
	var inFlight, peak atomic.Int32
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		id := r.Header.Get("X-Correlation-Id")
		mu.Lock()
		seen[id]++
		mu.Unlock()
		if strings.HasSuffix(id, "-fail") {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	client := httpx.NewClient(httpx.Options{})

	requests := concurrentRequests(12, "GET", server.URL, "", nil,
		[]perRequestHeader{{name: "X-Correlation-Id", prefix: "req"}})
	require.NoError(t, sendAllConcurrently(context.Background(), client, requests, 4))
	require.Len(t, seen, 12)
	for id, count := range seen {
		require.Equal(t, 1, count, "request %s", id)
	}
	require.LessOrEqual(t, peak.Load(), int32(4))
	require.Greater(t, peak.Load(), int32(1))

	failing := []httpx.Request{
		{Method: "GET", URL: server.URL, Headers: map[string]string{"X-Correlation-Id": "ok"}},
		{Method: "GET", URL: server.URL, Headers: map[string]string{"X-Correlation-Id": "b-fail"}},
	}
	err := sendAllConcurrently(context.Background(), client, failing, 2)
	require.ErrorContains(t, err, "request 2 of 2 returned")

	require.ErrorContains(t, sendAllConcurrently(context.Background(), client, requests, 0), "must be positive")
	require.ErrorContains(t, sendAllConcurrently(context.Background(), nil, requests, 1), "client is required")
	require.NoError(t, sendAllConcurrently(context.Background(), client, nil, 1))
}

func TestSendConcurrentRejectsInvalidCount(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	table := headerTable(t, []string{"X-A", "a"})
	for _, n := range []int{0, -1, maxConcurrentSendRequests + 1} {
		require.ErrorContains(t, (&Base{}).sendConcurrent(ctx, n, "GET", "/p", table), "must be between")
	}
}

// sendConcurrent is a funnel exception: it clears the previously published response and
// publishes none of its own, whether its requests succeed or fail, so a later response
// assertion can neither pass against a stale response nor read an arbitrary concurrent one.
func TestSendConcurrentLeavesNoPublishedResponse(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-Id")
		mu.Lock()
		seen[id] = r.Method + " " + r.URL.Path
		mu.Unlock()
		if id == "fail-2" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	const gatewayPort = 8080
	definition := &components.Definition{
		Name:      "platform-gateway",
		Alias:     "platform-gateway",
		Endpoints: []components.Endpoint{{Name: "http", Port: gatewayPort, Scheme: "http"}},
	}
	instance, err := components.NewInstance(definition, 0, 1, "127.0.0.1",
		map[int]int{gatewayPort: server.Listener.Addr().(*net.TCPAddr).Port})
	require.NoError(t, err)
	instances := components.NewSet()
	require.NoError(t, instances.Add(instance))
	base := &Base{
		topo:   &frameworkruntime.Topology{Instances: instances},
		funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0),
	}
	stale := &httpx.Response{StatusCode: http.StatusTeapot}

	ctx := publishedContext(t, stale)
	require.NoError(t, base.sendConcurrent(ctx, 3, "get", "/api/v1/resource",
		headerTable(t, []string{"X-Correlation-Id", "req"})))
	require.Equal(t, map[string]string{
		"req-1": "GET /api/v1/resource",
		"req-2": "GET /api/v1/resource",
		"req-3": "GET /api/v1/resource",
	}, seen)
	_, err = httpx.Published(ctx)
	require.ErrorContains(t, err, "no response has been published")

	ctx = publishedContext(t, stale)
	require.ErrorContains(t, base.sendConcurrent(ctx, 2, "GET", "/api/v1/resource",
		headerTable(t, []string{"X-Correlation-Id", "fail"})), "request 2 of 2 returned")
	_, err = httpx.Published(ctx)
	require.ErrorContains(t, err, "no response has been published")
}

func publishedContext(t *testing.T, resp *httpx.Response) context.Context {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, resp))
	return ctx
}

func TestStoreResponseBodyKeepsTheExactBytes(t *testing.T) {
	base := &Base{}
	body := "{\"name\": \"Trip Planner\"}\n"
	ctx := publishedContext(t, &httpx.Response{StatusCode: 200, Body: []byte(body)})

	require.NoError(t, base.storeResponseBody(ctx, "card"))
	value, ok := tcontext.Get(ctx, "card")
	require.True(t, ok)
	require.Equal(t, body, value, "the stored body must keep its whitespace for a byte comparison")

	require.ErrorContains(t, base.storeResponseBody(ctx, " "), "empty key")
	empty := publishedContext(t, &httpx.Response{StatusCode: 304})
	require.ErrorContains(t, base.storeResponseBody(empty, "card"), "empty response body")
	require.Error(t, base.storeResponseBody(tcontext.WithLocal(context.Background(), tcontext.NewLocal("r")), "card"),
		"nothing published")
	noLocal := context.Background()
	require.Error(t, base.storeResponseBody(noLocal, "card"))
}

func TestStoreResponseHeaderRequiresTheHeader(t *testing.T) {
	base := &Base{}
	ctx := publishedContext(t, &httpx.Response{StatusCode: 200, Headers: map[string][]string{"Etag": {`"abc"`}}})

	require.NoError(t, base.storeResponseHeader(ctx, "ETag", "etag"))
	value, ok := tcontext.Get(ctx, "etag")
	require.True(t, ok)
	require.Equal(t, `"abc"`, value)

	require.ErrorContains(t, base.storeResponseHeader(ctx, "X-Missing", "missing"), "did not send it")
	require.ErrorContains(t, base.storeResponseHeader(ctx, "ETag", ""), "empty key")
}

func TestResponseHeaderNotEquals(t *testing.T) {
	base := &Base{}
	ctx := publishedContext(t, &httpx.Response{StatusCode: 200, Headers: map[string][]string{"Etag": {`"new"`}}})
	require.NoError(t, tcontext.Set(ctx, "old", `"old"`))
	require.NoError(t, tcontext.Set(ctx, "same", `"new"`))

	require.NoError(t, base.responseHeaderNotEquals(ctx, "ETag", "${CTX:old}"))
	require.ErrorContains(t, base.responseHeaderNotEquals(ctx, "etag", "${CTX:same}"), "not to be")
	require.NoError(t, base.responseHeaderNotEquals(ctx, "X-Absent", "anything"), "an absent header is not the value")
	require.Error(t, base.responseHeaderNotEquals(ctx, "ETag", "${CTX:unknown}"))
}
