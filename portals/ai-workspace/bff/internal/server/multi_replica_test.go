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

// Multi-replica behaviour: with [session] store = "cookie" the BFF holds nothing
// per-session, so a request may land on any replica. These tests model that by
// building two entirely separate instances that share only their configuration —
// exactly what two pods behind a load balancer have in common — and driving one
// instance's output into the other.

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/secure"
	"ai-workspace-bff/internal/session"
)

const testSealMaterial = "shared-oidc-client-secret"

// replicaServer is the smallest Server that can exercise the cookie store: config,
// codec and store, with no upstreams.
func replicaServer(t *testing.T, material string) *Server {
	t.Helper()
	sealer, err := secure.NewSealer(secure.DeriveKey(material, config.StateSealLabel))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return &Server{
		cfg: &config.Config{
			Cookie: config.CookieConfig{
				Name1:       "_ai_workspace_session_1",
				Name2:       "_ai_workspace_session_2",
				StatePrefix: "_ai_workspace_state_",
				Secure:      true,
				SameSite:    "lax",
			},
		},
		stateCodec: session.NewCookieCodec(sealer, stateChunkSize, stateMaxChunks, session.DefaultClaimMapping()),
		store:      cookieStore{},
	}
}

// putOnReplica runs a request through a replica's state middleware, hands the handler
// the store, and returns the cookies the response told the browser to keep.
func putOnReplica(t *testing.T, s *Server, fn func(ctx context.Context)) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		fn(r.Context())
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Result().Cookies()
}

// getOnReplica replays cookies onto a second replica and reads the session back.
func getOnReplica(t *testing.T, s *Server, cookies []*http.Cookie, id string) (*session.Session, bool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		if c.MaxAge >= 0 {
			req.AddCookie(c)
		}
	}
	var got *session.Session
	var ok bool
	s.withSessionState(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, ok, _ = s.store.Get(r.Context(), id)
	})).ServeHTTP(httptest.NewRecorder(), req)
	return got, ok
}

// The central claim: everything the BFF used to keep in process memory survives a
// hop to a replica that has never seen this user.
func TestSessionStateTravelsToAnotherReplica(t *testing.T) {
	replicaA, replicaB := replicaServer(t, testSealMaterial), replicaServer(t, testSealMaterial)

	want := &session.Session{
		ID:             "access-token",
		Mode:           session.ModeOIDC,
		AccessToken:    "access-token",
		RefreshToken:   "refresh-token",
		IDToken:        "id-token",
		AbsoluteExpiry: time.Now().Add(8 * time.Hour),
		User:           session.User{Name: "alice", Scopes: []string{"ap:project:read"}},
		OrgHandle:      "acme",
		OrgDiscovered:  true,
		Exchanged: session.ExchangedToken{
			Token:  "exchanged-token",
			Expiry: time.Now().Add(time.Hour),
			Scopes: []string{"ap:project:read"},
		},
	}
	cookies := putOnReplica(t, replicaA, func(ctx context.Context) {
		if err := replicaA.store.Put(ctx, want); err != nil {
			t.Errorf("Put on replica A: %v", err)
		}
	})

	got, ok := getOnReplica(t, replicaB, cookies, "access-token")
	if !ok {
		t.Fatal("replica B found no session — the state did not survive the hop")
	}
	if got.RefreshToken != want.RefreshToken {
		t.Errorf("refresh token = %q, want %q", got.RefreshToken, want.RefreshToken)
	}
	if got.OrgHandle != "acme" || !got.OrgDiscovered {
		t.Errorf("org selection lost: handle=%q discovered=%v", got.OrgHandle, got.OrgDiscovered)
	}
	if got.Exchanged.Token != "exchanged-token" {
		t.Errorf("cached exchanged token lost: %q", got.Exchanged.Token)
	}
}

// Every state cookie is HttpOnly, so the sealed record — which carries the refresh
// token and the exchanged token — is never readable by script in the page.
func TestStateCookiesAreHttpOnly(t *testing.T) {
	s := replicaServer(t, testSealMaterial)
	cookies := putOnReplica(t, s, func(ctx context.Context) {
		_ = s.store.Put(ctx, &session.Session{
			ID: "tok", AccessToken: "tok", RefreshToken: "r",
			AbsoluteExpiry: time.Now().Add(time.Hour),
		})
	})
	if len(cookies) == 0 {
		t.Fatal("no state cookies were set")
	}
	for _, c := range cookies {
		if !c.HttpOnly || !c.Secure {
			t.Errorf("cookie %q: HttpOnly=%v Secure=%v, want both true", c.Name, c.HttpOnly, c.Secure)
		}
	}
}

