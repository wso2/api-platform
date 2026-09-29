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

import { afterEach, describe, expect, it, vi } from 'vitest';

const policyHubUrl =
  'https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0';

const loadRuntimeConfig = async () => {
  vi.resetModules();
  return (await import('./runtime')).runtimeConfig;
};

afterEach(() => {
  vi.unstubAllEnvs();
  delete window.config;
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
});

describe('runtimeConfig', () => {
  it('uses the default Policy Hub in production when overrides are empty', async () => {
    vi.stubEnv('DEV', false);
    vi.stubEnv('MODE', 'production');
    vi.stubEnv('VITE_POLICY_HUB_BASE_URL', '');
    window.__RUNTIME_CONFIG__ = { POLICY_HUB_BASE_URL: '' };

    expect((await loadRuntimeConfig()).policyHubBaseUrl).toBe(policyHubUrl);
  });

  it('lets build configuration override the default Policy Hub', async () => {
    vi.stubEnv('VITE_POLICY_HUB_BASE_URL', `${policyHubUrl}/`);

    expect((await loadRuntimeConfig()).policyHubBaseUrl).toBe(`${policyHubUrl}/`);
  });

  it('prefers runtime Policy Hub configuration over build configuration', async () => {
    vi.stubEnv('VITE_POLICY_HUB_BASE_URL', `${policyHubUrl}/`);
    window.__RUNTIME_CONFIG__ = { POLICY_HUB_BASE_URL: policyHubUrl };

    expect((await loadRuntimeConfig()).policyHubBaseUrl).toBe(policyHubUrl);
  });

  it('uses the current Vite base path by default', async () => {
    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.appBasePath).toBe('');
  });

  it('defaults to basic auth mode when unset', async () => {
    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.authMode).toBe('basic');
  });

  it('reads the BFF-bridged runtime config', async () => {
    window.__RUNTIME_CONFIG__ = {
      API_BASE_URL: 'https://app.example.com',
      authMode: 'oidc',
      platformApiBaseUrl: '/proxy',
      billingProxyEnabled: 'true',
      API_PLATFORM_HOME_PAGE: 'https://wso2.com/api-platform/',
      ORGANIZATION_API_URL: 'https://apis.example.com/orgs/1.0.0',
      PROJECT_API_BASE_URL: 'https://apis.example.com/projects/1.0.0/graphql',
      PRIVACY_POLICY_LINK: 'https://wso2.com/api-platform/privacy-policy',
      TERMS_OF_USE_LINK: 'https://wso2.com/api-platform/terms-of-use',
      TOS_SERVICE_NAME: 'api-platform',
      USERS_MANAGEMENT_API_URL: 'https://apis.example.com/user-mgt/1.0.0',
    };

    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.apiBaseUrl).toBe('https://app.example.com');
    expect(runtimeConfig.authMode).toBe('oidc');
    expect(runtimeConfig.platformApiBaseUrl).toBe('/proxy');
    expect(runtimeConfig.billingProxyEnabled).toBe(true);
    expect(runtimeConfig.organizationApiUrl).toBe(
      'https://apis.example.com/orgs/1.0.0'
    );
    expect(runtimeConfig.privacyPolicyLink).toBe(
      'https://wso2.com/api-platform/privacy-policy'
    );
    expect(runtimeConfig.projectApiBaseUrl).toBe(
      'https://apis.example.com/projects/1.0.0/graphql'
    );
    expect(runtimeConfig.termsOfUseLink).toBe(
      'https://wso2.com/api-platform/terms-of-use'
    );
    expect(runtimeConfig.toSServiceName).toBe('api-platform');
    expect(runtimeConfig.usersManagementApiUrl).toBe(
      'https://apis.example.com/user-mgt/1.0.0'
    );
  });

  it('an unrecognized authMode value falls back to basic', async () => {
    window.__RUNTIME_CONFIG__ = { authMode: 'something-else' };

    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.authMode).toBe('basic');
  });

  it('billingProxyEnabled defaults to false when absent', async () => {
    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.billingProxyEnabled).toBe(false);
  });

  it('cloudProxyEnabled defaults to false when absent', async () => {
    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.cloudProxyEnabled).toBe(false);
  });

  it('reads cloudProxyEnabled and moesifAppUrl from runtime config', async () => {
    window.__RUNTIME_CONFIG__ = {
      cloudProxyEnabled: 'true',
      moesifAppUrl: 'https://www.moesif.com',
    };

    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.cloudProxyEnabled).toBe(true);
    expect(runtimeConfig.moesifAppUrl).toBe('https://www.moesif.com');
  });
});
