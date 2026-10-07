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

// A merged multi-component config file also carries a foreign [platform_api] section
// with its own interpolation tokens — here deliberately poisonous ones: an {{ env }}
// with no default that is left unset, and a {{ file }} path outside the AI Workspace's
// allowlist. Load must interpolate and consume ONLY the [ai_workspace] subtree, leaving
// the foreign section (and its tokens) untouched. Guards the k.Cut(aiWorkspaceConfigKey)
// scoping in loadConfigKoanf: without cutting before interpolation, the whole-tree
// expand would fail closed on these tokens.
func TestLoad_IgnoresForeignComponentSection(t *testing.T) {
	// APIP_CP_SECURITY_ENCRYPTION_KEY is intentionally never set, and /etc/platform-api
	// is not on the AI Workspace's {{ file }} allowlist.
	cfgPath := writeConfig(t, `
[ai_workspace.logging]
level = "warn"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[platform_api.security]
encryption_key = '{{ env "APIP_CP_SECURITY_ENCRYPTION_KEY" }}'

[platform_api.auth.jwt]
public_key = '{{ file "/etc/platform-api/keys/jwt_public.pem" }}'
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v — the foreign [platform_api] tokens must not be resolved", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.Logging.Level, "warn")
	}
	if cfg.ControlPlane.URL != "https://platform-api:9243" {
		t.Errorf("ControlPlane.URL = %q, want the config.toml value", cfg.ControlPlane.URL)
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
redirect_url  = "https://localhost:9643/api/auth/callback"

[ai_workspace.session.cookie]
encryption_key = "test-session-key-at-least-32-characters"
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
redirect_url  = "https://localhost:9643/api/auth/callback"

[ai_workspace.session.cookie]
encryption_key = "test-session-key-at-least-32-characters"
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
	cfgPath := writeConfig(t, "[ai_workspace]\ndefault_org_region = \"us\"")

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
[ai_workspace]
default_org_region = "us"

[ai_workspace.auth]
mode = "oidc"

[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.oidc]
authority     = "https://idp.example.com"
client_id     = "client-id"
client_secret = "s3cr3t"
redirect_url  = "https://localhost:9643/api/auth/callback"

[ai_workspace.session.cookie]
encryption_key = "test-session-key-at-least-32-characters"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := cfg.RuntimeConfig["APIP_AIW_DEFAULT_ORG_REGION"]; got != "us" {
		t.Errorf("APIP_AIW_DEFAULT_ORG_REGION = %q, want the browser-safe value to be surfaced", got)
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
[ai_workspace]
default_org_region = '{{ env "APIP_AIW_DEFAULT_ORG_REGION" "us" }}'

[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)
	t.Setenv("APIP_AIW_DEFAULT_ORG_REGION", "eu")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.RuntimeConfig["APIP_AIW_DEFAULT_ORG_REGION"]; got != "eu" {
		t.Errorf("APIP_AIW_DEFAULT_ORG_REGION = %q, want the token-resolved value to reach the browser", got)
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
// distinct dotted paths, so [server.http] enabled and [server.https] enabled are
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
authority     = "https://idp.example.com"
client_id     = "client-id"
client_secret = "s3cr3t"
redirect_url  = "https://localhost:9643/api/auth/callback"

[ai_workspace.session.cookie]
encryption_key = "test-session-key-at-least-32-characters"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Server.HTTP.Enabled {
		t.Error("Server.HTTP.Enabled = false, want true")
	}
	if cfg.Server.HTTPS.Enabled {
		t.Error("Server.HTTPS.Enabled = true, want false — [server.https] enabled must not read [server.http] enabled")
	}
	if !cfg.Auth.OIDCEnabled() {
		t.Error("OIDCEnabled() = false, want true — [auth] mode is the only OIDC switch")
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

// Scope mode is the default, so an operator who never mentions [auth.authorization]
// keeps today's behaviour.
func TestLoad_AuthorizationModeDefaultsToScope(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.Authorization.Mode != AuthzModeScope {
		t.Errorf("Authorization.Mode = %q, want %q", cfg.Auth.Authorization.Mode, AuthzModeScope)
	}
}

func TestLoad_AuthorizationRoleMode(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.authorization]
mode = "role"
role_to_scope_mapping = "/etc/ai-workspace/role-to-scope-mapping.yaml"
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.Authorization.Mode != AuthzModeRole {
		t.Errorf("Authorization.Mode = %q, want %q", cfg.Auth.Authorization.Mode, AuthzModeRole)
	}
	if cfg.Auth.Authorization.RoleToScopeMapping == "" {
		t.Error("RoleToScopeMapping is empty, want the configured path")
	}
	// The grant table is a server-side concern; the SPA gates on the scopes
	// /api/session reports, never on the table itself.
	if _, ok := cfg.RuntimeConfig["APIP_AIW_AUTH_AUTHORIZATION_ROLE_TO_SCOPE_MAPPING"]; ok {
		t.Error("role_to_scope_mapping must not reach the browser")
	}
}

// Role mode with no grant table can only expand to zero scopes, which would present a
// UI in which nothing is permitted. Refuse to start instead.
func TestLoad_RoleModeWithoutMapping_Errors(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.authorization]
mode = "role"
`)
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() succeeded, want an error when role mode has no role_to_scope_mapping")
	}
	if !strings.Contains(err.Error(), "role_to_scope_mapping is required") {
		t.Errorf("error = %v, want it to name role_to_scope_mapping", err)
	}
}

