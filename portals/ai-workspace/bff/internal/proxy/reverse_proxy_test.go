/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestReverseProxy_InjectsBearerStripsCookieAndPrefix verifies the three security
// invariants of the proxy director: the prefix is stripped, the browser cookie is
// removed, and the session bearer token is injected.
func TestReverseProxy_InjectsBearerStripsCookieAndPrefix(t *testing.T) {
	var gotPath, gotAuth, gotCookie string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer backend.Close()

	target, _ := url.Parse(backend.URL)
	rp := ReverseProxy(target, "/api/proxy", backend.Client().Transport)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy/api/v0.9/projects", nil)
	req.Header.Set("Cookie", "_bff_session=secret-session-id")
	req = WithToken(req, "injected-bearer-token")

	rec := httptest.NewRecorder()
	rp.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotPath != "/api/v0.9/projects" {
		t.Errorf("upstream path = %q, want /api/v0.9/projects (prefix not stripped)", gotPath)
	}
	if gotAuth != "Bearer injected-bearer-token" {
		t.Errorf("upstream Authorization = %q, want injected bearer", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("upstream Cookie = %q, want empty (browser cookie must not leak)", gotCookie)
	}
}

func TestReverseProxy_NoTokenNoAuthHeader(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer backend.Close()

	target, _ := url.Parse(backend.URL)
	rp := ReverseProxy(target, "/api/proxy", backend.Client().Transport)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy/health", nil)
	req.Header.Set("Authorization", "Bearer should-be-removed")
	rec := httptest.NewRecorder()
	rp.ServeHTTP(rec, req)

	if gotAuth != "" {
		t.Errorf("upstream Authorization = %q, want empty when no session token", gotAuth)
	}
}
