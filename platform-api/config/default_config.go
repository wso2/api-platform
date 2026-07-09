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
		LogLevel:                   "INFO",
		LogFormat:                  "text",
		Port:                       "9243",
		DBSchemaPath:               "./internal/database/schema.sql",
		OpenAPISpecPath:            "./resources/openapi.yaml",
		LLMTemplateDefinitionsPath: "./resources/default-llm-provider-templates",
		EnableScopeValidation:      true,
		Database: Database{
			Driver:          "sqlite3",
			Path:            "./data/api_platform.db",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 300,
		},
		Auth: Auth{
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
				"/api/internal/v1/gateways",
				"/api/internal/v1/deployments",
				"/api/internal/v1/artifacts",
				"/api/internal/v1/secrets",
				"/api/internal/v1/websub-apis",
				"/api/internal/v1/webbroker-apis",
				"/api/internal/" + constants.APIVersion + "/webhook/events",
			},
			JWT: JWT{
				Enabled:        true,
				Issuer:         "platform-api",
				SkipValidation: true,
			},
			IDP: IDP{
				Enabled:        false,
				ValidationMode: "scope",
				ClaimMappings: IDPClaimMappings{
					OrganizationClaimName: "organization",
					OrgNameClaimName:      "org_name",
					OrgHandleClaimName:    "org_handle",
					UserIDClaimName:       "sub",
					UsernameClaimName:     "username",
					EmailClaimName:        "email",
					ScopeClaimName:        "scope",
				},
			},
			FileBased: FileBased{
				Enabled: false,
				Organization: FileBasedOrg{
					ID:          "default",
					DisplayName: "Default",
					Region:      "us",
					// UUID left empty: seedFileBasedOrg generates one at startup
					// unless an operator pins it via config/env for a stable org.
				},
				Users: FileBasedUsers{
					{
						Username:     "admin",
						PasswordHash: "$2y$10$U2yKMwGamGwDoMu0hRPT7u8nCuP8z/qxHFOKV6dhIxkJN9NJ0eVQ.",
						Scopes:       "ap:organization:manage ap:gateway:manage ap:gateway_custom_policy:manage ap:rest_api:manage ap:llm_provider:manage ap:llm_proxy:manage ap:mcp_proxy:manage ap:webbroker_api:manage ap:websub_api:manage ap:application:manage ap:subscription:manage ap:subscription_plan:manage ap:project:manage ap:llm_template:manage ap:devportal:manage ap:api_key:read ap:secret:manage",
					},
				},
			},
		},
		WebSocket: WebSocket{
			MaxConnections:       1000,
			ConnectionTimeout:    30,
			RateLimitPerMin:      1000,
			MaxConnectionsPerOrg: 3,
			MetricsLogEnabled:    true,
			MetricsLogInterval:   10,
		},
		DefaultDevPortal: DefaultDevPortal{
			Enabled:               true,
			Name:                  "Default DevPortal",
			Identifier:            "default",
			APIUrl:                "http://localhost:3001",
			Hostname:              "devportal.local",
			APIKey:                "default-api-key",
			HeaderKeyName:         "x-wso2-api-key",
			Timeout:               10,
			RoleClaimName:         "roles",
			GroupsClaimName:       "groups",
			OrganizationClaimName: "organizationID",
			AdminRole:             "admin",
			SubscriberRole:        "Internal/subscriber",
			SuperAdminRole:        "superAdmin",
		},
		Deployments: Deployments{
			MaxPerAPIGateway: 20,
			TimeoutEnabled:   true,
			TimeoutInterval:  20,
			TimeoutDuration:  60,
		},
		// ArtifactLimits are unlimited by default: every limit is left at its
		// zero value, which LimitReached treats as "no limit". Operators can cap
		// a specific artifact kind per organization by setting a positive value
		// (config file key artifact_limits.max_* or env ARTIFACT_LIMITS_MAX_*).
		ArtifactLimits: ArtifactLimits{},
		TLS: TLS{
			Enabled: true,
			CertDir: "./data/certs",
		},
		APIKey: APIKey{
			HashingAlgorithms: []string{"sha256"},
		},
		EventHub: EventHub{
			PollInterval:    3 * time.Second,
			CleanupInterval: 10 * time.Minute,
			RetentionPeriod: 1 * time.Hour,
		},
		Webhook: Webhook{
			Enabled:            false,
			GatewayType:        "wso2/api-platform",
			SignatureTolerance: 5 * time.Minute,
			MaxBodySize:        1 << 20, // 1 MiB
			SignatureHeader:    "X-Devportal-Signature",
		},
	}
}
