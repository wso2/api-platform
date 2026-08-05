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
// window.__RUNTIME_CONFIG__. It is an allowlist, not a filter: only these keys
// ever reach the browser, so a new server-side key (a secret, an upstream URL, a
// cookie setting) cannot leak into the page merely by being added to
// config.toml.
//
// [auth.oidc] client_secret / client_id / authority are deliberately absent — the
// BFF performs the whole OIDC handshake, so the SPA needs no client identity.
var browserSafeKeys = []string{
	"logging.browser_debug",
	"auth.oidc.scope",
	"auth.claim_mappings.username",
	"auth.claim_mappings.email",
	"auth.claim_mappings.organization",
	"auth.claim_mappings.org_name",
	"auth.claim_mappings.org_handle",
}

// runtimeKey converts a config key into the name the SPA reads: EnvPrefix + the
// key's dotted path uppercased, with dots as underscores ("auth.oidc.scope" ->
// APIP_ACP_AUTH_OIDC_SCOPE). It is the same spelling the key's {{ env }} token
// conventionally names, so a value has one name across config.toml, the
// environment, and window.__RUNTIME_CONFIG__.
func runtimeKey(configKey string) string {
	return EnvPrefix + strings.ToUpper(strings.ReplaceAll(configKey, ".", "_"))
}

// buildRuntimeConfig collects the browser-safe values the SPA reads from
// window.__RUNTIME_CONFIG__, then forces the API base URL onto the same-origin
// proxy prefix so the browser only ever talks to this BFF.
//
// Values are read straight from the parsed config (k, rooted at
// [api_control_plane]), not from the resolved Config struct, so only keys
// actually present in config.toml are surfaced — a code default is used
// internally by the BFF but never pushed to the browser.
func buildRuntimeConfig(cfg *Config, k *koanf.Koanf) map[string]string {
	out := make(map[string]string, len(browserSafeKeys)+2)
	for _, key := range browserSafeKeys {
		if !k.Exists(key) {
			continue
		}
		if v := fmt.Sprint(k.Get(key)); v != "" {
			out[runtimeKey(key)] = v
		}
	}

	out[runtimeKey("platform_api_base_url")] = cfg.ControlPlane.ProxyPrefix + "/api/v0.9"
	out[runtimeKey("auth_mode")] = cfg.Auth.Mode

	return out
}
