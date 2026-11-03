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
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"api-control-plane-bff/internal/config"
)

// TestRootDeploymentContract pins, byte for byte, what a deployment with no base path
// serves: bodies, redirects, security headers, cookies and the runtime config. It
// depends only on helpers older than the base path, and never names the key, so the
// same file passes against the code from before the key existed.
func TestRootDeploymentContract(t *testing.T) {
	var loginStatus int
	token := makeJWT(map[string]any{"username": "admin"})
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/login") {
			loginStatus = http.StatusOK
			json.NewEncoder(w).Encode(map[string]any{
				"token":      token,
				"expires_at": time.Now().Add(time.Hour).Unix(),
			})
			return
		}
		w.Header().Set("X-Upstream-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer platform.Close()

	file := filepath.Join(t.TempDir(), "config.toml")
	body := `
[api_control_plane.server.http]
enabled = true
port = 9780
[api_control_plane.server.https]
enabled = false
[api_control_plane]
gateway_versions = []
[api_control_plane.policy_hub]
base_url = "https://hub.example.com"
[api_control_plane.session.cookie]
name = "_test_session"
secure = false
[api_control_plane.control_plane]
url = "` + platform.URL + `"
`
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Server.StaticDir = newStaticTestDir(t)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	maxAge := regexp.MustCompile(`Max-Age=[1-9][0-9]*`)
	send := func(method, path, reqBody string) (*http.Response, string) {
		t.Helper()
		req, err := http.NewRequest(method, bff.URL+path, strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(config.CSRFHeaderName, "api-control-plane")
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return res, string(b)
	}
	const runtimeConfig = `window.__RUNTIME_CONFIG__ = {"POLICY_HUB_BASE_URL":"https://hub.example.com",` +
		`"authMode":"basic","platformApiBaseUrl":"/proxy"};` + "\n"
	const clearedCookies = "_test_session=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax|" +
		"_test_refresh=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax"

	for _, tc := range []struct {
		method, path, reqBody string
		status                int
		body                  string
		location              string
		cookies               string
	}{
		{method: http.MethodGet, path: "/healthz", status: 200, body: `{"status":"ok"}` + "\n"},
		{method: http.MethodGet, path: "/", status: 200, body: indexBody},
		{method: http.MethodGet, path: "/projects/foo", status: 200, body: indexBody},
		{method: http.MethodGet, path: "/assets/app.js", status: 200, body: assetBody},
		// With no base path configured, a prefix is just another SPA route.
		{method: http.MethodGet, path: "/console", status: 200, body: indexBody},
		{method: http.MethodGet, path: "/console/healthz", status: 200, body: indexBody},
		{method: http.MethodGet, path: "/console/assets/app.js", status: 200, body: indexBody},
		{method: http.MethodGet, path: "/console/proxy/x", status: 200, body: indexBody},
		{method: http.MethodGet, path: "/api-platform.env.config.js", status: 200, body: runtimeConfig},
		{method: http.MethodGet, path: "/api-platform.common.config.js", status: 200},
		{method: http.MethodGet, path: "/api/session", status: 401, body: `{"authenticated":false}` + "\n"},
		{method: http.MethodGet, path: "/proxy/api/v0.9/projects", status: 401,
			body: `{"status":"error","code":"NOT_AUTHENTICATED","message":"not authenticated"}` + "\n"},
		{method: http.MethodGet, path: "/api/auth/login", status: 400,
			body: `{"status":"error","code":"AUTH_METHOD_DISABLED","message":"oidc auth is not enabled"}` + "\n"},
		{method: http.MethodPost, path: "/api/login", reqBody: `{"username":"admin","password":"test-value"}`, status: 200,
			body: `{"user":{"name":"admin","email":"","scopes":[]}}` + "\n",
			cookies: "_test_session=" + token + "; Path=/; Max-Age=N; HttpOnly; SameSite=Lax|" +
				"_test_refresh=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax"},
		{method: http.MethodPost, path: "/api/logout", status: 204, cookies: clearedCookies},
	} {
		res, got := send(tc.method, tc.path, tc.reqBody)
		if res.StatusCode != tc.status {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.path, res.StatusCode, tc.status)
		}
		if got != tc.body {
			t.Errorf("%s %s body = %q, want %q", tc.method, tc.path, got, tc.body)
		}
		if loc := res.Header.Get("Location"); loc != tc.location {
			t.Errorf("%s %s Location = %q, want %q", tc.method, tc.path, loc, tc.location)
		}
		var cookies []string
		for _, c := range res.Header.Values("Set-Cookie") {
			cookies = append(cookies, maxAge.ReplaceAllString(c, "Max-Age=N"))
		}
		if joined := strings.Join(cookies, "|"); joined != tc.cookies {
			t.Errorf("%s %s Set-Cookie = %q, want %q", tc.method, tc.path, joined, tc.cookies)
		}
		for name, want := range map[string]string{
			"X-Frame-Options":                   "DENY",
			"Content-Security-Policy":           "frame-ancestors 'self'",
			"X-Content-Type-Options":            "nosniff",
			"Referrer-Policy":                   "no-referrer",
			"X-Permitted-Cross-Domain-Policies": "none",
		} {
			if got := res.Header.Values(name); !slices.Equal(got, []string{want}) {
				t.Errorf("%s %s %s = %v, want [%s]", tc.method, tc.path, name, got, want)
			}
		}
	}
	if loginStatus != http.StatusOK {
		t.Error("login never reached the platform API")
	}

	// OIDC redirects and the transaction cookie.
	idp := newMockIDP(t)
	defer idp.Close()
	oidcCfg := newTestConfig("https://unused.example.com")
	oidcCfg.Auth.Mode = "oidc"
	oidcCfg.Auth.OIDC = config.OIDCConfig{
		Enabled: true, Discovery: true, Authority: idp.URL, Issuer: idp.URL,
		ClientID: "client-1", ClientSecret: "s3cret", ClientAuthMethod: "client_secret_post",
		RedirectURL: "http://bff.example.com/api/auth/callback", Scopes: "openid",
	}
	oidcSrv, err := New(context.Background(), oidcCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer oidcSrv.Close()
	oidcBFF := httptest.NewServer(oidcSrv.Handler())
	defer oidcBFF.Close()
	get := func(path string) *http.Response {
		t.Helper()
		res, err := client.Get(oidcBFF.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = res.Body.Close()
		return res
	}
	res := get("/api/auth/login?return=/dashboard")
	if !strings.HasPrefix(res.Header.Get("Location"), idp.URL+"/authorize") {
		t.Errorf("login Location = %q", res.Header.Get("Location"))
	}
	txCookie := regexp.MustCompile(`^_bff_oidc_tx=[^;]*;`).ReplaceAllString(res.Header.Get("Set-Cookie"), "_bff_oidc_tx=X;")
	if txCookie != "_bff_oidc_tx=X; Path=/api/auth; Max-Age=600; HttpOnly; SameSite=Lax" {
		t.Errorf("tx Set-Cookie = %q", txCookie)
	}
	for path, want := range map[string]string{
		"/api/auth/callback?error=access_denied": "/login?error=access_denied",
		"/api/auth/callback?state=x&code=y":      "/login?error=auth_failed",
	} {
		if loc := get(path).Header.Get("Location"); loc != want {
			t.Errorf("GET %s Location = %q, want %q", path, loc, want)
		}
	}
}
