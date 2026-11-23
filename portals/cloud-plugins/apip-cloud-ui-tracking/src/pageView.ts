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
 * GA4 page views as a data-layer event carrying the route *pattern*, never the
 * real path: detail pages put organization, project and API handles in the URL.
 * Same event shape as the WSO2 Cloud console's `virtual_page_view`, so the shared
 * container's GA4 tag reads it the same way.
 */

export const PAGE_TITLE = 'API Platform Console';
export const PRODUCT = 'api-platform';
export const NOT_FOUND_PATTERN = '/(not-found)';

export type PatternMatcher = (pattern: string, pathname: string) => boolean;

/** Most specific first: fewer `:params`, so `/gateways/new` beats `/gateways/:gatewayId`. */
export function sortPatterns(patterns: readonly string[]): string[] {
  const params = (pattern: string) => (pattern.match(/:/g) ?? []).length;
  return [...new Set(patterns)].sort((a, b) => params(a) - params(b) || b.length - a.length);
}

export function routePattern(
  pathname: string,
  sortedPatterns: readonly string[],
  matches: PatternMatcher
): string {
  return sortedPatterns.find((pattern) => matches(pattern, pathname)) ?? NOT_FOUND_PATTERN;
}

/** The origin of a referrer from outside this site; empty for the same site or none. */
export function outsideReferrerOrigin(referrer: string, ownOrigin: string): string {
  if (!referrer) return '';
  try {
    const origin = new URL(referrer).origin;
    return origin === ownOrigin ? '' : origin;
  } catch {
    return '';
  }
}

export function pageViewEvent(pattern: string, referrer: string, ownOrigin: string) {
  return {
    event: 'virtual_page_view',
    page_location: `${ownOrigin}${pattern}`,
    page_path: pattern,
    page_referrer: outsideReferrerOrigin(referrer, ownOrigin),
    page_title: PAGE_TITLE,
    product: PRODUCT,
  };
}

export function pushPageView(win: Window, pattern: string): void {
  const w = win as Window & { dataLayer?: unknown[] };
  w.dataLayer = w.dataLayer ?? [];
  w.dataLayer.push(pageViewEvent(pattern, win.document.referrer, win.location.origin));
}
