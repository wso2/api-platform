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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// enforcerTestSpec mirrors the real spec's shape: a base path on servers[0], and a
// security block on every operation. "GET /organizations" is deliberately
// declared with a read scope and "POST /organizations" with a manage scope so
// method-level separation is covered too.
const enforcerTestSpec = `
openapi: 3.0.3
info:
  title: test
  version: v0.9
servers:
  - url: https://localhost:9243/api/v0.9
paths:
  /organizations:
    get:
      security:
        - OAuth2Security:
            - ap:organization:read
    post:
      security:
        - OAuth2Security:
            - ap:organization:manage
  /gateways/{gatewayId}:
    get:
      security:
        - OAuth2Security:
            - ap:gateway:read
    delete:
      security:
        - OAuth2Security:
            - ap:gateway:manage
  /secrets:
    get:
      security:
        - OAuth2Security:
            - ap:secret:read
            - ap:secret:manage
`

// newTestRegistry loads enforcerTestSpec into a ScopeRegistry.
func newTestRegistry(t *testing.T) *ScopeRegistry {
	t.Helper()
	registry, err := LoadScopeRegistryFromBytes([]byte(enforcerTestSpec))
	if err != nil {
		t.Fatalf("LoadScopeRegistryFromBytes: %v", err)
	}
	return registry
}

// newTestMux registers a route for every operation enforcerTestSpec declares,
// plus three it deliberately does not: DELETE /secrets, whose path is declared
// but whose method is not, and two skip-path routes authenticated by other
// means.
func newTestMux(reached *bool) *http.ServeMux {
	mux := http.NewServeMux()
	hit := func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	}
	mux.HandleFunc("GET /api/v0.9/organizations", hit)
	mux.HandleFunc("POST /api/v0.9/organizations", hit)
	mux.HandleFunc("GET /api/v0.9/gateways/{gatewayId}", hit)
	mux.HandleFunc("DELETE /api/v0.9/gateways/{gatewayId}", hit)
	mux.HandleFunc("GET /api/v0.9/secrets", hit)
	// enforcerTestSpec declares only "get" for /secrets, so this route exists
	// with no scope requirement behind it — the shape a handler gains when a
	// method is added without updating the spec. It must fail closed rather
	// than pass through unauthorized.
	mux.HandleFunc("DELETE /api/v0.9/secrets", hit)
	// Authenticated by a gateway token, not a user JWT — covered by SkipPaths.
	mux.HandleFunc("GET /api/internal/v1/secrets", hit)
	mux.HandleFunc("GET /health", hit)
	return mux
}

var testSkipPaths = []string{"/health", "/api/internal/v1/secrets"}

// serveWithChain wires the enforcer exactly as server.go does: as an outer
// middleware wrapping the mux, so r.Pattern is empty when it runs. This is the
// wiring the original bypass depended on.
func serveWithChain(t *testing.T, cfg ScopeEnforcerConfig, scope, method, path string) (*httptest.ResponseRecorder, bool) {
	t.Helper()

	var reached bool
	mux := newTestMux(&reached)
	if cfg.Routes == nil {
		cfg.Routes = mux
	}
	if cfg.SkipPaths == nil {
		cfg.SkipPaths = testSkipPaths
	}

	enforcer, err := ScopeEnforcer(newTestRegistry(t), cfg)
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(context.WithValue(req.Context(), keyScope, scope))

	rec := httptest.NewRecorder()
	enforcer(mux).ServeHTTP(rec, req)
	return rec, reached
}

func enabledConfig() ScopeEnforcerConfig {
	return ScopeEnforcerConfig{ValidationMode: ValidationModeScope, Enabled: true}
}

// TestScopeEnforcer_EnforcesWhenWrappingTheMux is the regression test for the
// bypass: with the enforcer registered outside the router, r.Pattern is empty,
// and a lookup keyed off it matched nothing — so every scoped route was served
// without a scope check. A low-privileged token must now be rejected.
func TestScopeEnforcer_EnforcesWhenWrappingTheMux(t *testing.T) {
	rec, reached := serveWithChain(t, enabledConfig(), "ap:organization:read", http.MethodPost, "/api/v0.9/organizations")

	if rec.Code != http.StatusForbidden {
		t.Errorf("POST /organizations with only a read scope: status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if reached {
		t.Error("handler was reached; the request must be rejected before routing")
	}
}

func TestScopeEnforcer_DeniesWithoutRequiredScope(t *testing.T) {
	tests := []struct {
		name   string
		scope  string
		method string
		path   string
	}{
		{"no scopes at all", "", http.MethodPost, "/api/v0.9/organizations"},
		{"unrelated resource scope", "ap:gateway:manage", http.MethodPost, "/api/v0.9/organizations"},
		{"read scope on a manage operation", "ap:gateway:read", http.MethodDelete, "/api/v0.9/gateways/gw-1"},
		{"manage scope on another resource", "ap:organization:manage", http.MethodGet, "/api/v0.9/gateways/gw-1"},
		{"neither of the accepted scopes", "ap:gateway:read ap:project:manage", http.MethodGet, "/api/v0.9/secrets"},
		{"wildcard scoped to another resource", "ap:project:*", http.MethodGet, "/api/v0.9/gateways/gw-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, reached := serveWithChain(t, enabledConfig(), tc.scope, tc.method, tc.path)
			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if reached {
				t.Error("handler was reached despite insufficient scopes")
			}
		})
	}
}

