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

// The default scope set must satisfy every scoped operation the Platform API declares.
// This is what makes the trimmed list safe: the granular create/update/delete scopes
// are omitted only because each of their operations also accepts a resource :manage,
// and that is per-endpoint enumeration in the spec, not scope hierarchy — nothing
// expands `ap:x:manage` into `ap:x:y:read` at request time.
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
	if slices.Contains(strings.Fields(defaultOIDCScopes), "ap:api_key:all:manage") {
		t.Error("defaultOIDCScopes requests the cross-user override scope ap:api_key:all:manage")
	}
}

// offline_access is what makes silent token renewal possible; without it most IDPs
// return no refresh token and the session dies at the access token's expiry.
func TestDefaultOIDCScopesRequestOfflineAccess(t *testing.T) {
	if !slices.Contains(strings.Fields(defaultOIDCScopes), "offline_access") {
		t.Error("defaultOIDCScopes omits offline_access — token refresh would be impossible")
	}
}

// The default set requests every declared scope on purpose, so that whatever subset a
// least-privilege user actually holds survives the IDP's intersection of requested and
// entitled. A user granted only ap:rest_api:create would lose it if the request were
// trimmed to :manage/:read.
//
// The cost is size: encoded, this parameter is several kilobytes, which exceeds
// Microsoft Entra ID's authorize-URL limit outright (AADSTS90015). That is not fixed by
// trimming the shared default — Entra cannot mint ap:* scopes at all, so such a
// deployment must override [auth.oidc] scope with its own resource scope and pair that
// with [auth.authorization] mode = "role". This test pins the trade-off so the size is
// a conscious decision rather than a surprise.
func TestDefaultOIDCScopesRequestGranularScopes(t *testing.T) {
	granted := strings.Fields(defaultOIDCScopes)
	// Representative granular scopes that a :manage/:read-only request would drop.
	for _, scope := range []string{
		"ap:rest_api:create",
		"ap:project:delete",
		"ap:llm_proxy:update",
		"ap:rest_api:deployment:undeploy",
	} {
		if !slices.Contains(granted, scope) {
			t.Errorf("defaultOIDCScopes omits %s — a user holding only that grant would lose it, "+
				"since an IDP grants the intersection of requested and entitled", scope)
		}
	}
	t.Logf("requesting %d scopes, %d bytes encoded (Entra ID deployments must override this)",
		len(granted), len(url.QueryEscape(defaultOIDCScopes)))
}
