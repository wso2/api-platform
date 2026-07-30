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

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// pluginRoutes is every route the event-gateway plugin registers, with a
// concrete value substituted for each path parameter.
//
// Two defects previously made every one of these unenforceable, and neither was
// visible at runtime — both presented as "no scope requirement found", exactly
// like the route-matching bug this suite exists for:
//
//   - The plugin's OpenAPI spec failed to load at all. A path item was decoded
//     as map[string]openAPIOperation, and a path-level "parameters:" block is a
//     YAML sequence, which failed the whole document — discarding all 34 of its
//     scope declarations. The spec has 19 such blocks.
//   - The spec documents "{apiId}" while the handlers register "{webSubApiId}"
//     and "{webBrokerApiId}". Parameter names were part of the registry key, so
//     every lookup missed even once the spec did load.
var pluginRoutes = []struct{ method, path string }{
	{http.MethodPost, "/api/v0.9/websub-apis"},
	{http.MethodGet, "/api/v0.9/websub-apis"},
	{http.MethodGet, "/api/v0.9/websub-apis/a1"},
	{http.MethodPut, "/api/v0.9/websub-apis/a1"},
	{http.MethodDelete, "/api/v0.9/websub-apis/a1"},
	{http.MethodPost, "/api/v0.9/websub-apis/a1/deployments"},
	{http.MethodGet, "/api/v0.9/websub-apis/a1/deployments"},
	{http.MethodGet, "/api/v0.9/websub-apis/a1/deployments/d1"},
	{http.MethodDelete, "/api/v0.9/websub-apis/a1/deployments/d1"},
	{http.MethodPost, "/api/v0.9/websub-apis/a1/deployments/d1/undeploy"},
	{http.MethodPost, "/api/v0.9/websub-apis/a1/deployments/d1/restore"},
	{http.MethodPost, "/api/v0.9/websub-apis/a1/api-keys"},
	{http.MethodPut, "/api/v0.9/websub-apis/a1/api-keys/k1"},
	{http.MethodDelete, "/api/v0.9/websub-apis/a1/api-keys/k1"},
	{http.MethodPost, "/api/v1/websub-apis/a1/secrets"},
	{http.MethodGet, "/api/v1/websub-apis/a1/secrets"},
	{http.MethodDelete, "/api/v1/websub-apis/a1/secrets/s1"},
	{http.MethodPost, "/api/v1/websub-apis/a1/secrets/s1/regenerate"},
	{http.MethodPost, "/api/v0.9/webbroker-apis"},
	{http.MethodGet, "/api/v0.9/webbroker-apis"},
	{http.MethodGet, "/api/v0.9/webbroker-apis/b1"},
	{http.MethodPut, "/api/v0.9/webbroker-apis/b1"},
	{http.MethodDelete, "/api/v0.9/webbroker-apis/b1"},
	{http.MethodPost, "/api/v0.9/webbroker-apis/b1/deployments"},
	{http.MethodGet, "/api/v0.9/webbroker-apis/b1/deployments"},
	{http.MethodGet, "/api/v0.9/webbroker-apis/b1/deployments/d1"},
	{http.MethodDelete, "/api/v0.9/webbroker-apis/b1/deployments/d1"},
	{http.MethodPost, "/api/v0.9/webbroker-apis/b1/deployments/d1/undeploy"},
	{http.MethodPost, "/api/v0.9/webbroker-apis/b1/deployments/d1/restore"},
	{http.MethodPost, "/api/v0.9/webbroker-apis/b1/api-keys"},
	{http.MethodPut, "/api/v0.9/webbroker-apis/b1/api-keys/k1"},
	{http.MethodDelete, "/api/v0.9/webbroker-apis/b1/api-keys/k1"},
}

// TestPluginSpecLoadsWithItsScopes asserts the plugin's embedded spec parses
// into a non-trivial set of scope declarations. It previously produced an error
// and therefore zero — and because initPlugins treats that as fatal, it also
// aborted startup on the experimental build.
func TestPluginSpecLoadsWithItsScopes(t *testing.T) {
	registry := loadMergedRegistry(t)
	core, err := middleware.LoadScopeRegistry(realSpecPath)
	if err != nil {
		t.Fatalf("LoadScopeRegistry(%q): %v", realSpecPath, err)
	}

	if contributed := registry.Len() - core.Len(); contributed < len(pluginRoutes) {
		t.Errorf("plugin contributed %d scope declarations, want at least %d (one per registered route)",
			contributed, len(pluginRoutes))
	}
}

// TestEveryPluginRouteResolvesToDeclaredScopes sends a request at each real
// plugin route and asserts the registry finds a scope requirement for the
// pattern the router actually matched — the lookup ScopeEnforcer performs. With
// deny-by-default, a miss here is a 403 on a working endpoint.
func TestEveryPluginRouteResolvesToDeclaredScopes(t *testing.T) {
	registry := loadMergedRegistry(t)

	mux := http.NewServeMux()
	registerAllRoutes(mux)

	for _, route := range pluginRoutes {
		req, err := http.NewRequest(route.method, route.path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}

		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("%s %s is not registered", route.method, route.path)
			continue
		}

		_, matchedPath, _ := strings.Cut(pattern, " ")
		scopes, found := registry.Lookup(route.method, matchedPath)
		if !found || len(scopes) == 0 {
			t.Errorf("%s %s matched %q, which resolves to no declared scope — it would be denied",
				route.method, route.path, pattern)
		}
	}
}

// TestScopeEnforcementOnRealPluginRoutes drives the real enforcer over the real
// router and the real merged registry, proving the property end to end on a
// route whose spec and handler disagree on the path-parameter name.
//
// next is a sentinel rather than the mux: the handlers are constructed with nil
// services, so admitting a request into one would panic. The mux is still the
// route matcher, so pattern resolution is the production path.
func TestScopeEnforcementOnRealPluginRoutes(t *testing.T) {
	mux := http.NewServeMux()
	registerAllRoutes(mux)

	enforcer, err := middleware.ScopeEnforcer(loadMergedRegistry(t), middleware.ScopeEnforcerConfig{
		ValidationMode: middleware.ValidationModeScope,
		Enabled:        true,
		Routes:         mux,
	})
	if err != nil {
		t.Fatalf("ScopeEnforcer: %v", err)
	}

	var admitted bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admitted = true
		w.WriteHeader(http.StatusOK)
	})

	// Spec: DELETE /api/v0.9/websub-apis/{apiId} requires
	// ap:websub_api:delete or ap:websub_api:manage.
	// Route: DELETE /api/v0.9/websub-apis/{webSubApiId}.
	tests := []struct {
		name  string
		scope string
		want  int
	}{
		{"no scopes", "", http.StatusForbidden},
		{"read scope on a delete operation", "ap:websub_api:read", http.StatusForbidden},
		{"delete scope on another resource", "ap:webbroker_api:delete", http.StatusForbidden},
		{"declared delete scope", "ap:websub_api:delete", http.StatusOK},
		{"declared manage scope", "ap:websub_api:manage", http.StatusOK},
		{"resource wildcard", "ap:websub_api:*", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			admitted = false
			req := middleware.WithScope(
				httptest.NewRequest(http.MethodDelete, "/api/v0.9/websub-apis/a1", nil), tc.scope)

			rec := httptest.NewRecorder()
			enforcer(next).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if admitted != (tc.want == http.StatusOK) {
				t.Errorf("admitted = %v, want %v", admitted, tc.want == http.StatusOK)
			}
		})
	}
}
