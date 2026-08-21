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
			HTTP: HTTPListener{
				Enabled: false,
				Port:    9680,
			},
			HTTPS: HTTPSListener{
				Enabled: true,
				Port:    9643,
				// Convention matches the container's mount path. A certificate pair is
				// required there whenever the listener terminates TLS.
				CertFile:               "/etc/ai-workspace/tls/cert.pem",
				KeyFile:                "/etc/ai-workspace/tls/key.pem",
				MinimumProtocolVersion: "TLS1_2",
				MaximumProtocolVersion: "TLS1_3",
				Ciphers:                "",
				EcdhCurves:             "X25519,P-256",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		// HTTPClient defaults reproduce exactly what proxy.NewTransport hardcoded
		// before this became configurable, so an existing deployment that omits
		// [ai_workspace.http_client] entirely sees zero behavior change. Everything
		// else (Pooling.MaxIdleConns, Timeouts.Dial/TLSHandshake/ResponseHeader/
		// ExpectContinue) already matches httpclient.DefaultConfig()'s own values, so
		// only the previously-hardcoded overrides are set explicitly below.
		HTTPClient: HTTPClientConfig{
			Pooling: HTTPClientPoolingConfig{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				MaxConnsPerHost:     0,
				IdleConnTimeout:     90 * time.Second,
				KeepAlive:           30 * time.Second,
				EnableHTTP2:         true,
			},
			Timeouts: HTTPClientTimeoutsConfig{
				Overall:          30 * time.Second,
				Dial:             10 * time.Second,
				TLSHandshake:     10 * time.Second,
				ResponseHeader:   10 * time.Second,
				ExpectContinue:   1 * time.Second,
				MaxResponseBytes: -1,
			},
			// TLS.MinVersion/MaxVersion/CipherSuites/CurvePreferences stay empty:
			// Go's own crypto/tls default (TLS 1.2 floor, no configured ceiling)
			// applies, matching today's behavior.
			Proxy: HTTPClientProxyConfig{
				Mode: "environment", // matches the previous hardcoded http.ProxyFromEnvironment
			},
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
			// Mirrors the Platform API's [auth.authorization] default. Both sides must
			// be switched to "role" together for an IDP that mints no ap:* scopes.
			Authorization: AuthorizationConfig{
				Mode: AuthzModeScope,
			},
		},
	}
}
