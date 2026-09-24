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
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/proxy"
	"ai-workspace-bff/internal/session"
)

const txCookieName = "_bff_oidc_tx"

// Login failure reasons handed to the SPA's login page as ?error=. Deliberately a
// coarse, user-facing classification and never the IDP's own reason: which check
// failed is a probing oracle (see auth.ErrStateMismatch), and the specific cause is
// already logged internally.
//
// The login page must be able to render every one of these WITHOUT restarting the
// login it has just returned from. AutoLoginPage redirects to the IDP whenever it
// finds itself unauthenticated, so a reason it does not recognise becomes a redirect
// loop against the IDP rather than a message — adding a reason here means adding it
// there too.
const (
	loginErrAuthFailed          = "auth_failed"
	loginErrSessionFailed       = "session_failed"
	loginErrExchangeRejected    = "token_exchange_rejected"
	loginErrUpstreamUnavailable = "upstream_unavailable"
)

// ---------------------------------------------------------------------------
// File-based login / logout / session
// ---------------------------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin (POST <base>/api/login) — file-based credentials → server-side session.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.fileBased == nil {
		writeErrorJSON(w, http.StatusBadRequest, "AUTH_METHOD_DISABLED", "file-based auth is not enabled")
		return
	}

	var req loginRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "invalid request body")
			return
		}
	} else {
		_ = r.ParseForm()
		req.Username = r.PostForm.Get("username")
		req.Password = r.PostForm.Get("password")
	}
	if req.Username == "" || req.Password == "" {
		writeErrorJSON(w, http.StatusBadRequest, "MISSING_CREDENTIALS", "username and password are required")
		return
	}

	sess, err := s.fileBased.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		var bad auth.ErrInvalidCredentials
		if errors.As(err, &bad) {
			writeErrorJSON(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials")
			return
		}
		slog.Error("file-based login failed", "err", err)
		writeServerErrorJSON(w, http.StatusBadGateway, "LOGIN_FAILED", "login failed", w.Header().Get("X-Request-Id"))
		return
	}

	// The cookie carries the JWT itself. File-based sessions have no refresh
	// token, so nothing is stored server-side at all.
	s.setSessionCookie(w, sess.AccessToken, sess.AbsoluteExpiry)
	writeJSON(w, http.StatusOK, map[string]any{"user": sess.User, "accessToken": sess.AccessToken})
}

// handleLogout (POST <base>/api/logout) — clear the cookie and (OIDC) drop the
// refresh-state entry, returning the IDP end-session URL.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	jwt, _ := s.tokenFromCookie(r)
	s.clearSessionCookie(w)

	if s.oidc != nil && jwt != "" {
		idToken := ""
		if sess, ok, _ := s.store.Get(r.Context(), jwt); ok {
			idToken = sess.IDToken
		}
		_ = s.store.Delete(r.Context(), jwt)
		writeJSON(w, http.StatusOK, map[string]string{"logoutUrl": s.oidc.LogoutURL(idToken)})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSession (GET <base>/api/session) — hydrate the SPA, including the access token.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Per-user authentication state (and the token it now carries) must never be
	// cached by browsers or proxies.
	w.Header().Set("Cache-Control", "no-store")
	jwt, ok := s.tokenFromCookie(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	// Fatal to the response, deliberately. In exchange mode the exchanged token is
	// what the Platform API authorizes, so a failed exchange means this handler cannot
	// say what the caller may do. Answering 200 with the LOGIN token's scopes would be
	// the exact substitution exchange mode exists to prevent — arriving at the identity
	// layer instead of on the wire — and the UI would then offer actions every proxied
	// call refuses.
	//
	// Classified by writeExchangeError rather than by a second rule here, so hydration
	// and proxying always agree about which failures end a session: a rejection is a
	// verdict on this subject token (401, session destroyed), everything else is the
	// deployment's problem, not the user's (502, session kept).
	//
	// The result is carried into userFromToken rather than read back from the stored
	// session, which is not merely a saved lookup: doExchange's write to the store is
	// best-effort and only logs on failure, so re-reading can miss the token that was
	// just minted and report the login token's scopes instead.
	var exchanged *auth.Result
	if s.exchanger != nil {
		res, err := s.exchangedToken(r.Context(), jwt)
		if err != nil {
			slog.Warn("token exchange failed while hydrating session", "err", err)
			s.writeExchangeError(w, r, err)
			return
		}
		exchanged = res
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          s.userFromToken(r.Context(), jwt, exchanged),
		"accessToken":   jwt,
	})
}

