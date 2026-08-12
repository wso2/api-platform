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

package oauth2generator

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	xoauth2 "golang.org/x/oauth2"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func validParams() map[string]interface{} {
	return map[string]interface{}{
		"tokenEndpoint": "https://idp.example.com/oauth2/token",
		"clientId":      "gateway-client",
		"clientSecret":  "s3cr3t",
	}
}

func newRequestHeaderCtx() *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{},
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          "/v1/chat/completions",
		Method:        http.MethodPost,
		Authority:     "gateway.example.com",
		Scheme:        "https",
	}
}

func newTestPolicy() *Policy {
	return &Policy{
		mode:          ModeTokenEndpoint,
		tokenEndpoint: "https://idp.example.com/oauth2/token",
		clientID:      "gateway-client",
		headerName:    defaultHeaderName,
		valuePrefix:   defaultValuePrefix,
	}
}

// fakeTokenSource is a tokenProvider test double that counts Purge() calls,
// for OnResponseHeaders tests that only care whether a purge happened, not
// the real cache mechanics (see token_cache_test.go for those).
type fakeTokenSource struct {
	purgeCalls  int
	purgeCalled bool
}

func (f *fakeTokenSource) Token() (*xoauth2.Token, error) {
	return &xoauth2.Token{AccessToken: "unused"}, nil
}

func (f *fakeTokenSource) Purge() {
	f.purgeCalls++
	f.purgeCalled = true
}

func newResponseHeaderCtx(status int) *policy.ResponseHeaderContext {
	return &policy.ResponseHeaderContext{
		SharedContext:   &policy.SharedContext{},
		ResponseHeaders: policy.NewHeaders(map[string][]string{}),
		ResponseStatus:  status,
	}
}

// ─── GetPolicy / param validation ────────────────────────────────────────────

func TestGetPolicy_ValidParams(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa, ok := p.(*Policy)
	if !ok {
		t.Fatalf("expected *Policy, got %T", p)
	}
	if oa.tokenEndpoint != "https://idp.example.com/oauth2/token" {
		t.Errorf("unexpected tokenEndpoint: %q", oa.tokenEndpoint)
	}
	if oa.clientID != "gateway-client" {
		t.Errorf("unexpected clientID: %q", oa.clientID)
	}
	if oa.grantType != GrantTypeClientCredentials {
		t.Errorf("expected grantType to default to %q when omitted, got %q", GrantTypeClientCredentials, oa.grantType)
	}
}

// TestGetPolicy_DefaultCacheStrategy_NeverTouchesRedis locks in that
// omitting systemParameters entirely (the common case) never constructs a
// Redis client - cacheStrategy defaults to "memory" - so GetPolicy succeeds
// with no Redis available at all.
func TestGetPolicy_DefaultCacheStrategy_NeverTouchesRedis(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	src, ok := p.(*Policy).tokenSource.(*redisCachingTokenSource)
	if !ok {
		t.Fatalf("expected *redisCachingTokenSource, got %T", p.(*Policy).tokenSource)
	}
	if src.redisClient != nil {
		t.Error("expected no redis client to be constructed under the default cacheStrategy: memory")
	}
}

func TestGetPolicy_ExplicitCacheStrategyRedis_ConstructsRedisClient(t *testing.T) {
	params := validParams()
	params["cacheStrategy"] = "redis"
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 6399}
	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	src := p.(*Policy).tokenSource.(*redisCachingTokenSource)
	if src.redisClient == nil {
		t.Error("expected a redis client to be constructed under cacheStrategy: redis")
	}
}

func TestGetPolicy_ExplicitGrantType(t *testing.T) {
	params := validParams()
	params["grantType"] = GrantTypeClientCredentials
	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)
	if oa.grantType != GrantTypeClientCredentials {
		t.Errorf("unexpected grantType: %q", oa.grantType)
	}
}

func TestGetPolicy_UnsupportedGrantType(t *testing.T) {
	// grantType exists precisely so a future grant can be added without a
	// schema-breaking change - but until that grant is actually implemented,
	// an unrecognized value must fail loudly at configuration time, not be
	// silently treated as client_credentials.
	params := validParams()
	params["grantType"] = "authorization_code"
	_, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err == nil {
		t.Fatal("expected error for unsupported grantType, got nil")
	}
	if !strings.Contains(err.Error(), "grantType") {
		t.Errorf("expected error to mention grantType, got: %v", err)
	}
}

// ─── clientAuthMethod ────────────────────────────────────────────────────────

func TestGetPolicy_ClientAuthMethod_DefaultsToBasic(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)
	if oa.clientAuthMethod != ClientAuthMethodBasic {
		t.Errorf("expected clientAuthMethod to default to %q when omitted, got %q", ClientAuthMethodBasic, oa.clientAuthMethod)
	}
}

func TestGetPolicy_ClientAuthMethod_ExplicitPost(t *testing.T) {
	params := validParams()
	params["clientAuthMethod"] = ClientAuthMethodPost
	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)
	if oa.clientAuthMethod != ClientAuthMethodPost {
		t.Errorf("unexpected clientAuthMethod: %q", oa.clientAuthMethod)
	}
}

func TestGetPolicy_ClientAuthMethod_InvalidValue(t *testing.T) {
	params := validParams()
	params["clientAuthMethod"] = "client_secret_jwt"
	_, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err == nil {
		t.Fatal("expected error for unsupported clientAuthMethod, got nil")
	}
	if !strings.Contains(err.Error(), "clientAuthMethod") {
		t.Errorf("expected error to mention clientAuthMethod, got: %v", err)
	}
}

// ─── tokenRequestTimeout / defaultTokenTTL ───────────────────────────────────

