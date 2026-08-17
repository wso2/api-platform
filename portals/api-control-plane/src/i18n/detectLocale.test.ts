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

import { afterEach, describe, expect, it } from 'vitest';

import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  PSEUDO_LOCALES,
  SUPPORTED_LOCALES,
  USER_SELECTABLE_LOCALES,
} from './config';
import { clearLocaleQueryParam, detectLocale } from './detectLocale';

const setQuery = (search: string) =>
  window.history.replaceState({}, '', `/${search}`);

afterEach(() => {
  window.history.replaceState({}, '', '/');
  localStorage.clear();
});

describe('detectLocale precedence', () => {
  it('prefers ?lang= over a stored preference', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'en');
    setQuery('?lang=en-XA');

    // The override is the point of the QA link: it has to win on a machine that
    // already has a preference saved.
    expect(detectLocale()).toBe('en-XA');
  });

  it('accepts a hand-typed ?lang= in any casing', () => {
    setQuery('?lang=en-xa');

    expect(detectLocale()).toBe('en-XA');
  });

  it('falls through an unsupported ?lang= instead of failing', () => {
    setQuery('?lang=fr-CA');

    expect(detectLocale()).toBe(DEFAULT_LOCALE);
  });

  it('ignores a malformed ?lang= rather than throwing', () => {
    setQuery('?lang=not!a!tag');

    expect(detectLocale()).toBe(DEFAULT_LOCALE);
  });

  it('uses the stored preference when no override is present', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'en-XA');

    expect(detectLocale()).toBe('en-XA');
  });
});

describe('clearLocaleQueryParam', () => {
  it('drops only lang, so a switch stops the override winning on reload', () => {
    setQuery('?lang=en-XA&tab=deploy');

    clearLocaleQueryParam();

    expect(window.location.search).toBe('?tab=deploy');
    expect(detectLocale()).toBe(DEFAULT_LOCALE);
  });

  it('leaves the URL untouched when there is no override', () => {
    setQuery('?tab=deploy');

    clearLocaleQueryParam();

    expect(window.location.search).toBe('?tab=deploy');
  });
});

describe('pseudo-locale registration', () => {
  it('is reachable via ?lang= but never offered in a switcher', () => {
    for (const pseudo of PSEUDO_LOCALES) {
      expect(SUPPORTED_LOCALES).toContain(pseudo);
      expect(USER_SELECTABLE_LOCALES).not.toContain(pseudo);
    }
  });

  it('is never the default — a real user must not land in it', () => {
    expect(PSEUDO_LOCALES.has(DEFAULT_LOCALE)).toBe(false);
    expect(USER_SELECTABLE_LOCALES.length).toBeGreaterThan(0);
  });
});