// A typo'd mode must not silently degrade to reading the scope claim.
func TestLoad_InvalidAuthorizationMode_Errors(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"

[ai_workspace.auth.authorization]
mode = "roles"
`)
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() succeeded, want an error for an unknown authorization mode")
	}
	if !strings.Contains(err.Error(), "[auth.authorization] mode") {
		t.Errorf("error = %v, want it to name [auth.authorization] mode", err)
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

// The SPA composes its API base URLs itself, from import.meta.env.BASE_URL plus the
// same fixed prefixes the BFF strips (src/config.env.ts). Emitting them here as well
// would create a runtime value that could disagree with the prefix this server
// actually routes and strips, so their absence is asserted, not incidental.
func TestLoad_RuntimeConfigOmitsAPIBaseURLs(t *testing.T) {
	cfgPath := writeConfig(t, `
[ai_workspace.control_plane]
url = "https://platform-api:9243"
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, key := range []string{"APIP_AIW_PLATFORM_API_BASE_URL", "APIP_AIW_PORTAL_API_BASE_URL"} {
		if got, ok := cfg.RuntimeConfig[key]; ok {
			t.Errorf("RuntimeConfig[%s] = %q, want it absent — the SPA composes it from its own base path", key, got)
		}
	}
	if got := cfg.RuntimeConfig["APIP_AIW_AUTH_MODE"]; got == "" {
		t.Error("RuntimeConfig[APIP_AIW_AUTH_MODE] is empty, want the resolved auth mode")
	}
}

// There is one store, so the default must be it.
func TestSessionStoreDefaultsToCookie(t *testing.T) {
	if got := defaultConfig().Session.Store; got != SessionStoreCookie {
		t.Fatalf("default [session] store = %q, want %q", got, SessionStoreCookie)
	}
}

