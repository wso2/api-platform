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
	"strings"
)

// secretViteKeys are VITE_* values that must NEVER be shipped to the browser.
// (The OIDC handshake is done entirely by the BFF, so the SPA needs no client
// id/secret. We strip both to keep the browser surface minimal.)
var secretViteKeys = map[string]bool{
	"VITE_OIDC_CLIENT_SECRET": true,
	"VITE_OIDC_CLIENT_ID":     true,
	"VITE_OIDC_AUTHORITY":     true,
}

// buildRuntimeConfig collects the browser-safe VITE_* values that the SPA reads
// from window.__RUNTIME_CONFIG__. It mirrors the old entrypoint.sh behaviour
// (export every VITE_* var) but:
//   - drops secret/OIDC-client keys (the BFF, not the SPA, talks to the IDP), and
//   - rewrites the Platform/Portal API base URLs to the same-origin proxy prefix
//     so the SPA only ever talks to the BFF.
func buildRuntimeConfig(cfg *Config) map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key := kv[:eq]
		if !strings.HasPrefix(key, "VITE_") || secretViteKeys[key] {
			continue
		}
		out[key] = kv[eq+1:]
	}

	// Force the SPA's API base URLs through the BFF same-origin proxy.
	out["VITE_PLATFORM_API_BASE_URL"] = cfg.ProxyPrefix + "/api/v0.9"
	out["VITE_PORTAL_API_BASE_URL"] = cfg.ProxyPrefix + "/api/portal/v0.9"
	out["VITE_AUTH_MODE"] = cfg.AuthMode

	return out
}
