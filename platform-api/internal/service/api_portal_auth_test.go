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

package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// --- test helpers ------------------------------------------------------------

// newTestJWTConfig writes a fresh RSA private key to t.TempDir and returns a
// config.JWT pointing at it. Each test gets its own key so parallel runs stay
// isolated.
func newTestJWTConfig(t *testing.T) (*config.JWT, *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "signing.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	return &config.JWT{PrivateKeyFile: path}, &priv.PublicKey
}

// --- LocalAuthProvider tests ------------------------------------------------

func TestLocalAuthProvider_MintsVerifiableRS256(t *testing.T) {
	jwtCfg, pub := newTestJWTConfig(t)
	p := newLocalAuthProvider(jwtCfg)

	header, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatalf("AuthorizationHeader: %v", err)
	}
	if !strings.HasPrefix(header, "Bearer ") {
		t.Fatalf("expected Bearer prefix, got %q", header)
	}
	raw := strings.TrimPrefix(header, "Bearer ")

	parsed, err := jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			t.Fatalf("unexpected signing method: %v", token.Method)
		}
		return pub, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token failed to verify: %v (valid=%v)", err, parsed != nil && parsed.Valid)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "platform-api-system" {
		t.Errorf("sub: got %v", claims["sub"])
	}
	if claims["iss"] != "platform-api" {
		t.Errorf("iss: got %v", claims["iss"])
	}
	rolesIface, ok := claims["roles"].([]interface{})
	if !ok || len(rolesIface) != 1 || rolesIface[0] != "platform-api-system" {
		t.Errorf("roles claim: got %v", claims["roles"])
	}
}

func TestLocalAuthProvider_CachesToken(t *testing.T) {
	jwtCfg, _ := newTestJWTConfig(t)
	p := newLocalAuthProvider(jwtCfg)

	h1, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatalf("first mint: %v", err)
	}
	h2, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatalf("second mint: %v", err)
	}
	if h1 != h2 {
		t.Errorf("expected cached token to be reused; got %q vs %q", h1, h2)
	}
}

func TestLocalAuthProvider_InvalidateForcesRefresh(t *testing.T) {
	jwtCfg, _ := newTestJWTConfig(t)
	p := newLocalAuthProvider(jwtCfg)

	h1, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatalf("first mint: %v", err)
	}
	// Sleep a moment so iat/exp claims differ between mints; RS256 signing is
	// deterministic for identical inputs, so a same-second re-mint would
	// produce the same signature and defeat the assertion.
	time.Sleep(time.Second + 100*time.Millisecond)
	p.InvalidateCache()
	h2, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatalf("post-invalidate mint: %v", err)
	}
	if h1 == h2 {
		t.Errorf("expected fresh token after Invalidate; got same value")
	}
}

// --- ClientCredentialsAuthProvider tests ------------------------------------

// stsStub is a minimal STS token endpoint used to verify the request body and
// return a canned token response. Counts requests so tests can assert caching
// behaviour.
type stsStub struct {
	server     *httptest.Server
	calls      int32
	nextToken  string
	nextTTL    int
	nextStatus int
	lastForm   string
}

func newSTSStub() *stsStub {
	s := &stsStub{nextToken: "tok-1", nextTTL: 3600, nextStatus: 200}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.calls, 1)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.lastForm = r.Form.Encode()
		if s.nextStatus < 200 || s.nextStatus >= 300 {
			w.WriteHeader(s.nextStatus)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(clientCredentialsTokenResponse{
			AccessToken: s.nextToken,
			ExpiresIn:   s.nextTTL,
		})
	}))
	return s
}

func (s *stsStub) URL() string { return s.server.URL }
func (s *stsStub) Calls() int  { return int(atomic.LoadInt32(&s.calls)) }
func (s *stsStub) Close()      { s.server.Close() }

