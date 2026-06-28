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

// tomlKeyToVite mirrors the case-statement in the legacy entrypoint.sh so the
// same config.toml keys continue to drive the SPA's VITE_* runtime config.
var tomlKeyToVite = map[string]string{
	"auth_mode":                 "VITE_AUTH_MODE",
	"domain":                    "VITE_DOMAIN",
	"oidc_authority":            "VITE_OIDC_AUTHORITY",
	"oidc_client_id":            "VITE_OIDC_CLIENT_ID",
	"default_org_region":        "VITE_DEFAULT_ORG_REGION",
	"platform_api_base_url":     "VITE_PLATFORM_API_BASE_URL",
	"controlplane_host":         "VITE_CONTROLPLANE_HOST",
	"oidc_org_id_claim":         "VITE_OIDC_ORG_ID_CLAIM",
	"oidc_org_name_claim":       "VITE_OIDC_ORG_NAME_CLAIM",
	"oidc_org_handle_claim":     "VITE_OIDC_ORG_HANDLE_CLAIM",
	"oidc_scope":                "VITE_OIDC_SCOPE",
	"platform_gateway_versions": "VITE_PLATFORM_GATEWAY_VERSIONS",
}

// applyTOMLToEnv reads simple key = value lines from config.toml and exports the
// mapped VITE_* environment variables — but only when they are not already set,
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

		viteKey, ok := tomlKeyToVite[key]
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(viteKey); !exists {
			_ = os.Setenv(viteKey, val)
		}
	}
}
