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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package oidc

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

const (
	testIssuer   = "https://testbench:3013/oauth2/token"
	testClient   = "portal-client"
	testCallback = "http://portal:9543/api-portal/default/callback"
	testVerifier = "a-code-verifier-long-enough-for-pkce-0123456789"
)

func newService(t *testing.T) *Service {
	t.Helper()
	s, err := New(nil, nil, testIssuer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return s
}

func login(t *testing.T, claims map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func authorizeURL(params map[string]string) string {
	values := url.Values{
		"response_type": {"code"}, "client_id": {testClient}, "redirect_uri": {testCallback},
		"state": {"st"}, "code_challenge": {pkceChallenge(testVerifier)}, "code_challenge_method": {"S256"},
	}
	for key, value := range params {
		if value == "" {
			values.Del(key)
			continue
		}
		values.Set(key, value)
	}
	return "/oauth2/authorize?" + values.Encode()
}

func authorize(t *testing.T, s *Service, loginHeader string, params map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, authorizeURL(params), nil)
	if loginHeader != "" {
		req.Header.Set(LoginHeader, loginHeader)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func redirectQuery(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body = %s", rec.Code, rec.Body.String())
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := location.Scheme + "://" + location.Host + location.Path; got != testCallback {
		t.Fatalf("redirected to %q, want %q", got, testCallback)
	}
	return location.Query()
}

func exchange(t *testing.T, s *Service, form url.Values, basicUser string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicUser != "" {
		req.SetBasicAuth(url.QueryEscape(basicUser), "secret")
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func tokenForm(code string) url.Values {
	return url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {testCallback}, "code_verifier": {testVerifier},
	}
}

func publicKey(t *testing.T, s *Service) *rsa.PublicKey {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth2/jwks", nil))
	var set struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil || len(set.Keys) != 1 {
		t.Fatalf("JWKS = %s, err = %v", rec.Body.String(), err)
	}
	key := set.Keys[0]
	if key["kid"] != keyID || key["alg"] != "RS256" {
		t.Fatalf("JWK = %v", key)
	}
	n, err := base64.RawURLEncoding.DecodeString(key["n"])
	if err != nil {
		t.Fatal(err)
	}
	e, err := base64.RawURLEncoding.DecodeString(key["e"])
	if err != nil {
		t.Fatal(err)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
}

func verify(t *testing.T, s *Service, raw string) jwt.MapClaims {
	t.Helper()
	key := publicKey(t, s)
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Header["kid"] != keyID {
			t.Errorf("kid = %v", token.Header["kid"])
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !parsed.Valid {
		t.Fatalf("token did not verify against the JWKS: %v", err)
	}
	return claims
}

func TestNewRejectsAHalfSuppliedOrInvalidKeyPair(t *testing.T) {
	cert, key, err := selfSigned()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][2][]byte{
		"certificate only": {cert, nil},
		"key only":         {nil, key},
		"not PEM":          {[]byte("certificate"), []byte("key")},
	}
	for name, pair := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := New(pair[0], pair[1], testIssuer); err == nil {
				t.Fatal("New() accepted an unusable TLS key pair")
			}
		})
	}
}

func TestNewServesTheSuppliedCertificate(t *testing.T) {
	cert, key, err := selfSigned()
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cert, key, testIssuer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	got := s.TLSConfig().Certificates
	if len(got) != 1 {
		t.Fatalf("certificates = %d, want 1", len(got))
	}
	block, _ := pemBlock(cert)
	if string(got[0].Certificate[0]) != string(block) {
		t.Fatal("TLSConfig() does not carry the supplied certificate")
	}
}

func TestFromEnvironmentReadsTheIssuerAndKeyPair(t *testing.T) {
	cert, key, err := selfSigned()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvTLSCert, string(cert))
	t.Setenv(EnvTLSKey, string(key))
	t.Setenv(EnvIssuer, testIssuer)
	s, err := FromEnvironment()
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}
	if s.issuer != testIssuer {
		t.Fatalf("issuer = %q, want %q", s.issuer, testIssuer)
	}
}

func TestTheServiceIsAStatelessTLSServiceTheTestbenchAccepts(t *testing.T) {
	s := newService(t)
	if s.Name() != "oidc" || s.Port() != Port || s.Stateful() {
		t.Fatalf("Name/Port/Stateful = %q/%d/%v", s.Name(), s.Port(), s.Stateful())
	}
	if err := (&testbench.Registry{}).Register(s); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
}

