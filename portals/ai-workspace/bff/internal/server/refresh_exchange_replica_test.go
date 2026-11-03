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

// The three mechanisms that have to hold together once the session stops living in
// one process: the access JWT split across two cookies, the token exchange whose
// result is cached on the session, and the refresh that rotates BOTH the access token
// (re-keying the state) and the refresh token itself. Each is driven here on a
// different replica than the one before it.

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/proxy"
	"ai-workspace-bff/internal/secure"
	"ai-workspace-bff/internal/session"
	"net/url"
)

// ---------------------------------------------------------------------------
// A browser-faithful cookie jar
// ---------------------------------------------------------------------------

// jar models what a browser actually keeps: one value per (name, Path) — the key a
// browser uses — replaced by each Set-Cookie and removed by an expiry. Replaying the
// raw Set-Cookie list instead would let a test pass on a response carrying two values
// for one cookie, which a real browser collapses to the last and an intermediary may
// collapse to the first.
//
// Path-aware rather than name-only because clearSessionCookie deliberately expires the
// same name at two Paths (the current one and the pre-upgrade origin root), and those
// are two distinct cookies, not a duplicate write.
type jar map[string]string

func jarKey(name, path string) string { return name + "\x00" + path }

func (j jar) apply(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	seen := map[string]int{}
	for _, c := range rec.Result().Cookies() {
		key := jarKey(c.Name, c.Path)
		seen[key]++
		if c.MaxAge < 0 || c.Value == "" {
			delete(j, key)
			continue
		}
		j[key] = c.Value
	}
	for key, n := range seen {
		if n > 1 {
			name, path, _ := strings.Cut(key, "\x00")
			t.Errorf("response set cookie %q at Path %q %d times — a browser keeps only the "+
				"last, and an intermediary may keep the first", name, path, n)
		}
	}
}

