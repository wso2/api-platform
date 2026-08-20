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

export type PlatformRole = 'admin' | 'developer' | 'publisher' | 'operator' | 'viewer';

export const PLATFORM_ROLES: readonly PlatformRole[] = ['admin', 'developer', 'publisher', 'operator', 'viewer'];

export function isPlatformRole(value: unknown): value is PlatformRole {
  return typeof value === 'string' && (PLATFORM_ROLES as readonly string[]).includes(value);
}

/**
 * Tooltip shown on a control disabled purely because the signed-in user lacks
 * the required scope. Deliberately does not name the missing scope — that would
 * let a caller probe the authorization model.
 */
export const NO_PERMISSION_TOOLTIP =
  'You do not have permission to perform this action. Please contact your admin.';

/**
 * `sx` for a permission-disabled action that renders as a link
 * (`component={RouterLink}`), i.e. an `<a>` rather than a `<button>`.
 *
 * A disabled `<a>` still blocks the click, but it does not pick up the
 * theme's disabled colouring the way a native disabled `<button>` does, so the
 * control looks fully enabled while doing nothing — which reads as a broken
 * button rather than a permission boundary. Dimming it makes the disabled state
 * visible. Not needed for plain `<Button onClick=...>`.
 */
export const DISABLED_ACTION_SX = {
  '&.Mui-disabled': { opacity: 0.55 },
} as const;

