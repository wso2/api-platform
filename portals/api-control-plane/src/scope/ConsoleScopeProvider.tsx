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
import { defineMessages, useIntl } from 'react-intl';
import { Navigate, useLocation, useParams } from 'react-router-dom';

import { ApiScopeProvider } from '../api/core/ApiScopeProvider';
import { ErrorState, LoadingState } from '../components/StateViews';
import { useAuth } from '../contexts/auth/AuthProvider';
import { OrganizationSwitchError } from '../contexts/auth/authTypes';
import { getApiCapabilities } from '../pages/appShell/appShellPages/apis/utils/apiCapabilities';
import {
  ConsoleScopeContext,
  type ConsoleRouteParams,
  type ConsoleScope,
} from './ConsoleScopeContext';
import { getRouteParamsFromPathname } from './consoleRouteParams';
import { useRestApi } from '../api/resources/restApis';
import { useGraphQLApi } from '../api/resources/graphqlApis';
import { useOrganizations } from '../api/resources/organizations';
import { useProject, useProjects } from '../api/resources/projects';
import { routes } from '../routes/paths';

// Re-export so existing imports from this module keep working.
export {
  ConsoleScopeContext,
  useConsoleScope,
  type ConsoleRouteParams,
  type ConsoleScope,
} from './ConsoleScopeContext';

const messages = defineMessages({
  switching: {
    id: 'apiControlPlane.scope.ConsoleScopeProvider.switching',
    defaultMessage: 'Switching organization',
    description: 'Shown while the session moves into the organization the user opened.',
  },
  switchFailedTitle: {
    id: 'apiControlPlane.scope.ConsoleScopeProvider.switchFailedTitle',
    defaultMessage: 'Unable to switch organization',
  },
  switchFailedMessage: {
    id: 'apiControlPlane.scope.ConsoleScopeProvider.switchFailedMessage',
    defaultMessage: 'Refresh the page to try again.',
  },
});

