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
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"api-control-plane-bff/internal/config"
)

const testBasePath = "/console"

// newRoutedServer starts the BFF over a static test dir, with cfg.Server.BasePath
// as the caller left it.
func newRoutedServer(t *testing.T, cfg *config.Config) *httptest.Server {
	t.Helper()
	cfg.Server.StaticDir = newStaticTestDir(t)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	bff := httptest.NewServer(srv.Handler())
	t.Cleanup(bff.Close)
	return bff
}

func prefixedConfig(controlPlaneURL string) *config.Config {
	cfg := newTestConfig(controlPlaneURL)
	cfg.Server.BasePath = testBasePath
	return cfg
}

// noRedirectClient returns the first response as-is so redirects can be asserted.
func noRedirectClient(jar http.CookieJar) *http.Client {
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func assertRoute(t *testing.T, client *http.Client, base, path string, status int, contains string) {
	t.Helper()
	res, err := client.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body := readBody(t, res)
	if res.StatusCode != status {
		t.Errorf("GET %s status = %d, want %d", path, res.StatusCode, status)
	}
	if contains != "" && !strings.Contains(body, contains) {
		t.Errorf("GET %s body = %q, want it to contain %q", path, body, contains)
	}
}

// With the key unset the prefix means nothing: it is an ordinary SPA route, and the
// runtime config, cookies and frame header are what they were before the key existed.
func TestBasePathUnset_BehaviourUnchanged(t *testing.T) {
	bff := newRoutedServer(t, newTestConfig("https://unused.example.com"))
	client := noRedirectClient(nil)

	for _, tc := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/healthz", http.StatusOK, "ok"},
		{"/", http.StatusOK, indexBody},
		{"/projects/foo", http.StatusOK, indexBody},
		{"/assets/app.js", http.StatusOK, assetBody},
		{"/api-platform.env.config.js", http.StatusOK, "__RUNTIME_CONFIG__"},
		{"/api/session", http.StatusUnauthorized, "authenticated"},
		{"/proxy/api/v0.9/projects", http.StatusUnauthorized, "NOT_AUTHENTICATED"},
		{testBasePath + "/healthz", http.StatusOK, indexBody},
		{testBasePath + "/assets/app.js", http.StatusOK, indexBody},
		{testBasePath, http.StatusOK, indexBody},
	} {
		assertRoute(t, client, bff.URL, tc.path, tc.status, tc.contains)
	}

	res, err := client.Get(bff.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	_ = readBody(t, res)
	if got := res.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
}

// Every BFF route, the SPA and its assets answer under the prefix and at the root.
func TestBasePath_RoutesServedUnderPrefixAndRoot(t *testing.T) {
	bff := newRoutedServer(t, prefixedConfig("https://unused.example.com"))
	client := noRedirectClient(nil)

	for _, prefix := range []string{testBasePath, ""} {
		for _, tc := range []struct {
			path     string
			status   int
			contains string
		}{
			{"/healthz", http.StatusOK, "ok"},
			{"/", http.StatusOK, indexBody},
			{"/projects/foo/nested", http.StatusOK, indexBody},
			{"/assets/app.js", http.StatusOK, assetBody},
			{"/api-platform.env.config.js", http.StatusOK, "__RUNTIME_CONFIG__"},
			{"/api-platform.common.config.js", http.StatusOK, ""},
			{"/api/session", http.StatusUnauthorized, "authenticated"},
			{"/proxy/api/v0.9/projects", http.StatusUnauthorized, "NOT_AUTHENTICATED"},
		} {
			assertRoute(t, client, bff.URL, prefix+tc.path, tc.status, tc.contains)
		}
	}

	// A path that merely starts with the prefix is not under it.
	assertRoute(t, client, bff.URL, testBasePath+"x/assets/app.js", http.StatusOK, indexBody)

	res, err := client.Get(bff.URL + testBasePath)
	if err != nil {
		t.Fatalf("GET %s: %v", testBasePath, err)
	}
	_ = readBody(t, res)
	if res.StatusCode != http.StatusTemporaryRedirect || res.Header.Get("Location") != testBasePath+"/" {
		t.Errorf("GET %s = %d Location %q, want 307 to %s/", testBasePath, res.StatusCode, res.Header.Get("Location"), testBasePath)
	}
}

// The gateway relay moves with the other BFF routes. Disabled, its handler answers a
// JSON NOT_FOUND, which proves the route is mounted: POST on an unmounted path would
// reach the SPA instead.
func TestBasePath_GatewayRelayRoute(t *testing.T) {
	bff := newRoutedServer(t, prefixedConfig("https://unused.example.com"))
	for _, path := range []string{testBasePath + "/api/gateway-relay/invoke", "/api/gateway-relay/invoke"} {
		req, err := http.NewRequest(http.MethodPost, bff.URL+path, strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(config.CSRFHeaderName, "api-control-plane")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		if body := readBody(t, res); res.StatusCode != http.StatusNotFound || !strings.Contains(body, `"NOT_FOUND"`) {
			t.Errorf("POST %s = %d %q, want the handler's 404 NOT_FOUND", path, res.StatusCode, body)
		}
	}
}

func TestBasePath_RuntimeConfigScript(t *testing.T) {
	cfg := prefixedConfig("https://unused.example.com")
	cfg.RuntimeConfig = map[string]string{"appBasePath": testBasePath}
	bff := newRoutedServer(t, cfg)
	assertRoute(t, http.DefaultClient, bff.URL, testBasePath+"/api-platform.env.config.js", http.StatusOK, `"appBasePath":"/console"`)
}

// The proxy strips the whole mounted prefix, on both mounts and for named upstreams.
func TestBasePath_ProxyStripsComposedPrefix(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := prefixedConfig(upstream.URL)
	cfg.ControlPlane.Upstreams = []config.UpstreamConfig{{Name: "billing", URL: upstream.URL}}
	bff := newRoutedServer(t, cfg)
	token := makeJWT(map[string]any{"username": "admin", "exp": time.Now().Add(time.Hour).Unix()})

	for path, want := range map[string]string{
		testBasePath + "/proxy/api/v0.9/projects":    "/api/v0.9/projects",
		"/proxy/api/v0.9/projects":                   "/api/v0.9/projects",
		testBasePath + "/proxy/billing/organization": "/organization",
		"/proxy/billing/organization":                "/organization",
	} {
		req, err := http.NewRequest(http.MethodGet, bff.URL+path, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: "_test_session", Value: token})
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = readBody(t, res)
		assertStatus(t, res, http.StatusOK)
		if gotPath != want {
			t.Errorf("GET %s reached upstream at %q, want %q", path, gotPath, want)
		}
	}
}

// The strip happens before spaHandler's containment check, so the check still guards
// everything under the prefix.
func TestBasePath_StaticStripContainsTraversal(t *testing.T) {
	bff := newRoutedServer(t, prefixedConfig("https://unused.example.com"))
	for _, p := range []string{
		testBasePath + "/../secret.txt",
		testBasePath + "/../../secret.txt",
		testBasePath + "/assets/../../secret.txt",
		testBasePath + "/%2e%2e%2fsecret.txt",
		testBasePath + "/..%2f..%2fsecret.txt",
		testBasePath + "/assets/%2e%2e%2f%2e%2e%2fsecret.txt",
		testBasePath + "%2f..%2f..%2fsecret.txt",
	} {
		req, err := http.NewRequest(http.MethodGet, bff.URL, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		// Set the opaque form so the client sends the payload unnormalized.
		req.URL.Opaque = p
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		if body := readBody(t, res); strings.Contains(body, secretBody) {
			t.Fatalf("GET %s escaped staticDir (status %d)", p, res.StatusCode)
		}
	}

	// Past the mux, a strip that leaves a relative path meets the containment check.
	staticDir := newStaticTestDir(t)
	h := http.StripPrefix(testBasePath, spaHandler(staticDir))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.Path = testBasePath + "../secret.txt"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), secretBody) {
		t.Errorf("relative strip = %d %q, want a 404 from the containment check", rec.Code, rec.Body.String())
	}
}

