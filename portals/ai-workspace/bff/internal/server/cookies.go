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

// setSessionCookie writes the session cookie carrying the JWT itself, bounded by
// the supplied absolute expiry. The cookie stays HttpOnly so browser JS cannot
// read it, but the proxy reads the JWT straight from it — no server-side lookup.
//
// Path is scoped to the app's base path so a host serving several portals under
// different prefixes doesn't send this session to any of the others.
func (s *Server) setSessionCookie(w http.ResponseWriter, jwt string, absExpiry time.Time) {
	maxAge := 0
	if !absExpiry.IsZero() {
		if d := time.Until(absExpiry); d > 0 {
			maxAge = int(d.Seconds())
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Cookie.Name,
		Value:    jwt,
		Path:     s.path("/"),
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: sameSite(s.cfg.Cookie.SameSite),
		MaxAge:   maxAge,
	})
}

// legacyRootCookiePath is the origin-root Path the session cookie used before it moved
// under the app's base path. clearSessionCookie must expire it too: a browser keys a
// cookie by (name, domain, path), so an expiry written for one Path creates a separate
// cookie rather than removing an existing one at another Path. Left behind, a
// pre-upgrade cookie at "/" keeps matching every request below the root, so
// /api/session goes on reporting the stale session as authenticated while every proxied
// call 401s on its no-longer-verifiable token — a login loop no logout can break.
const legacyRootCookiePath = "/"

// clearSessionCookie expires the session cookie at every Path it may have been set on:
// the current base-path-scoped one, plus the legacy origin-root Path.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	paths := []string{s.path("/")}
	if paths[0] != legacyRootCookiePath {
		paths = append(paths, legacyRootCookiePath)
	}
	for _, path := range paths {
		http.SetCookie(w, &http.Cookie{
			Name:     s.cfg.Cookie.Name,
			Value:    "",
			Path:     path,
			HttpOnly: true,
			Secure:   s.cfg.Cookie.Secure,
			SameSite: sameSite(s.cfg.Cookie.SameSite),
			MaxAge:   -1,
		})
	}
}

// setTxCookie writes the short-lived OIDC login-transaction cookie.
func (s *Server) setTxCookie(w http.ResponseWriter, txID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     txCookieName,
		Value:    txID,
		Path:     s.path("/api/auth"),
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
		Path:     s.path("/api/auth"),
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
