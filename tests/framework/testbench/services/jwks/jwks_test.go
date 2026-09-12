/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package jwks

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return s
}

func TestServeJWKSReturnsTheSigningKeyAsAJWK(t *testing.T) {
	s := newTestService(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/jwks", nil)

	s.serveJWKS(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var set jsonWebKeySet
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("decoding JWKS response: %v", err)
	}
	if len(set.Keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(set.Keys))
	}
	key := set.Keys[0]
	if key.Kty != "RSA" || key.Use != "sig" || key.Alg != "RS256" || key.Kid != keyID {
		t.Errorf("key = %+v, want kty=RSA use=sig alg=RS256 kid=%s", key, keyID)
	}
	if key.N == "" || key.E == "" {
		t.Errorf("key modulus/exponent must not be empty: %+v", key)
	}

	// The published key must reconstruct the exact public key issueToken signs with.
	published := publicKeyFromJWK(t, key)
	if published.N.Cmp(s.privateKey.PublicKey.N) != 0 || published.E != s.privateKey.PublicKey.E {
		t.Errorf("published JWK does not match the service's signing key")
	}
}

func TestIssueTokenSignsAVerifiableTokenWithTheRequestedClaims(t *testing.T) {
	s := newTestService(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/token?issuer=https://issuer.example.com&scope=read+write&claim_org_id=org-42", nil)

	s.issueToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(rec.Body.String(), &claims, func(tok *jwt.Token) (any, error) {
		if tok.Header["kid"] != keyID {
			t.Errorf("token kid = %v, want %s", tok.Header["kid"], keyID)
		}
		return &s.privateKey.PublicKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		t.Fatalf("parsing/verifying the issued token: %v", err)
	}
	if !token.Valid {
		t.Fatalf("token reported invalid")
	}

	if claims["sub"] != "test-user" {
		t.Errorf("sub = %v, want test-user", claims["sub"])
	}
	if claims["iss"] != "https://issuer.example.com" {
		t.Errorf("iss = %v, want https://issuer.example.com", claims["iss"])
	}
	if claims["scope"] != "read write" {
		t.Errorf("scope = %v, want %q", claims["scope"], "read write")
	}
	if claims["org_id"] != "org-42" {
		t.Errorf("org_id = %v, want org-42", claims["org_id"])
	}
	aud, ok := claims["aud"].([]any)
	if !ok || len(aud) != 1 || aud[0] != "test-audience" {
		t.Errorf("aud = %v, want [test-audience]", claims["aud"])
	}
}

func TestIssueTokenDefaultsIssuerAndScopeWhenOmitted(t *testing.T) {
	s := newTestService(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/token", nil)

	s.issueToken(rec, req)

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(rec.Body.String(), &claims); err != nil {
		t.Fatalf("parsing the issued token: %v", err)
	}
	if claims["iss"] != defaultIssuer {
		t.Errorf("iss = %v, want %s", claims["iss"], defaultIssuer)
	}
	if claims["scope"] != "default" {
		t.Errorf("scope = %v, want default", claims["scope"])
	}
}

func TestIssueTokenRejectsUnexpectedConfiguredSecret(t *testing.T) {
	s := newTestService(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/token?expected_secret=right", nil)
	req.SetBasicAuth("client", "wrong")

	s.issueToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandlersRejectNonGetMethods(t *testing.T) {
	s := newTestService(t)
	for _, path := range []string{"/jwks", "/.well-known/jwks.json"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want %d", path, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestIssueTokenAcceptsClientCredentialsPost(t *testing.T) {
	s := newTestService(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("scope=read+write"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.issueToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decoding token response: %v", err)
	}
	if response.AccessToken == "" || response.TokenType != "Bearer" || response.Scope != "read write" {
		t.Fatalf("unexpected token response metadata: type=%q scope=%q token present=%t", response.TokenType, response.Scope, response.AccessToken != "")
	}
}

func TestServiceMetadata(t *testing.T) {
	s := newTestService(t)
	if s.Name() != "jwks" {
		t.Errorf("Name() = %q, want jwks", s.Name())
	}
	if s.Port() != Port {
		t.Errorf("Port() = %d, want %d", s.Port(), Port)
	}
	if s.Stateful() {
		t.Errorf("Stateful() = true, want false")
	}
}

// publicKeyFromJWK reconstructs an rsa.PublicKey from a JWK's base64url-encoded modulus
// and exponent, mirroring how a real verifier would consume the JWKS response.
func publicKeyFromJWK(t *testing.T, key jsonWebKey) rsa.PublicKey {
	t.Helper()
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		t.Fatalf("decoding n: %v", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		t.Fatalf("decoding e: %v", err)
	}
	return rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}
}
