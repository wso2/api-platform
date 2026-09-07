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

export type RuntimeConfig = {
  appBasePath: string;
  apiBaseUrl: string;
  /**
   * "basic" — the BFF's file-based login against Platform API's own user
   * store (no external IdP). "oidc" — the BFF runs a confidential/PKCE code
   * exchange against a configured external IdP. Either way, the browser
   * never sees a token; the BFF holds the session server-side.
   */
  authMode: 'basic' | 'oidc';
  /**
   * Per-deployment default locale (BCP 47, e.g. "en"). Falls through
   * to `DEFAULT_LOCALE` in `src/i18n/config.ts` if empty or unsupported.
   */
  defaultLocale: string;
  environmentName: string;
  featureFlags: string[];
  apiPlatformHomePage: string;
  organizationApiUrl: string;
  /**
   * Set when the BFF has a "billing" named upstream configured (cloud only).
   * When true, ProductActivation calls it via the same-origin proxy
   * (/proxy/billing/...) — the browser never learns the real billing URL.
   */
  billingProxyEnabled: boolean;
  /**
   * Same-origin path the BFF proxies to the Platform API (typically
   * "/proxy") — the browser only ever calls this BFF's own origin, which
   * injects the session's bearer token server-side.
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
  /**
   * Moesif analytics console. The Insights pages link out to it rather than
   * rendering charts here, so the deployment's own workspace URL is config,
   * not a hardcoded link.
   */
  moesifWebUrl: string;
  /**
   * API Designer for VS Code. The wizard's "design from scratch" step hands
   * off to the extension rather than hosting a canvas here, so the
   * deployment's own marketplace and docs links are config, not hardcoded.
   */
  apiDesignerVsCodeUrl: string;
  apiDesignerDocsUrl: string;
  privacyPolicyLink: string;
  projectApiBaseUrl: string;
  termsOfUseLink: string;
  toSServiceName: string;
  usersManagementApiUrl: string;
};

type LegacyWindowConfig = Partial<{
  API_BASE_URL: string;
  API_PLATFORM_HOME_PAGE: string;
  AUTH_MODE: string;
  ORGANIZATION_API_URL: string;
  BILLING_PROXY_ENABLED: string;
  billingProxyEnabled: boolean | string;
  DEFAULT_LOCALE: string;
  defaultLocale: string;
  PLATFORM_API_BASE_URL: string;
  platformApiBaseUrl: string;
  PLATFORM_API_VERSION: string;
  platformApiVersion: string;
  GATEWAY_CONTROL_PLANE_HOST: string;
  POLICY_HUB_BASE_URL: string;
  POLICY_HUB_WEB_URL: string;
  MOESIF_WEB_URL: string;
  API_DESIGNER_VSCODE_URL: string;
  API_DESIGNER_DOCS_URL: string;
  PRIVACY_POLICY_LINK: string;
  PROJECT_API_BASE_URL: string;
  TERMS_OF_USE_LINK: string;
  TOS_SERVICE_NAME: string;
  USERS_MANAGEMENT_API_URL: string;
  appBasePath: string;
  apiBaseUrl: string;
  baseUrl: string;
  authMode: string;
  FEATURE_FLAGS: string;
  environmentName: string;
  apiPlatformHomePage: string;
  organizationApiUrl: string;
  privacyPolicyLink: string;
  projectApiBaseUrl: string;
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

const splitCommaConfigList = (value?: string) => value?.split(',').filter(Boolean) ?? [];

const readBoolean = (value: boolean | string | undefined) => value === true || value === 'true';

const readAuthMode = (value: string | undefined): RuntimeConfig['authMode'] =>
  value === 'oidc' ? 'oidc' : 'basic';

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
      '',
  ),
  apiBaseUrl:
    fromWindow().apiBaseUrl ||
    fromWindow().API_BASE_URL ||
    fromWindow().baseUrl ||
    import.meta.env.VITE_API_BASE_URL ||
    '',
  authMode: readAuthMode(
    fromWindow().authMode || fromWindow().AUTH_MODE || import.meta.env.VITE_AUTH_MODE,
  ),
  defaultLocale:
    fromWindow().DEFAULT_LOCALE ||
    fromWindow().defaultLocale ||
    import.meta.env.VITE_DEFAULT_LOCALE ||
    '',
  environmentName: fromWindow().environmentName || import.meta.env.VITE_ENVIRONMENT_NAME || 'local',
  featureFlags: splitCommaConfigList(
    fromWindow().FEATURE_FLAGS || import.meta.env.VITE_FEATURE_FLAGS,
  ),
  apiPlatformHomePage:
    fromWindow().API_PLATFORM_HOME_PAGE ||
    fromWindow().apiPlatformHomePage ||
    import.meta.env.VITE_API_PLATFORM_HOME_PAGE ||
    'https://wso2.com/api-platform/',
  organizationApiUrl:
    fromWindow().ORGANIZATION_API_URL ||
    fromWindow().organizationApiUrl ||
    import.meta.env.VITE_ORGANIZATION_API_URL ||
    '',
  billingProxyEnabled: readBoolean(
    fromWindow().BILLING_PROXY_ENABLED ||
      fromWindow().billingProxyEnabled ||
      import.meta.env.VITE_BILLING_PROXY_ENABLED,
  ),
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
    fromWindow().POLICY_HUB_BASE_URL || import.meta.env.VITE_POLICY_HUB_BASE_URL || '',
  policyHubWebUrl:
    fromWindow().POLICY_HUB_WEB_URL ||
    import.meta.env.VITE_POLICY_HUB_WEB_URL ||
    'https://wso2.com/api-platform/policy-hub/',
  moesifWebUrl:
    fromWindow().MOESIF_WEB_URL ||
    import.meta.env.VITE_MOESIF_WEB_URL ||
    'https://www.moesif.com/wrap/basic',
  apiDesignerVsCodeUrl:
    fromWindow().API_DESIGNER_VSCODE_URL ||
    import.meta.env.VITE_API_DESIGNER_VSCODE_URL ||
    'https://marketplace.visualstudio.com/items?itemName=WSO2.api-designer',
  apiDesignerDocsUrl:
    fromWindow().API_DESIGNER_DOCS_URL ||
    import.meta.env.VITE_API_DESIGNER_DOCS_URL ||
    'https://wso2.com/api-platform/docs/tools/vscode-api-design/getting-started/',
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
  termsOfUseLink:
    fromWindow().TERMS_OF_USE_LINK ||
    fromWindow().termsOfUseLink ||
    import.meta.env.VITE_TERMS_OF_USE_LINK ||
    'https://wso2.com/api-platform/terms-of-use',
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
