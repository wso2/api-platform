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
	"strings"
	"time"

	"api-control-plane-bff/internal/auth"
	"api-control-plane-bff/internal/proxy"
	"api-control-plane-bff/internal/session"
)

const txCookieName = "_bff_oidc_tx"

// ---------------------------------------------------------------------------
// File-based login / logout / session
// ---------------------------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin (POST /api/login) — file-based credentials -> server-side session.
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

	// The cookie carries the token itself. File-based sessions have no refresh
	// token, so nothing is stored server-side at all. The token is never
	// returned in the response body — the browser only ever gets the display
	// user; every backend call is routed through the same-origin proxy, which
	// injects the token itself.
	s.setSessionCookie(w, sess.AccessToken, sess.AbsoluteExpiry)
	writeJSON(w, http.StatusOK, map[string]any{"user": sess.User})
}

// handleLogout (POST /api/logout) — clear the cookie and (OIDC) drop the
// refresh-state entry, returning the IDP end-session URL.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token, _ := s.tokenFromCookie(r)
	s.clearSessionCookie(w)

	if s.oidc != nil && token != "" {
		idToken := ""
		if sess, ok, _ := s.store.Get(r.Context(), token); ok {
			idToken = sess.IDToken
		}
		_ = s.store.Delete(r.Context(), token)
		writeJSON(w, http.StatusOK, map[string]string{"logoutUrl": s.oidc.LogoutURL(idToken)})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSession (GET /api/session) — hydrate the SPA. The token itself is
// never included in the response; only the display user.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Per-user authentication state must never be cached by browsers or proxies.
	w.Header().Set("Cache-Control", "no-store")
	token, ok := s.tokenFromCookie(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          s.userFromToken(r.Context(), token),
	})
}

// ---------------------------------------------------------------------------
// OIDC
// ---------------------------------------------------------------------------

// handleOIDCLogin (GET /api/auth/login) — redirect to the IDP authorize endpoint.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeErrorJSON(w, http.StatusBadRequest, "AUTH_METHOD_DISABLED", "oidc auth is not enabled")
		return
	}
	ret := sanitizeReturn(r.URL.Query().Get("return"))
	authURL, txID, err := s.oidc.AuthCodeURL(ret)
	if err != nil {
		slog.Error("oidc authorize url failed", "err", err)
		writeServerErrorJSON(w, http.StatusInternalServerError, "LOGIN_INIT_FAILED", "login init failed", w.Header().Get("X-Request-Id"))
		return
	}
	s.setTxCookie(w, txID)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback (GET /api/auth/callback) — exchange code, create session.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeErrorJSON(w, http.StatusBadRequest, "AUTH_METHOD_DISABLED", "oidc auth is not enabled")
		return
	}
	q := r.URL.Query()
	if errCode := q.Get("error"); errCode != "" {
		slog.Warn("oidc callback error", "error", errCode, "desc", q.Get("error_description"))
		http.Redirect(w, r, "/login?error="+errCode, http.StatusFound)
		return
	}

	txID := ""
	if c, err := r.Cookie(txCookieName); err == nil {
		txID = c.Value
	}
	s.clearTxCookie(w)

	sess, ret, err := s.oidc.Callback(r.Context(), txID, q.Get("state"), q.Get("code"))
	if err != nil {
		slog.Warn("oidc callback failed", "err", err)
		http.Redirect(w, r, "/login?error=auth_failed", http.StatusFound)
		return
	}
	// OIDC: the cookie carries the access token, while the refresh/id tokens are
	// kept server-side keyed by that token so the proxy can renew it later.
	if err := s.putRefreshState(r.Context(), sess); err != nil {
		http.Redirect(w, r, "/login?error=session_failed", http.StatusFound)
		return
	}
	s.setSessionCookie(w, sess.AccessToken, sess.AbsoluteExpiry)
	http.Redirect(w, r, sanitizeReturn(ret), http.StatusFound)
}

// ---------------------------------------------------------------------------
// Reverse proxy
// ---------------------------------------------------------------------------

// proxyHandler returns a handler that takes the token straight from the
// cookie and forwards it via rp. No server-side lookup is involved unless the
// token is an OIDC access token that is near expiry and must be refreshed —
// the same refresh logic runs regardless of which mounted upstream (primary
// or named) is being called, since all of them share the one session cookie.
func (s *Server) proxyHandler(rp *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := s.tokenFromCookie(r)
		if !ok {
			writeErrorJSON(w, http.StatusUnauthorized, "NOT_AUTHENTICATED", "not authenticated")
			return
		}

		// Refresh near-expiry OIDC access tokens before proxying. The expiry is
		// read from the token itself (not the store); the store is consulted
		// only when an actual refresh is required.
		if s.oidc != nil {
			exp := session.ExpiryFromClaims(session.DecodeJWTClaims(token))
			if needsRefreshSoon(exp) {
				refreshed, err := s.refreshByToken(r.Context(), token)
				if err != nil {
					slog.Warn("token refresh failed", "err", err)
					_ = s.store.Delete(r.Context(), token)
					s.clearSessionCookie(w)
					writeErrorJSON(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
					return
				}
				token = refreshed.AccessToken
				s.setSessionCookie(w, token, refreshed.AbsoluteExpiry)
			}
		}

		rp.ServeHTTP(w, proxy.WithToken(r, token))
	}
}

