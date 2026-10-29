/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// specOperation is one scoped operation: the alternatives its security block accepts.
type specOperation struct {
	method, path string
	accepts      []string
}

// loadScopedOperations reads every operation carrying an OAuth2 security block from an
// OpenAPI spec. Returns nil when the spec is not present (a trimmed checkout).
func loadScopedOperations(t *testing.T, specPath string) []specOperation {
	t.Helper()
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil
	}
	// Navigated loosely rather than through a fixed struct: a path item legitimately
	// carries non-operation keys (path-level "parameters", "$ref") that no operation
	// shape can absorb.
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", specPath, err)
	}
	paths, _ := doc["paths"].(map[string]any)
	var ops []specOperation
	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		for method, rawOp := range item {
			op, ok := rawOp.(map[string]any)
			if !ok {
				continue
			}
			security, ok := op["security"].([]any)
			if !ok {
				continue
			}
			var accepts []string
			for _, rawScheme := range security {
				scheme, ok := rawScheme.(map[string]any)
				if !ok {
					continue
				}
				for _, rawScopes := range scheme {
					scopes, ok := rawScopes.([]any)
					if !ok {
						continue
					}
					for _, s := range scopes {
						if str, ok := s.(string); ok {
							accepts = append(accepts, str)
						}
					}
				}
			}
			if len(accepts) > 0 {
				ops = append(ops, specOperation{strings.ToUpper(method), path, accepts})
			}
		}
	}
	return ops
}

func specPaths() []string {
	root := filepath.Join("..", "..", "..", "..", "..")
	return []string{
		filepath.Join(root, "platform-api", "resources", "openapi.yaml"),
		filepath.Join(root, "platform-api", "plugins", "eventgateway", "openapi.yaml"),
	}
}

// excludedScopeResources are the resource families the AI Workspace does not surface,
// so the default scope set deliberately requests nothing for them. An operation whose
// every accepted alternative names one of these is out of scope for the coverage check
// below rather than a gap in the requested set.
var excludedScopeResources = []string{
	"ap:rest_api:",
	"ap:subscription:",
	"ap:subscription_plan:",
	"ap:websub_api:",
	"ap:webbroker_api:",
	// Publishing a REST API to a portal, unlike the type-agnostic draft/publication
	// scopes above, lives on dedicated rest-api-only endpoints and will never cover an
	// MCP proxy — that gets its own scopes (see ap:api_portal:mcp_proxy: above) once it
	// ships. Consistent with ap:rest_api: itself being excluded.
	"ap:api_portal:rest_api:",
	// GraphQL API publishing is the same per-type flow as REST's, surfaced only in
	// the API control plane console — the AI Workspace has no GraphQL publish page.
	"ap:api_portal:graphql_api:",
	"ap:docs:",
	"ap:thumbnail:",
	// The same resources by their permission-tree names (permission-tree.yaml).
	"ap:api:rest:",
	"ap:api:websub:",
	"ap:api:webbroker:",
	"ap:portal:rest_api:",
	"ap:portal:graphql_api:",
	"ap:content:",
}

// ownershipOverrideScopes widen whose API keys a caller reaches; they are never
// requested on a user's behalf, by either of their names.
var ownershipOverrideScopes = []string{"ap:credential:api_key:all:manage", "ap:api_key:all:manage"}

func isExcludedScope(scope string) bool {
	// The cross-user ownership override is never requested by default
	// (TestDefaultOIDCScopesExcludeOwnershipOverride), so it can't stand in as the
	// remaining alternative that keeps an excluded-family operation in the check.
	if slices.Contains(ownershipOverrideScopes, scope) {
		return true
	}
	return slices.ContainsFunc(excludedScopeResources, func(prefix string) bool {
		return strings.HasPrefix(scope, prefix)
	})
}

// The default scope set must satisfy every scoped operation the Platform API declares,
// excluding the resource families listed in excludedScopeResources. This is what makes
// the trimmed list safe: the granular create/update/delete scopes are omitted only
// because each of their operations also accepts a resource :manage, and that is
// per-endpoint enumeration in the spec, not scope hierarchy — nothing expands
// `ap:x:manage` into `ap:x:y:read` at request time.
func TestDefaultOIDCScopesCoverEveryOperation(t *testing.T) {
	granted := strings.Fields(defaultOIDCScopes)
	checked := 0
	for _, specPath := range specPaths() {
		ops := loadScopedOperations(t, specPath)
		if ops == nil {
			t.Logf("spec not present, skipping: %s", specPath)
			continue
		}
		for _, op := range ops {
			// Only reachable via an excluded family — not a coverage gap.
			if !slices.ContainsFunc(op.accepts, func(s string) bool { return !isExcludedScope(s) }) {
				continue
			}
			checked++
			satisfied := slices.ContainsFunc(op.accepts, func(s string) bool {
				return slices.Contains(granted, s)
			})
			if !satisfied {
				t.Errorf("%s %s accepts %v — none of which the default scope set requests",
					op.method, op.path, op.accepts)
			}
		}
	}
	if checked == 0 {
		t.Skip("no specs available to check against")
	}
	t.Logf("verified %d scoped operations against %d requested scopes", checked, len(granted))
}

