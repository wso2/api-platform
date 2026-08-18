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

import { runtimeConfig } from '../config/runtime';

import { DEFAULT_LOCALE, SupportedLocale, isSupportedLocale } from './config';
import { readStoredLocale } from './localePreference';

/** `?lang=de-DE`: overrides everything, for QA and translator review. */
const LOCALE_QUERY_PARAM = 'lang';


/*  A raw locale tag as it arrives from the outside world */
type LocaleCandidate = string | null | undefined;

/**
 * Reads a locale source, returning `null` when unavailable.
 * Precedence is handled by `detectLocale`.
 */
type LocaleSource = {
  /** Identifies the source in debugging/telemetry only. */
  readonly name: string;
  readonly read: () => LocaleCandidate;
};

/**
 * Removes `?lang=` so later loads use the stored locale.
 * Persists until the user explicitly switches locales.
 */
export function clearLocaleQueryParam(): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (!url.searchParams.has(LOCALE_QUERY_PARAM)) return;
  url.searchParams.delete(LOCALE_QUERY_PARAM);
  // Preserve existing history state for react-router.
  window.history.replaceState(window.history.state, '', url);
}

function fromQueryParam(): LocaleCandidate {
  if (typeof window === 'undefined') return null;
  // Don't persist this: shared links must not change the recipient's saved locale.
  return new URLSearchParams(window.location.search).get(LOCALE_QUERY_PARAM);
}

function fromStoredPreference(): LocaleCandidate {
  // Guarded inside ./localePreference, which the switcher writes through
  return readStoredLocale();
}

function fromNavigatorLanguages(): LocaleCandidate {
  if (typeof navigator === 'undefined') return null;
  const preferred = navigator.languages?.length
    ? navigator.languages
    : [navigator.language];
  // First supported tag wins (e.g., "fr, en-GB, en" picks en).
  for (const tag of preferred) {
    const match = negotiate(tag);
    if (match) return match;
  }
  return null;
}

/** Per-deployment default from `window.config.DEFAULT_LOCALE` or `VITE_DEFAULT_LOCALE`. */
function fromRuntimeConfig(): LocaleCandidate {
  const configured: unknown = runtimeConfig.defaultLocale;
  if (typeof configured !== 'string' || !configured.trim()) return null;
  return configured;
}

/** Highest precedence first. */
const LOCALE_SOURCES: readonly LocaleSource[] = [
  { name: 'queryParam', read: fromQueryParam },
  { name: 'storedPreference', read: fromStoredPreference },
  { name: 'navigatorLanguages', read: fromNavigatorLanguages },
  { name: 'runtimeConfig', read: fromRuntimeConfig },
];

/**
 * Resolves a raw tag to a supported locale, widening through subtags.
 * Returns `null` when malformed or unsupported.
 */
function negotiate(tag: LocaleCandidate): SupportedLocale | null {
  if (!tag) return null;

  let candidate: string | undefined;
  try {
    // Canonicalizes casing (e.g. `PT-br` → `pt-BR`); throws on invalid BCP 47.
    [candidate] = Intl.getCanonicalLocales(tag.trim());
  } catch {
    return null;
  }

  while (candidate) {
    if (isSupportedLocale(candidate)) return candidate;
    const lastSubtag = candidate.lastIndexOf('-');
    if (lastSubtag === -1) return null;
    candidate = candidate.slice(0, lastSubtag);
  }
  return null;
}

/**
 * Picks the first supported locale from:
 * `?lang=` → stored preference → `navigator.languages` → config → `en`.
 */
export function detectLocale(): SupportedLocale {
  for (const source of LOCALE_SOURCES) {
    const match = negotiate(source.read());
    if (match) return match;
  }
  return DEFAULT_LOCALE;
}