func (j jar) onto(r *http.Request) {
	for key, value := range j {
		name, _, _ := strings.Cut(key, "\x00")
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
}

// get returns the live value for a cookie name, whatever Path it was set at. The BFF
// only ever sets these at the app's base path, so a name is unambiguous in practice.
func (j jar) get(name string) string {
	for key, value := range j {
		if n, _, _ := strings.Cut(key, "\x00"); n == name {
			return value
		}
	}
	return ""
}

// sessionToken reassembles the access JWT from the jar the way the BFF does.
func (j jar) sessionToken(cfg config.CookieConfig) string {
	p1, p2 := j.get(cfg.Name1), j.get(cfg.Name2)
	if p1 == "" || p2 == "" {
		return ""
	}
	return joinSessionToken(p1, p2)
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type rotatingIDP struct {
	url        string
	exchanges  atomic.Int32
	refreshes  atomic.Int32
	nonce      atomic.Value
	upstreamAt atomic.Value // last Authorization header the upstream saw
}

// scopeHeavyJWT mints a decodable access token the size this product actually issues:
// the default scope set is 122 entries, which is why the session cookie is split in
// two to begin with. Sized realistically so the state record is exercised at its real
// footprint, not a toy one.
func scopeHeavyJWT(t *testing.T, sub string, expiresIn time.Duration) string {
	t.Helper()
	scopes := make([]string, 0, 122)
	for _, r := range []string{"api_key", "organization", "project", "gateway", "api", "mcp_proxy",
		"llm_provider", "subscription", "application", "api_portal", "policy", "environment",
		"component", "deployment", "secret", "role", "user", "team", "billing", "usage",
		"analytics", "audit", "webhook", "integration", "connector"} {
		for _, v := range []string{"read", "create", "update", "delete", "manage"} {
			scopes = append(scopes, "ap:"+r+":"+v)
		}
	}
	return unsignedJWT(t, map[string]any{
		"sub": sub, "exp": time.Now().Add(expiresIn).Unix(), "iat": time.Now().Unix(),
		"scope": strings.Join(scopes, " "), "username": "alice@acme.example.com",
		"org_handle": "acme-corp",
	})
}

func newRotatingHarness(t *testing.T, accessTTL time.Duration) (*rotatingIDP, *Server, *Server) {
	t.Helper()
	idp := &rotatingIDP{}
	idp.nonce.Store("")
	idp.upstreamAt.Store("")

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": srv.URL, "authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint": srv.URL + "/token",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			n, _ := idp.nonce.Load().(string)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  scopeHeavyJWT(t, "login", accessTTL),
				"refresh_token": "refresh-1",
				"id_token":      unsignedJWT(t, map[string]any{"nonce": n, "username": "alice"}),
				"token_type":    "Bearer", "expires_in": int(accessTTL.Seconds()),
			})
		case "refresh_token":
			// A rotating IDP: presenting refresh-1 twice must not be how this works,
			// so the test asserts the rotated one is what the next replica holds.
			if got := r.PostForm.Get("refresh_token"); got != "refresh-1" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			idp.refreshes.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  scopeHeavyJWT(t, "rotated", time.Hour),
				"refresh_token": "refresh-2",
				"token_type":    "Bearer", "expires_in": 3600,
			})
		default: // token exchange
			n := idp.exchanges.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":      scopeHeavyJWT(t, "exchanged", time.Hour),
				"issued_token_type": auth.TokenTypeAccessToken,
				"token_type":        "Bearer", "expires_in": 3600,
				"scope": "ap:project:read ap:gateway:read",
				// Echoed so the assertions can name which exchange produced a token.
				"exchange_seq": n,
			})
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	idp.url = srv.URL

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idp.upstreamAt.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}

	teCfg := config.TokenExchangeConfig{
		Enabled: true, GrantType: config.GrantTokenExchange,
		ClientID: "c", ClientSecret: "s", Audience: "platform-api",
		Scopes:           "ap:project:read ap:gateway:read",
		SubjectTokenType: auth.TokenTypeJWT, RequestedTokenType: auth.TokenTypeAccessToken,
		CacheEnabled: true, MinValidity: 60 * time.Second,
	}
	cfg := &config.Config{
		Cookie: config.CookieConfig{
			Name1: "_ai_workspace_session_1", Name2: "_ai_workspace_session_2",
			StatePrefix: "_ai_workspace_state_", Secure: true, SameSite: "lax",
		},
		Auth:    config.AuthConfig{Mode: config.AuthModeOIDC, OIDC: config.OIDCConfig{TokenExchange: teCfg}},
		Session: config.SessionConfig{Store: config.SessionStoreCookie, AbsoluteTTL: 8 * time.Hour},
	}

	build := func() *Server {
		oidcClient := replicaOIDC(t, idp.url, txSealer(t, testSealMaterial))
		sealer, err := secure.NewSealer(secure.DeriveKey(testSealMaterial, config.StateSealLabel))
		if err != nil {
			t.Fatalf("NewSealer: %v", err)
		}
		return &Server{
			cfg: cfg, claims: session.DefaultClaimMapping(), oidc: oidcClient,
			proxy:      proxy.ReverseProxy(target, paths.Base+paths.Proxy, http.DefaultTransport),
			exchanger:  auth.NewExchanger(http.DefaultClient, teCfg, oidcClient.TokenEndpoint()),
			stateCodec: session.NewCookieCodec(sealer, stateChunkSize, stateMaxChunks, session.DefaultClaimMapping()),
			store:      cookieStore{}, refreshLocks: make(map[string]*refreshLock),
			exchangeLocks: make(map[string]*exchangeLock), sessionLocks: make(map[string]*sessionLock),
			discoverLocks: make(map[string]*discoverLock),
		}
	}
	return idp, build(), build()
}

// loginOn drives a full OIDC login on one replica and returns the browser's jar.
func (idp *rotatingIDP) loginOn(t *testing.T, s *Server, j jar) {
	t.Helper()
	authURL, txID, err := s.oidc.AuthCodeURL(paths.Base+"/", nil)
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	idp.nonce.Store(u.Query().Get("nonce"))

	req := httptest.NewRequest(http.MethodGet,
		paths.Base+"/api/auth/callback?code=auth-code&state="+url.QueryEscape(u.Query().Get("state")), nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: txID})
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(s.handleOIDCCallback)).ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Fatalf("login failed: %d -> %s", rec.Code, rec.Header().Get("Location"))
	}
	j.apply(t, rec)
}

func proxyOn(t *testing.T, s *Server, j jar) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, paths.Base+paths.Proxy+"/api/v0.9/projects", nil)
	j.onto(req)
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(s.handleProxy)).ServeHTTP(rec, req)
	j.apply(t, rec)
	return rec
}

func sessionOn(t *testing.T, s *Server, j jar) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, paths.Base+"/api/session", nil)
	j.onto(req)
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(s.handleSession)).ServeHTTP(rec, req)
	j.apply(t, rec)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /api/session (%d): %v", rec.Code, err)
	}
	return body
}

