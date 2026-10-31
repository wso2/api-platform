/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
 * Configuration for AI Workspace Authentication
 */

// Extend Window interface to include runtime config
declare global {
  interface Window {
    __RUNTIME_CONFIG__?: Record<string, string>;
  }
}

import { getEnvOrDefault } from './utils/getEnvOrDefault';
// Path prefixes are fixed contracts with the BFF, not configuration — see ./paths.
import { BASE_PATH } from './paths';


/*
 * Single line environment variable definitions with defaults using getEnvOrDefault utility to improve readability and maintainability.
 */

// Verbose logging in the browser console.
export const DEBUG = getEnvOrDefault('APIP_AIW_LOGGING_BROWSER_DEBUG', false);

// Default region used when auto-registering an organization on first login.
export const DEFAULT_ORG_REGION = getEnvOrDefault('APIP_AIW_DEFAULT_ORG_REGION', 'us');

// Whether the BFF has a billing upstream, proxied same-origin at <base>/proxy/billing.
// Set only where one is configured, which is what keeps a standalone deployment from
// calling a route that does not exist. The billing URL itself never reaches the browser.
export const BILLING_PROXY_ENABLED = getEnvOrDefault('APIP_AIW_BILLING_PROXY_ENABLED', false);


// Auth mode: 'basic' (default) posts credentials to /api/portal/v0.9/auth/login; 'oidc' uses react-oidc-context.
export const AUTH_MODE = getEnvOrDefault('APIP_AIW_AUTH_MODE', 'basic') as 'oidc' | 'basic';

// JWT claim name mappings — configure to match your IDP's token structure. The names
// mirror the Platform API's [auth.claim_mappings] key for key, and must agree with
// them: both sides read the same claims out of the same token. Shared by both auth
// modes on the BFF side, hence APIP_AIW_AUTH_CLAIM_MAPPINGS_* (not _OIDC_) names.
export const ORG_ID_CLAIM     = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_ORGANIZATION', 'organization');
export const ORG_NAME_CLAIM   = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_ORG_NAME',     'org_name');
export const ORG_HANDLE_CLAIM = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_ORG_HANDLE',   'org_handle');
export const ORG_IDS_CLAIM = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_ORGANIZATIONS', 'organizations');
// JWT claim names for user display — configure to match your IDP's token structure.
// The defaults mirror the BFF's [auth.claim_mappings] defaults, so both sides read
// the same claim when the key is left unset.
// Common alternatives: 'name', 'given_name', 'preferred_username' (Keycloak), 'upn' (Azure AD)
export const USERNAME_CLAIM = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_USERNAME', 'username');
export const EMAIL_CLAIM = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_EMAIL', 'email');
// Absolute URL to the user's avatar. Standard OIDC name; configurable for an IDP
// that releases it under another claim. Empty/absent renders initials instead.
export const PICTURE_CLAIM = getEnvOrDefault('APIP_AIW_AUTH_CLAIM_MAPPINGS_PICTURE', 'picture');

//Static OIDC configuration — set these to match the root-org OIDC app in your IDP.
// Authority is the issuer URL; the OIDC client will auto-discover endpoints from {authority}/.well-known/openid-configuration.
export const OIDC_AUTHORITY  = getEnvOrDefault('APIP_AIW_OIDC_AUTHORITY', '');
export const OIDC_CLIENT_ID  = getEnvOrDefault('APIP_AIW_OIDC_CLIENT_ID', '');