// ---------------------------------------------------------------------------
// OIDC
// ---------------------------------------------------------------------------

// handleOIDCLogin (GET <base>/api/auth/login) — redirect to the IDP authorize endpoint.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeErrorJSON(w, http.StatusBadRequest, "AUTH_METHOD_DISABLED", "oidc auth is not enabled")
		return
	}
	ret := s.sanitizeReturn(r.URL.Query().Get("return"))
	authURL, txID, err := s.oidc.AuthCodeURL(ret)
	if err != nil {
		slog.Error("oidc authorize url failed", "err", err)
		writeServerErrorJSON(w, http.StatusInternalServerError, "LOGIN_INIT_FAILED", "login init failed", w.Header().Get("X-Request-Id"))
		return
	}
	s.setTxCookie(w, txID)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback (GET <base>/api/auth/callback) — exchange code, create session.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeErrorJSON(w, http.StatusBadRequest, "AUTH_METHOD_DISABLED", "oidc auth is not enabled")
		return
	}
	q := r.URL.Query()
	if errCode := q.Get("error"); errCode != "" {
		slog.Warn("oidc callback error", "error", errCode, "desc", q.Get("error_description"))
		http.Redirect(w, r, s.path("/login")+"?error="+url.QueryEscape(errCode), http.StatusFound)
		return
	}

	txID := ""
	if c, err := r.Cookie(txCookieName); err == nil {
		txID = c.Value
	}
	s.clearTxCookie(w)

	sess, ret, err := s.oidc.Callback(r.Context(), txID, q.Get("state"), q.Get("code"))
	if err != nil {
		// tx_cookie_present is the field that separates "the browser never sent the
		// cookie" (a Path/SameSite problem) from "the server forgot the transaction"
		// (a restart) — the two look identical in the error alone.
		slog.Warn("oidc callback failed", "err", err,
			"path", r.URL.Path,
			"tx_cookie_present", txID != "",
			"tx_cookie_path", s.txCookiePath())
		http.Redirect(w, r, s.path("/login")+"?error="+loginErrAuthFailed, http.StatusFound)
		return
	}
	// OIDC: the cookie carries the access JWT, while the refresh/id tokens are
	// kept server-side keyed by that JWT so the proxy can renew it later.
	if err := s.putRefreshState(r.Context(), sess); err != nil {
		http.Redirect(w, r, s.path("/login")+"?error="+loginErrSessionFailed, http.StatusFound)
		return
	}
	// Eager, so a failure surfaces at login rather than as a 502 on the SPA's first
	// API call, and the first /api/session already has scopes.
	//
	// Both classes fail the login — a session that cannot produce an upstream token is
	// not a usable one — but they are reported apart, because "retrying will not help
	// you" and "retrying in a moment probably will" are different things to tell
	// someone, and the login page renders them differently. Reported apart is also all
	// they get: the session is destroyed either way, so the login stays atomic.
	if s.exchanger != nil {
		if _, err := s.exchangedToken(r.Context(), sess.AccessToken); err != nil {
			reason := loginErrUpstreamUnavailable
			if errors.Is(err, auth.ErrExchangeRejected) {
				reason = loginErrExchangeRejected
			}
			slog.Error("token exchange failed at login", "err", err, "login_error", reason)
			_ = s.store.Delete(r.Context(), sess.AccessToken)
			s.clearSessionCookie(w)
			http.Redirect(w, r, s.path("/login")+"?error="+reason, http.StatusFound)
			return
		}
	}

	s.setSessionCookie(w, sess.AccessToken, sess.AbsoluteExpiry)
	http.Redirect(w, r, s.sanitizeReturn(ret), http.StatusFound)
}

// ---------------------------------------------------------------------------
// Reverse proxy
// ---------------------------------------------------------------------------

// handleProxy (<base>/proxy/*) — take the JWT straight from the cookie and forward
// it upstream. No server-side lookup is involved unless the token is an OIDC
// access token that is near expiry and must be refreshed.
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	s.serveProxy(s.proxy, w, r)
}

