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

import type { TrackingConfig } from './config';
import { installOptanonWrapper, setConsentDefaults } from './consent';

const COOKIEPRO_SRC = 'https://cookie-cdn.cookiepro.com/scripttemplates/otSDKStub.js';
const GTM_SRC = 'https://www.googletagmanager.com/gtm.js';

let started = false;

/** Whether `startTracking` has loaded GTM in this page. */
export const isTrackingStarted = (): boolean => started;

/**
 * Loads CookiePro and the GTM container, consent first: the Google defaults and
 * `OptanonWrapper` are in place before either script runs, so no Google tag
 * stores anything until the visitor accepts. Safe to call more than once.
 */
export function startTracking(config: TrackingConfig, win: Window = window): void {
  if (started) return;
  started = true;
  const doc = win.document;

  setConsentDefaults(win, doc.cookie);
  installOptanonWrapper(win);

  const cookiePro = doc.createElement('script');
  cookiePro.src = COOKIEPRO_SRC;
  cookiePro.type = 'text/javascript';
  cookiePro.charset = 'UTF-8';
  cookiePro.setAttribute('data-domain-script', config.cookieProDomainScript);
  doc.head.appendChild(cookiePro);

  const w = win as Window & { dataLayer?: unknown[] };
  w.dataLayer = w.dataLayer ?? [];
  w.dataLayer.push({ 'gtm.start': Date.now(), event: 'gtm.js' });
  const gtm = doc.createElement('script');
  gtm.async = true;
  gtm.src = `${GTM_SRC}?id=${encodeURIComponent(config.gtmContainerId)}`;
  doc.head.appendChild(gtm);

  // Clarity (set up inside the container) masks all page text in recordings.
  doc.body.setAttribute('data-clarity-mask', 'true');
}

/** For tests. */
export function resetTrackingForTests(): void {
  started = false;
}
