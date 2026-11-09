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

import { Box, ComplexSelect, Header, IconButton } from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { useIntl } from 'react-intl';
import { useLocation, useNavigate } from 'react-router-dom';

import { useAllGraphQLApis } from '../../api/resources/graphqlApis';
import { useAllRestApis } from '../../api/resources/restApis';
import SearchableComplexSelect from '../../components/common/SearchableComplexSelect';
import { findTargetAtScope } from '../../navigation/navigationRegistry';
import { toGraphQLListableApi, toRestListableApi } from './appShellPages/apis/listing/apiListItem';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import APIQuickSelector from './APIQuickSelector';
import ProjectQuickSelector from './ProjectQuickSelector';

// Switcher options can carry long display names/handles; bound the trigger width
// and let the option text ellipsize instead of overflowing the header.
const SWITCHER_SELECT_SX = {
  minWidth: 220,
  maxWidth: 260,
  // Keep the trigger compact (~53px); the 0.23 default padding made it ~63px.
  '& .MuiSelect-select.MuiSelect-outlined.MuiInputBase-input': {
    paddingTop: '5px !important',
    paddingBottom: '5px !important',
  },
};

// Selectors with a clear (X) button: the X sits at the top-right corner and the
// chevron drops to the bottom-right, both on the same right edge, so they never overlap.
const SWITCHER_SELECT_WITH_CLEAR_SX = {
  ...SWITCHER_SELECT_SX,
  '& .MuiSelect-icon': { top: 'auto', bottom: -6, right: 10 },
};

const TRUNCATED_OPTION_TEXT_SLOT_PROPS = {
  primary: { noWrap: true },
  secondary: { variant: 'caption' as const, noWrap: true },
};

/**
 * The organization / project / API switchers in the header.
 *
 * Split out of `AppHeader` so it can sit behind its own error boundary. Every
 * scope-dependent hook and every derivation over that data lives here, which is
 * the point: those run before any JSX is returned, so a boundary placed inside
 * the header's markup could never catch them. Keeping them in a child component
 * is what makes the failure containable — the brand, the notification bell and
 * the user menu's logout stay reachable when this throws.
 *
 * It owns the `params.orgHandle` guard too. Lifting that back into `AppHeader`
 * would drag `useConsoleScope()` up with it and put the throw back outside the
 * boundary.
 */
