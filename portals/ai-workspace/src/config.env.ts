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

/*
 * Single line environment variable definitions with defaults using getEnvOrDefault utility to improve readability and maintainability.
 */

// Debug mode
export const DEBUG = getEnvOrDefault('VITE_DEBUG', false);

// Sentry environment (used for CookiePro and other env-specific features)
export const SENTRY_ENV = getEnvOrDefault('VITE_SENTRY_ENV', 'DEV');

// Domain and environment settings
export const DOMAIN = getEnvOrDefault('VITE_DOMAIN', 'localhost:3009');
export const CHOREO_SYSTEM_ORG = getEnvOrDefault(
  'VITE_CHOREO_SYSTEM_ORG',
  'choreocontrolplane'
);

// The org handle this workspace deployment serves. Used to fetch OIDC config
// from the platform API's unauthenticated discovery endpoint before login.
export const ORG_HANDLE = getEnvOrDefault('VITE_ORG_HANDLE', '');

// Scopes to request at login. Authority, client_id, and logout URL are fetched
// dynamically from GET /portal/api/v1/organizations/{handle}/auth at startup.
export const OIDC_SCOPE = getEnvOrDefault(
  'VITE_OIDC_SCOPE',
  'openid profile email api-platform:gateway:manage api-platform:gateway:create api-platform:gateway:read api-platform:gateway:update api-platform:gateway:delete api-platform:gateway:token:manage api-platform:gateway:token:read api-platform:gateway:token:create api-platform:gateway:token:delete api-platform:gateway:policy:manage api-platform:gateway:policy:read api-platform:gateway:policy:create api-platform:gateway:policy:delete api-platform:gateway:artifacts:read api-platform:gateway:manifest:read api-platform:gateway:status:read api-platform:rest_api:manage api-platform:rest_api:create api-platform:rest_api:read api-platform:rest_api:update api-platform:rest_api:delete api-platform:rest_api:publish api-platform:rest_api:import api-platform:rest_api:gateway:manage api-platform:rest_api:gateway:create api-platform:rest_api:gateway:read api-platform:rest_api:deployment:manage api-platform:rest_api:deployment:create api-platform:rest_api:deployment:read api-platform:rest_api:deployment:delete api-platform:rest_api:deployment:undeploy api-platform:rest_api:deployment:restore api-platform:rest_api:api_key:manage api-platform:rest_api:api_key:create api-platform:rest_api:api_key:read api-platform:rest_api:api_key:update api-platform:rest_api:api_key:delete api-platform:project:manage api-platform:project:create api-platform:project:read api-platform:project:update api-platform:project:delete api-platform:application:manage api-platform:application:create api-platform:application:read api-platform:application:update api-platform:application:delete api-platform:application:api_key:manage api-platform:application:api_key:create api-platform:application:api_key:read api-platform:application:api_key:delete api-platform:application:associations:manage api-platform:application:associations:create api-platform:application:associations:read api-platform:application:associations:delete api-platform:application:associations:api_key:read api-platform:devportal:manage api-platform:devportal:create api-platform:devportal:read api-platform:devportal:update api-platform:devportal:delete api-platform:subscription:manage api-platform:subscription:create api-platform:subscription:read api-platform:subscription:update api-platform:subscription:delete api-platform:subscription_plan:manage api-platform:subscription_plan:create api-platform:subscription_plan:read api-platform:subscription_plan:update api-platform:subscription_plan:delete api-platform:llm_template:manage api-platform:llm_template:create api-platform:llm_template:read api-platform:llm_template:update api-platform:llm_template:delete api-platform:llm_provider:manage api-platform:llm_provider:create api-platform:llm_provider:read api-platform:llm_provider:update api-platform:llm_provider:delete api-platform:llm_provider:deployment:manage api-platform:llm_provider:key:manage api-platform:llm_proxy:manage api-platform:llm_proxy:create api-platform:llm_proxy:read api-platform:llm_proxy:update api-platform:llm_proxy:delete api-platform:llm_proxy:deployment:manage api-platform:llm_proxy:key:manage api-platform:mcp_proxy:manage api-platform:mcp_proxy:create api-platform:mcp_proxy:read api-platform:mcp_proxy:update api-platform:mcp_proxy:delete api-platform:mcp_proxy:deployment:manage api-platform:websub_api:manage api-platform:websub_api:create api-platform:websub_api:read api-platform:websub_api:update api-platform:websub_api:delete api-platform:websub_api:deployment:manage api-platform:websub_api:publish api-platform:websub_api:key:manage api-platform:webbroker_api:manage api-platform:webbroker_api:create api-platform:webbroker_api:read api-platform:webbroker_api:update api-platform:webbroker_api:delete api-platform:webbroker_api:deployment:manage api-platform:webbroker_api:publish api-platform:webbroker_api:key:manage api-platform:git:read'
);

// OIDC redirect URIs — app-specific, not IDP-specific.
export const OIDC_REDIRECT_URI = getEnvOrDefault(
  'VITE_OIDC_REDIRECT_URI',
  `https://${DOMAIN}/signin`
);
export const OIDC_POST_LOGOUT_REDIRECT_URI = getEnvOrDefault(
  'VITE_OIDC_POST_LOGOUT_REDIRECT_URI',
  `https://${DOMAIN}/login`
);