func TestCookiePaths(t *testing.T) {
	cases := []struct {
		name, basePath, session, tx string
	}{
		{"root", "", "/", "/api/auth"},
		{"prefixed", testBasePath, testBasePath, testBasePath + "/api/auth"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newTestConfig("https://unused.example.com")
			cfg.Server.BasePath = tc.basePath
			srv, err := New(context.Background(), cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer srv.Close()

			writers := map[string]struct {
				write func(http.ResponseWriter)
				want  string
			}{
				"set session+refresh": {func(w http.ResponseWriter) {
					srv.setSessionCookie(w, "access", "refresh", time.Now().Add(time.Hour))
				}, tc.session},
				"set session, clear refresh": {func(w http.ResponseWriter) {
					srv.setSessionCookie(w, "access", "", time.Now().Add(time.Hour))
				}, tc.session},
				"clear session+refresh": {srv.clearSessionCookie, tc.session},
				"set tx":                {func(w http.ResponseWriter) { srv.setTxCookie(w, "tx") }, tc.tx},
				"clear tx":              {srv.clearTxCookie, tc.tx},
			}
			for name, wr := range writers {
				rec := httptest.NewRecorder()
				wr.write(rec)
				cookies := rec.Result().Cookies()
				if len(cookies) == 0 {
					t.Fatalf("%s wrote no cookies", name)
				}
				for _, c := range cookies {
					if c.Path != wr.want {
						t.Errorf("%s: cookie %s Path = %q, want %q", name, c.Name, c.Path, wr.want)
					}
				}
			}
		})
	}
}

