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
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// TestScopeEnforcementOnAgentProxyAPIKeyRoutes drives the real ScopeEnforcer
// over the real router and registry for the four public Agent proxy API-key
// operations. Each accepted scope must admit the request on its own — they are
// OR alternatives — and a key read scope must never admit a mutation.
//
// next is a sentinel rather than the mux: the handlers are constructed with nil
// services, so admitting a request into one would panic. The mux is still the
// route matcher, so pattern resolution is the production path.
func TestScopeEnforcementOnAgentProxyAPIKeyRoutes(t *testing.T) {
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

	const (
		collection = "/api/v0.9/agent-proxies/weather-agent/api-keys"
		member     = collection + "/consumer-key"
	)
	umbrellas := []string{"ap:agent_proxy:api_key:manage", "ap:agent_proxy:manage", "ap:api_key:all:manage"}

	for _, op := range []struct {
		method, path string
		fineGrained  string
		denied       []string
	}{
		{http.MethodGet, collection, "ap:agent_proxy:api_key:read",
			[]string{"ap:agent_proxy:read", "ap:api_key:read", "ap:llm_proxy:api_key:read"}},
		{http.MethodPost, collection, "ap:agent_proxy:api_key:create",
			[]string{"ap:agent_proxy:api_key:read", "ap:agent_proxy:create", "ap:agent_proxy:api_key:update", "ap:llm_proxy:api_key:create"}},
		{http.MethodPut, member, "ap:agent_proxy:api_key:update",
			[]string{"ap:agent_proxy:api_key:read", "ap:agent_proxy:update", "ap:agent_proxy:api_key:create", "ap:rest_api:api_key:update"}},
		{http.MethodDelete, member, "ap:agent_proxy:api_key:delete",
			[]string{"ap:agent_proxy:api_key:read", "ap:agent_proxy:delete", "ap:agent_proxy:api_key:update", "ap:llm_proxy:api_key:delete"}},
	} {
		cases := map[string]int{"": http.StatusForbidden, op.fineGrained: http.StatusOK}
		for _, s := range umbrellas {
			cases[s] = http.StatusOK
		}
		for _, s := range op.denied {
			cases[s] = http.StatusForbidden
		}

		for scope, want := range cases {
			t.Run(op.method+" "+op.path+" with "+scopeLabel(scope), func(t *testing.T) {
				admitted := false
				next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					admitted = true
					w.WriteHeader(http.StatusOK)
				})

				req := middleware.WithScope(httptest.NewRequest(op.method, op.path, nil), scope)
				rec := httptest.NewRecorder()
				enforcer(next).ServeHTTP(rec, req)

				if rec.Code != want {
					t.Errorf("status = %d, want %d", rec.Code, want)
				}
				if admitted != (want == http.StatusOK) {
					t.Errorf("admitted = %v, want %v", admitted, want == http.StatusOK)
				}
			})
		}
	}
}

func scopeLabel(scope string) string {
	if scope == "" {
		return "no scope"
	}
	return scope
}