func TestScopeEnforcer_AllowsWithRequiredScope(t *testing.T) {
	tests := []struct {
		name   string
		scope  string
		method string
		path   string
	}{
		{"exact scope", "ap:organization:manage", http.MethodPost, "/api/v0.9/organizations"},
		{"exact scope on a path parameter route", "ap:gateway:read", http.MethodGet, "/api/v0.9/gateways/gw-1"},
		{"exact scope, delete", "ap:gateway:manage", http.MethodDelete, "/api/v0.9/gateways/gw-1"},
		{"one of several space-separated scopes", "ap:project:read ap:organization:manage ap:gateway:read", http.MethodPost, "/api/v0.9/organizations"},
		{"first of two accepted scopes", "ap:secret:read", http.MethodGet, "/api/v0.9/secrets"},
		{"second of two accepted scopes", "ap:secret:manage", http.MethodGet, "/api/v0.9/secrets"},
		{"resource wildcard", "ap:gateway:*", http.MethodDelete, "/api/v0.9/gateways/gw-1"},
		{"root wildcard covers a resource action", "ap:*", http.MethodGet, "/api/v0.9/gateways/gw-1"},
		{"method-specific read scope on the read operation", "ap:organization:read", http.MethodGet, "/api/v0.9/organizations"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, reached := serveWithChain(t, enabledConfig(), tc.scope, tc.method, tc.path)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if !reached {
				t.Error("handler was not reached despite sufficient scopes")
			}
		})
	}
}

// TestScopeEnforcer_DeniesRouteWithNoDeclaredScope covers the deny-by-default
// half: a route registered on the mux but carrying no OpenAPI security block
// has nothing to authorize against, so it must fail closed rather than serve.
// DELETE /secrets is registered; the spec declares only GET for that path.
// Holding every scope the spec does declare for /secrets must not help.
func TestScopeEnforcer_DeniesRouteWithNoDeclaredScope(t *testing.T) {
	rec, reached := serveWithChain(t, enabledConfig(), "ap:secret:read ap:secret:manage", http.MethodDelete, "/api/v0.9/secrets")

	if rec.Code != http.StatusForbidden {
		t.Errorf("undeclared route: status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if reached {
		t.Error("handler for an undeclared route was reached")
	}
}

// TestScopeEnforcer_EnforcesWhenPathParamNamesDiffer covers the shape the
// event-gateway plugin ships: the spec documents "{apiId}" while the route
// registers "{webSubApiId}". The parameter name is not part of the route's
// identity, so the scope must still be enforced rather than silently skipped.
func TestScopeEnforcer_EnforcesWhenPathParamNamesDiffer(t *testing.T) {
	const spec = `
openapi: 3.0.3
info:
  title: test
  version: v0.9
paths:
  /api/v0.9/websub-apis/{apiId}:
    delete:
      security:
        - OAuth2Security:
            - ap:websub_api:manage
`
	registry, err := LoadScopeRegistryFromBytes([]byte(spec))
	if err != nil {
		t.Fatalf("LoadScopeRegistryFromBytes: %v", err)
	}

	var reached bool
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v0.9/websub-apis/{webSubApiId}", func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	enforcer, err := ScopeEnforcer(registry, ScopeEnforcerConfig{Enabled: true, Routes: mux})
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	for _, tc := range []struct {
		name  string
		scope string
		want  int
	}{
		{"without the scope", "ap:websub_api:read", http.StatusForbidden},
		{"with the scope", "ap:websub_api:manage", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodDelete, "/api/v0.9/websub-apis/api-1", nil)
			req = req.WithContext(context.WithValue(req.Context(), keyScope, tc.scope))

			rec := httptest.NewRecorder()
			enforcer(mux).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached != (tc.want == http.StatusOK) {
				t.Errorf("reached = %v, want %v", reached, tc.want == http.StatusOK)
			}
		})
	}
}

