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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/proxy"
	"ai-workspace-bff/internal/session"
)

// exchangeTestHarness wires a Server with a token exchange pointed at a stub IDP and
// a stub Platform API, so a request can be driven end to end through handleProxy and
// the Authorization header that actually reached upstream can be asserted.
type exchangeTestHarness struct {
	server      *Server
	idpCalls    *atomic.Int32
	idpStatus   func() (int, any)
	upstreamGot *atomic.Value // last upstream Authorization header
	idpLastForm *atomic.Value // last exchange request's PostForm, as url.Values
	// idTokenNonce is the nonce the stub mints its id_token with, set by
	// callbackRequest so a real login transaction can pass the callback's nonce check.
	idTokenNonce *atomic.Value
}

// unsignedJWT builds a decodable (never verified) JWT. The BFF only ever decodes
// claims — the Platform API is what validates tokens — so a signature is not needed
// to exercise any of this.
func unsignedJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal claims: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]any{"alg": "none", "typ": "JWT"}) + "." + enc(claims) + ".sig"
}

func newExchangeHarness(t *testing.T, cfgMut func(*config.TokenExchangeConfig)) *exchangeTestHarness {
	t.Helper()

	h := &exchangeTestHarness{
		idpCalls:     &atomic.Int32{},
		upstreamGot:  &atomic.Value{},
		idpLastForm:  &atomic.Value{},
		idTokenNonce: &atomic.Value{},
	}
	h.idTokenNonce.Store("")
	h.upstreamGot.Store("")
	h.idpStatus = func() (int, any) {
		return http.StatusOK, map[string]any{
			"access_token":      "exchanged-token",
			"issued_token_type": auth.TokenTypeAccessToken,
			"token_type":        "Bearer",
			"expires_in":        3600,
			"scope":             "ap:project:read ap:gateway:read",
		}
	}

	// The stub IDP serves both the discovery document (so a real auth.OIDC client can
	// be constructed — token exchange only runs in OIDC mode, and handleSession's user
	// lookup takes a different branch without one) and the token endpoint.
	mux := http.NewServeMux()
	var idp *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 idp.URL,
			"authorization_endpoint": idp.URL + "/authorize",
			"token_endpoint":         idp.URL + "/token",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		// One endpoint, every grant. Only the exchange goes through h.idpStatus, and
		// only it is counted — idpCalls exists to count exchanges.
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") == "authorization_code" {
			nonce, _ := h.idTokenNonce.Load().(string)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "subject-token",
				"refresh_token": "refresh-token",
				// The callback rejects an id_token whose nonce does not match the
				// transaction it opened, so the stub has to echo this login's nonce.
				"id_token":   unsignedJWT(t, map[string]any{"nonce": nonce, "username": "alice"}),
				"token_type": "Bearer",
				"expires_in": 3600,
			})
			return
		}
		if r.PostForm.Get("grant_type") == "refresh_token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "rotated-subject-token",
				"refresh_token": "rotated-refresh-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
			return
		}
		h.idpCalls.Add(1)
		h.idpLastForm.Store(r.PostForm)
		status, body := h.idpStatus()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	})
	idp = httptest.NewServer(mux)
	t.Cleanup(idp.Close)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.upstreamGot.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}

	teCfg := config.TokenExchangeConfig{
		Enabled:            true,
		GrantType:          config.GrantTokenExchange,
		ClientID:           "c",
		ClientSecret:       "s",
		Audience:           "platform-api",
		Scopes:             "ap:project:read ap:gateway:read",
		SubjectTokenType:   auth.TokenTypeJWT,
		RequestedTokenType: auth.TokenTypeAccessToken,
		CacheEnabled:       true,
		MinValidity:        60 * time.Second,
	}
	if cfgMut != nil {
		cfgMut(&teCfg)
	}

	cfg := &config.Config{
		Cookie: config.CookieConfig{Name: "_ai_workspace_session", Secure: true, SameSite: "lax"},
		Auth: config.AuthConfig{
			Mode: config.AuthModeOIDC,
			OIDC: config.OIDCConfig{
				TokenExchange: teCfg,
			},
		},
		Session: config.SessionConfig{IdleTimeout: 30 * time.Minute, AbsoluteTTL: 8 * time.Hour},
	}

	oidcClient, err := auth.NewOIDC(
		context.Background(), idp.Client(),
		idp.URL, "c", "s",
		"https://localhost:9643"+paths.Base+"/api/auth/callback", "", "openid",
		session.DefaultClaimMapping(), 8*time.Hour,
	)
	if err != nil {
		t.Fatalf("build oidc client: %v", err)
	}
	t.Cleanup(oidcClient.Close)

	h.server = &Server{
		cfg:           cfg,
		claims:        session.DefaultClaimMapping(),
		store:         session.NewMemoryStore(),
		oidc:          oidcClient,
		proxy:         proxy.ReverseProxy(target, paths.Base+paths.Proxy, http.DefaultTransport),
		exchanger:     auth.NewExchanger(idp.Client(), teCfg, oidcClient.TokenEndpoint()),
		refreshLocks:  make(map[string]*refreshLock),
		exchangeLocks: make(map[string]*exchangeLock),
		sessionLocks:  make(map[string]*sessionLock),
	}
	t.Cleanup(func() { _ = h.server.store.Close() })
	return h
}

