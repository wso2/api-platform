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
	"path/filepath"
	"testing"
)

// The files that ship with the distribution. configs/config.toml is the one the
// container mounts and `make bff-run` passes to -config, so it is the only config the
// quickstart ever loads; config-template.toml is the full-key reference users copy.
var (
	quickstartConfig = filepath.Join("..", "..", "..", "configs", "config.toml")
	templateConfig   = filepath.Join("..", "..", "..", "configs", "config-template.toml")
)

// The shipped config.toml must load with nothing set in the environment — that is the
// quickstart: `docker compose up` with no .env beyond the two secrets. Its [oidc] keys
// default to empty, which must leave OIDC off rather than fail startup.
func TestShippedConfig_QuickstartLoadsWithNoEnv(t *testing.T) {
	cfg, err := Load(quickstartConfig)
	if err != nil {
		t.Fatalf("Load(configs/config.toml) error = %v — the shipped quickstart config must load with an empty environment", err)
	}

	if cfg.AuthMode != "basic" {
		t.Errorf("AuthMode = %q, want %q", cfg.AuthMode, "basic")
	}
	// The empty [oidc] defaults must not switch the OIDC client on, or Load would
	// demand an authority/client_id/client_secret the quickstart has no use for.
	if cfg.OIDC.Enabled {
		t.Error("OIDC.Enabled = true, want false — the empty [oidc] defaults must leave basic mode intact")
	}
	// The defaults in the file are the container's: the port it publishes and the SPA
	// baked into the image.
	if cfg.Addr != ":5380" {
		t.Errorf("Addr = %q, want the container default %q", cfg.Addr, ":5380")
	}
	if cfg.StaticDir != "/app" {
		t.Errorf("StaticDir = %q, want the container default %q", cfg.StaticDir, "/app")
	}
	if cfg.PlatformAPI.URL != "https://platform-api:9243" {
		t.Errorf("PlatformAPI.URL = %q, want the compose hostname", cfg.PlatformAPI.URL)
	}
	// The Platform API generates its own certificate in demo mode, so the upstream hop
	// cannot verify it. The https scheme still stands: the hop is encrypted.
	if !cfg.PlatformAPI.TLSSkipVerify {
		t.Error("PlatformAPI.TLSSkipVerify = false, want true — the quickstart upstream serves a self-signed certificate")
	}
}

// `make bff-run` loads this same file and points the container-shaped defaults at the
// developer's machine, purely through the {{ env }} tokens in it. A token is the only
// way an environment variable reaches a key, so dropping one of these keys from
// config.toml would silently strip the override and leave the BFF on :5380 serving
// /app. This test pins the Makefile's contract with the file.
func TestShippedConfig_MakeBffRunOverrides(t *testing.T) {
	// Exactly the variables the bff-run target sets.
	t.Setenv("APIP_AIW_PLATFORM_API_URL", "https://localhost:9243")
	t.Setenv("APIP_AIW_PLATFORM_API_TLS_SKIP_VERIFY", "true")
	t.Setenv("APIP_AIW_LISTEN_ADDR", ":8081")
	t.Setenv("APIP_AIW_STATIC_DIR", "../dist")
	t.Setenv("APIP_AIW_LOG_LEVEL", "debug")

	cfg, err := Load(quickstartConfig)
	if err != nil {
		t.Fatalf("Load(configs/config.toml) error = %v", err)
	}

	for _, tc := range []struct{ name, got, want string }{
		{"Addr", cfg.Addr, ":8081"},
		{"StaticDir", cfg.StaticDir, "../dist"},
		{"PlatformAPI.URL", cfg.PlatformAPI.URL, "https://localhost:9243"},
		{"LogLevel", cfg.LogLevel, "debug"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q — `make bff-run` sets this variable, so configs/config.toml must carry the matching {{ env }} token",
				tc.name, tc.got, tc.want)
		}
	}
}

// The [oidc] tokens in the shipped config are what let `make bff-run` — and a compose
// deployment — turn on OIDC from the environment alone, without editing the file.
func TestShippedConfig_OIDCFromEnvAlone(t *testing.T) {
	t.Setenv("APIP_AIW_AUTH_MODE", "oidc")
	t.Setenv("APIP_AIW_OIDC_AUTHORITY", "https://idp.example.com")
	t.Setenv("APIP_AIW_OIDC_CLIENT_ID", "client-id")
	t.Setenv("APIP_AIW_OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("APIP_AIW_OIDC_REDIRECT_URL", "https://localhost:5380/api/auth/callback")

	cfg, err := Load(quickstartConfig)
	if err != nil {
		t.Fatalf("Load(configs/config.toml) error = %v — the [oidc] tokens must make OIDC settable from the environment", err)
	}
	if !cfg.OIDC.Enabled {
		t.Fatal("OIDC.Enabled = false, want true — auth_mode = oidc must enable the client")
	}
	if cfg.OIDC.ClientSecret != "s3cr3t" {
		t.Errorf("OIDC.ClientSecret = %q, want the value from APIP_AIW_OIDC_CLIENT_SECRET", cfg.OIDC.ClientSecret)
	}
	// Whatever the mode, the client credentials stay server-side.
	for _, key := range []string{"APIP_AIW_OIDC_CLIENT_SECRET", "APIP_AIW_OIDC_CLIENT_ID", "APIP_AIW_OIDC_AUTHORITY"} {
		if _, ok := cfg.RuntimeConfig[key]; ok {
			t.Errorf("runtime config must not contain %s — the BFF owns the OIDC handshake", key)
		}
	}
}

// config-template.toml is copied to config.toml by users, so it must stay loadable.
// Its client_secret token deliberately has no default — an unset variable fails startup
// rather than running OIDC with an empty credential — so the variable is set here.
func TestShippedConfig_TemplateLoads(t *testing.T) {
	t.Setenv("APIP_AIW_OIDC_CLIENT_SECRET", "s3cr3t")

	cfg, err := Load(templateConfig)
	if err != nil {
		t.Fatalf("Load(configs/config-template.toml) error = %v — the template must remain a loadable config", err)
	}
	if cfg.AuthMode != "basic" {
		t.Errorf("AuthMode = %q, want %q", cfg.AuthMode, "basic")
	}
	// Unlike the quickstart file, the template defaults to verifying the upstream
	// certificate — it is the starting point for a real deployment.
	if cfg.PlatformAPI.TLSSkipVerify {
		t.Error("PlatformAPI.TLSSkipVerify = true, want false — the template must default to a verified upstream")
	}
}

// The template's client_secret has no default on purpose: leaving the variable unset
// must abort startup, not resolve to an empty credential.
func TestShippedConfig_TemplateFailsClosedOnMissingSecret(t *testing.T) {
	t.Setenv("APIP_AIW_OIDC_CLIENT_SECRET", "")

	if _, err := Load(templateConfig); err == nil {
		t.Fatal("Load(configs/config-template.toml) succeeded with APIP_AIW_OIDC_CLIENT_SECRET unset, want an error — the token has no default and must fail closed")
	}
}
