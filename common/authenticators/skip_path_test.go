/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
package authenticators

import "testing"

// A skip-path match bypasses authentication AND sets the authz-skip flag, so an
// over-broad match exempts a route from both. The matcher must therefore cover
// only the paths an operator listed and what is nested under them (GO-AUTH-004).
func TestHasPathPrefix(t *testing.T) {
	skips := []string{"/health", "/api/internal/v1/secrets"}

	tests := []struct {
		path string
		want bool
	}{
		// The skip path itself and anything nested under it.
		{"/health", true},
		{"/health/", true},
		{"/health/live", true},
		{"/api/internal/v1/secrets", true},
		{"/api/internal/v1/secrets/handle/value", true},

		// A different route that merely starts with the same characters.
		{"/healthz", false},
		{"/health-probe-fake", false},
		{"/api/internal/v1/secrets-admin", false},

		// Traversal out of a skip path must not stay exempt.
		{"/health/../api/v0.9/organizations", false},

		// Unnormalized spellings of a skip path resolve to it, so they stay exempt.
		{"//health", true},
		{"/./health", true},

		{"/api/v0.9/organizations", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := HasPathPrefix(tc.path, skips); got != tc.want {
				t.Errorf("HasPathPrefix(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// A "/" entry would exempt every request on the server. It is rejected at
// startup by callers, and ignored here as well so a list that slips through
// cannot silently disable authentication wholesale.
func TestHasPathPrefix_RootPrefixIsIgnored(t *testing.T) {
	if HasPathPrefix("/api/v0.9/organizations", []string{"/"}) {
		t.Error("a root skip prefix must not exempt every request")
	}
	if !HasPathPrefix("/health", []string{"/", "/health"}) {
		t.Error("a valid entry alongside a root prefix must still match")
	}
}
