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

import { LEGACY_LOCALE_STORAGE_KEY, LOCALE_STORAGE_KEY } from './config';
import { readStoredLocale, writeStoredLocale } from './localePreference';

afterEach(() => {
  localStorage.clear();
});

describe('stored locale preference', () => {
  it('writes under the namespaced key only', () => {
    writeStoredLocale('en-XA');
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en-XA');
    expect(localStorage.getItem(LEGACY_LOCALE_STORAGE_KEY)).toBeNull();
  });

  it('prefers the namespaced key over the legacy one', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'en');
    localStorage.setItem(LEGACY_LOCALE_STORAGE_KEY, 'en-XA');
    expect(readStoredLocale()).toBe('en');
  });

  it('adopts a legacy preference once and leaves the legacy key alone', () => {
    localStorage.setItem(LEGACY_LOCALE_STORAGE_KEY, 'en-XA');
    expect(readStoredLocale()).toBe('en-XA');
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en-XA');
    expect(localStorage.getItem(LEGACY_LOCALE_STORAGE_KEY)).toBe('en-XA');
  });

  it('returns null when nothing is stored', () => {
    expect(readStoredLocale()).toBeNull();
  });
});