// Scopes to request at login — derived from openapi.yaml x-required-scopes (ap: prefix).
// offline_access is required so the IDP issues a refresh token; without it the
// BFF cannot silently renew the access token and the user is logged out as soon
// as it expires. Keep it if you override this scope list.
export const OIDC_SCOPE = getEnvOrDefault(
  'APIP_AIW_AUTH_OIDC_SCOPE',
  'openid profile email offline_access' +
  ' ap:api_key:read' +
  ' ap:organization:read ap:organization:manage' +
  ' ap:project:read ap:project:create ap:project:update ap:project:delete ap:project:manage' +
  ' ap:application:read ap:application:create ap:application:update ap:application:delete ap:application:manage' +
  ' ap:application:api_key:read ap:application:api_key:create ap:application:api_key:delete ap:application:api_key:manage' +
  ' ap:application:association:read ap:application:association:create ap:application:association:delete ap:application:association:manage ap:application:association:api_key:read' +
  ' ap:gateway:read ap:gateway:create ap:gateway:update ap:gateway:delete ap:gateway:manage' +
  ' ap:gateway:token:read ap:gateway:token:create ap:gateway:token:delete ap:gateway:token:manage' +
  ' ap:gateway_custom_policy:read ap:gateway_custom_policy:create ap:gateway_custom_policy:delete ap:gateway_custom_policy:manage' +
  ' ap:gateway:artifact:read ap:gateway:manifest:read' +
  ' ap:llm_template:read ap:llm_template:create ap:llm_template:update ap:llm_template:delete ap:llm_template:manage' +
  ' ap:llm_provider:read ap:llm_provider:create ap:llm_provider:update ap:llm_provider:delete ap:llm_provider:manage' +
  ' ap:llm_provider:api_key:read ap:llm_provider:api_key:create ap:llm_provider:api_key:delete ap:llm_provider:api_key:manage' +
  ' ap:llm_provider:deployment:read ap:llm_provider:deployment:create ap:llm_provider:deployment:delete ap:llm_provider:deployment:manage ap:llm_provider:deployment:undeploy ap:llm_provider:deployment:restore' +
  ' ap:llm_proxy:read ap:llm_proxy:create ap:llm_proxy:update ap:llm_proxy:delete ap:llm_proxy:manage' +
  ' ap:llm_proxy:api_key:read ap:llm_proxy:api_key:create ap:llm_proxy:api_key:delete ap:llm_proxy:api_key:manage' +
  ' ap:llm_proxy:deployment:read ap:llm_proxy:deployment:create ap:llm_proxy:deployment:delete ap:llm_proxy:deployment:manage ap:llm_proxy:deployment:undeploy ap:llm_proxy:deployment:restore' +
  ' ap:mcp_proxy:read ap:mcp_proxy:create ap:mcp_proxy:update ap:mcp_proxy:delete ap:mcp_proxy:manage' +
  ' ap:mcp_proxy:deployment:read ap:mcp_proxy:deployment:create ap:mcp_proxy:deployment:delete ap:mcp_proxy:deployment:manage ap:mcp_proxy:deployment:undeploy ap:mcp_proxy:deployment:restore' +
  ' ap:agent_proxy:read ap:agent_proxy:create ap:agent_proxy:update ap:agent_proxy:delete ap:agent_proxy:manage' +
  ' ap:agent_proxy:api_key:read ap:agent_proxy:api_key:create ap:agent_proxy:api_key:update ap:agent_proxy:api_key:delete ap:agent_proxy:api_key:manage' +
  ' ap:agent_proxy:deployment:read ap:agent_proxy:deployment:create ap:agent_proxy:deployment:delete ap:agent_proxy:deployment:manage ap:agent_proxy:deployment:undeploy ap:agent_proxy:deployment:restore' +
  ' ap:secret:read ap:secret:create ap:secret:update ap:secret:delete ap:secret:manage' +
  // Permission-tree names (platform-api/resources/permission-tree.yaml) for the same resources.
  ' ap:api:read ap:api:manage ap:api:graphql:read ap:api:graphql:manage ap:api:graphql:build:read ap:api:graphql:build:manage ap:api:graphql:deployment:read ap:api:graphql:deployment:manage ap:api:llm:read ap:api:llm:manage ap:api:llm:build:read ap:api:llm:build:manage ap:api:llm:deployment:read ap:api:llm:deployment:manage ap:api:llm:api_key:read ap:api:mcp:read ap:api:mcp:manage ap:api:mcp:build:read ap:api:mcp:build:manage ap:api:mcp:deployment:read ap:api:mcp:deployment:manage ap:api:agent:read ap:api:agent:manage ap:api:agent:build:read ap:api:agent:build:manage ap:api:agent:deployment:read ap:api:agent:deployment:manage ap:api:agent:api_key:read' +
  ' ap:llm_provider:template:read ap:llm_provider:template:manage ap:llm_provider:build:read ap:llm_provider:build:manage' +
  ' ap:gateway_policy:read ap:gateway_policy:manage' +
  ' ap:portal:read ap:portal:manage ap:portal:draft:read ap:portal:draft:manage ap:portal:publication:read' +
  ' ap:credential:secret:read ap:credential:secret:manage ap:credential:api_key:read'
);

// OIDC redirect URIs — app-specific, not IDP-specific. Defaults are derived from
// window.location.origin rather than a configured domain/scheme: the browser always
// knows its own actual origin, so this can never drift out of sync with whichever
// listener/domain it's actually being served from (unlike a static config value).
export const OIDC_REDIRECT_URI = getEnvOrDefault(
  'APIP_AIW_OIDC_REDIRECT_URI',
  `${window.location.origin}${BASE_PATH}/signin`
);
export const OIDC_POST_LOGOUT_REDIRECT_URI = getEnvOrDefault(
  'APIP_AIW_OIDC_POST_LOGOUT_REDIRECT_URI',
  `${window.location.origin}${BASE_PATH}/login`
);

