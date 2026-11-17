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
	"encoding/json"
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

// The default catalog ships "regular" entries, so a deployment that never mentions
// [[gateway_versions]] still gets a download step. It has no "event" entry: no event
// gateway release is published to link to.
func TestLoad_GatewayVersionsDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	types := map[string]bool{}
	for _, gv := range cfg.GatewayVersions {
		types[gv.Type] = true
	}
	if !types["regular"] {
		t.Errorf("default GatewayVersions has no regular entry")
	}
	if types["event"] {
		t.Errorf("default GatewayVersions has an event entry, but no event release is published")
	}
}

// A catalog in the config replaces the built-in one: nothing is inherited from the
// defaults, so an entry that omits latest_version gets none rather than another release's.
func TestLoad_GatewayVersionsReplaceDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane+`
[[api_control_plane.gateway_versions]]
type = "regular"
version = "1.2"
channel = "LTS"
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []GatewayVersionEntry{{Type: "regular", Version: "1.2", Channel: "LTS"}}
	if len(cfg.GatewayVersions) != len(want) || cfg.GatewayVersions[0] != want[0] {
		t.Errorf("GatewayVersions = %+v, want exactly %+v", cfg.GatewayVersions, want)
	}
}

func TestLoad_GatewayVersionsFromTOML(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane+`
[[api_control_plane.gateway_versions]]
type = "regular"
version = "1.2"
latest_version = "v1.2.0"
dist_version = "1.2.0"
channel = "LTS"
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	var got *GatewayVersionEntry
	for i := range cfg.GatewayVersions {
		if cfg.GatewayVersions[i].Version == "1.2" {
			got = &cfg.GatewayVersions[i]
		}
	}
	if got == nil {
		t.Fatalf("GatewayVersions = %+v, want an entry with version 1.2", cfg.GatewayVersions)
	}
	want := GatewayVersionEntry{Type: "regular", Version: "1.2", LatestVersion: "v1.2.0", DistVersion: "1.2.0", Channel: "LTS"}
	if *got != want {
		t.Errorf("entry = %+v, want %+v", *got, want)
	}
}

func TestLoad_GatewayVersionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		entry   string
		wantErr string
	}{
		{"invalid type", "type = \"bogus\"\nversion = \"1.0\"\nchannel = \"LTS\"", "invalid type"},
		{"invalid channel", "type = \"regular\"\nversion = \"1.0\"\nchannel = \"BETA\"", "invalid channel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, minimalControlPlane+"\n[[api_control_plane.gateway_versions]]\n"+tt.entry+"\n"))
			if err == nil {
				t.Fatal("Load() succeeded, want a gateway_versions validation error")
			}
			if !strings.Contains(err.Error(), "[[gateway_versions]]") || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to mention [[gateway_versions]] and %q", err, tt.wantErr)
			}
		})
	}
}

