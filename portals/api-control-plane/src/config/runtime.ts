export type AsgardeoSdkConfig = {
  afterSignInUrl?: string;
  afterSignOutUrl?: string;
  baseUrl?: string;
  clientID?: string;
  clientId?: string;
  enablePKCE?: boolean;
  responseMode?: string;
  scope?: string[];
  signInRedirectURL?: string;
  signOutRedirectURL?: string;
  storage?: 'sessionStorage' | 'localStorage' | 'webWorker';
  validateIDToken?: boolean;
  [key: string]: unknown;
};

/**
 * OIDC settings for a generic provider (Thunder). Endpoints default to the
 * Thunder `/oauth2/*` layout derived from `authBaseUrl` when not given. The
 * adapter (ThunderAuthAdapter) builds an oidc-client-ts metadata object from
 * this; explicit endpoints win over the derived defaults.
 */
export type ThunderConfig = {
  /** id_token `iss` value to validate against (e.g. "platform_idp"). */
  issuer?: string;
  authorizationEndpoint?: string;
  tokenEndpoint?: string;
  userinfoEndpoint?: string;
  jwksUri?: string;
  endSessionEndpoint?: string;
};

export type RuntimeConfig = {
  appBasePath: string;
  apiBaseUrl: string;
  authMode: 'asgardeo' | 'local-file' | 'thunder';
  asgardeoSdkConfig?: AsgardeoSdkConfig;
  asgardeoSdkResourceServerUrls: string[];
  asgardeoSdkScopes: string[];
  authBaseUrl: string;
  authClientId: string;
  /** OAuth scopes requested by the generic (Thunder) OIDC flow. */
  authScopes: string[];
  thunder?: ThunderConfig;
  enableLocalAuthFallback: boolean;
  environmentName: string;
  featureFlags: string[];
  localAuthFileUrl: string;
  fidpEmail: string;
  fidpEnterprise?: string;
  fidpGithub?: string;
  fidpGoogle?: string;
  fidpMicrosoft?: string;
  availableLoginRegions: string[];
  apiPlatformHomePage: string;
  disableEnterpriseLogin: boolean;
  enableEmailLogin: boolean;
  enableMicrosoftLogin: boolean;
  organizationApiUrl: string;
  /**
   * Base URL of the billing user-api (e.g. ".../billing-service-user-api/api/v1").
   * Calling GET {billingServiceUrl}/organization?product=api-platform after login both
   * reads the subscription and performs first-login activation of the api-platform
   * subscription (which fires subscription.activated -> gateway provisioning).
   */
  billingServiceUrl: string;
  /**
   * Base URL of the BML / platform-api gateway. When set, read flows use
   * platform-api REST (proxied via BML) instead of the legacy GraphQL.
   */
  platformApiBaseUrl: string;
  /**
   * platform-api REST version segment (e.g. "v0.9"). Appended by the client as
   * `${platformApiBaseUrl}/api/${platformApiVersion}` to form the request base,
   * so bumping the API version is a config change, not a code change. Defaults
   * to "v0.9" (the mount path of the current platform-api release, see its
   * `constants.APIVersion`) when unset.
   */
  platformApiVersion: string;
  /**
   * Host a self-hosted gateway agent connects to (control plane). Shown in the
   * gateway setup instructions. Defaults to the platformApiBaseUrl host.
   */
  gatewayControlPlaneHost: string;
  /** Policy Hub API (catalog of mediation policies) + its web UI link. */
  policyHubBaseUrl: string;
  policyHubWebUrl: string;
  privacyPolicyLink: string;
  projectApiBaseUrl: string;
  signupUrl: string;
  termsOfUseLink: string;
  tokenExchangeConfig?: Record<string, unknown>;
  toSServiceName: string;
  usersManagementApiUrl: string;
};