// API Base URLs
/**
 * Address of the API Portal this deployment publishes to.
 *
 * Used as the fallback for "View in API Portal" when the organization's own API
 * Portal registration carries no `url` — see `resolveApiPortalUrl`. Empty means
 * no fallback, and a deployment with neither shows no link.
 *
 * Replaces `APIP_AIW_DEV_PORTAL_BASE_URL`, which named the same portal in the
 * older spelling; it had no consumer left when this was renamed.
 */
export const API_PORTAL_URL = getEnvOrDefault('APIP_AIW_API_PORTAL_URL', '');

// Whether MCP proxy publish/unpublish to the API Portal (MCP Hub) is enabled.
// See configs/config-template.toml.
export const API_PORTAL_ENABLED = getEnvOrDefault(
  'APIP_AIW_API_PORTAL_ENABLED',
  false
);

// Mirrors [ai_workspace.feature_flags]. A feature is hidden completely until its
// flag is set: no navigation entry, no route, no overview entry. One entry per
// feature, kept only while that feature is rolling out.
const FEATURE_FLAGS = {
  'agent-proxy': getEnvOrDefault(
    'APIP_AIW_FEATURE_FLAGS_AGENT_PROXY_ENABLED',
    false
  ),
};

export type FeatureId = keyof typeof FEATURE_FLAGS;

export function isFeatureEnabled(feature: FeatureId): boolean {
  return FEATURE_FLAGS[feature];
}

// The single API Portal MCP proxies publish to. Hardcoded stand-in until
// proper Dev Portal selection support is added — not yet operator-configurable.
export const DEFAULT_API_PORTAL_ID = 'default';


export const API_BASE_URLS = {
  policyHubApi: getEnvOrDefault(
    'APIP_AIW_API_POLICY_HUB',
    'https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0'
  ),
} as const;

// Moesif web console base URL
export const MOESIF_WEB_URL = getEnvOrDefault(
  'APIP_AIW_MOESIF_WEB_URL',
  'https://web-dev.moesif.com'
);

// Moesif Application API Key for event tracking
export const MOESIF_APP_API_KEY = getEnvOrDefault(
  'APIP_AIW_MOESIF_APP_API_KEY',
  ''
);

export interface GatewayVersionEntry {
  version: string;
  latestVersion?: string;
  /**
   * Version segment of the distribution zip name, when the published asset name
   * differs from the release tag (e.g. tag `v1.2.0` shipping
   * `wso2apip-ai-gateway-1.2.0-rc.zip`). Defaults to `latestVersion` without
   * its leading `v`.
   */
  distVersion?: string;
  /**
   * Version segment of the directory the zip unpacks into, when it differs from
   * the zip name itself (e.g. `wso2apip-ai-gateway-1.2.0-rc.zip` extracting to
   * `wso2apip-ai-gateway-1.2.0/`). Defaults to `distVersion`.
   */
  distFolderVersion?: string;
  channel: 'STS' | 'LTS';
}

export const PLATFORM_GATEWAY_VERSIONS = getEnvOrDefault<GatewayVersionEntry[]>(
  'APIP_AIW_GATEWAY_PLATFORM_GATEWAY_VERSIONS',
  [
    {
      version: '1.2',
      latestVersion: 'v1.2.0',
      distVersion: '1.2.0',
      distFolderVersion: '1.2.0',
      channel: 'LTS',
    }
  ]
);


// Policy Hub web URL
export const POLICY_HUB_WEB_URL = getEnvOrDefault(
  'APIP_AIW_POLICY_HUB_WEB_URL',
  'https://wso2.com/api-platform/policy-hub/'
);


// Control-plane host shown in gateway setup instructions (host:port).
// Distinct from the BFF proxy base URLs in ./paths — this is a host:port an
// externally deployed gateway dials, not a path this browser calls.
/**
 * What `CONTROLPLANE_HOST` is when the deployment does not set it: the address a gateway
 * container on the same machine uses to reach a locally running control plane. A gateway
 * anywhere else cannot use it.
 */
export const DEFAULT_CONTROLPLANE_HOST = 'host.docker.internal:9243';

export const CONTROLPLANE_HOST = getEnvOrDefault(
  'APIP_AIW_GATEWAY_CONTROLPLANE_HOST',
  DEFAULT_CONTROLPLANE_HOST
);


// CSRF header sent on all BFF requests. Cross-site attackers cannot set a custom
// header (CORS is closed), so its presence proves the request is same-origin.
// Fixed, not configurable: it is a contract between the BFF and this SPA, not a
// deployment concern. Must match the BFF's config.CSRFHeaderName constant.
export const CSRF_HEADER = 'X-Requested-By';
export const CSRF_VALUE = 'ai-workspace';
