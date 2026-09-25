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
	"testing"

	"ai-workspace-bff/internal/paths"
)

func TestNormalizeBasePath(t *testing.T) {
	for _, tc := range []struct{ in, def, want string }{
		{"", paths.PlatformAPI, paths.PlatformAPI},  // unset falls back, never to "/"
		{"/", paths.PlatformAPI, paths.PlatformAPI}, // a lone slash is still "unset"
		{"v0.9", paths.PlatformAPI, "/v0.9"},        // leading slash added
		{"/v0.9/", paths.PlatformAPI, "/v0.9"},      // trailing slash dropped
		{"/api/v0.9", paths.PlatformAPI, "/api/v0.9"},
	} {
		if got := normalizeBasePath(tc.in, tc.def); got != tc.want {
			t.Errorf("normalizeBasePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUpstreamPath(t *testing.T) {
	// Defaults: the BFF's own prefixes, so nothing is rewritten.
	def := ControlPlaneConfig{PlatformAPIBasePath: paths.PlatformAPI, PortalAPIBasePath: paths.PortalAPI}
	for _, p := range []string{
		"/api/v0.9/organizations",
		"/api/portal/v0.9/auth/login",
		"/healthz",
	} {
		if got := def.UpstreamPath(p); got != p {
			t.Errorf("default mapping changed %q to %q", p, got)
		}
	}

	// The Choreo STS shape: url = https://sts.../api/am/platform-api, resources /v0.9/*.
	gw := ControlPlaneConfig{PlatformAPIBasePath: "/v0.9", PortalAPIBasePath: "/portal/v0.9"}
	for _, tc := range []struct{ in, want string }{
		{"/api/v0.9/organizations", "/v0.9/organizations"},
		{"/api/v0.9", "/v0.9"},
		{"/api/portal/v0.9/auth/login", "/portal/v0.9/auth/login"},
		// Prefix matches are whole-segment only, and anything under neither API
		// prefix is forwarded untouched rather than guessed at.
		{"/api/v0.99/x", "/api/v0.99/x"},
		{"/api/v0.9x/y", "/api/v0.9x/y"},
		{"/healthz", "/healthz"},
	} {
		if got := gw.UpstreamPath(tc.in); got != tc.want {
			t.Errorf("UpstreamPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