// subjectSession seeds a session whose subject token is far from expiry, so the
// refresh path in handleProxy is never taken and only the exchange is exercised.
func (h *exchangeTestHarness) subjectSession(t *testing.T) string {
	t.Helper()
	const subject = "subject-token"
	sess := &session.Session{
		ID:             subject,
		Mode:           session.ModeOIDC,
		AccessToken:    subject,
		RefreshToken:   "refresh-token",
		AccessExpiry:   time.Now().Add(time.Hour),
		AbsoluteExpiry: time.Now().Add(8 * time.Hour),
		User:           session.User{Name: "alice", Scopes: []string{"login-scope"}},
	}
	if err := h.server.store.Put(context.Background(), sess); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return subject
}

func (h *exchangeTestHarness) proxyRequest(subject string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, paths.Base+paths.Proxy+"/api/v0.9/projects", nil)
	req.AddCookie(&http.Cookie{Name: h.server.cfg.Cookie.Name, Value: subject})
	rec := httptest.NewRecorder()
	h.server.handleProxy(rec, req)
	return rec
}

// TestProxyForwardsExchangedToken is the core behaviour: what reaches the Platform
// API is the exchanged token, never the login token.
func TestProxyForwardsExchangedToken(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)

	rec := h.proxyRequest(subject)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := h.upstreamGot.Load().(string); got != "Bearer exchanged-token" {
		t.Errorf("upstream Authorization = %q, want the exchanged token", got)
	}
}

// TestProxyFailsClosedWhenExchangeRejected is the security-critical guarantee: a
// refused exchange must never fall back to forwarding the unexchanged login token,
// which would reach the Platform API with the wrong audience and — on an IDP that
// mints no ap:* scopes — no platform authorization at all (GO-AUTH-001).
func TestProxyFailsClosedWhenExchangeRejected(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)
	h.idpStatus = func() (int, any) {
		return http.StatusBadRequest, map[string]string{"error": "invalid_request"}
	}

	rec := h.proxyRequest(subject)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for a rejected exchange", rec.Code)
	}
	if got := h.upstreamGot.Load().(string); got != "" {
		t.Errorf("upstream was called with %q — a rejected exchange must not reach the Platform API", got)
	}
	// A rejection means this session can never produce an upstream token, so it is
	// cleared and the SPA is sent back through login.
	if _, ok, _ := h.server.store.Get(context.Background(), subject); ok {
		t.Error("a rejected session must be dropped from the store")
	}
}

// TestProxyFailsClosedWhenIDPUnavailable distinguishes a transient fault from a
// rejection: the user keeps their session and sees a 502, because logging them out
// over a token-endpoint blip is both wrong and self-inflicted.
func TestProxyFailsClosedWhenIDPUnavailable(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)
	h.idpStatus = func() (int, any) {
		return http.StatusServiceUnavailable, nil
	}

	rec := h.proxyRequest(subject)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 for an unavailable IDP", rec.Code)
	}
	if got := h.upstreamGot.Load().(string); got != "" {
		t.Errorf("upstream was called with %q — a failed exchange must not reach the Platform API", got)
	}
	if _, ok, _ := h.server.store.Get(context.Background(), subject); !ok {
		t.Error("a transient IDP failure must NOT destroy the session")
	}
}