// A config that omits the key entirely must land on the only store there is, rather
// than on an empty string that nothing recognises.
func TestConfigWithoutSessionStoreKeyDefaultsToCookie(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.toml")
	if err := os.WriteFile(path, []byte(`
[ai_workspace]
[ai_workspace.control_plane]
url = "https://platform-api:9243"
[ai_workspace.session]
idle_timeout = "30m"
absolute_ttl = "8h"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Session.Store != SessionStoreCookie {
		t.Fatalf("[session] store = %q for a config that omits it, want %q",
			cfg.Session.Store, SessionStoreCookie)
	}
}

// The key checks are a sanity filter for a hand-made value, not a measurement of
// entropy. The repeated-pattern cases are the ones that matter: a character-frequency
// score rates "0123456789abcdef" four times exactly as highly as a genuinely random
// 64-character hex key, so that check alone vouched for a value with no entropy at all.
func TestEncryptionKeyRejectsHandMadeValues(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		weak      bool
	}{
		{"too short", "short", true},
		{"one repeated character", strings.Repeat("a", 64), true},
		{"a two-character pattern", strings.Repeat("ab", 32), true},
		{"a 16-character pattern", strings.Repeat("abcdefghijklmnop", 4), true},
		{"a hex alphabet pattern", strings.Repeat("0123456789abcdef", 4), true},
		{"a mixed-case pattern", strings.Repeat("aAbBcCdDeEfFgGhH", 4), true},
		{"a repeated password", strings.Repeat("Passw0rd!", 8), true},
		{"a repeated word", "passwordpasswordpasswordpassword", true},
		{"a repeated placeholder", "changeme-changeme-changeme-changeme", true},

		{"openssl rand -hex 32", "51e97c3a355ec9b8ffcb9bb0fdec8f1b9a7b1b3bef7d352e71d3f71ff2a484e0", false},
		{"openssl rand -base64 32", "K7x+Qm2ZpL9vN4sR8tW1yU6oE3iA5bC0dF/gH=jK", false},
		{"a long passphrase", "correct horse battery staple correct horse", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason := weakKeyReason(tc.key)
			if (reason != "") != tc.weak {
				t.Fatalf("weakKeyReason() = %q, want weak=%v", reason, tc.weak)
			}
			// The reason is surfaced in a startup error, so it must never carry the
			// secret or a fragment of it.
			if reason != "" && len(tc.key) >= 4 && strings.Contains(reason, tc.key[:4]) {
				t.Errorf("the rejection reason leaks part of the key: %q", reason)
			}
		})
	}
}

// A pattern and real material of the same alphabet are indistinguishable by character
// frequency alone — the gap shortestPeriod exists to close.
func TestVarietyScoreCannotSeeRepetition(t *testing.T) {
	pattern := strings.Repeat("0123456789abcdef", 4)
	if varietyScoreBits(pattern) < minVarietyScoreBits {
		t.Fatal("precondition: the pattern is expected to pass the variety check on its own")
	}
	if weakKeyReason(pattern) == "" {
		t.Fatal("the repeated pattern was accepted — shortestPeriod did not catch it")
	}
}

// The key moved from [session] to [session.cookie] before release. A config carrying
// the old spelling must be told so, rather than meeting "encryption_key is required"
// with the value plainly set in front of the operator.
// validOIDCConfig is the smallest config that passes validate in OIDC mode.
func validOIDCConfig(t *testing.T) *Config {
	t.Helper()
	c := defaultConfig()
	c.Session.Cookie.EncryptionKey = strings.Repeat("Ab3!xY7#", 8)
	c.Auth.Mode = AuthModeOIDC
	c.Auth.Authorization.Mode = AuthzModeScope
	c.Auth.OIDC.Issuer = "https://idp.example.com"
	c.Auth.OIDC.ClientID = "client"
	c.Auth.OIDC.ClientSecret = "a-client-secret-long-enough-to-pass"
	c.Auth.OIDC.RedirectURL = "https://portal.example.com/cb"
	c.Server.HTTPS.Enabled = false
	c.Server.HTTP.Enabled = true
	c.Server.HTTP.Port = 8080
	c.ControlPlane.URL = "https://platform-api:9243"
	return c
}

func TestLegacyTopLevelEncryptionKeyIsNamed(t *testing.T) {
	cfg := validOIDCConfig(t)
	cfg.Session.Cookie.EncryptionKey = ""
	cfg.Session.LegacyEncryptionKey = strings.Repeat("k", MinSessionKeyLength)

	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "[session.cookie]") {
		t.Fatalf("validate() = %v, want it to name the new location", err)
	}
}
