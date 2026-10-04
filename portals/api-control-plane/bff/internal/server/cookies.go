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
)

// defaultRefreshCookieSuffix is appended (or replaces "_session") to form the
// companion HttpOnly cookie that carries the OIDC refresh token. With the
// product default session name this yields _api_control_plane_refresh.
const defaultRefreshCookieSuffix = "_refresh"

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

// refreshCookieName is the HttpOnly cookie that carries the OIDC refresh
// token. The access token stays in the unchanged session cookie (bare JWT);
// splitting them avoids the 4KB single-cookie limit when both tokens are large.
func (s *Server) refreshCookieName() string {
	name := s.cfg.Session.Cookie.Name
	if strings.HasSuffix(name, "_session") {
		return strings.TrimSuffix(name, "_session") + defaultRefreshCookieSuffix
	}
	return name + defaultRefreshCookieSuffix
}

// setSessionCookie writes the access-token session cookie as a bare JWT
// (unchanged from the pre-envelope design). When refresh is non-empty it also
// writes the companion refresh cookie; when refresh is empty the refresh
// cookie is cleared so a file-based login cannot leave a stale OIDC refresh.
func (s *Server) setSessionCookie(w http.ResponseWriter, access, refresh string, absExpiry time.Time) {
	maxAge := 0
	if !absExpiry.IsZero() {
		if d := time.Until(absExpiry); d > 0 {
			maxAge = int(d.Seconds())
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Session.Cookie.Name,
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Session.Cookie.Secure,
		SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
		MaxAge:   maxAge,
	})
	if refresh != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     s.refreshCookieName(),
			Value:    refresh,
			Path:     "/",
			HttpOnly: true,
			Secure:   s.cfg.Session.Cookie.Secure,
			SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
			MaxAge:   maxAge,
		})
	} else {
		s.clearRefreshCookie(w)
	}
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.refreshCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Session.Cookie.Secure,
		SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
		MaxAge:   -1,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Session.Cookie.Name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Session.Cookie.Secure,
		SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
		MaxAge:   -1,
	})
	s.clearRefreshCookie(w)
}

// setTxCookie writes the short-lived OIDC login-transaction cookie.
func (s *Server) setTxCookie(w http.ResponseWriter, txID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     txCookieName,
		Value:    txID,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   s.cfg.Session.Cookie.Secure,
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
		Secure:   s.cfg.Session.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