// TestExchangeResponseLeaksNoIDPDetail: neither failure mode may tell the browser
// why the exchange failed — that maps out the deployment's trust configuration
// (error-handling.md directives 1 and 4).
func TestExchangeResponseLeaksNoIDPDetail(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   any
	}{
		{"rejected", http.StatusBadRequest, map[string]string{
			"error": "invalid_target", "error_description": "audience platform-api is not registered"}},
		{"unavailable", http.StatusInternalServerError, map[string]string{
			"error": "server_error", "error_description": "internal database failure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newExchangeHarness(t, nil)
			subject := h.subjectSession(t)
			h.idpStatus = func() (int, any) { return tc.status, tc.body }

			rec := h.proxyRequest(subject)
			body := rec.Body.String()
			for _, leak := range []string{"invalid_target", "server_error", "not registered", "database", "platform-api"} {
				if strings.Contains(body, leak) {
					t.Errorf("response body leaks IDP detail %q: %s", leak, body)
				}
			}
		})
	}
}

// TestExchangeIsCachedAcrossRequests: an exchange is a blocking round trip to the
// IDP, so repeating it per request would put the token endpoint in the path of every
// API call the SPA makes.
func TestExchangeIsCachedAcrossRequests(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)

	for i := 0; i < 3; i++ {
		if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}
	if got := h.idpCalls.Load(); got != 1 {
		t.Errorf("IDP called %d times for 3 requests, want 1 (cached)", got)
	}
}

// TestExchangeNotCachedWhenDisabled is the debugging escape hatch working as stated.
func TestExchangeNotCachedWhenDisabled(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.CacheEnabled = false })
	subject := h.subjectSession(t)

	for i := 0; i < 3; i++ {
		if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}
	if got := h.idpCalls.Load(); got != 3 {
		t.Errorf("IDP called %d times, want 3 with caching off", got)
	}
}

// TestExchangeNotCachedWithoutExpiry: a token whose lifetime is unknown cannot be
// checked for freshness, so reusing it risks forwarding an expired credential.
func TestExchangeNotCachedWithoutExpiry(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)
	h.idpStatus = func() (int, any) {
		return http.StatusOK, map[string]any{
			"access_token":      "opaque-token-no-exp",
			"issued_token_type": auth.TokenTypeAccessToken,
			"token_type":        "Bearer",
			// no expires_in, and the token is opaque so there is no exp claim either
		}
	}

	for i := 0; i < 2; i++ {
		if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}
	if got := h.idpCalls.Load(); got != 2 {
		t.Errorf("IDP called %d times, want 2 — a token with no known expiry must not be cached", got)
	}
}

// TestExchangeStaleFingerprintForcesReExchange: a cached token minted under a
// different audience/scope configuration must not be reused after a reconfiguration,
// or the change would be masked until every session expired.
func TestExchangeStaleFingerprintForcesReExchange(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)

	sess, ok, _ := h.server.store.Get(context.Background(), subject)
	if !ok {
		t.Fatal("seeded session missing")
	}
	sess.Exchanged = session.ExchangedToken{
		Token:             "token-from-old-config",
		Expiry:            time.Now().Add(time.Hour),
		Scopes:            []string{"ap:project:read"},
		ConfigFingerprint: "a-different-configuration",
	}
	if err := h.server.store.Put(context.Background(), sess); err != nil {
		t.Fatalf("put: %v", err)
	}

	if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := h.idpCalls.Load(); got != 1 {
		t.Errorf("IDP called %d times, want 1 — a stale fingerprint must force a re-exchange", got)
	}
	if got := h.upstreamGot.Load().(string); got != "Bearer exchanged-token" {
		t.Errorf("upstream Authorization = %q, want the freshly exchanged token", got)
	}
}

// TestExchangeSingleFlight: the SPA fires a burst of parallel calls on page load, and
// they must collapse into one exchange rather than one per request.
func TestExchangeSingleFlight(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)

	const parallel = 12
	var wg sync.WaitGroup
	wg.Add(parallel)
	for i := 0; i < parallel; i++ {
		go func() {
			defer wg.Done()
			h.proxyRequest(subject)
		}()
	}
	wg.Wait()

	if got := h.idpCalls.Load(); got != 1 {
		t.Errorf("IDP called %d times for %d parallel requests, want 1", got, parallel)
	}
	// The single-flight map must not retain an entry once the group completes.
	h.server.exchangeMu.Lock()
	remaining := len(h.server.exchangeLocks)
	h.server.exchangeMu.Unlock()
	if remaining != 0 {
		t.Errorf("exchangeLocks retained %d entries, want 0", remaining)
	}
}

