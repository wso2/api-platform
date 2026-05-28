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

// Asgardeo configuration
export const ASGARDEO_ORG_DOT = getEnvOrDefault(
  'VITE_ASGARDEO_ORG_DOT',
  'dev.'
);
export const ASGARDEO_URI = getEnvOrDefault('VITE_ASGARDEO_URI', 'asgardeo.io');
export const ASGARDEO_CLIENT_ID = getEnvOrDefault(
  'VITE_ASGARDEO_CLIENT_ID',
  'GtPvf9AfAd6v_DGU3WLADxV0_4Aa'
);

// Federated IDP identifiers
export const FIDP_GOOGLE = getEnvOrDefault('VITE_FIDP_GOOGLE', 'google');
export const FIDP_GITHUB = getEnvOrDefault('VITE_FIDP_GITHUB', 'github');
export const FIDP_MICROSOFT = getEnvOrDefault(
  'VITE_FIDP_MICROSOFT',
  'microsoft'
);
export const FIDP_ANONYMOUS = getEnvOrDefault(
  'VITE_FIDP_ANONYMOUS',
  'choreoanonymous'
);
export const FIDP_ENTERPRISE = getEnvOrDefault(
  'VITE_FIDP_ENTERPRISE',
  'EnterpriseIDP'
);
export const FIDP_E2E = getEnvOrDefault('VITE_FIDP_E2E', 'choreoe2etest');

// Scopes
export const ASGARDEO_SDK_SCOPES = getEnvOrDefault(
  'VITE_ASGARDEO_SDK_SCOPES',
  'email|profile|groups'
);

// Asgardeo SDK Config object
export const asgardeoSdkConfig = {
  endpoints: {
    authorizationEndpoint: getEnvOrDefault(
      'VITE_ASGARDEO_AUTH_ENDPOINT',
      `https://${ASGARDEO_ORG_DOT}api.${ASGARDEO_URI}/t/a/oauth2/authorize`
    ),
    tokenEndpoint: getEnvOrDefault(
      'VITE_ASGARDEO_TOKEN_ENDPOINT',
      `https://${ASGARDEO_ORG_DOT}api.${ASGARDEO_URI}/t/a/oauth2/token`
    ),
    endSessionEndpoint: getEnvOrDefault(
      'VITE_ASGARDEO_END_SESSION_ENDPOINT',
      `https://${ASGARDEO_ORG_DOT}api.${ASGARDEO_URI}/t/a/oidc/logout`
    ),
  },
  overrideWellEndpointConfig: getEnvOrDefault(
    'VITE_ASGARDEO_OVERRIDE_WELL_ENDPOINT',
    true
  ),
  signInRedirectURL: getEnvOrDefault(
    'VITE_ASGARDEO_SIGNIN_REDIRECT_URL',
    `https://${DOMAIN}/signin`
  ),
  signOutRedirectURL: getEnvOrDefault(
    'VITE_ASGARDEO_SIGNOUT_REDIRECT_URL',
    `https://${DOMAIN}/login`
  ),
  clientID: ASGARDEO_CLIENT_ID,
  baseUrl: getEnvOrDefault(
    'VITE_ASGARDEO_BASE_URL',
    `https://${ASGARDEO_ORG_DOT}api.${ASGARDEO_URI}`
  ),
  clientHost: getEnvOrDefault('VITE_ASGARDEO_CLIENT_HOST', `https://${DOMAIN}`),
  enablePKCE: getEnvOrDefault('VITE_ASGARDEO_ENABLE_PKCE', true),
  storage: getEnvOrDefault('VITE_ASGARDEO_STORAGE', 'sessionStorage') as
    | 'sessionStorage'
    | 'localStorage'
    | 'webWorker',
  checkSessionInterval: getEnvOrDefault(
    'VITE_ASGARDEO_CHECK_SESSION_INTERVAL',
    -1
  ),
  disableTrySignInSilently: getEnvOrDefault(
    'VITE_ASGARDEO_DISABLE_TRY_SIGNIN_SILENTLY',
    true
  ),
};

