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

package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/common/models"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const testSAUUID = "0198a1b2-0000-7000-8000-000000000001"

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// signToken signs claims; kid "" omits the header, "auto" uses the key's thumbprint.
func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	switch kid {
	case "":
	case "auto":
		tok.Header["kid"] = utils.RSAThumbprint(&key.PublicKey)
	default:
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func saClaims(iss string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss": iss, "sub": "sa:acme:ci-bot:" + testSAUUID, "aud": "platform-api",
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(), "organization": "org-1",
		constants.ServiceAccountTokenVersionClaim: 1,
	}
}

func humanClaims(iss string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{"iss": iss, "sub": "alice", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(), "organization": "org-1"}
}

func TestIssuerKeyMap_Verify(t *testing.T) {
	local, sa, retired, stranger := mustKey(t), mustKey(t), mustKey(t), mustKey(t)
	m, err := NewIssuerKeyMap("platform-api",
		IssuerKeys{Issuer: "platform-api", Kind: IssuerKindLocal, Current: &local.PublicKey},
		IssuerKeys{Issuer: "platform-api-sa", Kind: IssuerKindServiceAccount, Current: &sa.PublicKey, Retired: []*rsa.PublicKey{&retired.PublicKey}},
	)
	if err != nil {
		t.Fatal(err)
	}

	wrongAud := saClaims("platform-api-sa")
	wrongAud["aud"] = "other-service"

	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"login token without kid or aud", signToken(t, local, "", humanClaims("platform-api")), true},
		{"login token with a foreign kid falls back to the current key", signToken(t, local, "someone-elses-kid", humanClaims("platform-api")), true},
		{"SA token on the shared local issuer", signToken(t, local, "auto", saClaims("platform-api")), true},
		{"SA token on its own issuer", signToken(t, sa, "auto", saClaims("platform-api-sa")), true},
		{"SA token signed by a retired key", signToken(t, retired, "auto", saClaims("platform-api-sa")), true},
		{"SA token with no kid", signToken(t, sa, "", saClaims("platform-api-sa")), false},
		{"SA token with an unknown kid", signToken(t, sa, "nope", saClaims("platform-api-sa")), false},
		{"SA token with the wrong aud", signToken(t, sa, "auto", wrongAud), false},
		{"unknown issuer never falls through to another key", signToken(t, local, "", humanClaims("somebody-else")), false},
		{"SA issuer's key cannot sign for the local issuer", signToken(t, sa, "", humanClaims("platform-api")), false},
		{"key not registered anywhere", signToken(t, stranger, "auto", saClaims("platform-api-sa")), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.verify(tt.token)
			if (err == nil) != tt.ok {
				t.Fatalf("verify: ok=%v err=%v", tt.ok, err)
			}
		})
	}
}

func TestIssuerKeyMap_DuplicateIssuerFails(t *testing.T) {
	k := mustKey(t)
	_, err := NewIssuerKeyMap("platform-api",
		IssuerKeys{Issuer: "x", Kind: IssuerKindIDP},
		IssuerKeys{Issuer: "x", Kind: IssuerKindServiceAccount, Current: &k.PublicKey})
	if err == nil {
		t.Fatal("two entries claiming one issuer must fail")
	}
}

func TestIsServiceAccountToken(t *testing.T) {
	m, _ := NewIssuerKeyMap("platform-api",
		IssuerKeys{Issuer: "local", Kind: IssuerKindLocal},
		IssuerKeys{Issuer: "sa", Kind: IssuerKindServiceAccount},
		IssuerKeys{Issuer: "idp", Kind: IssuerKindIDP})
	cases := []struct {
		iss, sub string
		want     bool
	}{
		{"local", "sa:acme:ci-bot:" + testSAUUID, true},
		{"local", "alice", false},
		{"sa", "anything", true},
		{"idp", "sa:acme:ci-bot:" + testSAUUID, false},
		{"unknown", "sa:acme:ci-bot:" + testSAUUID, false},
	}
	for _, c := range cases {
		if got := m.IsServiceAccountToken(c.iss, c.sub); got != c.want {
			t.Errorf("IsServiceAccountToken(%q, %q) = %v, want %v", c.iss, c.sub, got, c.want)
		}
	}
}

