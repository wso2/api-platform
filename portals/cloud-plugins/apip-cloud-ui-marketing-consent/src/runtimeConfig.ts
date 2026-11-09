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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

type WindowConfig = Partial<{
  PLATFORM_API_BASE_URL: string;
  platformApiBaseUrl: string;
}>;

const fromWindow = (): WindowConfig => {
  const globalWindow = window as Window & {
    config?: WindowConfig;
    __RUNTIME_CONFIG__?: WindowConfig;
  };
  return {
    ...(globalWindow.__RUNTIME_CONFIG__ ?? {}),
    ...(globalWindow.config ?? {}),
  };
};

// Placeholder origin a candidate path is resolved against, to see whether the
// URL parser keeps it on this origin.
const PROBE_ORIGIN = 'https://same-origin.invalid';

/** Whether `path` stays on this page's origin (`//host` and `/\host` do not). */
const isSameOriginPath = (path: string): boolean => {
  if (!path.startsWith('/')) return false;
  try {
    return new URL(path, PROBE_ORIGIN).origin === PROBE_ORIGIN;
  } catch {
    return false;
  }
};

/**
 * The same-origin path this console's named upstreams sit under: `/proxy` behind
 * the BFF, `/api-management/proxy` behind a gateway that serves the console under
 * a path. Same rule as the entitlements plugin.
 */
export function proxyBase(): string {
  const config = fromWindow();
  const base = (config.PLATFORM_API_BASE_URL || config.platformApiBaseUrl || '')
    .trim()
    .replace(/\/+$/, '');
  return isSameOriginPath(base) ? base : '/proxy';
}

/**
 * Where the `marketing-consent` named upstream is reached. The BFF swaps the
 * `{proxy}/marketing-consent` prefix for the upstream URL (the internal gateway's
 * marketing-consent endpoint) and injects the signed-in person's own access token,
 * so neither the gateway address nor a token ever reaches the browser.
 */
export const consentUrl = (): string => `${proxyBase()}/marketing-consent/marketing-consent`;