// Steady state, hopping replicas at every step: the split cookie pair and the single
// cached exchanged token must carry a session across replicas with no re-exchange and
// no refresh. A long-lived access token keeps the renewal window out of the picture —
// rotation gets its own test below.
func TestSplitCookieAndExchangeAcrossReplicas(t *testing.T) {
	idp, replicaA, replicaB := newRotatingHarness(t, time.Hour)
	j := jar{}

	idp.loginOn(t, replicaA, j)

	loginToken := j.sessionToken(replicaA.cfg.Cookie)
	if loginToken == "" {
		t.Fatal("login left no usable session cookie pair")
	}
	if len(loginToken) < 2000 {
		t.Fatalf("the test's access token is %d B — too small to exercise the split", len(loginToken))
	}
	if j.get(replicaA.cfg.Cookie.Name1) == loginToken || j.get(replicaA.cfg.Cookie.Name2) == loginToken {
		t.Fatal("the access JWT was not actually split across the two cookies")
	}
	if j.get("_ai_workspace_state_0") == "" {
		t.Fatal("login sealed no session state into the browser")
	}
	if n := idp.exchanges.Load(); n != 1 {
		t.Fatalf("login performed %d exchanges, want exactly 1 (eager, at login)", n)
	}

	// Replica B has never seen this user: it must hydrate from the cookies alone, and
	// report the EXCHANGED token's scopes rather than the login token's 125.
	body := sessionOn(t, replicaB, j)
	if body["authenticated"] != true {
		t.Fatalf("replica B did not recognise the session: %v", body)
	}
	user, _ := body["user"].(map[string]any)
	scopes, _ := user["scopes"].([]any)
	if len(scopes) != 2 {
		t.Fatalf("replica B reported %d scopes, want the 2 the exchanged token carries", len(scopes))
	}

	for i, replica := range []*Server{replicaA, replicaB, replicaA} {
		rec := proxyOn(t, replica, j)
		if rec.Code != http.StatusOK {
			t.Fatalf("proxied call %d = %d (%s)", i, rec.Code, rec.Body)
		}
		got, _ := idp.upstreamAt.Load().(string)
		if !strings.HasPrefix(got, "Bearer ") || got == "Bearer "+loginToken {
			t.Fatalf("call %d: upstream got %q — want the exchanged token, never the login token", i, got)
		}
	}
	if n := idp.exchanges.Load(); n != 1 {
		t.Errorf("%d exchanges across four requests on two replicas, want 1 — the cached "+
			"exchanged token is not surviving the hop", n)
	}
	if n := idp.refreshes.Load(); n != 0 {
		t.Errorf("refreshes = %d, want 0 for a token nowhere near expiry", n)
	}
	if tok := j.sessionToken(replicaA.cfg.Cookie); tok != loginToken {
		t.Error("the session cookie pair changed without a rotation")
	}
}

