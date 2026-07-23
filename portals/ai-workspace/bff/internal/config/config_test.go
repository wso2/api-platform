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
	"time"
)

// writeConfig writes a config.toml into a temp dir and returns the path to pass Load(cfgPath).
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// A literal config.toml value is used as written.
func TestLoad_ConfigFileValue(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.logging]
level = "warn"

[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ControlPlane.URL != "https://platform-api:9243" {
		t.Errorf("ControlPlane.URL = %q, want the config.toml value", cfg.ControlPlane.URL)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.Logging.Level, "warn")
	}
}

// The environment reaches a key only through that key's {{ env }} token: the token
// supplies the variable's value, and its default applies when the variable is unset.
func TestLoad_EnvTokenSuppliesValueAndDefault(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.logging]
level  = '{{ env "APIP_AIW_LOGGING_LEVEL" "info" }}'
format = '{{ env "APIP_AIW_LOGGING_FORMAT" "text" }}'

[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)
	t.Setenv("APIP_AIW_LOGGING_LEVEL", "debug") // named by the token
	// APIP_AIW_LOGGING_FORMAT is left unset, so the token's default stands.

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("LogLevel = %q, want %q (the token's variable is set)", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("LogFormat = %q, want the token default %q", cfg.Logging.Format, "text")
	}
}

// There is no implicit environment overlay: a key written as a literal keeps that
// literal even when the conventionally-named APIP_AIW_ variable is set. Only a token
// pulls a value in from the environment.
func TestLoad_EnvVarWithoutTokenIsIgnored(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.logging]
level = "warn"

[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)
	t.Setenv("APIP_AIW_LOGGING_LEVEL", "debug")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("LogLevel = %q, want the config.toml literal %q — an env var must not override a key with no token",
			cfg.Logging.Level, "warn")
	}
}

// An {{ env }} token names its variable explicitly, so a key may be pointed at any
// variable — the APIP_AIW_ prefix is a convention, not a requirement.
func TestLoad_InterpolatesEnvToken(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.oidc]
authority     = "https://idp.example.com"
client_id     = "client-id"
client_secret = '{{ env "CUSTOM_SECRET_VAR" }}'
redirect_url  = "https://localhost:5380/api/auth/callback"
`)
	t.Setenv("CUSTOM_SECRET_VAR", "s3cr3t")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.OIDC.ClientSecret != "s3cr3t" {
		t.Errorf("OIDC.ClientSecret = %q, want the value resolved from the env token", cfg.Auth.OIDC.ClientSecret)
	}
}

// A {{ file }} token reads a mounted secret file inside an allowed directory.
func TestLoad_InterpolatesFileToken(t *testing.T) {
	secretDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(secretDir, "oidc_client_secret"), []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("APIP_CONFIG_FILE_SOURCE_ALLOWLIST", secretDir)

	cfgPath := writeConfig(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.oidc]
authority     = "https://idp.example.com"
client_id     = "client-id"
client_secret = '{{ file "`+filepath.Join(secretDir, "oidc_client_secret")+`" }}'
redirect_url  = "https://localhost:5380/api/auth/callback"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// The trailing newline every secret file ends with must be trimmed.
	if cfg.Auth.OIDC.ClientSecret != "from-file" {
		t.Errorf("OIDC.ClientSecret = %q, want %q", cfg.Auth.OIDC.ClientSecret, "from-file")
	}
}

// Interpolation fails closed: a file outside the allowlist must abort startup
// rather than resolve to an empty credential.
func TestLoad_FileTokenOutsideAllowlist_Errors(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("nope"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("APIP_CONFIG_FILE_SOURCE_ALLOWLIST", t.TempDir()) // a different directory

	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.oidc]
client_secret = '{{ file "`+outside+`" }}'
`)

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("Load() succeeded, want an error for a file outside the allowlist")
	}
}

