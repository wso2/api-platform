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

/**
 * What the deployment gives this plugin, from the console's runtime config
 * (`window.__RUNTIME_CONFIG__` with `window.config` over it, merged the way the console reads it):
 *
 *   cookieProDomainScript  the CookiePro domain script id (a UUID, `-test` on dev)
 *   gtmContainerId         the Google Tag Manager container (`GTM-XXXX`)
 *
 * Both are needed. A container without CookiePro could never be granted consent,
 * so tracking stays off (and says so) rather than load a container that cannot
 * be gated. Values are checked against their formats, since they are placed
 * into a script URL and attribute.
 */

export type TrackingConfig = {
  cookieProDomainScript: string;
  gtmContainerId: string;
};

const DOMAIN_SCRIPT = /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}(-test)?$/i;
const GTM_ID = /^GTM-[A-Z0-9]{4,12}$/;

type RuntimeWindow = Window & {
  __RUNTIME_CONFIG__?: Record<string, unknown>;
  config?: Record<string, unknown>;
};

/** The console's own runtime config: `window.config` over `window.__RUNTIME_CONFIG__`. */
const mergedRuntimeConfig = (): Record<string, unknown> => {
  const runtimeWindow = window as RuntimeWindow;
  return { ...(runtimeWindow.__RUNTIME_CONFIG__ ?? {}), ...(runtimeWindow.config ?? {}) };
};

const text = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

export function readTrackingConfig(
  source: Record<string, unknown> | undefined = mergedRuntimeConfig(),
  warn: (message: string) => void = (message) => console.warn(message)
): TrackingConfig | null {
  const cookieProDomainScript = text(source?.cookieProDomainScript);
  const gtmContainerId = text(source?.gtmContainerId);
  if (!gtmContainerId && !cookieProDomainScript) return null;
  if (!gtmContainerId || !cookieProDomainScript) {
    warn('Tracking is off: it needs both gtmContainerId and cookieProDomainScript.');
    return null;
  }
  if (!GTM_ID.test(gtmContainerId) || !DOMAIN_SCRIPT.test(cookieProDomainScript)) {
    warn('Tracking is off: gtmContainerId or cookieProDomainScript is not in the expected format.');
    return null;
  }
  return { cookieProDomainScript, gtmContainerId };
}
