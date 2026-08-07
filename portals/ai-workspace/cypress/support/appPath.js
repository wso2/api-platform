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

/*
 * The app is served under a path prefix (`/ai-workspace` — src/paths.ts BASE_PATH,
 * bff/internal/paths.Base), and the router is mounted with that prefix as its
 * basename. So a route the app navigates to ('/organizations/default') is NOT what
 * window.location.pathname reads back: that carries the prefix too.
 *
 * cy.visit()/cy.request() need no help — Cypress joins them onto baseUrl, which
 * already includes the prefix — but any assertion on cy.location('pathname') does.
 * These helpers derive the prefix from baseUrl rather than hardcoding it, so a run
 * pointed at a deployment served under a different prefix (CYPRESS_BASE_URL) stays
 * correct.
 */

/** Path prefix the app is served under, without a trailing slash ('' if none). */
export function basePath() {
  const baseUrl = Cypress.config('baseUrl');
  return new URL(baseUrl).pathname.replace(/\/+$/, '');
}

/** Absolute `window.location.pathname` for a router route, e.g. '/login'. */
export function appPath(route) {
  return `${basePath()}${route}`;
}

/**
 * RegExp matching `window.location.pathname` for a router route pattern, anchored at
 * the start of the path: appPathPattern('/organizations/[^/]+$') matches
 * '/ai-workspace/organizations/default'. `routePattern` is regex source, so any
 * literal metacharacter in it must already be escaped by the caller.
 */
export function appPathPattern(routePattern, flags) {
  const escapedBase = basePath().replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`^${escapedBase}${routePattern}`, flags);
}