export function HeaderScopeSwitchers() {
  const navigate = useNavigate();
  const location = useLocation();
  const intl = useIntl();
  const scope = useConsoleScope();
  const {
    component,
    organization,
    organizations,
    params,
    project,
    projects,
    isLoading,
    projectsError,
  } = scope;

  // Every switcher action below re-resolves the *current page* at the new
  // scope (e.g. picking a project while on the organization's Settings tab
  // lands on that project's Settings, not its Overview; clearing the project
  // back out returns to the organization's Settings, not its Overview)
  // instead of always bouncing to that scope's Home — see `findTargetAtScope`.
  // Each falls back to its own Home route when the current page has no
  // equivalent at the new scope (an API-only page, say, cleared down to the
  // project that owns it).
  const changeOrganization = (orgHandle: string) => {
    if (!orgHandle || orgHandle === params.orgHandle) return;
    const target = findTargetAtScope(location.pathname, scope, { orgHandle });
    navigate(target ?? routes.organizationHome(orgHandle));
  };

  const changeProject = (projectHandler: string) => {
    if (!params.orgHandle || !projectHandler) return;
    const target = findTargetAtScope(location.pathname, scope, {
      orgHandle: params.orgHandle,
      projectHandler,
    });
    navigate(target ?? routes.projectHome(params.orgHandle, projectHandler));
  };

  const changeApi = (apiHandler: string) => {
    if (!params.orgHandle || !params.projectHandler || !apiHandler) return;
    const isGraphQLApi = graphqlApiHandlers.has(apiHandler);
    const scopeParams = { orgHandle: params.orgHandle, projectHandler: params.projectHandler };
    // The current page at the new API, as for the switchers above. A GraphQL
    // API only takes that target when the page has a GraphQL equivalent —
    // a REST-only page (the Definition editor, say) resolves to a REST path,
    // which a GraphQL handle would not open, so it falls back to the API's Home.
    const target = findTargetAtScope(
      location.pathname,
      scope,
      isGraphQLApi ? { ...scopeParams, graphqlApiHandler: apiHandler } : { ...scopeParams, apiHandler },
    );
    if (isGraphQLApi) {
      const graphqlHome = routes.graphqlApi(params.orgHandle, params.projectHandler, apiHandler);
      navigate(target?.startsWith(graphqlHome) ? target : graphqlHome);
      return;
    }
    navigate(target ?? routes.api(params.orgHandle, params.projectHandler, apiHandler));
  };

  const clearProjectSelection = () => {
    if (!params.orgHandle) return;
    const target = findTargetAtScope(location.pathname, scope, { orgHandle: params.orgHandle });
    navigate(target ?? routes.organizationHome(params.orgHandle));
  };

  const clearApiSelection = () => {
    if (!params.orgHandle || !params.projectHandler) return;
    const target = findTargetAtScope(location.pathname, scope, {
      orgHandle: params.orgHandle,
      projectHandler: params.projectHandler,
    });
    navigate(target ?? routes.projectHome(params.orgHandle, params.projectHandler));
  };

  // organizations may not be loaded yet on first paint; keep the current org
  // selectable so the switcher never renders an out-of-range value.
  const orgOptions: { handle: string; name: string }[] =
    organizations.length > 0
      ? organizations.map((org) => ({ handle: org.id, name: org.displayName || org.id }))
      : params.orgHandle
        ? [{ handle: params.orgHandle, name: organization?.displayName || params.orgHandle }]
        : [];

  const projectOptions: { handler: string; name: string; description?: string }[] =
    projects.length > 0
      ? projects.map((project) => ({
          handler: project.id,
          name: project.displayName || project.id,
          description: project.description,
        }))
      : params.projectHandler && project
        ? [{ handler: params.projectHandler, name: project.displayName || params.projectHandler, description: project.description }]
        : [];

  const restApisQuery = useAllRestApis({}, { projectId: project?.id, orgId: organization?.id });
  const graphqlApisQuery = useAllGraphQLApis(
    {},
    { projectId: project?.id, orgId: organization?.id },
  );
  const apisLoading = restApisQuery.isPending || graphqlApisQuery.isPending;
  const apisErrorValue = restApisQuery.error ?? graphqlApisQuery.error;
  const apis = [
    ...(restApisQuery.data?.list ?? []).map(toRestListableApi),
    ...(graphqlApisQuery.data?.list ?? []).map(toGraphQLListableApi),
  ];
  // The switcher only ever navigates by handler, so this is the one place
  // that needs to remember which handlers are GraphQL — everything else
  // downstream just carries `{handler, name}` through, same as before.
  const graphqlApiHandlers = new Set(
    (graphqlApisQuery.data?.list ?? [])
      .map((api) => api.id)
      .filter((id): id is string => typeof id === 'string' && id.length > 0),
  );
  const loadedApiOptions: { handler: string; name: string; description?: string }[] =
    (projects.length > 0 && project) ?
    apis
      .filter(
        (
          api
        ): api is NonNullable<typeof api> & { id: string; displayName?: string; description?: string } =>
          typeof api?.id === 'string' && api.id.length > 0
      )
      .map((api) => ({ handler: api.id, name: api.displayName ?? api.id, description: api.description }))
    : [];

  // Whichever kind of API is in scope — REST's `apiHandler` or GraphQL's
  // `graphqlApiHandler` (see `graphqlApiPath`'s doc comment for why they're
  // separate params) — is "the selected API" for this switcher; the two are
  // never both set at once.
  const selectedApiHandler = params.apiHandler || params.graphqlApiHandler;

  // apis may not be loaded yet on first paint; keep the current API selectable
  // so the switcher never renders an out-of-range value.
  const apiOptions: { handler: string; name: string; description?: string }[] =
    loadedApiOptions.length > 0
      ? loadedApiOptions
      : selectedApiHandler
        ? [
            {
              handler: selectedApiHandler,
              name: component?.displayName || selectedApiHandler,
              description: component?.description,
            },
          ]
        : [];

  // A single loaded organization is read-only; use the loaded list so the
  // switcher is not locked while the real list is still loading.
  const isSingleOrganization = organizations.length === 1;

  if (!params.orgHandle) return null;

  return (
    <Header.Switchers showDivider={false}>
      <SearchableComplexSelect
            aria-label={intl.formatMessage({ id: 'appShell.header.org.aria', defaultMessage: 'Organizations' })}
            label={intl.formatMessage({ id: 'appShell.header.org.label', defaultMessage: 'Organizations' })}
            readOnly={isSingleOrganization}
            value={params.orgHandle}
            selectedOption={orgOptions.filter((item) => item.handle === params.orgHandle).map(item => ({
              id: item.handle,
              handler: item.handle,
              name: item.name,
            }))[0] || undefined}
            onChange={(id) => {
              changeOrganization(id);
            }}
            options={orgOptions.map((item) => ({
              id: item.handle,
              handler: item.handle,
              name: item.name,
            }))}
            renderOptionContent={(option) => (
              <ComplexSelect.MenuItem.Text
                primary={option.name}
                secondary={option.id}
                slotProps={TRUNCATED_OPTION_TEXT_SLOT_PROPS}
              />
            )}
            searchPlaceholder={intl.formatMessage({ id: 'appShell.header.org.placeholder', defaultMessage: 'Search organizations...' })}
            emptyMessage={intl.formatMessage({ id: 'appShell.header.org.empty', defaultMessage: 'No organizations found' })}
            noResultsMessage={intl.formatMessage({ id: 'appShell.header.org.noResults', defaultMessage: 'No matching organizations' })}
            sx={SWITCHER_SELECT_SX}
          />

      {params.projectHandler && (
        <Box sx={{position: 'relative'}}>
          <SearchableComplexSelect
            aria-label={intl.formatMessage({ id: 'appShell.header.project.aria', defaultMessage: 'Projects' })}
            label={intl.formatMessage({ id: 'appShell.header.project.label', defaultMessage: 'Projects' })}
            value={params.projectHandler}
            selectedOption={projectOptions.filter((item) => item.handler === params.projectHandler).map(item => ({
              id: item.handler,
              handler: item.handler,
              name: item.name,
              description: item.description,
            }))[0] || undefined}
            onChange={(id) => {
              changeProject(id);
            }}
            options={projectOptions.map((item) => ({
              id: item.handler,
              handler: item.handler,
              name: item.name,
              description: item.description,
            }))}
            renderOptionContent={(option) => (
              <ComplexSelect.MenuItem.Text
                primary={option.name}
                secondary={option.description || option.id}
                slotProps={TRUNCATED_OPTION_TEXT_SLOT_PROPS}
              />
            )}
            searchPlaceholder={intl.formatMessage({ id: 'appShell.header.project.placeholder', defaultMessage: 'Search projects...' })}
            emptyMessage={intl.formatMessage({ id: 'appShell.header.project.empty', defaultMessage: 'No projects found' })}
            noResultsMessage={intl.formatMessage({ id: 'appShell.header.project.noResults', defaultMessage: 'No matching projects' })}
            sx={SWITCHER_SELECT_WITH_CLEAR_SX}
          />

          <IconButton
            size="small"
            aria-label={intl.formatMessage({ id: 'appShell.header.project.goToOrganization', defaultMessage: 'Go to organization level' })}
            onMouseDown={(event) => {
              event.preventDefault();
              event.stopPropagation();
            }}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              clearProjectSelection();
            }}
            sx={{
              position: "absolute",
              top: 5,
              right: 10,
              opacity: 0.6,
              '&:hover': { opacity: 1 },
              zIndex: 1,
              width: 24,
              height: 20,
            }}
          >
            <X size={12} />
          </IconButton>
        </Box>
      )}

      {!params.projectHandler && (
        <ProjectQuickSelector
          disabled={!params.orgHandle}
          isProjectsLoading={isLoading && projectsError === undefined}
          projectsError={projectsError}
          projectOptions={projectOptions.map((item) => ({
            id: item.handler,
            handler: item.handler,
            name: item.name,
          }))}
          onSelectProject={(projectHandler) => {
            changeProject(projectHandler);
          }}
        />
      )}

      {selectedApiHandler && (
        <Box sx={{position: 'relative'}}>
          <SearchableComplexSelect
            aria-label={intl.formatMessage({ id: 'appShell.header.api.aria', defaultMessage: 'APIs' })}
            label={intl.formatMessage({ id: 'appShell.header.api.label', defaultMessage: 'APIs' })}
            value={selectedApiHandler}
            selectedOption={apiOptions.filter((item) => item.handler === selectedApiHandler).map(item => ({
              id: item.handler,
              handler: item.handler,
              name: item.name,
              description: item.description,
            }))[0] || undefined}
            onChange={(id) => {
              changeApi(id);
            }}
            options={apiOptions.map((item) => ({
              id: item.handler,
              handler: item.handler,
              name: item.name,
              description: item.description,
            }))}
            renderOptionContent={(option) => (
              <ComplexSelect.MenuItem.Text
                primary={option.name}
                secondary={option.description || option.id}
                slotProps={TRUNCATED_OPTION_TEXT_SLOT_PROPS}
              />
            )}
            searchPlaceholder={intl.formatMessage({ id: 'appShell.header.api.placeholder', defaultMessage: 'Search APIs...' })}
            emptyMessage={intl.formatMessage({ id: 'appShell.header.api.empty', defaultMessage: 'No APIs found' })}
            noResultsMessage={intl.formatMessage({ id: 'appShell.header.api.noResults', defaultMessage: 'No matching APIs' })}
            sx={SWITCHER_SELECT_WITH_CLEAR_SX}
          />

          <IconButton
            size="small"
            aria-label={intl.formatMessage({ id: 'appShell.header.api.goToProjectLevel', defaultMessage: 'Go to project level' })}
            onMouseDown={(event) => {
              event.preventDefault();
              event.stopPropagation();
            }}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              clearApiSelection();
            }}
            sx={{
              position: "absolute",
              top: 5,
              right: 10,
              opacity: 0.6,
              '&:hover': { opacity: 1 },
              zIndex: 1,
              width: 24,
              height: 20,
            }}
          >
            <X size={12} />
          </IconButton>
        </Box>
      )}

      {!selectedApiHandler && params.projectHandler && (
        <APIQuickSelector
          disabled={!params.projectHandler}
          isApisLoading={apisLoading}
          apisError={apisErrorValue}
          apiOptions={apiOptions.map((item) => ({
            id: item.handler,
            handler: item.handler,
            name: item.name,
          }))}
          onSelectApi={(apiHandler) => {
            changeApi(apiHandler);
          }}
        />
      )}
    </Header.Switchers>
  );
}