// ---------------------------------------------------------------------------
// Runtime config / health
// ---------------------------------------------------------------------------

// handleRuntimeConfig (GET /api-platform.env.config.js) — emit
// window.__RUNTIME_CONFIG__.
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

// handleCommonConfig (GET /api-platform.common.config.js) — index.html loads
// this alongside the runtime config above; it's intentionally empty (all
// runtime configuration lives in window.__RUNTIME_CONFIG__), and exists only
// so the <script> tag resolves instead of 404ing.
func (s *Server) handleCommonConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-store")
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Session helpers
// ---------------------------------------------------------------------------

// tokenFromCookie returns the token stored directly in the session cookie.
func (s *Server) tokenFromCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(s.cfg.Session.Cookie.Name)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// userFromToken builds the display User for /api/session. File-based claims
// are self-contained in the token. For OIDC the stored entry holds the richer
// User (which merged id_token claims at login); we fall back to decoding the
// access token if that entry is gone (e.g. after a BFF restart).
func (s *Server) userFromToken(ctx context.Context, token string) session.User {
	if s.oidc != nil {
		if sess, ok, _ := s.store.Get(ctx, token); ok {
			return sess.User
		}
		return s.oidc.UserFromAccessToken(token)
	}
	return session.UserFromClaims(session.DecodeJWTClaims(token), nil, s.claims)
}

// putRefreshState stores the OIDC refresh/id tokens keyed by the access token
// so the proxy can renew it later. The cookie itself carries the token.
func (s *Server) putRefreshState(ctx context.Context, sess *session.Session) error {
	sess.ID = sess.AccessToken
	return s.store.Put(ctx, sess)
}

// needsRefreshSoon reports whether an access token is within the renewal
// window of its expiry. A zero expiry (no known expiry at all) is treated as
// not-refreshable — refreshing is only attempted when there is a deadline to
// refresh ahead of.
func needsRefreshSoon(accessExpiry time.Time) bool {
	if accessExpiry.IsZero() {
		return false
	}
	return time.Now().Add(60 * time.Second).After(accessExpiry)
}

// refreshByToken performs a single-flight refresh keyed by the current access
// token, rotating the stored token set and re-keying the store entry to the new
// access token. Called by the reverse proxy handler before forwarding a
// near-expiry OIDC session upstream.
func (s *Server) refreshByToken(ctx context.Context, token string) (*session.Session, error) {
	s.refreshMu.Lock()
	mu := s.refreshLocks[token]
	if mu == nil {
		mu = &refreshLock{}
		s.refreshLocks[token] = mu
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

	mu.result, mu.err = s.doRefresh(ctx, token)
	mu.done = true

	// The single-flight owner always drops the lock entry, on every exit path, so
	// the map cannot leak. Waiters already hold the mu pointer and read the
	// cached result above even after the map entry is gone.
	s.refreshMu.Lock()
	delete(s.refreshLocks, token)
	s.refreshMu.Unlock()

	return mu.result, mu.err
}

// doRefresh rotates the stored token set for the given access token and
// re-keys the store entry to the new access token. It is invoked exactly once
// per single-flight group by refreshByToken, which handles locking and cleanup.
func (s *Server) doRefresh(ctx context.Context, token string) (*session.Session, error) {
	cur, ok, _ := s.store.Get(ctx, token)
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
	// Preserve the original absolute deadline: the hard cap must bound total
	// session lifetime, not slide forward on every refresh (which would let an
	// active session live indefinitely and disagree with the cookie's MaxAge).
	updated.AbsoluteExpiry = cur.AbsoluteExpiry
	if err := s.store.Put(ctx, updated); err != nil {
		return nil, err
	}
	// Drop the old entry now that the token rotated.
	_ = s.store.Delete(ctx, token)
	return updated, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// errorBody mirrors the Platform API's standard error shape so the frontend
// can parse every error response — proxied or BFF-originated — the same way.
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

// sanitizeReturn ensures redirect targets are local paths (no open redirect).
func sanitizeReturn(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return "/"
	}
	return strings.ReplaceAll(strings.ReplaceAll(p, "\r", ""), "\n", "")
}
