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

import "time"

// defaultConfig returns a Config with all built-in defaults. Load overlays the parsed
// config.toml on top of it, so any key absent from the file keeps the value here. The
// required keys (control_plane.url) have no meaningful default and are enforced by
// Config.validate instead. Cookie and RuntimeConfig are populated by Load, not here.
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			StaticDir: "/app",
			HTTPS: HTTPSConfig{
				Enabled: true,
				Port:    5380,
				// Convention matches the container's mount path. A certificate pair is
				// required there whenever the listener terminates TLS.
				CertFile: "/etc/ai-workspace/tls/cert.pem",
				KeyFile:  "/etc/ai-workspace/tls/key.pem",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		ControlPlane: ControlPlaneConfig{
			PortalBasePath: "/api/portal/v0.9",
			ProxyPrefix:    "/proxy",
		},
		Session: SessionConfig{
			Store:       "memory",
			IdleTimeout: 30 * time.Minute,
			AbsoluteTTL: 8 * time.Hour,
		},
		Auth: AuthConfig{
			Mode: "basic",
			OIDC: OIDCConfig{
				Scopes: defaultOIDCScopes,
			},
			// Defaults mirror the Platform API's own claim_mappings defaults so the two
			// agree out of the box; override on both sides together when an IDP uses
			// different claim names (e.g. Asgardeo's "org_id").
			ClaimMappings: ClaimMappingConfig{
				Username:  "username",
				Email:     "email",
				Roles:     "roles",
				Scope:     "scope",
				OrgID:     "organization",
				OrgName:   "org_name",
				OrgHandle: "org_handle",
			},
		},
	}
}
