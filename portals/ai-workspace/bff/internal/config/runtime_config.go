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

import "strings"

// browserSafeKeys is the allowlist of config keys the SPA may read from
// window.__RUNTIME_CONFIG__. It is an allowlist, not a filter: only these keys ever
// reach the browser, so a new server-side key (a secret, an upstream URL, a cookie
// setting) cannot leak into the page merely by being added to config.toml.
//
// oidc_client_secret / oidc_client_id / oidc_authority are deliberately absent — the
// BFF performs the whole OIDC handshake, so the SPA needs no client identity.
var browserSafeKeys = []string{
	// Identity of the deployment
	"domain",
	"auth_mode",
	"default_org_region",
	"controlplane_host",
	"platform_gateway_versions",
	"csrf_header",
	"debug",

	// Claim names the SPA displays user/org identity from
	"oidc_scope",
	"oidc_username_claim",
	"oidc_email_claim",
	"oidc_org_id_claim",
	"oidc_org_name_claim",
	"oidc_org_handle_claim",

	// External links and SPA-only endpoints
	"dev_portal_base_url",
	"api_policy_hub",
	"policy_hub_web_url",
	"moesif_web_url",
	"moesif_app_api_key",
}

// runtimeKey converts a config key into the name the SPA reads. It is the same
// spelling as the key's environment-variable override (APIP_AIW_ + the uppercased
// key), so a value has exactly one name across config.toml, the environment, Vite's
// import.meta.env, and window.__RUNTIME_CONFIG__.
func runtimeKey(configKey string) string {
	return EnvPrefix + strings.ToUpper(configKey)
}

// buildRuntimeConfig collects the browser-safe values the SPA reads from
// window.__RUNTIME_CONFIG__, then forces the API base URLs onto the same-origin
// proxy prefix so the browser only ever talks to the BFF.
func buildRuntimeConfig(cfg *Config, s settings) map[string]string {
	out := make(map[string]string, len(browserSafeKeys)+3)
	for _, key := range browserSafeKeys {
		if v, ok := s[key]; ok && v != "" {
			out[runtimeKey(key)] = v
		}
	}

	// Force the SPA's API base URLs through the BFF same-origin proxy.
	out[runtimeKey("platform_api_base_url")] = cfg.ProxyPrefix + "/api/v0.9"
	out[runtimeKey("portal_api_base_url")] = cfg.ProxyPrefix + "/api/portal/v0.9"
	out[runtimeKey("auth_mode")] = cfg.AuthMode

	return out
}