type LegacyWindowConfig = Partial<{
  API_BASE_URL: string;
  ASGARDEO_SDK_CONFIG: AsgardeoSdkConfig;
  ASGARDEO_SDK_RESOURCE_SERVER_URLS: string;
  ASGARDEO_SDK_SCOPES: string;
  FIDP_ENTERPRISE: string;
  FIDP_GITHUB: string;
  FIDP_GOOGLE: string;
  FIDP_MICROSOFT: string;
  AVAILABLE_LOGIN_REGIONS: string;
  API_PLATFORM_HOME_PAGE: string;
  DISABLE_ENTERPRISE_LOGIN: string;
  EMAIL_LOGIN_CONFIGS: {
    ASGARDEO_SIGNUP_URL?: string;
    ENABLE_EMAIL_LOGIN?: string;
  };
  ENABLE_MICROSOFT_LOGIN: string;
  AUTH_SCOPES: string;
  THUNDER_CONFIG: ThunderConfig;
  ORGANIZATION_API_URL: string;
  BILLING_SERVICE_URL: string;
  billingServiceUrl: string;
  PLATFORM_API_BASE_URL: string;
  platformApiBaseUrl: string;
  PLATFORM_API_VERSION: string;
  platformApiVersion: string;
  GATEWAY_CONTROL_PLANE_HOST: string;
  POLICY_HUB_BASE_URL: string;
  POLICY_HUB_WEB_URL: string;
  PRIVACY_POLICY_LINK: string;
  PROJECT_API_BASE_URL: string;
  TERMS_OF_USE_LINK: string;
  TOKEN_EXCHANGE_CONFIG: Record<string, unknown>;
  TOS_SERVICE_NAME: string;
  USERS_MANAGEMENT_API_URL: string;
  availableLoginRegions: string;
  appBasePath: string;
  apiBaseUrl: string;
  baseUrl: string;
  authMode: string;
  authBaseUrl: string;
  asgardeoBaseUrl: string;
  authClientId: string;
  clientId: string;
  ENABLE_LOCAL_AUTH_FALLBACK: string;
  FEATURE_FLAGS: string;
  LOCAL_AUTH_FILE_URL: string;
  environmentName: string;
  localAuthFileUrl: string;
  enableLocalAuthFallback: boolean | string;
  fidpEnterprise: string;
  fidpGithub: string;
  fidpGoogle: string;
  fidpMicrosoft: string;
  apiPlatformHomePage: string;
  disableEnterpriseLogin: boolean | string;
  enableEmailLogin: boolean | string;
  enableMicrosoftLogin: boolean | string;
  organizationApiUrl: string;
  privacyPolicyLink: string;
  projectApiBaseUrl: string;
  signupUrl: string;
  termsOfUseLink: string;
  toSServiceName: string;
  usersManagementApiUrl: string;
}>;

declare global {
  interface Window {
    config?: LegacyWindowConfig;
    __RUNTIME_CONFIG__?: LegacyWindowConfig;
  }
}

const fromWindow = (): LegacyWindowConfig => ({
  ...(window.__RUNTIME_CONFIG__ ?? {}),
  ...(window.config ?? {}),
});

const splitConfigList = (value?: string) =>
  value?.split('|').filter(Boolean) ?? [];

const splitCommaConfigList = (value?: string) =>
  value?.split(',').filter(Boolean) ?? [];

const readBoolean = (value: boolean | string | undefined) =>
  value === true || value === 'true';

const readAuthMode = (
  value: string | undefined
): RuntimeConfig['authMode'] | undefined => {
  if (value === 'local-file') return 'local-file';
  if (value === 'asgardeo') return 'asgardeo';
  if (value === 'thunder') return 'thunder';
  return undefined;
};

const normalizeBasePath = (value?: string) => {
  if (!value || value === '/') return '';
  return `/${value.replace(/^\/|\/$/g, '')}`;
};

// VITE_ override wins so a local BML can be targeted in dev even when the
// runtime config injects a hosted "connect" gateway URL.
const resolvedPlatformApiBaseUrl =
  import.meta.env.VITE_PLATFORM_API_BASE_URL ||
  fromWindow().PLATFORM_API_BASE_URL ||
  fromWindow().platformApiBaseUrl ||
  '';

