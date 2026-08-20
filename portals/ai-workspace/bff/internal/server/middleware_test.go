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

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-workspace-bff/internal/config"
)

func csrfTestServer() *Server {
	return &Server{cfg: &config.Config{}}
}

func TestRequireCSRF(t *testing.T) {
	s := csrfTestServer()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := s.requireCSRF(ok)

	cases := []struct {
		name       string
		method     string
		header     bool
		wantStatus int
	}{
		{"GET without header allowed", http.MethodGet, false, http.StatusOK},
		{"HEAD without header allowed", http.MethodHead, false, http.StatusOK},
		{"POST without header rejected", http.MethodPost, false, http.StatusForbidden},
		{"POST with header allowed", http.MethodPost, true, http.StatusOK},
		{"DELETE without header rejected", http.MethodDelete, false, http.StatusForbidden},
		{"PUT with header allowed", http.MethodPut, true, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/proxy/x", nil)
			if tc.header {
				req.Header.Set("X-Requested-By", "ai-workspace")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// Return targets must stay inside the app's own prefix: the home fallback carries it,
// and a target belonging to another app on the same host (or a lookalike prefix like
// "/ai-workspace-admin") is not a local return target here.
func TestSanitizeReturn(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	cases := map[string]string{
		"/ai-workspace/projects":     "/ai-workspace/projects",
		"/ai-workspace":              "/ai-workspace",
		"/ai-workspace/":             "/ai-workspace/",
		"/ai-workspace/ok\r\ninject": "/ai-workspace/okinject",
		"":                           "/ai-workspace/",
		"/projects":                  "/ai-workspace/",
		"/api-portal/apis":           "/ai-workspace/",
		"/ai-workspace-admin/users":  "/ai-workspace/",
		"//evil.com":                 "/ai-workspace/",
		// Browsers normalize a backslash to a slash in a URL, so "/\\evil.com" would be
		// read as protocol-relative and leave the origin. Confining the target to the
		// app's own prefix rules that out structurally.
		"/\\evil.com":      "/ai-workspace/",
		"/\\/evil.com":     "/ai-workspace/",
		"https://evil.com": "/ai-workspace/",
	}
	for in, want := range cases {
		if got := s.sanitizeReturn(in); got != want {
			t.Errorf("sanitizeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}
