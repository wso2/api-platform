import { afterEach, describe, expect, it, vi } from 'vitest';

const loadRuntimeConfig = async () => {
  vi.resetModules();
  return (await import('./runtime')).runtimeConfig;
};

afterEach(() => {
  delete window.config;
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
});

describe('runtimeConfig', () => {
  it('uses the current Vite base path by default', async () => {
    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.appBasePath).toBe('');
  });

  it('reads legacy choreo-console runtime auth config', async () => {
    window.__RUNTIME_CONFIG__ = {
      API_BASE_URL: 'https://app.example.com',
      ASGARDEO_SDK_CONFIG: {
        clientID: 'client-id',
        signInRedirectURL: 'https://old.example.com/signin',
      },
      ASGARDEO_SDK_RESOURCE_SERVER_URLS: 'https://api-one|https://api-two',
      ASGARDEO_SDK_SCOPES: 'email|profile|groups',
      FIDP_GITHUB: 'github',
      FIDP_GOOGLE: 'google',
      FIDP_MICROSOFT: 'microsoft',
      FIDP_ENTERPRISE: 'EnterpriseIDP',
      AVAILABLE_LOGIN_REGIONS: 'US::https://localhost:3000,EU::https://eu.example.com',
      API_PLATFORM_HOME_PAGE: 'https://wso2.com/api-platform/',
      DISABLE_ENTERPRISE_LOGIN: 'false',
      EMAIL_LOGIN_CONFIGS: {
        ASGARDEO_SIGNUP_URL: 'https://dev.asgardeo.io/signup',
        ENABLE_EMAIL_LOGIN: 'true',
      },
      ENABLE_MICROSOFT_LOGIN: 'true',
      ORGANIZATION_API_URL: 'https://apis.example.com/orgs/1.0.0',
      PROJECT_API_BASE_URL: 'https://apis.example.com/projects/1.0.0/graphql',
      PRIVACY_POLICY_LINK: 'https://wso2.com/api-platform/privacy-policy',
      TERMS_OF_USE_LINK: 'https://wso2.com/api-platform/terms-of-use',
      TOS_SERVICE_NAME: 'api-platform',
      USERS_MANAGEMENT_API_URL: 'https://apis.example.com/user-mgt/1.0.0',
    };

    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.apiBaseUrl).toBe('https://app.example.com');
    expect(runtimeConfig.asgardeoSdkConfig?.clientID).toBe('client-id');
    expect(runtimeConfig.asgardeoSdkResourceServerUrls).toEqual([
      'https://api-one',
      'https://api-two',
    ]);
    expect(runtimeConfig.asgardeoSdkScopes).toEqual([
      'email',
      'profile',
      'groups',
    ]);
    expect(runtimeConfig.fidpGoogle).toBe('google');
    expect(runtimeConfig.fidpGithub).toBe('github');
    expect(runtimeConfig.fidpMicrosoft).toBe('microsoft');
    expect(runtimeConfig.fidpEnterprise).toBe('EnterpriseIDP');
    expect(runtimeConfig.availableLoginRegions).toEqual([
      'US::https://localhost:3000',
      'EU::https://eu.example.com',
    ]);
    expect(runtimeConfig.disableEnterpriseLogin).toBe(false);
    expect(runtimeConfig.enableEmailLogin).toBe(true);
    expect(runtimeConfig.enableMicrosoftLogin).toBe(true);
    expect(runtimeConfig.signupUrl).toBe('https://dev.asgardeo.io/signup');
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

  it('supports explicit local file auth mode', async () => {
    window.__RUNTIME_CONFIG__ = {
      authMode: 'local-file',
      LOCAL_AUTH_FILE_URL: '/auth/local-session.json',
    };

    const runtimeConfig = await loadRuntimeConfig();

    expect(runtimeConfig.authMode).toBe('local-file');
    expect(runtimeConfig.localAuthFileUrl).toBe('/auth/local-session.json');
  });
});
