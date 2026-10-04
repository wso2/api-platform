/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const handlerTestSAUUID = "0198a1b2-0000-7000-8000-000000000001"

type revokeSet map[string]bool

func (s revokeSet) IsRevoked(id string, _ int64) bool { return s[id] }

func newSAHandlerForTest(t *testing.T, revoked revokeSet) (*ServiceAccountHandler, *rsa.PrivateKey, *http.ServeMux) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyMap, err := middleware.NewIssuerKeyMap("platform-api",
		middleware.IssuerKeys{Issuer: "platform-api", Kind: middleware.IssuerKindLocal, Current: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	h := NewServiceAccountHandler(nil, nil, keyMap, revoked, []*rsa.PublicKey{&key.PublicKey}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, key, mux
}

func saToken(t *testing.T, key *rsa.PrivateKey, mutate func(jwt.MapClaims), withKid bool) string {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": "platform-api", "sub": "sa:acme:ci-bot:" + handlerTestSAUUID, "aud": "platform-api",
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(), "azp": "sa_acme_ci-bot_abc123", "scope": "ap:rest_api:read",
		constants.ServiceAccountTokenVersionClaim: 1,
	}
	if mutate != nil {
		mutate(claims)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if withKid {
		tok.Header["kid"] = utils.RSAThumbprint(&key.PublicKey)
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func introspect(mux *http.ServeMux, token string) string {
	req := httptest.NewRequest(http.MethodPost, constants.APIBasePath+"/service-accounts/introspect",
		strings.NewReader(url.Values{"token": {token}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Body.String()
}

// Every inactive cause must be byte-identical, or the endpoint is an oracle.
func TestIntrospectNegativesAreIdentical(t *testing.T) {
	_, key, mux := newSAHandlerForTest(t, revokeSet{handlerTestSAUUID: false})
	other, _ := rsa.GenerateKey(rand.Reader, 2048)

	causes := map[string]string{
		"unknown":      "not-a-jwt-at-all",
		"malformed":    "a.b.c",
		"expired":      saToken(t, key, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }, true),
		"wrong issuer": saToken(t, key, func(c jwt.MapClaims) { c["iss"] = "someone-else" }, true),
		"forged":       saToken(t, other, nil, true),
		"human token":  saToken(t, key, func(c jwt.MapClaims) { c["sub"] = "alice"; delete(c, "aud") }, false),
		"no version":   saToken(t, key, func(c jwt.MapClaims) { delete(c, constants.ServiceAccountTokenVersionClaim) }, true),
	}
	var first string
	for name, tok := range causes {
		body := introspect(mux, tok)
		if first == "" {
			first = body
		}
		if body != first {
			t.Errorf("%s: %q differs from %q", name, body, first)
		}
	}
	if strings.TrimSpace(first) != `{"active":false}` {
		t.Fatalf("negative body = %q", first)
	}

	_, key2, mux2 := newSAHandlerForTest(t, revokeSet{handlerTestSAUUID: true})
	if body := introspect(mux2, saToken(t, key2, nil, true)); body != first {
		t.Errorf("revoked: %q differs from %q", body, first)
	}
}

func TestIntrospectActive(t *testing.T) {
	_, key, mux := newSAHandlerForTest(t, revokeSet{})
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(introspect(mux, saToken(t, key, nil, true))), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["active"] != true || resp["client_id"] != "sa_acme_ci-bot_abc123" || resp["aud"] != "platform-api" {
		t.Fatalf("active response: %v", resp)
	}
}

func TestServiceAccountManagementRefusesServiceAccounts(t *testing.T) {
	_, _, mux := newSAHandlerForTest(t, revokeSet{})
	base := constants.APIBasePath + "/service-accounts"
	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, base},
		{http.MethodPost, base},
		{http.MethodGet, base + "/ci-bot"},
		{http.MethodPut, base + "/ci-bot"},
		{http.MethodDelete, base + "/ci-bot"},
		{http.MethodPost, base + "/ci-bot/regenerate-secret"},
	} {
		req := middleware.WithServiceAccount(httptest.NewRequest(probe.method, probe.path, nil), handlerTestSAUUID)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s by an SA token: %d, want 403", probe.method, probe.path, rec.Code)
		}
	}
}

func TestJWKSPublishesTheSigningKey(t *testing.T) {
	_, key, mux := newSAHandlerForTest(t, revokeSet{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, constants.APIBasePath+"/service-accounts/jwks.json", nil))
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 1 || jwks.Keys[0]["kid"] != utils.RSAThumbprint(&key.PublicKey) || jwks.Keys[0]["kty"] != "RSA" {
		t.Fatalf("jwks = %v", jwks)
	}
}
