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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package oauth2

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenEndpointSupportsBasicAndPostAuthentication(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	h := s.Handler()

	basic := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader("grant_type=client_credentials&scope=read"))
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(basic, req)
	if basic.Code != http.StatusOK || !strings.Contains(basic.Body.String(), "access_token") {
		t.Fatalf("basic token response = %d %q", basic.Code, basic.Body.String())
	}

	post := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader("grant_type=client_credentials&client_id="+clientID+"&client_secret="+clientSecret))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(post, req)
	if post.Code != http.StatusOK {
		t.Fatalf("post token status = %d, want %d", post.Code, http.StatusOK)
	}

	stats := httptest.NewRecorder()
	h.ServeHTTP(stats, httptest.NewRequest(http.MethodGet, "/block-a/debug/stats", nil))
	var got struct {
		Count int `json:"tokenRequestCount"`
	}
	if err := json.Unmarshal(stats.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if got.Count != 2 {
		t.Errorf("token request count = %d, want 2", got.Count)
	}
}

func TestHandlerNormalizesHTTPMethodBeforeDispatch(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("post", "/block-a/oauth2/token", strings.NewReader(
		"grant_type=client_credentials&client_id="+clientID+"&client_secret="+clientSecret,
	))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("lowercase POST status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "access_token") {
		t.Fatalf("lowercase POST response = %q, want access_token", recorder.Body.String())
	}
}

func TestTokenEndpointRejectsOversizedForm(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader(
		strings.Repeat("x", maxBodyBytes+1),
	))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized form status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestTokenEndpointUsesUniformUnauthorizedResponse(t *testing.T) {
	tests := []struct {
		name string
		form string
	}{
		{name: "missing client credentials", form: "grant_type=client_credentials"},
		{name: "invalid client credentials", form: "grant_type=client_credentials&client_id=invalid&client_secret=invalid"},
		{name: "invalid resource owner credentials", form: "grant_type=password&client_id=" + clientID + "&client_secret=" + clientSecret + "&username=invalid&password=invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader(tt.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			svc, err := New()
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			svc.Handler().ServeHTTP(recorder, req)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			if got, want := strings.TrimSpace(recorder.Body.String()), `{"error":"unauthorized","message":"Invalid or expired credentials."}`; got != want {
				t.Fatalf("body = %q, want %q", got, want)
			}
		})
	}
}

