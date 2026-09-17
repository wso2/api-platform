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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTokenEndpointSupportsBasicAndPostAuthentication(t *testing.T) {
	s := New()
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
	s := New()
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
	s := New()
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

			New().Handler().ServeHTTP(recorder, req)

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
	s := New()
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

func TestTokenEndpointRetainsOnlyTheNewestHistory(t *testing.T) {
	s := New()
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
