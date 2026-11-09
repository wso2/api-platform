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
 * The one place every Moesif request passes before it leaves the browser.
 *
 * moesif-browser-js adds data of its own to what we send. Most importantly,
 * track() always sets `request.uri` to the full page URL, and console URLs
 * carry organization, project and API handles. Moesif is a third party, so the
 * WSO2 Moesif data rules allow IDs, codes and enums only.
 *
 * This filter rebuilds every payload from an allowlist instead of deleting
 * known-bad fields, so a field the SDK starts sending in a later version is
 * dropped by default rather than leaked:
 *
 * - `request.uri` becomes the route template, e.g.
 *   `/organizations/:orgHandle/projects/:projectHandler/apis/new`, with the
 *   query string and fragment removed.
 * - User, company, anonymous, session and transaction IDs survive only as UUIDs.
 * - Action metadata is rebuilt from the taxonomy registry.
 * - Anything else, including every API-call capture endpoint, is dropped.
 *
 * It returns null when nothing in a request is safe to send.
 */

import { routes, SELECT_SCOPE_SEGMENT } from '@/routes/paths';
import { featureMetadata, isFeatureCode } from './taxonomy';

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const uuidOrUndefined = (value: unknown): string | undefined =>
  typeof value === 'string' && UUID_PATTERN.test(value) ? value : undefined;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/** Copies only the entries that have a value, so absent fields stay absent. */
const compact = <T extends Record<string, unknown>>(record: T): Partial<T> =>
  Object.fromEntries(
    Object.entries(record).filter(([, value]) => value !== undefined),
  ) as Partial<T>;

// ---------------------------------------------------------------------------
// URL masking
// ---------------------------------------------------------------------------

/**
 * Every route template the console declares, as segments, most static segments
 * first so a literal (`projects/new`) wins over a parameter (`projects/:id`).
 * Derived from `routes`, so a route added later is covered without a change
 * here.
 */
const ROUTE_TEMPLATES: string[][] = Object.values(routes)
  .map((route) => (typeof route === 'function' ? (route as () => string)() : route))
  .filter(
    (template): template is string => typeof template === 'string' && template.startsWith('/'),
  )
  .map((template) => template.split('/').filter(Boolean))
  .sort(
    (a, b) =>
      b.filter((segment) => !segment.startsWith(':')).length -
      a.filter((segment) => !segment.startsWith(':')).length,
  );

/** The static segments of every route, the only segments a fallback may keep. */
const STATIC_SEGMENTS = new Set([
  SELECT_SCOPE_SEGMENT,
  ...ROUTE_TEMPLATES.flat().filter((segment) => !segment.startsWith(':')),
]);

/**
 * The route template a console path belongs to. A path no template matches
 * (an alias, a route deeper than the map) keeps only known static segments and
 * masks the rest as `:id`: an unrecognized segment is far more likely to be a
 * handle than a new keyword, and guessing the other way leaks it.
 */
export function maskPath(pathname: string): string {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 0) return '/';
  for (const template of ROUTE_TEMPLATES) {
    if (
      template.length === segments.length &&
      template.every((segment, i) => segment.startsWith(':') || segment === segments[i])
    ) {
      return `/${template.join('/')}`;
    }
  }
  return `/${segments.map((segment) => (STATIC_SEGMENTS.has(segment) ? segment : ':id')).join('/')}`;
}

/**
 * A page URL with its path reduced to the route template and the query string
 * and fragment removed. The origin and the deployment's base path are kept:
 * they name the console, not a customer.
 */
export function maskUri(uri: unknown, basePath = ''): string {
  if (typeof uri !== 'string') return '/';
  let url: URL;
  try {
    url = new URL(uri);
  } catch {
    return '/';
  }
  let path = url.pathname;
  let prefix = '';
  if (basePath && (path === basePath || path.startsWith(`${basePath}/`))) {
    prefix = basePath;
    path = path.slice(basePath.length) || '/';
  }
  return `${url.origin}${prefix}${maskPath(path)}`;
}

// ---------------------------------------------------------------------------
// Payloads
// ---------------------------------------------------------------------------

function filterAction(raw: unknown, basePath: string): Record<string, unknown> | null {
  if (!isRecord(raw) || !isFeatureCode(raw.action_name)) return null;
  const request = isRecord(raw.request) ? raw.request : {};
  return compact({
    action_name: raw.action_name,
    request: compact({
      time: typeof request.time === 'string' ? request.time : undefined,
      uri: maskUri(request.uri, basePath),
      user_agent_string:
        typeof request.user_agent_string === 'string' ? request.user_agent_string : undefined,
      verb: 'GET',
    }),
    company_id: uuidOrUndefined(raw.company_id),
    user_id: uuidOrUndefined(raw.user_id),
    anonymous_id: uuidOrUndefined(raw.anonymous_id),
    session_token: uuidOrUndefined(raw.session_token),
    transaction_id: uuidOrUndefined(raw.transaction_id),
    metadata: featureMetadata(raw.action_name, isRecord(raw.metadata) ? raw.metadata : undefined),
  });
}

/** A user update keeps its IDs only: no metadata, no campaign data. */
function filterUser(raw: unknown): Record<string, unknown> | null {
  if (!isRecord(raw)) return null;
  const userId = uuidOrUndefined(raw.user_id);
  if (!userId) return null;
  return compact({
    user_id: userId,
    company_id: uuidOrUndefined(raw.company_id),
    anonymous_id: uuidOrUndefined(raw.anonymous_id),
    session_token: uuidOrUndefined(raw.session_token),
  });
}

/** A company update keeps its ID only: no name, domain, metadata or campaign data. */
function filterCompany(raw: unknown): Record<string, unknown> | null {
  if (!isRecord(raw)) return null;
  const companyId = uuidOrUndefined(raw.company_id);
  if (!companyId) return null;
  return compact({
    company_id: companyId,
    session_token: uuidOrUndefined(raw.session_token),
  });
}

const filterBatch = (
  raw: unknown,
  filterOne: (item: unknown) => Record<string, unknown> | null,
): Record<string, unknown>[] | null => {
  if (!Array.isArray(raw)) return null;
  const kept = raw.map(filterOne).filter((item): item is Record<string, unknown> => item !== null);
  return kept.length > 0 ? kept : null;
};

/**
 * The safe version of a request the SDK is about to send to `url`, or null to
 * drop it. Only the Collection API routes this app uses are allowed through;
 * API-call events (`/v1/events`) are never sent, since this app does not
 * capture its own API calls.
 */
export function filterMoesifRequest(url: string, data: unknown, basePath = ''): unknown {
  let pathname: string;
  try {
    pathname = new URL(url).pathname;
  } catch {
    return null;
  }
  const action = (item: unknown) => filterAction(item, basePath);
  switch (pathname) {
    case '/v1/actions':
      return action(data);
    case '/v1/actions/batch':
      return filterBatch(data, action);
    case '/v1/users':
      return filterUser(data);
    case '/v1/users/batch':
      return filterBatch(data, filterUser);
    case '/v1/companies':
      return filterCompany(data);
    case '/v1/companies/batch':
      return filterBatch(data, filterCompany);
    default:
      return null;
  }
}
