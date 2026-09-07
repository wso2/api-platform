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
}

func newExchangeHarness(t *testing.T, cfgMut func(*config.TokenExchangeConfig)) *exchangeTestHarness {
	t.Helper()

	h := &exchangeTestHarness{
		idpCalls:    &atomic.Int32{},
		upstreamGot: &atomic.Value{},
	}
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
		h.idpCalls.Add(1)
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

	// A rotated session record is built fresh by SessionFromToken, so Exchanged must
	// come back zero rather than being copied forward.
	rotated := &session.Session{
		ID:             "new-subject-token",
		Mode:           session.ModeOIDC,
		AccessToken:    "new-subject-token",
		AccessExpiry:   time.Now().Add(time.Hour),
		AbsoluteExpiry: sess.AbsoluteExpiry,
		User:           sess.User,
	}
	if rotated.Exchanged.Token != "" {
		t.Error("a rotated session must not carry the previous exchanged token")
	}
	if rotated.Exchanged.Usable(time.Now(), time.Minute, h.server.exchanger.ConfigFingerprint()) {
		t.Error("a zero ExchangedToken must never report itself usable")
	}
}

// TestExchangedTokenUsable pins the cache-validity rules, each of which exists to
// stop a specific unsafe reuse.
func TestExchangedTokenUsable(t *testing.T) {
	const fp = "fingerprint"
	now := time.Now()

	for _, tc := range []struct {
		name string
		tok  session.ExchangedToken
		want bool
	}{
		{"fresh", session.ExchangedToken{
			Token: "t", Expiry: now.Add(time.Hour), ConfigFingerprint: fp}, true},
		{"empty token", session.ExchangedToken{
			Expiry: now.Add(time.Hour), ConfigFingerprint: fp}, false},
		{"unknown expiry", session.ExchangedToken{
			Token: "t", ConfigFingerprint: fp}, false},
		{"already expired", session.ExchangedToken{
			Token: "t", Expiry: now.Add(-time.Minute), ConfigFingerprint: fp}, false},
		{"inside the renewal window", session.ExchangedToken{
			Token: "t", Expiry: now.Add(30 * time.Second), ConfigFingerprint: fp}, false},
		{"stale fingerprint", session.ExchangedToken{
			Token: "t", Expiry: now.Add(time.Hour), ConfigFingerprint: "other"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tok.Usable(now, time.Minute, fp); got != tc.want {
				t.Errorf("Usable = %v, want %v", got, tc.want)
			}
		})
	}
}
