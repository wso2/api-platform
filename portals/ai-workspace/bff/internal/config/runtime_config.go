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
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
)

// browserSafeKeys is the allowlist of config keys the SPA may read from
// window.__RUNTIME_CONFIG__. It is an allowlist, not a filter: only these keys ever
// reach the browser, so a new server-side key (a secret, an upstream URL, a cookie
// setting) cannot leak into the page merely by being added to config.toml.
//
// [auth.oidc] client_secret / client_id / authority are deliberately absent — the BFF
// performs the whole OIDC handshake, so the SPA needs no client identity.
var browserSafeKeys = []string{
	// Identity of the deployment. auth.mode is not listed: buildRuntimeConfig
	// always emits it from the parsed cfg.Auth.Mode instead.
	"server.domain",
	"default_org_region",
	"gateway.controlplane_host",
	"gateway.platform_gateway_versions",
	"logging.browser_debug",

	// Claim names the SPA displays user/org identity from. The keys mirror the
	// Platform API's [auth.claim_mappings] exactly — same claim, same name. Shared
	// by both auth modes, so it lives under [auth.claim_mappings], not nested in
	// [auth.oidc].
	"auth.oidc.scope",
	"auth.claim_mappings.username",
	"auth.claim_mappings.email",
	"auth.claim_mappings.organization",
	"auth.claim_mappings.org_name",
	"auth.claim_mappings.org_handle",

	// External links and SPA-only endpoints
	"dev_portal_base_url",
	"api_policy_hub",
	"policy_hub_web_url",
	"moesif_web_url",
	"moesif_app_api_key",
}

// runtimeKey converts a config key into the name the SPA reads: APIP_AIW_ + the key's
// dotted path uppercased, with dots as underscores ("auth.oidc.scope" ->
// APIP_AIW_AUTH_OIDC_SCOPE). It is the same spelling the key's {{ env }} token
// conventionally names, so a value has one name across config.toml, the environment,
// Vite's import.meta.env, and window.__RUNTIME_CONFIG__.
func runtimeKey(configKey string) string {
	return EnvPrefix + strings.ToUpper(strings.ReplaceAll(configKey, ".", "_"))
}

// buildRuntimeConfig collects the browser-safe values the SPA reads from
// window.__RUNTIME_CONFIG__, then forces the API base URLs onto the same-origin
// proxy prefix so the browser only ever talks to the BFF.
//
// Values are read straight from the parsed config (k, rooted at [ai_workspace]), not
// from the resolved Config struct, so only keys actually present in config.toml are
// surfaced — a code default is used internally by the BFF but never pushed to the
// browser. Most browser-safe keys (domain, gateway.*, moesif_*, dev_portal_base_url,
// ...) are browser-only and not modeled on Config at all; this is where they flow out.
func buildRuntimeConfig(cfg *Config, k *koanf.Koanf) map[string]string {
	out := make(map[string]string, len(browserSafeKeys)+3)
	for _, key := range browserSafeKeys {
		if !k.Exists(key) {
			continue
		}
		// Stringify so a value written bare (a bool/int, e.g. logging.browser_debug)
		// reaches the SPA the same as a quoted one.
		if v := fmt.Sprint(k.Get(key)); v != "" {
			out[runtimeKey(key)] = v
		}
	}

	// Force the SPA's API base URLs through the BFF same-origin proxy.
	out[runtimeKey("platform_api_base_url")] = cfg.ControlPlane.ProxyPrefix + "/api/v0.9"
	out[runtimeKey("portal_api_base_url")] = cfg.ControlPlane.ProxyPrefix + "/api/portal/v0.9"
	out[runtimeKey("auth_mode")] = cfg.Auth.Mode

	return out
}