func TestValidateAndExtractParams_TimeoutAndTTLDefaults(t *testing.T) {
	p, err := validateAndExtractParams(validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.requestTimeout != defaultTokenRequestTimeout {
		t.Errorf("expected requestTimeout to default to %s, got %s", defaultTokenRequestTimeout, p.requestTimeout)
	}
	if p.tokenTTLFallback != defaultTokenTTLFallback {
		t.Errorf("expected tokenTTLFallback to default to %s, got %s", defaultTokenTTLFallback, p.tokenTTLFallback)
	}
}

func TestValidateAndExtractParams_TimeoutAndTTLExplicitOverride(t *testing.T) {
	params := validParams()
	params["tokenRequestTimeout"] = "2500ms"
	params["defaultTokenTTL"] = "30m"

	p, err := validateAndExtractParams(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.requestTimeout != 2500*time.Millisecond {
		t.Errorf("unexpected requestTimeout: %s", p.requestTimeout)
	}
	if p.tokenTTLFallback != 30*time.Minute {
		t.Errorf("unexpected tokenTTLFallback: %s", p.tokenTTLFallback)
	}
}

func TestValidateAndExtractParams_TimeoutAndTTLUnparsable_FallsBackToDefault(t *testing.T) {
	params := validParams()
	params["tokenRequestTimeout"] = "not-a-duration"
	params["defaultTokenTTL"] = "also-not-a-duration"

	p, err := validateAndExtractParams(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.requestTimeout != defaultTokenRequestTimeout {
		t.Errorf("expected unparsable tokenRequestTimeout to fall back to default %s, got %s", defaultTokenRequestTimeout, p.requestTimeout)
	}
	if p.tokenTTLFallback != defaultTokenTTLFallback {
		t.Errorf("expected unparsable defaultTokenTTL to fall back to default %s, got %s", defaultTokenTTLFallback, p.tokenTTLFallback)
	}
}

// ─── expiryBuffer ─────────────────────────────────────────────────────────────

func TestValidateAndExtractParams_ExpiryBufferDefaults(t *testing.T) {
	p, err := validateAndExtractParams(validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.expiryBuffer != defaultExpiryBuffer {
		t.Errorf("expected expiryBuffer to default to %s, got %s", defaultExpiryBuffer, p.expiryBuffer)
	}
}

func TestValidateAndExtractParams_ExpiryBufferExplicitOverride(t *testing.T) {
	params := validParams()
	params["expiryBuffer"] = "45s"

	p, err := validateAndExtractParams(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.expiryBuffer != 45*time.Second {
		t.Errorf("unexpected expiryBuffer: %s", p.expiryBuffer)
	}
}

func TestValidateAndExtractParams_ExpiryBufferUnparsable_FallsBackToDefault(t *testing.T) {
	params := validParams()
	params["expiryBuffer"] = "not-a-duration"

	p, err := validateAndExtractParams(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.expiryBuffer != defaultExpiryBuffer {
		t.Errorf("expected unparsable expiryBuffer to fall back to default %s, got %s", defaultExpiryBuffer, p.expiryBuffer)
	}
}

// TestValidateAndExtractParams_ExpiryBufferNegative_FallsBackToDefault locks
// in that a negative buffer (which would invert tokenFreshEnough's check,
// treating a token as fresh for longer the further past its expiry it gets)
// is rejected the same way getIntParam already rejects a negative retry
// count.
func TestValidateAndExtractParams_ExpiryBufferNegative_FallsBackToDefault(t *testing.T) {
	params := validParams()
	params["expiryBuffer"] = "-5s"

	p, err := validateAndExtractParams(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.expiryBuffer != defaultExpiryBuffer {
		t.Errorf("expected negative expiryBuffer to fall back to default %s, got %s", defaultExpiryBuffer, p.expiryBuffer)
	}
}

// TestClientCredentials_TokenRequestTimeout_BoundsHungIdP proves
// tokenRequestTimeout actually bounds the token-endpoint HTTP call - without
// it, golang.org/x/oauth2 falls back to http.DefaultClient (Timeout: 0, no
// bound at all), so a hung IdP would block a token fetch indefinitely.
func TestClientCredentials_TokenRequestTimeout_BoundsHungIdP(t *testing.T) {
	const idpDelay = 2 * time.Second
	const configuredTimeout = 100 * time.Millisecond
	// Generous upper bound: comfortably covers the ~0.5-1s of connection-retry
	// overhead the "point redis at 127.0.0.1:1" pattern adds on top of
	// configuredTimeout (see TestPasswordGrant_EndToEnd, which shows the same
	// overhead), while still being well under idpDelay - so this only passes
	// if the timeout actually aborted the request rather than waiting out
	// the full delay.
	const maxAcceptableElapsed = 1500 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(idpDelay)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "should-never-be-returned",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := validParams()
	params["tokenEndpoint"] = server.URL
	params["tokenRequestTimeout"] = configuredTimeout.String()
	// See TestPasswordGrant_EndToEnd for why Redis is pinned to an
	// unreachable address here.
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	start := time.Now()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	elapsed := time.Since(start)

	if elapsed >= maxAcceptableElapsed {
		t.Errorf("expected the %s timeout to abort the request well before the IdP's %s delay, took %s", configuredTimeout, idpDelay, elapsed)
	}

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected ImmediateResponse (timeout should fail the request), got %T", action)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway, got %d", resp.StatusCode)
	}
}

// TestClientCredentials_ClientSecretPost_EndToEnd proves clientAuthMethod:
// client_secret_post actually changes wire behavior for client_credentials -
// client_id/client_secret arrive as form fields, not a Basic auth header.
func TestClientCredentials_ClientSecretPost_EndToEnd(t *testing.T) {
	var gotAuthHeader, gotClientID, gotClientSecret string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		gotAuthHeader = r.Header.Get("Authorization")
		gotClientID = r.PostForm.Get("client_id")
		gotClientSecret = r.PostForm.Get("client_secret")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "cc-post-token-xyz",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := validParams()
	params["tokenEndpoint"] = server.URL
	params["clientAuthMethod"] = ClientAuthMethodPost
	// See TestPasswordGrant_EndToEnd for why Redis is pinned to an
	// unreachable address here.
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer cc-post-token-xyz" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}

	if gotAuthHeader != "" {
		t.Errorf("expected no Basic auth header with client_secret_post, got %q", gotAuthHeader)
	}
	if gotClientID != "gateway-client" {
		t.Errorf("expected client_id=gateway-client in form body, got %q", gotClientID)
	}
	if gotClientSecret != "s3cr3t" {
		t.Errorf("expected client_secret=s3cr3t in form body, got %q", gotClientSecret)
	}
}

// TestPasswordGrant_ClientSecretPost_EndToEnd proves clientAuthMethod:
// client_secret_post applies identically to the password grant, since both
// grants route through the same golang.org/x/oauth2 internal AuthStyle
// handling.
func TestPasswordGrant_ClientSecretPost_EndToEnd(t *testing.T) {
	var gotAuthHeader, gotClientID, gotClientSecret string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		gotAuthHeader = r.Header.Get("Authorization")
		gotClientID = r.PostForm.Get("client_id")
		gotClientSecret = r.PostForm.Get("client_secret")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "password-post-token-xyz",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := passwordGrantParams()
	params["tokenEndpoint"] = server.URL
	params["clientAuthMethod"] = ClientAuthMethodPost
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer password-post-token-xyz" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}

	if gotAuthHeader != "" {
		t.Errorf("expected no Basic auth header with client_secret_post, got %q", gotAuthHeader)
	}
	if gotClientID != "gateway-client" {
		t.Errorf("expected client_id=gateway-client in form body, got %q", gotClientID)
	}
	if gotClientSecret != "s3cr3t" {
		t.Errorf("expected client_secret=s3cr3t in form body, got %q", gotClientSecret)
	}
}

// ─── password grant (RFC 6749 Section 4.3) ──────────────────────────────────

func passwordGrantParams() map[string]interface{} {
	return map[string]interface{}{
		"grantType":     GrantTypePassword,
		"tokenEndpoint": "https://idp.example.com/oauth2/token",
		"clientId":      "gateway-client",
		"clientSecret":  "s3cr3t",
		"username":      "resource-owner",
		"password":      "hunter2",
	}
}

func TestGetPolicy_PasswordGrant_ValidParams(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, passwordGrantParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)
	if pol.grantType != GrantTypePassword {
		t.Errorf("unexpected grantType: %q", pol.grantType)
	}
}

func TestGetPolicy_PasswordGrant_MissingUsername(t *testing.T) {
	params := passwordGrantParams()
	delete(params, "username")
	_, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "username") {
		t.Errorf("expected error to mention username, got: %v", err)
	}
}

func TestGetPolicy_PasswordGrant_MissingPassword(t *testing.T) {
	params := passwordGrantParams()
	delete(params, "password")
	_, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("expected error to mention password, got: %v", err)
	}
}

func TestGetPolicy_ClientCredentials_UsernamePasswordNotRequired(t *testing.T) {
	// username/password are password-grant-only; client_credentials (the
	// default grantType) must not require them.
	params := validParams()
	if _, ok := params["username"]; ok {
		t.Fatal("test fixture unexpectedly sets username")
	}
	_, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPasswordGrant_EndToEnd exercises the real passwordTokenSource against
// an httptest server simulating a password-grant token endpoint - unlike
// client_credentials (which delegates entirely to the well-exercised
// golang.org/x/oauth2/clientcredentials package), the password grant's
// token-fetch path is new code in this policy, so it's worth a real,
// non-mocked-tokenFunc test.
func TestPasswordGrant_EndToEnd(t *testing.T) {
	var gotGrantType, gotUsername, gotPassword string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		gotGrantType = r.PostForm.Get("grant_type")
		gotUsername = r.PostForm.Get("username")
		gotPassword = r.PostForm.Get("password")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "password-grant-token-abc",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := passwordGrantParams()
	params["tokenEndpoint"] = server.URL
	params["username"] = "resource-owner"
	params["password"] = "hunter2"
	// Point Redis at a guaranteed-unreachable address. Without this, GetPolicy
	// defaults to localhost:6379 - if anything is actually listening there in
	// whatever environment this test runs in (a stray local Redis, a container
	// left running from manual testing), this test would silently read back a
	// previously-cached token instead of exercising the real HTTP round trip
	// it exists to verify, and gotGrantType/gotUsername/gotPassword below
	// would stay at their zero value with no indication why.
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer password-grant-token-abc" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}

	if gotGrantType != "password" {
		t.Errorf("expected token endpoint to receive grant_type=password, got %q", gotGrantType)
	}
	if gotUsername != "resource-owner" {
		t.Errorf("expected username=resource-owner, got %q", gotUsername)
	}
	if gotPassword != "hunter2" {
		t.Errorf("expected password=hunter2, got %q", gotPassword)
	}
}

func TestGetPolicy_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]interface{})
		wantErr string
	}{
		{
			name:    "missing tokenEndpoint",
			mutate:  func(p map[string]interface{}) { delete(p, "tokenEndpoint") },
			wantErr: "'tokenEndpoint' parameter is required",
		},
		{
			name:    "missing clientId",
			mutate:  func(p map[string]interface{}) { delete(p, "clientId") },
			wantErr: "'clientId' parameter is required",
		},
		{
			name:    "missing clientSecret",
			mutate:  func(p map[string]interface{}) { delete(p, "clientSecret") },
			wantErr: "'clientSecret' parameter is required",
		},
		{
			name:    "empty tokenEndpoint",
			mutate:  func(p map[string]interface{}) { p["tokenEndpoint"] = "   " },
			wantErr: "'tokenEndpoint' cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := validParams()
			tt.mutate(params)
			_, err := GetPolicy(policy.PolicyMetadata{}, params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// ─── auth path selection (token vs tokenEndpoint/clientId/clientSecret) ─────

func TestGetPolicy_NeitherAuthPathConfigured(t *testing.T) {
	_, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when neither 'bearerToken' nor the token-endpoint fields are configured")
	}
	if !strings.Contains(err.Error(), "'bearerToken'") || !strings.Contains(err.Error(), "tokenEndpoint") {
		t.Errorf("expected error to mention both auth paths, got: %v", err)
	}
}

func TestGetPolicy_BothAuthPathsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{"bearerToken plus full token-endpoint block", func(p map[string]interface{}) { p["bearerToken"] = "static-abc123" }},
		{"bearerToken plus clientId only", func(p map[string]interface{}) {
			p["bearerToken"] = "static-abc123"
			delete(p, "tokenEndpoint")
			delete(p, "clientSecret")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := validParams()
			tt.mutate(params)
			_, err := GetPolicy(policy.PolicyMetadata{}, params)
			if err == nil {
				t.Fatal("expected error when both auth paths are configured, got nil")
			}
			if !strings.Contains(err.Error(), "cannot be combined") {
				t.Errorf("expected a mutual-exclusion error, got: %v", err)
			}
		})
	}
}

