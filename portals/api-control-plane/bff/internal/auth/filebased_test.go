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

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-control-plane-bff/internal/session"
)

func makeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pb, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(pb)
	return header + "." + payload + ".fakesignature"
}

func TestFileBased_Login_Success(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin", "organization": "org-1"})
	expiresAt := time.Now().Add(time.Hour).Unix()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/portal/v0.9/auth/login" {
			t.Errorf("path = %s, want /api/portal/v0.9/auth/login", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.PostForm.Get("username") != "admin" || r.PostForm.Get("password") != "secret" {
			t.Errorf("unexpected credentials: %v", r.PostForm)
		}
		json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": expiresAt})
	}))
	defer srv.Close()

	fb := NewFileBased(srv.Client(), srv.URL, "/api/portal/v0.9", 8*time.Hour, session.DefaultClaimMapping())
	sess, err := fb.Login(context.Background(), "admin", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sess.AccessToken != tok {
		t.Errorf("AccessToken = %q, want %q", sess.AccessToken, tok)
	}
	if sess.AccessExpiry.Unix() != expiresAt {
		t.Errorf("AccessExpiry = %v, want response's expires_at", sess.AccessExpiry)
	}
	if sess.User.Name != "admin" {
		t.Errorf("User.Name = %q, want admin", sess.User.Name)
	}
}

func TestFileBased_Login_InvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	fb := NewFileBased(srv.Client(), srv.URL, "/api/portal/v0.9", time.Hour, session.DefaultClaimMapping())
	_, err := fb.Login(context.Background(), "admin", "wrong")

	var bad ErrInvalidCredentials
	if !errors.As(err, &bad) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestFileBased_Login_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"not_a_token_field": "x"}`))
	}))
	defer srv.Close()

	fb := NewFileBased(srv.Client(), srv.URL, "/api/portal/v0.9", time.Hour, session.DefaultClaimMapping())
	if _, err := fb.Login(context.Background(), "admin", "secret"); err == nil {
		t.Fatal("expected an error for a response with no token field")
	}
}

func TestFileBased_Login_UpstreamServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	fb := NewFileBased(srv.Client(), srv.URL, "/api/portal/v0.9", time.Hour, session.DefaultClaimMapping())
	_, err := fb.Login(context.Background(), "admin", "secret")
	var bad ErrInvalidCredentials
	if errors.As(err, &bad) {
		t.Fatal("a 500 must not be reported as invalid credentials")
	}
	if err == nil {
		t.Fatal("expected an error for a 5xx upstream response")
	}
}

// AbsoluteExpiry must never exceed the configured absolute TTL, even when the
// token's own expiry is further out.
func TestFileBased_Login_AbsoluteTTLCapsExpiry(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})
	farFuture := time.Now().Add(30 * 24 * time.Hour).Unix()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": farFuture})
	}))
	defer srv.Close()

	shortTTL := time.Minute
	fb := NewFileBased(srv.Client(), srv.URL, "/api/portal/v0.9", shortTTL, session.DefaultClaimMapping())
	sess, err := fb.Login(context.Background(), "admin", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sess.AbsoluteExpiry.After(time.Now().Add(shortTTL + time.Second)) {
		t.Errorf("AbsoluteExpiry = %v, exceeds absolute TTL cap", sess.AbsoluteExpiry)
	}
}
