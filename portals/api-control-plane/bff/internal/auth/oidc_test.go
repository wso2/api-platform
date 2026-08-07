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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"api-control-plane-bff/internal/session"
)

// idpServer is a minimal mock IdP: discovery document + token endpoint. The
// token handler is supplied by each test so it can assert on the request
// (client auth method, PKCE verifier) and script the response.
func idpServer(t *testing.T, tokenHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"end_session_endpoint":   srv.URL + "/logout",
		})
	})
	mux.HandleFunc("/token", tokenHandler)
	srv = httptest.NewServer(mux)
	return srv
}

func baseOpts(srv *httptest.Server) OIDCOptions {
	return OIDCOptions{
		Authority:        srv.URL,
		Issuer:           srv.URL,
		Discovery:        true,
		ClientID:         "client-1",
		ClientSecret:     "s3cret",
		ClientAuthMethod: "client_secret_post",
		RedirectURL:      "https://console.example.com/api/auth/callback",
		Scopes:           "openid profile email",
	}
}

func TestOIDC_AuthCodeURL_ContainsRequiredParams(t *testing.T) {
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()

	o, err := NewOIDC(context.Background(), srv.Client(), baseOpts(srv), session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()

	authURL, txID, err := o.AuthCodeURL("/dashboard")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	if txID == "" {
		t.Fatal("expected a non-empty tx id")
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	q := u.Query()
	for _, key := range []string{"state", "nonce", "code_challenge"} {
		if q.Get(key) == "" {
			t.Errorf("authorize URL missing %q", key)
		}
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get("client_id") != "client-1" {
		t.Errorf("client_id = %q, want client-1", q.Get("client_id"))
	}
}

// A callback whose state doesn't match the transaction record must be rejected
// — this is what prevents CSRF against the login flow.
func TestOIDC_Callback_StateMismatch(t *testing.T) {
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("token endpoint must not be called when state mismatches")
	})
	defer srv.Close()

	o, err := NewOIDC(context.Background(), srv.Client(), baseOpts(srv), session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()

	_, _, txID, err := authCodeURLParts(t, o, "/return")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}

	_, _, err = o.Callback(context.Background(), txID, "wrong-state", "some-code")
	if _, ok := err.(ErrStateMismatch); !ok {
		t.Fatalf("err = %v, want ErrStateMismatch", err)
	}
}

// An id_token whose nonce doesn't match the transaction record must be
// rejected even though the code exchange itself succeeded — this is what
// binds the back-channel token to the specific login attempt.
func TestOIDC_Callback_NonceMismatch(t *testing.T) {
	idToken := makeJWT(map[string]any{"nonce": "attacker-supplied-nonce", "sub": "u1"})
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": makeJWT(map[string]any{"sub": "u1"}),
			"id_token":     idToken,
			"expires_in":   3600,
		})
	})
	defer srv.Close()

	o, err := NewOIDC(context.Background(), srv.Client(), baseOpts(srv), session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()

	state, _, txID, err := authCodeURLParts(t, o, "/return")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}

	_, _, err = o.Callback(context.Background(), txID, state, "some-code")
	if _, ok := err.(ErrNonceMismatch); !ok {
		t.Fatalf("err = %v, want ErrNonceMismatch", err)
	}
}

func TestOIDC_Callback_Success(t *testing.T) {
	var gotVerifier string
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotVerifier = r.PostForm.Get("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  makeJWT(map[string]any{"sub": "u1", "username": "alice"}),
			"refresh_token": "refresh-abc",
			"id_token":      currentIDToken,
			"expires_in":    3600,
		})
	})
	defer srv.Close()

	o, err := NewOIDC(context.Background(), srv.Client(), baseOpts(srv), session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()

	state, nonce, txID, err := authCodeURLParts(t, o, "/return-here")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	currentIDToken = makeJWT(map[string]any{"nonce": nonce})

	sess, ret, err := o.Callback(context.Background(), txID, state, "auth-code")
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	if ret != "/return-here" {
		t.Errorf("return URL = %q, want /return-here", ret)
	}
	if sess.RefreshToken != "refresh-abc" {
		t.Errorf("RefreshToken = %q, want refresh-abc", sess.RefreshToken)
	}
	if sess.User.Name != "alice" {
		t.Errorf("User.Name = %q, want alice", sess.User.Name)
	}
	if gotVerifier == "" {
		t.Error("expected a non-empty PKCE code_verifier sent to the token endpoint")
	}

	// A replayed txID must fail — transactions are single-use.
	if _, _, err := o.Callback(context.Background(), txID, state, "auth-code"); err == nil {
		t.Error("expected an error replaying an already-consumed transaction")
	}
}

// currentIDToken is set by the test after computing the real nonce, then read
// by the mock server's handler closure above.
var currentIDToken string

// authCodeURLParts calls AuthCodeURL and extracts state/nonce for assertions
// (the real login flow never needs the caller to see these — only the tx id
// travels to the browser via the tx cookie).
func authCodeURLParts(t *testing.T, o *OIDC, ret string) (state, nonce, txID string, err error) {
	t.Helper()
	authURL, txID, err := o.AuthCodeURL(ret)
	if err != nil {
		return "", "", "", err
	}
	u, err := url.Parse(authURL)
	if err != nil {
		return "", "", "", err
	}
	return u.Query().Get("state"), u.Query().Get("nonce"), txID, nil
}

