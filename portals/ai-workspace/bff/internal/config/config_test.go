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

// writeConfig writes a config.toml into a temp dir and points Load() at it.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv(configFileEnv, path)
}

// A config.toml value is used when no environment variable overrides it.
func TestLoad_ConfigFileValue(t *testing.T) {
	writeConfig(t, `
platform_api_url = "https://platform-api:9243"
log_level        = "warn"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PlatformAPI.URL != "https://platform-api:9243" {
		t.Errorf("PlatformAPI.URL = %q, want the config.toml value", cfg.PlatformAPI.URL)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warn")
	}
}

// Config-override env vars must carry the APIP_AIW_ prefix: the prefixed name wins
// over config.toml, while the same name without the prefix is ignored entirely.
func TestLoad_EnvPrefixOverridesFileAndBareNameIsIgnored(t *testing.T) {
	writeConfig(t, `
platform_api_url = "https://platform-api:9243"
log_level        = "warn"
`)
	t.Setenv("APIP_AIW_LOG_LEVEL", "debug") // honored
	t.Setenv("LOG_LEVEL", "error")          // ignored — unprefixed

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q (prefixed env must win, bare name must be ignored)", cfg.LogLevel, "debug")
	}
}

// The OIDC client secret is supplied as an ordinary prefixed config override, so it
// never has to be written into config.toml and needs no interpolation token.
func TestLoad_ClientSecretFromPrefixedEnv(t *testing.T) {
	writeConfig(t, `
platform_api_url  = "https://platform-api:9243"
auth_mode         = "oidc"
oidc_authority    = "https://idp.example.com"
oidc_client_id    = "client-id"
oidc_redirect_url = "https://localhost:5380/api/auth/callback"
`)
	t.Setenv("APIP_AIW_OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_CLIENT_SECRET", "ignored") // unprefixed: not a config key

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OIDC.ClientSecret != "s3cr3t" {
		t.Errorf("OIDC.ClientSecret = %q, want the APIP_AIW_-prefixed value", cfg.OIDC.ClientSecret)
	}
}

// An {{ env }} token resolves any environment variable by its bare name. The secret
// is normally supplied via APIP_AIW_OIDC_CLIENT_SECRET, but the token remains
// available for pointing a key at an arbitrarily-named variable.
func TestLoad_InterpolatesEnvToken(t *testing.T) {
	writeConfig(t, `
platform_api_url   = "https://platform-api:9243"
auth_mode          = "oidc"
oidc_authority     = "https://idp.example.com"
oidc_client_id     = "client-id"
oidc_client_secret = '{{ env "CUSTOM_SECRET_VAR" }}'
oidc_redirect_url  = "https://localhost:5380/api/auth/callback"
`)
	t.Setenv("CUSTOM_SECRET_VAR", "s3cr3t")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OIDC.ClientSecret != "s3cr3t" {
		t.Errorf("OIDC.ClientSecret = %q, want the value resolved from the env token", cfg.OIDC.ClientSecret)
	}
}

// A {{ file }} token reads a mounted secret file inside an allowed directory.
func TestLoad_InterpolatesFileToken(t *testing.T) {
	secretDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(secretDir, "oidc_client_secret"), []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("APIP_CONFIG_FILE_SOURCE_ALLOWLIST", secretDir)

	writeConfig(t, `
platform_api_url   = "https://platform-api:9243"
auth_mode          = "oidc"
oidc_authority     = "https://idp.example.com"
oidc_client_id     = "client-id"
oidc_client_secret = '{{ file "`+filepath.Join(secretDir, "oidc_client_secret")+`" }}'
oidc_redirect_url  = "https://localhost:5380/api/auth/callback"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// The trailing newline every secret file ends with must be trimmed.
	if cfg.OIDC.ClientSecret != "from-file" {
		t.Errorf("OIDC.ClientSecret = %q, want %q", cfg.OIDC.ClientSecret, "from-file")
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

	writeConfig(t, `
platform_api_url   = "https://platform-api:9243"
oidc_client_secret = '{{ file "`+outside+`" }}'
`)

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded, want an error for a file outside the allowlist")
	}
}

// An {{ env }} token whose variable is unset must abort startup, not silently
// yield an empty secret.
func TestLoad_MissingEnvToken_Errors(t *testing.T) {
	writeConfig(t, `
platform_api_url   = "https://platform-api:9243"
oidc_client_secret = '{{ env "CUSTOM_SECRET_VAR" }}'
`)
	t.Setenv("CUSTOM_SECRET_VAR", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want an error for an unset env token")
	}
	if !strings.Contains(err.Error(), "CUSTOM_SECRET_VAR") {
		t.Errorf("error = %v, want it to name the missing variable", err)
	}
}

// The upstream URL is mandatory — the BFF has nothing to proxy to without it.
func TestLoad_MissingPlatformAPIURL_Errors(t *testing.T) {
	writeConfig(t, `log_level = "info"`)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want an error when platform_api_url is unset")
	}
	if !strings.Contains(err.Error(), "platform_api_url") {
		t.Errorf("error = %v, want it to name platform_api_url", err)
	}
}