func TestClientCredentialsAuthProvider_FetchesAndFormatsHeader(t *testing.T) {
	sts := newSTSStub()
	t.Cleanup(sts.Close)
	sts.nextToken = "abc.def.ghi"

	p := newClientCredentialsAuthProvider(sts.URL(), "client-id", "client-secret", nil)
	header, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatalf("AuthorizationHeader: %v", err)
	}
	if header != "Bearer abc.def.ghi" {
		t.Errorf("header: got %q, want %q", header, "Bearer abc.def.ghi")
	}
	// Assert the request body carries the expected grant params.
	if !strings.Contains(sts.lastForm, "grant_type=client_credentials") ||
		!strings.Contains(sts.lastForm, "client_id=client-id") ||
		!strings.Contains(sts.lastForm, "client_secret=client-secret") {
		t.Errorf("STS form fields wrong: %q", sts.lastForm)
	}
}

func TestClientCredentialsAuthProvider_CachesUntilNearExpiry(t *testing.T) {
	sts := newSTSStub()
	t.Cleanup(sts.Close)

	p := newClientCredentialsAuthProvider(sts.URL(), "id", "secret", nil)
	for i := 0; i < 3; i++ {
		if _, err := p.AuthorizationHeader(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if sts.Calls() != 1 {
		t.Errorf("expected 1 STS call (cache hit for the rest); got %d", sts.Calls())
	}
}

func TestClientCredentialsAuthProvider_InvalidateForcesRefetch(t *testing.T) {
	sts := newSTSStub()
	t.Cleanup(sts.Close)

	p := newClientCredentialsAuthProvider(sts.URL(), "id", "secret", nil)
	if _, err := p.AuthorizationHeader(context.Background()); err != nil {
		t.Fatal(err)
	}
	p.InvalidateCache()
	if _, err := p.AuthorizationHeader(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sts.Calls() != 2 {
		t.Errorf("expected 2 STS calls after invalidate; got %d", sts.Calls())
	}
}

func TestClientCredentialsAuthProvider_NonSuccessStatusReturnsError(t *testing.T) {
	sts := newSTSStub()
	t.Cleanup(sts.Close)
	sts.nextStatus = 401

	p := newClientCredentialsAuthProvider(sts.URL(), "id", "wrong-secret", nil)
	_, err := p.AuthorizationHeader(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 from STS")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should surface the status code; got %v", err)
	}
}

func TestClientCredentialsAuthProvider_DefaultClientRefusesRedirects(t *testing.T) {
	// The default *http.Client the provider builds when caller passes hc=nil
	// must NOT follow redirects. A 3xx returned by the STS on the token
	// endpoint isn't a legitimate part of the client-credentials flow;
	// following it would re-send client_id + client_secret to whatever host
	// the redirect names. We treat the 3xx as a non-2xx and surface an
	// error.
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example.com/oauth2/token", http.StatusFound)
	}))
	t.Cleanup(redirecting.Close)

	p := newClientCredentialsAuthProvider(redirecting.URL, "id", "secret", nil)
	_, err := p.AuthorizationHeader(context.Background())
	if err == nil {
		t.Fatal("expected error when STS returns a redirect; got nil (client followed the redirect)")
	}
	if !strings.Contains(err.Error(), "302") {
		t.Errorf("error should surface the 3xx status code; got %v", err)
	}
}

func TestClientCredentialsAuthProvider_ConcurrentCallsIssueSingleFetch(t *testing.T) {
	sts := newSTSStub()
	t.Cleanup(sts.Close)

	p := newClientCredentialsAuthProvider(sts.URL(), "id", "secret", nil)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.AuthorizationHeader(context.Background())
		}()
	}
	wg.Wait()
	if sts.Calls() != 1 {
		t.Errorf("thundering-herd guard: expected 1 STS call, got %d", sts.Calls())
	}
}

// --- APIPortalAuthRegistry tests --------------------------------------------

func TestAPIPortalAuthRegistry_LocalRoundTrip(t *testing.T) {
	jwtCfg, _ := newTestJWTConfig(t)
	reg := NewAPIPortalAuthRegistry(jwtCfg, newTestVault(t), nil)
	portal := &model.APIPortal{Handle: "acme", AuthType: constants.APIPortalAuthTypeLocal}
	p, err := reg.Get(portal)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := p.AuthorizationHeader(context.Background()); err != nil {
		t.Fatalf("AuthorizationHeader: %v", err)
	}
}

