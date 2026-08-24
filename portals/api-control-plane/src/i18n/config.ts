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

export const SUPPORTED_LOCALES = ['en', 'en-XA'] as const;
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

export const DEFAULT_LOCALE: SupportedLocale = 'en';
export const LOCALE_STORAGE_KEY = 'app.locale';

/**
 * Pseudo-locale for layout testing. `en-XA` is a longer, accented version of
 * `en`, supported via `?lang=en-XA` but hidden from the language switcher.
 */
export const PSEUDO_LOCALES = new Set<SupportedLocale>(['en-XA']);

/** Real languages, i.e. what a language switcher may offer. */
export const USER_SELECTABLE_LOCALES: readonly SupportedLocale[] =
  SUPPORTED_LOCALES.filter((locale) => !PSEUDO_LOCALES.has(locale));

// Locales that render right-to-left (add 'ar', 'he', etc. as needed)
export const RTL_LOCALES = new Set<SupportedLocale>([]);

export function isSupportedLocale(value: string): value is SupportedLocale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}