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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"api-control-plane-bff/internal/config"
)

func TestRefreshCookieName_DerivedFromSession(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	cfg.Session.Cookie.Name = "_api_control_plane_session"
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	if got := srv.refreshCookieName(); got != "_api_control_plane_refresh" {
		t.Fatalf("refreshCookieName = %q", got)
	}
}

func TestSetSessionCookie_AccessUnchanged_RefreshSeparate(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	abs := time.Now().Add(8 * time.Hour)
	rec := httptest.NewRecorder()
	access := makeJWT(map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix()})
	srv.setSessionCookie(rec, access, "refresh-from-idp", abs)

	res := rec.Result()
	cookies := res.Cookies()
	byName := map[string]*http.Cookie{}
	for _, c := range cookies {
		byName[c.Name] = c
	}
	sess := byName[cfg.Session.Cookie.Name]
	ref := byName[srv.refreshCookieName()]
	if sess == nil || ref == nil {
		t.Fatalf("cookies = %v, want session + refresh", byName)
	}
	if sess.Value != access {
		t.Fatal("session cookie must stay the bare access JWT")
	}
	if !sess.HttpOnly || !ref.HttpOnly {
		t.Error("both cookies must be HttpOnly")
	}
	if ref.Value != "refresh-from-idp" {
		t.Errorf("refresh cookie = %q", ref.Value)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sess)
	req.AddCookie(ref)
	a, r, ok := srv.tokensFromCookie(req)
	if !ok || a != access || r != "refresh-from-idp" {
		t.Fatalf("tokensFromCookie = (%q,%q,%v)", a, r, ok)
	}
}

func TestSetSessionCookie_FileBasedClearsRefresh(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	rec := httptest.NewRecorder()
	srv.setSessionCookie(rec, "access-only", "stale-refresh", time.Now().Add(time.Hour))
	rec2 := httptest.NewRecorder()
	srv.setSessionCookie(rec2, "access-only", "", time.Now().Add(time.Hour))
	var cleared bool
	for _, c := range rec2.Result().Cookies() {
		if c.Name == srv.refreshCookieName() && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("expected refresh cookie to be cleared when refresh is empty")
	}
}

func TestRefreshUsingCookie_StoreMissRenewsFromRefreshCookie(t *testing.T) {
	n := 0
	var idp *httptest.Server
	idp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 idp.URL,
				"authorization_endpoint": idp.URL + "/authorize",
				"token_endpoint":         idp.URL + "/token",
			})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "cookie-refresh" {
			t.Fatalf("unexpected token request: %v", r.Form)
		}
		n++
		newAccess := makeJWT(map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix()})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newAccess,
			"refresh_token": "rotated-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer idp.Close()

	cfg := newTestConfig("https://unused.example.com")
	cfg.Auth.Mode = "oidc"
	cfg.Auth.OIDC = config.OIDCConfig{
		Enabled: true, Discovery: true, Authority: idp.URL, Issuer: idp.URL,
		ClientID: "client-1", ClientSecret: "s3cret", ClientAuthMethod: "client_secret_post",
		RedirectURL: "http://bff.example.com/api/auth/callback", Scopes: "openid",
	}
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	oldAccess := makeJWT(map[string]any{"sub": "u1", "exp": time.Now().Add(-time.Minute).Unix()})
	got, err := srv.refreshUsingCookie(context.Background(), oldAccess, "cookie-refresh")
	if err != nil {
		t.Fatalf("refreshUsingCookie: %v", err)
	}
	if n != 1 {
		t.Fatalf("token endpoint calls = %d, want 1", n)
	}
	if got.AccessToken == "" || got.AccessToken == oldAccess {
		t.Fatalf("expected a new access token, got %q", got.AccessToken)
	}
	if got.RefreshToken != "rotated-refresh" {
		t.Errorf("RefreshToken = %q, want rotated-refresh", got.RefreshToken)
	}
}

// TestRefreshByToken_CookieFallbackSingleFlight ensures concurrent store-miss
// renewals share one refresh_token grant under the per-token lock.
func TestRefreshByToken_CookieFallbackSingleFlight(t *testing.T) {
	var n atomic.Int32
	var idp *httptest.Server
	idp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 idp.URL,
				"authorization_endpoint": idp.URL + "/authorize",
				"token_endpoint":         idp.URL + "/token",
			})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("unexpected grant: %v", r.Form)
			http.Error(w, "bad grant", http.StatusBadRequest)
			return
		}
		n.Add(1)
		time.Sleep(50 * time.Millisecond) // widen the race window
		newAccess := makeJWT(map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix()})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newAccess,
			"refresh_token": "rotated-once",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer idp.Close()

	cfg := newTestConfig("https://unused.example.com")
	cfg.Auth.Mode = "oidc"
	cfg.Auth.OIDC = config.OIDCConfig{
		Enabled: true, Discovery: true, Authority: idp.URL, Issuer: idp.URL,
		ClientID: "client-1", ClientSecret: "s3cret", ClientAuthMethod: "client_secret_post",
		RedirectURL: "http://bff.example.com/api/auth/callback", Scopes: "openid",
	}
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	oldAccess := makeJWT(map[string]any{"sub": "u1", "exp": time.Now().Add(-time.Minute).Unix()})

	const goroutines = 8
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := srv.refreshByToken(context.Background(), oldAccess, "cookie-refresh")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("refreshByToken: %v", err)
		}
	}
	if got := n.Load(); got != 1 {
		t.Fatalf("token endpoint calls = %d, want 1 (single-flight)", got)
	}
}