// Asgardeo Resource Server URLs (pipe-separated)
export const ASGARDEO_SDK_RESOURCE_URLS = getEnvOrDefault(
  'VITE_ASGARDEO_SDK_RESOURCE_URLS',
  'https://devportal.preview-dv.bijira.dev|https://sts.preview-dv.choreo.dev|https://km.preview-dv.choreo.dev|https://apim.preview-dv.choreo.dev|https://127.0.0.1:9444/api/am/admin/v2|https://127.0.0.1:9444/api/am/publisher/v2|https://id.dv.choreo.dev|https://consolev2.preview-dv.choreo.dev|https://app.preview-dv.choreo.dev|https://choreocontrolplane.preview-dv.choreo.dev|https://api.dev-central.ballerina.io|https://subscriptions.dv.wso2.com|https://apis.preview-dv.choreo.dev/projects/1.0.0/graphql|https://apis.preview-dv.choreo.dev/moesif-key/0.1.0|https://apis.preview-dv.choreo.dev/org-mgt/1.0.0/orgs|https://apis.preview-dv.choreo.dev/org-mgt/1.0.0|https://apis.preview-dv.choreo.dev/devwfmgt/v1.0|https://apis.preview-dv.choreo.dev/component-mgt/1.0.0|https://apis.preview-dv.choreo.dev/user-mgt/1.0.0|https://apis.preview-dv.choreo.dev/orgs/1.0.0|https://apis.preview-dv.choreo.dev/config-mgt/1.0.0/orgs|https://apis.preview-dv.choreo.dev/alert-configuration-service|https://apis.preview-dv.choreo.dev/devops/1.0.0|https://apis.preview-dv.choreo.dev/cio-query-api/1.0.0/query|https://apis.preview-dv.choreo.dev/cio-incident-configurator/1.0.0|https://apis.preview-dv.choreo.dev/crypto-key-service/0.1.0|https://apis.preview-dv.choreo.dev/custom-domain/1.0.0/orgs|https://apis.preview-dv.choreo.dev/onprem-key-mgt/1.0.0/orgs|https://apis.preview-dv.choreo.dev/audit-logging/1.0.0/orgs|https://apis.preview-dv.choreo.dev/component-utils/1.0.0|https://apis.preview-dv.choreo.dev/marketplace/0.1.0|https://apis.preview-dv.choreo.dev/user-store-mgt/v1.0|https://choreo-shared-choreo-samples-cdne.azureedge.net|https://apis.preview-dv.choreo.dev/platform-services/v1.0|https://apis.preview-dv.choreo.dev/code-challenge-eval/v1|https://apis.preview-dv.choreo.dev/authz-mgt/v1.0|https://apis.preview-dv.choreo.dev/config-svc/v1.0|https://apis.preview-dv.choreo.dev/config-mapping-svc/v1|https://apis.preview-dv.choreo.dev/connections/v1|https://apis.preview-dv.choreo.dev/component-creation|https://apis.preview-dv.choreo.dev/apim-appdev/v1.0|https://apis.preview-dv.choreo.dev/url-mgt/v1.0|https://apis.preview-dv.choreo.dev/declarative-api/v1.0|https://apis.preview-dv.choreo.dev/code-challenge/v1.0|https://apis.preview-dv.choreo.dev/governance/v1.0|https://apis.preview-dv.choreo.dev/choreo-appdev-sts-management-service/v1.0|https://apis.preview-dv.choreo.dev/api-key-service/v1.0|https://apis.preview-dv.choreo.dev/contract-service/1.0.0|https://apis.preview-dv.choreo.dev/oas-provider-service/1.0.0|https://apis.preview-dv.choreo.dev/architect-agent-api-design/v1.0|https://apis.preview-dv.choreo.dev/federation/v1|https://apis.preview-dv.choreo.dev/moesif-key/0.1.0'
);

