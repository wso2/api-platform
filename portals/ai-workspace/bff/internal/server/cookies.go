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
	"net/http"
	"strings"
	"time"

	"ai-workspace-bff/internal/session"
)

func sameSite(v string) http.SameSite {
	switch strings.ToLower(v) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// setSessionCookie writes the opaque session id cookie, bounded by the session's
// absolute expiry.
func (s *Server) setSessionCookie(w http.ResponseWriter, sess *session.Session) {
	maxAge := 0
	if !sess.AbsoluteExpiry.IsZero() {
		if d := time.Until(sess.AbsoluteExpiry); d > 0 {
			maxAge = int(d.Seconds())
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Cookie.Name,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: sameSite(s.cfg.Cookie.SameSite),
		MaxAge:   maxAge,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Cookie.Name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: sameSite(s.cfg.Cookie.SameSite),
		MaxAge:   -1,
	})
}

// setTxCookie writes the short-lived OIDC login-transaction cookie.
func (s *Server) setTxCookie(w http.ResponseWriter, txID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     txCookieName,
		Value:    txID,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
}

func (s *Server) clearTxCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     txCookieName,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
