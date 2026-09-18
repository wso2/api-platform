/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/testproxy"
)

const invokePath = "/api/test-console/invoke"

// testConsoleConfig returns the config the relay tests share: the feature on,
// with bounds small enough that a test can reach them.
func testConsoleConfig(controlPlaneURL string) *config.Config {
	cfg := newTestConfig(controlPlaneURL)
	cfg.TestConsole = config.TestConsoleConfig{
		Enabled:          true,
		RequestTimeout:   5 * time.Second,
		MaxRequestBytes:  4096,
		MaxResponseBytes: 1 << 20,
		MaxConcurrent:    4,
		MaxPending:       4,
		ResolveCacheTTL:  time.Minute,
		ResolveCacheSize: 16,
	}
	return cfg
}

// platformAPIFor answers the resolver's two reads, pointing the named gateway
// at gatewayURL.
func platformAPIFor(t *testing.T, gatewayURL string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/gateways"):
			_, _ = w.Write([]byte(`{"list":[{"id":"gw-prod","organizationId":"acme",` +
				`"endpoints":["` + gatewayURL + `"],"isDeployed":true}]}`))
		case strings.HasSuffix(r.URL.Path, "/auth/login"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":      makeJWT(map[string]any{"username": "admin", "org_handle": "acme"}),
				"expires_at": time.Now().Add(time.Hour).Unix(),
			})
		default:
			_, _ = w.Write([]byte(`{"context":"/pizza/v1"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// invokeFixture wires a BFF with a live session cookie against a stub gateway.
type invokeFixture struct {
	bff    *httptest.Server
	client *http.Client
	cookie *http.Cookie
}

func newInvokeFixture(t *testing.T, cfg *config.Config) *invokeFixture {
	t.Helper()
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	bff := httptest.NewServer(srv.Handler())
	t.Cleanup(bff.Close)

	return &invokeFixture{
		bff:    bff,
		client: &http.Client{},
		cookie: &http.Cookie{
			Name:  cfg.Session.Cookie.Name,
			Value: makeJWT(map[string]any{"username": "admin", "org_handle": "acme", "exp": time.Now().Add(time.Hour).Unix()}),
		},
	}
}

func (f *invokeFixture) post(t *testing.T, body string, withSession bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.bff.URL+invokePath, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(config.CSRFHeaderName, "api-control-plane")
	if withSession {
		req.AddCookie(f.cookie)
	}
	res, err := f.client.Do(req)
	if err != nil {
		t.Fatalf("invoke request: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func decodeResult(t *testing.T, res *http.Response) testproxy.Result {
	t.Helper()
	var out testproxy.Result
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return out
}

func decodeError(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return out
}

func TestInvokeRelaysToTheResolvedGateway(t *testing.T) {
	var gotAuth, gotCookie, gotKey, gotURI string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotKey = r.Header.Get("X-API-Key")
		gotURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer gateway.Close()

	platform := platformAPIFor(t, gateway.URL)
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	res := f.post(t, `{
		"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod",
		"method":"GET","path":"/order",
		"query":[{"name":"size","value":"L"}],
		"headers":[{"name":"X-API-Key","value":"test-key"}]
	}`, true)

	assertStatus(t, res, http.StatusOK)
	result := decodeResult(t, res)
	if result.Outcome != "response" {
		t.Errorf("outcome = %q, want %q", result.Outcome, "response")
	}
	if result.Response.Status != http.StatusOK {
		t.Errorf("upstream status = %d, want 200", result.Response.Status)
	}
	if result.Response.Body != `{"ok":true}` {
		t.Errorf("body = %q", result.Response.Body)
	}
	if gotURI != "/pizza/v1/order?size=L" {
		t.Errorf("gateway saw %q, want /pizza/v1/order?size=L", gotURI)
	}
	if gotKey != "test-key" {
		t.Errorf("test key header = %q, want test-key", gotKey)
	}

	// The whole reason this is a separate code path from internal/proxy.
	if gotAuth != "" {
		t.Errorf("the session's bearer token reached the gateway: %q", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("the session cookie reached the gateway: %q", gotCookie)
	}
}

func TestInvokeReportsAGatewayErrorAsDataNotAsARelayFailure(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`backend is down`))
	}))
	defer gateway.Close()

	platform := platformAPIFor(t, gateway.URL)
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/order"}`, true)

	// A relay that reached the gateway always answers 200. Mapping the
	// gateway's 502 onto the BFF's own status would leave the console unable to
	// tell "your backend is down" from "the portal could not reach it".
	assertStatus(t, res, http.StatusOK)
	result := decodeResult(t, res)
	if result.Response.Status != http.StatusBadGateway {
		t.Errorf("upstream status = %d, want 502", result.Response.Status)
	}
	if result.Response.Body != "backend is down" {
		t.Errorf("body = %q", result.Response.Body)
	}
}

func TestInvokeRequiresASession(t *testing.T) {
	platform := platformAPIFor(t, "https://gw.example.com")
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/"}`, false)
	assertStatus(t, res, http.StatusUnauthorized)
}

