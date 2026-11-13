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
 * Builds the Moesif actions the console sends. Moesif is a third party, so the
 * WSO2 Moesif data rules allow IDs, codes and enums only. Every field is set
 * here from an allowlist; nothing the browser or a page adds is passed through:
 *
 * - `request.uri` is the page URL reduced to its route template, e.g.
 *   `/organizations/:orgHandle/projects/:projectHandler/apis/new`, with the
 *   query string and fragment removed. Console URLs carry organization, project
 *   and API handles.
 * - The user and company IDs are sent only as UUIDs.
 * - The metadata is built from the taxonomy registry: the feature's declared
 *   dimensions, and only values in their enums.
 */

import { routes, SELECT_SCOPE_SEGMENT } from '@/routes/paths';
import { featureMetadata, type ApiPlatformFeatureCode, type FeatureDimensions } from './taxonomy';

/** Who an action is attributed to. Only UUIDs are sent; anything else is left out. */
export type FeatureUsageIdentity = {
  /** The IdP user ID (`sub`). */
  userId?: string;
  /** The organization UUID. */
  companyId?: string;
};

/**
 * One action, in the shape Moesif's Collection API (`POST /v1/actions/batch`)
 * expects. The same shape agent-manager sends through the moesif-collector.
 */
export type MoesifAction = {
  action_name: ApiPlatformFeatureCode;
  request: {
    time: string;
    uri: string;
    user_agent_string?: string;
  };
  user_id?: string;
  company_id?: string;
  session_token?: string;
  metadata: Record<string, string>;
};

/** Where and when the action happened, as the collector reads it from the page. */
export type ActionContext = {
  /** The page URL, before masking. */
  pageUrl: string;
  /** The deployment's base path, kept in the masked URL. */
  basePath?: string;
  time: Date;
  userAgent?: string;
  /** A UUID for this browser tab's session. */
  sessionToken?: string;
};

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const uuidOrUndefined = (value: unknown): string | undefined =>
  typeof value === 'string' && UUID_PATTERN.test(value) ? value : undefined;

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
// Actions
// ---------------------------------------------------------------------------

/** Builds one action for `code`. Absent or unsafe fields are left out, not sent empty. */
export function buildAction<C extends ApiPlatformFeatureCode>(
  code: C,
  dimensions: FeatureDimensions<C> | undefined,
  identity: FeatureUsageIdentity,
  context: ActionContext,
): MoesifAction {
  const action: MoesifAction = {
    action_name: code,
    request: {
      time: context.time.toISOString(),
      uri: maskUri(context.pageUrl, context.basePath),
    },
    metadata: featureMetadata(code, dimensions),
  };
  if (context.userAgent) action.request.user_agent_string = context.userAgent;
  const userId = uuidOrUndefined(identity.userId);
  if (userId) action.user_id = userId;
  const companyId = uuidOrUndefined(identity.companyId);
  if (companyId) action.company_id = companyId;
  const sessionToken = uuidOrUndefined(context.sessionToken);
  if (sessionToken) action.session_token = sessionToken;
  return action;
}
