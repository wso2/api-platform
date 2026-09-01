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

import { SELECT_SCOPE_SEGMENT } from '../routes/paths';
import type { ConsoleRouteParams } from './ConsoleScopeContext';

/**
 * Segments that sit where a handle would but name a page, not a resource.
 *
 * `routes.newApi` is `.../apis/new`, so without this the create page reads back
 * as an API whose handle is the literal `new` — which would put a bogus entry in
 * the header's API switcher and the breadcrumb trail, flip `isApiScope` on while
 * the API does not exist yet, and fire a detail request for it. `select-scope` is
 * reserved for the same reason (`SELECT_SCOPE_SEGMENT`), though the builders
 * already keep it out of a handle position by construction.
 */
const RESERVED_HANDLE_SEGMENTS = new Set<string>([SELECT_SCOPE_SEGMENT, 'new']);

/**
 * The handle one segment past `marker`, or `undefined` when that slot is absent
 * or holds a reserved segment rather than a real handle.
 */
const handleAfter = (
  segments: string[],
  marker: string
): string | undefined => {
  const markerIndex = segments.indexOf(marker);
  if (markerIndex < 0) return undefined;

  const candidate = segments[markerIndex + 1];
  if (!candidate || RESERVED_HANDLE_SEGMENTS.has(candidate)) return undefined;
  return candidate;
};

/**
 * Reads scope handles out of a pathname positionally: the segment after
 * `organizations` is the org handle, the one after `projects` the project, the
 * one after `apis` the API.
 *
 * `ConsoleScopeProvider` needs this because it is mounted as a pathless layout
 * route, where `useParams()` sees only the params of the branch matched so far —
 * not the leaf page's `:projectHandler`/`:apiHandler`.
 *
 * Being positional, it also constrains what a URL may look like: a page's
 * scope-less alias must not leave a `projects`/`apis` segment with the page's own
 * suffix behind it, or that suffix is read back as a handle. This is exactly why
 * those aliases carry `SELECT_SCOPE_SEGMENT` instead of dropping segments — see
 * `projectPath` in `routes/paths.ts`, and the round-trip test beside it.
 *
 * A page whose suffix genuinely does sit in a handle slot — `routes.newApi`, at
 * `.../apis/new` — is excluded by `RESERVED_HANDLE_SEGMENTS` instead.
 */
export const getRouteParamsFromPathname = (
  pathname: string
): ConsoleRouteParams => {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.indexOf('organizations') < 0) return {};

  return {
    apiHandler: handleAfter(segments, 'apis'),
    deploymentId: handleAfter(segments, 'deployments'),
    environmentId: handleAfter(segments, 'environments'),
    orgHandle: handleAfter(segments, 'organizations'),
    projectHandler: handleAfter(segments, 'projects'),
  };
};
