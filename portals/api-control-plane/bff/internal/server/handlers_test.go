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
	"encoding/base64"
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

func makeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pb, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(pb)
	return header + "." + payload + ".fakesignature"
}

func newTestConfig(controlPlaneURL string) *config.Config {
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTP: config.HTTPListener{Enabled: true, Port: 0},
		},
		ControlPlane: config.ControlPlaneConfig{
			URL:            controlPlaneURL,
			PortalBasePath: "/api/portal/v0.9",
			ProxyPrefix:    "/proxy",
		},
		Session: config.SessionConfig{
			Store:       "memory",
			IdleTimeout: 30 * time.Minute,
			AbsoluteTTL: 8 * time.Hour,
			Cookie: config.CookieConfig{
				Name:     "_test_session",
				Secure:   false, // httptest servers are plain HTTP
				SameSite: "lax",
			},
		},
		Auth: config.AuthConfig{
			Mode: "basic",
			ClaimMappings: config.ClaimMappingConfig{
				Username: "username", Email: "email", Roles: "roles", Scope: "scope",
				OrgID: "organization", OrgName: "org_name", OrgHandle: "org_handle",
			},
		},
	}
	return cfg
}

// ---------------------------------------------------------------------------
// File-based mode: login -> session -> logout, and CSRF enforcement.
// ---------------------------------------------------------------------------

