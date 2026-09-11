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

import { ReactNode, useMemo } from 'react';
import { useLocation, useParams } from 'react-router-dom';

import { ApiScopeProvider } from '../api/core/ApiScopeProvider';
import { useAuth } from '../contexts/auth/AuthProvider';
import { OrganizationAccessDeniedPage } from '../pages/appShell/appShellPages/system/SystemPages';
import { getApiCapabilities } from '../pages/appShell/appShellPages/apis/utils/apiCapabilities';
import {
  ConsoleScopeContext,
  type ConsoleRouteParams,
  type ConsoleScope,
} from './ConsoleScopeContext';
import { getRouteParamsFromPathname } from './consoleRouteParams';
import { useRestApi } from '../api/resources/restApis';
import { useOrganizations } from '../api/resources/organizations';
import { useProject, useProjects } from '../api/resources/projects';

// Re-export so existing imports from this module keep working.
export {
  ConsoleScopeContext,
  useConsoleScope,
  type ConsoleRouteParams,
  type ConsoleScope,
} from './ConsoleScopeContext';

/**
 * This console's BFF forwards one bearer token per session (see
 * `AuthProvider`) — there is no per-org token exchange, so a session is
 * scoped to exactly one organization for its whole lifetime. That organization
 * is resolved server-side from the token's own claims and exposed as
 * `user.org` on `/api/session` (see `bff/internal/session/claims.go`).
 *
 * The route, however, carries its own `:orgHandle` segment — and nothing
 * stops a link from naming a *different* organization than the session's own.
 * Because `organizations`/`projects`/`environments` on platform-api don't
 * accept an explicit org id (they implicitly answer "for whichever org this
 * bearer token belongs to"), blindly trusting `params.orgHandle` to label
 * whatever those calls return is what let the console show your own org's
 * data under someone else's org handle. So this provider checks the route's
 * org handle against `user.org.handle` *before* anything else, and renders an
 * access-denied page in place of the whole app shell on a mismatch — never a
 * per-page gate, since every org-scoped page's data ultimately traces back to
 * this same session-bound org.
 */
export function ConsoleScopeProvider({ children }: { children: ReactNode }) {
  const routeParams = useParams<ConsoleRouteParams>();
  const location = useLocation();
  const { user } = useAuth();
  const pathnameParams = useMemo(
    () => getRouteParamsFromPathname(location.pathname),
    [location.pathname]
  );
  const params = useMemo<ConsoleRouteParams>(
    () => ({
      apiHandler:
        routeParams.apiHandler || pathnameParams.apiHandler,
      deploymentId: routeParams.deploymentId || pathnameParams.deploymentId,
      environmentId: routeParams.environmentId || pathnameParams.environmentId,
      orgHandle: routeParams.orgHandle || pathnameParams.orgHandle,
      projectHandler:
        routeParams.projectHandler || pathnameParams.projectHandler,
    }),
    [
      pathnameParams.apiHandler,
      pathnameParams.deploymentId,
      pathnameParams.environmentId,
      pathnameParams.orgHandle,
      pathnameParams.projectHandler,
      routeParams.apiHandler,
      routeParams.deploymentId,
      routeParams.environmentId,
      routeParams.orgHandle,
      routeParams.projectHandler,
    ]
  );

  const organizationsQuery = useOrganizations();

  // `user.org.handle` (a JWT claim resolved server-side, see
  // `bff/internal/session/claims.go`) is read exactly once, here, and used
  // for both the access check below and the recovery destination passed to
  // `OrganizationAccessDeniedPage` — never re-derived independently in a
  // second place. Two independent reads of "the session's own org handle"
  // are only guaranteed to agree by convention, not by anything the type
  // system enforces; a page that read it separately could silently drift
  // from what this check actually verified.
  const sessionOrgHandle = user?.org?.handle;

  // A session with no `org` claim at all (basic/file-based auth, which has no
  // notion of multiple organizations — see `AuthProvider`) has nothing to
  // mismatch against, so it isn't gated here. Only a *known* session org that
  // disagrees with the route is treated as denied.
  const orgAccessDenied = Boolean(
    params.orgHandle && sessionOrgHandle && params.orgHandle !== sessionOrgHandle
  );

  // Only ever query with an org handle the session actually owns — never the
  // raw route param — so a mismatched route can't leak a real request out
  // under the wrong label.
  const queryOrgHandle = orgAccessDenied ? undefined : params.orgHandle;

  const apiQuery = useRestApi(params.apiHandler, {orgId: queryOrgHandle });
  const projectsQuery = useProjects({}, {orgId: queryOrgHandle });
  const projectQuery = useProject(params.projectHandler, {orgId: queryOrgHandle });

  const organization = useMemo(
    () =>
      organizationsQuery.data?.list?.find((item) => item.id === params.orgHandle),
    [organizationsQuery.data?.list, params.orgHandle]
  );

  const project =
    projectQuery.data ||
    projectsQuery.data?.list?.find((item) => item.id === params.projectHandler);

  const component = apiQuery.data;
  const capabilities = useMemo(
    () => getApiCapabilities(component),
    [component]
  );

  const value = useMemo<ConsoleScope>(
    () => ({
      activeScope: {
        orgHandle: queryOrgHandle,
        projectHandler: params.projectHandler,
        apiHandler: params.apiHandler,
      },
      capabilities,
      component,
      isApiScope: Boolean(params.apiHandler),
      isLoading:
        organizationsQuery.isLoading ||
        projectsQuery.isLoading ||
        projectQuery.isLoading ||
        apiQuery.isLoading,
      isOrganizationScope: Boolean(params.orgHandle),
      isProjectScope: Boolean(params.projectHandler),
      orgAccessDenied,
      organization,
      organizations: organizationsQuery.data?.list || [],
      params,
      project,
      projects: projectsQuery.data?.list || [],
      projectsError: projectsQuery.error || undefined,
    }),
    [
      capabilities,
      component,
      apiQuery.isLoading,
      orgAccessDenied,
      organization,
      organizationsQuery.data,
      organizationsQuery.isLoading,
      params,
      project,
      projectQuery.isLoading,
      projectsQuery.data,
      projectsQuery.error,
      projectsQuery.isLoading,
      queryOrgHandle,
    ]
  );

  if (orgAccessDenied) {
    return <OrganizationAccessDeniedPage myOrgHandle={sessionOrgHandle} />;
  }

  return (
    <ConsoleScopeContext.Provider value={value}>
      {/*
        Bridges route scope into the new API layer, whose hooks read
        `ApiScopeContext` and stay gated until an organization is known.

        Mounted here, inside this provider, because this component already owns
        the (currently two-source) route-param derivation. That is transitional:
        once the contexts are split properly, `ApiScopeProvider` moves above
        this one and takes its ids straight from the router, and this nesting
        goes away.
      */}
      <ApiScopeProvider
        orgId={params.orgHandle}
        projectId={params.projectHandler}
      >
        {children}
      </ApiScopeProvider>
    </ConsoleScopeContext.Provider>
  );
}
