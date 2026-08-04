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

// Package paths holds the URL path prefixes this app is wired with. Every one of them
// is a fixed contract — between the BFF and the SPA it ships, or between the BFF and
// the Platform API version it is built against — rather than a deployment concern, so
// they are constants here instead of keys in config.toml. They live in their own
// package because both the server (which routes on them) and config (which composes
// the SPA's runtime config from them) need them.
package paths

// Base is the URL path prefix the whole app is served under: the SPA and its assets,
// the auth endpoints, the runtime-config script and the same-origin proxy all sit
// below it, so https://host:9643/ai-workspace/ is the UI and /ai-workspace/proxy/...
// the API hop. One ingress rule can therefore route a single prefix here with no path
// rewriting, and an all-in-one deployment can put several portals behind one host and
// port without their routes colliding.
//
// The same prefix is baked into the SPA bundle at build time (Vite's `base` in
// vite.config.ts) because index.html references its assets by absolute path — a server
// that moved without a matching rebuild would serve a page whose assets all 404. The
// two must be changed together, and the shipped image is built for this value.
//
// Health is the one route outside the prefix: /healthz answers at the origin root as
// well, so container and Kubernetes probes — which dial the pod directly, bypassing
// the ingress that adds the prefix — keep working.
const Base = "/ai-workspace"

// Proxy is the same-origin reverse-proxy prefix the SPA calls, sitting under Base: the
// browser calls /ai-workspace/proxy/... and the BFF strips both parts before
// forwarding, so the Platform API sees neither and the browser never talks to it
// directly. The SPA receives it pre-composed in its runtime config.
const Proxy = "/proxy"

// PlatformAPI and PortalAPI are the Platform API's versioned route prefixes: the main
// management API, and the portal API under which file-based login (/auth/login) lives.
// They are properties of the upstream this BFF is built against — a different prefix
// means a different API version, which needs code changes here, not a config edit.
const (
	PlatformAPI = "/api/v0.9"
	PortalAPI   = "/api/portal/v0.9"
)