// TestSessionReportsExchangedScopes is the payoff for a role-mode IDP: the UI gates
// on what the Platform API will authorize, which in exchange mode is decided by the
// exchanged token rather than the login token.
func TestSessionReportsExchangedScopes(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)

	req := httptest.NewRequest(http.MethodGet, paths.Base+"/api/session", nil)
	req.AddCookie(&http.Cookie{Name: h.server.cfg.Cookie.Name, Value: subject})
	rec := httptest.NewRecorder()
	h.server.handleSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		User struct {
			Scopes []string `json:"scopes"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.User.Scopes) != 2 {
		t.Fatalf("scopes = %v, want the two granted by the exchange", got.User.Scopes)
	}
	for _, s := range got.User.Scopes {
		if s == "login-scope" {
			t.Error("session reported the login token's scopes, not the exchanged token's")
		}
	}
}

// callbackRequest drives a real OIDC login: it opens a transaction through the live
// auth.OIDC client, teaches the stub IDP which nonce to mint the id_token with, and
// returns the callback request the browser would send back.
func (h *exchangeTestHarness) callbackRequest(t *testing.T) *http.Request {
	t.Helper()
	authURL, txID, err := h.server.oidc.AuthCodeURL(paths.Base + "/")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	h.idTokenNonce.Store(u.Query().Get("nonce"))

	req := httptest.NewRequest(http.MethodGet,
		paths.Base+"/api/auth/callback?code=auth-code&state="+url.QueryEscape(u.Query().Get("state")), nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: txID})
	return req
}

// sessionCookieValue returns the value the response set the session cookie to, or ""
// when it set none or cleared it. Clearing writes the cookie with an empty value, so
// "no usable session cookie" and "no cookie at all" are the same assertion.
func sessionCookieValue(h *exchangeTestHarness, rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == h.server.cfg.Cookie.Name && c.Value != "" && c.MaxAge >= 0 {
			return c.Value
		}
	}
	return ""
}

// TestLoginClassifiesExchangeFailure: the login path must tell its two failure classes
// apart, because "retrying will not help you" and "retrying in a moment probably will"
// are different things to say. Both still fail the login — a session that cannot
// produce an upstream token is not a usable one — so the difference is the reason
// carried to the login page, which renders them differently and, crucially, stops
// auto-restarting the login on either (AutoLoginPage would otherwise redirect straight
// back to the IDP and loop).
func TestLoginClassifiesExchangeFailure(t *testing.T) {
	login := func(t *testing.T, idp func() (int, any)) (*exchangeTestHarness, *httptest.ResponseRecorder) {
		t.Helper()
		h := newExchangeHarness(t, nil)
		if idp != nil {
			h.idpStatus = idp
		}
		rec := httptest.NewRecorder()
		h.server.handleOIDCCallback(rec, h.callbackRequest(t))
		return h, rec
	}
	assertFailedLogin := func(t *testing.T, h *exchangeTestHarness, rec *httptest.ResponseRecorder, wantReason string) {
		t.Helper()
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
		}
		want := paths.Base + "/login?error=" + wantReason
		if got := rec.Header().Get("Location"); got != want {
			t.Errorf("Location = %q, want %q", got, want)
		}
		if got := sessionCookieValue(h, rec); got != "" {
			t.Errorf("a failed login set a session cookie (%q)", got)
		}
		// The login is atomic: a session that never exchanged must not be left behind
		// for a later request to pick up.
		if _, ok, _ := h.server.store.Get(context.Background(), "subject-token"); ok {
			t.Error("a failed login left the session in the store")
		}
	}

	t.Run("rejected", func(t *testing.T) {
		h, rec := login(t, func() (int, any) {
			return http.StatusBadRequest, map[string]any{"error": "invalid_grant"}
		})
		assertFailedLogin(t, h, rec, "token_exchange_rejected")
	})

	t.Run("unavailable", func(t *testing.T) {
		h, rec := login(t, func() (int, any) {
			return http.StatusServiceUnavailable, map[string]any{"error": "server_error"}
		})
		assertFailedLogin(t, h, rec, "upstream_unavailable")
	})

	// The success path is asserted alongside so the two failures above are known to be
	// the exchange failing, not the login harness failing to log in at all.
	t.Run("success", func(t *testing.T) {
		h, rec := login(t, nil)
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Location"); got != paths.Base+"/" {
			t.Errorf("Location = %q, want the return URL %q", got, paths.Base+"/")
		}
		if got := sessionCookieValue(h, rec); got != "subject-token" {
			t.Errorf("session cookie = %q, want the login token", got)
		}
	})
}

// TestSessionFailsClosedWhenExchangeFails: hydration must never fall back to the login
// token's authority. Reporting it would offer the UI actions every proxied call then
// refuses — the substitution exchange mode exists to prevent, arriving at the identity
// layer instead of on the wire. The two classes are asserted together because the split
// is the whole behaviour: only a rejection may end the session.
func TestSessionFailsClosedWhenExchangeFails(t *testing.T) {
	sessionRequest := func(h *exchangeTestHarness, subject string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, paths.Base+"/api/session", nil)
		req.AddCookie(&http.Cookie{Name: h.server.cfg.Cookie.Name, Value: subject})
		rec := httptest.NewRecorder()
		h.server.handleSession(rec, req)
		return rec
	}
	// No assertion on the body beyond "it is not a hydrated session": the point is that
	// no scope set reaches the SPA at all, whichever class the failure fell into.
	assertNoScopesLeaked := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		if strings.Contains(rec.Body.String(), "login-scope") {
			t.Errorf("response carried the login token's scopes: %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"authenticated":true`) {
			t.Errorf("a failed exchange still reported the session as hydrated: %s", rec.Body.String())
		}
	}

	t.Run("rejection destroys the session", func(t *testing.T) {
		h := newExchangeHarness(t, nil)
		subject := h.subjectSession(t)
		h.idpStatus = func() (int, any) {
			return http.StatusBadRequest, map[string]any{"error": "invalid_grant"}
		}

		rec := sessionRequest(h, subject)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
		}
		assertNoScopesLeaked(t, rec)
		// A rejected subject token can never produce an upstream token, so the session
		// must go — exactly as it does on the proxy path.
		if _, ok, _ := h.server.store.Get(context.Background(), subject); ok {
			t.Error("a rejected exchange left the session in the store")
		}
	})

	t.Run("unavailable keeps the session", func(t *testing.T) {
		h := newExchangeHarness(t, nil)
		subject := h.subjectSession(t)
		h.idpStatus = func() (int, any) {
			return http.StatusServiceUnavailable, map[string]any{"error": "server_error"}
		}

		rec := sessionRequest(h, subject)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body: %s", rec.Code, rec.Body.String())
		}
		assertNoScopesLeaked(t, rec)
		// The IDP may recover; logging the user out over a blip would be self-inflicted,
		// and the login they would then attempt fails the same way.
		if _, ok, _ := h.server.store.Get(context.Background(), subject); !ok {
			t.Error("a transient IDP failure destroyed the session")
		}
	})
}