// skip_validation trusts login tokens unverified, but never an SA token.
func TestSkipValidationStillVerifiesServiceAccountTokens(t *testing.T) {
	local, forger := mustKey(t), mustKey(t)
	m, _ := NewIssuerKeyMap("platform-api", IssuerKeys{Issuer: "platform-api", Kind: IssuerKindLocal, Current: &local.PublicKey})
	cfg := AuthConfig{SkipValidation: true, KeyMap: m, ClaimMappings: ClaimMappings{OrganizationClaim: "organization"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v0.9/x", nil)

	if _, err := validateLocalJWT(req, signToken(t, forger, "", humanClaims("platform-api")), cfg); err != nil {
		t.Fatalf("login token under skip_validation must pass unverified: %v", err)
	}
	if _, err := validateLocalJWT(req, signToken(t, forger, "auto", saClaims("platform-api")), cfg); err == nil {
		t.Fatal("forged SA token accepted under skip_validation")
	}
	if _, err := validateLocalJWT(req, signToken(t, local, "auto", saClaims("platform-api")), cfg); err != nil {
		t.Fatalf("genuine SA token rejected: %v", err)
	}
}

type fakeRevocations struct{ revoked map[string]int64 }

func (f fakeRevocations) IsRevoked(id string, version int64) bool {
	minVersion, ok := f.revoked[id]
	return ok && version < minVersion
}

func TestServiceAccountRevocationMiddleware(t *testing.T) {
	local := mustKey(t)
	m, _ := NewIssuerKeyMap("platform-api", IssuerKeys{Issuer: "platform-api", Kind: IssuerKindLocal, Current: &local.PublicKey})
	cfg := AuthConfig{KeyMap: m, ClaimMappings: ClaimMappings{OrganizationClaim: "organization"}}

	run := func(token string, revs fakeRevocations) (int, bool) {
		var sawSA bool
		h := LocalJWTAuthMiddleware(cfg)(ServiceAccountRevocationMiddleware(m, revs)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sawSA = IsServiceAccountRequest(r) })))
		req := httptest.NewRequest(http.MethodGet, "/api/v0.9/x", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, sawSA
	}

	saTok := signToken(t, local, "auto", saClaims("platform-api"))
	if code, sa := run(saTok, fakeRevocations{}); code != http.StatusOK || !sa {
		t.Fatalf("live SA token: code=%d sa=%v", code, sa)
	}
	future := fakeRevocations{revoked: map[string]int64{testSAUUID: 2}}
	if code, _ := run(saTok, future); code != http.StatusUnauthorized {
		t.Fatalf("revoked SA token: code=%d", code)
	}
	if code, sa := run(signToken(t, local, "", humanClaims("platform-api")), future); code != http.StatusOK || sa {
		t.Fatalf("human token must not touch the revocation check: code=%d sa=%v", code, sa)
	}
	bad := saClaims("platform-api")
	bad["sub"] = "sa:"
	if code, _ := run(signToken(t, local, "auto", bad), fakeRevocations{}); code != http.StatusUnauthorized {
		t.Fatalf("unparseable SA subject: code=%d", code)
	}
	noVersion := saClaims("platform-api")
	delete(noVersion, constants.ServiceAccountTokenVersionClaim)
	if code, _ := run(signToken(t, local, "auto", noVersion), fakeRevocations{}); code != http.StatusUnauthorized {
		t.Fatalf("SA token without a token version: code=%d", code)
	}
}

// An IdP may not mint an identity in the reserved service-account namespace.
func TestPlatformClaimsRejectsReservedSubjectFromIdP(t *testing.T) {
	run := func(sub string) int {
		h := PlatformClaimsMiddleware(ClaimMappings{OrganizationClaim: "organization"})(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		req := httptest.NewRequest(http.MethodGet, "/api/v0.9/x", nil)
		ctx := authenticators.WithAuthContext(req.Context(), models.AuthContext{
			Claims: jwt.MapClaims{"sub": sub, "organization": "org-1"},
		})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		return rec.Code
	}
	if code := run("sa:acme:ci-bot:" + testSAUUID); code != http.StatusUnauthorized {
		t.Fatalf("reserved IdP subject: code=%d", code)
	}
	if code := run("alice"); code != http.StatusOK {
		t.Fatalf("ordinary IdP subject: code=%d", code)
	}
}