const hostFromUrl = (url: string) => {
  try {
    return url ? new URL(url).host : '';
  } catch {
    return '';
  }
};

export const runtimeConfig: RuntimeConfig = {
  appBasePath: normalizeBasePath(
    fromWindow().appBasePath ||
      import.meta.env.VITE_APP_BASE_PATH ||
      import.meta.env.BASE_URL ||
      '/oxygen-console'
  ),
  apiBaseUrl:
    fromWindow().apiBaseUrl ||
    fromWindow().API_BASE_URL ||
    fromWindow().baseUrl ||
    import.meta.env.VITE_API_BASE_URL ||
    '',
  authMode:
    readAuthMode(fromWindow().authMode || import.meta.env.VITE_AUTH_MODE) ||
    (readBoolean(
      fromWindow().enableLocalAuthFallback ||
        fromWindow().ENABLE_LOCAL_AUTH_FALLBACK ||
        import.meta.env.VITE_ENABLE_LOCAL_AUTH_FALLBACK
    )
      ? 'local-file'
      : 'asgardeo'),
  asgardeoSdkConfig: fromWindow().ASGARDEO_SDK_CONFIG,
  asgardeoSdkResourceServerUrls: splitConfigList(
    fromWindow().ASGARDEO_SDK_RESOURCE_SERVER_URLS
  ),
  asgardeoSdkScopes: splitConfigList(fromWindow().ASGARDEO_SDK_SCOPES),
  authBaseUrl:
    fromWindow().authBaseUrl ||
    fromWindow().asgardeoBaseUrl ||
    import.meta.env.VITE_AUTH_BASE_URL ||
    '',
  authClientId:
    fromWindow().authClientId ||
    fromWindow().clientId ||
    import.meta.env.VITE_AUTH_CLIENT_ID ||
    '',
  authScopes: splitCommaConfigList(
    fromWindow().AUTH_SCOPES || import.meta.env.VITE_AUTH_SCOPES
  ),
  thunder: fromWindow().THUNDER_CONFIG,
  enableLocalAuthFallback: readBoolean(
    fromWindow().enableLocalAuthFallback ||
      fromWindow().ENABLE_LOCAL_AUTH_FALLBACK ||
      import.meta.env.VITE_ENABLE_LOCAL_AUTH_FALLBACK
  ),
  environmentName:
    fromWindow().environmentName ||
    import.meta.env.VITE_ENVIRONMENT_NAME ||
    'local',
  featureFlags: splitCommaConfigList(
    fromWindow().FEATURE_FLAGS || import.meta.env.VITE_FEATURE_FLAGS
  ),
  localAuthFileUrl:
    fromWindow().LOCAL_AUTH_FILE_URL ||
    fromWindow().localAuthFileUrl ||
    import.meta.env.VITE_LOCAL_AUTH_FILE_URL ||
    '',
  fidpEmail: 'LOCAL',
  fidpEnterprise:
    fromWindow().FIDP_ENTERPRISE ||
    fromWindow().fidpEnterprise ||
    import.meta.env.VITE_FIDP_ENTERPRISE,
  fidpGithub:
    fromWindow().FIDP_GITHUB ||
    fromWindow().fidpGithub ||
    import.meta.env.VITE_FIDP_GITHUB,
  fidpGoogle:
    fromWindow().FIDP_GOOGLE ||
    fromWindow().fidpGoogle ||
    import.meta.env.VITE_FIDP_GOOGLE,
  fidpMicrosoft:
    fromWindow().FIDP_MICROSOFT ||
    fromWindow().fidpMicrosoft ||
    import.meta.env.VITE_FIDP_MICROSOFT,
  availableLoginRegions: splitCommaConfigList(
    fromWindow().AVAILABLE_LOGIN_REGIONS ||
      fromWindow().availableLoginRegions ||
      import.meta.env.VITE_AVAILABLE_LOGIN_REGIONS
  ),
  apiPlatformHomePage:
    fromWindow().API_PLATFORM_HOME_PAGE ||
    fromWindow().apiPlatformHomePage ||
    import.meta.env.VITE_API_PLATFORM_HOME_PAGE ||
    'https://wso2.com/api-platform/',
  disableEnterpriseLogin: readBoolean(
    fromWindow().disableEnterpriseLogin ||
      fromWindow().DISABLE_ENTERPRISE_LOGIN ||
      import.meta.env.VITE_DISABLE_ENTERPRISE_LOGIN
  ),
  enableEmailLogin: readBoolean(
    fromWindow().enableEmailLogin ||
      fromWindow().EMAIL_LOGIN_CONFIGS?.ENABLE_EMAIL_LOGIN ||
      import.meta.env.VITE_ENABLE_EMAIL_LOGIN
  ),
  enableMicrosoftLogin: readBoolean(
    fromWindow().enableMicrosoftLogin ||
      fromWindow().ENABLE_MICROSOFT_LOGIN ||
      import.meta.env.VITE_ENABLE_MICROSOFT_LOGIN
  ),
  organizationApiUrl:
    fromWindow().ORGANIZATION_API_URL ||
    fromWindow().organizationApiUrl ||
    import.meta.env.VITE_ORGANIZATION_API_URL ||
    '',
  billingServiceUrl:
    fromWindow().BILLING_SERVICE_URL ||
    fromWindow().billingServiceUrl ||
    import.meta.env.VITE_BILLING_SERVICE_URL ||
    '',
  platformApiBaseUrl: resolvedPlatformApiBaseUrl,
  platformApiVersion:
    fromWindow().PLATFORM_API_VERSION ||
    fromWindow().platformApiVersion ||
    import.meta.env.VITE_PLATFORM_API_VERSION ||
    'v0.9',
  gatewayControlPlaneHost:
    fromWindow().GATEWAY_CONTROL_PLANE_HOST ||
    import.meta.env.VITE_GATEWAY_CONTROL_PLANE_HOST ||
    hostFromUrl(resolvedPlatformApiBaseUrl) ||
    'localhost:9243',
  policyHubBaseUrl:
    fromWindow().POLICY_HUB_BASE_URL ||
    import.meta.env.VITE_POLICY_HUB_BASE_URL ||
    '',
  policyHubWebUrl:
    fromWindow().POLICY_HUB_WEB_URL ||
    import.meta.env.VITE_POLICY_HUB_WEB_URL ||
    'https://wso2.com/api-platform/policy-hub/',
  privacyPolicyLink:
    fromWindow().PRIVACY_POLICY_LINK ||
    fromWindow().privacyPolicyLink ||
    import.meta.env.VITE_PRIVACY_POLICY_LINK ||
    'https://wso2.com/api-platform/privacy-policy',
  projectApiBaseUrl:
    fromWindow().PROJECT_API_BASE_URL ||
    fromWindow().projectApiBaseUrl ||
    import.meta.env.VITE_PROJECT_API_BASE_URL ||
    '',
  signupUrl:
    fromWindow().signupUrl ||
    fromWindow().EMAIL_LOGIN_CONFIGS?.ASGARDEO_SIGNUP_URL ||
    import.meta.env.VITE_SIGNUP_URL ||
    '/signup',
  termsOfUseLink:
    fromWindow().TERMS_OF_USE_LINK ||
    fromWindow().termsOfUseLink ||
    import.meta.env.VITE_TERMS_OF_USE_LINK ||
    'https://wso2.com/api-platform/terms-of-use',
  tokenExchangeConfig: fromWindow().TOKEN_EXCHANGE_CONFIG,
  toSServiceName:
    fromWindow().TOS_SERVICE_NAME ||
    fromWindow().toSServiceName ||
    import.meta.env.VITE_TOS_SERVICE_NAME ||
    'api-platform',
  usersManagementApiUrl:
    fromWindow().USERS_MANAGEMENT_API_URL ||
    fromWindow().usersManagementApiUrl ||
    import.meta.env.VITE_USERS_MANAGEMENT_API_URL ||
    '',
};
