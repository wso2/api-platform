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
	"sort"
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

	if err := middleware.ValidateRoleScopeMap(roleScopes, loadMergedRegistry(t), loadShippedLegacyScopeMap(t)); err != nil {
		t.Fatal(err)
	}
}

// TestShippedRoleScopeMapGrantsTreePermissionsOnly pins the shipped roles to
// the permission-tree vocabulary: a pre-tree name still works through the
// legacy mapping, but the file that documents each persona must not rely on
// it, or the mapping could never be retired.
func TestShippedRoleScopeMapGrantsTreePermissionsOnly(t *testing.T) {
	roleScopes, err := middleware.LoadRoleScopeMap(realRoleScopeMapPath)
	if err != nil {
		t.Fatalf("LoadRoleScopeMap(%q): %v", realRoleScopeMapPath, err)
	}
	legacy := loadShippedLegacyScopeMap(t)
	for role, scopes := range roleScopes {
		for _, s := range scopes {
			if _, isLegacy := legacy[s]; isLegacy {
				t.Errorf("role %q grants pre-tree scope %q; grant %v instead", role, s, legacy[s])
			}
		}
	}
}

// TestAgentProxyScopesAreGrantedAndDeclared asserts the Agent Proxy permission
// family is wired at both ends: declared by an operation in the spec (which is
// what makes it grantable at all), and actually granted to at least one role —
// directly or through an ancestor (ap:api) — without which every Agent Proxy
// endpoint is reachable by nobody.
func TestAgentProxyScopesAreGrantedAndDeclared(t *testing.T) {
	const agentTreePrefix = "ap:api:agent:"

	declared := loadMergedRegistry(t).AllScopes()
	for _, scope := range []string{
		agentTreePrefix + "read",
		agentTreePrefix + "manage",
		agentTreePrefix + "build:read",
		agentTreePrefix + "build:manage",
		agentTreePrefix + "deployment:read",
		agentTreePrefix + "deployment:manage",
		agentTreePrefix + "api_key:read",
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
			if strings.HasPrefix(s, agentTreePrefix) || s == "ap:api:read" || s == "ap:api:manage" {
				granted = true
			}
		}
	}
	if !granted {
		t.Errorf("no role in %q grants any %s* scope or an ancestor of it", realRoleScopeMapPath, agentTreePrefix)
	}
}

