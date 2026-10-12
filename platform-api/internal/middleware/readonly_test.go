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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed.

package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

const (
	readOnlyTestOrgA = "11111111-1111-1111-1111-111111111111"
	readOnlyTestOrgB = "22222222-2222-2222-2222-222222222222"
)

// readOnlyTestSkipPaths mirrors the shape of config.Auth.SkipPaths: the routes
// authenticated by a gateway token or by the login endpoint itself, which
// carry no organization in the context.
var readOnlyTestSkipPaths = []string{"/api/internal/v1", "/api/portal/v0.9/auth/login"}

// newReadOnlyTestMux registers routes under the exact patterns the guard's exempt
// list names (so a rename there fails here), a few ordinary write routes, and a
// gateway-token write route that carries no organization in the context.
func newReadOnlyTestMux(reached *bool) *http.ServeMux {
	mux := http.NewServeMux()
	hit := func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	}
	base := constants.APIBasePath
	mux.HandleFunc("GET "+base+"/projects", hit)
	mux.HandleFunc("POST "+base+"/projects", hit)
	mux.HandleFunc("PUT "+base+"/projects/{projectId}", hit)
	mux.HandleFunc("PATCH "+base+"/projects/{projectId}", hit)
	mux.HandleFunc("DELETE "+base+"/projects/{projectId}", hit)
	mux.HandleFunc("POST "+base+"/rest-apis/validate-openapi", hit)
	mux.HandleFunc("POST "+base+"/mcp-proxies/fetch-server-info", hit)
	mux.HandleFunc("POST "+base+"/agent-proxies/fetch-agent-card", hit)
	mux.HandleFunc("POST /api/internal/v1/deployments/fetch-batch", hit)
	mux.HandleFunc("POST /api/internal/v1/artifacts/exists", hit)
	mux.HandleFunc("POST /api/internal/v1/gateways/{gatewayId}/manifest", hit)
	return mux
}

// serveReadOnly wires the guard exactly as server.go does — as an outer middleware
// wrapping the mux, so r.Pattern is empty when it runs — and serves one request.
func serveReadOnly(t *testing.T, ro *config.ReadOnly, setOrg bool, org, method, path string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	var reached bool
	mux := newReadOnlyTestMux(&reached)
	guard, err := ReadOnlyGuard(ReadOnlyGuardConfig{
		ReadOnly:  ro,
		Routes:    mux,
		SkipPaths: readOnlyTestSkipPaths,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("ReadOnlyGuard: %v", err)
	}
	req := httptest.NewRequest(method, path, nil)
	if setOrg {
		req = WithOrganization(req, org)
	}
	rec := httptest.NewRecorder()
	guard(mux).ServeHTTP(rec, req)
	return rec, reached
}

func TestReadOnlyGuard(t *testing.T) {
	base := constants.APIBasePath
	frozenExceptB := &config.ReadOnly{Enabled: true, WritableOrganizations: []string{readOnlyTestOrgB}}
	allFrozen := &config.ReadOnly{Enabled: true}
	disabled := &config.ReadOnly{Enabled: false, WritableOrganizations: []string{readOnlyTestOrgB}}

	tests := []struct {
		name        string
		ro          *config.ReadOnly
		setOrg      bool
		org         string
		method      string
		path        string
		want        int
		wantReached bool
	}{
		{"disabled: write by unlisted org passes", disabled, true, readOnlyTestOrgA, http.MethodPost, base + "/projects", http.StatusOK, true},
		{"nil config: write passes", nil, true, readOnlyTestOrgA, http.MethodPost, base + "/projects", http.StatusOK, true},
		{"frozen: GET passes", frozenExceptB, true, readOnlyTestOrgA, http.MethodGet, base + "/projects", http.StatusOK, true},
		{"frozen: HEAD passes", frozenExceptB, true, readOnlyTestOrgA, http.MethodHead, base + "/projects", http.StatusOK, true},
		{"frozen: POST rejected", frozenExceptB, true, readOnlyTestOrgA, http.MethodPost, base + "/projects", http.StatusServiceUnavailable, false},
		{"frozen: PUT rejected", frozenExceptB, true, readOnlyTestOrgA, http.MethodPut, base + "/projects/p1", http.StatusServiceUnavailable, false},
		{"frozen: PATCH rejected", frozenExceptB, true, readOnlyTestOrgA, http.MethodPatch, base + "/projects/p1", http.StatusServiceUnavailable, false},
		{"frozen: DELETE rejected", frozenExceptB, true, readOnlyTestOrgA, http.MethodDelete, base + "/projects/p1", http.StatusServiceUnavailable, false},
		{"frozen: uppercase org claim still matches the list", frozenExceptB, true, strings.ToUpper(readOnlyTestOrgB), http.MethodPost, base + "/projects", http.StatusOK, true},
		{"writable org: POST passes", frozenExceptB, true, readOnlyTestOrgB, http.MethodPost, base + "/projects", http.StatusOK, true},
		{"frozen: exempt validate-openapi passes", frozenExceptB, true, readOnlyTestOrgA, http.MethodPost, base + "/rest-apis/validate-openapi", http.StatusOK, true},
		{"frozen: exempt fetch-server-info passes", frozenExceptB, true, readOnlyTestOrgA, http.MethodPost, base + "/mcp-proxies/fetch-server-info", http.StatusOK, true},
		{"frozen: exempt fetch-agent-card passes", frozenExceptB, true, readOnlyTestOrgA, http.MethodPost, base + "/agent-proxies/fetch-agent-card", http.StatusOK, true},
		{"frozen: exempt gateway sync read passes", frozenExceptB, true, readOnlyTestOrgA, http.MethodPost, "/api/internal/v1/deployments/fetch-batch", http.StatusOK, true},
		{"frozen: gateway-token write without org (skip path) falls through to the handler", frozenExceptB, false, "", http.MethodPost, "/api/internal/v1/gateways/gw/manifest", http.StatusOK, true},
		{"frozen: protected write without org is rejected (fail closed)", frozenExceptB, false, "", http.MethodPost, base + "/projects", http.StatusServiceUnavailable, false},
		{"frozen: protected write with empty org claim is rejected (fail closed)", frozenExceptB, true, "", http.MethodPost, base + "/projects", http.StatusServiceUnavailable, false},
		{"frozen: protected GET without org still passes", frozenExceptB, false, "", http.MethodGet, base + "/projects", http.StatusOK, true},
		{"frozen: unknown path stays 404", frozenExceptB, true, readOnlyTestOrgA, http.MethodPost, "/nope", http.StatusNotFound, false},
		{"frozen: unregistered method stays 405", frozenExceptB, true, readOnlyTestOrgA, http.MethodPatch, base + "/projects", http.StatusMethodNotAllowed, false},
		{"all frozen: POST by any org rejected", allFrozen, true, readOnlyTestOrgB, http.MethodPost, base + "/projects", http.StatusServiceUnavailable, false},
		{"all frozen: exempt sync read without org passes", allFrozen, false, "", http.MethodPost, "/api/internal/v1/artifacts/exists", http.StatusOK, true},
		{"all frozen: GET passes", allFrozen, true, readOnlyTestOrgB, http.MethodGet, base + "/projects", http.StatusOK, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, reached := serveReadOnly(t, tc.ro, tc.setOrg, tc.org, tc.method, tc.path)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: got status %d, want %d (body: %s)", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
			}
			if reached != tc.wantReached {
				t.Fatalf("%s %s: handler reached = %v, want %v", tc.method, tc.path, reached, tc.wantReached)
			}
			if tc.want != http.StatusServiceUnavailable {
				return
			}
			var body apperror.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode 503 body %q: %v", rec.Body.String(), err)
			}
			if body.Code != apperror.CodeCommonOrganizationReadOnly {
				t.Errorf("body.code = %q, want %q", body.Code, apperror.CodeCommonOrganizationReadOnly)
			}
			if body.Status != "error" || body.Message == "" {
				t.Errorf("unexpected 503 envelope: %+v", body)
			}
		})
	}
}