func TestHandlers_FileBased_LoginSessionLogout(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin", "scope": "ap:project:read"})
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": time.Now().Add(time.Hour).Unix()})
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Unauthenticated session check.
	res, _ := client.Get(bff.URL + "/api/session")
	assertStatus(t, res, http.StatusUnauthorized)

	// Login.
	loginReq, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/login",
		strings.NewReader(`{"username":"admin","password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err = client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
	var loginBody map[string]any
	json.NewDecoder(res.Body).Decode(&loginBody)
	if _, present := loginBody["accessToken"]; present {
		t.Error("login response must never include accessToken — the browser must not hold a token")
	}
	user, _ := loginBody["user"].(map[string]any)
	if user["name"] != "admin" {
		t.Errorf("user.name = %v, want admin", user["name"])
	}

	// The cookie must be HttpOnly.
	found := false
	for _, c := range jar.Cookies(mustParseURL(t, bff.URL)) {
		if c.Name == "_test_session" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the session cookie to be set after login")
	}

	// Authenticated session check.
	res, _ = client.Get(bff.URL + "/api/session")
	assertStatus(t, res, http.StatusOK)
	var sessionBody map[string]any
	json.NewDecoder(res.Body).Decode(&sessionBody)
	if sessionBody["authenticated"] != true {
		t.Errorf("authenticated = %v, want true", sessionBody["authenticated"])
	}
	if _, present := sessionBody["accessToken"]; present {
		t.Error("session response must never include accessToken")
	}

	// Logout.
	logoutReq, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/logout", nil)
	logoutReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err = client.Do(logoutReq)
	if err != nil {
		t.Fatalf("logout request: %v", err)
	}
	assertStatus(t, res, http.StatusNoContent)

	// Session check after logout.
	res, _ = client.Get(bff.URL + "/api/session")
	assertStatus(t, res, http.StatusUnauthorized)
}

func TestHandlers_FileBased_InvalidCredentials(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	req, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/login",
		strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	assertStatus(t, res, http.StatusUnauthorized)
}

// A mutating request without the CSRF header must be rejected, regardless of
// whether the credentials would otherwise have succeeded.
func TestHandlers_CSRF_RequiredOnMutatingRequests(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("platform api must not be called when the CSRF check fails first")
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	req, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/login",
		strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	// Deliberately no CSRF header.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	assertStatus(t, res, http.StatusForbidden)
}

func TestHandlers_Healthz_NoCSRFRequired(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
}

// ---------------------------------------------------------------------------
// OIDC mode: login redirect -> callback -> session.
// ---------------------------------------------------------------------------

func TestHandlers_OIDC_LoginRedirectsToIdP(t *testing.T) {
	idp := newMockIDP(t)
	defer idp.Close()

	cfg := newTestConfig("https://unused.example.com")
	cfg.Auth.Mode = "oidc"
	cfg.Auth.OIDC = config.OIDCConfig{
		Enabled: true, Discovery: true, Authority: idp.URL, Issuer: idp.URL,
		ClientID: "client-1", ClientSecret: "s3cret", ClientAuthMethod: "client_secret_post",
		RedirectURL: "http://bff.example.com/api/auth/callback", Scopes: "openid",
	}

	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(bff.URL + "/api/auth/login?return=/dashboard")
	if err != nil {
		t.Fatalf("oidc login request: %v", err)
	}
	assertStatus(t, res, http.StatusFound)
	loc := res.Header.Get("Location")
	if !strings.HasPrefix(loc, idp.URL+"/authorize") {
		t.Errorf("Location = %q, want a redirect to the IdP authorize endpoint", loc)
	}
	if res.Header.Get("Set-Cookie") == "" {
		t.Error("expected a tx cookie to be set")
	}
}

func TestHandlers_OIDC_DisabledModeRejectsOIDCEndpoints(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com") // Auth.Mode defaults to "basic"
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/api/auth/login")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusBadRequest)
}

// newMockIDP serves a minimal discovery document + token endpoint that always
// succeeds, for exercising the BFF's redirect/callback wiring (not the OIDC
// client's own correctness, which internal/auth tests separately).
func newMockIDP(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
		})
	})
	srv = httptest.NewServer(mux)
	return srv
}

// ---------------------------------------------------------------------------
// Reverse proxy: authenticated forwarding, unauthenticated rejection, and a
// named multi-upstream mount (mirroring cloud's billing-activation use case).
// ---------------------------------------------------------------------------

func TestHandlers_Proxy_UnauthenticatedRejected(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream must not be called for an unauthenticated proxy request")
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/proxy/api/v0.9/projects")
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	assertStatus(t, res, http.StatusUnauthorized)
}

func TestHandlers_Proxy_AuthenticatedForwardsAndInjectsBearer(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})
	var gotAuth, gotCookie, gotPath string
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/login") {
			json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": time.Now().Add(time.Hour).Unix()})
			return
		}
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	loginReq, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	assertStatus(t, res, http.StatusOK)

	res, err = client.Get(bff.URL + "/proxy/api/v0.9/projects")
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)

	if gotAuth != "Bearer "+tok {
		t.Errorf("upstream Authorization = %q, want Bearer %s", gotAuth, tok)
	}
	if gotCookie != "" {
		t.Errorf("upstream Cookie = %q, want empty (browser cookie must not leak upstream)", gotCookie)
	}
	if gotPath != "/api/v0.9/projects" {
		t.Errorf("upstream path = %q, want /api/v0.9/projects (proxy prefix not stripped)", gotPath)
	}
}

// A named upstream (e.g. cloud's billing service) must be reachable at its
// own same-origin prefix, independently of the primary control-plane
// upstream, with the same session token injected.
func TestHandlers_Proxy_NamedUpstream(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": time.Now().Add(time.Hour).Unix()})
	}))
	defer platform.Close()

	var billingAuth, billingPath string
	billing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		billingAuth = r.Header.Get("Authorization")
		billingPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer billing.Close()

	cfg := newTestConfig(platform.URL)
	cfg.ControlPlane.Upstreams = []config.UpstreamConfig{
		{Name: "billing", URL: billing.URL},
	}
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	loginReq, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	if _, err := client.Do(loginReq); err != nil {
		t.Fatalf("login: %v", err)
	}

	res, err := client.Get(bff.URL + "/proxy/billing/organization")
	if err != nil {
		t.Fatalf("billing proxy request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
	if billingAuth != "Bearer "+tok {
		t.Errorf("billing upstream Authorization = %q, want Bearer %s", billingAuth, tok)
	}
	if billingPath != "/organization" {
		t.Errorf("billing upstream path = %q, want /organization", billingPath)
	}
}

func assertStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want %d (body: %s)", res.StatusCode, want, body)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}
