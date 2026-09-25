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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-workspace-bff/internal/config"
)

func cookieTestServer() *Server {
	return &Server{cfg: &config.Config{
		Cookie: config.CookieConfig{Name: "_ai_workspace_session", Secure: true, SameSite: "lax"},
	}}
}

// The session cookie is scoped to the app's base path, so a host serving several
// portals under different prefixes never forwards this session to the others.
func TestSetSessionCookieScopedToBasePath(t *testing.T) {
	s := cookieTestServer()
	rec := httptest.NewRecorder()
	s.setSessionCookie(rec, "jwt-value", time.Now().Add(time.Hour))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	if cookies[0].Path != "/ai-workspace/" {
		t.Errorf("Path = %q, want %q", cookies[0].Path, "/ai-workspace/")
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Errorf("cookie must stay HttpOnly and Secure, got HttpOnly=%v Secure=%v",
			cookies[0].HttpOnly, cookies[0].Secure)
	}
}

// Regression test for a login loop: a session cookie set at the origin root before the
// app moved under a base path can only be removed by expiring it at that same Path,
// because a browser keys a cookie by (name, domain, path). Clearing just the current
// Path left the old cookie alive, so /api/session kept reporting the stale session as
// authenticated while every proxied call 401'd — and logout could never break out of it.
func TestClearSessionCookieAlsoClearsLegacyRootPath(t *testing.T) {
	s := cookieTestServer()
	rec := httptest.NewRecorder()
	s.clearSessionCookie(rec)

	byPath := map[string]int{}
	for _, c := range rec.Result().Cookies() {
		if c.Name != s.cfg.Cookie.Name {
			t.Fatalf("unexpected cookie %q", c.Name)
		}
		if c.MaxAge >= 0 || c.Value != "" {
			t.Errorf("cookie at Path %q not expired: MaxAge=%d Value=%q", c.Path, c.MaxAge, c.Value)
		}
		byPath[c.Path]++
	}
	for _, want := range []string{"/ai-workspace/", "/"} {
		if byPath[want] != 1 {
			t.Errorf("got %d expiries for Path %q, want exactly 1 (all Set-Cookie: %v)",
				byPath[want], want, rec.Result().Header["Set-Cookie"])
		}
	}
}

// TestTxCookieReachesTheCallback pins the invariant whose violation fails every
// login with "oidc state mismatch": a browser sends a cookie only to paths at or
// below its Path attribute, so the login-transaction cookie's Path MUST cover the
// callback route. When it does not, the callback receives no transaction id at all —
// which looks exactly like a forged state, and the error names neither cookies nor
// paths.
func TestTxCookieReachesTheCallback(t *testing.T) {
	s := oidcRoutesTestServer(t, "https://localhost:9643/ai-workspace/api/auth/callback")

	callbackPath := s.path("/api/auth/callback")
	if !strings.HasPrefix(callbackPath, s.txCookiePath()) {
		t.Errorf("cookie Path %q does not cover callback path %q — the browser would not send it",
			s.txCookiePath(), callbackPath)
	}

	// set and clear must agree, or the deletion silently misses and the next login
	// reads a stale transaction id.
	recSet := httptest.NewRecorder()
	s.setTxCookie(recSet, "tx-123")
	recClear := httptest.NewRecorder()
	s.clearTxCookie(recClear)
	setCookie := recSet.Result().Cookies()[0]
	clearCookie := recClear.Result().Cookies()[0]
	if setCookie.Path != clearCookie.Path {
		t.Errorf("set Path %q != clear Path %q — the cookie would survive the clear",
			setCookie.Path, clearCookie.Path)
	}
	if !setCookie.HttpOnly || setCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("tx cookie lost its HttpOnly/SameSite=Lax protection: %+v", setCookie)
	}
}