// A replica configured with different key material is a different deployment: it must
// not be able to open the record, rather than opening it partially or crashing.
func TestStateFromAnotherDeploymentIsIgnored(t *testing.T) {
	replicaA := replicaServer(t, testSealMaterial)
	stranger := replicaServer(t, "a-totally-different-secret")

	cookies := putOnReplica(t, replicaA, func(ctx context.Context) {
		_ = replicaA.store.Put(ctx, &session.Session{
			ID: "tok", AccessToken: "tok", RefreshToken: "r",
			AbsoluteExpiry: time.Now().Add(time.Hour),
		})
	})
	if _, ok := getOnReplica(t, stranger, cookies, "tok"); ok {
		t.Fatal("a record sealed by another deployment was accepted")
	}
}

// The refresh path puts the rotated session and then deletes the pre-rotation one. In
// a cookie world both name the same storage, so the delete must not undo the put.
func TestRotationWithinOneRequestKeepsTheNewRecord(t *testing.T) {
	s := replicaServer(t, testSealMaterial)
	cookies := putOnReplica(t, s, func(ctx context.Context) {
		if err := s.store.Put(ctx, &session.Session{
			ID: "new-token", AccessToken: "new-token", RefreshToken: "rotated",
			AbsoluteExpiry: time.Now().Add(time.Hour),
		}); err != nil {
			t.Errorf("Put: %v", err)
		}
		_ = s.store.Delete(ctx, "old-token") // the pre-rotation key
	})

	got, ok := getOnReplica(t, s, cookies, "new-token")
	if !ok {
		t.Fatal("the rotated session was deleted along with the token it replaced")
	}
	if got.RefreshToken != "rotated" {
		t.Errorf("refresh token = %q, want %q", got.RefreshToken, "rotated")
	}
}

func TestDeleteClearsTheStateCookies(t *testing.T) {
	s := replicaServer(t, testSealMaterial)
	live := putOnReplica(t, s, func(ctx context.Context) {
		_ = s.store.Put(ctx, &session.Session{
			ID: "tok", AccessToken: "tok", RefreshToken: "r",
			AbsoluteExpiry: time.Now().Add(time.Hour),
		})
	})

	// Replay the live cookies into a request that logs out, then check the response
	// expires them rather than leaving them in the browser.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range live {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_ = s.store.Delete(r.Context(), "tok")
	})).ServeHTTP(rec, req)

	expired := 0
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			expired++
		}
	}
	if expired == 0 {
		t.Fatal("logout left the sealed state cookies in the browser")
	}
}

// ---------------------------------------------------------------------------
// OIDC login transactions
// ---------------------------------------------------------------------------

// stubIssuer serves just enough discovery and token response to drive a login.
func stubIssuer(t *testing.T, nonce *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + srv.URL + `","authorization_endpoint":"` + srv.URL +
			`/authorize","token_endpoint":"` + srv.URL + `/token"}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","token_type":"Bearer",` +
			`"expires_in":3600,"id_token":"` + unsignedJWT(t, map[string]any{"nonce": *nonce}) + `"}`))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func replicaOIDC(t *testing.T, issuer string, sealer *secure.Sealer) *auth.OIDC {
	t.Helper()
	o, err := auth.NewOIDC(context.Background(), http.DefaultClient,
		issuer, "client-id", testSealMaterial,
		"https://portal.example.com/ai-workspace/api/auth/callback", "", "openid",
		session.DefaultClaimMapping(), 8*time.Hour, sealer)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	t.Cleanup(o.Close)
	return o
}

// txSealer builds the login-transaction key the way server.New does.
func txSealer(t *testing.T, material string) *secure.Sealer {
	t.Helper()
	s, err := secure.NewSealer(secure.DeriveKey(material, config.TxSealLabel))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return s
}