// Enabled without a route matcher cannot recognise the exempt routes, and
// enabled without skip paths would reject every gateway-token write before its
// handler — refuse both at construction (GO-AUTH-011) rather than fail at runtime.
func TestReadOnlyGuard_EnabledRequiresRoutesAndSkipPaths(t *testing.T) {
	enabled := &config.ReadOnly{Enabled: true}
	mux := http.NewServeMux()
	if _, err := ReadOnlyGuard(ReadOnlyGuardConfig{ReadOnly: enabled, SkipPaths: readOnlyTestSkipPaths}); err == nil {
		t.Fatal("expected an error when read-only mode is enabled without a route matcher")
	}
	if _, err := ReadOnlyGuard(ReadOnlyGuardConfig{ReadOnly: enabled, Routes: mux}); err == nil {
		t.Fatal("expected an error when read-only mode is enabled without auth skip paths")
	}
	if _, err := ReadOnlyGuard(ReadOnlyGuardConfig{ReadOnly: enabled, Routes: mux, SkipPaths: readOnlyTestSkipPaths}); err != nil {
		t.Fatalf("complete config must construct, got: %v", err)
	}
	if _, err := ReadOnlyGuard(ReadOnlyGuardConfig{ReadOnly: &config.ReadOnly{Enabled: false}}); err != nil {
		t.Fatalf("disabled mode must not need a route matcher or skip paths, got: %v", err)
	}
	if _, err := ReadOnlyGuard(ReadOnlyGuardConfig{}); err != nil {
		t.Fatalf("nil config must not need a route matcher or skip paths, got: %v", err)
	}
}

func TestValidateReadOnlyExemptRoutes(t *testing.T) {
	var reached bool
	if err := ValidateReadOnlyExemptRoutes(newReadOnlyTestMux(&reached)); err != nil {
		t.Fatalf("every exempt route is registered, got: %v", err)
	}
	if err := ValidateReadOnlyExemptRoutes(nil); err != nil {
		t.Fatalf("nil router must be a no-op, got: %v", err)
	}

	// A mux missing one exempt route (the pattern changed underneath the list)
	// must be reported by name.
	mux := http.NewServeMux()
	hit := func(w http.ResponseWriter, r *http.Request) {}
	mux.HandleFunc("POST "+constants.APIBasePath+"/rest-apis/validate-openapi", hit)
	mux.HandleFunc("POST "+constants.APIBasePath+"/mcp-proxies/fetch-server-info", hit)
	mux.HandleFunc("POST /api/internal/v1/deployments/fetch-batch", hit)
	mux.HandleFunc("POST /api/internal/v1/artifacts/{artifactId}/exists", hit) // renamed
	err := ValidateReadOnlyExemptRoutes(mux)
	if err == nil {
		t.Fatal("expected an error for a missing exempt route")
	}
	if !strings.Contains(err.Error(), "POST /api/internal/v1/artifacts/exists") {
		t.Errorf("error should name the missing route, got: %v", err)
	}
	if strings.Contains(err.Error(), "fetch-batch") {
		t.Errorf("error should not report routes that are registered, got: %v", err)
	}
}
