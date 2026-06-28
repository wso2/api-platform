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

	if err := s.persistAndSetCookie(w, r, sess); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": sess.User})
}

// handleLogout (POST /api/logout) — destroy session, clear cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.currentSession(r)
	if sess != nil {
		_ = s.store.Delete(r.Context(), sess.ID)
	}
	s.clearSessionCookie(w)

	if sess != nil && sess.Mode == session.ModeOIDC && s.oidc != nil {
		writeJSON(w, http.StatusOK, map[string]string{"logoutUrl": s.oidc.LogoutURL(sess.IDToken)})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSession (GET /api/session) — hydrate the SPA (no token in the body).
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": sess.User})
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
	if err := s.persistAndSetCookie(w, r, sess); err != nil {
		http.Redirect(w, r, "/login?error=session_failed", http.StatusFound)
		return
	}
	http.Redirect(w, r, sanitizeReturn(ret), http.StatusFound)
}

// ---------------------------------------------------------------------------
// Reverse proxy
// ---------------------------------------------------------------------------

// handleProxy (/api/proxy/*) — inject the session's bearer and forward upstream.
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	// Refresh near-expiry OIDC access tokens before proxying.
	if sess.Mode == session.ModeOIDC && s.oidc != nil && s.needsRefresh(sess) {
		refreshed, err := s.refreshSession(r.Context(), sess)
		if err != nil {
			slog.Warn("token refresh failed", "err", err)
			_ = s.store.Delete(r.Context(), sess.ID)
			s.clearSessionCookie(w)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
			return
		}
		sess = refreshed
	}

	s.proxy.ServeHTTP(w, proxy.WithToken(r, sess.AccessToken))
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

func (s *Server) currentSession(r *http.Request) (*session.Session, bool) {
	c, err := r.Cookie(s.cfg.Cookie.Name)
	if err != nil || c.Value == "" {
		return nil, false
	}
	sess, ok, err := s.store.Get(r.Context(), c.Value)
	if err != nil || !ok {
		return nil, false
	}
	return sess, true
}

func (s *Server) persistAndSetCookie(w http.ResponseWriter, r *http.Request, sess *session.Session) error {
	id, err := session.NewID()
	if err != nil {
		return err
	}
	sess.ID = id
	if err := s.store.Put(r.Context(), sess); err != nil {
		return err
	}
	s.setSessionCookie(w, sess)
	return nil
}

func (s *Server) needsRefresh(sess *session.Session) bool {
	if sess.RefreshToken == "" || sess.AccessExpiry.IsZero() {
		return false
	}
	return time.Now().Add(60 * time.Second).After(sess.AccessExpiry)
}

// refreshSession performs a single-flight refresh per session id and rotates the
// stored refresh token.
func (s *Server) refreshSession(ctx context.Context, sess *session.Session) (*session.Session, error) {
	s.refreshMu.Lock()
	mu := s.refreshLocks[sess.ID]
	if mu == nil {
		mu = &refreshLock{}
		s.refreshLocks[sess.ID] = mu
	}
	s.refreshMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	// Re-read: another goroutine may have refreshed while we waited.
	if cur, ok, _ := s.store.Get(ctx, sess.ID); ok && !s.needsRefresh(cur) {
		return cur, nil
	}

	tok, err := s.oidc.Refresh(ctx, sess.RefreshToken)
	if err != nil {
		return nil, err
	}
	updated := s.oidc.SessionFromToken(tok, sess)
	updated.ID = sess.ID
	if err := s.store.Put(ctx, updated); err != nil {
		return nil, err
	}
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
