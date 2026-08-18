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

/**
 * Public i18n API. Import from `../i18n`; keep catalog, storage, and query-param
 * internals private. Import message APIs directly from `react-intl` so FormatJS
 * extraction and linting continue to recognize them.
 */

export {
  I18nProvider,
  useLocale,
  type Direction,
  type LocaleContextValue,
} from './I18nProvider';

export {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  PSEUDO_LOCALES,
  RTL_LOCALES,
  SUPPORTED_LOCALES,
  USER_SELECTABLE_LOCALES,
  isSupportedLocale,
  type SupportedLocale,
} from './config';

/**
 * Formats dates, times, and numbers using the active app locale.
 * Avoids falling back to the browser's locale.
 */
export {
  useFormatters,
  type DateLike,
  type Formatters,
} from './useFormatters';

/**
 * Resolves the locale for non-React callers; inside React, prefer `useLocale()`.
 */
export { detectLocale } from './detectLocale';
