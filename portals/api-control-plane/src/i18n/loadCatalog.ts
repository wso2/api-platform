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

import type { MessageFormatElement } from 'react-intl';

// Keep the common locale statically imported: first render needs no request.
// The literal path is required; see `BUNDLED_LOCALE`. The glob also matches
// this file, so the build's dynamic-import warning is expected and intentional.

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore : This file is generated at build time via pre-scripts
import bundledJson from './compiled/en.json';

import type { SupportedLocale } from './config';

/**
 * Compiled catalog shape: message IDs map to ICU AST nodes.
 * `--ast` makes these `MessageFormatElement[]`; without it, they're strings.
 * Both forms are supported by `IntlProvider`.
 */
export type MessageCatalog = Record<string, MessageFormatElement[]>;

/**
 * Locale compiled into the entry bundle; keep in sync with `DEFAULT_LOCALE`.
 * Static imports can't be interpolated.
 */
const BUNDLED_LOCALE: SupportedLocale = 'en';

// Compilation guarantees the shape; JSON inference doesn't match the AST union.
const bundledCatalog = bundledJson as unknown as MessageCatalog;

/**
 * Returns the bundled catalog for `locale`, or `null` if unavailable.
 * Preserves object identity to avoid rebuilding `IntlProvider`'s `intl`.
 */
export function bundledCatalogFor(
  locale: SupportedLocale
): MessageCatalog | null {
  return locale === BUNDLED_LOCALE ? bundledCatalog : null;
}

// Vite and webpack both resolve this dynamic template import into
// one lazy chunk per matching file under compiled-lang/.
export async function loadCatalog(
  locale: SupportedLocale
): Promise<MessageCatalog> {
  const mod = await import(`./compiled/${locale}.json`);
  // Dynamic paths are untyped here; the compile step guarantees the shape.
  return mod.default as MessageCatalog;
}

/**
 * Load a catalog for `locale`, preferring a lazy chunk but falling back to
 * the bundled catalog on error. `load` is injectable for testing.
 */
export async function loadCatalogWithFallback(
  locale: SupportedLocale,
  load: (locale: SupportedLocale) => Promise<MessageCatalog> = loadCatalog
): Promise<MessageCatalog> {
  const bundled = bundledCatalogFor(locale);
  if (bundled) return bundled;

  try {
    return await load(locale);
  } catch (error) {
    console.error(
      `i18n: could not load the "${locale}" message catalog, falling back to "${BUNDLED_LOCALE}"`,
      error
    );
    return bundledCatalog;
  }
}