// The runtime config served to the browser is an allowlist: server-side settings
// and OIDC client credentials must never appear in it.
func TestLoad_RuntimeConfigExcludesServerSideKeys(t *testing.T) {
	writeConfig(t, `
platform_api_url   = "https://platform-api:9243"
auth_mode          = "oidc"
domain             = "localhost:5380"
oidc_authority     = "https://idp.example.com"
oidc_client_id     = "client-id"
oidc_client_secret = "s3cr3t"
oidc_redirect_url  = "https://localhost:5380/api/auth/callback"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := cfg.RuntimeConfig["APIP_AIW_DOMAIN"]; got != "localhost:5380" {
		t.Errorf("APIP_AIW_DOMAIN = %q, want the browser-safe domain to be surfaced", got)
	}
	for _, v := range cfg.RuntimeConfig {
		if strings.Contains(v, "s3cr3t") || strings.Contains(v, "platform-api:9243") {
			t.Errorf("runtime config leaked a server-side value: %q", v)
		}
	}
	for _, key := range []string{"APIP_AIW_OIDC_CLIENT_SECRET", "APIP_AIW_OIDC_CLIENT_ID", "APIP_AIW_OIDC_AUTHORITY"} {
		if _, ok := cfg.RuntimeConfig[key]; ok {
			t.Errorf("runtime config must not contain %s — the BFF owns the OIDC handshake", key)
		}
	}
}

// A browser-safe key reaches the SPA under the same name it has as an environment
// override, so one spelling works in config.toml, the environment, and the browser.
func TestLoad_BrowserSafeKeyUsesSameName(t *testing.T) {
	writeConfig(t, `
platform_api_url = "https://platform-api:9243"
moesif_web_url   = "https://moesif.example.com"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.RuntimeConfig["APIP_AIW_MOESIF_WEB_URL"]; got != "https://moesif.example.com" {
		t.Errorf("APIP_AIW_MOESIF_WEB_URL = %q, want the config.toml value", got)
	}
}

// A browser-safe key set via its APIP_AIW_ environment override must reach the SPA
// under that same name, exactly as if it had been set in config.toml.
func TestLoad_BrowserSafeKeyFromEnvOverride(t *testing.T) {
	writeConfig(t, `platform_api_url = "https://platform-api:9243"`)
	t.Setenv("APIP_AIW_DOMAIN", "app.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.RuntimeConfig["APIP_AIW_DOMAIN"]; got != "app.example.com" {
		t.Errorf("APIP_AIW_DOMAIN = %q, want the env override to reach the browser", got)
	}
}

// A malformed boolean must fail startup rather than fall back to the default.
func TestLoad_InvalidBool_Errors(t *testing.T) {
	writeConfig(t, `
platform_api_url = "https://platform-api:9243"
cookie_secure    = "maybe"
`)

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded, want an error for a malformed boolean")
	}
}
