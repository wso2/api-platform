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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const handlerTestSAUUID = "0198a1b2-0000-7000-8000-000000000001"

// newSAHandlerForTest publishes two keys: a verify-only one named "previous",
// then the returned key under its thumbprint.
func newSAHandlerForTest(t *testing.T) (*ServiceAccountHandler, *rsa.PrivateKey, *http.ServeMux) {
	t.Helper()
	key, previous := mustHandlerKey(t), mustHandlerKey(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewServiceAccountService(nil, nil, nil, nil,
		map[string][]string{"ap_sa_reader": {"ap:rest_api:read", "ap:gateway:read"}}, nil, time.Minute, config.AuthzModeScope, logger)
	h := NewServiceAccountHandler(svc, nil, []service.ServiceAccountKey{
		mustSAKey(t, &previous.PublicKey, "previous"),
		mustSAKey(t, &key.PublicKey, ""),
	}, 2*time.Minute, logger)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, key, mux
}

// mustSAKey is an RS256 key published under kid, or its thumbprint when kid is "".
func mustSAKey(t *testing.T, pub *rsa.PublicKey, kid string) service.ServiceAccountKey {
	t.Helper()
	jwk, err := utils.NewJWK(pub, "RS256", kid)
	if err != nil {
		t.Fatal(err)
	}
	return service.ServiceAccountKey{JWK: jwk, Public: pub}
}

func mustHandlerKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestServiceAccountManagementRefusesServiceAccounts(t *testing.T) {
	_, _, mux := newSAHandlerForTest(t)
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

func TestJWKSPublishesECKeys(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := utils.NewJWK(&ec.PublicKey, "ES384", "ec-key")
	if err != nil {
		t.Fatal(err)
	}
	rsaKey := mustSAKey(t, &mustHandlerKey(t).PublicKey, "rsa-key")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewServiceAccountHandler(service.NewServiceAccountService(nil, nil, nil, nil, nil, nil, time.Minute, config.AuthzModeScope, logger),
		nil, []service.ServiceAccountKey{rsaKey, {JWK: jwk, Public: &ec.PublicKey}}, time.Minute, logger)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, constants.JWKSPath, nil))
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 {
		t.Fatalf("jwks = %s", rec.Body)
	}
	if k := jwks.Keys[0]; k["kty"] != "RSA" || k["crv"] != "" || k["x"] != "" {
		t.Errorf("RSA key = %v", k)
	}
	k := jwks.Keys[1]
	if k["kty"] != "EC" || k["crv"] != "P-384" || k["alg"] != "ES384" || k["kid"] != "ec-key" || k["x"] == "" || k["y"] == "" || k["n"] != "" || k["e"] != "" {
		t.Errorf("EC key = %v", k)
	}
}

func TestJWKSPublishesEveryKeyInConfigOrder(t *testing.T) {
	_, key, mux := newSAHandlerForTest(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, constants.JWKSPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/jwk-set+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=120" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if etag := rec.Header().Get("ETag"); len(etag) != 34 || etag[0] != '"' || etag[33] != '"' {
		t.Errorf("ETag = %q, want 32 quoted hex digits", etag)
	}
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 || jwks.Keys[0]["kid"] != "previous" || jwks.Keys[1]["kid"] != utils.Thumbprint(&key.PublicKey) {
		t.Fatalf("jwks = %v", jwks)
	}
	for _, k := range jwks.Keys {
		if k["kty"] != "RSA" || k["use"] != "sig" || k["alg"] != "RS256" {
			t.Errorf("key %s = %v", k["kid"], k)
		}
	}
	n, errN := base64.RawURLEncoding.DecodeString(jwks.Keys[1]["n"])
	e, errE := base64.RawURLEncoding.DecodeString(jwks.Keys[1]["e"])
	if errN != nil || errE != nil || new(big.Int).SetBytes(n).Cmp(key.PublicKey.N) != 0 ||
		new(big.Int).SetBytes(e).Int64() != int64(key.PublicKey.E) {
		t.Fatalf("published n/e do not match the signing key")
	}
}

func TestJWKSConditionalRequest(t *testing.T) {
	_, _, mux := newSAHandlerForTest(t)
	first := httptest.NewRecorder()
	mux.ServeHTTP(first, httptest.NewRequest(http.MethodGet, constants.JWKSPath, nil))
	etag := first.Header().Get("ETag")

	for _, tt := range []struct {
		name    string
		headers []string
		want    int
	}{
		{"exact", []string{etag}, http.StatusNotModified},
		{"weak", []string{"W/" + etag}, http.StatusNotModified},
		{"any", []string{"*"}, http.StatusNotModified},
		{"in a list", []string{`"other", ` + etag}, http.StatusNotModified},
		{"in a second header", []string{`"other"`, etag}, http.StatusNotModified},
		{"stale", []string{`"0123"`}, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, constants.JWKSPath, nil)
			for _, h := range tt.headers {
				req.Header.Add("If-None-Match", h)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if rec.Header().Get("ETag") != etag || rec.Header().Get("Cache-Control") == "" {
				t.Errorf("ETag and Cache-Control must be on every answer: %v", rec.Header())
			}
			if tt.want == http.StatusNotModified && rec.Body.Len() != 0 {
				t.Errorf("a 304 must carry no body")
			}
		})
	}
}

func TestJWKSMethods(t *testing.T) {
	_, _, mux := newSAHandlerForTest(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	head, err := http.Head(srv.URL + constants.JWKSPath)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(head.Body)
	head.Body.Close()
	if head.StatusCode != http.StatusOK || len(body) != 0 || head.Header.Get("ETag") == "" {
		t.Errorf("HEAD: %d, %d body bytes, ETag %q", head.StatusCode, len(body), head.Header.Get("ETag"))
	}

	post, err := http.Post(srv.URL+constants.JWKSPath, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusMethodNotAllowed || post.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("POST: %d, Allow %q", post.StatusCode, post.Header.Get("Allow"))
	}
}

func TestClientCredentials(t *testing.T) {
	form := func(v url.Values) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		return r
	}

	r := form(url.Values{"grant_type": {"client_credentials"}, "client_id": {"id-1"}, "client_secret": {"s-1"}})
	if id, secret, err := clientCredentials(r); err != nil || id != "id-1" || secret != "s-1" {
		t.Fatalf("form: %q %q %v", id, secret, err)
	}

	// Basic values are form-urlencoded first: "a%3Ab" is the ID "a:b".
	r = form(url.Values{"grant_type": {"client_credentials"}})
	r.SetBasicAuth("a%3Ab", "s+1")
	if id, secret, err := clientCredentials(r); err != nil || id != "a:b" || secret != "s 1" {
		t.Fatalf("basic: %q %q %v", id, secret, err)
	}

	r = form(url.Values{"grant_type": {"client_credentials"}, "client_id": {"id-1"}})
	r.SetBasicAuth("id-1", "s-1")
	if _, _, err := clientCredentials(r); err == nil {
		t.Fatal("both methods at once must be refused")
	}

	r = form(url.Values{"grant_type": {"client_credentials"}})
	r.SetBasicAuth("%zz", "s-1")
	if _, _, err := clientCredentials(r); err == nil {
		t.Fatal("a malformed Basic value must be refused")
	}
}
