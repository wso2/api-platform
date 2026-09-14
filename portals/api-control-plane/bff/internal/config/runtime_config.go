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

// buildRuntimeConfig collects the values the SPA reads from
// window.__RUNTIME_CONFIG__ (see src/config/runtime.ts's fromWindow()). Unlike
// ai-workspace's BFF — which invented its own APIP_AIW_* runtime-config
// vocabulary because its SPA was written alongside it — api-control-plane's
// SPA already has an established key vocabulary (camelCase, e.g.
// "platformApiBaseUrl") predating this BFF. This bridges to THOSE exact names
// rather than introducing a second convention the frontend would need to
// learn.
//
// Every backend call the SPA makes must go through this BFF's same-origin
// proxy — the browser never holds a token — so platformApiBaseUrl is always
// forced to the configured proxy prefix, never the upstream's real URL.
func buildRuntimeConfig(cfg *Config) map[string]string {
	out := map[string]string{
		"authMode":           cfg.Auth.Mode,
		"platformApiBaseUrl": cfg.ControlPlane.ProxyPrefix,
	}

	// billingProxyEnabled tells the SPA a "billing" named upstream exists, so
	// ProductActivation can call it (same-origin, via /proxy/billing/...)
	// without ever knowing the real billing service URL. Absent (defaults
	// false client-side) for every deployment that doesn't configure one —
	// every standalone deployment today.
	for _, u := range cfg.ControlPlane.Upstreams {
		switch u.Name {
		case "billing":
			out["billingProxyEnabled"] = "true"
		case "cloud":
			out["cloudProxyEnabled"] = "true"
		}
	}

	// Moesif wrap/basic iframe origin for cloud Insights embeds. Emitted
	// explicitly so the SPA never guesses from environmentName.
	if cfg.ControlPlane.MoesifAppURL != "" {
		out["moesifAppUrl"] = cfg.ControlPlane.MoesifAppURL
	}

	return out
}