// handleCloudProxy (<base>/proxy/cloud/*) — same session cookie injection as
// handleProxy, but against the optional Moesif / cloud analytics upstream.
func (s *Server) handleCloudProxy(w http.ResponseWriter, r *http.Request) {
	s.serveProxy(s.cloudProxy, w, r)
}

func (s *Server) handleBillingProxy(w http.ResponseWriter, r *http.Request) {
	s.serveProxy(s.billingProxy, w, r)
}

func (s *Server) serveProxy(rp *httputil.ReverseProxy, w http.ResponseWriter, r *http.Request) {
	if rp == nil {
		http.NotFound(w, r)
		return
	}

	jwt, ok := s.tokenFromCookie(r)
	if !ok {
		writeErrorJSON(w, http.StatusUnauthorized, "NOT_AUTHENTICATED", "not authenticated")
		return
	}

	// Refresh near-expiry OIDC access tokens before proxying. The expiry is read
	// from the JWT itself (not the store); the store is consulted only when an
	// actual refresh is required.
	if s.oidc != nil {
		exp := session.ExpiryFromClaims(session.DecodeJWTClaims(jwt))
		if needsRefreshSoon(exp) {
			refreshed, err := s.refreshByToken(r.Context(), jwt)
			if err != nil {
				slog.Warn("token refresh failed", "err", err)
				_ = s.store.Delete(r.Context(), jwt)
				s.clearSessionCookie(w)
				writeErrorJSON(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
				return
			}
			jwt = refreshed.AccessToken
			s.setSessionCookie(w, jwt, refreshed.AbsoluteExpiry)
		}
	}

	// Both hops authorize the forwarded token, so the exchange applies to whichever
	// one rp targets; the cloud hop must not fall back to the login token.
	upstream, err := s.upstreamToken(r.Context(), jwt)
	if err != nil {
		slog.Warn("token exchange failed for proxied request", "err", err, "path", r.URL.Path)
		s.writeExchangeError(w, r, err)
		return
	}

	rp.ServeHTTP(w, proxy.WithToken(r, upstream))
}

// ---------------------------------------------------------------------------
// Runtime config / health
// ---------------------------------------------------------------------------

// handleRuntimeConfig (GET <base>/runtime-config.js) — emit window.__RUNTIME_CONFIG__.
func (s *Server) handleRuntimeConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-store")

	var b strings.Builder
	b.WriteString("window.__RUNTIME_CONFIG__ = ")
	enc, _ := json.Marshal(s.cfg.RuntimeConfig)
	b.Write(enc)
	b.WriteString(";\n")
	_, _ = w.Write([]byte(b.String()))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Session helpers
// ---------------------------------------------------------------------------

// tokenFromCookie returns the JWT stored directly in the session cookie.
func (s *Server) tokenFromCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(s.cfg.Cookie.Name)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// userFromToken builds the display User for /api/session. File-based claims are
// self-contained in the JWT. For OIDC the stored entry holds the richer User
// (which merged id_token claims at login); we fall back to decoding the access
// token if that entry is gone (e.g. after a BFF restart).
//
// exchanged is the result of this request's own exchange, or nil when there was
// none (no exchanger configured, or the exchange failed). It takes precedence over
// whatever the stored session happens to hold, which may be older or — if the
// best-effort cache write failed — absent.
func (s *Server) userFromToken(ctx context.Context, jwt string, exchanged *auth.Result) session.User {
	if s.oidc != nil {
		if sess, ok, _ := s.store.Get(ctx, jwt); ok {
			return s.withExchangedIdentity(sess.User, exchanged, sess.Exchanged)
		}
		return s.oidc.UserFromAccessToken(jwt)
	}
	return session.UserFromClaims(session.DecodeJWTClaims(jwt), nil, s.claims)
}

// withExchangedIdentity reports what the Platform API will actually see: in exchange
// mode the exchanged token decides both what the caller may do AND which org they are
// in, because that is the token every proxied request carries.
//
// Scopes are what let an IDP that cannot mint ap:* scopes run with
// [auth.authorization] mode = "scope" instead of mirroring a grant table across two
// services. The org matters for the same reason one step further on: the login token
// names the IDP's own tenant (an Asgardeo org), while the STS resolves that to the
// platform org it issues for. Reporting the login token's org would show the user one
// org in the UI while every API call they make is scoped to another — and the org
// they would then "create resources in" is the exchanged one regardless.
//
// A zero ExchangedToken (Token == "") is left alone: a freshly restored session has
// not exchanged yet, and blanking scopes would show nothing as permitted for a fully
// authorized session. Once a session has exchanged, its scopes are copied verbatim —
// including a legitimately empty set — since that's what the Platform API authorizes.
//
// The org is copied only when the issued token actually carries one. A nil Org means
// the token said nothing about the org, not that the caller has none, so the login
// token's org stands — an STS that passes org claims through untouched, or a
// deployment with no org mapping configured, keeps working exactly as before.
func (s *Server) withExchangedIdentity(u session.User, fresh *auth.Result, cached session.ExchangedToken) session.User {
	if s.exchanger == nil {
		return u
	}
	if fresh != nil {
		u.Scopes = fresh.Scopes
		applyExchangedOrg(&u, fresh.Org)
		return u
	}
	if cached.Token == "" {
		return u
	}
	u.Scopes = cached.Scopes
	applyExchangedOrg(&u, cached.Org)
	return u
}

// applyExchangedOrg overwrites the org the UI shows with the one the issued token
// asserts, leaving it untouched when the token carries none.
//
// Only the current org. The org LIST stays as the login token stated it: which orgs
// a user belongs to does not change because a token was minted for one of them, and
// an STS is free to put something narrower (or nothing) in the issued token.
func applyExchangedOrg(u *session.User, org *session.Org) {
	if org != nil {
		u.Org = org
	}
}

// putRefreshState stores the OIDC refresh/id tokens keyed by the access JWT so
// the proxy can renew the token later. The cookie itself carries the JWT.
func (s *Server) putRefreshState(ctx context.Context, sess *session.Session) error {
	sess.ID = sess.AccessToken
	return s.store.Put(ctx, sess)
}

// needsRefreshSoon reports whether an access token is within the renewal window
// of its expiry. A zero expiry (no exp claim) is treated as not-refreshable.
func needsRefreshSoon(accessExpiry time.Time) bool {
	if accessExpiry.IsZero() {
		return false
	}
	return time.Now().Add(60 * time.Second).After(accessExpiry)
}

// refreshByToken performs a single-flight refresh keyed by the current access
// JWT, rotating the stored token set and re-keying the store entry to the new
// access token.
func (s *Server) refreshByToken(ctx context.Context, jwt string) (*session.Session, error) {
	s.refreshMu.Lock()
	mu := s.refreshLocks[jwt]
	if mu == nil {
		mu = &refreshLock{}
		s.refreshLocks[jwt] = mu
	}
	s.refreshMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	// A concurrent caller that shared this old token already performed the
	// refresh; hand them the same rotated result rather than re-reading the
	// store (whose old entry it has since deleted), which would otherwise turn a
	// successful rotation into a spurious session-expired error.
	if mu.done {
		return mu.result, mu.err
	}

	mu.result, mu.err = s.doRefresh(ctx, jwt)
	mu.done = true

	// The single-flight owner always drops the lock entry, on every exit path, so
	// the map cannot leak. Waiters already hold the mu pointer and read the
	// cached result above even after the map entry is gone.
	s.refreshMu.Lock()
	delete(s.refreshLocks, jwt)
	s.refreshMu.Unlock()

	return mu.result, mu.err
}

// doRefresh rotates the stored token set for the given access JWT and re-keys
// the store entry to the new access token. It is invoked exactly once per
// single-flight group by refreshByToken, which handles locking and cleanup.
func (s *Server) doRefresh(ctx context.Context, jwt string) (*session.Session, error) {
	cur, ok, _ := s.store.Get(ctx, jwt)
	if !ok {
		return nil, errors.New("session no longer exists")
	}
	if cur.RefreshToken == "" {
		return nil, errors.New("session has no refresh token")
	}
	if !needsRefreshSoon(cur.AccessExpiry) {
		return cur, nil
	}

	tok, err := s.oidc.Refresh(ctx, cur.RefreshToken)
	if err != nil {
		return nil, err
	}
	updated := s.oidc.SessionFromToken(tok, cur)
	updated.ID = updated.AccessToken
	// SessionFromToken returns a fresh record, so Exchanged is already zero. Do not
	// copy cur.Exchanged forward: it was derived from the token that just rotated.
	// Preserve the original absolute deadline: the hard cap must bound total
	// session lifetime, not slide forward on every refresh (which would let an
	// active session live indefinitely and disagree with the cookie's MaxAge).
	updated.AbsoluteExpiry = cur.AbsoluteExpiry
	// OrgHandle is the user's own selection, not a property of the token that just
	// rotated, so it survives the rotation — the opposite of Exchanged above, and for
	// the opposite reason. Dropped, the session silently reverts to default_org: the
	// next exchange mints a token for a DIFFERENT org, every proxied call is scoped to
	// that one, and nothing says so — the UI goes on showing the org the user picked.
	// The refresh fires once per token lifetime for any active session, so this is the
	// ordinary path, not an edge case.
	updated.OrgHandle = cur.OrgHandle

	var putErr error
	s.withSessionLock(jwt, func() {
		if putErr = s.store.Put(ctx, updated); putErr != nil {
			return
		}
		// Drop the old entry now that the token rotated. Locked against doExchange
		// so a concurrent exchange can't read the old entry before this delete and
		// write it back after — which would resurrect it under the rotated-out key.
		_ = s.store.Delete(ctx, jwt)
	})
	if putErr != nil {
		return nil, putErr
	}
	return updated, nil
}

// withSessionLock serializes fn against any other call for the same token,
// coordinating doExchange's cache write with doRefresh's rekey/delete so a
// stale session can never be reinserted after refresh removes it. This is
// separate from the single-flight locks above, which only dedupe concurrent
// callers of the *same* operation.
func (s *Server) withSessionLock(token string, fn func()) {
	l := s.acquireSessionLock(token)
	l.mu.Lock()
	defer s.releaseSessionLock(token, l)
	fn()
}

// sessionLock is one token's mutex plus a count of the callers currently holding or
// waiting for it. The count exists because the entry cannot simply be deleted when a
// caller finishes: a waiter already blocked on this mutex still needs the map to
// hand the SAME mutex to whoever arrives next. Deleting it there lets the next
// arrival create a second mutex for the same token and run concurrently with the
// waiter — which is exactly the doExchange/doRefresh overlap this lock exists to
// prevent, and it would appear only under load.
type sessionLock struct {
	mu sync.Mutex
	// users is guarded by Server.sessionMu, never by mu: it is read and written
	// while deciding whether the entry may leave the map, which must not require
	// holding the per-token lock.
	users int
}

// acquireSessionLock returns the token's lock, registering this caller as a user of
// it so the entry survives until the last of them is done.
func (s *Server) acquireSessionLock(token string) *sessionLock {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	l, ok := s.sessionLocks[token]
	if !ok {
		l = &sessionLock{}
		s.sessionLocks[token] = l
	}
	l.users++
	return l
}

// releaseSessionLock unlocks and drops the entry once no one else wants it, so the
// map does not grow with one entry per token ever seen. Deferred as a unit with the
// unlock, so a panic inside fn cannot leave the token's lock held forever.
func (s *Server) releaseSessionLock(token string, l *sessionLock) {
	l.mu.Unlock()

	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	l.users--
	if l.users == 0 {
		delete(s.sessionLocks, token)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// errorBody mirrors the Platform API's standard error shape (see
// platform-api/resources/openapi.yaml "Error" schema) so the frontend can
// parse every error response — proxied or BFF-originated — the same way.
type errorBody struct {
	Status     string `json:"status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	TrackingID string `json:"trackingId,omitempty"`
}

// writeErrorJSON writes a BFF-originated error (auth/session/CSRF failures
// that never reach the Platform API) in the same shape as Platform API
// errors. code must match ^[A-Z][A-Z0-9_]*$.
func writeErrorJSON(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Status: "error", Code: code, Message: message})
}

// writeServerErrorJSON writes a 5xx BFF-originated error, echoing the
// request's correlation id as trackingId — matching the Platform API
// contract that trackingId is present on server-side failures.
func writeServerErrorJSON(w http.ResponseWriter, status int, code, message, trackingID string) {
	writeJSON(w, status, errorBody{Status: "error", Code: code, Message: message, TrackingID: trackingID})
}

// sanitizeReturn ensures redirect targets are local paths inside this app (no open
// redirect). The SPA sends its own window.location.pathname, which already carries
// the base path, so anything landing outside the prefix — another portal on the same
// host, or a scheme-relative "//host" that a browser would treat as absolute — is
// replaced by the app root rather than followed or echoed back.
func (s *Server) sanitizeReturn(p string) string {
	home := s.path("/")
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return home
	}
	p = strings.ReplaceAll(strings.ReplaceAll(p, "\r", ""), "\n", "")
	if p != paths.Base && !strings.HasPrefix(p, home) {
		return home
	}
	return p
}

// ---------------------------------------------------------------------------
// Token exchange
// ---------------------------------------------------------------------------

// upstreamToken resolves the token to forward to the Platform API. With an exchange
// configured there is no fallback to the subject token: forwarding it would carry the
// wrong audience and, on an IDP that mints no ap:* scopes, no authorization at all.
func (s *Server) upstreamToken(ctx context.Context, subjectToken string) (string, error) {
	if s.exchanger == nil {
		return subjectToken, nil
	}
	res, err := s.exchangedToken(ctx, subjectToken)
	if err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

// exchangedToken returns a cached exchanged token when one is still usable, and
// performs an exchange otherwise. The org it exchanges/validates against is read
// from the session's OrgHandle — set only by handleSwitchOrg — not passed in here,
// so every call site (proxy, session hydration) automatically follows whatever org
// is currently selected without threading it through each of them individually.
func (s *Server) exchangedToken(ctx context.Context, subjectToken string) (*auth.Result, error) {
	fingerprint := s.exchanger.ConfigFingerprint()

	sess, ok, _ := s.store.Get(ctx, subjectToken)
	// Resolved once and used for BOTH the cache check and the exchange below, so a
	// cached token is never judged against a different org than the one it was
	// minted for. The user's selection wins over the configured default; the default
	// only fills the gap before they have made one.
	orgHandle := s.cfg.Auth.OIDC.TokenExchange.DefaultOrg
	if ok {
		if sess.OrgHandle != "" {
			orgHandle = sess.OrgHandle
		}
		if s.exchanger.CacheEnabled() && sess.Exchanged.Usable(time.Now(), s.exchanger.MinValidity(), fingerprint, orgHandle) {
			// Org travels with the cached token: a cache hit must describe the
			// caller exactly as the exchange that produced it did. Omitted, the
			// session would report the LOGIN token's org for the cached token's
			// whole lifetime — visible only as the wrong org name in the UI while
			// every API call is scoped to the right one.
			return &auth.Result{
				AccessToken: sess.Exchanged.Token,
				Expiry:      sess.Exchanged.Expiry,
				Scopes:      sess.Exchanged.Scopes,
				Org:         sess.Exchanged.Org,
			}, nil
		}
	}
	return s.exchangeSingleFlight(ctx, subjectToken, fingerprint, orgHandle)
}

// exchangeSingleFlight performs one exchange per (subject token, org) pair at a
// time, mirroring refreshByToken's structure. Keying on the pair rather than the
// subject token alone matters once org-scoped exchange is in play: a quick
// double-switch between two orgs must not let the second caller's request
// coalesce onto the first org's in-flight result.
func (s *Server) exchangeSingleFlight(ctx context.Context, subjectToken, fingerprint, orgHandle string) (*auth.Result, error) {
	key := subjectToken + "\x1f" + orgHandle

	s.exchangeMu.Lock()
	mu := s.exchangeLocks[key]
	if mu == nil {
		mu = &exchangeLock{}
		s.exchangeLocks[key] = mu
	}
	s.exchangeMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	if mu.done {
		return mu.result, mu.err
	}

	mu.result, mu.err = s.doExchange(ctx, subjectToken, fingerprint, orgHandle)
	mu.done = true

	// The owner drops the entry on every exit path; waiters hold the pointer and read
	// the cached result above even after it is gone.
	s.exchangeMu.Lock()
	delete(s.exchangeLocks, key)
	s.exchangeMu.Unlock()

	return mu.result, mu.err
}

// doExchange performs the exchange and caches the result on the session record.
func (s *Server) doExchange(ctx context.Context, subjectToken, fingerprint, orgHandle string) (*auth.Result, error) {
	res, err := s.exchanger.Exchange(ctx, subjectToken, orgHandle)
	if err != nil {
		return nil, err
	}

	if !s.exchanger.CacheEnabled() {
		return res, nil
	}

	if res.Expiry.IsZero() {
		slog.Warn("exchanged token has no expiry (no expires_in and no exp claim) — " +
			"caching skipped, so every upstream request will perform its own exchange")
		return res, nil
	}

	// Best-effort: a missing entry (BFF restarted mid-session, or one just rotated
	// out from under us — see withSessionLock) only costs a re-exchange next
	// request, so it must not fail this one.
	s.withSessionLock(subjectToken, func() {
		if sess, ok, _ := s.store.Get(ctx, subjectToken); ok {
			sess.Exchanged = session.ExchangedToken{
				Token:             res.AccessToken,
				Expiry:            res.Expiry,
				Scopes:            res.Scopes,
				ConfigFingerprint: fingerprint,
				OrgHandle:         orgHandle,
				Org:               res.Org,
			}
			if err := s.store.Put(ctx, sess); err != nil {
				slog.Warn("failed to cache exchanged token on the session", "err", err)
			}
		}
	})
	return res, nil
}

// writeExchangeError destroys the session on a rejection (it can never produce an
// upstream token) but keeps it on an unavailable IDP, which may recover — logging the
// user out over a transient blip would be self-inflicted. Neither response carries the
// IDP's reason; the exchanger already logged it.
func (s *Server) writeExchangeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, auth.ErrExchangeRejected) {
		if s.store != nil {
			if tok, ok := s.tokenFromCookie(r); ok {
				_ = s.store.Delete(r.Context(), tok)
			}
		}
		s.clearSessionCookie(w)
		writeErrorJSON(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
		return
	}
	writeErrorJSON(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream temporarily unavailable")
}

const maxSwitchOrgBodyBytes = 1 << 10 // an org handle is tiny; ample headroom

type switchOrgRequest struct {
	Org string `json:"org"`
}

// handleSwitchOrg (POST <base>/api/session/org) — re-exchange for the org the SPA
// just selected, so an IDP that mints org-scoped scopes (see [auth.oidc.token_exchange]
// org_param) issues a token for that org rather than whatever was last cached.
//
// Unlike writeExchangeError, a rejection here must not destroy the session: "you are
// not a member of that org" is an expected, recoverable outcome of switching, not a
// reason to log the user out of the org they were already in.
func (s *Server) handleSwitchOrg(w http.ResponseWriter, r *http.Request) {
	if s.exchanger == nil {
		writeErrorJSON(w, http.StatusBadRequest, "ORG_SCOPING_DISABLED", "token exchange is not enabled")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSwitchOrgBodyBytes)
	var req switchOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Org == "" {
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "org is required")
		return
	}

	// Both authentication failures below — no session cookie, and a cookie whose
	// session is gone — answer with one identical 401. Branching the payload on
	// which it was tells a caller whether a token they hold is merely unknown here
	// or was valid until recently, and nothing legitimate needs to tell them apart.
	jwt, ok := s.tokenFromCookie(r)
	if !ok {
		writeErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired credentials.")
		return
	}

	var previousOrg string
	var sessionErr error
	s.withSessionLock(jwt, func() {
		sess, found, _ := s.store.Get(r.Context(), jwt)
		if !found {
			sessionErr = errors.New("session not found")
			return
		}
		previousOrg = sess.OrgHandle
		sess.OrgHandle = req.Org
		sessionErr = s.store.Put(r.Context(), sess)
	})
	if sessionErr != nil {
		writeErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired credentials.")
		return
	}

	res, err := s.exchangedToken(r.Context(), jwt)
	if err != nil {
		// Roll back to the org the session was actually still authorized for, so
		// proxied calls right after a rejected switch keep working rather than
		// repeatedly retrying the exchange against an org that just failed.
		s.withSessionLock(jwt, func() {
			if sess, found, _ := s.store.Get(r.Context(), jwt); found {
				sess.OrgHandle = previousOrg
				_ = s.store.Put(r.Context(), sess)
			}
		})
		if errors.Is(err, auth.ErrExchangeRejected) {
			writeErrorJSON(w, http.StatusForbidden, "ORG_SWITCH_REJECTED", "unable to switch to that organization")
			return
		}
		writeErrorJSON(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream temporarily unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scopes": res.Scopes})
}
