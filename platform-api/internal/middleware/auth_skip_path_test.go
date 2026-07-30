/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The authentication middleware and ScopeEnforcer bypass the same skip list, so
// they must agree on what it covers. A raw strings.HasPrefix match exempted any
// path merely starting with a skip entry, and matched before ServeMux normalizes
// the path — both reachable without credentials (GO-AUTH-004).
func TestLocalJWTAuthMiddleware_SkipPathMatchesOnSegmentBoundary(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantSkip bool
	}{
		{"skip path itself", "/health", true},
		{"nested under skip path", "/health/live", true},
		{"nested internal route", "/api/internal/v1/secrets/abc", true},
		{"superstring of a skip path", "/health-probe-fake", false},
		{"superstring of an internal skip path", "/api/internal/v1/secrets-admin", false},
		{"traversal out of a skip path", "/health/../api/v0.9/organizations", false},
		{"unrelated protected route", "/api/v0.9/organizations", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			handler := LocalJWTAuthMiddleware(AuthConfig{
				SkipPaths: []string{"/health", "/api/internal/v1/secrets"},
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
			}))

			// No Authorization header: the handler is reached only when the
			// request was exempted, so reaching it is the bypass signal.
			req := httptest.NewRequest(http.MethodGet, "http://example.com"+tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if reached != tc.wantSkip {
				t.Errorf("auth bypassed = %v, want %v (status %d)", reached, tc.wantSkip, rec.Code)
			}
			if !tc.wantSkip && rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d for an enforced path", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}