// startLogin returns the tx cookie value plus the state and nonce the authorize URL
// carried — the two the IDP echoes back, and which the callback checks the sealed
// record against.
func startLogin(t *testing.T, o *auth.OIDC) (txID, state, nonce string) {
	t.Helper()
	authURL, txID, err := o.AuthCodeURL("/ai-workspace/", nil)
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	return txID, u.Query().Get("state"), u.Query().Get("nonce")
}

// The login half of the multi-replica story: the IDP's callback is a fresh browser
// navigation, so the load balancer is free to route it to a different replica than
// the one that started the login.
func TestLoginStartedOnOneReplicaCompletesOnAnother(t *testing.T) {
	nonce := ""
	idp := stubIssuer(t, &nonce)
	sealer := txSealer(t, testSealMaterial)

	replicaA := replicaOIDC(t, idp.URL, sealer)
	replicaB := replicaOIDC(t, idp.URL, sealer)

	// The stub echoes this login's nonce, which the callback checks the id_token
	// against before trusting any of its claims.
	txID, state, n := startLogin(t, replicaA)
	nonce = n

	sess, ret, err := replicaB.Callback(context.Background(), txID, state, "auth-code")
	if err != nil {
		t.Fatalf("replica B could not complete a login started on replica A: %v", err)
	}
	if sess.AccessToken != "at" || sess.RefreshToken != "rt" {
		t.Errorf("session = %+v, want the stub's token set", sess)
	}
	if ret != "/ai-workspace/" {
		t.Errorf("return url = %q, want the one replica A recorded", ret)
	}
}

// A sealed transaction from another deployment must not complete a login here.
func TestSealedTransactionFromAnotherDeploymentIsRejected(t *testing.T) {
	nonce := ""
	idp := stubIssuer(t, &nonce)

	stranger := replicaOIDC(t, idp.URL, txSealer(t, "another-deployments-secret"))
	ours := replicaOIDC(t, idp.URL, txSealer(t, testSealMaterial))

	txID, state, n := startLogin(t, stranger)
	nonce = n
	if _, _, err := ours.Callback(context.Background(), txID, state, "auth-code"); err == nil {
		t.Fatal("a transaction sealed under another deployment's key completed a login")
	}
}

// ---------------------------------------------------------------------------
// End to end through the real handlers
// ---------------------------------------------------------------------------

// peerReplica returns a second Server sharing everything a second pod would (the same
// IDP client, upstreams and config) and nothing it would not: its own codec, built
// independently from the same key material.
func peerReplica(t *testing.T, origin *Server) *Server {
	t.Helper()
	sealer, err := secure.NewSealer(secure.DeriveKey(testSealMaterial, config.StateSealLabel))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	// Field by field rather than a struct copy: Server carries its own mutexes, and a
	// second pod does not inherit the first's locks any more than it inherits its map.
	return &Server{
		cfg:           origin.cfg,
		claims:        origin.claims,
		oidc:          origin.oidc,
		proxy:         origin.proxy,
		exchanger:     origin.exchanger,
		stateCodec:    session.NewCookieCodec(sealer, stateChunkSize, stateMaxChunks, session.DefaultClaimMapping()),
		store:         cookieStore{},
		refreshLocks:  make(map[string]*refreshLock),
		exchangeLocks: make(map[string]*exchangeLock),
		sessionLocks:  make(map[string]*sessionLock),
		discoverLocks: make(map[string]*discoverLock),
	}
}

// useCookieStore switches a harness's server onto the cookie store, as
// [session] store = "cookie" does in server.New.
func useCookieStore(t *testing.T, s *Server) {
	t.Helper()
	sealer, err := secure.NewSealer(secure.DeriveKey(testSealMaterial, config.StateSealLabel))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	_ = s.store.Close()
	s.cfg.Cookie.StatePrefix = "_ai_workspace_state_"
	s.stateCodec = session.NewCookieCodec(sealer, stateChunkSize, stateMaxChunks, session.DefaultClaimMapping())
	s.store = cookieStore{}
}

// login drives a full OIDC callback through the state middleware, exactly as routes()
// wires it — the callback is where the session is first written, so without the
// carrier there is nowhere to write it.
func login(t *testing.T, h *exchangeTestHarness) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.server.withSessionState(http.HandlerFunc(h.server.handleOIDCCallback)).
		ServeHTTP(rec, h.callbackRequest(t))
	if rec.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302 (body %s)", rec.Code, rec.Body)
	}
	return rec
}

