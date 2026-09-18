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
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

const realRoleScopeMapPath = "../../resources/role-to-scope-mapping.yaml"

// TestShippedRoleScopeMapResolvesAgainstTheShippedSpec runs the startup check on
// the files actually shipped in resources/. An "ap:" scope named in the roles
// file but declared by no operation in the OpenAPI spec aborts startup, so a
// typo — or a scope added to a role ahead of the operation that accepts it —
// takes the server down rather than surfacing later as a 403. Catching it here
// turns that into a build-time failure.
func TestShippedRoleScopeMapResolvesAgainstTheShippedSpec(t *testing.T) {
	roleScopes, err := middleware.LoadRoleScopeMap(realRoleScopeMapPath)
	if err != nil {
		t.Fatalf("LoadRoleScopeMap(%q): %v", realRoleScopeMapPath, err)
	}
	if len(roleScopes) == 0 {
		t.Fatalf("role-to-scope map loaded from %q is empty", realRoleScopeMapPath)
	}

	if err := middleware.ValidateRoleScopeMap(roleScopes, loadMergedRegistry(t)); err != nil {
		t.Fatal(err)
	}
}

// TestAgentProxyScopesAreGrantedAndDeclared asserts the Agent Proxy scope family
// is wired at both ends: declared by an operation in the spec (which is what
// makes it grantable at all), and actually granted to at least one role (without
// which every Agent Proxy endpoint is reachable by nobody).
func TestAgentProxyScopesAreGrantedAndDeclared(t *testing.T) {
	const agentScopePrefix = "ap:agent_proxy:"

	declared := loadMergedRegistry(t).AllScopes()
	for _, scope := range []string{
		agentScopePrefix + "create",
		agentScopePrefix + "read",
		agentScopePrefix + "update",
		agentScopePrefix + "delete",
		agentScopePrefix + "manage",
		agentScopePrefix + "deployment:create",
		agentScopePrefix + "deployment:read",
		agentScopePrefix + "deployment:delete",
		agentScopePrefix + "deployment:undeploy",
		agentScopePrefix + "deployment:restore",
		agentScopePrefix + "deployment:manage",
		agentScopePrefix + "api_key:create",
		agentScopePrefix + "api_key:read",
		agentScopePrefix + "api_key:update",
		agentScopePrefix + "api_key:delete",
		agentScopePrefix + "api_key:manage",
	} {
		if _, ok := declared[scope]; !ok {
			t.Errorf("scope %q is declared by no operation in the OpenAPI spec", scope)
		}
	}

	roleScopes, err := middleware.LoadRoleScopeMap(realRoleScopeMapPath)
	if err != nil {
		t.Fatalf("LoadRoleScopeMap(%q): %v", realRoleScopeMapPath, err)
	}
	granted := false
	for _, scopes := range roleScopes {
		for _, s := range scopes {
			if strings.HasPrefix(s, agentScopePrefix) {
				granted = true
			}
		}
	}
	if !granted {
		t.Errorf("no role in %q grants any %s* scope", realRoleScopeMapPath, agentScopePrefix)
	}
}

// TestAgentProxyOperationsAcceptScopeAlternatives pins the OR semantics the
// Agent Proxy operations are specified with: each fine-grained scope and the
// owning :manage umbrella are separate Security Requirement Objects, and the
// registry flattens them into one accepted-scope list. A read scope must never
// end up accepted on a mutating operation.
func TestAgentProxyOperationsAcceptScopeAlternatives(t *testing.T) {
	registry := loadMergedRegistry(t)
	const base = "/api/v0.9/agent-proxies"

	for _, tc := range []struct {
		method, path string
		want         []string
		forbidden    []string
	}{
		{"POST", base, []string{"ap:agent_proxy:create", "ap:agent_proxy:manage"},
			[]string{"ap:agent_proxy:read"}},
		{"GET", base, []string{"ap:agent_proxy:read", "ap:agent_proxy:manage"}, nil},
		{"PUT", base + "/{agentProxyId}", []string{"ap:agent_proxy:update", "ap:agent_proxy:manage"},
			[]string{"ap:agent_proxy:read"}},
		{"DELETE", base + "/{agentProxyId}", []string{"ap:agent_proxy:delete", "ap:agent_proxy:manage"},
			[]string{"ap:agent_proxy:read"}},
		{"POST", base + "/fetch-agent-card", []string{"ap:agent_proxy:read", "ap:agent_proxy:manage"},
			[]string{"ap:agent_proxy:create"}},
		{"POST", base + "/{agentProxyId}/api-keys",
			[]string{"ap:agent_proxy:api_key:create", "ap:agent_proxy:api_key:manage", "ap:agent_proxy:manage", "ap:api_key:all:manage"},
			[]string{"ap:agent_proxy:api_key:read"}},
		{"DELETE", base + "/{agentProxyId}/api-keys/{apiKeyId}",
			[]string{"ap:agent_proxy:api_key:delete", "ap:agent_proxy:api_key:manage", "ap:agent_proxy:manage", "ap:api_key:all:manage"},
			[]string{"ap:agent_proxy:api_key:read"}},
	} {
		scopes, ok := registry.Lookup(tc.method, tc.path)
		if !ok {
			t.Errorf("%s %s declares no scope requirement", tc.method, tc.path)
			continue
		}
		accepted := make(map[string]struct{}, len(scopes))
		for _, s := range scopes {
			accepted[s] = struct{}{}
		}
		for _, s := range tc.want {
			if _, found := accepted[s]; !found {
				t.Errorf("%s %s does not accept %q; accepts %v", tc.method, tc.path, s, scopes)
			}
		}
		for _, s := range tc.forbidden {
			if _, found := accepted[s]; found {
				t.Errorf("%s %s wrongly accepts %q", tc.method, tc.path, s)
			}
		}
	}
}
