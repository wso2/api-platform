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
		Cookie: config.CookieConfig{
			Name1:       "_ai_workspace_session_1",
			Name2:       "_ai_workspace_session_2",
			StatePrefix: "_ai_workspace_state_",
			Secure:      true,
			SameSite:    "lax",
		},
	}}
}

// The session is split across two cookies, both scoped to the app's base path, so a
// host serving several portals under different prefixes never forwards this session to
// the others.
func TestSetSessionCookieScopedToBasePath(t *testing.T) {
	s := cookieTestServer()
	rec := httptest.NewRecorder()
	s.setSessionCookie(rec, "jwt-value", time.Now().Add(time.Hour))

	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	names := map[string]bool{}
	for _, c := range cookies {
		names[c.Name] = true
		if c.Path != "/ai-workspace/" {
			t.Errorf("cookie %q Path = %q, want %q", c.Name, c.Path, "/ai-workspace/")
		}
		if !c.HttpOnly || !c.Secure {
			t.Errorf("cookie %q must stay HttpOnly and Secure, got HttpOnly=%v Secure=%v",
				c.Name, c.HttpOnly, c.Secure)
		}
	}
	if !names[s.cfg.Cookie.Name1] || !names[s.cfg.Cookie.Name2] {
		t.Fatalf("got cookies %v, want both %q and %q", names, s.cfg.Cookie.Name1, s.cfg.Cookie.Name2)
	}
}

// The JWT reassembled from the two cookie parts must equal the original value.
func TestSetSessionCookieRoundTrips(t *testing.T) {
	s := cookieTestServer()
	rec := httptest.NewRecorder()
	want := "header.payload-with-a-lot-of-scopes.signature"
	s.setSessionCookie(rec, want, time.Now().Add(time.Hour))

	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	got, ok := s.tokenFromCookie(req)
	if !ok || got != want {
		t.Fatalf("tokenFromCookie() = %q, %v, want %q, true", got, ok, want)
	}
}

// A missing second cookie must fail closed rather than silently returning a truncated
// JWT — see GO-AUTH-001.
func TestTokenFromCookieFailsClosedOnMissingPart(t *testing.T) {
	s := cookieTestServer()
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: s.cfg.Cookie.Name1, Value: "only-part-one"})

	if _, ok := s.tokenFromCookie(req); ok {
		t.Fatal("tokenFromCookie() = ok, want false when the second cookie is missing")
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

	wantNames := map[string]bool{
		s.cfg.Cookie.Name1:      true,
		s.cfg.Cookie.Name2:      true,
		config.LegacyCookieName: true,
	}
	byPath := map[string]map[string]bool{"/ai-workspace/": {}, "/": {}}
	for _, c := range rec.Result().Cookies() {
		// This server has no state codec, so clearSessionCookie also sweeps any
		// sealed state cookies orphaned by a switch to store = "memory". They are
		// not part of what this test is about — see TestDeleteClearsTheStateCookies.
		if strings.HasPrefix(c.Name, s.cfg.Cookie.StatePrefix) {
			if c.MaxAge >= 0 {
				t.Errorf("orphan state cookie %q not expired: MaxAge=%d", c.Name, c.MaxAge)
			}
			continue
		}
		if !wantNames[c.Name] {
			t.Fatalf("unexpected cookie %q", c.Name)
		}
		if c.MaxAge >= 0 || c.Value != "" {
			t.Errorf("cookie %q at Path %q not expired: MaxAge=%d Value=%q", c.Name, c.Path, c.MaxAge, c.Value)
		}
		if _, ok := byPath[c.Path]; !ok {
			t.Fatalf("unexpected Path %q", c.Path)
		}
		byPath[c.Path][c.Name] = true
	}
	for _, path := range []string{"/ai-workspace/", "/"} {
		for name := range wantNames {
			if !byPath[path][name] {
				t.Errorf("missing expiry for cookie %q at Path %q (all Set-Cookie: %v)",
					name, path, rec.Result().Header["Set-Cookie"])
			}
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