func TestTheHandlerAnswersOverTLS(t *testing.T) {
	cert, key, err := selfSigned()
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cert, key, testIssuer)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(s.Handler())
	server.TLS = s.TLSConfig()
	server.StartTLS()
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(cert)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "testbench"}}}
	resp, err := client.Get(server.URL + "/oauth2/jwks")
	if err != nil {
		t.Fatalf("GET over TLS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAnAuthorizationCodeExchangesForTokensCarryingTheLoginClaims(t *testing.T) {
	s := newService(t)
	header := login(t, map[string]any{"sub": "ivy", OrgClaim: "org-1", "org_name": "Initech", "roles": []string{"ap_admin"}})
	query := redirectQuery(t, authorize(t, s, header, map[string]string{"nonce": "n-1"}))
	if query.Get("state") != "st" || query.Get("code") == "" || query.Get("error") != "" {
		t.Fatalf("redirect query = %v", query)
	}

	rec := exchange(t, s, tokenForm(query.Get("code")), testClient)
	if rec.Code != http.StatusOK {
		t.Fatalf("token status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var tokens map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatal(err)
	}
	id := verify(t, s, tokens["id_token"].(string))
	access := verify(t, s, tokens["access_token"].(string))
	for name, claims := range map[string]jwt.MapClaims{"id": id, "access": access} {
		if claims["iss"] != testIssuer || claims["aud"] != testClient || claims["sub"] != "ivy" ||
			claims[OrgClaim] != "org-1" || claims["org_name"] != "Initech" {
			t.Errorf("%s token claims = %v", name, claims)
		}
		roles, _ := claims["roles"].([]any)
		if len(roles) != 1 || roles[0] != "ap_admin" {
			t.Errorf("%s token roles = %v", name, claims["roles"])
		}
	}
	if id["nonce"] != "n-1" || id["given_name"] != "ivy" {
		t.Errorf("id token = %v", id)
	}
	if access["client_id"] != testClient || access["scope"] != "openid profile email" {
		t.Errorf("access token = %v", access)
	}
}

func TestTheClientMayAuthenticateInTheFormBody(t *testing.T) {
	s := newService(t)
	query := redirectQuery(t, authorize(t, s, login(t, map[string]any{"sub": "u"}), nil))
	form := tokenForm(query.Get("code"))
	form.Set("client_id", testClient)
	if rec := exchange(t, s, form, ""); rec.Code != http.StatusOK {
		t.Fatalf("token status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSilentAuthorizationNeedsASessionInTheHintedOrganization(t *testing.T) {
	s := newService(t)
	member := login(t, map[string]any{"sub": "sam", OrgClaim: "org-x"})
	cases := []struct {
		name, header, hint string
		wantCode           bool
	}{
		{name: "no session", wantCode: false},
		{name: "session in another organization", header: member, hint: "org-y", wantCode: false},
		{name: "session in the hinted organization", header: member, hint: "org-x", wantCode: true},
		{name: "session and no hint", header: member, wantCode: true},
		{name: "session without an organization and a hint", header: login(t, map[string]any{"sub": "n"}), hint: "org-x", wantCode: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query := redirectQuery(t, authorize(t, s, tc.header, map[string]string{"prompt": "none", "org": tc.hint}))
			if query.Get("state") != "st" {
				t.Fatalf("state = %q", query.Get("state"))
			}
			if gotCode := query.Get("code") != ""; gotCode != tc.wantCode {
				t.Fatalf("code issued = %v, want %v (query %v)", gotCode, tc.wantCode, query)
			}
			if !tc.wantCode && query.Get("error") != "login_required" {
				t.Fatalf("error = %q, want login_required", query.Get("error"))
			}
		})
	}
}

func TestAnInteractiveAuthorizationWithoutAnIdentityIsRefused(t *testing.T) {
	rec := authorize(t, newService(t), "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthorizeRejectsMalformedRequests(t *testing.T) {
	s := newService(t)
	valid := login(t, map[string]any{"sub": "u"})
	cases := []struct {
		name, header string
		params       map[string]string
	}{
		{name: "missing redirect_uri", header: valid, params: map[string]string{"redirect_uri": ""}},
		{name: "relative redirect_uri", header: valid, params: map[string]string{"redirect_uri": "/callback"}},
		{name: "token response type", header: valid, params: map[string]string{"response_type": "token"}},
		{name: "missing client_id", header: valid, params: map[string]string{"client_id": ""}},
		{name: "plain code challenge", header: valid, params: map[string]string{"code_challenge_method": "plain"}},
		{name: "header not base64url", header: "not base64!"},
		{name: "header not an object", header: base64.RawURLEncoding.EncodeToString([]byte(`["a"]`))},
		{name: "header without sub", header: login(t, map[string]any{OrgClaim: "o"})},
		{name: "header too large", header: strings.Repeat("a", maxLoginHeader+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := authorize(t, s, tc.header, tc.params); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTheTokenEndpointRejectsInvalidGrants(t *testing.T) {
	s := newService(t)
	code := redirectQuery(t, authorize(t, s, login(t, map[string]any{"sub": "u"}), nil)).Get("code")
	encoded, _, _ := strings.Cut(code, ".")
	cases := []struct {
		name   string
		form   url.Values
		client string
	}{
		{name: "tampered code", form: tokenForm(encoded + ".AAAA"), client: testClient},
		{name: "unsigned code", form: tokenForm(encoded), client: testClient},
		{name: "missing code", form: tokenForm(""), client: testClient},
		{name: "another client", form: tokenForm(code), client: "other-client"},
		{name: "unauthenticated client", form: tokenForm(code)},
		{name: "another redirect_uri", form: func() url.Values {
			f := tokenForm(code)
			f.Set("redirect_uri", "http://elsewhere/callback")
			return f
		}(), client: testClient},
		{name: "wrong code verifier", form: func() url.Values {
			f := tokenForm(code)
			f.Set("code_verifier", "another-verifier")
			return f
		}(), client: testClient},
		{name: "unsupported grant type", form: func() url.Values {
			f := tokenForm(code)
			f.Set("grant_type", "client_credentials")
			return f
		}(), client: testClient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := exchange(t, s, tc.form, tc.client); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAnExpiredCodeIsRejected(t *testing.T) {
	s := newService(t)
	code := redirectQuery(t, authorize(t, s, login(t, map[string]any{"sub": "u"}), nil)).Get("code")
	s.now = func() time.Time { return time.Now().Add(codeTTL + time.Minute) }
	if rec := exchange(t, s, tokenForm(code), testClient); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestMintSignsTheSuppliedClaimsWithDefaults(t *testing.T) {
	s := newService(t)
	mint := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mint", strings.NewReader(body)))
		return rec
	}
	rec := mint(`{"sub":"nora","aud":"portal-client","org_id":["a","b"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	claims := verify(t, s, out["token"])
	if claims["iss"] != testIssuer || claims["aud"] != "portal-client" || claims["iat"] == nil || claims["exp"] == nil {
		t.Fatalf("claims = %v", claims)
	}
	if orgs, _ := claims[OrgClaim].([]any); len(orgs) != 2 {
		t.Fatalf("org claim = %v, want the list as supplied", claims[OrgClaim])
	}

	rec = mint(`{"sub":"s","iss":"https://elsewhere/oauth2/token"}`)
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if claims := verify(t, s, out["token"]); claims["iss"] != "https://elsewhere/oauth2/token" {
		t.Fatalf("iss = %v, want the supplied issuer", claims["iss"])
	}

	for _, body := range []string{``, `[]`, `null`, `not json`} {
		if rec := mint(body); rec.Code != http.StatusBadRequest {
			t.Errorf("mint(%q) status = %d, want 400", body, rec.Code)
		}
	}
}

func TestTheIssuerFollowsTheRequestHostWhenUnconfigured(t *testing.T) {
	s, err := New(nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "https://idp.example:3013/mint", strings.NewReader(`{"sub":"s"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if claims := verify(t, s, out["token"]); claims["iss"] != "https://idp.example:3013/oauth2/token" {
		t.Fatalf("iss = %v", claims["iss"])
	}
}

func TestConcurrentLoginsAreIndependent(t *testing.T) {
	s := newService(t)
	key := publicKey(t, s)
	loginAs := func(sub string) error {
		raw, _ := json.Marshal(map[string]any{"sub": sub})
		req := httptest.NewRequest(http.MethodGet, authorizeURL(nil), nil)
		req.Header.Set(LoginHeader, base64.RawURLEncoding.EncodeToString(raw))
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		location, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			return err
		}
		form := tokenForm(location.Query().Get("code"))
		form.Set("client_id", testClient)
		tokenReq := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tokenRec := httptest.NewRecorder()
		s.Handler().ServeHTTP(tokenRec, tokenReq)
		var tokens map[string]any
		if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokens); err != nil {
			return err
		}
		idToken, _ := tokens["id_token"].(string)
		claims := jwt.MapClaims{}
		if _, err := jwt.ParseWithClaims(idToken, claims, func(*jwt.Token) (any, error) { return key, nil }); err != nil {
			return err
		}
		if claims["sub"] != sub {
			return fmt.Errorf("login %q received identity %v", sub, claims["sub"])
		}
		return nil
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := loginAs(fmt.Sprintf("user-%d", i)); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func pemBlock(data []byte) ([]byte, bool) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, false
	}
	return block.Bytes, true
}