// TestScopeEnforcer_AllowsSkipPaths keeps the routes that are authenticated by
// other means (gateway token, unauthenticated probes) reachable — deny-by-default
// must not take them down.
func TestScopeEnforcer_AllowsSkipPaths(t *testing.T) {
	for _, path := range []string{"/health", "/api/internal/v1/secrets"} {
		t.Run(path, func(t *testing.T) {
			rec, reached := serveWithChain(t, enabledConfig(), "", http.MethodGet, path)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if !reached {
				t.Error("skip-path handler was not reached")
			}
		})
	}
}

// TestScopeEnforcer_UnroutedRequestFallsThrough asserts a request matching no
// route still gets the router's own 404/405 rather than a 403 — a blanket
// Forbidden would turn every typo into an authorization signal.
func TestScopeEnforcer_UnroutedRequestFallsThrough(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"unknown path", http.MethodGet, "/api/v0.9/does-not-exist", http.StatusNotFound},
		{"method not registered for a known path", http.MethodPatch, "/api/v0.9/organizations", http.StatusMethodNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, reached := serveWithChain(t, enabledConfig(), "", tc.method, tc.path)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached {
				t.Error("a handler was reached for an unrouted request")
			}
		})
	}
}

// TestScopeEnforcer_RoleMode resolves effective scopes from the platform roles
// the IDP claim mapping produced, rather than from the scope claim.
func TestScopeEnforcer_RoleMode(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  int
	}{
		{"role expands to the required scope", []string{"ap:organization:manage"}, http.StatusOK},
		{"role expands only to an unrelated scope", []string{"ap:gateway:read"}, http.StatusForbidden},
		{"no roles", nil, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			mux := newTestMux(&reached)
			enforcer, err := ScopeEnforcer(newTestRegistry(t), ScopeEnforcerConfig{
				ValidationMode: ValidationModeRole,
				Enabled:        true,
				Routes:         mux,
				SkipPaths:      testSkipPaths,
			})
			if err != nil {
				t.Fatalf("ScopeEnforcer: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v0.9/organizations", nil)
			// The scope claim carries a satisfying value that must be ignored in
			// role mode — only the expanded roles count.
			ctx := context.WithValue(req.Context(), keyScope, "ap:organization:manage")
			ctx = context.WithValue(ctx, keyPlatformRoles, tc.roles)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			enforcer(mux).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestScopeEnforcer_DisabledPassesThrough documents the explicit opt-out: with
// scope validation switched off, nothing is enforced (the server logs a warning
// at startup), and the enforcer builds without a route matcher.
func TestScopeEnforcer_DisabledPassesThrough(t *testing.T) {
	var reached bool
	mux := newTestMux(&reached)

	enforcer, err := ScopeEnforcer(newTestRegistry(t), ScopeEnforcerConfig{Enabled: false})
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0.9/organizations", nil)
	rec := httptest.NewRecorder()
	enforcer(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !reached {
		t.Errorf("disabled enforcer: status = %d, reached = %v; want 200 and true", rec.Code, reached)
	}
}

// TestScopeEnforcer_FailsClosedOnUnenforceableConfig asserts the construction
// error that keeps the server from starting with a config that would silently
// enforce nothing (GO-AUTH-011).
func TestScopeEnforcer_FailsClosedOnUnenforceableConfig(t *testing.T) {
	t.Run("no route matcher", func(t *testing.T) {
		if _, err := ScopeEnforcer(newTestRegistry(t), ScopeEnforcerConfig{Enabled: true}); err == nil {
			t.Error("expected an error when enforcement is enabled without a route matcher")
		}
	})

	t.Run("no registry", func(t *testing.T) {
		if _, err := ScopeEnforcer(nil, ScopeEnforcerConfig{Enabled: true, Routes: http.NewServeMux()}); err == nil {
			t.Error("expected an error when enforcement is enabled without a registry")
		}
	})
}

// TestScopeEnforcer_UsesPatternWhenAlreadyMatched covers the case where the
// enforcer is placed inside the router: r.Pattern is already set and must be
// used as-is, without consulting the matcher.
func TestScopeEnforcer_UsesPatternWhenAlreadyMatched(t *testing.T) {
	enforcer, err := ScopeEnforcer(newTestRegistry(t), ScopeEnforcerConfig{
		Enabled: true,
		Routes:  http.NewServeMux(), // Empty — matching must come from r.Pattern.
	})
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	var reached bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range []struct {
		name  string
		scope string
		want  int
	}{
		{"sufficient", "ap:organization:manage", http.StatusOK},
		{"insufficient", "ap:organization:read", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodPost, "/api/v0.9/organizations", nil)
			req.Pattern = "POST /api/v0.9/organizations"
			req = req.WithContext(context.WithValue(req.Context(), keyScope, tc.scope))

			rec := httptest.NewRecorder()
			enforcer(inner).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if reached != (tc.want == http.StatusOK) {
				t.Errorf("reached = %v, want %v", reached, tc.want == http.StatusOK)
			}
		})
	}
}