// STS Token Exchange Configuration
export const STS_CLIENT_ID = getEnvOrDefault(
  'VITE_STS_CLIENT_ID',
  'VGfdOpECCQ8skOxW_39rOjhuMioa'
);
export const STS_TOKEN_ENDPOINT = getEnvOrDefault(
  'VITE_STS_TOKEN_ENDPOINT',
  'https://sts.preview-dv.choreo.dev:443/oauth2/token'
);
export const STS_SCOPE = getEnvOrDefault(
  'VITE_STS_SCOPE',
  'apim:api_create apim:api_manage apim:subscription_manage apim:tier_manage apim:admin apim:publisher_settings environments:view_prod environments:view_dev choreo:user_manage apim:dcr:app_manage choreo:deployment_manage choreo:dev_env_manage choreo:prod_env_manage choreo:non_prod_env_manage choreo:component_manage choreo:project_manage choreo:project_view apim:api_publish apim:document_manage apim:api_settings apim:subscription_view apim:environment_manage choreo:log_view_non_prod choreo:log_view_prod apim:subscribe urn:choreocontrolplane:usermanagement:role_mapping_manage urn:choreocontrolplane:usermanagement:role_mapping_view urn:choreocontrolplane:usermanagement:role_mapping_create urn:choreocontrolplane:usermanagement:role_mapping_update urn:choreocontrolplane:usermanagement:role_mapping_delete urn:choreocontrolplane:configmanagement:global_config_view urn:choreocontrolplane:configmanagement:global_config_create urn:choreocontrolplane:configmanagement:global_config_update urn:choreocontrolplane:configmanagement:global_config_delete urn:choreocontrolplane:configmanagement:global_config_manage urn:choreocontrolplane:configmanagement:config_view urn:choreocontrolplane:configmanagement:config_create urn:choreocontrolplane:configmanagement:config_delete urn:choreocontrolplane:configmanagement:config_manage urn:choreocontrolplane:customdomainapi:custom_domain_view urn:choreocontrolplane:customdomainapi:custom_domain_create urn:choreocontrolplane:customdomainapi:custom_domain_update urn:choreocontrolplane:customdomainapi:custom_domain_delete urn:choreocontrolplane:customdomainapi:custom_domain_manage urn:choreocontrolplane:usermanagement:role_view urn:choreocontrolplane:usermanagement:role_create urn:choreocontrolplane:usermanagement:role_update urn:choreocontrolplane:usermanagement:role_delete urn:choreocontrolplane:usermanagement:role_manage urn:choreocontrolplane:usermanagement:role_manage urn:choreocontrolplane:onpremkeymanagement:on_prem_key_view urn:choreocontrolplane:onpremkeymanagement:on_prem_key_create urn:choreocontrolplane:onpremkeymanagement:on_prem_key_delete urn:choreocontrolplane:onpremkeymanagement:on_prem_key_update urn:choreocontrolplane:onpremkeymanagement:on_prem_key_manage urn:choreocontrolplane:organizationmanagement:theme_view urn:choreocontrolplane:organizationmanagement:theme_create urn:choreocontrolplane:organizationmanagement:theme_delete urn:choreocontrolplane:organizationmanagement:theme_deploy urn:choreocontrolplane:organizationmanagement:theme_manage urn:choreocontrolplane:organizationmanagement:enterprise_login_config_view urn:choreocontrolplane:organizationmanagement:enterprise_login_config_manage urn:choreocontrolplane:organizationmanagement:self_signup_config_view urn:choreocontrolplane:organizationmanagement:self_signup_config_update urn:choreocontrolplane:organizationmanagement:self_signup_manage urn:choreocontrolplane:organizationmanagement:self_signup_approval_view urn:choreocontrolplane:organizationmanagement:self_signup_approval_update urn:choreocontrolplane:usermanagement:user_manage urn:choreocontrolplane:usermanagement:user_view urn:choreocontrolplane:usermanagement:user_delete urn:choreocontrolplane:usermanagement:user_update urn:choreocontrolplane:usermanagement:permission_view urn:choreocontrolplane:usermanagement:invitation_manage urn:choreocontrolplane:usermanagement:invitation_view urn:choreocontrolplane:usermanagement:invitation_send urn:choreocontrolplane:usermanagement:invitation_delete urn:choreocontrolplane:choreodevopsportalapi:deployment_manage urn:choreocontrolplane:choreodevopsportalapi:component_manage urn:choreocontrolplane:componentsmanagement:component_trigger urn:choreocontrolplane:componentsmanagement:component_create urn:choreocontrolplane:componentsmanagement:component_config_view urn:choreocontrolplane:componentsmanagement:component_logs_view urn:choreocontrolplane:componentsmanagement:component_init_view urn:choreocontrolplane:componentsmanagement:component_file_view urn:choreocontrolplane:componentsmanagement:component_manage urn:choreocontrolplane:choreodevopsportalapi:deployment_view apim:api_view apim:tier_view apim:api_generate_key urn:choreocontrolplane:choreodevopsportalapi:deployment_view urn:choreocontrolplane:choreoauditloggingapi:audit_logs_view urn:choreocontrolplane:choreoauditloggingapi:audit_logs_manage urn:choreocontrolplane:componentutils:component_manage urn:choreocontrolplane:componentutils:component_file_view urn:choreocontrolplane:componentutils:component_trigger urn:choreocontrolplane:componentutils:component_logs_view urn:choreocontrolplane:organizationapi:org_manage choreo:domain_manage choreo:domain_view choreo:url_mapping_manage choreo:url_mapping_approve choreo:url_mapping_view choreo:insights_org_view choreo:workflow_component_promotion_approve choreo:workflow_subscription_approve choreo:workflow_url_mapping_approve choreo:workflow_request choreo:platform_engineer  choreo:env_manage choreo:log_view'
);
export const TOKEN_EXCHANGE_CONFIG_ID = getEnvOrDefault(
  'VITE_TOKEN_EXCHANGE_CONFIG_ID',
  'choreo-sts-token-exchange'
);

