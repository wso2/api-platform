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

	"ai-workspace-bff/internal/auth"
	"ai-workspace-bff/internal/config"
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
// the supplied absolute expiry. The JWT is split across two HttpOnly cookies
// (Name1/Name2) so a single Set-Cookie value stays under browsers' and
// intermediate proxies' per-cookie size ceiling even when the JWT's scope list is
// large (see defaultOIDCScopes). Neither half is meaningful on its own; the proxy
// reads the reassembled JWT straight from the pair — no server-side lookup.
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
	part1, part2 := splitSessionToken(jwt)
	path := s.path("/")
	s.writeSessionCookiePart(w, s.cfg.Cookie.Name1, part1, path, maxAge)
	s.writeSessionCookiePart(w, s.cfg.Cookie.Name2, part2, path, maxAge)
}

func (s *Server) writeSessionCookiePart(w http.ResponseWriter, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: sameSite(s.cfg.Cookie.SameSite),
		MaxAge:   maxAge,
	})
}

// splitSessionToken divides the JWT into two arbitrary halves so it can be carried
// by two cookies instead of one. The split point carries no meaning on its own —
// joinSessionToken just concatenates the parts back in order.
func splitSessionToken(jwt string) (string, string) {
	mid := (len(jwt) + 1) / 2
	return jwt[:mid], jwt[mid:]
}

// joinSessionToken reassembles the JWT from its two session-cookie parts, in the same
// order splitSessionToken produced them. Used by tokenFromCookie (handlers.go).
func joinSessionToken(part1, part2 string) string {
	return part1 + part2
}

// legacyRootCookiePath is the origin-root Path the session cookie used before it moved
// under the app's base path. clearSessionCookie must expire it too: a browser keys a
// cookie by (name, domain, path), so an expiry written for one Path creates a separate
// cookie rather than removing an existing one at another Path. Left behind, a
// pre-upgrade cookie at "/" keeps matching every request below the root, so
// /api/session goes on reporting the stale session as authenticated while every proxied
// call 401s on its no-longer-verifiable token — a login loop no logout can break.
const legacyRootCookiePath = "/"

// legacyCookieNames are prior names the session was carried under before it was split
// into two parts. Clearing them alongside the current pair, for the same reason
// legacyRootCookiePath is also cleared: a browser keys a cookie by (name, domain,
// path), so an old name never expires on its own just because the BFF stopped setting
// it — a client that never logs out again after this upgrade would otherwise keep a
// stale single-cookie session sitting alongside the new pair indefinitely.
var legacyCookieNames = []string{config.LegacyCookieName}

// clearSessionCookie expires every cookie the session may be carried in — the current
// two-part pair plus any legacy single-cookie name — at every Path it may have been set
// on: the current base-path-scoped one, plus the legacy origin-root Path. The sealed
// sealed session-state cookies are not cleared here: they belong to the request's
// carrier, and every call site below is paired with a store.Delete that clears them.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	paths := []string{s.path("/")}
	if paths[0] != legacyRootCookiePath {
		paths = append(paths, legacyRootCookiePath)
	}
	names := append([]string{s.cfg.Cookie.Name1, s.cfg.Cookie.Name2}, legacyCookieNames...)
	for _, path := range paths {
		for _, name := range names {
			http.SetCookie(w, &http.Cookie{
				Name:     name,
				Value:    "",
				Path:     path,
				HttpOnly: true,
				Secure:   s.cfg.Cookie.Secure,
				SameSite: sameSite(s.cfg.Cookie.SameSite),
				MaxAge:   -1,
			})
		}
	}
}

// txCookiePath scopes the login-transaction cookie to the auth routes. It must cover
// the callback route ("/api/auth/callback"), or the browser never sends the cookie there
// and every login fails with a state mismatch — the transaction id simply absent,
// indistinguishable from a forged one. One helper rather than the literal twice,
// because setTxCookie and clearTxCookie must agree or the deletion silently misses.
func (s *Server) txCookiePath() string { return s.path("/api/auth") }

// setTxCookie writes the short-lived OIDC login-transaction cookie.
func (s *Server) setTxCookie(w http.ResponseWriter, txID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     txCookieName,
		Value:    txID,
		Path:     s.txCookiePath(),
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		// The transaction's lifetime plus the window the server keeps an expired
		// record around to explain itself. Validity is still TxTTL — Callback
		// rejects anything past Expiry — but the cookie has to outlast it, or an
		// aged-out login arrives with no cookie at all and is reported as a
		// Path/SameSite fault rather than as the expiry it was.
		MaxAge: int(auth.TxCookieTTL.Seconds()),
	})
}

// clearTxCookie must use the exact Path setTxCookie wrote, or the browser keeps the
// original cookie alongside the deletion and the next login reads a stale txID.
func (s *Server) clearTxCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     txCookieName,
		Value:    "",
		Path:     s.txCookiePath(),
		HttpOnly: true,
		Secure:   s.cfg.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
