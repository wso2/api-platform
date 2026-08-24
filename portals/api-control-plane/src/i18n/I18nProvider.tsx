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

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { IntlProvider, ReactIntlErrorCode } from 'react-intl';
import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  RTL_LOCALES,
  USER_SELECTABLE_LOCALES,
  type SupportedLocale,
} from './config';
import { clearLocaleQueryParam, detectLocale } from './detectLocale';
import { DISPLAY_TIME_ZONE, INTL_FORMATS } from './formats';
import {
  bundledCatalogFor,
  loadCatalogWithFallback,
  type MessageCatalog,
} from './loadCatalog';
import { writeStoredLocale } from './localePreference';

export type Direction = 'ltr' | 'rtl';

export interface LocaleContextValue {
  locale: SupportedLocale;
  setLocale: (next: SupportedLocale) => void;
  /** Every locale this build ships, so a language switcher renders its options from the context instead of importing the config module directly. */
  availableLocales: readonly SupportedLocale[];
  /** Writing direction of the active locale — mirrors `<html dir>`. */
  dir: Direction;
  /** True while a catalog is loading; children keep rendering the previous locale's messages. */
  isLoading: boolean;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

export function useLocale(): LocaleContextValue {
  const ctx = useContext(LocaleContext);
  if (!ctx) throw new Error('useLocale must be used within <I18nProvider>');
  return ctx;
}

type IntlErrorHandler = NonNullable<
  React.ComponentProps<typeof IntlProvider>['onError']
>;

/**
 * `MISSING_DATA` fires per formatted value, so dedupe by message to avoid
 * repeated reports for the same locale/data gap.
 */
const reportedMissingData = new Set<string>();

/**
 * Hoisted to avoid recreating `IntlProvider`'s memoized `intl` object on each
 * render, which would trigger unnecessary re-renders in all consumers.
 */
const handleIntlError: IntlErrorHandler = (err) => {
  switch (err.code) {
    // Expected during translation work; noisy in prod, useful in dev.
    case ReactIntlErrorCode.MISSING_TRANSLATION:
      if (import.meta.env.PROD) return;
      break;
    case ReactIntlErrorCode.MISSING_DATA:
      if (reportedMissingData.has(err.message)) return;
      reportedMissingData.add(err.message);
      break;
    default:
      // FORMAT_ERROR / INVALID_CONFIG / UNSUPPORTED_FORMATTER are real defects
      break;
  }
  console.error(err);
};

function directionFor(locale: SupportedLocale): Direction {
  return RTL_LOCALES.has(locale) ? 'rtl' : 'ltr';
}

function applyDocumentLocale(locale: SupportedLocale) {
  document.documentElement.lang = locale;
  document.documentElement.dir = directionFor(locale);
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<SupportedLocale>(detectLocale);
  const [messages, setMessages] = useState<MessageCatalog | null>(() => // Seed from the bundled catalog when available; only non-bundled locales start null.
    bundledCatalogFor(locale) 
  );
  const [isLoading, setIsLoading] = useState(() => messages === null);

  // Apply `lang`/`dir` before paint so the document stays correctly labeled,
  // even if the catalog fails to load.
  useLayoutEffect(() => {
    applyDocumentLocale(locale);
  }, [locale]);

  useEffect(() => {
    const bundled = bundledCatalogFor(locale);
    if (bundled) {
      // Already bundled: no request or loading state; stable identity avoids re-renders.
      setMessages(bundled);
      setIsLoading(false);
      return;
    }

    let cancelled = false;
    setIsLoading(true);
    // Falls back to the bundled catalog
    loadCatalogWithFallback(locale)
      .then((catalog) => {
        if (!cancelled) setMessages(catalog);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [locale]);

  // Re-resolve on cross-tab changes to preserve precedence and ignore invalid values.
  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      // A wholesale `localStorage.clear()` reports key === null.
      if (event.key !== null && event.key !== LOCALE_STORAGE_KEY) return;
      setLocaleState(detectLocale());
    };
    window.addEventListener('storage', handleStorage);
    return () => window.removeEventListener('storage', handleStorage);
  }, []);

  const setLocale = useCallback((next: SupportedLocale) => {
    // Best-effort persistence; errors won't block the switch
    writeStoredLocale(next);
    // Explicit choice retires ?lang= override which outranks storage
    clearLocaleQueryParam();
    setLocaleState(next);
  }, []);

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      setLocale,
      availableLocales: USER_SELECTABLE_LOCALES,
      dir: directionFor(locale),
      isLoading,
    }),
    [locale, setLocale, isLoading]
  );

  // Wait for the catalog to avoid flashing untranslated content.
  if (!messages) return null;

  return (
    <LocaleContext.Provider value={value}>
      <IntlProvider
        locale={locale}
        defaultLocale={DEFAULT_LOCALE}
        messages={messages}
        formats={INTL_FORMATS}
        timeZone={DISPLAY_TIME_ZONE}
        onError={handleIntlError}
      >
        {children}
      </IntlProvider>
    </LocaleContext.Provider>
  );
}