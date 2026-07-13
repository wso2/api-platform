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
	"bufio"
	"os"
	"strings"
)

// tomlKeyToEnv maps each config.toml key to the environment variable the rest of
// the BFF reads it from. Two kinds of target exist:
//
//   - VITE_* keys drive the SPA's browser runtime config (served via
//     /runtime-config.js). These mirror the case-statement in the legacy
//     entrypoint.sh so existing config.toml files keep working.
//   - Plain (non-VITE_) keys drive the BFF's own server-side behaviour — the
//     upstream Platform API, session/cookie lifetime, CSRF, TLS, etc. These are
//     read by Load() and are NEVER emitted to the browser (buildRuntimeConfig
//     only copies VITE_* keys), so backend-only settings stay server-side.
//
// The OIDC client secret is deliberately absent: secrets belong in the
// environment / a .env file, not in a config file that may be committed.
var tomlKeyToEnv = map[string]string{
	// --- SPA runtime config (served to the browser) ---
	"auth_mode":                 "VITE_AUTH_MODE",
	"domain":                    "VITE_DOMAIN",
	"oidc_authority":            "VITE_OIDC_AUTHORITY",
	"oidc_client_id":            "VITE_OIDC_CLIENT_ID",
	"default_org_region":        "VITE_DEFAULT_ORG_REGION",
	"platform_api_base_url":     "VITE_PLATFORM_API_BASE_URL",
	"controlplane_host":         "VITE_CONTROLPLANE_HOST",
	"oidc_username_claim":       "VITE_OIDC_USERNAME_CLAIM",
	"oidc_org_id_claim":         "VITE_OIDC_ORG_ID_CLAIM",
	"oidc_org_name_claim":       "VITE_OIDC_ORG_NAME_CLAIM",
	"oidc_org_handle_claim":     "VITE_OIDC_ORG_HANDLE_CLAIM",
	"oidc_scope":                "VITE_OIDC_SCOPE",
	"platform_gateway_versions": "VITE_PLATFORM_GATEWAY_VERSIONS",

	// --- BFF listener & TLS ---
	"bff_addr":        "BFF_ADDR",
	"tls_enabled":     "BFF_TLS_ENABLED",
	"tls_self_signed": "BFF_TLS_SELF_SIGNED",
	"tls_cert_file":   "BFF_TLS_CERT_FILE",
	"tls_key_file":    "BFF_TLS_KEY_FILE",

	// --- Logging ---
	"log_level":  "LOG_LEVEL",
	"log_format": "LOG_FORMAT",

	// --- Upstream Platform API ---
	"platform_api_url":             "PLATFORM_API_URL",
	"platform_api_ca_file":         "PLATFORM_API_CA_FILE",
	"platform_api_tls_skip_verify": "PLATFORM_API_TLS_SKIP_VERIFY",
	"platform_login_path":          "PLATFORM_LOGIN_PATH",
	"proxy_prefix":                 "PROXY_PREFIX",

	// --- Session & cookie ---
	"session_store":        "SESSION_STORE",
	"session_idle_timeout": "SESSION_IDLE_TIMEOUT",
	"session_absolute_ttl": "SESSION_ABSOLUTE_TTL",
	"cookie_name":          "COOKIE_NAME",
	"cookie_secure":        "COOKIE_SECURE",
	"cookie_samesite":      "COOKIE_SAMESITE",
	"csrf_header":          "CSRF_HEADER",

	// --- OIDC server-side (BFF confidential client) ---
	"oidc_enabled":                  "OIDC_ENABLED",
	"oidc_redirect_url":             "OIDC_REDIRECT_URL",
	"oidc_post_logout_redirect_url": "OIDC_POST_LOGOUT_REDIRECT_URL",
	"oidc_email_claim":              "OIDC_CLAIM_EMAIL",
	"oidc_role_claim":               "OIDC_CLAIM_ROLE",
	"oidc_scope_claim":              "OIDC_CLAIM_SCOPE",
}

// applyTOMLToEnv reads simple key = value lines from config.toml and exports the
// mapped environment variables — but only when they are not already set,
// preserving the "env vars win" precedence of the old entrypoint.sh.
//
// This is intentionally a naive line parser (not a full TOML decoder) to match
// the previous shell behaviour and to keep the BFF dependency-free.
func applyTOMLToEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no config.toml mounted — env-only configuration
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)

		envKey, ok := tomlKeyToEnv[key]
		if !ok {
			continue
		}
		// Treat an empty env var as unset so a present-but-empty value doesn't
		// shadow the TOML value — matching getenv()'s "empty == unset" precedence.
		// A set, non-empty env var always wins over the TOML file.
		if v, exists := os.LookupEnv(envKey); !exists || v == "" {
			_ = os.Setenv(envKey, val)
		}
	}
}
