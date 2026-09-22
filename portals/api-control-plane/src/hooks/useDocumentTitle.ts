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

import { useEffect } from 'react';

/**
 * The product name every tab title ends with.
 *
 * A brand name, so it is deliberately not a translatable message — and it is
 * kept in step with the `<title>` in `index.html`, which is what the tab shows
 * for the split second before React mounts.
 */
export const APP_TITLE = 'WSO2 API Platform';

/** `"APIs | WSO2 API Platform"`, or just the product name for a page with no name of its own. */
export const formatDocumentTitle = (pageTitle?: string): string =>
  pageTitle ? `${pageTitle} | ${APP_TITLE}` : APP_TITLE;

/**
 * Writes the browser tab title for the current page.
 *
 * Call this **once per rendered route** — `AppLayout` does it for every page
 * inside the app shell (see `usePageTitle`), and the few pages that render
 * outside the shell (login, the error pages) call it themselves. A second call
 * from a component nested inside a page would race the first: effects run
 * child-before-parent, so the outer title would win on mount and the inner one
 * on every later change.
 *
 * Passing `undefined` falls back to the bare product name rather than leaving
 * the previous page's title in the tab.
 */
export function useDocumentTitle(pageTitle?: string): void {
  useEffect(() => {
    document.title = formatDocumentTitle(pageTitle);
  }, [pageTitle]);
}