func TestOIDC_ClientAuthMethod_SecretPost(t *testing.T) {
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.PostForm.Get("client_secret") != "s3cret" {
			t.Errorf("client_secret_post: form client_secret = %q, want s3cret", r.PostForm.Get("client_secret"))
		}
		if _, _, hasBasic := r.BasicAuth(); hasBasic {
			t.Error("client_secret_post must not also send HTTP Basic auth")
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 60})
	})
	defer srv.Close()

	opts := baseOpts(srv)
	opts.ClientAuthMethod = "client_secret_post"
	o, err := NewOIDC(context.Background(), srv.Client(), opts, session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()
	if _, err := o.exchange(context.Background(), "code", "verifier"); err != nil {
		t.Fatalf("exchange: %v", err)
	}
}

func TestOIDC_ClientAuthMethod_SecretBasic(t *testing.T) {
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client-1" || pass != "s3cret" {
			t.Errorf("client_secret_basic: BasicAuth = (%q,%q,%v), want (client-1,s3cret,true)", user, pass, ok)
		}
		r.ParseForm()
		if r.PostForm.Get("client_secret") != "" {
			t.Error("client_secret_basic must not also put the secret in the form body")
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 60})
	})
	defer srv.Close()

	opts := baseOpts(srv)
	opts.ClientAuthMethod = "client_secret_basic"
	o, err := NewOIDC(context.Background(), srv.Client(), opts, session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()
	if _, err := o.exchange(context.Background(), "code", "verifier"); err != nil {
		t.Fatalf("exchange: %v", err)
	}
}

// "none" is for a public client driven server-side by the BFF (some IdPs
// cannot register a confidential client) — no secret must be sent by any
// mechanism. PKCE is what protects this flow instead.
func TestOIDC_ClientAuthMethod_None(t *testing.T) {
	srv := idpServer(t, func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := r.BasicAuth(); ok {
			t.Error(`client_auth_method "none" must not send HTTP Basic auth`)
		}
		r.ParseForm()
		if r.PostForm.Get("client_secret") != "" {
			t.Error(`client_auth_method "none" must not send a client_secret in the form body`)
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 60})
	})
	defer srv.Close()

	opts := baseOpts(srv)
	opts.ClientAuthMethod = "none"
	opts.ClientSecret = "" // a public client has none
	o, err := NewOIDC(context.Background(), srv.Client(), opts, session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()
	if _, err := o.exchange(context.Background(), "code", "verifier"); err != nil {
		t.Fatalf("exchange: %v", err)
	}
}

// Discovery=false must make no HTTP call at all and use the explicit endpoints
// verbatim — this is what makes a non-conformant IdP (no discovery document)
// usable.
func TestOIDC_DiscoveryDisabled_UsesExplicitEndpoints(t *testing.T) {
	discoveryCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discoveryCalled = true
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 60})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opts := OIDCOptions{
		Discovery:             false,
		AuthorizationEndpoint: srv.URL + "/authorize",
		TokenEndpoint:         srv.URL + "/token",
		ClientID:              "client-1",
		ClientSecret:          "s3cret",
		ClientAuthMethod:      "client_secret_post",
		RedirectURL:           "https://console.example.com/api/auth/callback",
		Scopes:                "openid",
	}
	o, err := NewOIDC(context.Background(), srv.Client(), opts, session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()

	if discoveryCalled {
		t.Error("discovery endpoint was called despite Discovery: false")
	}
	authURL, _, err := o.AuthCodeURL("/x")
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	if _, err := url.Parse(authURL); err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	u, _ := url.Parse(authURL)
	if u.Scheme+"://"+u.Host+u.Path != srv.URL+"/authorize" {
		t.Errorf("authorize endpoint = %s, want the explicitly configured one", authURL)
	}
}

// Discovery=false with a missing required endpoint must fail at construction,
// not surface as a confusing runtime error on first login.
func TestOIDC_DiscoveryDisabled_MissingEndpointFailsAtConstruction(t *testing.T) {
	opts := OIDCOptions{
		Discovery:        false,
		ClientID:         "client-1",
		ClientAuthMethod: "client_secret_post",
	}
	if _, err := NewOIDC(context.Background(), http.DefaultClient, opts, session.DefaultClaimMapping(), time.Hour); err == nil {
		t.Fatal("expected an error when discovery is disabled and endpoints are unset")
	}
}

// A discovery document's issuer must match the configured expectation — but
// deliberately as a plain string comparison, since some IdPs (WSO2 Thunder)
// issue a bare name rather than a URL.
func TestOIDC_Discovery_IssuerMismatchRejected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "unexpected-issuer",
			"authorization_endpoint": "https://idp.example.com/authorize",
			"token_endpoint":         "https://idp.example.com/token",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opts := baseOpts(srv)
	opts.Issuer = "platform-idp" // bare-name issuer, deliberately not a URL
	if _, err := NewOIDC(context.Background(), srv.Client(), opts, session.DefaultClaimMapping(), time.Hour); err == nil {
		t.Fatal("expected an error when discovery's issuer disagrees with the configured issuer")
	}
}

func TestOIDC_LogoutURL_NoEndSessionEndpoint(t *testing.T) {
	opts := OIDCOptions{
		Discovery:             false,
		AuthorizationEndpoint: "https://idp.example.com/authorize",
		TokenEndpoint:         "https://idp.example.com/token",
		PostLogoutRedirectURL: "https://console.example.com/login",
		ClientAuthMethod:      "client_secret_post",
	}
	o, err := NewOIDC(context.Background(), http.DefaultClient, opts, session.DefaultClaimMapping(), time.Hour)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	defer o.Close()

	if got := o.LogoutURL("some-id-token"); got != opts.PostLogoutRedirectURL {
		t.Errorf("LogoutURL = %q, want the post-logout URL directly (no end_session_endpoint)", got)
	}
}
