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
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
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

func TestExpectedErrorPathRejectsUnsafeSegments(t *testing.T) {
	path, err := expectedErrorPath("root", "1.2.0", "jwt-auth-missing-token")
	require.NoError(t, err)
	require.Equal(t, filepath.Join("root", "resources", "expected-error-responses", "jwt-auth-missing-token.json"), path)

	for _, id := range []string{"", "../escape", "Upper", "with space", "trailing-", "a/b"} {
		_, err := expectedErrorPath("root", "1.2.0", id)
		require.Error(t, err, "id %q", id)
	}
	for _, version := range []string{"", "..", "../x", "a/b", ".hidden"} {
		_, err := expectedErrorPath("root", version, "ok")
		require.Error(t, err, "version %q", version)
	}
	_, err = expectedErrorPath("", "1.2.0", "ok")
	require.Error(t, err)
}

func TestExpectedErrorPathPrefersAVersionOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "resources", "expected-error-responses")
	shared := filepath.Join(dir, "jwt-auth-missing-token.json")
	override := filepath.Join(dir, "1.1.0", "jwt-auth-missing-token.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(override), 0o755))
	require.NoError(t, os.WriteFile(override, []byte("{}"), 0o644))

	path, err := expectedErrorPath(root, "1.1.0", "jwt-auth-missing-token")
	require.NoError(t, err)
	require.Equal(t, override, path, "the version's own file wins")

	path, err = expectedErrorPath(root, "1.2.0", "jwt-auth-missing-token")
	require.NoError(t, err)
	require.Equal(t, shared, path, "a version without an override uses the shared file")
}

func TestNormalizeErrorResponseKeepsOnlyPinnedHeadersAndMasksVolatileValues(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("WWW-Authenticate", `Bearer realm="api"`)
	headers.Set("Date", "Mon, 05 Oct 2026 04:59:35 GMT")
	headers.Set("X-Request-Id", "3f2a1c9e-1b2c-4d5e-8f90-123456789abc")
	body := []byte(`{"message":"API orders_abcde_7 rejected at 2026-10-05T04:59:35Z",` +
		`"id":"3f2a1c9e-1b2c-4d5e-8f90-123456789abc","nested":[{"ctx":"/orders-abcde-12"}],"n":3}`)

	got, err := normalizeErrorResponse(401, headers, body, "abcde")
	require.NoError(t, err)

	require.Equal(t, 401, got.Status)
	require.Equal(t, map[string]string{
		"content-type":     "application/json",
		"www-authenticate": `Bearer realm="api"`,
	}, got.Headers)
	require.JSONEq(t, `{"message":"API orders<unique> rejected at <timestamp>","id":"<uuid>",`+
		`"nested":[{"ctx":"/orders<unique>"}],"n":3}`, string(got.Body))
	require.Nil(t, got.BodyText)
}

func TestNormalizeErrorResponseHandlesTextAndEmptyBodies(t *testing.T) {
	text, err := normalizeErrorResponse(503, http.Header{}, []byte("no healthy upstream"), "")
	require.NoError(t, err)
	require.NotNil(t, text.BodyText)
	require.Equal(t, "no healthy upstream", *text.BodyText)
	require.Nil(t, text.Body)
	require.Nil(t, text.Headers)

	empty, err := normalizeErrorResponse(204, nil, nil, "")
	require.NoError(t, err)
	require.Equal(t, normalizedErrorResponse{Status: 204}, empty)

	blank, err := normalizeErrorResponse(200, nil, []byte("  \n"), "")
	require.NoError(t, err)
	require.Nil(t, blank.Body)
	require.Nil(t, blank.BodyText)
}

func TestMatchExpectedErrorRecordsThenComparesStructurally(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources", "expected-error-responses", "1.2.0", "case.json")
	first, err := normalizeErrorResponse(422, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"b":2,"a":1}`), "")
	require.NoError(t, err)

	require.Error(t, matchExpectedError(path, first, expectedErrorCompare), "a missing expected error response must fail, not record")
	require.NoError(t, matchExpectedError(path, first, expectedErrorRecord))
	recorded, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(recorded), `"a": 1`)

	reordered, err := normalizeErrorResponse(422, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{ "a": 1, "b": 2 }`), "")
	require.NoError(t, err)
	require.NoError(t, matchExpectedError(path, reordered, expectedErrorCompare), "key order and whitespace are not a change")

	changed, err := normalizeErrorResponse(422, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"a":1,"b":3}`), "")
	require.NoError(t, err)
	require.ErrorContains(t, matchExpectedError(path, changed, expectedErrorCompare), "differs")

	status, err := normalizeErrorResponse(400, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"a":1,"b":2}`), "")
	require.NoError(t, err)
	require.Error(t, matchExpectedError(path, status, expectedErrorCompare), "a different status is a change")

	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))
	require.ErrorContains(t, matchExpectedError(path, first, expectedErrorCompare), "not an expected error response")
}