/** All platform API OAuth2 scopes derived from openapi.yaml x-required-scopes (ap: prefix). */
export const SCOPES = {
  // Organization
  ORGANIZATION_READ:              'ap:organization:read',
  ORGANIZATION_MANAGE:            'ap:organization:manage',

  // Projects
  PROJECT_READ:   'ap:project:read',
  PROJECT_CREATE: 'ap:project:create',
  PROJECT_UPDATE: 'ap:project:update',
  PROJECT_DELETE: 'ap:project:delete',
  PROJECT_MANAGE: 'ap:project:manage',

  // Applications
  APPLICATION_READ:                    'ap:application:read',
  APPLICATION_CREATE:                  'ap:application:create',
  APPLICATION_UPDATE:                  'ap:application:update',
  APPLICATION_DELETE:                  'ap:application:delete',
  APPLICATION_MANAGE:                  'ap:application:manage',
  APPLICATION_API_KEY_READ:            'ap:application:api_key:read',
  APPLICATION_API_KEY_CREATE:          'ap:application:api_key:create',
  APPLICATION_API_KEY_DELETE:          'ap:application:api_key:delete',
  APPLICATION_API_KEY_MANAGE:          'ap:application:api_key:manage',
  APPLICATION_ASSOCIATIONS_READ:       'ap:application:association:read',
  APPLICATION_ASSOCIATIONS_CREATE:     'ap:application:association:create',
  APPLICATION_ASSOCIATIONS_DELETE:     'ap:application:association:delete',
  APPLICATION_ASSOCIATIONS_MANAGE:     'ap:application:association:manage',
  APPLICATION_ASSOCIATIONS_API_KEY_READ: 'ap:application:association:api_key:read',

  // AI Gateways
  GATEWAY_READ:           'ap:gateway:read',
  GATEWAY_CREATE:         'ap:gateway:create',
  GATEWAY_UPDATE:         'ap:gateway:update',
  GATEWAY_DELETE:         'ap:gateway:delete',
  GATEWAY_MANAGE:         'ap:gateway:manage',
  GATEWAY_TOKEN_READ:     'ap:gateway:token:read',
  GATEWAY_TOKEN_CREATE:   'ap:gateway:token:create',
  GATEWAY_TOKEN_DELETE:   'ap:gateway:token:delete',
  GATEWAY_TOKEN_MANAGE:   'ap:gateway:token:manage',
  GATEWAY_CUSTOM_POLICY_READ:    'ap:gateway_custom_policy:read',
  GATEWAY_CUSTOM_POLICY_CREATE:  'ap:gateway_custom_policy:create',
  GATEWAY_CUSTOM_POLICY_DELETE:  'ap:gateway_custom_policy:delete',
  GATEWAY_CUSTOM_POLICY_MANAGE:  'ap:gateway_custom_policy:manage',
  GATEWAY_ARTIFACTS_READ: 'ap:gateway:artifact:read',
  GATEWAY_MANIFEST_READ:  'ap:gateway:manifest:read',

  // REST APIs
  REST_API_READ:              'ap:rest_api:read',
  REST_API_CREATE:            'ap:rest_api:create',
  REST_API_UPDATE:            'ap:rest_api:update',
  REST_API_DELETE:            'ap:rest_api:delete',
  REST_API_MANAGE:            'ap:rest_api:manage',
  REST_API_GATEWAY_READ:      'ap:rest_api:gateway:read',
  REST_API_GATEWAY_CREATE:    'ap:rest_api:gateway:create',
  REST_API_GATEWAY_MANAGE:    'ap:rest_api:gateway:manage',
  REST_API_DEPLOYMENT_READ:   'ap:rest_api:deployment:read',
  REST_API_DEPLOYMENT_CREATE: 'ap:rest_api:deployment:create',
  REST_API_DEPLOYMENT_DELETE: 'ap:rest_api:deployment:delete',
  REST_API_DEPLOYMENT_MANAGE: 'ap:rest_api:deployment:manage',
  REST_API_API_KEY_READ:      'ap:rest_api:api_key:read',
  REST_API_API_KEY_CREATE:    'ap:rest_api:api_key:create',
  REST_API_API_KEY_UPDATE:    'ap:rest_api:api_key:update',
  REST_API_API_KEY_DELETE:    'ap:rest_api:api_key:delete',
  REST_API_API_KEY_MANAGE:    'ap:rest_api:api_key:manage',
  REST_API_PUBLICATION_READ:  'ap:rest_api:publication:read',

  // Subscriptions
  SUBSCRIPTION_READ:        'ap:subscription:read',
  SUBSCRIPTION_CREATE:      'ap:subscription:create',
  SUBSCRIPTION_UPDATE:      'ap:subscription:update',
  SUBSCRIPTION_DELETE:      'ap:subscription:delete',
  SUBSCRIPTION_MANAGE:      'ap:subscription:manage',
  SUBSCRIPTION_PLAN_READ:   'ap:subscription_plan:read',
  SUBSCRIPTION_PLAN_CREATE: 'ap:subscription_plan:create',
  SUBSCRIPTION_PLAN_UPDATE: 'ap:subscription_plan:update',
  SUBSCRIPTION_PLAN_DELETE: 'ap:subscription_plan:delete',
  SUBSCRIPTION_PLAN_MANAGE: 'ap:subscription_plan:manage',

  // LLM Templates
  LLM_TEMPLATE_READ:   'ap:llm_template:read',
  LLM_TEMPLATE_CREATE: 'ap:llm_template:create',
  LLM_TEMPLATE_UPDATE: 'ap:llm_template:update',
  LLM_TEMPLATE_DELETE: 'ap:llm_template:delete',
  LLM_TEMPLATE_MANAGE: 'ap:llm_template:manage',

  // LLM Providers
  LLM_PROVIDER_READ:              'ap:llm_provider:read',
  LLM_PROVIDER_CREATE:            'ap:llm_provider:create',
  LLM_PROVIDER_UPDATE:            'ap:llm_provider:update',
  LLM_PROVIDER_DELETE:            'ap:llm_provider:delete',
  LLM_PROVIDER_MANAGE:            'ap:llm_provider:manage',
  LLM_PROVIDER_API_KEY_READ:      'ap:llm_provider:api_key:read',
  LLM_PROVIDER_API_KEY_CREATE:    'ap:llm_provider:api_key:create',
  LLM_PROVIDER_API_KEY_DELETE:    'ap:llm_provider:api_key:delete',
  LLM_PROVIDER_API_KEY_MANAGE:    'ap:llm_provider:api_key:manage',
  LLM_PROVIDER_DEPLOYMENT_READ:   'ap:llm_provider:deployment:read',
  LLM_PROVIDER_DEPLOYMENT_CREATE: 'ap:llm_provider:deployment:create',
  LLM_PROVIDER_DEPLOYMENT_DELETE: 'ap:llm_provider:deployment:delete',
  LLM_PROVIDER_DEPLOYMENT_MANAGE: 'ap:llm_provider:deployment:manage',

  // LLM Proxies
  LLM_PROXY_READ:              'ap:llm_proxy:read',
  LLM_PROXY_CREATE:            'ap:llm_proxy:create',
  LLM_PROXY_UPDATE:            'ap:llm_proxy:update',
  LLM_PROXY_DELETE:            'ap:llm_proxy:delete',
  LLM_PROXY_MANAGE:            'ap:llm_proxy:manage',
  LLM_PROXY_API_KEY_READ:      'ap:llm_proxy:api_key:read',
  LLM_PROXY_API_KEY_CREATE:    'ap:llm_proxy:api_key:create',
  LLM_PROXY_API_KEY_DELETE:    'ap:llm_proxy:api_key:delete',
  LLM_PROXY_API_KEY_MANAGE:    'ap:llm_proxy:api_key:manage',
  LLM_PROXY_DEPLOYMENT_READ:   'ap:llm_proxy:deployment:read',
  LLM_PROXY_DEPLOYMENT_CREATE: 'ap:llm_proxy:deployment:create',
  LLM_PROXY_DEPLOYMENT_DELETE: 'ap:llm_proxy:deployment:delete',
  LLM_PROXY_DEPLOYMENT_MANAGE: 'ap:llm_proxy:deployment:manage',

  // MCP Proxies
  MCP_PROXY_READ:              'ap:mcp_proxy:read',
  MCP_PROXY_CREATE:            'ap:mcp_proxy:create',
  MCP_PROXY_UPDATE:            'ap:mcp_proxy:update',
  MCP_PROXY_DELETE:            'ap:mcp_proxy:delete',
  MCP_PROXY_MANAGE:            'ap:mcp_proxy:manage',
  MCP_PROXY_DEPLOYMENT_READ:   'ap:mcp_proxy:deployment:read',
  MCP_PROXY_DEPLOYMENT_CREATE: 'ap:mcp_proxy:deployment:create',
  MCP_PROXY_DEPLOYMENT_DELETE: 'ap:mcp_proxy:deployment:delete',
  MCP_PROXY_DEPLOYMENT_MANAGE: 'ap:mcp_proxy:deployment:manage',

  // WebSub APIs
  WEBSUB_API_READ:              'ap:websub_api:read',
  WEBSUB_API_CREATE:            'ap:websub_api:create',
  WEBSUB_API_UPDATE:            'ap:websub_api:update',
  WEBSUB_API_DELETE:            'ap:websub_api:delete',
  WEBSUB_API_MANAGE:            'ap:websub_api:manage',
  WEBSUB_API_API_KEY_MANAGE:    'ap:websub_api:api_key:manage',
  WEBSUB_API_DEPLOYMENT_MANAGE: 'ap:websub_api:deployment:manage',
  WEBSUB_API_PUBLICATION_READ:  'ap:websub_api:publication:read',

  // WebBroker APIs
  WEBBROKER_API_READ:              'ap:webbroker_api:read',
  WEBBROKER_API_CREATE:            'ap:webbroker_api:create',
  WEBBROKER_API_UPDATE:            'ap:webbroker_api:update',
  WEBBROKER_API_DELETE:            'ap:webbroker_api:delete',
  WEBBROKER_API_MANAGE:            'ap:webbroker_api:manage',
  WEBBROKER_API_API_KEY_MANAGE:    'ap:webbroker_api:api_key:manage',
  WEBBROKER_API_DEPLOYMENT_MANAGE: 'ap:webbroker_api:deployment:manage',
  WEBBROKER_API_PUBLICATION_READ:  'ap:webbroker_api:publication:read',

  // Secrets
  SECRET_READ:   'ap:secret:read',
  SECRET_CREATE: 'ap:secret:create',
  SECRET_UPDATE: 'ap:secret:update',
  SECRET_DELETE: 'ap:secret:delete',
  SECRET_MANAGE: 'ap:secret:manage',
} as const;

