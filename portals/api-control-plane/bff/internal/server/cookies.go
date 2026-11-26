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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Browsers drop a cookie whose name and value together pass ~4096 bytes, and an
// access token carrying every scope of a broad role (the cloud Admin holds 135
// ap:* scopes, ~5.6 KB) does not fit in one. The token is split across up to
// sessionMaxChunks cookies, each sized so its name and value stay within
// sessionCookieBudget (a margin under the browser limit), and a split session
// records its part count in a companion cookie so a read can tell a part is missing.
const (
	sessionCookieBudget = 4000
	sessionMaxChunks    = 4
	// minSessionChunk is the least value room a session cookie name may leave.
	minSessionChunk = 1024
	// sessionCountSuffix names the companion cookie holding a split session's part count.
	sessionCountSuffix = ".n"
)

// errSessionTooLarge is returned when an access token cannot fit in the session
// cookies, so the caller reports a failed sign-in instead of a session that breaks
// on the next request.
var errSessionTooLarge = errors.New("access token too large for the session cookies")

// sessionChunkSize is the value room each session cookie gets: the budget less the
// longest cookie name a session uses (the last part's), so every part fits.
func (s *Server) sessionChunkSize() int {
	return sessionCookieBudget - len(s.sessionChunkName(sessionMaxChunks-1)) - len("=")
}

// validateSessionCookieName rejects a configured session cookie name too long to
// leave room for a token value; a session written under it would always be dropped.
func validateSessionCookieName(name string) error {
	room := sessionCookieBudget - len(name) - len("."+strconv.Itoa(sessionMaxChunks-1)) - len("=")
	if room < minSessionChunk {
		return fmt.Errorf("session.cookie.name is %d bytes, leaving %d bytes for its value; it must leave at least %d", len(name), room, minSessionChunk)
	}
	return nil
}

func (s *Server) sessionCountName() string { return s.cfg.Session.Cookie.Name + sessionCountSuffix }

// sessionChunkName names the i-th session cookie: the first keeps the configured
// name, so a token that fits stays one cookie exactly as before; later ones add ".<i>".
func (s *Server) sessionChunkName(i int) string {
	if i == 0 {
		return s.cfg.Session.Cookie.Name
	}
	return s.cfg.Session.Cookie.Name + "." + strconv.Itoa(i)
}

// splitToken cuts token into values of at most size bytes. A JWT is ASCII, so a
// byte boundary never splits a character.
func splitToken(token string, size int) []string {
	var chunks []string
	for len(token) > size {
		chunks = append(chunks, token[:size])
		token = token[size:]
	}
	return append(chunks, token)
}

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

// setSessionCookie writes the access token across the session cookies (one
// cookie when it fits, see sessionChunkName). It returns errSessionTooLarge, and
// writes no session, when the token needs more than sessionMaxChunks cookies. When refresh is non-empty it also
// writes the companion refresh cookie; when refresh is empty the refresh
// cookie is cleared so a file-based login cannot leave a stale OIDC refresh.
func (s *Server) setSessionCookie(w http.ResponseWriter, access, refresh string, absExpiry time.Time) error {
	maxAge := 0
	if !absExpiry.IsZero() {
		if d := time.Until(absExpiry); d > 0 {
			maxAge = int(d.Seconds())
		}
	}
	chunks := splitToken(access, s.sessionChunkSize())
	if len(chunks) > sessionMaxChunks {
		// Too large for any browser to send back: write nothing rather than a
		// token cut short, and tell the caller, which must not report success.
		s.clearSessionCookie(w)
		return errSessionTooLarge
	}
	for i, chunk := range chunks {
		http.SetCookie(w, &http.Cookie{
			Name:     s.sessionChunkName(i),
			Value:    chunk,
			Path:     "/",
			HttpOnly: true,
			Secure:   s.cfg.Session.Cookie.Secure,
			SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
			MaxAge:   maxAge,
		})
	}
	// A shorter token than the last one leaves chunks behind; clear them so a
	// read never joins a stale tail onto the new token.
	s.clearSessionChunks(w, len(chunks))
	if len(chunks) > 1 {
		http.SetCookie(w, &http.Cookie{
			Name:     s.sessionCountName(),
			Value:    strconv.Itoa(len(chunks)),
			Path:     "/",
			HttpOnly: true,
			Secure:   s.cfg.Session.Cookie.Secure,
			SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
			MaxAge:   maxAge,
		})
	} else {
		s.expireCookie(w, s.sessionCountName())
	}
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
	return nil
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
	s.clearSessionChunks(w, 0)
	s.expireCookie(w, s.sessionCountName())
	s.clearRefreshCookie(w)
}

// clearSessionChunks expires the session cookies from chunk index from on.
func (s *Server) clearSessionChunks(w http.ResponseWriter, from int) {
	for i := from; i < sessionMaxChunks; i++ {
		s.expireCookie(w, s.sessionChunkName(i))
	}
}

// expireCookie tells the browser to drop the named session cookie.
func (s *Server) expireCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Session.Cookie.Secure,
		SameSite: sameSite(s.cfg.Session.Cookie.SameSite),
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
