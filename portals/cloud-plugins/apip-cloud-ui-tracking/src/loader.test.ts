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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { isTrackingStarted, resetTrackingForTests, startTracking } from './loader';

const config = { cookieProDomainScript: '01a1a5f4-f908-7fef-aa9a-e3d16219021b-test', gtmContainerId: 'GTM-ABC123' };

describe('startTracking', () => {
  beforeEach(() => {
    resetTrackingForTests();
    document.head.innerHTML = '';
    document.body.removeAttribute('data-clarity-mask');
    (window as unknown as { dataLayer?: unknown[] }).dataLayer = [];
  });

  it('loads only CookiePro and GTM, with the consent defaults ahead of GTM', () => {
    startTracking(config);
    const scripts = Array.from(document.head.querySelectorAll("script"));
    expect(scripts.map((s) => s.src)).toEqual([
      'https://cookie-cdn.cookiepro.com/scripttemplates/otSDKStub.js',
      'https://www.googletagmanager.com/gtm.js?id=GTM-ABC123',
    ]);
    expect(scripts[0].getAttribute('data-domain-script')).toBe(config.cookieProDomainScript);
    const layer = (window as unknown as { dataLayer: unknown[] }).dataLayer;
    expect(Array.from(layer[0] as ArrayLike<unknown>).slice(0, 2)).toEqual(['consent', 'default']);
    expect(layer[1]).toMatchObject({ event: 'gtm.js' });
    expect(typeof (window as unknown as { OptanonWrapper: unknown }).OptanonWrapper).toBe('function');
  });
  it('masks the page for Clarity and loads once', () => {
    startTracking(config);
    startTracking(config);
    expect(document.body.getAttribute('data-clarity-mask')).toBe('true');
    expect(document.head.querySelectorAll('script')).toHaveLength(2);
    expect(isTrackingStarted()).toBe(true);
  });

  describe('the floating cookie icon', () => {
    afterEach(() => {
      vi.unstubAllGlobals();
      delete (document as unknown as { adoptedStyleSheets?: unknown }).adoptedStyleSheets;
    });

    it('is hidden with a constructable stylesheet', () => {
      const replaceSync = vi.fn();
      class FakeSheet {
        replaceSync = replaceSync;
      }
      vi.stubGlobal('CSSStyleSheet', FakeSheet);
      (document as unknown as { adoptedStyleSheets: unknown[] }).adoptedStyleSheets = [];
      startTracking(config);
      expect(replaceSync).toHaveBeenCalledWith(expect.stringContaining('#ot-sdk-btn-floating'));
      expect((document as unknown as { adoptedStyleSheets: unknown[] }).adoptedStyleSheets).toHaveLength(1);
    });

    it('is left alone where constructable stylesheets are not supported', () => {
      expect(() => startTracking(config)).not.toThrow();
    });
  });
});