// Missing fields are checked on validate() directly.
func TestValidate_GatewayVersionsMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		entry   GatewayVersionEntry
		wantErr string
	}{
		{"missing type", GatewayVersionEntry{Version: "1.0", Channel: "LTS"}, "invalid type"},
		{"missing version", GatewayVersionEntry{Type: "regular", Channel: "LTS"}, "missing version"},
		{"missing channel", GatewayVersionEntry{Type: "event", Version: "1.0"}, "invalid channel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.ControlPlane.URL = "https://platform-api:9243"
			cfg.GatewayVersions = []GatewayVersionEntry{tt.entry}
			err := cfg.validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validate() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// "ai" is accepted by validation even though this BFF only serves regular/event.
func TestLoad_GatewayVersionsAcceptsAIType(t *testing.T) {
	if _, err := Load(writeConfig(t, minimalControlPlane+`
[[api_control_plane.gateway_versions]]
type = "ai"
version = "1.0"
channel = "STS"
`)); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

// The catalog reaches the browser as one JSON-encoded runtime value using the
// camelCase keys of the TS GatewayVersionEntry contract; empty optional fields
// are omitted.
func TestLoad_GatewayVersionsRuntimeConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	raw, ok := cfg.RuntimeConfig["gatewayVersions"]
	if !ok {
		t.Fatal("RuntimeConfig has no gatewayVersions key")
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("gatewayVersions is not valid JSON: %v", err)
	}
	if len(decoded) != len(cfg.GatewayVersions) {
		t.Fatalf("decoded %d entries, want %d", len(decoded), len(cfg.GatewayVersions))
	}
	first := decoded[0]
	if first["type"] != "regular" || first["version"] != "2026.09.24" || first["channel"] != "STS" {
		t.Errorf("first entry = %v, want regular/2026.09.24/STS", first)
	}
	for _, k := range []string{"latestVersion", "distVersion"} {
		if _, present := first[k]; present {
			t.Errorf("first entry has empty %q, want it omitted", k)
		}
	}
	if decoded[1]["latestVersion"] != "v1.1.0" {
		t.Errorf("second entry latestVersion = %v, want v1.1.0", decoded[1]["latestVersion"])
	}
}

func TestBuildRuntimeConfig_OmitsGatewayVersionsWhenEmpty(t *testing.T) {
	out := buildRuntimeConfig(&Config{})
	if _, ok := out["gatewayVersions"]; ok {
		t.Error("gatewayVersions present for an empty catalog, want it omitted")
	}
}

// An empty scope key, which is the shipped config.toml's env token with
// APIP_ACP_AUTH_OIDC_SCOPE unset, keeps the built-in default rather than
// requesting no scope; a set one replaces the default verbatim.
func TestLoad_OIDCScopeEmptyKeepsDefault(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.auth.oidc]
scope = ""
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.OIDC.Scopes != defaultOIDCScopes {
		t.Errorf("Scopes = %q, want the built-in default", cfg.Auth.OIDC.Scopes)
	}
	cfg, err = Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.auth.oidc]
scope = "openid offline_access ap:project:read"
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.OIDC.Scopes != "openid offline_access ap:project:read" {
		t.Errorf("Scopes = %q, want the override", cfg.Auth.OIDC.Scopes)
	}
}

// A scope override replaces the default, so one that names only a plugin's
// scopes, or drops offline_access, is reported at startup.
func TestOIDCScopeOverrideWarnings(t *testing.T) {
	if w := oidcScopeOverrideWarnings(defaultOIDCScopes); len(w) != 0 {
		t.Errorf("default list: got warnings %q, want none", w)
	}
	if w := oidcScopeOverrideWarnings(defaultOIDCScopes + " ap:environment:read ap:pipeline:read"); len(w) != 0 {
		t.Errorf("default plus plugin scopes: got warnings %q, want none", w)
	}
	w := oidcScopeOverrideWarnings("openid offline_access ap:environment:read ap:pipeline:read")
	if len(w) != 1 || !strings.Contains(w[0], "replaces the built-in default") {
		t.Errorf("plugin scopes only: got %q, want one missing-default-scopes warning", w)
	}
	w = oidcScopeOverrideWarnings(strings.Replace(defaultOIDCScopes, " offline_access", "", 1))
	if len(w) != 1 || !strings.Contains(w[0], "offline_access") {
		t.Errorf("no offline_access: got %q, want one offline_access warning", w)
	}
}

// The default scope list is the Platform API spec's, so it has to name the
// resources added since the list was first written, and nothing the spec lacks.
func TestDefaultOIDCScopes_FollowTheSpec(t *testing.T) {
	have := " " + defaultOIDCScopes + " "
	for _, s := range []string{"openid", "offline_access", "ap:rest_api:read", "ap:graphql_api:read",
		"ap:llm_proxy:manage", "ap:mcp_proxy:read", "ap:agent_proxy:read", "ap:api_portal:draft:read",
		"ap:meter:read", "ap:subscription_plan:pricing:read", "ap:api_portal:mcp_proxy:manage",
		"ap:service_account:read", "ap:service_account:manage"} {
		if !strings.Contains(have, " "+s+" ") {
			t.Errorf("default scopes miss %q", s)
		}
	}
	for _, s := range []string{"ap:rest_api:import", "ap:organization:subscription:read", "ap:environment:read"} {
		if strings.Contains(have, " "+s+" ") {
			t.Errorf("default scopes carry %q, which the Platform API spec does not declare", s)
		}
	}
}

// The resource indicator is empty unless configured, and trimmed when it is.
func TestLoad_OIDCResource(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.OIDC.Resource != "" {
		t.Errorf("Resource = %q, want empty by default", cfg.Auth.OIDC.Resource)
	}
	cfg, err = Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.auth.oidc]
resource = " urn:wso2:cloud "
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.OIDC.Resource != "urn:wso2:cloud" {
		t.Errorf("Resource = %q, want urn:wso2:cloud", cfg.Auth.OIDC.Resource)
	}
}