func TestInvokeRequiresTheCSRFHeader(t *testing.T) {
	platform := platformAPIFor(t, "https://gw.example.com")
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	req, _ := http.NewRequest(http.MethodPost, f.bff.URL+invokePath, strings.NewReader(`{}`))
	req.AddCookie(f.cookie)
	res, err := f.client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	// The endpoint is always POST precisely so this applies even when the
	// operation being tested is a GET.
	assertStatus(t, res, http.StatusForbidden)
}

func TestInvoke404sWhenTheRelayIsDisabled(t *testing.T) {
	platform := platformAPIFor(t, "https://gw.example.com")
	cfg := testConsoleConfig(platform.URL)
	cfg.TestConsole.Enabled = false
	f := newInvokeFixture(t, cfg)

	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/"}`, true)
	assertStatus(t, res, http.StatusNotFound)
}

func TestInvokeDoesNotLeakTheResolvedTarget(t *testing.T) {
	// A gateway record naming the cloud metadata endpoint is refused by the
	// dial-time guard, whose error names the address. The response must not.
	platform := platformAPIFor(t, "http://169.254.169.254")
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/latest/meta-data/"}`, true)
	assertStatus(t, res, http.StatusBadGateway)

	body, _ := io.ReadAll(res.Body)
	for _, leak := range []string{"169.254", "disallowed", "netguard", "meta-data"} {
		if strings.Contains(string(body), leak) {
			t.Errorf("response leaked %q, which maps internal topology for a caller: %s", leak, body)
		}
	}
}

func TestInvokeRefusesAGatewayTheAPIIsNotDeployedTo(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/gateways") {
			_, _ = w.Write([]byte(`{"list":[{"id":"gw-prod","endpoints":["https://gw.example.com"],"isDeployed":false}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"context":"/pizza"}`))
	}))
	defer platform.Close()

	f := newInvokeFixture(t, testConsoleConfig(platform.URL))
	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/"}`, true)

	assertStatus(t, res, http.StatusForbidden)
	if code := decodeError(t, res)["code"]; code != "TARGET_NOT_ALLOWED" {
		t.Errorf("code = %v, want TARGET_NOT_ALLOWED", code)
	}
}

func TestInvokeRejectsARequestShapeTheRelayRefuses(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the gateway must never be dialed for a request that failed validation")
		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	platform := platformAPIFor(t, gateway.URL)
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	for _, tc := range []struct{ name, body string }{
		{"traversal out of the API context", `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/../admin"}`},
		{"forbidden header", `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/","headers":[{"name":"Host","value":"evil.example.com"}]}`},
		{"header injection", `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/","headers":[{"name":"X-A","value":"a\r\nX-Evil: 1"}]}`},
		{"unsupported method", `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"CONNECT","path":"/"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := f.post(t, tc.body, true)
			assertStatus(t, res, http.StatusBadRequest)
			if code := decodeError(t, res)["code"]; code != "INVALID_TEST_REQUEST" {
				t.Errorf("code = %v, want INVALID_TEST_REQUEST", code)
			}
		})
	}
}

func TestInvokeRejectsAnOversizedEnvelope(t *testing.T) {
	platform := platformAPIFor(t, "https://gw.example.com")
	cfg := testConsoleConfig(platform.URL)
	cfg.TestConsole.MaxRequestBytes = 128
	f := newInvokeFixture(t, cfg)

	body := `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"POST","path":"/","body":"` +
		strings.Repeat("a", 500) + `"}`
	res := f.post(t, body, true)

	assertStatus(t, res, http.StatusRequestEntityTooLarge)
	if msg, _ := decodeError(t, res)["message"].(string); strings.Contains(msg, "128") {
		t.Errorf("the rejection echoed the configured limit back: %q", msg)
	}
}

func TestInvokeRejectsUnknownEnvelopeFields(t *testing.T) {
	// DisallowUnknownFields is what stops a future field (a caller-supplied
	// target URL, a per-request TLS override) from being silently accepted by
	// an older BFF that does not implement it.
	platform := platformAPIFor(t, "https://gw.example.com")
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/","targetUrl":"https://evil.example.com"}`, true)
	assertStatus(t, res, http.StatusBadRequest)
}

func TestInvokeResponseIsAlwaysInertJSON(t *testing.T) {
	// The gateway's own Content-Type and Set-Cookie must reach the browser as
	// JSON string data, never as response headers on the portal's origin.
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Set-Cookie", "evil=1; Path=/")
		_, _ = w.Write([]byte(`<script>alert(1)</script>`))
	}))
	defer gateway.Close()

	platform := platformAPIFor(t, gateway.URL)
	f := newInvokeFixture(t, testConsoleConfig(platform.URL))

	res := f.post(t, `{"orgHandle":"acme","restApiId":"api-1","gatewayId":"gw-prod","method":"GET","path":"/"}`, true)
	assertStatus(t, res, http.StatusOK)

	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if got := res.Header.Get("Set-Cookie"); got != "" {
		t.Errorf("a gateway set a cookie on the portal origin: %q", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	result := decodeResult(t, res)
	var sawContentType, sawSetCookie bool
	for _, h := range result.Response.Headers {
		switch http.CanonicalHeaderKey(h.Name) {
		case "Content-Type":
			sawContentType = true
		case "Set-Cookie":
			sawSetCookie = true
		}
	}
	if !sawContentType || !sawSetCookie {
		t.Error("the gateway's headers should still be visible to the tester, as data")
	}
}
