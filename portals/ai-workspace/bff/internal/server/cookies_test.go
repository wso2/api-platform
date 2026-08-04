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
	"net/http/httptest"
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
