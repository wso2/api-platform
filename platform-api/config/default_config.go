/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package config

import (
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// defaultConfig returns a Server with all default values.
func defaultConfig() *Server {
	return &Server{
		Logging: Logging{
			Level:  "info",
			Format: "text",
		},
		DBSchemaPath:               "./internal/database/schema.sql",
		OpenAPISpecPath:            "./resources/openapi.yaml",
		LLMTemplateDefinitionsPath: "./resources/default-llm-provider-templates",
		Database: Database{
			Driver:          "sqlite3",
			Path:            "./data/api_platform.db",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 300,
		},
		Auth: Auth{
			// Default mode verifies locally-issued, asymmetrically-signed (RS256) JWTs;
			// the quickstart config selects "file" to add username/password login on top.
			Mode: AuthModeInternalToken,
			Authorization: Authorization{
				Enabled: true,
				Mode:    AuthzModeScope,
				// RoleToScopeMapping is left empty on purpose: the mapping file is
				// operator-owned and mounted (the packs ship a sample), so a
				// built-in path would make startup depend on a file the image
				// does not carry. The shipped config.toml points at the mount.
			},
			// SkipPaths bypasses JWT/IDP auth middleware. Paths below the health/metrics
			// probes are internal gateway routes authenticated via gateway token instead.
			SkipPaths: []string{
				"/health",
				"/metrics",
				"/api/portal/v0.9/auth/login",
				"/api/internal/v1/ws/gateways/connect",
				"/api/internal/v1/apis",
				"/api/internal/v1/llm-providers",
				"/api/internal/v1/llm-proxies",
				"/api/internal/v1/subscription-plans",
				"/api/internal/v1/mcp-proxies",
				"/api/internal/v1/graphql-apis",
				"/api/internal/v1/gateways",
				"/api/internal/v1/deployments",
				"/api/internal/v1/artifacts",
				"/api/internal/v1/secrets",
				"/api/internal/v1/websub-apis",
				"/api/internal/v1/webbroker-apis",
				"/api/internal/" + constants.APIVersion + "/webhook/events",
			},
			JWT: JWT{
				Issuer:   "platform-api",
				TokenTTL: time.Hour,
			},
			ClaimMappings: ClaimMappings{
				Organization: "organization",
				OrgName:      "org_name",
				OrgHandle:    "org_handle",
				UserID:       "sub",
				Username:     "username",
				Email:        "email",
				Scope:        "scope",
				// Default to the flat "roles" claim — what Asgardeo and Entra ID
				// emit, and what the file-mode login endpoint signs — so switching
				// auth.authorization.mode to "role" needs no extra claim wiring.
				// Keycloak overrides it with "realm_access.roles".
				Roles: "roles",
			},
			File: FileBased{
				Organization: FileBasedOrg{
					ID:          "default",
					DisplayName: "Default",
					Region:      "us",
					// UUID left empty: seedFileBasedOrg generates one at startup
					// unless an operator pins it via config/env for a stable org.
				},
				// No default user: shipping a functional username/password hash would give
				// every fresh install a known-credential admin — one that now also holds
				// ap:api_key:all:manage over every user's API keys. Operators supply the
				// admin via config (see config.toml's fail-closed {{ env }} tokens), and
				// validateFileBasedConfig refuses to start when auth.mode=file leaves this
				// empty.
				Users: FileBasedUsers{},
			},
		},
		Deployments: Deployments{
			MaxPerAPIGateway: 20,
			TimeoutEnabled:   true,
			TimeoutInterval:  20,
			TimeoutDuration:  60,
		},
		// By default the HTTPS listener serves on 9243 and the plain-HTTP listener
		// is off — preserving the historical single-TLS-port behavior. Enable the
		// HTTP listener (and/or move ports) via the [server.http] / [server.https]
		// config sections.
		Listeners: ServerListeners{
			HTTP: HTTPListener{
				Enabled: false,
				Port:    9080,
			},
			HTTPS: HTTPSListener{
				Enabled:                true,
				Port:                   9243,
				CertFile:               "./data/certs/cert.pem",
				KeyFile:                "./data/certs/key.pem",
				MinimumProtocolVersion: "TLS1_2",
				MaximumProtocolVersion: "TLS1_3",
				Ciphers:                "",
				EcdhCurves:             "X25519MLKEM768,X25519,P-256",
			},
			// Finite by default so a slow or idle peer cannot hold a connection open
			// indefinitely. Write is the loosest of the four because some handlers
			// proxy slow upstreams (LLM completions, deployments).
			Timeouts: Timeouts{
				ReadHeader: 10 * time.Second,
				Read:       60 * time.Second,
				Write:      120 * time.Second,
				Idle:       120 * time.Second,
			},
			WebSocket: WebSocket{
				MaxConnections:     1000,
				ConnectionTimeout:  30,
				RateLimitPerMin:    1000,
				MetricsLogEnabled:  true,
				MetricsLogInterval: 10,
			},
		},
		Security: Security{
			APIKey: APIKey{
				HashingAlgorithms: []string{"sha256"},
			},
		},
		EventHub: EventHub{
			PollInterval:    3 * time.Second,
			CleanupInterval: 10 * time.Minute,
			RetentionPeriod: 1 * time.Hour,
		},
		Webhook: Webhook{
			Enabled:            false,
			SignatureTolerance: 5 * time.Minute,
			MaxBodySize:        1 << 20, // 1 MiB
			SignatureHeader:    "X-Api-Portal-Signature",
		},
		HTTPClient: HTTPClientConfig{
			Pooling: HTTPClientPoolingConfig{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				MaxConnsPerHost:     100,
				IdleConnTimeout:     90 * time.Second,
				KeepAlive:           30 * time.Second,
				// A one-off outbound probe/call (MCP reachability, OpenAPI spec fetch) has
				// nothing to gain from connection reuse — matches the two pre-existing
				// per-call clients this consolidates, which both set this true.
				DisableKeepAlives: true,
			},
			Timeouts: HTTPClientTimeoutsConfig{
				Overall:        60 * time.Second, // safety-net only; see HTTPClientConfig's doc comment
				Dial:           10 * time.Second,
				TLSHandshake:   10 * time.Second,
				ResponseHeader: 10 * time.Second,
				ExpectContinue: 1 * time.Second,
				// MUST stay negative — see HTTPClientConfig's doc comment on why a
				// client-level cap here would silently shadow OpenAPISpecMaxFetchBytes.
				MaxResponseBytes: -1,
			},
			TLS: HTTPClientTLSConfig{
				MinVersion:       "TLS1_2",
				MaxVersion:       "TLS1_3",
				CurvePreferences: "",
			},
			Proxy: HTTPClientProxyConfig{
				Mode: "none",
			},
			SSRF: HTTPClientSSRFConfig{
				Enabled:        true,
				Preset:         "permit_private_block_metadata",
				MaxRedirects:   5,
				AllowedSchemes: []string{"http", "https"},
			},
		},
	}
}