// TestUpstreamTokenPassthroughWithoutExchanger: with no exchange configured the login
// token is forwarded unchanged, so an existing deployment sees no behavior change.
func TestUpstreamTokenPassthroughWithoutExchanger(t *testing.T) {
	s := &Server{
		cfg:           &config.Config{},
		exchangeLocks: make(map[string]*exchangeLock),
	}
	got, err := s.upstreamToken(context.Background(), "login-token")
	if err != nil {
		t.Fatalf("upstreamToken: %v", err)
	}
	if got != "login-token" {
		t.Errorf("upstreamToken = %q, want the login token unchanged", got)
	}
}

// TestRefreshDropsExchangedToken: the exchanged token was derived from the subject
// token that just rotated, so it must not survive a refresh — otherwise a derived
// credential outlives the one it was minted from.
func TestRefreshDropsExchangedToken(t *testing.T) {
	h := newExchangeHarness(t, nil)
	subject := h.subjectSession(t)

	// Prime the cache.
	if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
		t.Fatalf("priming request: status %d", rec.Code)
	}
	sess, ok, _ := h.server.store.Get(context.Background(), subject)
	if !ok || sess.Exchanged.Token == "" {
		t.Fatal("expected a cached exchanged token after the first request")
	}

	// Drive the real rotation; a hand-built literal would pass even if doRefresh
	// copied Exchanged forward, which is the regression this guards.
	sess.AccessExpiry = time.Now().Add(10 * time.Second) // inside the renewal window
	if err := h.server.store.Put(context.Background(), sess); err != nil {
		t.Fatalf("store session: %v", err)
	}

	rotated, err := h.server.refreshByToken(context.Background(), subject)
	if err != nil {
		t.Fatalf("refreshByToken: %v", err)
	}
	if rotated.AccessToken != "rotated-subject-token" {
		t.Fatalf("access token = %q, want the IDP's rotated subject token — "+
			"the refresh branch never ran", rotated.AccessToken)
	}
	if rotated.Exchanged.Token != "" {
		t.Errorf("a rotated session carried the previous exchanged token %q", rotated.Exchanged.Token)
	}
	if rotated.Exchanged.Usable(time.Now(), time.Minute, h.server.exchanger.ConfigFingerprint(), "") {
		t.Error("a zero ExchangedToken must never report itself usable")
	}

	// The rotated record is what a later request reads.
	stored, ok, _ := h.server.store.Get(context.Background(), rotated.AccessToken)
	if !ok {
		t.Fatal("rotated session was not re-keyed to the new access token")
	}
	if stored.Exchanged.Token != "" {
		t.Errorf("stored rotated session carried the previous exchanged token %q", stored.Exchanged.Token)
	}
}