/**
 * Deployment scopes per deployable AI artifact kind, keyed by the same resource
 * type `GatewayDeployProvider` takes. Shared by the deploy page/context and the
 * "Deploy to Gateway" entry points on the overview pages, so one map decides
 * both whether the deploy actions render and whether the button leading to them
 * is reachable.
 */
export const DEPLOYMENT_SCOPES = {
  provider: {
    read: SCOPES.LLM_PROVIDER_DEPLOYMENT_READ,
    create: SCOPES.LLM_PROVIDER_DEPLOYMENT_CREATE,
  },
  proxy: {
    read: SCOPES.LLM_PROXY_DEPLOYMENT_READ,
    create: SCOPES.LLM_PROXY_DEPLOYMENT_CREATE,
  },
  'mcp-server': {
    read: SCOPES.MCP_PROXY_DEPLOYMENT_READ,
    create: SCOPES.MCP_PROXY_DEPLOYMENT_CREATE,
  },
} as const;

export type DeployableResourceType = keyof typeof DEPLOYMENT_SCOPES;

/**
 * Scopes that must be held explicitly and are never derived from a broader
 * `:manage`. `ap:api_key:all:manage` is an ownership override (it widens which
 * users' keys are reachable, not which actions), so holding `ap:api_key:manage`
 * must not confer it — matching the service-layer rule in
 * `platform-api/internal/service` and `/me/api-keys` in openapi.yaml, the one
 * operation whose accepted-scope list omits its parent `:manage`.
 */
