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

import type { ConsoleRouteParams } from './ConsoleScopeContext';

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
 */
export const getRouteParamsFromPathname = (
  pathname: string
): ConsoleRouteParams => {
  const segments = pathname.split('/').filter(Boolean);
  const organizationsIndex = segments.indexOf('organizations');
  if (organizationsIndex < 0) return {};

  const orgHandle = segments[organizationsIndex + 1];
  const projectsIndex = segments.indexOf('projects');
  const projectHandler =
    projectsIndex >= 0 ? segments[projectsIndex + 1] : undefined;
  const apisIndex = segments.indexOf('apis');
  const apiHandler = apisIndex >= 0 ? segments[apisIndex + 1] : undefined;
  const environmentsIndex = segments.indexOf('environments');
  const environmentId =
    environmentsIndex >= 0 ? segments[environmentsIndex + 1] : undefined;
  const deploymentsIndex = segments.indexOf('deployments');
  const deploymentId =
    deploymentsIndex >= 0 ? segments[deploymentsIndex + 1] : undefined;

  return {
    apiHandler,
    deploymentId,
    environmentId,
    orgHandle,
    projectHandler,
  };
};
