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

// reserved_permissions_test.go — unit tests for the spec's x-reserved-permissions:
// permission-tree permissions that are declared and grantable ahead of the
// operation that will accept them, and that never admit a request on their own.

package middleware

import (
	"strings"
	"testing"
)

func reservedTestRegistry(t *testing.T) *ScopeRegistry {
	t.Helper()
	registry, err := LoadScopeRegistryFromBytes([]byte(`
openapi: 3.0.0
x-reserved-permissions:
  - ap:portal:mcp_proxy:manage
  - ap:portal:mcp_proxy:publish
paths:
  /api-portals:
    get:
      security:
        - oauth2:
            - ap:portal:read
            - ap:portal:manage
`))
	if err != nil {
		t.Fatalf("building the scope registry: %v", err)
	}
	return registry
}

func TestScopeRegistry_ReservedPermissions(t *testing.T) {
	registry := reservedTestRegistry(t)
	if !registry.IsReserved("ap:portal:mcp_proxy:manage") {
		t.Error("ap:portal:mcp_proxy:manage is not reserved, want reserved")
	}
	if registry.IsReserved("ap:portal:read") {
		t.Error("ap:portal:read is reserved, want an ordinary declared scope")
	}
	// Reserved permissions are not operation scopes: nothing accepts them, and
	// a legacy name may never map onto one (ValidateLegacyScopeMap uses AllScopes).
	if _, ok := registry.AllScopes()["ap:portal:mcp_proxy:manage"]; ok {
		t.Error("AllScopes includes a reserved permission")
	}

	// Merging a plugin registry keeps both reserved sets.
	plugin, err := LoadScopeRegistryFromBytes([]byte(`
openapi: 3.0.0
x-reserved-permissions:
  - ap:api:websub:future:manage
paths: {}
`))
	if err != nil {
		t.Fatalf("plugin registry: %v", err)
	}
	registry.Merge(plugin)
	if !registry.IsReserved("ap:api:websub:future:manage") || !registry.IsReserved("ap:portal:mcp_proxy:publish") {
		t.Error("Merge dropped a reserved permission")
	}
}

func TestValidateRoleScopeMap_AcceptsReservedPermissions(t *testing.T) {
	registry := reservedTestRegistry(t)
	ok := map[string][]string{"ap_operator": {"ap:portal:read", "ap:portal:mcp_proxy:manage"}}
	if err := ValidateRoleScopeMap(ok, registry, nil); err != nil {
		t.Errorf("ValidateRoleScopeMap rejected a reserved permission: %v", err)
	}
	bad := map[string][]string{"ap_operator": {"ap:portal:mcp_proxy:delete"}}
	if err := ValidateRoleScopeMap(bad, registry, nil); err == nil || !strings.Contains(err.Error(), "unknown scope") {
		t.Errorf("ValidateRoleScopeMap(%v) = %v, want an unknown-scope error", bad, err)
	}
}
