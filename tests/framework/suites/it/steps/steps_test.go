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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/messages/go/v34"
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

func TestGenerateSizedValueExactLength(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	b := &Base{}

	require.NoError(t, b.generateSizedValue(ctx, 10, "ab", "divides"))
	divides, ok := tcontext.Get(ctx, "divides")
	require.True(t, ok)
	require.Equal(t, "ababababab", divides)

	require.NoError(t, b.generateSizedValue(ctx, 7, "abc", "notDivides"))
	notDivides, ok := tcontext.Get(ctx, "notDivides")
	require.True(t, ok)
	require.Equal(t, "abcabca", notDivides)
	require.Len(t, notDivides.(string), 7)

	require.NoError(t, b.generateSizedValue(ctx, 1, "x", "single"))
	single, _ := tcontext.Get(ctx, "single")
	require.Equal(t, "x", single)
}

func TestConcurrentCyclingRowsParsesValuesAndAbsentMarker(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	require.NoError(t, tcontext.Set(ctx, "k1", "secret-value"))
	table := &godog.Table{Rows: []*messages.PickleTableRow{
		{Cells: []*messages.PickleTableCell{{Value: "${CTX:k1}"}, {Value: "200"}}},
		{Cells: []*messages.PickleTableCell{{Value: "bad"}, {Value: "401"}}},
		{Cells: []*messages.PickleTableCell{{Value: absentHeaderMarker}, {Value: "401"}}},
	}}

	rows, err := concurrentCyclingRows(ctx, table)
	require.NoError(t, err)
	require.Equal(t, []concurrentCyclingRow{
		{value: "secret-value", status: 200},
		{value: "bad", status: 401},
		{absent: true, status: 401},
	}, rows)
}

func TestEveryConcurrentResponseMatchesExpectedStatus(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	b := &Base{}

	// No requests sent yet.
	require.Error(t, b.everyConcurrentResponseMatchesExpectedStatus(ctx))

	require.NoError(t, tcontext.Set(ctx, keyConcurrentOutcomes, []concurrentCyclingOutcome{
		{row: concurrentCyclingRow{value: "k1", status: 200}, gotStatus: 200},
		{row: concurrentCyclingRow{absent: true, status: 401}, gotStatus: 401},
	}))
	require.NoError(t, b.everyConcurrentResponseMatchesExpectedStatus(ctx))

	require.NoError(t, tcontext.Set(ctx, keyConcurrentOutcomes, []concurrentCyclingOutcome{
		{row: concurrentCyclingRow{value: "k1", status: 200}, gotStatus: 401},
	}))
	err := b.everyConcurrentResponseMatchesExpectedStatus(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "want 200, got 401")
}

// newTestBaseWithServer builds a Base whose "platform-gateway" http endpoint resolves to a
// local test server, so data-plane steps can be exercised without a running gateway.
func newTestBaseWithServer(t *testing.T, handler http.HandlerFunc) *Base {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	addr := srv.Listener.Addr().(*net.TCPAddr)
	definition := &components.Definition{
		Name: "platform-gateway", Alias: "platform-gateway",
		Endpoints: []components.Endpoint{{Name: "http", Port: addr.Port, Scheme: "http"}},
	}
	instance, err := components.NewInstance(definition, 0, 1, "localhost", map[int]int{addr.Port: addr.Port})
	require.NoError(t, err)
	instances := components.NewSet()
	require.NoError(t, instances.Add(instance))

	return &Base{
		topo:   &frameworkruntime.Topology{Instances: instances},
		funnel: httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 5 * time.Second}), 0, 0),
	}
}

func TestBackgroundTrafficRecordsAndStops(t *testing.T) {
	var hits int32
	b := newTestBaseWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if r.Header.Get("API-Key") == "good" {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = n
		w.WriteHeader(http.StatusUnauthorized)
	})
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))

	require.NoError(t, b.startBackgroundTraffic(ctx, "GET", "/probe", "API-Key", "bad", "probe"))
	require.Error(t, b.startBackgroundTraffic(ctx, "GET", "/probe", "API-Key", "bad", "probe"))

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&hits) >= 3
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, b.stopBackgroundTraffic(ctx, "probe"))
	require.NoError(t, b.backgroundTrafficAtLeastAttempts(ctx, "probe", 1))
	require.NoError(t, b.backgroundTrafficNeverReceivedStatus(ctx, "probe", http.StatusOK))
	require.Error(t, b.backgroundTrafficNeverReceivedStatus(ctx, "probe", http.StatusUnauthorized))
	require.Error(t, b.backgroundTrafficAtLeastAttempts(ctx, "probe", 1000))
}

func TestStopAllBackgroundTrafficStopsEveryProbe(t *testing.T) {
	b := newTestBaseWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))

	require.NoError(t, b.startBackgroundTraffic(ctx, "GET", "/probe", "X", "1", "a"))
	require.NoError(t, b.startBackgroundTraffic(ctx, "GET", "/probe", "X", "1", "b"))

	b.stopAllBackgroundTraffic(ctx)

	probes, err := b.backgroundProbes(ctx)
	require.NoError(t, err)
	for name, probe := range probes {
		select {
		case <-probe.done:
		default:
			t.Fatalf("probe %q did not stop", name)
		}
	}
}

func TestSendRequestOverHTTP2RejectsNonHTTP2Server(t *testing.T) {
	// httptest.NewServer speaks HTTP/1.1 only, so a caller asserting protocol parity
	// gets a clear error instead of a silent fallback.
	b := newTestBaseWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	require.NoError(t, ctx.Err())
	err := b.sendRequestOverHTTP2(ctx, "GET", "/probe")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP/2")
}

func TestIsConnectionRejection(t *testing.T) {
	require.True(t, isConnectionRejection(io.EOF))
	require.True(t, isConnectionRejection(io.ErrUnexpectedEOF))
	require.True(t, isConnectionRejection(fmt.Errorf("wrapped: %w", io.EOF)))
	require.True(t, isConnectionRejection(errors.New("read tcp 127.0.0.1:1234: connection reset by peer")))
	require.False(t, isConnectionRejection(context.DeadlineExceeded))
	require.False(t, isConnectionRejection(errors.New("no such host")))
}

func TestSendRequestExpectingRejectionAcceptsAnAbruptConnectionClose(t *testing.T) {
	b := newTestBaseWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, _, err := hijacker.Hijack()
		require.NoError(t, err)
		_ = conn.Close() // closes before any response line is written, as an oversized-header rejection would
	})
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	require.NoError(t, b.sendRequestExpectingRejection(ctx, "GET", "/probe"))
}

func TestSendRequestExpectingRejectionFailsOnSuccess(t *testing.T) {
	b := newTestBaseWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	err := b.sendRequestExpectingRejection(ctx, "GET", "/probe")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected the gateway to reject the request")
}

func TestSendRequestExpectingRejectionFailsOnATimeout(t *testing.T) {
	release := make(chan struct{})
	// Closed by this defer, which runs before newTestBaseWithServer's t.Cleanup(srv.Close) -
	// httptest.Server.Close blocks until every in-flight handler returns, so the server's own
	// cleanup would deadlock against this one if release were still open when it ran.
	defer close(release)
	b := newTestBaseWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		<-release
	})
	b.funnel = httpx.NewFunnel(httpx.NewClient(httpx.Options{Timeout: 50 * time.Millisecond}), 0, 0)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("test-runner"))
	err := b.sendRequestExpectingRejection(ctx, "GET", "/probe")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "expected the gateway to reject the request")
}