// TestRefreshPreservesSelectedOrg is the counterpart to the test above, and the
// distinction between them is the whole point: Exchanged is DERIVED from the token
// that just rotated, so it must be dropped; OrgHandle is the user's own SELECTION,
// so it must survive. Lost, the session silently falls back to default_org and every
// later call is scoped to a different org than the one the UI still shows — and the
// refresh that causes it fires once per token lifetime for any active session.
func TestRefreshPreservesSelectedOrg(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.OrgParam = "orgHandle" })
	subject := h.subjectSession(t)

	if rec := h.switchOrgRequest(subject, "org-b"); rec.Code != http.StatusOK {
		t.Fatalf("switch to org-b: status %d; body: %s", rec.Code, rec.Body.String())
	}

	// Drive the real rotation rather than hand-building the rotated record, so this
	// asserts what doRefresh actually carries forward.
	sess, ok, _ := h.server.store.Get(context.Background(), subject)
	if !ok {
		t.Fatal("session missing after the switch")
	}
	sess.AccessExpiry = time.Now().Add(10 * time.Second) // inside the renewal window
	if err := h.server.store.Put(context.Background(), sess); err != nil {
		t.Fatalf("store session: %v", err)
	}

	rotated, err := h.server.refreshByToken(context.Background(), subject)
	if err != nil {
		t.Fatalf("refreshByToken: %v", err)
	}
	if rotated.OrgHandle != "org-b" {
		t.Errorf("rotated session OrgHandle = %q, want %q", rotated.OrgHandle, "org-b")
	}
	stored, ok, _ := h.server.store.Get(context.Background(), rotated.AccessToken)
	if !ok {
		t.Fatal("rotated session was not re-keyed to the new access token")
	}
	if stored.OrgHandle != "org-b" {
		t.Errorf("stored rotated session OrgHandle = %q, want %q", stored.OrgHandle, "org-b")
	}

	// The consequence, not just the field. Rotation drops Exchanged, so this request
	// genuinely re-exchanges — and the org it names is what the Platform API will
	// scope the caller to.
	if rec := h.proxyRequest(rotated.AccessToken); rec.Code != http.StatusOK {
		t.Fatalf("proxy after refresh: status %d; body: %s", rec.Code, rec.Body.String())
	}
	form := h.idpLastForm.Load()
	if form == nil {
		t.Fatal("IDP was never called")
	}
	if got := form.(interface{ Get(string) string }).Get("orgHandle"); got != "org-b" {
		t.Errorf("the exchange after a refresh sent orgHandle = %q, want %q — "+
			"the user's org selection did not survive the token rotation", got, "org-b")
	}
}

