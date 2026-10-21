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

func newSAHandlerForTest(t *testing.T) (*ServiceAccountHandler, *rsa.PrivateKey, *http.ServeMux) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewServiceAccountService(nil, nil, nil, nil,
		map[string][]string{"ap_sa_reader": {"ap:rest_api:read", "ap:gateway:read"}}, nil, time.Minute, config.AuthzModeScope, logger)
	h := NewServiceAccountHandler(svc, nil, []*rsa.PublicKey{&key.PublicKey}, logger)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, key, mux
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

func TestJWKSPublishesTheSigningKey(t *testing.T) {
	_, key, mux := newSAHandlerForTest(t)
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
	n, errN := base64.RawURLEncoding.DecodeString(jwks.Keys[0]["n"])
	e, errE := base64.RawURLEncoding.DecodeString(jwks.Keys[0]["e"])
	if errN != nil || errE != nil || new(big.Int).SetBytes(n).Cmp(key.PublicKey.N) != 0 ||
		new(big.Int).SetBytes(e).Int64() != int64(key.PublicKey.E) {
		t.Fatalf("published n/e do not match the signing key")
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
