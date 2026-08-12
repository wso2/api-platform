/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// unsignedToken builds an alg:none JWT with the given claims, mirroring the
// unsigned internal token a trusted mediation layer forwards.
func unsignedToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	s, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign unsigned token: %v", err)
	}
	return s
}

func serve(mw func(http.Handler) http.Handler, token string, next http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v0.9/environments", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	return rec
}

// With SkipValidation, an unsigned token is accepted and its organization claim
// is enriched into the request context for GetOrganizationFromRequest.
func TestLocalJWTAuthMiddleware_SkipValidation_AcceptsUnsignedAndResolvesOrg(t *testing.T) {
	mw := LocalJWTAuthMiddleware(AuthConfig{SkipValidation: true})
	token := unsignedToken(t, jwt.MapClaims{"organization": "org-uuid-123", "sub": "system"})

	var gotOrg string
	var called bool
	rec := serve(mw, token, func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotOrg, _ = GetOrganizationFromRequest(r)
	})

	if !called {
		t.Fatalf("next handler not called; status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotOrg != "org-uuid-123" {
		t.Errorf("organization = %q, want org-uuid-123", gotOrg)
	}
}

// Even with SkipValidation, a token missing the organization claim is rejected.
func TestLocalJWTAuthMiddleware_SkipValidation_RejectsMissingOrgClaim(t *testing.T) {
	mw := LocalJWTAuthMiddleware(AuthConfig{SkipValidation: true})
	token := unsignedToken(t, jwt.MapClaims{"sub": "system"})

	called := false
	rec := serve(mw, token, func(http.ResponseWriter, *http.Request) { called = true })

	if called {
		t.Fatal("next handler should not be called when the org claim is absent")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// Without SkipValidation, an unsigned (alg:none) token is rejected — signature
// enforcement stays strict by default.
func TestLocalJWTAuthMiddleware_StrictByDefault_RejectsUnsigned(t *testing.T) {
	mw := LocalJWTAuthMiddleware(AuthConfig{SkipValidation: false})
	token := unsignedToken(t, jwt.MapClaims{"organization": "org-uuid-123"})

	called := false
	rec := serve(mw, token, func(http.ResponseWriter, *http.Request) { called = true })

	if called {
		t.Fatal("next handler should not be called for an unsigned token under strict validation")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