// API Base URLs
export const DEV_PORTAL_BASE_URL = getEnvOrDefault(
  'VITE_DEV_PORTAL_BASE_URL',
  'https://devportal.preview-dv.bijira.dev'
);

export const API_BASE_URLS = {
  projectApi: getEnvOrDefault(
    'VITE_API_PROJECT_API',
    'https://apis.preview-dv.choreo.dev/projects/1.0.0/graphql'
  ),
  orgManagement: getEnvOrDefault(
    'VITE_API_ORG_MANAGEMENT',
    'https://apis.preview-dv.choreo.dev/org-mgt/1.0.0'
  ),
  organizationApi: getEnvOrDefault(
    'VITE_API_ORGANIZATION_API',
    'https://apis.preview-dv.choreo.dev/orgs/1.0.0'
  ),
  componentManagement: getEnvOrDefault(
    'VITE_API_COMPONENT_MANAGEMENT',
    'https://apis.preview-dv.choreo.dev/component-mgt/1.0.0'
  ),
  userManagement: getEnvOrDefault(
    'VITE_API_USER_MANAGEMENT',
    'https://apis.preview-dv.choreo.dev/user-mgt/1.0.0'
  ),
  devOps: getEnvOrDefault(
    'VITE_API_DEVOPS',
    'https://apis.preview-dv.choreo.dev/devops/1.0.0'
  ),
  adminApi: getEnvOrDefault(
    'VITE_API_ADMIN_API',
    'https://sts.preview-dv.choreo.dev/api/am/admin/v2'
  ),
  publisherApi: getEnvOrDefault(
    'VITE_API_PUBLISHER_API',
    'https://sts.preview-dv.choreo.dev/api/am/publisher/v2'
  ),
  policyHubApi: getEnvOrDefault(
    'VITE_API_POLICY_HUB',
    'https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0'
  ),
  moesifAPI: getEnvOrDefault(
    'VITE_API_MOESIF_API',
    'https://apis.preview-dv.choreo.dev/moesif-key/0.1.0'
  ),
} as const;

// Moesif web console base URL
export const MOESIF_WEB_URL = getEnvOrDefault(
  'VITE_MOESIF_WEB_URL',
  'https://web-dev.moesif.com'
);

// Moesif Application API Key for event tracking
export const MOESIF_APP_API_KEY = getEnvOrDefault(
  'VITE_MOESIF_APP_API_KEY',
  'eyJhcHAiOiI5Mjo1NjYiLCJ2ZXIiOiIyLjEiLCJvcmciOiI2Mjg6NDE3IiwicHViIjp0cnVlLCJpYXQiOjE3Njk5MDQwMDB9.gxcZJ7eybasZ5JY_JJj2ARuTiWZNnYIeAtL8oQbhfxk'
);

// ToS Service name for user validation
export const TOS_SERVICE_NAME = getEnvOrDefault(
  'VITE_TOS_SERVICE_NAME',
  'bijira'
);

// Platform Gateway Version
export const PLATFORM_GATEWAY_VERSION = getEnvOrDefault(
  'VITE_PLATFORM_GATEWAY_VERSION',
  'v1.0.0'
);

// Platform Control Plane URL for gateway configuration
export const PLATFORM_CONTROL_PLANE_URL = getEnvOrDefault(
  'VITE_PLATFORM_CONTROL_PLANE_URL',
  'https://connect.preview-dv.bijira.dev'
);

// Policy Hub web URL
export const POLICY_HUB_WEB_URL = getEnvOrDefault(
  'VITE_POLICY_HUB_WEB_URL',
  'https://wso2.com/api-platform/policy-hub/'
);

// Platform API base URL (local dev: https://localhost:9243/api/v1)
export const PLATFORM_API_BASE_URL = getEnvOrDefault(
  'VITE_PLATFORM_API_BASE_URL',
  'https://localhost:9243/api/v1'
);

// JWT claim names for user display — configure to match your IDP's token structure.
// Common alternatives: 'name', 'preferred_username' (Keycloak), 'upn' (Azure AD)
export const OIDC_USERNAME_CLAIM = getEnvOrDefault('VITE_OIDC_USERNAME_CLAIM', 'given_name');
export const OIDC_EMAIL_CLAIM = getEnvOrDefault('VITE_OIDC_EMAIL_CLAIM', 'email');

// Super admin credentials — stored as username + bcrypt password hash.
// Generate a hash with: node -e "const b=require('bcryptjs');b.hash('yourpassword',12).then(console.log)"
// Set VITE_SUPER_ADMIN_PASSWORD_HASH to a non-empty value to enable super admin login.
export const SUPER_ADMIN_USERNAME = getEnvOrDefault('VITE_SUPER_ADMIN_USERNAME', 'admin');
export const SUPER_ADMIN_PASSWORD_HASH = getEnvOrDefault('VITE_SUPER_ADMIN_PASSWORD_HASH', '');