// The browser only returns the tx cookie to the prefixed callback if its Path covers
// it; a real cookie jar plays the browser, so a wrong Path fails on state mismatch.
func TestBasePath_OIDCRoundTrip(t *testing.T) {
	var nonce string
	var srvURL string
	idpMux := http.NewServeMux()
	idpMux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 srvURL,
			"authorization_endpoint": srvURL + "/authorize",
			"token_endpoint":         srvURL + "/token",
		})
	})
	idpMux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  makeJWT(map[string]any{"username": "admin", "exp": time.Now().Add(time.Hour).Unix()}),
			"refresh_token": "refresh-1",
			"id_token":      makeJWT(map[string]any{"iss": srvURL, "aud": "client-1", "nonce": nonce}),
			"expires_in":    3600,
		})
	})
	idp := httptest.NewServer(idpMux)
	defer idp.Close()
	srvURL = idp.URL

	cfg := prefixedConfig("https://unused.example.com")
	cfg.Auth.Mode = "oidc"
	cfg.Auth.OIDC = config.OIDCConfig{
		Enabled: true, Discovery: true, Authority: idp.URL, Issuer: idp.URL,
		ClientID: "client-1", ClientSecret: "s3cret", ClientAuthMethod: "client_secret_post",
		RedirectURL: "http://bff.example.com" + testBasePath + "/api/auth/callback", Scopes: "openid",
	}
	bff := newRoutedServer(t, cfg)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := noRedirectClient(jar)

	res, err := client.Get(bff.URL + testBasePath + "/api/auth/login?return=" + url.QueryEscape(testBasePath+"/projects"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	_ = readBody(t, res)
	assertStatus(t, res, http.StatusFound)
	authz := mustParseURL(t, res.Header.Get("Location"))
	if got := authz.Query().Get("redirect_uri"); !strings.HasSuffix(got, testBasePath+"/api/auth/callback") {
		t.Errorf("redirect_uri = %q, want the prefixed callback", got)
	}
	nonce = authz.Query().Get("nonce")

	cb := bff.URL + testBasePath + "/api/auth/callback?code=c1&state=" + url.QueryEscape(authz.Query().Get("state"))
	res, err = client.Get(cb)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	_ = readBody(t, res)
	assertStatus(t, res, http.StatusFound)
	if loc := res.Header.Get("Location"); loc != testBasePath+"/projects" {
		t.Fatalf("callback Location = %q, want %s/projects", loc, testBasePath)
	}

	assertRoute(t, client, bff.URL, testBasePath+"/api/session", http.StatusOK, `"authenticated":true`)
	// The session stays with this console on a shared origin.
	if got := jar.Cookies(mustParseURL(t, bff.URL+"/other/")); len(got) != 0 {
		t.Errorf("cookies sent outside the prefix = %v, want none", got)
	}

	// IdP and exchange failures land on the prefixed login page.
	for path, want := range map[string]string{
		"/api/auth/callback?error=access_denied": "/login?error=access_denied",
		"/api/auth/callback?state=x&code=y":      "/login?error=auth_failed",
	} {
		res, err := client.Get(bff.URL + testBasePath + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = readBody(t, res)
		if loc := res.Header.Get("Location"); loc != testBasePath+want {
			t.Errorf("GET %s Location = %q, want %s%s", path, loc, testBasePath, want)
		}
	}
}

func TestSanitizeReturn(t *testing.T) {
	prefixed := &Server{cfg: &config.Config{Server: config.ServerConfig{BasePath: testBasePath}}}
	for in, want := range map[string]string{
		"":                          testBasePath + "/",
		"/":                         testBasePath + "/",
		testBasePath:                testBasePath,
		testBasePath + "/":          testBasePath + "/",
		testBasePath + "/a?x=1":     testBasePath + "/a?x=1",
		"/sibling/":                 testBasePath + "/",
		testBasePath + "x/evil":     testBasePath + "/",
		"//evil.com/console/":       testBasePath + "/",
		"https://evil.com/console/": testBasePath + "/",
		`/\evil.com`:                testBasePath + "/",
	} {
		if got := prefixed.sanitizeReturn(in); got != want {
			t.Errorf("prefixed sanitizeReturn(%q) = %q, want %q", in, got, want)
		}
	}

	root := &Server{cfg: &config.Config{}}
	for in, want := range map[string]string{
		"":            "/",
		"/sibling/":   "/sibling/",
		"//evil.com":  "/",
		"https://e.c": "/",
	} {
		if got := root.sanitizeReturn(in); got != want {
			t.Errorf("root sanitizeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestXFrameOptionsHeader(t *testing.T) {
	for _, want := range []string{config.XFrameDeny, config.XFrameSameOrigin} {
		cfg := newTestConfig("https://unused.example.com")
		cfg.Server.XFrameOptions = want
		bff := newRoutedServer(t, cfg)
		res, err := http.Get(bff.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		_ = readBody(t, res)
		if got := res.Header.Values("X-Frame-Options"); len(got) != 1 || got[0] != want {
			t.Errorf("X-Frame-Options = %v, want [%s]", got, want)
		}
		if got := res.Header.Get("Content-Security-Policy"); got != "frame-ancestors 'self'" {
			t.Errorf("Content-Security-Policy = %q", got)
		}
	}
}
