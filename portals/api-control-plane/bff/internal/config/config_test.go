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
	"slices"
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

// {{ env }} renders only into strings, so a comma-separated env var must decode
// into a list of entries rather than one entry containing the commas.
func TestLoad_AllowHostsFromCommaSeparatedEnv(t *testing.T) {
	body := minimalControlPlane + `
[api_control_plane.gateway_relay.egress]
allow_hosts = '{{ env "TEST_ACP_ALLOW_HOSTS" "" }}'
`
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{"several", " https://a.example.com:8443 , *.gw.example.com,,", []string{"https://a.example.com:8443", "*.gw.example.com"}},
		{"single", "https://a.example.com", []string{"https://a.example.com"}},
		{"unset", "", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_ACP_ALLOW_HOSTS", tc.env)
			cfg, err := Load(writeConfig(t, body))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := cfg.GatewayRelay.Egress.AllowHosts; !slices.Equal(got, tc.want) {
				t.Errorf("AllowHosts = %q, want %q", got, tc.want)
			}
		})
	}
}

// A TOML array keeps working alongside the env-token form.
func TestLoad_AllowHostsFromTOMLArray(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane+`
[api_control_plane.gateway_relay.egress]
allow_hosts = ["https://a.example.com", "https://b.example.com:9443"]
`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"https://a.example.com", "https://b.example.com:9443"}
	if got := cfg.GatewayRelay.Egress.AllowHosts; !slices.Equal(got, want) {
		t.Errorf("AllowHosts = %q, want %q", got, want)
	}
}

// The default catalog ships "regular" and "event" entries, so a deployment that
// never mentions [[gateway_versions]] still gets a download step.
func TestLoad_GatewayVersionsDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalControlPlane))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	types := map[string]bool{}
	for _, gv := range cfg.GatewayVersions {
		types[gv.Type] = true
	}
	for _, want := range []string{"regular", "event"} {
		if !types[want] {
			t.Errorf("default GatewayVersions has no %q entry", want)
		}
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

// Missing fields are checked on validate() directly: through Load, an override
// entry is decoded onto the default entry at the same index and inherits any
// field it omits, so they can't be reproduced from TOML alone.
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
	if first["type"] != "regular" || first["version"] != "2026.05.13" || first["channel"] != "STS" {
		t.Errorf("first entry = %v, want regular/2026.05.13/STS", first)
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
