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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a config.toml to a temp dir and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

const minimalControlPlane = `
[api_control_plane.control_plane]
url = "https://platform-api:9243"
`

// Scope mode is the default, so an operator who never mentions [auth.authorization]
// keeps today's behaviour.
func TestLoad_AuthorizationModeDefaultsToScope(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.Authorization.Mode != AuthzModeScope {
		t.Errorf("Authorization.Mode = %q, want %q", cfg.Auth.Authorization.Mode, AuthzModeScope)
	}
}

func TestLoad_AuthorizationRoleMode(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.auth.authorization]
mode = "Role"
role_to_scope_mapping = "/etc/api-control-plane/role-to-scope-mapping.yaml"
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Case-folded, like [auth] mode.
	if cfg.Auth.Authorization.Mode != AuthzModeRole {
		t.Errorf("Authorization.Mode = %q, want %q", cfg.Auth.Authorization.Mode, AuthzModeRole)
	}
	if cfg.Auth.Authorization.RoleToScopeMapping == "" {
		t.Error("RoleToScopeMapping is empty, want the configured path")
	}
	// The grant table is a server-side concern; the SPA gates on the scopes
	// /api/session reports, never on the table itself.
	for k, v := range cfg.RuntimeConfig {
		if strings.Contains(v, "role-to-scope-mapping") {
			t.Errorf("runtime config key %q leaks the grant table path to the browser", k)
		}
	}
}

// Role mode with no grant table can only expand to zero scopes, which would present a
// UI in which nothing is permitted. Refuse to start instead.
func TestLoad_RoleModeWithoutMapping_Errors(t *testing.T) {
	_, err := Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.auth.authorization]
mode = "role"
`))
	if err == nil {
		t.Fatal("Load() succeeded, want an error when role mode has no role_to_scope_mapping")
	}
	if !strings.Contains(err.Error(), "role_to_scope_mapping is required") {
		t.Errorf("error = %v, want it to name role_to_scope_mapping", err)
	}
}

// A typo'd mode must not silently degrade to reading the scope claim.
func TestLoad_InvalidAuthorizationMode_Errors(t *testing.T) {
	_, err := Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.auth.authorization]
mode = "roles"
`))
	if err == nil {
		t.Fatal("Load() succeeded, want an error for an unknown authorization mode")
	}
	if !strings.Contains(err.Error(), "invalid [auth.authorization] mode") {
		t.Errorf("error = %v, want it to name [auth.authorization] mode", err)
	}
}
