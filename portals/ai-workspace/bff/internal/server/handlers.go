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
	"strings"
	"time"

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/proxy"
	"ai-workspace-bff/internal/session"
)

const txCookieName = "_bff_oidc_tx"

// ---------------------------------------------------------------------------
// File-based login / logout / session
// ---------------------------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin (POST /api/login) — file-based credentials → server-side session.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.fileBased == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file-based auth is not enabled"})
		return
	}

	var req loginRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
	} else {
		_ = r.ParseForm()
		req.Username = r.PostForm.Get("username")
		req.Password = r.PostForm.Get("password")
	}
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	sess, err := s.fileBased.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		var bad auth.ErrInvalidCredentials
		if errors.As(err, &bad) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		slog.Error("file-based login failed", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "login failed"})
		return
	}

	// The cookie carries the JWT itself. File-based sessions have no refresh
	// token, so nothing is stored server-side at all.
	s.setSessionCookie(w, sess.AccessToken, sess.AbsoluteExpiry)
	writeJSON(w, http.StatusOK, map[string]any{"user": sess.User, "accessToken": sess.AccessToken})
}

// handleLogout (POST /api/logout) — clear the cookie and (OIDC) drop the
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

// handleSession (GET /api/session) — hydrate the SPA, including the access token.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Per-user authentication state (and the token it now carries) must never be
	// cached by browsers or proxies.
	w.Header().Set("Cache-Control", "no-store")
	jwt, ok := s.tokenFromCookie(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          s.userFromToken(r.Context(), jwt),
		"accessToken":   jwt,
	})
}

// ---------------------------------------------------------------------------
// OIDC
// ---------------------------------------------------------------------------

// handleOIDCLogin (GET /api/auth/login) — redirect to the IDP authorize endpoint.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "oidc auth is not enabled"})
		return
	}
	ret := sanitizeReturn(r.URL.Query().Get("return"))
	authURL, txID, err := s.oidc.AuthCodeURL(ret)
	if err != nil {
		slog.Error("oidc authorize url failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "login init failed"})
		return
	}
	s.setTxCookie(w, txID)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback (GET /api/auth/callback) — exchange code, create session.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "oidc auth is not enabled"})
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
	// OIDC: the cookie carries the access JWT, while the refresh/id tokens are
	// kept server-side keyed by that JWT so the proxy can renew it later.
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

// handleProxy (/api/proxy/*) — take the JWT straight from the cookie and forward
// it upstream. No server-side lookup is involved unless the token is an OIDC
// access token that is near expiry and must be refreshed.
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	jwt, ok := s.tokenFromCookie(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
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
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
				return
			}
			jwt = refreshed.AccessToken
			s.setSessionCookie(w, jwt, refreshed.AbsoluteExpiry)
		}
	}

	s.proxy.ServeHTTP(w, proxy.WithToken(r, jwt))
}

// ---------------------------------------------------------------------------
// Runtime config / health
// ---------------------------------------------------------------------------

// handleRuntimeConfig (GET /runtime-config.js) — emit window.__RUNTIME_CONFIG__.
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
func (s *Server) userFromToken(ctx context.Context, jwt string) session.User {
	if s.oidc != nil {
		if sess, ok, _ := s.store.Get(ctx, jwt); ok {
			return sess.User
		}
		return s.oidc.UserFromAccessToken(jwt)
	}
	return session.UserFromClaims(session.DecodeJWTClaims(jwt), nil, session.DefaultClaimMapping())
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
	// Preserve the original absolute deadline: the hard cap must bound total
	// session lifetime, not slide forward on every refresh (which would let an
	// active session live indefinitely and disagree with the cookie's MaxAge).
	updated.AbsoluteExpiry = cur.AbsoluteExpiry
	if err := s.store.Put(ctx, updated); err != nil {
		return nil, err
	}
	// Drop the old entry now that the token rotated.
	_ = s.store.Delete(ctx, jwt)
	return updated, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// sanitizeReturn ensures redirect targets are local paths (no open redirect).
func sanitizeReturn(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return "/"
	}
	return strings.ReplaceAll(strings.ReplaceAll(p, "\r", ""), "\n", "")
}