export function ConsoleScopeProvider({ children }: { children: ReactNode }) {
  const intl = useIntl();
  const routeParams = useParams<ConsoleRouteParams>();
  const location = useLocation();
  const { canSwitchOrganization, exchangeOrgToken, isAuthenticated, user } = useAuth();
  const sessionOrgHandle = user?.org?.handle;
  const [switchFailure, setSwitchFailure] = useState<{ orgHandle: string; error: unknown }>();
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
      graphqlApiHandler:
        routeParams.graphqlApiHandler || pathnameParams.graphqlApiHandler,
      orgHandle: routeParams.orgHandle || pathnameParams.orgHandle,
      projectHandler:
        routeParams.projectHandler || pathnameParams.projectHandler,
    }),
    [
      pathnameParams.apiHandler,
      pathnameParams.deploymentId,
      pathnameParams.environmentId,
      pathnameParams.graphqlApiHandler,
      pathnameParams.orgHandle,
      pathnameParams.projectHandler,
      routeParams.apiHandler,
      routeParams.deploymentId,
      routeParams.environmentId,
      routeParams.graphqlApiHandler,
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

    if (canSwitchOrganization || !params.orgHandle || !isAuthenticated) {
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
  }, [canSwitchOrganization, isAuthenticated, params.orgHandle]);

  // When the deployment switches organizations, the session is in one
  // organization at a time: the URL's is ready once the session is in it, and
  // until then the session is moved there. Keyed on the session's organization
  // too, so a session that lands back elsewhere (signed in again through
  // Native SSO, say) is moved again.
  useEffect(() => {
    const orgHandle = params.orgHandle;
    if (!canSwitchOrganization || !isAuthenticated || !orgHandle) return undefined;
    if (orgHandle === sessionOrgHandle) return undefined;
    let isCurrent = true;
    setSwitchFailure(undefined);
    exchangeOrgTokenRef.current(orgHandle).catch((error: unknown) => {
      if (isCurrent) setSwitchFailure({ orgHandle, error });
    });
    return () => {
      isCurrent = false;
    };
  }, [canSwitchOrganization, isAuthenticated, params.orgHandle, sessionOrgHandle]);

  const switchError =
    switchFailure && switchFailure.orgHandle === params.orgHandle ? switchFailure.error : undefined;

  const queryOrgHandle = canSwitchOrganization
    ? params.orgHandle && params.orgHandle === sessionOrgHandle
      ? params.orgHandle
      : undefined
    : tokenReadyOrgHandle === params.orgHandle
      ? params.orgHandle
      : undefined;

  const apiQuery = useRestApi(params.apiHandler, {orgId: queryOrgHandle });
  const graphqlApiQuery = useGraphQLApi(params.graphqlApiHandler, {orgId: queryOrgHandle });
  const organizationsQuery = useOrganizations();
  const projectsQuery = useProjects({}, {orgId: queryOrgHandle });
  const projectQuery = useProject(params.projectHandler, {orgId: queryOrgHandle });

  const organization = useMemo(
    () =>
      organizationsQuery.data?.list?.find((item) => item.id === params.orgHandle),
    [organizationsQuery.data?.list, params.orgHandle]
  );

  // The Platform API lists an organization for a user once they have used it,
  // so a list read before a switch can lack the one just switched to. Read it
  // once more for that organization.
  const refetchedFor = useRef<string | undefined>(undefined);
  const { refetch: refetchOrganizations } = organizationsQuery;
  const organizationsSettled = organizationsQuery.isSuccess && !organizationsQuery.isFetching;
  useEffect(() => {
    if (!canSwitchOrganization || !queryOrgHandle || organization || !organizationsSettled) return;
    if (refetchedFor.current === queryOrgHandle) return;
    refetchedFor.current = queryOrgHandle;
    void refetchOrganizations();
  }, [canSwitchOrganization, organization, organizationsSettled, queryOrgHandle, refetchOrganizations]);

  const project =
    projectQuery.data ||
    projectsQuery.data?.list?.find((item) => item.id === params.projectHandler);

  const component = apiQuery.data;
  const graphqlComponent = graphqlApiQuery.data;
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
      graphqlComponent,
      isApiScope: Boolean(params.apiHandler),
      isGraphQLApiScope: Boolean(params.graphqlApiHandler),
      isLoading:
        organizationsQuery.isLoading ||
        Boolean(params.orgHandle && !queryOrgHandle && !orgTokenError) ||
        projectsQuery.isLoading ||
        projectQuery.isLoading ||
        apiQuery.isLoading,
      isOrganizationScope: Boolean(params.orgHandle),
      isProjectScope: Boolean(params.projectHandler),
      organization,
      organizations: organizationsQuery.data?.list || [],
      params,
      project,
      projects: projectsQuery.data?.list || [],
      projectsError: orgTokenError || projectsQuery.error || undefined,
    }),
    [
      capabilities,
      component,
      graphqlComponent,
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
    ]
  );

  // The org in the URL is only a request: it can outlive the session that put
  // it there (a login return path, a bookmark, another tab left open across an
  // account switch). Once the signed-in user's organizations have settled and
  // the URL's org is not among them, send them to the organization picker,
  // which lands them on one they can actually reach. Waiting out `isFetching`
  // keeps a refetch after onboarding registers the org from bouncing the user
  // off it on stale data; a truncated list proves nothing, so it never redirects.
  if (canSwitchOrganization && isAuthenticated && params.orgHandle) {
    // Membership is the identity provider's to decide here: an organization the
    // session can't move into is foreign, whatever the Platform API lists.
    if (switchError instanceof OrganizationSwitchError && switchError.reason === 'forbidden') {
      return <Navigate to={routes.organizations} replace />;
    }
    if (switchError) {
      return (
        <ErrorState
          title={intl.formatMessage(messages.switchFailedTitle)}
          message={intl.formatMessage(messages.switchFailedMessage)}
        />
      );
    }
    // Nothing below may call the backend until the session is in this
    // organization: it would act on the previous one.
    if (!queryOrgHandle) {
      return <LoadingState fullScreen label={intl.formatMessage(messages.switching)} />;
    }
  }

  const organizationList = organizationsQuery.data?.list;
  const isForeignOrganization =
    !canSwitchOrganization &&
    Boolean(params.orgHandle) &&
    organizationsQuery.isSuccess &&
    !organizationsQuery.isFetching &&
    !organization &&
    (organizationsQuery.data?.pagination?.total ?? 0) <=
      (organizationList?.length ?? 0);

  if (isForeignOrganization) {
    return <Navigate to={routes.organizations} replace />;
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