// ap:api_key:all:manage is the cross-user ownership override (GO-AUTH-019). Every
// endpoint accepting it also accepts a narrower scope, so it must never be requested
// for every session by default.
func TestDefaultOIDCScopesExcludeOwnershipOverride(t *testing.T) {
	for _, scope := range ownershipOverrideScopes {
		if slices.Contains(strings.Fields(defaultLoginScopes(true)), scope) {
			t.Errorf("defaultOIDCScopes requests the cross-user override scope %s", scope)
		}
	}
}

// offline_access is what makes silent token renewal possible; without it most IDPs
// return no refresh token and the session dies at the access token's expiry.
func TestDefaultOIDCScopesRequestOfflineAccess(t *testing.T) {
	if !slices.Contains(strings.Fields(defaultOIDCScopes), "offline_access") {
		t.Error("defaultOIDCScopes omits offline_access — token refresh would be impossible")
	}
}

// The default set requests two vocabularies, each on its own rule, so that whatever
// a least-privilege user actually holds survives the IDP's intersection of requested
// and entitled:
//
//   - Pre-tree names: every declared one, as before. A user granted only
//     ap:llm_proxy:create would lose it if the request were trimmed to :manage/:read.
//   - Permission-tree names: exactly the permissions the shipped roles in
//     platform-api/resources/role-to-scope-mapping.yaml grant. A parent or root
//     permission (ap:api:manage, ap:manage) admits every operation beneath it, so
//     the fine-grained tree actions are not requested; a deployment whose IDP roles
//     grant one of those overrides [auth.oidc] scope.
//
// The cost is size: encoded, this parameter is several kilobytes, which exceeds
// Microsoft Entra ID's authorize-URL limit outright (AADSTS90015). That is not fixed by
// trimming the shared default — Entra cannot mint ap:* scopes at all, so such a
// deployment must override [auth.oidc] scope with its own resource scope and pair that
// with [auth.authorization] mode = "role". This test pins the trade-off so the size is
// a conscious decision rather than a surprise.
func TestDefaultOIDCScopesRequestGranularScopes(t *testing.T) {
	// While request_legacy_scopes is on (the built-in default), the pre-tree
	// granular names are still requested.
	granted := strings.Fields(defaultLoginScopes(true))
	// Representative pre-tree granular scopes that a :manage/:read-only request would drop.
	for _, scope := range []string{
		"ap:llm_proxy:create",
		"ap:project:delete",
		"ap:llm_proxy:update",
		"ap:llm_proxy:deployment:undeploy",
	} {
		if !slices.Contains(granted, scope) {
			t.Errorf("defaultOIDCScopes omits %s — a user holding only that grant would lose it, "+
				"since an IDP grants the intersection of requested and entitled", scope)
		}
	}

	// Every tree permission a shipped role grants, read from the file so the two
	// cannot drift apart, is in the minimal request (defaultOIDCScopes) — which
	// is what request_legacy_scopes = false sends. The ownership override is
	// never requested by default.
	granted = strings.Fields(defaultOIDCScopes)
	rolesPath := filepath.Join("..", "..", "..", "..", "..", "platform-api", "resources", "role-to-scope-mapping.yaml")
	data, err := os.ReadFile(rolesPath)
	if err != nil {
		t.Logf("role mapping not present, skipping the role-grant check: %s", rolesPath)
	} else {
		var doc struct {
			Roles []struct {
				Name   string   `yaml:"name"`
				Scopes []string `yaml:"scopes"`
			} `yaml:"roles"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parse %s: %v", rolesPath, err)
		}
		for _, role := range doc.Roles {
			for _, scope := range role.Scopes {
				if !strings.HasPrefix(scope, "ap:") || slices.Contains(ownershipOverrideScopes, scope) {
					continue
				}
				if !slices.Contains(granted, scope) {
					t.Errorf("defaultOIDCScopes omits %s, which the shipped role %s grants — an IDP role mirroring it would lose it", scope, role.Name)
				}
			}
		}
	}
	t.Logf("requesting %d scopes, %d bytes encoded (Entra ID deployments must override this)",
		len(granted), len(url.QueryEscape(defaultOIDCScopes)))
}