func TestExpectedErrorModeRequested(t *testing.T) {
	cases := []struct {
		update, accept string
		want           expectedErrorMode
	}{
		{"", "", expectedErrorCompare},
		{"0", "no", expectedErrorCompare},
		{"1", "", expectedErrorRecord},
		{"TRUE", "1", expectedErrorRecord},
		{"", "true", expectedErrorAccept},
	}
	for _, c := range cases {
		t.Setenv(EnvRecordExpectedErrors, c.update)
		t.Setenv(EnvAcceptErrorChanges, c.accept)
		require.Equal(t, c.want, expectedErrorModeRequested(), "update %q accept %q", c.update, c.accept)
	}
}

func TestMatchExpectedErrorAcceptsARecordedChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expected error response", "case.json")
	accepted := strings.TrimSuffix(path, ".json") + acceptedSuffix
	released, err := normalizeErrorResponse(401, nil, []byte(`{"error":"Unauthorized"}`), "")
	require.NoError(t, err)
	changed, err := normalizeErrorResponse(401, nil, []byte(`{"jsonrpc":"2.0","error":{"code":-32600}}`), "")
	require.NoError(t, err)
	other, err := normalizeErrorResponse(401, nil, []byte(`{"error":"something else"}`), "")
	require.NoError(t, err)

	require.Error(t, matchExpectedError(path, changed, expectedErrorAccept), "accepting needs an expected error response first")
	require.NoError(t, matchExpectedError(path, released, expectedErrorRecord))
	require.ErrorContains(t, matchExpectedError(path, changed, expectedErrorCompare), "differs")

	require.NoError(t, matchExpectedError(path, changed, expectedErrorAccept))
	require.FileExists(t, accepted)
	require.NoError(t, matchExpectedError(path, changed, expectedErrorCompare), "the accepted change passes")
	require.NoError(t, matchExpectedError(path, released, expectedErrorCompare), "and so does the expected error response")
	require.ErrorContains(t, matchExpectedError(path, other, expectedErrorCompare), "matches neither")

	require.NoError(t, matchExpectedError(path, released, expectedErrorAccept))
	_, statErr := os.Stat(accepted)
	require.NoError(t, statErr, "a response matching the expected error response leaves the accepted change alone")
}

func TestNormalizeErrorResponseMasksStreamIdentifiers(t *testing.T) {
	stream := "data: {\"created\":1791279593,\"id\":\"chatcmpl-388035d47928447293f25cb81cba71ef\"}\n\n"
	got, err := normalizeErrorResponse(200, nil, []byte(stream), "")
	require.NoError(t, err)
	require.NotNil(t, got.BodyText)
	require.Equal(t, "data: {\"created\":\"<unix-time>\",\"id\":\"chatcmpl-<hex-id>\"}\n\n", *got.BodyText)

	body, err := normalizeErrorResponse(200, nil, []byte(`{"created":1791279593,"count":3}`), "")
	require.NoError(t, err)
	require.JSONEq(t, `{"created":"<unix-time>","count":3}`, string(body.Body),
		"only the created time is masked; other numbers are compared")
}

func TestNormalizeErrorResponsePinsRateLimitHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("X-RateLimit-Limit", "1")
	headers.Set("X-RateLimit-Remaining", "0")
	headers.Set("X-RateLimit-Quota", "default")
	headers.Set("RateLimit-Policy", `"default";q=1;w=3600`)
	headers.Set("RateLimit", `"default";r=0;t=1712, "burst";r=4;t=58`)
	headers.Set("X-RateLimit-Reset", "1791212400")
	headers.Set("Retry-After", "1712")
	headers.Set("Location", "https://example.test/moved")
	headers.Set("X-Envoy-Upstream-Service-Time", "12")

	got, err := normalizeErrorResponse(429, headers, nil, "")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"content-type":          "application/json",
		"x-ratelimit-limit":     "1",
		"x-ratelimit-remaining": "0",
		"x-ratelimit-quota":     "default",
		"ratelimit-policy":      `"default";q=1;w=3600`,
		"ratelimit":             `"default";r=0;t=<seconds>, "burst";r=4;t=<seconds>`,
		"x-ratelimit-reset":     "<present>",
		"retry-after":           "<present>",
		"location":              "https://example.test/moved",
	}, got.Headers)

	later := headers.Clone()
	later.Set("RateLimit", `"default";r=0;t=9, "burst";r=4;t=3`)
	later.Set("X-RateLimit-Reset", "1791216000")
	later.Set("Retry-After", "9")
	again, err := normalizeErrorResponse(429, later, nil, "")
	require.NoError(t, err)
	require.Equal(t, got, again, "clock-dependent values must not change the expected error response")

	missing := headers.Clone()
	missing.Del("Retry-After")
	without, err := normalizeErrorResponse(429, missing, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, got, without, "a dropped Retry-After must change the expected error response")
}
