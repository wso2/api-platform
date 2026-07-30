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
	"strings"
	"testing"
)

func noopHandler(w http.ResponseWriter, r *http.Request) {}

func TestValidateScopeRegistryRoutes_MatchingRoutesPass(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0.9/organizations", noopHandler)
	mux.HandleFunc("POST /api/v0.9/organizations", noopHandler)
	mux.HandleFunc("GET /api/v0.9/gateways/{gatewayId}", noopHandler)
	mux.HandleFunc("DELETE /api/v0.9/gateways/{gatewayId}", noopHandler)
	mux.HandleFunc("GET /api/v0.9/secrets", noopHandler)

	if err := ValidateScopeRegistryRoutes(mux, newTestRegistry(t)); err != nil {
		t.Fatalf("expected the matching route set to validate, got: %v", err)
	}
}

func TestValidateScopeRegistryRoutes_DetectsDrift(t *testing.T) {
	tests := []struct {
		name    string
		routes  []string
		wantIn  string
		wantErr bool
	}{
		{
			// A parameter name is not part of a route's identity — the registry
			// normalizes it away, so this must not be reported as drift.
			name: "path parameter renamed is not drift",
			routes: []string{
				"GET /api/v0.9/organizations", "POST /api/v0.9/organizations",
				"GET /api/v0.9/gateways/{id}", "DELETE /api/v0.9/gateways/{id}",
				"GET /api/v0.9/secrets",
			},
			wantErr: false,
		},
		{
			// Nothing is registered for the declared operation, so nothing gets
			// wrongly denied — the entry is simply dead.
			name: "declared operation with no route at all is not fatal",
			routes: []string{
				"GET /api/v0.9/organizations",
				"GET /api/v0.9/gateways/{gatewayId}", "DELETE /api/v0.9/gateways/{gatewayId}",
				"GET /api/v0.9/secrets",
			},
			wantErr: false,
		},
		{
			// The declared path is served by a catch-all registered one segment
			// up, which carries no scope of its own: real traffic to
			// /api/v0.9/secrets would land there and be denied.
			name: "declared path swallowed by a structurally different route",
			routes: []string{
				"GET /api/v0.9/organizations", "POST /api/v0.9/organizations",
				"GET /api/v0.9/gateways/{gatewayId}", "DELETE /api/v0.9/gateways/{gatewayId}",
				"GET /api/v0.9/{resource}/",
			},
			wantIn:  "GET /api/v0.9/secrets",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			for _, route := range tc.routes {
				mux.HandleFunc(route, noopHandler)
			}

			err := ValidateScopeRegistryRoutes(mux, newTestRegistry(t))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention the drifted operation %q", err, tc.wantIn)
			}
		})
	}
}