func TestGetPolicy_StaticToken_ValidParams(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{"bearerToken": "static-abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)
	if oa.mode != ModeStaticToken {
		t.Errorf("expected mode %q, got %q", ModeStaticToken, oa.mode)
	}
	if len(oa.purgeStatusCodes) != 0 {
		t.Errorf("expected purgeStatusCodes to be forced empty for the static-token path, got %v", oa.purgeStatusCodes)
	}
	tok, err := oa.tokenFunc()
	if err != nil {
		t.Fatalf("unexpected error fetching the static token: %v", err)
	}
	if tok.AccessToken != "static-abc123" {
		t.Errorf("expected the configured token to be returned as-is, got %q", tok.AccessToken)
	}
}

func TestGetPolicy_StaticToken_OnRequestHeaders_EndToEnd(t *testing.T) {
	// Unlike the token-endpoint path, this never spins up an httptest server -
	// there is nothing to call; the configured value is injected as-is.
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{"bearerToken": "static-abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := oa.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if got := mods.HeadersToSet["Authorization"]; got != "Bearer static-abc123" {
		t.Errorf("unexpected Authorization header: %q", got)
	}
	if reqCtx.SharedContext.AuthContext.AuthType != AuthTypeStaticToken {
		t.Errorf("expected AuthType %q, got %q", AuthTypeStaticToken, reqCtx.SharedContext.AuthContext.AuthType)
	}
}

func TestGetPolicy_StaticToken_Mode_SkipsResponseHeaders(t *testing.T) {
	// tokenPurgeStatusCodes has no effect on the static-token path - there is
	// no fresher token to fetch on a rejection - so response-header
	// processing must stay off even if it's explicitly configured.
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"bearerToken":           "static-abc123",
		"tokenPurgeStatusCodes": []interface{}{401},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mode := p.(*Policy).Mode()
	if mode.ResponseHeaderMode != policy.HeaderModeSkip {
		t.Errorf("expected response header processing to stay off for the static-token path, got %v", mode.ResponseHeaderMode)
	}
}

// ─── headerName / valuePrefix ────────────────────────────────────────────────

func TestGetPolicy_HeaderName_DefaultsToAuthorization(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oa := p.(*Policy); oa.headerName != "Authorization" {
		t.Errorf("expected headerName to default to %q, got %q", "Authorization", oa.headerName)
	}
}

func TestGetPolicy_ValuePrefix_DefaultsToBearer(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, validParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oa := p.(*Policy); oa.valuePrefix != "Bearer" {
		t.Errorf("expected valuePrefix to default to %q, got %q", "Bearer", oa.valuePrefix)
	}
}

func TestOnRequestHeaders_CustomHeaderNameAndPrefix(t *testing.T) {
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"bearerToken": "static-abc123",
		"headerName":  "X-Api-Token",
		"valuePrefix": "Token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := oa.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods := action.(policy.UpstreamRequestHeaderModifications)

	if _, hasAuth := mods.HeadersToSet["Authorization"]; hasAuth {
		t.Error("expected no Authorization header when headerName is overridden")
	}
	if got := mods.HeadersToSet["X-Api-Token"]; got != "Token static-abc123" {
		t.Errorf("unexpected X-Api-Token header: %q", got)
	}
}

func TestOnRequestHeaders_EmptyValuePrefix_NoScheme(t *testing.T) {
	// An explicitly empty valuePrefix must be honored as "no prefix" rather
	// than silently reverting to the "Bearer" default - see
	// getStringParamOrDefault.
	p, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{
		"bearerToken": "static-abc123",
		"valuePrefix": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa := p.(*Policy)
	if oa.valuePrefix != "" {
		t.Fatalf("expected valuePrefix to stay empty, got %q", oa.valuePrefix)
	}

	reqCtx := newRequestHeaderCtx()
	action := oa.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods := action.(policy.UpstreamRequestHeaderModifications)
	if got := mods.HeadersToSet["Authorization"]; got != "static-abc123" {
		t.Errorf("expected the raw credential with no scheme prefix, got %q", got)
	}
}

func TestGetStringMapParam(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   map[string]string
	}{
		{
			name:   "absent",
			params: map[string]interface{}{},
			want:   nil,
		},
		{
			name:   "wrong type",
			params: map[string]interface{}{"tokenRequestParams": "scope=read"},
			want:   nil,
		},
		{
			name:   "single string value",
			params: map[string]interface{}{"tokenRequestParams": map[string]interface{}{"scope": "read write"}},
			want:   map[string]string{"scope": "read write"},
		},
		{
			name: "multiple values, trimmed",
			params: map[string]interface{}{"tokenRequestParams": map[string]interface{}{
				"scope":    "  read write  ",
				"resource": "https://api.example.com",
			}},
			want: map[string]string{
				"scope":    "read write",
				"resource": "https://api.example.com",
			},
		},
		{
			name:   "non-string value dropped",
			params: map[string]interface{}{"tokenRequestParams": map[string]interface{}{"scope": 123}},
			want:   nil,
		},
		{
			name:   "blank value dropped",
			params: map[string]interface{}{"tokenRequestParams": map[string]interface{}{"scope": "   "}},
			want:   nil,
		},
		{
			name:   "empty map",
			params: map[string]interface{}{"tokenRequestParams": map[string]interface{}{}},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringMapParam(tt.params, "tokenRequestParams")
			if len(got) != len(tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestGetPurgeStatusCodesParam(t *testing.T) {
	def := []int{401}
	tests := []struct {
		name   string
		params map[string]interface{}
		want   map[int]struct{}
	}{
		{
			name:   "absent falls back to default",
			params: map[string]interface{}{},
			want:   map[int]struct{}{401: {}},
		},
		{
			name:   "wrong type falls back to default",
			params: map[string]interface{}{"tokenPurgeStatusCodes": "401"},
			want:   map[int]struct{}{401: {}},
		},
		{
			name:   "custom list",
			params: map[string]interface{}{"tokenPurgeStatusCodes": []interface{}{401, 403}},
			want:   map[int]struct{}{401: {}, 403: {}},
		},
		{
			// An explicit empty list is honored as-is (disabling purging),
			// unlike an absent key - it must NOT fall back to the default.
			name:   "explicit empty list disables rather than falling back",
			params: map[string]interface{}{"tokenPurgeStatusCodes": []interface{}{}},
			want:   map[int]struct{}{},
		},
		{
			name:   "float64 and numeric-string entries coerced",
			params: map[string]interface{}{"tokenPurgeStatusCodes": []interface{}{float64(401), "403"}},
			want:   map[int]struct{}{401: {}, 403: {}},
		},
		{
			name:   "non-numeric string entries dropped",
			params: map[string]interface{}{"tokenPurgeStatusCodes": []interface{}{401, "not-a-code"}},
			want:   map[int]struct{}{401: {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPurgeStatusCodesParam(tt.params, "tokenPurgeStatusCodes", def)
			if len(got) != len(tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
			for k := range tt.want {
				if _, ok := got[k]; !ok {
					t.Errorf("missing expected code %d in %#v", k, got)
				}
			}
		})
	}
}

func TestGetPolicy_ParamsIsOptional(t *testing.T) {
	params := validParams()
	params["tokenRequestParams"] = map[string]interface{}{"scope": "chat.completions embeddings"}
	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = p
}

func TestClientCredentials_EndToEnd_ParamsReachTokenEndpoint(t *testing.T) {
	var gotScope, gotResource string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		gotScope = r.PostForm.Get("scope")
		gotResource = r.PostForm.Get("resource")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "cc-grant-token-xyz",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := validParams()
	params["tokenEndpoint"] = server.URL
	params["tokenRequestParams"] = map[string]interface{}{
		"scope":    "read write",
		"resource": "https://api.example.com",
	}
	// See TestPasswordGrant_EndToEnd for why Redis is pinned to an
	// unreachable address here.
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer cc-grant-token-xyz" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}

	if gotScope != "read write" {
		t.Errorf("expected token endpoint to receive scope=%q, got %q", "read write", gotScope)
	}
	if gotResource != "https://api.example.com" {
		t.Errorf("expected token endpoint to receive resource=%q, got %q", "https://api.example.com", gotResource)
	}
}

// ─── headerInjectingRoundTripper / tokenRequestHeaders ───────────────────────

// recordingRoundTripper is a minimal http.RoundTripper test double that
// records the last request it saw and returns a canned response, without
// actually dialing anything.
type recordingRoundTripper struct {
	lastRequest *http.Request
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.lastRequest = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}, nil
}

func TestHeaderInjectingRoundTripper_AddsConfiguredHeaders(t *testing.T) {
	next := &recordingRoundTripper{}
	rt := &headerInjectingRoundTripper{headers: map[string]string{"X-Api-Key": "secret-123"}, next: next}

	req, _ := http.NewRequest(http.MethodPost, "https://idp.example.com/token", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := next.lastRequest.Header.Get("X-Api-Key"); got != "secret-123" {
		t.Errorf("expected X-Api-Key header to reach the wrapped RoundTripper, got %q", got)
	}
}

// TestHeaderInjectingRoundTripper_IgnoresAuthorizationAndContentType locks
// in the safety guard: letting tokenRequestHeaders override either would
// silently break client_secret_basic (which sets Authorization itself) or
// corrupt the form-encoded request body.
func TestHeaderInjectingRoundTripper_IgnoresAuthorizationAndContentType(t *testing.T) {
	next := &recordingRoundTripper{}
	rt := &headerInjectingRoundTripper{
		headers: map[string]string{
			"Authorization": "Bearer should-not-apply",
			"Content-Type":  "application/json",
			"X-Allowed":     "yes",
		},
		next: next,
	}

	req, _ := http.NewRequest(http.MethodPost, "https://idp.example.com/token", nil)
	req.Header.Set("Authorization", "Basic original-creds")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := next.lastRequest.Header.Get("Authorization"); got != "Basic original-creds" {
		t.Errorf("expected the original Authorization header to survive untouched, got %q", got)
	}
	if got := next.lastRequest.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Errorf("expected the original Content-Type header to survive untouched, got %q", got)
	}
	if got := next.lastRequest.Header.Get("X-Allowed"); got != "yes" {
		t.Errorf("expected a non-reserved header to still be applied, got %q", got)
	}
}

// TestClientCredentials_EndToEnd_HeadersReachTokenEndpoint proves
// tokenRequestHeaders reaches the real token-endpoint HTTP call end to end,
// through GetPolicy/buildTokenSource, not just the RoundTripper in
// isolation.
func TestClientCredentials_EndToEnd_HeadersReachTokenEndpoint(t *testing.T) {
	var gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Ocp-Apim-Subscription-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "header-test-token",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := validParams()
	params["tokenEndpoint"] = server.URL
	params["tokenRequestHeaders"] = map[string]interface{}{"Ocp-Apim-Subscription-Key": "abc-123"}
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer header-test-token" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}
	if gotHeader != "abc-123" {
		t.Errorf("expected the token endpoint to receive Ocp-Apim-Subscription-Key=%q, got %q", "abc-123", gotHeader)
	}
}

// TestPasswordGrant_ScopeReachesTokenEndpoint locks in that
// "tokenRequestParams.scope" is honored for the password grant - mapped to
// xoauth2.Config.Scopes, the one extensibility point
// xoauth2.Config.PasswordCredentialsToken actually has (see
// buildTokenSource).
func TestPasswordGrant_ScopeReachesTokenEndpoint(t *testing.T) {
	var gotScope string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		gotScope = r.PostForm.Get("scope")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "password-grant-token-with-scope",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := passwordGrantParams()
	params["tokenEndpoint"] = server.URL
	params["username"] = "resource-owner"
	params["password"] = "hunter2"
	params["tokenRequestParams"] = map[string]interface{}{"scope": "profile email"}
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer password-grant-token-with-scope" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}
	if gotScope != "profile email" {
		t.Errorf("expected token endpoint to receive scope=%q, got %q", "profile email", gotScope)
	}
}

// TestPasswordGrant_NonScopeParamsHaveNoEffect locks in that every
// "tokenRequestParams" entry other than "scope" stays scoped to client_credentials
// (see oauth2Params.customParams) - setting one alongside grantType:
// password must not error, but must also not reach the token endpoint,
// since xoauth2.Config.PasswordCredentialsToken has no hook to forward
// anything but scope.
func TestPasswordGrant_NonScopeParamsHaveNoEffect(t *testing.T) {
	var gotResource string
	var sawResourceKey bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		_, sawResourceKey = r.PostForm["resource"]
		gotResource = r.PostForm.Get("resource")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "password-grant-token-no-resource",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := passwordGrantParams()
	params["tokenEndpoint"] = server.URL
	params["username"] = "resource-owner"
	params["password"] = "hunter2"
	params["tokenRequestParams"] = map[string]interface{}{"resource": "https://api.example.com"}
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	reqCtx := newRequestHeaderCtx()
	action := pol.OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if mods.HeadersToSet["Authorization"] != "Bearer password-grant-token-no-resource" {
		t.Errorf("unexpected Authorization header: %q", mods.HeadersToSet["Authorization"])
	}
	if sawResourceKey {
		t.Errorf("expected no resource field to reach the token endpoint for the password grant, got %q", gotResource)
	}
}

// ─── Mode ────────────────────────────────────────────────────────────────────

// TestMode covers newTestPolicy()'s zero-value purgeStatusCodes (nil, same
// as an explicit empty list) - response-phase processing must be skipped
// entirely when there is nothing to purge on.
func TestMode(t *testing.T) {
	p := newTestPolicy()
	mode := p.Mode()
	if mode.RequestHeaderMode != policy.HeaderModeProcess {
		t.Errorf("expected RequestHeaderMode PROCESS, got %v", mode.RequestHeaderMode)
	}
	if mode.RequestBodyMode != policy.BodyModeSkip {
		t.Errorf("expected RequestBodyMode SKIP (no body needed to inject a bearer token), got %v", mode.RequestBodyMode)
	}
	if mode.ResponseHeaderMode != policy.HeaderModeSkip || mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("expected response phase to be skipped entirely")
	}
}

// TestMode_PurgeEnabled_ProcessesResponseHeadersOnly locks in that a
// non-empty purgeStatusCodes turns on response-header processing (needed to
// read ResponseStatus in OnResponseHeaders) but never the response body -
// the status code is enough, so this stays safe for streamed responses.
func TestMode_PurgeEnabled_ProcessesResponseHeadersOnly(t *testing.T) {
	p := newTestPolicy()
	p.purgeStatusCodes = map[int]struct{}{http.StatusUnauthorized: {}}
	mode := p.Mode()
	if mode.ResponseHeaderMode != policy.HeaderModeProcess {
		t.Errorf("expected ResponseHeaderMode PROCESS when purgeStatusCodes is non-empty, got %v", mode.ResponseHeaderMode)
	}
	if mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("expected ResponseBodyMode SKIP - purging only needs the status code, got %v", mode.ResponseBodyMode)
	}
}

// ─── OnResponseHeaders ───────────────────────────────────────────────────────

func TestOnResponseHeaders_PurgesOnConfiguredStatus(t *testing.T) {
	fake := &fakeTokenSource{}
	p := newTestPolicy()
	p.tokenSource = fake
	p.purgeStatusCodes = map[int]struct{}{http.StatusUnauthorized: {}}

	action := p.OnResponseHeaders(context.Background(), newResponseHeaderCtx(http.StatusUnauthorized), nil)

	if _, ok := action.(policy.DownstreamResponseHeaderModifications); !ok {
		t.Fatalf("expected DownstreamResponseHeaderModifications (pass-through), got %T", action)
	}
	if fake.purgeCalls != 1 {
		t.Errorf("expected exactly one Purge() call, got %d", fake.purgeCalls)
	}
}

func TestOnResponseHeaders_NoPurgeOnUnconfiguredStatus(t *testing.T) {
	fake := &fakeTokenSource{}
	p := newTestPolicy()
	p.tokenSource = fake
	p.purgeStatusCodes = map[int]struct{}{http.StatusUnauthorized: {}}

	// 403 (insufficient scope) is deliberately not purged by default - see
	// defaultPurgeStatusCodes.
	p.OnResponseHeaders(context.Background(), newResponseHeaderCtx(http.StatusForbidden), nil)

	if fake.purgeCalls != 0 {
		t.Errorf("expected no Purge() call for a status not in purgeStatusCodes, got %d", fake.purgeCalls)
	}
}

func TestOnResponseHeaders_NoPurgeOnSuccess(t *testing.T) {
	fake := &fakeTokenSource{}
	p := newTestPolicy()
	p.tokenSource = fake
	p.purgeStatusCodes = map[int]struct{}{http.StatusUnauthorized: {}}

	p.OnResponseHeaders(context.Background(), newResponseHeaderCtx(http.StatusOK), nil)

	if fake.purgeCalls != 0 {
		t.Errorf("expected no Purge() call on a successful response, got %d", fake.purgeCalls)
	}
}

func TestOnResponseHeaders_DisabledWhenPurgeStatusCodesEmpty(t *testing.T) {
	fake := &fakeTokenSource{}
	p := newTestPolicy()
	p.tokenSource = fake
	p.purgeStatusCodes = map[int]struct{}{} // explicitly disabled

	p.OnResponseHeaders(context.Background(), newResponseHeaderCtx(http.StatusUnauthorized), nil)

	if fake.purgeCalls != 0 {
		t.Errorf("expected no Purge() call when purgeStatusCodes is empty, got %d", fake.purgeCalls)
	}
}

// TestGetPolicy_PurgeOnUpstreamStatus_EndToEnd wires the real
// redisCachingTokenSource (via GetPolicy, not a fake) through a full
// prime -> reuse -> upstream-401 -> purge -> refetch cycle, proving
// OnResponseHeaders actually reaches and clears the same cache
// OnRequestHeaders reads from - the fake-based tests above only prove
// OnResponseHeaders calls Purge(), not that the wiring is correct end to end.
func TestGetPolicy_PurgeOnUpstreamStatus_EndToEnd(t *testing.T) {
	var tokenCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		accessToken := "token-1"
		if tokenCalls > 1 {
			accessToken = "token-2"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": accessToken,
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer server.Close()

	params := validParams()
	params["tokenEndpoint"] = server.URL
	// See TestPasswordGrant_EndToEnd for why Redis is pinned to an
	// unreachable address here.
	params["redis"] = map[string]interface{}{"host": "127.0.0.1", "port": 1}

	p, err := GetPolicy(policy.PolicyMetadata{}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pol := p.(*Policy)

	firstAction := pol.OnRequestHeaders(context.Background(), newRequestHeaderCtx(), nil)
	firstMods, ok := firstAction.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", firstAction)
	}
	if firstMods.HeadersToSet["Authorization"] != "Bearer token-1" {
		t.Fatalf("unexpected first Authorization header: %q", firstMods.HeadersToSet["Authorization"])
	}

	secondAction := pol.OnRequestHeaders(context.Background(), newRequestHeaderCtx(), nil)
	secondMods := secondAction.(policy.UpstreamRequestHeaderModifications)
	if secondMods.HeadersToSet["Authorization"] != "Bearer token-1" {
		t.Fatalf("expected the second request to reuse the cached token, got %q", secondMods.HeadersToSet["Authorization"])
	}
	if tokenCalls != 1 {
		t.Fatalf("expected exactly 1 token-endpoint call before the purge, got %d", tokenCalls)
	}

	respAction := pol.OnResponseHeaders(context.Background(), newResponseHeaderCtx(http.StatusUnauthorized), nil)
	if _, ok := respAction.(policy.DownstreamResponseHeaderModifications); !ok {
		t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", respAction)
	}

	thirdAction := pol.OnRequestHeaders(context.Background(), newRequestHeaderCtx(), nil)
	thirdMods := thirdAction.(policy.UpstreamRequestHeaderModifications)
	if thirdMods.HeadersToSet["Authorization"] != "Bearer token-2" {
		t.Errorf("expected a fresh token after the purge, got %q", thirdMods.HeadersToSet["Authorization"])
	}
	if tokenCalls != 2 {
		t.Errorf("expected exactly 2 token-endpoint calls total (initial + post-purge), got %d", tokenCalls)
	}
}

// ─── OnRequestHeaders ────────────────────────────────────────────────────────

func TestOnRequestHeaders_Success(t *testing.T) {
	p := newTestPolicy()
	var calls int
	p.tokenFunc = func() (*xoauth2.Token, error) {
		calls++
		return &xoauth2.Token{AccessToken: "abc123", TokenType: "Bearer"}, nil
	}

	reqCtx := newRequestHeaderCtx()
	action := p.OnRequestHeaders(context.Background(), reqCtx, nil)

	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if got := mods.HeadersToSet["Authorization"]; got != "Bearer abc123" {
		t.Errorf("unexpected Authorization header: %q", got)
	}
	if calls != 1 {
		t.Errorf("expected exactly one token fetch, got %d", calls)
	}

	if reqCtx.SharedContext.AuthContext == nil {
		t.Fatal("expected AuthContext to be set")
	}
	if !reqCtx.SharedContext.AuthContext.Authenticated {
		t.Error("expected Authenticated=true on success")
	}
	if reqCtx.SharedContext.AuthContext.AuthType != AuthType {
		t.Errorf("unexpected AuthType: %q", reqCtx.SharedContext.AuthContext.AuthType)
	}
	if reqCtx.SharedContext.AuthContext.CredentialID != "gateway-client" {
		t.Errorf("unexpected CredentialID: %q", reqCtx.SharedContext.AuthContext.CredentialID)
	}
}

func TestOnRequestHeaders_ReusesCachedToken(t *testing.T) {
	// This exercises the policy's own call path, not oauth2.ReuseTokenSource's
	// internals (already covered by the x/oauth2 package itself) — it proves
	// OnRequestHeaders calls through tokenFunc once per request rather than
	// bypassing it or calling it more than once.
	p := newTestPolicy()
	var calls int
	p.tokenFunc = func() (*xoauth2.Token, error) {
		calls++
		return &xoauth2.Token{AccessToken: "reused-token"}, nil
	}

	for i := 0; i < 3; i++ {
		action := p.OnRequestHeaders(context.Background(), newRequestHeaderCtx(), nil)
		mods := action.(policy.UpstreamRequestHeaderModifications)
		if mods.HeadersToSet["Authorization"] != "Bearer reused-token" {
			t.Fatalf("request %d: unexpected Authorization header", i)
		}
	}
	if calls != 3 {
		t.Errorf("expected tokenFunc called once per request (3), got %d", calls)
	}
}

func TestOnRequestHeaders_TokenFetchFailure(t *testing.T) {
	p := newTestPolicy()
	p.tokenFunc = func() (*xoauth2.Token, error) {
		return nil, errors.New("token endpoint returned invalid_client")
	}

	reqCtx := newRequestHeaderCtx()
	action := p.OnRequestHeaders(context.Background(), reqCtx, nil)

	resp, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected ImmediateResponse on failure, got %T", action)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway, got %d", resp.StatusCode)
	}
	if strings.Contains(string(resp.Body), "invalid_client") {
		t.Error("response body must not leak the underlying token-endpoint error detail")
	}
	if !strings.Contains(string(resp.Body), "failed to authenticate request to upstream service") {
		t.Errorf("expected generic failure message in body, got %q", resp.Body)
	}

	if reqCtx.SharedContext.AuthContext == nil || reqCtx.SharedContext.AuthContext.Authenticated {
		t.Error("expected AuthContext.Authenticated=false on failure")
	}
}

func TestOnRequestHeaders_PreservesPreviousAuthContext(t *testing.T) {
	p := newTestPolicy()
	p.tokenFunc = func() (*xoauth2.Token, error) {
		return &xoauth2.Token{AccessToken: "abc123"}, nil
	}

	reqCtx := newRequestHeaderCtx()
	reqCtx.SharedContext.AuthContext = &policy.AuthContext{
		Authenticated: true,
		AuthType:      "jwt",
		Subject:       "end-user-123",
	}

	p.OnRequestHeaders(context.Background(), reqCtx, nil)

	got := reqCtx.SharedContext.AuthContext
	if got.AuthType != AuthType {
		t.Errorf("expected current AuthType %q, got %q", AuthType, got.AuthType)
	}
	if got.Previous == nil || got.Previous.AuthType != "jwt" || got.Previous.Subject != "end-user-123" {
		t.Fatal("expected the prior inbound auth context to be preserved via Previous")
	}
}

// ─── OnUpstreamAttemptRequestHeaders ─────────────────────────────────────────

// newTestPolicyWithMockTokenSource returns a *Policy wired to a fakeTokenSource
// that always succeeds, for tests that don't care about purge behavior.
func newTestPolicyWithMockTokenSource(t *testing.T) *Policy {
	t.Helper()
	p := newTestPolicy()
	p.tokenSource = &fakeTokenSource{}
	return p
}

// newTestPolicyWithMockTokenSourceTrackingPurge returns a *Policy wired to a
// fakeTokenSource, plus that same fakeTokenSource so the caller can assert on
// its purgeCalled field - matching the fakeTokenSource convention already
// used by the OnResponseHeaders purge tests above.
func newTestPolicyWithMockTokenSourceTrackingPurge(t *testing.T) (*Policy, *fakeTokenSource) {
	t.Helper()
	fake := &fakeTokenSource{}
	p := newTestPolicy()
	p.tokenSource = fake
	return p, fake
}

// newTestPolicyWithFailingTokenSource returns a *Policy whose token fetch
// always errors, following the same tokenFunc-override convention as
// TestOnRequestHeaders_TokenFetchFailure. tokenSource is still a fakeTokenSource
// (rather than left nil) so a Purge() call on AttemptCount > 1 has somewhere
// safe to land.
func newTestPolicyWithFailingTokenSource(t *testing.T) *Policy {
	t.Helper()
	p := newTestPolicy()
	p.tokenSource = &fakeTokenSource{}
	p.tokenFunc = func() (*xoauth2.Token, error) {
		return nil, errors.New("token endpoint returned invalid_client")
	}
	return p
}

func TestOnUpstreamAttemptRequestHeaders_AttemptOneUsesCachedToken(t *testing.T) {
	p := newTestPolicyWithMockTokenSource(t)
	actx := &policy.UpstreamAttemptContext{AttemptCount: 1, Headers: policy.NewHeaders(nil)}

	action := p.OnUpstreamAttemptRequestHeaders(context.Background(), actx)
	mods, ok := action.(policy.UpstreamAttemptHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamAttemptHeaderModifications, got %T", action)
	}
	if _, set := mods.HeadersToSet["Authorization"]; !set {
		t.Error("expected Authorization to be set even on attempt 1")
	}
}

func TestOnUpstreamAttemptRequestHeaders_RetryPurgesAndRefetches(t *testing.T) {
	p, mockSource := newTestPolicyWithMockTokenSourceTrackingPurge(t)
	actx := &policy.UpstreamAttemptContext{AttemptCount: 2, Headers: policy.NewHeaders(nil)}

	_ = p.OnUpstreamAttemptRequestHeaders(context.Background(), actx)

	if !mockSource.purgeCalled {
		t.Error("expected AttemptCount > 1 to purge the cached token before refetching")
	}
}

func TestOnUpstreamAttemptRequestHeaders_FetchErrorFailsOpen(t *testing.T) {
	p := newTestPolicyWithFailingTokenSource(t)
	actx := &policy.UpstreamAttemptContext{AttemptCount: 2, Headers: policy.NewHeaders(nil)}

	action := p.OnUpstreamAttemptRequestHeaders(context.Background(), actx)
	mods, ok := action.(policy.UpstreamAttemptHeaderModifications)
	if !ok || len(mods.HeadersToSet) != 0 {
		t.Errorf("expected an empty no-op action on fetch failure (fail open), got %#v", action)
	}
}

// ─── buildTokenEndpointTransport ─────────────────────────────────────────────

// writeTestCACert generates a throwaway self-signed certificate and writes
// it to a PEM file under t.TempDir(), for tests that need a real,
// parseable CA cert path rather than just exercising the error paths.
func writeTestCACert(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "oauth2-generator-test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create test certificate: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("failed to write test CA cert: %v", err)
	}
	return path
}

func TestBuildTokenEndpointTransport_DefaultsToProxyFromEnvironment(t *testing.T) {
	transport, err := buildTokenEndpointTransport(oauth2Params{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport.Proxy == nil {
		t.Fatal("expected a default Proxy func (http.ProxyFromEnvironment) to always be set")
	}
	if transport.TLSClientConfig != nil {
		t.Errorf("expected no TLSClientConfig when neither tlsCaCertPath nor tlsInsecureSkipVerify is set, got %+v", transport.TLSClientConfig)
	}
}

func TestBuildTokenEndpointTransport_ExplicitProxyURLOverridesEnvironment(t *testing.T) {
	transport, err := buildTokenEndpointTransport(oauth2Params{proxyURL: "http://proxy.internal:8080"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://idp.example.com/token", nil)
	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("unexpected error resolving proxy: %v", err)
	}
	if got == nil || got.Host != "proxy.internal:8080" {
		t.Errorf("expected the configured proxyURL to be used, got %v", got)
	}
}

func TestBuildTokenEndpointTransport_InvalidProxyURL(t *testing.T) {
	if _, err := buildTokenEndpointTransport(oauth2Params{proxyURL: "://not-a-valid-url"}); err == nil {
		t.Fatal("expected an error for an invalid proxyURL")
	}
}

// TestBuildTokenEndpointTransport_TLSConfigStillSetsProxy locks in the
// specific regression this function exists to prevent: jwt-auth and
// opaque-token-auth (sibling policies in this repo) both build a bare
// &http.Transport{TLSClientConfig: ...} when a custom CA/skip-verify is
// configured, which leaves Transport.Proxy nil - "never proxy," not
// "inherit the default" - silently dropping HTTP_PROXY/HTTPS_PROXY support
// the moment TLS customization is used. Proxy must still be set whenever
// TLSClientConfig is.
func TestBuildTokenEndpointTransport_TLSConfigStillSetsProxy(t *testing.T) {
	transport, err := buildTokenEndpointTransport(oauth2Params{tlsInsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected TLSClientConfig.InsecureSkipVerify to be true")
	}
	if transport.Proxy == nil {
		t.Fatal("expected Proxy to still be set even though a custom TLSClientConfig was built")
	}
}

func TestBuildTokenEndpointTransport_InvalidCACertPath(t *testing.T) {
	if _, err := buildTokenEndpointTransport(oauth2Params{tlsCaCertPath: "/nonexistent/path/ca.pem"}); err == nil {
		t.Fatal("expected an error for a missing CA cert file")
	}
}

func TestBuildTokenEndpointTransport_ValidCACertPath(t *testing.T) {
	certPath := writeTestCACert(t)
	transport, err := buildTokenEndpointTransport(oauth2Params{tlsCaCertPath: certPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("expected RootCAs to be populated from the CA cert file")
	}
}

func TestGetOrCreateTokenEndpointTransport_SharesTransportForIdenticalConfig(t *testing.T) {
	p1 := oauth2Params{proxyURL: "http://shared-proxy:8080"}
	p2 := oauth2Params{proxyURL: "http://shared-proxy:8080"}
	t1, err := getOrCreateTokenEndpointTransport(p1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t2, err := getOrCreateTokenEndpointTransport(p2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if t1 != t2 {
		t.Error("expected two configs with identical proxy/TLS settings to share one *http.Transport")
	}
}

// ─── isRetryableTokenError / retryBackoff ────────────────────────────────────

func TestIsRetryableTokenError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"plain network error", errors.New("dial tcp: connection refused"), true},
		{"429 too many requests", &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusTooManyRequests}}, true},
		{"500 internal server error", &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusInternalServerError}}, true},
		{"503 service unavailable", &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusServiceUnavailable}}, true},
		{"400 invalid_request", &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}, ErrorCode: "invalid_request"}, false},
		{"401 invalid_client", &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusUnauthorized}, ErrorCode: "invalid_client"}, false},
		{"RetrieveError with no response", &xoauth2.RetrieveError{ErrorCode: "invalid_client"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableTokenError(tt.err); got != tt.want {
				t.Errorf("isRetryableTokenError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryBackoff_WithinExpectedBounds(t *testing.T) {
	for attempt := 1; attempt <= 6; attempt++ {
		d := retryBackoff(attempt)
		if d < retryBaseDelay {
			t.Errorf("attempt %d: backoff %s below base delay %s", attempt, d, retryBaseDelay)
		}
		if d > retryMaxDelay+retryMaxDelay/2 {
			t.Errorf("attempt %d: backoff %s exceeds expected cap+jitter (%s)", attempt, d, retryMaxDelay+retryMaxDelay/2)
		}
	}
}

// ─── resilientTokenSource ────────────────────────────────────────────────────

// flakyTokenSource fails failTimes calls (with failErr) before succeeding
// with token - a test double for resilientTokenSource's retry path,
// distinct from stubTokenSource (which never varies its response).
type flakyTokenSource struct {
	failTimes int
	calls     int
	token     *xoauth2.Token
	failErr   error
}

func (f *flakyTokenSource) Token() (*xoauth2.Token, error) {
	f.calls++
	if f.calls <= f.failTimes {
		return nil, f.failErr
	}
	return f.token, nil
}

var errTransient = &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusServiceUnavailable}}

func TestResilientTokenSource_RetriesTransientFailureThenSucceeds(t *testing.T) {
	inner := &flakyTokenSource{failTimes: 2, failErr: errTransient, token: &xoauth2.Token{AccessToken: "eventually-ok"}}
	src := &resilientTokenSource{inner: inner, maxRetries: 2}

	tok, err := src.Token()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "eventually-ok" {
		t.Errorf("unexpected access token: %q", tok.AccessToken)
	}
	if inner.calls != 3 {
		t.Errorf("expected 3 total attempts (1 initial + 2 retries), got %d", inner.calls)
	}
}

func TestResilientTokenSource_GivesUpAfterMaxRetries(t *testing.T) {
	inner := &flakyTokenSource{failTimes: 100, failErr: errTransient}
	src := &resilientTokenSource{inner: inner, maxRetries: 2}

	if _, err := src.Token(); err == nil {
		t.Fatal("expected an error after exhausting all retries")
	}
	if inner.calls != 3 {
		t.Errorf("expected exactly 3 attempts (1 initial + 2 retries), got %d", inner.calls)
	}
}

func TestResilientTokenSource_NonRetryableErrorStopsImmediately(t *testing.T) {
	nonRetryable := &xoauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}, ErrorCode: "invalid_request"}
	inner := &flakyTokenSource{failTimes: 100, failErr: nonRetryable}
	src := &resilientTokenSource{inner: inner, maxRetries: 2}

	if _, err := src.Token(); err == nil {
		t.Fatal("expected an error")
	}
	if inner.calls != 1 {
		t.Errorf("expected exactly 1 attempt - a non-retryable error must not be retried, got %d calls", inner.calls)
	}
}
