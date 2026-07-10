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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrganizationResolverMiddleware(t *testing.T) {
	// resolve simulates the DB lookup: the IDP org id "idp-123" maps to platform
	// UUID "plat-uuid-1"; the platform UUID resolves to itself (file-based mode).
	resolve := func(orgClaim string) (string, bool) {
		switch orgClaim {
		case "idp-123":
			return "plat-uuid-1", true
		case "plat-uuid-1":
			return "plat-uuid-1", true
		default:
			return "", false
		}
	}

	tests := []struct {
		name       string
		claim      string
		setClaim   bool
		wantOrg    string // organization the handler should see (resolved, or raw claim if unresolved)
		wantHasOrg bool
	}{
		{
			name:       "idp claim resolves to platform uuid",
			claim:      "idp-123",
			setClaim:   true,
			wantOrg:    "plat-uuid-1",
			wantHasOrg: true,
		},
		{
			name:       "file-based claim already the platform uuid",
			claim:      "plat-uuid-1",
			setClaim:   true,
			wantOrg:    "plat-uuid-1",
			wantHasOrg: true,
		},
		{
			name:       "unresolved claim (org not yet created) is left as-is",
			claim:      "brand-new-idp-org",
			setClaim:   true,
			wantOrg:    "brand-new-idp-org",
			wantHasOrg: true,
		},
		{
			name:       "no organization claim passes through untouched",
			setClaim:   false,
			wantOrg:    "",
			wantHasOrg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotOrg string
			var gotHasOrg bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotOrg, gotHasOrg = GetOrganizationFromRequest(r)
			})

			r := httptest.NewRequest(http.MethodGet, "/api/v0.9/rest-apis", nil)
			if tt.setClaim {
				r = WithOrganization(r, tt.claim)
			}

			OrganizationResolverMiddleware(resolve)(next).ServeHTTP(httptest.NewRecorder(), r)

			if gotHasOrg != tt.wantHasOrg {
				t.Errorf("organization present = %v, want %v", gotHasOrg, tt.wantHasOrg)
			}
			if gotOrg != tt.wantOrg {
				t.Errorf("resolved organization = %q, want %q", gotOrg, tt.wantOrg)
			}
		})
	}
}
