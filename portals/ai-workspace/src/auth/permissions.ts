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

/** All platform API OAuth2 scopes derived from openapi.yaml x-required-scopes (ap: prefix). */
export const SCOPES = {
  // Organization
  ORGANIZATION_READ:              'ap:organization:read',
  ORGANIZATION_MANAGE:            'ap:organization:manage',
  ORGANIZATION_SUBSCRIPTION_READ: 'ap:organization:subscription:read',

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
  REST_API_IMPORT:            'ap:rest_api:import',
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

  // DevPortals
  DEVPORTAL_READ:   'ap:devportal:read',
  DEVPORTAL_CREATE: 'ap:devportal:create',
  DEVPORTAL_UPDATE: 'ap:devportal:update',
  DEVPORTAL_DELETE: 'ap:devportal:delete',
  DEVPORTAL_MANAGE: 'ap:devportal:manage',

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

  // Git
  GIT_READ: 'ap:git:read',
} as const;

/**
 * Check whether a set of scopes grants a requested scope.
 *
 * Rules:
 *  1. Exact match — the scope is directly present.
 *  2. Parent :manage — `ap:<resource>:manage` covers all CRUD and sub-resource
 *     scopes under that resource (e.g. ap:gateway:manage covers ap:gateway:token:read).
 */
export function checkPermission(userScopes: string[], scope: string): boolean {
  if (userScopes.includes(scope)) return true;
  const parts = scope.split(':');
  if (parts.length >= 3) {
    const parentManage = `${parts[0]}:${parts[1]}:manage`;
    if (parentManage !== scope && userScopes.includes(parentManage)) return true;
  }
  return false;
}
