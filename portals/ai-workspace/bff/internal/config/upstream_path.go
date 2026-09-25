/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"strings"

	"ai-workspace-bff/internal/paths"
)

// normalizeBasePath accepts "v0.9", "/v0.9" and "/v0.9/" as the same value, and
// treats an empty one as unset — falling back to def rather than to the upstream
// root, since an {{ env }} token with nothing behind it means "not configured",
// not "the API is served from /".
func normalizeBasePath(p, def string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return def
	}
	return "/" + p
}

// UpstreamPath maps a path as this BFF composes it — always rooted at the prefix
// the Platform API serves itself, e.g. /api/v0.9/organizations — to the path the
// configured upstream actually exposes, by swapping that prefix for the configured
// base path. With the defaults it returns p unchanged.
//
// Both the reverse proxy and the BFF's own server-side calls go through it, so a
// browser-driven request and the session hydration behind it can never disagree
// about where the API lives.
//
// Only a whole prefix match counts: with platform_api_base_path = "/v0.9",
// "/api/v0.9/x" becomes "/v0.9/x", while "/api/v0.99/x" is left alone. A path
// under neither API prefix (nothing the BFF sends today) is forwarded untouched
// rather than guessed at.
func (c ControlPlaneConfig) UpstreamPath(p string) string {
	// Portal first: both prefixes start with /api, and only the more specific one
	// can match a portal path correctly.
	for _, m := range [...]struct{ canonical, configured string }{
		{paths.PortalAPI, c.PortalAPIBasePath},
		{paths.PlatformAPI, c.PlatformAPIBasePath},
	} {
		if m.configured == "" || m.configured == m.canonical {
			continue
		}
		if p == m.canonical {
			return m.configured
		}
		if rest, ok := strings.CutPrefix(p, m.canonical+"/"); ok {
			return m.configured + "/" + rest
		}
	}
	return p
}
