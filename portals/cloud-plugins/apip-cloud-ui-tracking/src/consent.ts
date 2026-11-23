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
 * Google consent mode, driven by CookiePro. Everything starts denied; the
 * visitor's CookiePro choice grants it: Performance (C0002) grants
 * `analytics_storage`, Targeting (C0004) the three ad settings. CookiePro's own
 * Google Consent Mode integration is off in the WSO2 account, so this is the only
 * thing that ever grants consent. Same mapping as the WSO2 Cloud console.
 */

export type ConsentValue = 'granted' | 'denied';
export type ConsentState = {
  ad_storage: ConsentValue;
  ad_user_data: ConsentValue;
  ad_personalization: ConsentValue;
  analytics_storage: ConsentValue;
};

export const PERFORMANCE_GROUP = 'C0002';
export const TARGETING_GROUP = 'C0004';

export const DENIED: ConsentState = {
  ad_storage: 'denied',
  ad_user_data: 'denied',
  ad_personalization: 'denied',
  analytics_storage: 'denied',
};

export function consentForGroups(groups: ReadonlySet<string>): ConsentState {
  const ads: ConsentValue = groups.has(TARGETING_GROUP) ? 'granted' : 'denied';
  return {
    ad_storage: ads,
    ad_user_data: ads,
    ad_personalization: ads,
    analytics_storage: groups.has(PERFORMANCE_GROUP) ? 'granted' : 'denied',
  };
}

/**
 * The groups the visitor accepted, from CookiePro's OptanonConsent cookie; null if
 * absent or malformed. The cookie is `key=value&key=value`, each *value* encoded
 * (`landingPath` holds a whole URL, query included), so it is split into fields
 * first and only then decoded: decoding the whole cookie first would turn an
 * encoded `&groups=...` inside `landingPath` into a field of its own.
 */
export function savedGroups(cookies: string): Set<string> | null {
  const entry = cookies.split(';').map((part) => part.trim()).find((part) => part.startsWith('OptanonConsent='));
  if (!entry) return null;
  try {
    const raw = entry.slice('OptanonConsent='.length);
    let groups: string | null = null;
    for (const field of raw.split('&')) {
      const at = field.indexOf('=');
      if (at > 0 && field.slice(0, at) === 'groups') {
        groups = decodeURIComponent(field.slice(at + 1));
        break;
      }
    }
    if (!groups) return null;
    const active = new Set<string>();
    for (const item of groups.split(',')) {
      const [id, state] = item.split(':');
      if (id && state === '1') active.add(id);
    }
    return active;
  } catch {
    return null;
  }
}

/** CookiePro's `OnetrustActiveGroups` is `,C0001,C0002,`. */
export function activeGroups(value: unknown): Set<string> {
  return new Set(typeof value === 'string' ? value.split(',').filter(Boolean) : []);
}

type TrackingWindow = Window & {
  dataLayer?: unknown[];
  OneTrustActiveGroups?: string;
  OnetrustActiveGroups?: string;
  OptanonWrapper?: () => void;
};

/** `gtag()`: GTM only treats `arguments` objects pushed to the data layer as commands. */
export function gtag(win: Window, command: string, ...rest: unknown[]): void {
  const w = win as TrackingWindow;
  const layer = (w.dataLayer = w.dataLayer ?? []);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const push = function (..._params: unknown[]) {
    // eslint-disable-next-line prefer-rest-params
    layer.push(arguments);
  };
  push(command, ...rest);
}

export function setConsentDefaults(win: Window, cookies: string): void {
  const saved = savedGroups(cookies);
  const state = saved ? consentForGroups(saved) : DENIED;
  gtag(win, 'consent', 'default', { ...state, wait_for_update: 500 });
}

/** Called by CookiePro on load and on every change of the visitor's choice. */
export function installOptanonWrapper(win: Window): void {
  const w = win as TrackingWindow;
  w.OptanonWrapper = () => {
    const raw = w.OnetrustActiveGroups ?? w.OneTrustActiveGroups;
    // Not yet set (CookiePro still starting): leave the defaults. Once set, every
    // choice updates consent, a withdrawal included.
    if (typeof raw !== 'string') return;
    const groups = activeGroups(raw);
    gtag(win, 'consent', 'update', consentForGroups(groups));
  };
}