// Rotation, driven on the replica that did NOT log the user in. It has to find the
// refresh token in the sealed state, rotate the access token the cookie pair carries,
// re-key the state to it, re-exchange (the cached token belonged to the token that
// just rotated), and leave a session the other replica can keep using.
func TestRefreshRotationAcrossReplicas(t *testing.T) {
	// 30s is inside handleProxy's 60s renewal window, so the first proxied call
	// refreshes. The login's own eager exchange has already happened by then.
	idp, replicaA, replicaB := newRotatingHarness(t, 30*time.Second)
	j := jar{}

	idp.loginOn(t, replicaA, j)
	loginToken := j.sessionToken(replicaA.cfg.Cookie)
	if loginToken == "" {
		t.Fatal("login left no usable session cookie pair")
	}
	if n := idp.exchanges.Load(); n != 1 {
		t.Fatalf("login performed %d exchanges, want 1", n)
	}

	rec := proxyOn(t, replicaB, j)
	if rec.Code != http.StatusOK {
		t.Fatalf("proxy on replica B during refresh = %d (%s)", rec.Code, rec.Body)
	}
	if n := idp.refreshes.Load(); n != 1 {
		t.Fatalf("refreshes = %d, want 1 — replica B could not use the refresh token "+
			"from the state replica A sealed", n)
	}

	rotatedToken := j.sessionToken(replicaB.cfg.Cookie)
	if rotatedToken == "" {
		t.Fatal("the rotated access token left no usable cookie pair")
	}
	if rotatedToken == loginToken {
		t.Fatal("the session cookie pair still carries the pre-rotation access token")
	}
	if n := idp.exchanges.Load(); n != 2 {
		t.Fatalf("exchanges = %d, want 2 — the rotated token must not reuse the exchange "+
			"minted for the token it replaced", n)
	}
	postRotation, _ := idp.upstreamAt.Load().(string)

	// The rotated session keeps working on the other replica, which is only possible
	// if the ROTATED refresh token and the new exchanged token both reached the
	// browser, sealed and re-bound to the new access token. The stub IDP rejects
	// anything but refresh-1, so a second refresh here would fail outright.
	if rec := proxyOn(t, replicaA, j); rec.Code != http.StatusOK {
		t.Fatalf("proxy on replica A after rotation = %d (%s)", rec.Code, rec.Body)
	}
	if n := idp.refreshes.Load(); n != 1 {
		t.Errorf("replica A refreshed again (%d) — the rotated token's expiry did not travel", n)
	}
	if n := idp.exchanges.Load(); n != 2 {
		t.Errorf("exchanges = %d, want 2 — the post-rotation exchanged token was not cached", n)
	}
	if got, _ := idp.upstreamAt.Load().(string); got != postRotation {
		t.Error("replica A forwarded a different token than the one minted at rotation")
	}

	body := sessionOn(t, replicaA, j)
	if body["authenticated"] != true {
		t.Fatalf("the rotated session is not recognised: %v", body)
	}
	if body["accessToken"] != rotatedToken {
		t.Error("/api/session reports a different access token than the cookie pair carries")
	}
}

// The pre-rotation state must not keep working: once the access token rotates, a state
// record bound to the old one is refused rather than silently reused.
func TestPreRotationStateIsNotAcceptedAfterwards(t *testing.T) {
	idp, replicaA, replicaB := newRotatingHarness(t, 30*time.Second)
	j := jar{}
	idp.loginOn(t, replicaA, j)

	stale := jar{}
	for k, v := range j {
		stale[k] = v
	}

	if rec := proxyOn(t, replicaB, j); rec.Code != http.StatusOK {
		t.Fatalf("refresh-triggering call = %d", rec.Code)
	}

	// The old access-token cookies paired with the NEW state, and vice versa: both are
	// mismatches the binding has to catch.
	const atPath = paths.Base + "/"
	mixed := jar{
		jarKey(replicaA.cfg.Cookie.Name1, atPath): stale.get(replicaA.cfg.Cookie.Name1),
		jarKey(replicaA.cfg.Cookie.Name2, atPath): stale.get(replicaA.cfg.Cookie.Name2),
		jarKey("_ai_workspace_state_0", atPath):   j.get("_ai_workspace_state_0"),
	}
	req := httptest.NewRequest(http.MethodGet, paths.Base+"/api/session", nil)
	mixed.onto(req)
	var found bool
	replicaA.withSessionState(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, found, _ = replicaA.store.Get(r.Context(), mixed.sessionToken(replicaA.cfg.Cookie))
	})).ServeHTTP(httptest.NewRecorder(), req)
	if found {
		t.Fatal("a state record bound to the rotated-out access token was accepted")
	}
}

// Logout on one replica must leave nothing usable on any other.
func TestLogoutOnOneReplicaEndsTheSessionEverywhere(t *testing.T) {
	idp, replicaA, replicaB := newRotatingHarness(t, time.Hour)
	j := jar{}
	idp.loginOn(t, replicaA, j)

	req := httptest.NewRequest(http.MethodPost, paths.Base+"/api/logout", nil)
	j.onto(req)
	rec := httptest.NewRecorder()
	replicaB.withSessionState(http.HandlerFunc(replicaB.handleLogout)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout on replica B = %d (%s)", rec.Code, rec.Body)
	}
	j.apply(t, rec)

	if len(j) != 0 {
		t.Fatalf("logout left cookies behind: %v", keysOf(j))
	}
	if body := sessionOn(t, replicaA, j); body["authenticated"] == true {
		t.Fatal("replica A still reports the logged-out session as authenticated")
	}
}

func keysOf(j jar) []string {
	out := make([]string, 0, len(j))
	for k := range j {
		name, path, _ := strings.Cut(k, "\x00")
		out = append(out, name+" @ "+path)
	}
	return out
}

var _ = context.Background