const NON_DERIVABLE_SCOPE_SUFFIX = ':all:manage';

/**
 * Check whether a set of scopes grants a requested scope.
 *
 * Mirrors how platform-api actually authorizes a request: each operation in
 * openapi.yaml declares a list of accepted scopes, and `ScopeEnforcer`
 * (`platform-api/internal/middleware/authorization.go`) admits the caller if a
 * held scope matches an entry in that list exactly — there is no wildcard form.
 * Rules:
 *
 *  1. Exact match — the scope is directly present.
 *  2. Own-level `:manage` — an operation accepting `<level>:<action>` also
 *     accepts `<level>:manage` (`ap:llm_provider:api_key:read` is satisfied by
 *     `ap:llm_provider:api_key:manage`).
 *  3. Ancestor `:manage` — a resource-level `:manage` also covers its
 *     sub-resources' operations, because those operations list it explicitly.
 *     `ap:llm_provider:manage` appears in the accepted list of every
 *     `/llm-providers/{id}/api-keys` and `/deployments` operation, so it grants
 *     `ap:llm_provider:api_key:create`, `ap:llm_provider:deployment:read`, etc.
 *     This holds for 47 of the 48 sub-resource operations in the spec; the sole
 *     exception is the override scope excluded above.
 */
export function checkPermission(userScopes: string[], scope: string): boolean {
  if (userScopes.includes(scope)) return true;

  const parts = scope.split(':');
  if (parts.length < 2) return false;

  if (scope.endsWith(NON_DERIVABLE_SCOPE_SUFFIX)) return false;

  // Own-level `:manage`, then each broader ancestor's `:manage`. For
  // `ap:llm_provider:api_key:read` that is `ap:llm_provider:api_key:manage`
  // followed by `ap:llm_provider:manage`.
  for (let depth = parts.length - 1; depth >= 2; depth -= 1) {
    const candidate = `${parts.slice(0, depth).join(':')}:manage`;
    if (candidate !== scope && userScopes.includes(candidate)) return true;
  }

  return false;
}