// TestExchangedTokenUsable pins the cache-validity rules, each of which exists to
// stop a specific unsafe reuse.
func TestExchangedTokenUsable(t *testing.T) {
	const fp = "fingerprint"
	now := time.Now()

	for _, tc := range []struct {
		name      string
		tok       session.ExchangedToken
		orgHandle string
		want      bool
	}{
		{"fresh", session.ExchangedToken{
			Token: "t", Expiry: now.Add(time.Hour), ConfigFingerprint: fp}, "", true},
		{"empty token", session.ExchangedToken{
			Expiry: now.Add(time.Hour), ConfigFingerprint: fp}, "", false},
		{"unknown expiry", session.ExchangedToken{
			Token: "t", ConfigFingerprint: fp}, "", false},
		{"already expired", session.ExchangedToken{
			Token: "t", Expiry: now.Add(-time.Minute), ConfigFingerprint: fp}, "", false},
		{"inside the renewal window", session.ExchangedToken{
			Token: "t", Expiry: now.Add(30 * time.Second), ConfigFingerprint: fp}, "", false},
		{"stale fingerprint", session.ExchangedToken{
			Token: "t", Expiry: now.Add(time.Hour), ConfigFingerprint: "other"}, "", false},
		{"org match", session.ExchangedToken{
			Token: "t", Expiry: now.Add(time.Hour), ConfigFingerprint: fp, OrgHandle: "org-a"}, "org-a", true},
		{"org mismatch", session.ExchangedToken{
			Token: "t", Expiry: now.Add(time.Hour), ConfigFingerprint: fp, OrgHandle: "org-a"}, "org-b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tok.Usable(now, time.Minute, fp, tc.orgHandle); got != tc.want {
				t.Errorf("Usable = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSessionReportsExchangedScopesWithCachingDisabled is the concrete case the
// "use this request's own result" change fixes.
//
// With cache_enabled = false, doExchange never writes Exchanged onto the stored
// session — there is nothing to cache. Reading the scopes back out of the store
// therefore found an empty ExchangedToken and fell through to the LOGIN token's
// scopes, so the UI gated on permissions the Platform API does not use. The same
// path is reachable with caching ON, since the store write is best-effort and only
// logs on failure.
func TestSessionReportsExchangedScopesWithCachingDisabled(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.CacheEnabled = false })
	subject := h.subjectSession(t)

	req := httptest.NewRequest(http.MethodGet, paths.Base+"/api/session", nil)
	req.AddCookie(&http.Cookie{Name: h.server.cfg.Cookie.Name, Value: subject})
	rec := httptest.NewRecorder()
	h.server.handleSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		User struct {
			Scopes []string `json:"scopes"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, s := range got.User.Scopes {
		if s == "login-scope" {
			t.Fatalf("scopes = %v — reported the login token's scopes, which the Platform API "+
				"does not authorize against in exchange mode", got.User.Scopes)
		}
	}
	if len(got.User.Scopes) != 2 {
		t.Errorf("scopes = %v, want the two granted by the exchange", got.User.Scopes)
	}
}

// TestExchangedOrgReplacesLoginOrg is the property the whole exchange rests on for
// org-scoped deployments: the org the UI reports must be the org the Platform API
// will scope requests to, and that is the EXCHANGED token's org. The login token
// names the IDP's own tenant ("login-tenant"), which the STS resolves to a platform
// org ("org-a") — reporting the login one would show a user an org they are not, in
// fact, operating in.
func TestExchangedOrgReplacesLoginOrg(t *testing.T) {
	loginUser := session.User{
		Name:          "Alice",
		Scopes:        []string{"openid"},
		Org:           &session.Org{ID: "login-tenant-uuid", Name: "Login Tenant", Handle: "login-tenant"},
		Organizations: []string{"login-tenant-uuid"},
	}
	exchanged := &auth.Result{
		Scopes: []string{"ap:project:manage"},
		Org:    &session.Org{ID: "org-a-uuid", Name: "org-a", Handle: "org-a"},
	}

	s := &Server{exchanger: &auth.Exchanger{}}
	got := s.withExchangedIdentity(loginUser, exchanged, session.ExchangedToken{})

	if got.Org == nil || got.Org.Handle != "org-a" {
		t.Fatalf("Org = %+v, want the exchanged token's org", got.Org)
	}
	// The org LIST is the login token's and stays put: belonging to an org is a
	// property of the user, not of the token minted for one of them.
	if len(got.Organizations) != 1 || got.Organizations[0] != "login-tenant-uuid" {
		t.Errorf("Organizations = %v, want the login token's list untouched", got.Organizations)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "ap:project:manage" {
		t.Errorf("Scopes = %v", got.Scopes)
	}
	// Everything the issued token says nothing about stays as the login token had it.
	if got.Name != "Alice" {
		t.Errorf("Name = %q — only org and scopes come from the exchange", got.Name)
	}
}

// A cache hit must report the same org a fresh exchange would, or the UI would show
// the login token's org for the cached token's whole lifetime.
func TestExchangedOrgFromCache(t *testing.T) {
	loginUser := session.User{Org: &session.Org{Handle: "login-tenant"}}
	cached := session.ExchangedToken{
		Token:  "cached-token",
		Scopes: []string{"ap:project:manage"},
		Org:    &session.Org{ID: "org-a-uuid", Handle: "org-a"},
	}

	s := &Server{exchanger: &auth.Exchanger{}}
	got := s.withExchangedIdentity(loginUser, nil, cached)

	if got.Org == nil || got.Org.Handle != "org-a" {
		t.Errorf("Org = %+v, want the cached exchange's org", got.Org)
	}
}

// An STS that passes org claims through untouched (or a deployment with no org
// mapping) reports no org on the exchange. That is "no information", not "no org" —
// blanking it would empty the org switcher for a perfectly valid session.
func TestExchangedOrgAbsentKeepsLoginOrg(t *testing.T) {
	loginUser := session.User{
		Org:           &session.Org{Handle: "login-tenant"},
		Organizations: []string{"login-tenant-uuid"},
	}
	s := &Server{exchanger: &auth.Exchanger{}}

	got := s.withExchangedIdentity(loginUser, &auth.Result{Scopes: []string{"ap:project:read"}}, session.ExchangedToken{})

	if got.Org == nil || got.Org.Handle != "login-tenant" {
		t.Errorf("Org = %+v, want the login org kept when the issued token carries none", got.Org)
	}
}

// TestDefaultOrgChangeInvalidatesCache backs the DefaultOrg fingerprint exemption:
// the configured default is resolved into the org actually requested, and Usable
// compares that against the org the cached token was minted for. Changing the
// default therefore invalidates the cache on its own — a token minted for one org
// must never be forwarded while the deployment now asks for another.
func TestDefaultOrgChangeInvalidatesCache(t *testing.T) {
	cached := session.ExchangedToken{
		Token:             "token-for-org-a",
		Expiry:            time.Now().Add(time.Hour),
		ConfigFingerprint: "fp",
		OrgHandle:         "org-a",
	}
	if !cached.Usable(time.Now(), time.Minute, "fp", "org-a") {
		t.Fatal("token must be usable for the org it was minted for")
	}
	if cached.Usable(time.Now(), time.Minute, "fp", "org-b") {
		t.Error("a token minted for org-a must not be reused once org-b is requested")
	}
}

// TestDefaultOrgSentBeforeAnySwitch: the configured default is what the exchange
// carries from the very first request, so the org the workspace operates in is the
// one stated in config rather than whichever org the STS happens to treat as the
// caller's default. Before this, no org parameter was sent at all until the user
// switched org — which for a single-org deployment was never.
func TestDefaultOrgSentBeforeAnySwitch(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) {
		c.OrgParam = "orgHandle"
		c.DefaultOrg = "org-a"
	})
	subject := h.subjectSession(t)

	if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
		t.Fatalf("proxy request: status %d", rec.Code)
	}

	form, _ := h.idpLastForm.Load().(url.Values)
	if got := form.Get("orgHandle"); got != "org-a" {
		t.Errorf("orgHandle = %q, want the configured default on the first exchange", got)
	}
}

// A user's own switch wins over the configured default — the default only fills the
// gap before one has been made.
func TestSwitchedOrgOverridesDefaultOrg(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) {
		c.OrgParam = "orgHandle"
		c.DefaultOrg = "org-a"
	})
	subject := h.subjectSession(t)

	sess, ok, _ := h.server.store.Get(context.Background(), subject)
	if !ok {
		t.Fatal("session not found")
	}
	sess.OrgHandle = "org-b"
	if err := h.server.store.Put(context.Background(), sess); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
		t.Fatalf("proxy request: status %d", rec.Code)
	}
	form, _ := h.idpLastForm.Load().(url.Values)
	if got := form.Get("orgHandle"); got != "org-b" {
		t.Errorf("orgHandle = %q, want the switched org to win over the default", got)
	}
}

// With no default configured and no switch made, nothing is sent — the behaviour
// every existing deployment has today.
func TestNoOrgParamWithoutDefaultOrSwitch(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) {
		c.OrgParam = "orgHandle"
	})
	subject := h.subjectSession(t)

	if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
		t.Fatalf("proxy request: status %d", rec.Code)
	}
	form, _ := h.idpLastForm.Load().(url.Values)
	if _, present := form["orgHandle"]; present {
		t.Errorf("orgHandle must be absent with no default and no switch, got %q", form.Get("orgHandle"))
	}
}
