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
  describe('sanitizeHtmlDocsEnabled', () => {
    it('defaults to on when unset', async () => {
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsEnabled).toBe(true);
    });

    it.each(['false', 'FALSE', 'False', ' false ', 'fAlSe'])('turns off for %j in any letter case', async (value) => {
      window.__RUNTIME_CONFIG__ = { sanitizeHtmlDocsEnabled: value };
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsEnabled).toBe(false);
    });

    it('turns off for a boolean false', async () => {
      window.__RUNTIME_CONFIG__ = { sanitizeHtmlDocsEnabled: false };
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsEnabled).toBe(false);
    });

    it.each(['true', 'TRUE', '', 'no', '0', 'off', 'flase'])('stays on for %j', async (value) => {
      window.__RUNTIME_CONFIG__ = { sanitizeHtmlDocsEnabled: value };
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsEnabled).toBe(true);
    });

    it('reads the build-time flag case-insensitively too', async () => {
      vi.stubEnv('VITE_SANITIZE_HTML_DOCS_ENABLED', 'False');
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsEnabled).toBe(false);
    });

    it('prefers the BFF runtime value over the build-time flag', async () => {
      vi.stubEnv('VITE_SANITIZE_HTML_DOCS_ENABLED', 'false');
      window.__RUNTIME_CONFIG__ = { sanitizeHtmlDocsEnabled: 'true' };
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsEnabled).toBe(true);
    });
  });

  it('reads the sanitize allowlist additions the BFF emits as JSON strings', async () => {
    window.__RUNTIME_CONFIG__ = {
      sanitizeHtmlDocsAdditionalAllowedAttributes: '{"video":["controls","poster"],"*":["hidden"]}',
      sanitizeHtmlDocsAdditionalAllowedTags: '["video"]',
    };
    const runtimeConfig = await loadRuntimeConfig();
    expect(runtimeConfig.sanitizeHtmlDocsAdditionalAllowedTags).toEqual(['video']);
    expect(runtimeConfig.sanitizeHtmlDocsAdditionalAllowedAttributes).toEqual({
      '*': ['hidden'],
      video: ['controls', 'poster'],
    });
  });
  describe('sanitizeHtmlDocsAdditionalAllowedTags', () => {
    it.each([
      ['a JSON array', '["video","iframe"]', ['video', 'iframe']],
      ['a comma-separated list', 'video, iframe,,', ['video', 'iframe']],
      ['a single tag', ' video ', ['video']],
      ['broken JSON', '["video","iframe"', []],
      ['a JSON array with a non-string', '["video",1]', []],
      ['a blank value', '  ', []],
    ])('reads %s', async (_label, value, expected) => {
      window.__RUNTIME_CONFIG__ = { sanitizeHtmlDocsAdditionalAllowedTags: value };
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsAdditionalAllowedTags).toEqual(expected);
    });
  });

  describe('sanitizeHtmlDocsAdditionalAllowedAttributes', () => {
    it.each([
      ['string arrays', '{"video":["controls","poster"]}', { video: ['controls', 'poster'] }],
      ['a single string', '{"div":"hidden"}', { div: ['hidden'] }],
      ['a comma-separated string', '{"video":"controls, poster"}', { video: ['controls', 'poster'] }],
      ['a blank string', '{"div":" ","img":["loading"]}', { img: ['loading'] }],
      ['an entry of another shape', '{"div":1,"p":["hidden",2],"img":["loading"]}', { img: ['loading'] }],
      ['a top-level array', '["src"]', {}],
      ['broken JSON', '{"iframe":["src"]', {}],
    ])('reads %s', async (_label, value, expected) => {
      window.__RUNTIME_CONFIG__ = { sanitizeHtmlDocsAdditionalAllowedAttributes: value };
      expect((await loadRuntimeConfig()).sanitizeHtmlDocsAdditionalAllowedAttributes).toEqual(expected);
    });
  });
});
