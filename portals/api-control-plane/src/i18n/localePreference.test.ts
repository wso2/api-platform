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

import { LOCALE_STORAGE_KEY } from './config';
import { readStoredLocale, writeStoredLocale } from './localePreference';

afterEach(() => {
  localStorage.clear();
});

describe('stored locale preference', () => {
  it('writes under the namespaced key only', () => {
    writeStoredLocale('en-XA');
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en-XA');
    expect(localStorage.getItem('app.locale')).toBeNull();
  });

  it('reads back the namespaced key', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'en');
    expect(readStoredLocale()).toBe('en');
  });

  it('ignores a preference left under the pre-namespace key', () => {
    localStorage.setItem('app.locale', 'en-XA');
    expect(readStoredLocale()).toBeNull();
  });

  it('returns null when nothing is stored', () => {
    expect(readStoredLocale()).toBeNull();
  });
});