// An {{ env }} token whose variable is unset must abort startup, not silently
// yield an empty secret.
func TestLoad_MissingEnvToken_Errors(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.oidc]
client_secret = '{{ env "CUSTOM_SECRET_VAR" }}'
`)
	t.Setenv("CUSTOM_SECRET_VAR", "")

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() succeeded, want an error for an unset env token")
	}
	if !strings.Contains(err.Error(), "CUSTOM_SECRET_VAR") {
		t.Errorf("error = %v, want it to name the missing variable", err)
	}
}

// The upstream URL is mandatory — the BFF has nothing to proxy to without it.
func TestLoad_MissingControlPlaneURL_Errors(t *testing.T) {
	cfgPath := writeConfig(t, "[ai_workspace.server]\ndomain = \"localhost:5380\"")

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() succeeded, want an error when [control_plane] url is unset")
	}
	if !strings.Contains(err.Error(), "[control_plane] url") {
		t.Errorf("error = %v, want it to name [control_plane] url", err)
	}
}

// The runtime config served to the browser is an allowlist: server-side settings
// and OIDC client credentials must never appear in it.
func TestLoad_RuntimeConfigExcludesServerSideKeys(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.server]
domain    = "localhost:5380"

[ai_workspace.auth]
mode = "oidc"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.oidc]
authority     = "https://idp.example.com"
client_id     = "client-id"
client_secret = "s3cr3t"
redirect_url  = "https://localhost:5380/api/auth/callback"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := cfg.RuntimeConfig["APIP_AIW_SERVER_DOMAIN"]; got != "localhost:5380" {
		t.Errorf("APIP_AIW_SERVER_DOMAIN = %q, want the browser-safe domain to be surfaced", got)
	}
	for _, v := range cfg.RuntimeConfig {
		if strings.Contains(v, "s3cr3t") || strings.Contains(v, "platform-api:9243") {
			t.Errorf("runtime config leaked a server-side value: %q", v)
		}
	}
	for _, key := range []string{"APIP_AIW_AUTH_OIDC_CLIENT_SECRET", "APIP_AIW_AUTH_OIDC_CLIENT_ID", "APIP_AIW_AUTH_OIDC_AUTHORITY"} {
		if _, ok := cfg.RuntimeConfig[key]; ok {
			t.Errorf("runtime config must not contain %s — the BFF owns the OIDC handshake", key)
		}
	}
}

// A browser-safe key reaches the SPA under the same name its {{ env }} token
// conventionally uses, so one spelling works in config.toml, the environment, and the
// browser.
func TestLoad_BrowserSafeKeyUsesSameName(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace]
moesif_web_url = "https://moesif.example.com"

[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.RuntimeConfig["APIP_AIW_MOESIF_WEB_URL"]; got != "https://moesif.example.com" {
		t.Errorf("APIP_AIW_MOESIF_WEB_URL = %q, want the config.toml value", got)
	}
}

// A browser-safe key whose token resolves from the environment must reach the SPA
// under that same name, exactly as if it had been written as a literal.
func TestLoad_BrowserSafeKeyFromEnvToken(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.server]
domain = '{{ env "APIP_AIW_SERVER_DOMAIN" "localhost:5380" }}'

[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)
	t.Setenv("APIP_AIW_SERVER_DOMAIN", "app.example.com")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.RuntimeConfig["APIP_AIW_SERVER_DOMAIN"]; got != "app.example.com" {
		t.Errorf("APIP_AIW_SERVER_DOMAIN = %q, want the token-resolved value to reach the browser", got)
	}
}

// TOML scalars may be written bare, not only as quoted strings: a token has to be a
// string, but a plain literal is naturally typed. Both forms must reach the same value.
func TestLoad_BareTOMLScalars(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.server.http]
enabled = true

[ai_workspace.server.https]
enabled = false

[ai_workspace.session]
absolute_ttl = "2h"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.HTTPS.Enabled {
		t.Error("Server.HTTPS.Enabled = true, want false from the bare TOML boolean")
	}
	if cfg.Session.AbsoluteTTL != 2*time.Hour {
		t.Errorf("Session.AbsoluteTTL = %s, want 2h", cfg.Session.AbsoluteTTL)
	}
}

// A key in a table must not collide with the same key in another table — they are
// distinct dotted paths, so [server.https] enabled and [auth.oidc] enabled are
// independent.
func TestLoad_SameKeyInDifferentTables(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.auth]
mode = "oidc"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.server.http]
enabled = true

[ai_workspace.server.https]
enabled = false

[ai_workspace.auth.oidc]
enabled       = true
authority     = "https://idp.example.com"
client_id     = "client-id"
client_secret = "s3cr3t"
redirect_url  = "https://localhost:5380/api/auth/callback"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.HTTPS.Enabled {
		t.Error("Server.HTTPS.Enabled = true, want false — [server.https] enabled must not read [auth.oidc] enabled")
	}
	if !cfg.Auth.OIDC.Enabled {
		t.Error("OIDC.Enabled = false, want true")
	}
}

// [auth.claim_mappings] mirrors the Platform API's [auth.claim_mappings] key for
// key, and is shared by both auth modes (not nested under [auth.oidc]): OIDC
// tokens from the configured IDP, and the HMAC JWTs the Platform API's
// file-based login endpoint signs using these same mapped claim names. The
// browser-safe ones reach the SPA under the matching
// APIP_AIW_AUTH_CLAIM_MAPPINGS_* names that src/config.env.ts looks up.
func TestLoad_ClaimMappingsMirrorControlPlane(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.claim_mappings]
organization = "org_uuid"
username     = "given_name"
roles        = "roles"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.ClaimMappings.OrgID != "org_uuid" {
		t.Errorf("Claims.OrgID = %q, want %q from organization", cfg.Auth.ClaimMappings.OrgID, "org_uuid")
	}
	if cfg.Auth.ClaimMappings.Roles != "roles" {
		t.Errorf("Claims.Roles = %q, want %q from roles", cfg.Auth.ClaimMappings.Roles, "roles")
	}
	if got := cfg.RuntimeConfig["APIP_AIW_AUTH_CLAIM_MAPPINGS_ORGANIZATION"]; got != "org_uuid" {
		t.Errorf("runtime APIP_AIW_AUTH_CLAIM_MAPPINGS_ORGANIZATION = %q, want %q", got, "org_uuid")
	}
	if got := cfg.RuntimeConfig["APIP_AIW_AUTH_CLAIM_MAPPINGS_USERNAME"]; got != "given_name" {
		t.Errorf("runtime APIP_AIW_AUTH_CLAIM_MAPPINGS_USERNAME = %q, want %q", got, "given_name")
	}
	// roles drives the BFF's session mapping only — it must not be published.
	if _, ok := cfg.RuntimeConfig["APIP_AIW_AUTH_CLAIM_MAPPINGS_ROLES"]; ok {
		t.Error("roles must not reach the browser — it is not in the browser-safe allowlist")
	}
}

// A malformed boolean must fail startup rather than fall back to the default.
func TestLoad_InvalidBool_Errors(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.server.https]
enabled = "maybe"
`)

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("Load() succeeded, want an error for a malformed boolean")
	}
}