func TestAPIPortalAuthRegistry_OAuth2DecryptsSecret(t *testing.T) {
	v := newTestVault(t)
	sts := newSTSStub()
	t.Cleanup(sts.Close)

	// Encrypt the plaintext secret the same way the service does on write.
	ciphertext, err := v.Encrypt(context.Background(), "s3cr3t")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	reg := NewAPIPortalAuthRegistry(nil, v, nil)
	portal := &model.APIPortal{
		Handle:   "acme",
		AuthType: constants.APIPortalAuthTypeOAuth2,
		AuthConfig: map[string]interface{}{
			constants.APIPortalAuthConfigKeySTSTokenURL:  sts.URL(),
			constants.APIPortalAuthConfigKeyClientID:     "cid",
			constants.APIPortalAuthConfigKeyClientSecret: encoded,
		},
	}
	p, err := reg.Get(portal)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := p.AuthorizationHeader(context.Background()); err != nil {
		t.Fatalf("AuthorizationHeader: %v", err)
	}
	// The provider should have sent the decrypted plaintext to the STS.
	if !strings.Contains(sts.lastForm, "client_secret=s3cr3t") {
		t.Errorf("provider did not decrypt clientSecret before sending; STS form: %q", sts.lastForm)
	}
}

func TestAPIPortalAuthRegistry_GetReturnsSameInstance(t *testing.T) {
	jwtCfg, _ := newTestJWTConfig(t)
	reg := NewAPIPortalAuthRegistry(jwtCfg, newTestVault(t), nil)
	portal := &model.APIPortal{Handle: "acme", AuthType: constants.APIPortalAuthTypeLocal}
	a, err := reg.Get(portal)
	if err != nil {
		t.Fatal(err)
	}
	b, err := reg.Get(portal)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("expected same cached instance across calls")
	}
}

func TestAPIPortalAuthRegistry_InvalidateEvicts(t *testing.T) {
	jwtCfg, _ := newTestJWTConfig(t)
	reg := NewAPIPortalAuthRegistry(jwtCfg, newTestVault(t), nil)
	portal := &model.APIPortal{Handle: "acme", AuthType: constants.APIPortalAuthTypeLocal}
	a, _ := reg.Get(portal)
	reg.Invalidate("acme")
	b, _ := reg.Get(portal)
	if a == b {
		t.Errorf("expected a fresh instance after Invalidate")
	}
}

func TestAPIPortalAuthRegistry_OAuth2MissingFieldsFails(t *testing.T) {
	reg := NewAPIPortalAuthRegistry(nil, newTestVault(t), nil)
	portal := &model.APIPortal{
		Handle:   "acme",
		AuthType: constants.APIPortalAuthTypeOAuth2,
		AuthConfig: map[string]interface{}{
			// stsTokenUrl missing.
			constants.APIPortalAuthConfigKeyClientID:     "cid",
			constants.APIPortalAuthConfigKeyClientSecret: "not-really-encrypted",
		},
	}
	if _, err := reg.Get(portal); err == nil {
		t.Fatal("expected error for missing oauth2 authConfig fields")
	}
}

func TestAPIPortalAuthRegistry_OAuth2BadCiphertextFails(t *testing.T) {
	reg := NewAPIPortalAuthRegistry(nil, newTestVault(t), nil)
	portal := &model.APIPortal{
		Handle:   "acme",
		AuthType: constants.APIPortalAuthTypeOAuth2,
		AuthConfig: map[string]interface{}{
			constants.APIPortalAuthConfigKeySTSTokenURL:  "https://sts",
			constants.APIPortalAuthConfigKeyClientID:     "cid",
			constants.APIPortalAuthConfigKeyClientSecret: "!!!not-base64!!!",
		},
	}
	if _, err := reg.Get(portal); err == nil {
		t.Fatal("expected error for non-base64 clientSecret")
	}
}