// Concurrent requests that coalesce onto one refresh must EACH leave the browser with a
// complete session.
//
// The single-flight hands one result to every waiter, but only the owner's request
// writes it to the store. Without staging it per request, a waiter's response set the
// rotated token cookies and no state cookie — so a browser applying that response last
// held a new access token bound to a stale record: no refresh token, no cached
// exchange, and a logout at the access token's expiry with nothing able to renew it.
func TestConcurrentRefreshLeavesEveryResponseComplete(t *testing.T) {
	idp, replica, _ := newRotatingHarness(t, 30*time.Second)
	j := jar{}
	idp.loginOn(t, replica, j)
	oldToken := j.sessionToken(replica.cfg.Cookie)

	const callers = 4
	var wg sync.WaitGroup
	recs := make([]*httptest.ResponseRecorder, callers)
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, paths.Base+paths.Proxy+"/api/v0.9/projects", nil)
			j.onto(req) // every caller carries the same pre-rotation cookies
			rec := httptest.NewRecorder()
			<-start
			replica.withSessionState(http.HandlerFunc(replica.handleProxy)).ServeHTTP(rec, req)
			recs[i] = rec
		}(i)
	}
	close(start)
	wg.Wait()

	if n := idp.refreshes.Load(); n != 1 {
		t.Fatalf("refreshes = %d, want 1 — the single-flight stopped coalescing", n)
	}

	for i, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Errorf("response %d = %d", i, rec.Code)
			continue
		}
		applied := jar{}
		applied.apply(t, rec)
		token := applied.sessionToken(replica.cfg.Cookie)
		if token == "" || token == oldToken {
			t.Errorf("response %d did not carry the rotated access token", i)
			continue
		}
		state := applied.get("_ai_workspace_state_0")
		if state == "" {
			t.Errorf("response %d carries the rotated token with no session state — a browser "+
				"applying it last would hold a session it cannot renew", i)
			continue
		}
		sess, ok := replica.stateCodec.Decode([]string{state}, token)
		if !ok {
			t.Errorf("response %d: its state does not bind to the token it set", i)
			continue
		}
		if sess.RefreshToken == "" {
			t.Errorf("response %d: no refresh token — the session cannot be renewed", i)
		}
	}
}

// Every caller coalescing onto one exchange must also end up with it cached, or a
// waiter's response leaves a session that re-exchanges on the very next request.
func TestConcurrentExchangeCachesOnEveryResponse(t *testing.T) {
	idp, replica, _ := newRotatingHarness(t, time.Hour)
	j := jar{}
	idp.loginOn(t, replica, j)
	afterLogin := idp.exchanges.Load()

	// A live session that has not exchanged yet — the state the waiters must end up
	// updating. Rewriting the record rather than deleting it: with no record at all
	// there is nothing to cache onto, which is a different (and already degraded) case.
	token := j.sessionToken(replica.cfg.Cookie)
	seed := httptest.NewRequest(http.MethodGet, "/", nil)
	j.onto(seed)
	seedRec := httptest.NewRecorder()
	replica.withSessionState(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		sess, ok, _ := replica.store.Get(r.Context(), token)
		if !ok {
			t.Fatal("seed: no session")
		}
		sess.Exchanged = session.ExchangedToken{}
		if err := replica.store.Put(r.Context(), sess); err != nil {
			t.Fatalf("seed: %v", err)
		}
	})).ServeHTTP(seedRec, seed)
	j.apply(t, seedRec)

	const callers = 4
	var wg sync.WaitGroup
	recs := make([]*httptest.ResponseRecorder, callers)
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, paths.Base+paths.Proxy+"/api/v0.9/projects", nil)
			j.onto(req)
			rec := httptest.NewRecorder()
			<-start
			replica.withSessionState(http.HandlerFunc(replica.handleProxy)).ServeHTTP(rec, req)
			recs[i] = rec
		}(i)
	}
	close(start)
	wg.Wait()

	if n := idp.exchanges.Load() - afterLogin; n != 1 {
		t.Fatalf("exchanges = %d for a burst of %d, want 1", n, callers)
	}
	for i, rec := range recs {
		applied := jar{}
		applied.apply(t, rec)
		state := applied.get("_ai_workspace_state_0")
		if state == "" {
			t.Errorf("response %d wrote no session state", i)
			continue
		}
		sess, ok := replica.stateCodec.Decode([]string{state}, token)
		if !ok || sess.Exchanged.Token == "" {
			t.Errorf("response %d did not cache the exchanged token it just used", i)
		}
	}
}
