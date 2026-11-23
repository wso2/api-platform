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

import { describe, expect, it } from 'vitest';
import { DENIED, activeGroups, consentForGroups, installOptanonWrapper, savedGroups, setConsentDefaults } from './consent';

const cookie = (groups: string) =>
  `a=b; OptanonConsent=${encodeURIComponent(`isGpcEnabled=0&groups=${groups}&hosts=`)}`;

describe('consent mapping', () => {
  it('denies everything without Performance or Targeting', () => {
    expect(consentForGroups(new Set(['C0001']))).toEqual(DENIED);
  });
  it('grants analytics for C0002 and ads for C0004', () => {
    expect(consentForGroups(new Set(['C0002']))).toMatchObject({ analytics_storage: 'granted', ad_storage: 'denied' });
    expect(consentForGroups(new Set(['C0004']))).toMatchObject({
      analytics_storage: 'denied',
      ad_storage: 'granted',
      ad_user_data: 'granted',
      ad_personalization: 'granted',
    });
  });
  it('reads the saved choice, and treats a missing or malformed cookie as none', () => {
    expect([...savedGroups(cookie('C0001:1,C0002:1,C0004:0'))!]).toEqual(['C0001', 'C0002']);
    expect(savedGroups('a=b')).toBeNull();
    expect(savedGroups('OptanonConsent=%E0%A4%A')).toBeNull();
  });
  it('parses CookiePro active groups', () => {
    expect([...activeGroups(',C0001,C0002,')]).toEqual(['C0001', 'C0002']);
    expect(activeGroups(undefined).size).toBe(0);
  });
});

describe('Google consent commands', () => {
  const layerOf = () => (window as unknown as { dataLayer: unknown[] }).dataLayer;
  const command = (index: number) => Array.from(layerOf()[index] as ArrayLike<unknown>);

  it('starts a new visitor denied, with arguments objects GTM understands', () => {
    (window as unknown as { dataLayer?: unknown[] }).dataLayer = [];
    setConsentDefaults(window, 'a=b');
    expect(command(0)).toEqual(['consent', 'default', { ...DENIED, wait_for_update: 500 }]);
    expect(Object.prototype.toString.call(layerOf()[0])).toBe('[object Arguments]');
  });
  it('starts a returning visitor from their saved choice', () => {
    (window as unknown as { dataLayer?: unknown[] }).dataLayer = [];
    setConsentDefaults(window, cookie('C0001:1,C0002:1,C0004:1'));
    expect(command(0)[2]).toMatchObject({ analytics_storage: 'granted', ad_storage: 'granted' });
  });
  it('updates consent from OptanonWrapper, a withdrawal included, and waits while groups are unset', () => {
    (window as unknown as { dataLayer?: unknown[] }).dataLayer = [];
    installOptanonWrapper(window);
    const w = window as unknown as { OptanonWrapper: () => void; OnetrustActiveGroups?: string };
    w.OnetrustActiveGroups = undefined;
    w.OptanonWrapper();
    expect(layerOf()).toHaveLength(0);
    w.OnetrustActiveGroups = ',C0001,C0002,C0004,';
    w.OptanonWrapper();
    expect(command(0).slice(0, 2)).toEqual(['consent', 'update']);
    expect(command(0)[2]).toMatchObject({ analytics_storage: 'granted', ad_storage: 'granted' });
    w.OnetrustActiveGroups = ',C0001,';
    w.OptanonWrapper();
    expect(command(1)[2]).toEqual(DENIED);
  });
});