// Token Exchange Config for Asgardeo SDK's requestCustomGrant
export const tokenExchangeConfig = {
  tokenEndpoint: STS_TOKEN_ENDPOINT,
  attachToken: true,
  data: {
    client_id: STS_CLIENT_ID,
    grant_type: getEnvOrDefault(
      'VITE_TOKEN_EXCHANGE_GRANT_TYPE',
      'urn:ietf:params:oauth:grant-type:token-exchange'
    ),
    subject_token_type: getEnvOrDefault(
      'VITE_TOKEN_EXCHANGE_SUBJECT_TOKEN_TYPE',
      'urn:ietf:params:oauth:token-type:jwt'
    ),
    requested_token_type: getEnvOrDefault(
      'VITE_TOKEN_EXCHANGE_REQUESTED_TOKEN_TYPE',
      'urn:ietf:params:oauth:token-type:jwt'
    ),
    scope: STS_SCOPE,
    subject_token: '{{token}}',
  },
  id: TOKEN_EXCHANGE_CONFIG_ID,
  returnResponse: getEnvOrDefault('VITE_TOKEN_EXCHANGE_RETURN_RESPONSE', true),
  returnsSession: getEnvOrDefault('VITE_TOKEN_EXCHANGE_RETURNS_SESSION', true),
  signInRequired: getEnvOrDefault('VITE_TOKEN_EXCHANGE_SIGNIN_REQUIRED', true),
  shouldReplayAfterRefresh: getEnvOrDefault(
    'VITE_TOKEN_EXCHANGE_REPLAY_AFTER_REFRESH',
    true
  ),
};

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
