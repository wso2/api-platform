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

import { ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useParams } from 'react-router-dom';

import {
  useApi,
  useOrganizations,
  useProject,
  useProjects,
} from '../api/hooks/useMvpQueries';
import { ApiScopeProvider } from '../api/core/ApiScopeProvider';
import { useAuth } from '../contexts/auth/AuthProvider';
import { getApiCapabilities } from '../pages/appShell/appShellPages/apis/apiCapabilities';
import {
  ConsoleScopeContext,
  type ConsoleRouteParams,
  type ConsoleScope,
} from './ConsoleScopeContext';

// Re-export so existing imports from this module keep working.
export {
  ConsoleScopeContext,
  useConsoleScope,
  type ConsoleRouteParams,
  type ConsoleScope,
} from './ConsoleScopeContext';

const getRouteParamsFromPathname = (pathname: string): ConsoleRouteParams => {
  const segments = pathname.split('/').filter(Boolean);
  const organizationsIndex = segments.indexOf('organizations');
  if (organizationsIndex < 0) return {};

  const orgHandle = segments[organizationsIndex + 1];
  const projectsIndex = segments.indexOf('projects');
  const projectHandler =
    projectsIndex >= 0 ? segments[projectsIndex + 1] : undefined;
  const apisIndex = segments.indexOf('apis');
  const apiHandler =
    apisIndex >= 0 ? segments[apisIndex + 1] : undefined;
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

export function ConsoleScopeProvider({ children }: { children: ReactNode }) {
  const routeParams = useParams<ConsoleRouteParams>();
  const location = useLocation();
  const { exchangeOrgToken, isAuthenticated } = useAuth();
  const [tokenReadyOrgHandle, setTokenReadyOrgHandle] = useState<string>();
  const [orgTokenError, setOrgTokenError] = useState<Error>();
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
  // `exchangeOrgToken` may not be referentially stable (it closes over Asgardeo
  // SDK functions). Read it through a ref and key the effect on stable
  // primitives only, so the exchange fires exactly once per org — never on
  // every render, which would recurse into repeated token-exchange calls.
  const exchangeOrgTokenRef = useRef(exchangeOrgToken);
  exchangeOrgTokenRef.current = exchangeOrgToken;

  useEffect(() => {
    let isMounted = true;

    if (!params.orgHandle || !isAuthenticated) {
      setTokenReadyOrgHandle(undefined);
      setOrgTokenError(undefined);
      return () => {
        isMounted = false;
      };
    }

    setTokenReadyOrgHandle(undefined);
    setOrgTokenError(undefined);
    exchangeOrgTokenRef
      .current(params.orgHandle)
      .then(() => {
        if (isMounted) setTokenReadyOrgHandle(params.orgHandle);
      })
      .catch((error) => {
        if (!isMounted) return;
        setOrgTokenError(
          error instanceof Error
            ? error
            : new Error('Unable to exchange organization token')
        );
      });

    return () => {
      isMounted = false;
    };
  }, [isAuthenticated, params.orgHandle]);

  const queryOrgHandle =
    tokenReadyOrgHandle === params.orgHandle ? params.orgHandle : undefined;
  const organizationsQuery = useOrganizations();
  const projectsQuery = useProjects(queryOrgHandle);
  const projectQuery = useProject(queryOrgHandle, params.projectHandler);
  const apiQuery = useApi(
    queryOrgHandle,
    params.projectHandler,
    params.apiHandler
  );

  const organization = useMemo(
    () =>
      organizationsQuery.data?.find((item) => item.handle === params.orgHandle),
    [organizationsQuery.data, params.orgHandle]
  );

  const project =
    projectQuery.data ||
    projectsQuery.data?.find((item) => item.handler === params.projectHandler);
  const component = apiQuery.data;
  const capabilities = useMemo(
    () => getApiCapabilities(component),
    [component]
  );

  const value = useMemo<ConsoleScope>(
    () => ({
      // Token-ready identifiers the data hooks default to (orgHandle only set
      // post token-exchange via queryOrgHandle), so context-aware queries never
      // fire before their bearer token is ready.
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
        Boolean(params.orgHandle && !tokenReadyOrgHandle && !orgTokenError) ||
        projectsQuery.isLoading ||
        projectQuery.isLoading ||
        apiQuery.isLoading,
      isOrganizationScope: Boolean(params.orgHandle),
      isProjectScope: Boolean(params.projectHandler),
      organization,
      organizations: organizationsQuery.data || [],
      params,
      project,
      projects: projectsQuery.data || [],
      projectsError: orgTokenError || projectsQuery.error || undefined,
    }),
    [
      capabilities,
      component,
      apiQuery.isLoading,
      organization,
      organizationsQuery.data,
      organizationsQuery.isLoading,
      orgTokenError,
      params,
      project,
      projectQuery.isLoading,
      projectsQuery.data,
      projectsQuery.error,
      projectsQuery.isLoading,
      queryOrgHandle,
      tokenReadyOrgHandle,
    ]
  );

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

        Note the ids differ from the ones above deliberately — the API layer
        wants the raw route params, not the token-gated `queryOrgHandle` the old
        hooks need, because it has no token exchange to wait on.
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
