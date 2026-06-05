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

/** All platform API OAuth2 scopes derived from openapi.yaml x-required-scopes. */
export const SCOPES = {
  // Projects
  PROJECT_READ:   'api-platform:project:read',
  PROJECT_CREATE: 'api-platform:project:create',
  PROJECT_UPDATE: 'api-platform:project:update',
  PROJECT_DELETE: 'api-platform:project:delete',
  PROJECT_MANAGE: 'api-platform:project:manage',

  // Applications
  APPLICATION_READ:              'api-platform:application:read',
  APPLICATION_CREATE:            'api-platform:application:create',
  APPLICATION_UPDATE:            'api-platform:application:update',
  APPLICATION_DELETE:            'api-platform:application:delete',
  APPLICATION_MANAGE:            'api-platform:application:manage',
  APPLICATION_API_KEY_READ:      'api-platform:application:api_key:read',
  APPLICATION_API_KEY_MANAGE:    'api-platform:application:api_key:manage',
  APPLICATION_ASSOCIATIONS_READ:   'api-platform:application:associations:read',
  APPLICATION_ASSOCIATIONS_MANAGE: 'api-platform:application:associations:manage',

  // AI Gateways
  GATEWAY_READ:            'api-platform:gateway:read',
  GATEWAY_CREATE:          'api-platform:gateway:create',
  GATEWAY_UPDATE:          'api-platform:gateway:update',
  GATEWAY_DELETE:          'api-platform:gateway:delete',
  GATEWAY_MANAGE:          'api-platform:gateway:manage',
  GATEWAY_TOKEN_READ:      'api-platform:gateway:token:read',
  GATEWAY_TOKEN_MANAGE:    'api-platform:gateway:token:manage',
  GATEWAY_POLICY_READ:     'api-platform:gateway:policy:read',
  GATEWAY_POLICY_MANAGE:   'api-platform:gateway:policy:manage',
  GATEWAY_ARTIFACTS_READ:  'api-platform:gateway:artifacts:read',
  GATEWAY_MANIFEST_READ:   'api-platform:gateway:manifest:read',
  GATEWAY_STATUS_READ:     'api-platform:gateway:status:read',

  // LLM Providers
  LLM_PROVIDER_READ:              'api-platform:llm_provider:read',
  LLM_PROVIDER_CREATE:            'api-platform:llm_provider:create',
  LLM_PROVIDER_UPDATE:            'api-platform:llm_provider:update',
  LLM_PROVIDER_DELETE:            'api-platform:llm_provider:delete',
  LLM_PROVIDER_MANAGE:            'api-platform:llm_provider:manage',
  LLM_PROVIDER_KEY_MANAGE:        'api-platform:llm_provider:key:manage',
  LLM_PROVIDER_DEPLOYMENT_MANAGE: 'api-platform:llm_provider:deployment:manage',

  // LLM Proxies
  LLM_PROXY_READ:              'api-platform:llm_proxy:read',
  LLM_PROXY_CREATE:            'api-platform:llm_proxy:create',
  LLM_PROXY_UPDATE:            'api-platform:llm_proxy:update',
  LLM_PROXY_DELETE:            'api-platform:llm_proxy:delete',
  LLM_PROXY_MANAGE:            'api-platform:llm_proxy:manage',
  LLM_PROXY_KEY_MANAGE:        'api-platform:llm_proxy:key:manage',
  LLM_PROXY_DEPLOYMENT_MANAGE: 'api-platform:llm_proxy:deployment:manage',

  // LLM Templates
  LLM_TEMPLATE_READ:   'api-platform:llm_template:read',
  LLM_TEMPLATE_MANAGE: 'api-platform:llm_template:manage',

  // MCP Proxies
  MCP_PROXY_READ:              'api-platform:mcp_proxy:read',
  MCP_PROXY_CREATE:            'api-platform:mcp_proxy:create',
  MCP_PROXY_UPDATE:            'api-platform:mcp_proxy:update',
  MCP_PROXY_DELETE:            'api-platform:mcp_proxy:delete',
  MCP_PROXY_MANAGE:            'api-platform:mcp_proxy:manage',
  MCP_PROXY_DEPLOYMENT_MANAGE: 'api-platform:mcp_proxy:deployment:manage',

  // DevPortals
  DEVPORTAL_READ:   'api-platform:devportal:read',
  DEVPORTAL_MANAGE: 'api-platform:devportal:manage',

  // Subscriptions
  SUBSCRIPTION_READ:        'api-platform:subscription:read',
  SUBSCRIPTION_MANAGE:      'api-platform:subscription:manage',
  SUBSCRIPTION_PLAN_READ:   'api-platform:subscription_plan:read',
  SUBSCRIPTION_PLAN_MANAGE: 'api-platform:subscription_plan:manage',

  // REST APIs
  REST_API_READ:              'api-platform:rest_api:read',
  REST_API_CREATE:            'api-platform:rest_api:create',
  REST_API_MANAGE:            'api-platform:rest_api:manage',
  REST_API_PUBLISH:           'api-platform:rest_api:publish',
  REST_API_DEPLOYMENT_MANAGE: 'api-platform:rest_api:deployment:manage',

  // Git
  GIT_READ: 'api-platform:git:read',
} as const;

/**
 * Check whether a set of scopes grants a requested scope.
 *
 * Rules (matching discussion #2045):
 *  1. Exact match — the scope is directly present.
 *  2. Parent :manage — `api-platform:X:manage` is a superset that covers all
 *     CRUD operations and sub-resource scopes under resource X.
 */
export function checkPermission(userScopes: string[], scope: string): boolean {
  if (userScopes.includes(scope)) return true;
  // Derive parent :manage scope: api-platform:<resource>:manage
  const parts = scope.split(':');
  if (parts.length >= 3) {
    const parentManage = `${parts[0]}:${parts[1]}:manage`;
    if (parentManage !== scope && userScopes.includes(parentManage)) return true;
  }
  return false;
}