// TestAgentProxyOperationsAcceptScopeAlternatives pins the OR semantics the
// Agent Proxy operations are specified with: each operation accepts its own
// node's permission and the same action on every ancestor (ap:api), flattened
// by the registry into one accepted-scope list. A read permission must never
// end up accepted on a mutating operation, and the API-key node — which
// defines no :manage of its own — admits writes only through its ancestors and
// the ownership override.
func TestAgentProxyOperationsAcceptScopeAlternatives(t *testing.T) {
	registry := loadMergedRegistry(t)
	const base = "/api/v0.9/agent-proxies"

	for _, tc := range []struct {
		method, path string
		want         []string
		forbidden    []string
	}{
		{"POST", base, []string{"ap:api:agent:manage", "ap:api:manage"},
			[]string{"ap:api:agent:read", "ap:api:read"}},
		{"GET", base, []string{"ap:api:agent:read", "ap:api:agent:manage", "ap:api:read", "ap:api:manage"}, nil},
		{"PUT", base + "/{agentProxyId}", []string{"ap:api:agent:manage", "ap:api:manage"},
			[]string{"ap:api:agent:read", "ap:api:read"}},
		{"DELETE", base + "/{agentProxyId}", []string{"ap:api:agent:manage", "ap:api:manage"},
			[]string{"ap:api:agent:read", "ap:api:read"}},
		{"POST", base + "/fetch-agent-card", []string{"ap:api:agent:read", "ap:api:agent:manage", "ap:api:read", "ap:api:manage"},
			nil},
		{"GET", base + "/{agentProxyId}/api-keys",
			[]string{"ap:api:agent:api_key:read", "ap:api:agent:read", "ap:api:agent:manage", "ap:api:read", "ap:api:manage", "ap:credential:api_key:all:manage"},
			[]string{"ap:credential:api_key:read", "ap:api:llm:api_key:read"}},
		{"POST", base + "/{agentProxyId}/api-keys",
			[]string{"ap:api:agent:manage", "ap:api:manage", "ap:credential:api_key:all:manage"},
			[]string{"ap:api:agent:api_key:read", "ap:api:agent:read", "ap:api:read", "ap:credential:api_key:read"}},
		{"PUT", base + "/{agentProxyId}/api-keys/{apiKeyId}",
			[]string{"ap:api:agent:manage", "ap:api:manage", "ap:credential:api_key:all:manage"},
			[]string{"ap:api:agent:api_key:read", "ap:api:agent:read", "ap:api:read", "ap:credential:api_key:read"}},
		{"DELETE", base + "/{agentProxyId}/api-keys/{apiKeyId}",
			[]string{"ap:api:agent:manage", "ap:api:manage", "ap:credential:api_key:all:manage"},
			[]string{"ap:api:agent:api_key:read", "ap:api:agent:read", "ap:api:read", "ap:credential:api_key:read"}},
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

// shippedSpecRegistry loads only the spec a default build ships. The
// event-gateway plugin's grants are commented out in the shipped mapping, so its
// operations are deliberately outside the persona checks below.
func shippedSpecRegistry(t *testing.T) *middleware.ScopeRegistry {
	t.Helper()
	registry, err := middleware.LoadScopeRegistry(realSpecPath)
	if err != nil {
		t.Fatalf("LoadScopeRegistry(%q): %v", realSpecPath, err)
	}
	return registry
}

// shippedRoleScopeSets loads the shipped mapping as role -> set of granted scopes.
func shippedRoleScopeSets(t *testing.T) map[string]map[string]struct{} {
	t.Helper()
	roleScopes, err := middleware.LoadRoleScopeMap(realRoleScopeMapPath)
	if err != nil {
		t.Fatalf("LoadRoleScopeMap(%q): %v", realRoleScopeMapPath, err)
	}
	sets := make(map[string]map[string]struct{}, len(roleScopes))
	for role, scopes := range roleScopes {
		set := make(map[string]struct{}, len(scopes))
		for _, s := range scopes {
			set[s] = struct{}{}
		}
		sets[role] = set
	}
	return sets
}

// roleReaches applies the same OR-evaluation ScopeEnforcer does: a role reaches
// an operation when it holds any one of the operation's accepted scopes.
func roleReaches(granted map[string]struct{}, accepted []string) bool {
	for _, s := range accepted {
		if _, ok := granted[s]; ok {
			return true
		}
	}
	return false
}

// sortedOperations returns the registry's operations in a stable order so a
// failure lists the same operations in the same order on every run.
func sortedOperations(registry *middleware.ScopeRegistry) []middleware.Operation {
	ops := registry.Operations()
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	return ops
}

// isReadOperation reports whether an operation is side-effect free. The two
// POSTs are reads the spec models as POSTs — they carry an upstream URL to probe
// in the request body and mutate nothing — which is why the shipped read-only
// role grants them.
func isReadOperation(op middleware.Operation) bool {
	switch op.Method {
	case http.MethodGet, http.MethodHead:
		return true
	case http.MethodPost:
		return op.Path == "/api/v0.9/mcp-proxies/fetch-server-info" ||
			op.Path == "/api/v0.9/agent-proxies/fetch-agent-card"
	}
	return false
}

// TestShippedAdminReachesEveryOperation pins the ap_admin persona — "every
// resource and operation". ap_admin is the role every pack's file-mode admin is
// given, so an operation none of its grants reach would ship usable by nobody
// out of the box.
func TestShippedAdminReachesEveryOperation(t *testing.T) {
	registry := shippedSpecRegistry(t)
	admin := shippedRoleScopeSets(t)["ap_admin"]
	for _, op := range sortedOperations(registry) {
		accepted, _ := registry.Lookup(op.Method, op.Path)
		if !roleReaches(admin, accepted) {
			t.Errorf("ap_admin cannot reach %s %s (accepts %v)", op.Method, op.Path, accepted)
		}
	}
}

// TestShippedOperatorReadsEverything pins the "reads everything else" half of
// the ap_operator persona: every read operation in the spec must be reachable.
// Writes are not checked here — which of them an operator runs is a product
// decision recorded in the mapping itself.
func TestShippedOperatorReadsEverything(t *testing.T) {
	registry := shippedSpecRegistry(t)
	operator := shippedRoleScopeSets(t)["ap_operator"]
	for _, op := range sortedOperations(registry) {
		if !isReadOperation(op) {
			continue
		}
		accepted, _ := registry.Lookup(op.Method, op.Path)
		if !roleReaches(operator, accepted) {
			t.Errorf("ap_operator cannot read %s %s (accepts %v)", op.Method, op.Path, accepted)
		}
	}
}

// TestShippedViewerIsReadOnlyAndComplete pins the ap_viewer persona at both
// ends: every read operation is reachable, and no mutating operation is. A new
// read endpoint the auditor cannot see, or a write it can suddenly perform, both
// fail here rather than surfacing in a deployment.
func TestShippedViewerIsReadOnlyAndComplete(t *testing.T) {
	registry := shippedSpecRegistry(t)
	viewer := shippedRoleScopeSets(t)["ap_viewer"]
	for _, op := range sortedOperations(registry) {
		accepted, _ := registry.Lookup(op.Method, op.Path)
		reaches := roleReaches(viewer, accepted)
		switch {
		case isReadOperation(op) && !reaches:
			t.Errorf("ap_viewer cannot read %s %s (accepts %v)", op.Method, op.Path, accepted)
		case !isReadOperation(op) && reaches:
			t.Errorf("ap_viewer can mutate via %s %s (accepts %v)", op.Method, op.Path, accepted)
		}
	}
}