// The end-to-end claim, through the handlers a browser actually hits: log in on one
// replica, then make an API call that lands on another. The second replica must forward
// the EXCHANGED token without re-exchanging, which is only possible if the session the
// first replica built reached it through the browser.
func TestLoginOnOneReplicaThenProxyOnAnother(t *testing.T) {
	h := newExchangeHarness(t, nil)
	useCookieStore(t, h.server)

	loginRec := login(t, h)
	token := sessionCookieValue(h, loginRec)
	if token == "" {
		t.Fatalf("login set no session cookie (redirected to %q)", loginRec.Header().Get("Location"))
	}
	exchangesAfterLogin := h.idpCalls.Load()

	// Everything the browser kept, replayed at a replica that served none of it.
	peer := peerReplica(t, h.server)
	req := httptest.NewRequest(http.MethodGet, paths.Base+paths.Proxy+"/api/v0.9/projects", nil)
	for _, c := range loginRec.Result().Cookies() {
		if c.Value != "" && c.MaxAge >= 0 {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	peer.withSessionState(http.HandlerFunc(peer.handleProxy)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("proxy on the peer replica = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got, _ := h.upstreamGot.Load().(string); got != "Bearer exchanged-token" {
		t.Errorf("upstream Authorization = %q, want the exchanged token", got)
	}
	if n := h.idpCalls.Load(); n != exchangesAfterLogin {
		t.Errorf("the peer replica performed %d extra exchange(s); the cached token did not reach it",
			n-exchangesAfterLogin)
	}
}

// An org the user switched to on one replica must still be the org the next replica
// exchanges for — otherwise a user ping-pongs between organizations as the load
// balancer picks, with the UI showing one and every API call scoped to the other.
func TestOrgSelectionSurvivesTheHopToAnotherReplica(t *testing.T) {
	h := newExchangeHarness(t, nil)
	useCookieStore(t, h.server)

	loginRec := login(t, h)
	token := sessionCookieValue(h, loginRec)
	if token == "" {
		t.Fatalf("login set no session cookie (redirected to %q)", loginRec.Header().Get("Location"))
	}
	cookies := loginRec.Result().Cookies()

	switchReq := httptest.NewRequest(http.MethodPost, paths.Base+"/api/session/org", nil)
	for _, c := range cookies {
		if c.Value != "" && c.MaxAge >= 0 {
			switchReq.AddCookie(c)
		}
	}
	switchReq.Body = http.NoBody
	switchRec := httptest.NewRecorder()
	h.server.withSessionState(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drive the store directly rather than the handler: handleSwitchOrg needs a
		// JSON body and an org_param-configured exchanger, neither of which this test
		// is about — what it asserts is that the selection reaches the next replica.
		sess, ok, _ := h.server.store.Get(r.Context(), token)
		if !ok {
			t.Error("the session was not readable on the replica that created it")
			return
		}
		sess.OrgHandle = "acme"
		sess.OrgDiscovered = true
		if err := h.server.store.Put(r.Context(), sess); err != nil {
			t.Errorf("Put: %v", err)
		}
	})).ServeHTTP(switchRec, switchReq)

	peer := peerReplica(t, h.server)
	got, ok := getOnReplica(t, peer, switchRec.Result().Cookies(), token)
	if !ok {
		t.Fatal("the peer replica found no session after the org switch")
	}
	if got.OrgHandle != "acme" || !got.OrgDiscovered {
		t.Fatalf("org on the peer replica = %q (discovered=%v), want acme/true",
			got.OrgHandle, got.OrgDiscovered)
	}
}

// Regression: a request that only READS the session must say nothing about the state
// cookies. It reached flush with nothing staged, which was indistinguishable from a
// delete — so every GET /api/session, and every proxied call that hit the exchanged-
// token cache, expired the record it had just read. The session then "worked" while
// silently losing its refresh token and cached exchange on every single request.
func TestReadOnlyRequestLeavesStateCookiesAlone(t *testing.T) {
	s := replicaServer(t, testSealMaterial)
	live := putOnReplica(t, s, func(ctx context.Context) {
		_ = s.store.Put(ctx, &session.Session{
			ID: "tok", AccessToken: "tok", RefreshToken: "refresh-token",
			AbsoluteExpiry: time.Now().Add(time.Hour),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range live {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok, _ := s.store.Get(r.Context(), "tok"); !ok {
			t.Error("the session was not readable")
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	for _, c := range rec.Result().Cookies() {
		t.Errorf("a read-only request rewrote cookie %q (MaxAge=%d); it must leave the "+
			"browser holding exactly what it sent", c.Name, c.MaxAge)
	}
}

// ---------------------------------------------------------------------------
// Logout revocation
// ---------------------------------------------------------------------------

// Dropping the server's copy of the refresh token stopped being a revocation the moment
// the browser started carrying one too. Logout has to tell the IDP.
func TestLogoutRevokesTheRefreshTokenAtTheIDP(t *testing.T) {
	var gotForm url.Values
	var revocations atomic.Int32
	var srv *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + srv.URL + `","authorization_endpoint":"` + srv.URL +
			`/authorize","token_endpoint":"` + srv.URL + `/token","revocation_endpoint":"` + srv.URL +
			`/revoke","end_session_endpoint":"` + srv.URL + `/logout"}`))
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		revocations.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := replicaServer(t, testSealMaterial)
	s.oidc = replicaOIDC(t, srv.URL, txSealer(t, testSealMaterial))

	const token = "access-token"
	live := putOnReplica(t, s, func(ctx context.Context) {
		_ = s.store.Put(ctx, &session.Session{
			ID: token, AccessToken: token, RefreshToken: "refresh-to-revoke",
			IDToken: "id-token", AbsoluteExpiry: time.Now().Add(time.Hour),
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	addSessionCookies(req, s.cfg.Cookie, token)
	for _, c := range live {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.withSessionState(http.HandlerFunc(s.handleLogout)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("logout = %d (%s)", rec.Code, rec.Body)
	}
	if n := revocations.Load(); n != 1 {
		t.Fatalf("revocation calls = %d, want 1 — a refresh token the browser still holds "+
			"would otherwise stay usable after logout", n)
	}
	if got := gotForm.Get("token"); got != "refresh-to-revoke" {
		t.Errorf("revoked token = %q, want the session's refresh token", got)
	}
	if got := gotForm.Get("token_type_hint"); got != "refresh_token" {
		t.Errorf("token_type_hint = %q, want %q (RFC 7009)", got, "refresh_token")
	}
}

// An IDP that is down, or that advertises no revocation endpoint, must not leave the
// user unable to sign out — the cookies are cleared either way.
func TestLogoutSucceedsWhenRevocationIsUnavailable(t *testing.T) {
	for name, revoke := range map[string]string{
		"no revocation endpoint advertised": "",
		"revocation endpoint errors":        "/revoke",
	} {
		t.Run(name, func(t *testing.T) {
			var srv *httptest.Server
			mux := http.NewServeMux()
			mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
				rev := ""
				if revoke != "" {
					rev = `,"revocation_endpoint":"` + srv.URL + revoke + `"`
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"issuer":"` + srv.URL + `","authorization_endpoint":"` + srv.URL +
					`/authorize","token_endpoint":"` + srv.URL + `/token"` + rev + `}`))
			})
			mux.HandleFunc("/revoke", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})
			srv = httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			s := replicaServer(t, testSealMaterial)
			s.oidc = replicaOIDC(t, srv.URL, txSealer(t, testSealMaterial))

			const token = "access-token"
			live := putOnReplica(t, s, func(ctx context.Context) {
				_ = s.store.Put(ctx, &session.Session{
					ID: token, AccessToken: token, RefreshToken: "rt",
					AbsoluteExpiry: time.Now().Add(time.Hour),
				})
			})
			req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
			addSessionCookies(req, s.cfg.Cookie, token)
			for _, c := range live {
				req.AddCookie(c)
			}
			rec := httptest.NewRecorder()
			s.withSessionState(http.HandlerFunc(s.handleLogout)).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("logout = %d, want 200 — a failed revocation must not block sign-out", rec.Code)
			}
			cleared := 0
			for _, c := range rec.Result().Cookies() {
				if c.MaxAge < 0 {
					cleared++
				}
			}
			if cleared == 0 {
				t.Error("logout cleared no cookies")
			}
		})
	}
}
