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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// writeRolesFile writes a roles.yaml mapping one role to the given scopes and
// returns its path.
func writeRolesFile(t *testing.T, role string, scopes ...string) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "roles:\n  - name: %s\n    scopes:\n", role)
	for _, s := range scopes {
		fmt.Fprintf(&b, "      - %s\n", s)
	}
	path := filepath.Join(t.TempDir(), "roles.yaml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("writing roles.yaml: %v", err)
	}
	return path
}

// roleModeConfig returns a config in role-authorization mode pointing at path.
// The authentication mode is deliberately left at its zero value: role-based
// authorization is independent of how tokens are authenticated.
func roleModeConfig(path string) *config.Server {
	cfg := &config.Server{}
	cfg.Auth.Authorization.Mode = config.AuthzModeRole
	cfg.Auth.Authorization.RoleMappings = path
	return cfg
}

// A wrapper must be able to map an IDP role to a scope its own plugin declares.
// Plugin scopes only enter the registry when initPlugins merges each plugin
// spec, so loadRoleScopeMap has to run after that merge — this asserts the
// order the server depends on, and the sibling test below asserts that the
// reverse order is what actually breaks.
func TestLoadRoleScopeMap_AcceptsPluginScopeAfterMerge(t *testing.T) {
	reg := emptyRegistry(t)
	if _, err := run(t, reg, &fakePlugin{name: "widgets", spec: specWithScopes}); err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}

	path := writeRolesFile(t, "widget-admin", "ap:widget_read")
	m, err := loadRoleScopeMap(roleModeConfig(path), reg, testLogger())
	if err != nil {
		t.Fatalf("loadRoleScopeMap: unexpected error for a plugin-declared scope: %v", err)
	}
	if got := m["widget-admin"]; len(got) != 1 || got[0] != "ap:widget_read" {
		t.Fatalf("unexpected role mapping: %v", m)
	}
}

// The pre-merge registry knows nothing of plugin scopes, so validating against
// it rejects the same roles.yaml. This is the failure the ordering above avoids;
// if someone moves loadRoleScopeMap back before initPlugins, the test above
// starts failing with exactly this error.
func TestLoadRoleScopeMap_RejectsPluginScopeBeforeMerge(t *testing.T) {
	reg := emptyRegistry(t)

	path := writeRolesFile(t, "widget-admin", "ap:widget_read")
	_, err := loadRoleScopeMap(roleModeConfig(path), reg, testLogger())
	if err == nil {
		t.Fatal("expected an error for a scope missing from the registry, got nil")
	}
	if !strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The mapping is loaded whenever it is configured, including in scope
// authorization mode — file-mode users name a role from this same file to
// inherit its scopes, so the login endpoint needs it there too. A bad roles.yaml
// therefore still fails startup in scope mode.
func TestLoadRoleScopeMap_LoadedInScopeMode(t *testing.T) {
	reg := emptyRegistry(t)
	if _, err := run(t, reg, &fakePlugin{name: "widgets", spec: specWithScopes}); err != nil {
		t.Fatalf("initPlugins: unexpected error: %v", err)
	}

	cfg := roleModeConfig(writeRolesFile(t, "widget-admin", "ap:widget_read"))
	cfg.Auth.Authorization.Mode = config.AuthzModeScope

	m, err := loadRoleScopeMap(cfg, reg, testLogger())
	if err != nil {
		t.Fatalf("loadRoleScopeMap: unexpected error: %v", err)
	}
	if got := m["widget-admin"]; len(got) != 1 || got[0] != "ap:widget_read" {
		t.Fatalf("expected the mapping to load in scope mode, got %v", m)
	}
}

// With no mapping file configured nothing consumes the mapping, so none is
// loaded — config validation is what guarantees the path is set wherever a role
// is actually named.
func TestLoadRoleScopeMap_SkippedWhenUnconfigured(t *testing.T) {
	m, err := loadRoleScopeMap(&config.Server{}, emptyRegistry(t), testLogger())
	if err != nil {
		t.Fatalf("loadRoleScopeMap: unexpected error: %v", err)
	}
	if m != nil {
		t.Fatalf("expected no mapping when role_mappings is unset, got %v", m)
	}
}

// A file-mode user naming a role absent from the mapping would get a token whose
// scope claim silently lacks everything the role was meant to grant — a login
// that succeeds and then 403s on every request. Catch the typo at startup.
func TestValidateFileUserRoles(t *testing.T) {
	roleScopeMap := map[string][]string{"ap_admin": {"ap:organization:manage"}}

	cfg := &config.Server{}
	cfg.Auth.Mode = config.AuthModeFile
	cfg.Auth.Authorization.RoleMappings = "/etc/platform-api/roles.yaml"
	cfg.Auth.File.Users = config.FileBasedUsers{{Username: "admin", Role: "ap_admin"}}
	if err := validateFileUserRoles(cfg, roleScopeMap); err != nil {
		t.Fatalf("unexpected error for a defined role: %v", err)
	}

	cfg.Auth.File.Users = config.FileBasedUsers{{Username: "admin", Role: "ap_admn"}}
	err := validateFileUserRoles(cfg, roleScopeMap)
	if err == nil {
		t.Fatal("expected an error for a role missing from the mapping, got nil")
	}
	if !strings.Contains(err.Error(), "ap_admn") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only file mode has users to check — another mode's config carries none.
	cfg.Auth.Mode = config.AuthModeIDP
	cfg.Auth.File.Users = config.FileBasedUsers{{Username: "admin", Role: "ap_admn"}}
	if err := validateFileUserRoles(cfg, roleScopeMap); err != nil {
		t.Fatalf("unexpected error outside file mode: %v", err)
	}
}

// The shipped sample mapping must load and validate against the shipped OpenAPI
// spec in a default build. It is mounted (not baked into the image) and named by
// the shipped config.toml, so a scope that this spec doesn't declare — or that
// only exists on a plugin build — would fail startup for every pack user.
func TestShippedSampleRolesValidateAgainstShippedSpec(t *testing.T) {
	reg, err := middleware.LoadScopeRegistry("../../resources/openapi.yaml")
	if err != nil {
		t.Fatalf("loading the shipped OpenAPI spec: %v", err)
	}

	m, err := middleware.LoadRoleScopeMap("../../resources/roles.yaml")
	if err != nil {
		t.Fatalf("loading the shipped roles.yaml: %v", err)
	}
	if err := middleware.ValidateRoleScopeMap(m, reg); err != nil {
		t.Fatalf("shipped roles.yaml is not valid against the shipped spec: %v", err)
	}

	// The documented role set. ap_admin in particular is what the shipped
	// config.toml grants its admin user, so a rename here breaks every pack.
	for _, role := range []string{"ap_admin", "ap_operator", "ap_publisher", "ap_subscriber", "ap_viewer"} {
		if _, ok := m[role]; !ok {
			t.Fatalf("shipped roles.yaml does not declare %q", role)
		}
	}

	// Roles span the platform: a role names Developer Portal scopes too, which
	// this server mints but does not enforce. If the namespace-scoped validation
	// above ever regresses to registry-only, this is what would start failing.
	found := false
	for _, s := range m["ap_admin"] {
		if strings.HasPrefix(s, "dp:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ap_admin to grant Developer Portal scopes: %v", m["ap_admin"])
	}
}
