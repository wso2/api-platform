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

import { DEFAULT_LOCALE, type SupportedLocale } from './config';
import {
  bundledCatalogFor,
  loadCatalogWithFallback,
  type MessageCatalog,
} from './loadCatalog';

// `de` isn't in SUPPORTED_LOCALES yet; the cast stands in for any locale that
// isn't the bundled one, which is the only case that takes the async path.
const NON_BUNDLED_LOCALE = 'de' as SupportedLocale;

describe('bundledCatalogFor', () => {
  it('covers the default locale, so first paint never waits on a chunk', () => {
    expect(bundledCatalogFor(DEFAULT_LOCALE)).not.toBeNull();
  });

  it('returns null for a locale that is not bundled', () => {
    expect(bundledCatalogFor(NON_BUNDLED_LOCALE)).toBeNull();
  });

  it('is the same object every call, so IntlProvider is not rebuilt', () => {
    expect(bundledCatalogFor(DEFAULT_LOCALE)).toBe(
      bundledCatalogFor(DEFAULT_LOCALE)
    );
  });
});

// Chunk 404s can happen for unrelated reasons, so this must end in text.
describe('loadCatalogWithFallback', () => {
  const catalogFor = (locale: string): MessageCatalog => ({
    [`test.${locale}`]: [{ type: 0, value: locale }] as MessageCatalog[string],
  });

  beforeEach(() => {
    // The fallback logs the failure; keep it out of the test output.
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('serves the bundled locale without loading a chunk at all', async () => {
    const load = vi.fn(async (locale: SupportedLocale) => catalogFor(locale));

    await expect(loadCatalogWithFallback(DEFAULT_LOCALE, load)).resolves.toBe(
      bundledCatalogFor(DEFAULT_LOCALE)
    );
    expect(load).not.toHaveBeenCalled();
  });

  it('loads a chunk for a non-bundled locale', async () => {
    const load = vi.fn(async (locale: SupportedLocale) => catalogFor(locale));

    await expect(
      loadCatalogWithFallback(NON_BUNDLED_LOCALE, load)
    ).resolves.toEqual(catalogFor(NON_BUNDLED_LOCALE));
    expect(load).toHaveBeenCalledTimes(1);
    expect(load).toHaveBeenCalledWith(NON_BUNDLED_LOCALE);
  });

  it('falls back to the bundled catalog when the chunk fails', async () => {
    const load = vi.fn(async () => {
      throw new Error('chunk 404');
    });

    // The bundled catalog can't 404 the way the chunk just did — that's the
    // point of falling back to it rather than to another lazy chunk.
    await expect(
      loadCatalogWithFallback(NON_BUNDLED_LOCALE, load)
    ).resolves.toBe(bundledCatalogFor(DEFAULT_LOCALE));
    expect(load).toHaveBeenCalledTimes(1);
  });
});