func TestTokenEndpointPartitionsStateAndSupportsFailureResponses(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	h := s.Handler()

	for _, block := range []string{"block-a", "block-b"} {
		reset := httptest.NewRecorder()
		h.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/"+block+"/debug/reset", nil))
		if reset.Code != http.StatusNoContent {
			t.Fatalf("%s reset status = %d", block, reset.Code)
		}
	}

	bad := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader("grant_type=client_credentials&client_id=broken-client&client_secret=ignored"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(bad, req)
	if bad.Code != http.StatusInternalServerError {
		t.Fatalf("broken client status = %d, want %d", bad.Code, http.StatusInternalServerError)
	}

	malformed := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/block-b/oauth2/token", strings.NewReader("grant_type=client_credentials&client_id=malformed-client&client_secret=ignored"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(malformed, req)
	if malformed.Code != http.StatusOK || strings.Contains(malformed.Body.String(), "access_token") {
		t.Fatalf("malformed response = %d %q", malformed.Code, malformed.Body.String())
	}

	stats := httptest.NewRecorder()
	h.ServeHTTP(stats, httptest.NewRequest(http.MethodGet, "/block-a/debug/stats", nil))
	if !strings.Contains(stats.Body.String(), "server_error") {
		t.Fatalf("block-a stats = %q, want server_error", stats.Body.String())
	}
	otherStats := httptest.NewRecorder()
	h.ServeHTTP(otherStats, httptest.NewRequest(http.MethodGet, "/block-b/debug/stats", nil))
	if !strings.Contains(otherStats.Body.String(), "malformed") {
		t.Fatalf("block-b stats = %q, want malformed", otherStats.Body.String())
	}
}

func TestTokenEndpointSupportsTokenExchangeGrant(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	h := s.Handler()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader(
		"grant_type=urn:ietf:params:oauth:grant-type:token-exchange"+
			"&subject_token=caller-token"+
			"&subject_token_type=urn:ietf:params:oauth:token-type:access_token"+
			"&requested_token_type=urn:ietf:params:oauth:token-type:jwt"+
			"&audience=backend-api&resource=https://backend.example.com",
	))
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %q", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var resp struct {
		AccessToken     string `json:"access_token"`
		IssuedTokenType string `json:"issued_token_type"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("access_token missing from response %q", recorder.Body.String())
	}
	if want := "urn:ietf:params:oauth:token-type:jwt"; resp.IssuedTokenType != want {
		t.Errorf("issued_token_type = %q, want %q", resp.IssuedTokenType, want)
	}

	stats := httptest.NewRecorder()
	h.ServeHTTP(stats, httptest.NewRequest(http.MethodGet, "/block-a/debug/stats", nil))
	body := stats.Body.String()
	for _, want := range []string{"backend-api", "backend.example.com", "call...oken"} {
		if !strings.Contains(body, want) {
			t.Errorf("stats body = %q, want it to contain %q", body, want)
		}
	}
	if strings.Contains(body, "caller-token") {
		t.Errorf("stats body leaked the raw subject token: %q", body)
	}
}

func TestTokenEndpointSupportsJWTBearerGrant(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	h := s.Handler()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader(
		"grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=header.payload.signature",
	))
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "access_token") {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "issued_token_type") {
		t.Errorf("jwt-bearer response should not carry issued_token_type (that's token-exchange-only): %q", recorder.Body.String())
	}
}

func TestTokenEndpointRejectsMalformedTokenExchangeAndJWTBearerRequests(t *testing.T) {
	tests := []struct {
		name       string
		form       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "token-exchange missing subject_token",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:token-exchange&subject_token_type=urn:ietf:params:oauth:token-type:access_token",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "token-exchange missing subject_token_type",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:token-exchange&subject_token=caller-token",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "token-exchange unsupported requested_token_type",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:token-exchange&subject_token=caller-token&subject_token_type=urn:ietf:params:oauth:token-type:access_token&requested_token_type=not-a-urn",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "token-exchange sentinel invalid subject_token",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:token-exchange&subject_token=invalid-subject-token&subject_token_type=urn:ietf:params:oauth:token-type:access_token",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "jwt-bearer missing assertion",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "jwt-bearer sentinel invalid assertion",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=invalid-assertion",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "unsupported grant type",
			form:       "grant_type=urn:ietf:params:oauth:grant-type:device_code",
			wantStatus: http.StatusBadRequest,
			wantError:  "unsupported_grant_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader(tt.form))
			req.SetBasicAuth(clientID, clientSecret)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			svc, err := New()
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			svc.Handler().ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %q", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			var got struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Error != tt.wantError {
				t.Errorf("error = %q, want %q", got.Error, tt.wantError)
			}
		})
	}
}

func TestTokenEndpointRetainsOnlyTheNewestHistory(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := &partition{}
	firstClientID := "client-0"
	newestClientID := fmt.Sprintf("client-%d", maxHistory)
	for i := 0; i < maxHistory+1; i++ {
		s.record(p, tokenRequest{ClientID: fmt.Sprintf("client-%d", i)})
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.history) != maxHistory {
		t.Fatalf("history length = %d, want %d", len(p.history), maxHistory)
	}
	if p.history[0].ClientID == firstClientID {
		t.Fatalf("first retained request = %q, want the first request to be discarded", p.history[0].ClientID)
	}
	if got := p.history[len(p.history)-1].ClientID; got != newestClientID {
		t.Fatalf("newest retained request = %q, want %q", got, newestClientID)
	}
}

func TestTokenExchangeAndJWTBearerIssueAVerifiableJWT(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	h := s.Handler()

	issue := func(form string) string {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/block-a/oauth2/token", strings.NewReader(form))
		req.SetBasicAuth(clientID, clientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
		}
		var resp struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp.AccessToken
	}

	verify := func(raw string) jwt.MapClaims {
		t.Helper()
		token, err := jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != "RS256" {
				return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
			}
			if kid, _ := token.Header["kid"].(string); kid != signingKeyID {
				t.Errorf("kid = %q, want %q", kid, signingKeyID)
			}
			return &s.signingKey.PublicKey, nil
		})
		if err != nil {
			t.Fatalf("parsing/verifying issued token %q: %v", raw, err)
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			t.Fatalf("issued token did not parse into valid claims: %q", raw)
		}
		return claims
	}

	t.Run("token-exchange", func(t *testing.T) {
		raw := issue("grant_type=urn:ietf:params:oauth:grant-type:token-exchange" +
			"&subject_token=caller-token&subject_token_type=urn:ietf:params:oauth:token-type:access_token" +
			"&audience=backend-api&scope=read")
		claims := verify(raw)
		if sub, _ := claims["sub"].(string); sub != clientID {
			t.Errorf("sub = %q, want %q", sub, clientID)
		}
		if scope, _ := claims["scope"].(string); scope != "read" {
			t.Errorf("scope = %q, want %q", scope, "read")
		}
		aud, _ := claims["aud"].([]interface{})
		if len(aud) != 1 || aud[0] != "backend-api" {
			t.Errorf("aud = %v, want [backend-api]", aud)
		}
		if exp, _ := claims["exp"].(float64); exp <= float64(time.Now().Unix()) {
			t.Errorf("exp = %v, want a time in the future", claims["exp"])
		}
	})

	t.Run("jwt-bearer", func(t *testing.T) {
		raw := issue("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=header.payload.signature")
		claims := verify(raw)
		if sub, _ := claims["sub"].(string); sub != clientID {
			t.Errorf("sub = %q, want %q", sub, clientID)
		}
		// No audience was supplied, so the default single-element audience applies.
		aud, _ := claims["aud"].([]interface{})
		if len(aud) != 1 || aud[0] != "backend" {
			t.Errorf("aud = %v, want [backend]", aud)
		}
	})

	t.Run("client_credentials keeps the opaque format", func(t *testing.T) {
		raw := issue("grant_type=client_credentials")
		if !strings.HasPrefix(raw, "mock-token-") {
			t.Fatalf("access_token = %q, want the unchanged opaque mock-token- format", raw)
		}
		if _, err := jwt.Parse(raw, func(*jwt.Token) (interface{}, error) { return nil, fmt.Errorf("not a JWT") }); err == nil {
			t.Fatalf("access_token %q unexpectedly parsed as a JWT", raw)
		}
	})
}

func TestJWKSEndpointServesTheSigningKeysPublicHalf(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	h := s.Handler()

	for _, path := range []string{"/block-a/jwks", "/block-a/.well-known/jwks.json"} {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %q", path, recorder.Code, recorder.Body.String())
		}
		var doc struct {
			Keys []jsonWebKey `json:"keys"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &doc); err != nil {
			t.Fatalf("%s: decode response: %v", path, err)
		}
		if len(doc.Keys) != 1 {
			t.Fatalf("%s: keys = %d, want 1", path, len(doc.Keys))
		}
		key := doc.Keys[0]
		if key.Kid != signingKeyID || key.Kty != "RSA" || key.Alg != "RS256" || key.Use != "sig" {
			t.Fatalf("%s: unexpected key metadata %+v", path, key)
		}
		n, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			t.Fatalf("%s: decoding n: %v", path, err)
		}
		e, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			t.Fatalf("%s: decoding e: %v", path, err)
		}
		got := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		want := &s.signingKey.PublicKey
		if got.N.Cmp(want.N) != 0 || got.E != want.E {
			t.Fatalf("%s: public key does not match the service's signing key", path)
		}
	}
}
