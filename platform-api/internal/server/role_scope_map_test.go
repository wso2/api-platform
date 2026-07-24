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

// roleModeConfig returns a config in IDP role-validation mode pointing at path.
func roleModeConfig(path string) *config.Server {
	cfg := &config.Server{}
	cfg.Auth.Mode = config.AuthModeIDP
	cfg.Auth.IDP.ValidationMode = "role"
	cfg.Auth.IDP.RoleMappings = path
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

// Outside IDP role mode the mapping is not loaded at all, so a roles.yaml
// referencing an unknown scope must not fail startup.
func TestLoadRoleScopeMap_SkippedOutsideRoleMode(t *testing.T) {
	path := writeRolesFile(t, "widget-admin", "ap:not_a_real_scope")

	cfg := roleModeConfig(path)
	cfg.Auth.IDP.ValidationMode = "scope"

	m, err := loadRoleScopeMap(cfg, emptyRegistry(t), testLogger())
	if err != nil {
		t.Fatalf("loadRoleScopeMap: unexpected error: %v", err)
	}
	if m != nil {
		t.Fatalf("expected no mapping outside role mode, got %v", m)
	}
}
