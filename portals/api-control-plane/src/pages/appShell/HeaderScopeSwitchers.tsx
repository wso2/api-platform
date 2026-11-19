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

import {
  Chip,
  GlobalStyles,
  Header,
  HeaderBreadcrumbs,
  writeHeaderBreadcrumbsState,
} from '@wso2/oxygen-ui';
import type { HeaderBreadcrumbsItem, HeaderBreadcrumbsLevel } from '@wso2/oxygen-ui';
import { Box, Building2, BoxesLinked } from '@wso2/oxygen-ui-icons-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useLocation, useNavigate } from 'react-router-dom';

import { useAllGraphQLApis } from '../../api/resources/graphqlApis';
import { useAllRestApis } from '../../api/resources/restApis';
import { useAuth } from '../../contexts/auth/AuthProvider';
import { findTargetAtScope } from '../../navigation/navigationRegistry';
import { useCan } from '../../permissions';
import { apiCreationEntry } from './appShellPages/apis/create/entryPoint';
import {
  toGraphQLListableApi,
  toRestListableApi,
  type ApiKind,
} from './appShellPages/apis/listing/apiListItem';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';

const ORGANIZATION_LEVEL = 'organization';
const PROJECT_LEVEL = 'project';
const API_LEVEL = 'api';

const LEVEL_ICON_SIZE = 18;

// Oxygen's menu entry has a leading `avatar` slot but no trailing one. The API
// type chip goes in the avatar slot, pulled out of the flow and pinned to the
// entry's right edge (a menu item is `position: relative`), left of the
// selected check. The rules reserve its room so a long name still truncates
// before reaching it, and drop the icon column the chip no longer fills.
const API_KIND_ATTRIBUTE = 'data-api-kind';
const API_KIND_MENU_STYLES = {
  [`.MuiMenuItem-root:has([${API_KIND_ATTRIBUTE}]) > .MuiListItemIcon-root`]: { minWidth: 0 },
  [`.MuiMenuItem-root:has([${API_KIND_ATTRIBUTE}]) > .MuiListItemText-root`]: { paddingRight: '76px' },
  [`.MuiMenuItem-root [${API_KIND_ATTRIBUTE}]`]: {
    position: 'absolute',
    right: 40,
    top: '50%',
    transform: 'translateY(-50%)',
  },
};

// The `appShell.header.*` IDs predate the breadcrumb (they named the select
// fields it replaced); kept as-is so existing translations are not discarded.
const messages = defineMessages({
  breadcrumbAria: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.breadcrumbAria',
    defaultMessage: 'Organization, project and API',
    description: 'Accessible name of the header breadcrumb trail that switches organization, project and API.',
  },
  orgLevel: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.orgLevel',
    defaultMessage: 'Organization',
    description: 'Name of the organization level in the header breadcrumb. Singular noun.',
  },
  projectLevel: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.projectLevel',
    defaultMessage: 'Project',
    description: 'Name of the project level in the header breadcrumb. Singular noun.',
  },
  apiLevel: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.apiLevel',
    defaultMessage: 'API',
    description: 'Name of the API level in the header breadcrumb. Singular noun.',
  },
  orgMenuTitle: { id: 'appShell.header.org.label', defaultMessage: 'Organizations' },
  projectMenuTitle: { id: 'appShell.header.project.label', defaultMessage: 'Projects' },
  apiMenuTitle: { id: 'appShell.header.api.label', defaultMessage: 'APIs' },
  orgSearch: { id: 'appShell.header.org.placeholder', defaultMessage: 'Search organizations...' },
  projectSearch: { id: 'appShell.header.project.placeholder', defaultMessage: 'Search projects...' },
  apiSearch: { id: 'appShell.header.api.placeholder', defaultMessage: 'Search APIs...' },
  orgEmpty: { id: 'appShell.header.org.empty', defaultMessage: 'No organizations found' },
  projectEmpty: { id: 'appShell.header.project.empty', defaultMessage: 'No projects found' },
  apiEmpty: { id: 'appShell.header.api.empty', defaultMessage: 'No APIs found' },
  projectOpen: { id: 'appShell.header.project.select', defaultMessage: 'Select Project' },
  apiOpen: { id: 'appShell.header.api.select', defaultMessage: 'Select API' },
  newApi: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.newApi',
    defaultMessage: 'New API',
  },
  newProject: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.newProject',
    defaultMessage: 'New project',
  },
  apiKindRest: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.apiKindRest',
    defaultMessage: 'REST',
    description: 'Badge on an API in the header API menu: a REST API. Technical term, usually kept as is.',
  },
  apiKindGraphql: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.apiKindGraphql',
    defaultMessage: 'GraphQL',
    description: 'Badge on an API in the header API menu: a GraphQL API. Product name, keep as is.',
  },
  projectsFailed: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.projectsFailed',
    defaultMessage: 'Failed to load projects',
  },
  apisPartial: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.apisPartial',
    defaultMessage: 'APIs (some failed to load)',
    description: 'Title of the header API menu when one kind of API (REST or GraphQL) could not be loaded but the other could.',
  },
  apisFailed: {
    id: 'apiControlPlane.pages.appShell.HeaderScopeSwitchers.apisFailed',
    defaultMessage: 'Failed to load APIs',
  },
});

/**
 * The organization / project / API trail in the header: Oxygen's
 * `HeaderBreadcrumbs` in its default look. Each segment's name goes up to that
 * level and its caret opens a menu to switch to a sibling; the first level
 * with nothing selected shows as a › button that opens its picker.
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
  const canCreateProject = useCan('CreateProject');
  const canCreateRestApi = useCan('CreateRESTAPI');
  const canCreateGraphQLApi = useCan('CreateGraphQLAPI');
  const { organizations: sessionOrganizations } = useAuth();
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

  // Where the session switches organizations, the identity provider lists
  // every organization the user belongs to; the Platform API only those
  // already used. Either may not be loaded yet on first paint; keep the
  // current org selectable so the switcher never renders an out-of-range value.
  const orgOptions: { handle: string; name: string }[] =
    sessionOrganizations && sessionOrganizations.length > 0
      ? sessionOrganizations.map((org) => ({ handle: org.handle, name: org.name }))
      : organizations.length > 0
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
  const loadedApiOptions: { handler: string; name: string; description?: string; kind: ApiKind }[] =
    (projects.length > 0 && project) ?
    apis
      .filter(
        (
          api
        ): api is NonNullable<typeof api> & { id: string; displayName?: string; description?: string } =>
          typeof api?.id === 'string' && api.id.length > 0
      )
      .map((api) => ({
        handler: api.id,
        name: api.displayName ?? api.id,
        description: api.description,
        kind: api.apiType,
      }))
    : [];

  // Whichever kind of API is in scope — REST's `apiHandler` or GraphQL's
  // `graphqlApiHandler` (see `graphqlApiPath`'s doc comment for why they're
  // separate params) — is "the selected API" for this switcher; the two are
  // never both set at once.
  const selectedApiHandler = params.apiHandler || params.graphqlApiHandler;

  // apis may not be loaded yet on first paint; keep the current API selectable
  // so the switcher never renders an out-of-range value.
  const apiOptions: { handler: string; name: string; description?: string; kind: ApiKind }[] =
    loadedApiOptions.length > 0
      ? loadedApiOptions
      : selectedApiHandler
        ? [
            {
              handler: selectedApiHandler,
              name: component?.displayName || selectedApiHandler,
              description: component?.description,
              kind: params.graphqlApiHandler ? 'graphql' : 'rest',
            },
          ]
        : [];

  // Remember where the user is, in the place every app using Oxygen's header
  // reads it (`oxygen-ui:header-breadcrumbs`), so the next visit, here or in
  // another app, starts from the same organization and project. Only while a
  // project is in scope: browsing organization-level pages keeps the last one.
  const { orgHandle: scopedOrg, projectHandler: scopedProject } = params;
  useEffect(() => {
    if (scopedOrg && scopedProject) {
      writeHeaderBreadcrumbsState({ organization: scopedOrg, project: scopedProject });
    }
  }, [scopedOrg, scopedProject]);

  if (!params.orgHandle) return null;

  const levels: HeaderBreadcrumbsLevel[] = [
    {
      id: ORGANIZATION_LEVEL,
      label: intl.formatMessage(messages.orgLevel),
      icon: <Building2 size={LEVEL_ICON_SIZE} />,
      items: orgOptions.map((item) => ({ id: item.handle, name: item.name, description: item.handle })),
      selectedId: params.orgHandle,
      menuTitle: intl.formatMessage(messages.orgMenuTitle),
      searchPlaceholder: intl.formatMessage(messages.orgSearch),
      emptyText: intl.formatMessage(messages.orgEmpty),
    },
    {
      id: PROJECT_LEVEL,
      label: intl.formatMessage(messages.projectLevel),
      icon: <Box size={LEVEL_ICON_SIZE} />,
      items: projectOptions.map((item) => ({
        id: item.handler,
        name: item.name,
        description: item.description || undefined,
      })),
      selectedId: params.projectHandler,
      searchable: true,
      createAction: canCreateProject
        ? { label: intl.formatMessage(messages.newProject), onClick: () => navigate(routes.newProject(params.orgHandle)) }
        : false,
      loading: isLoading && projectsError === undefined,
      // Oxygen shows `error` in place of the whole list, so raise it only when
      // there is nothing usable to pick; a partial failure is flagged in the title.
      error: projectsError && projects.length === 0 ? intl.formatMessage(messages.projectsFailed) : undefined,
      openLabel: intl.formatMessage(messages.projectOpen),
      menuTitle: intl.formatMessage(messages.projectMenuTitle),
      searchPlaceholder: intl.formatMessage(messages.projectSearch),
      emptyText: intl.formatMessage(messages.projectEmpty),
    },
    {
      id: API_LEVEL,
      label: intl.formatMessage(messages.apiLevel),
      icon: <BoxesLinked size={LEVEL_ICON_SIZE} strokeWidth={1} />,
      items: apiOptions.map((item) => ({
        id: item.handler,
        name: item.name,
        description: item.description || undefined,
        avatar: (
          <Chip
            {...{ [API_KIND_ATTRIBUTE]: item.kind }}
            size="small"
            variant="outlined"
            sx={{ height: 18, fontSize: '0.6875rem', borderRadius: 0.5, '& .MuiChip-label': { px: 0.75 } }}
            label={intl.formatMessage(item.kind === 'graphql' ? messages.apiKindGraphql : messages.apiKindRest)}
          />
        ),
      })),
      selectedId: selectedApiHandler,
      searchable: true,
      createAction:
        (canCreateRestApi || canCreateGraphQLApi) && params.projectHandler
          ? {
              label: intl.formatMessage(messages.newApi),
              onClick: () =>
                navigate(
                  routes.newApi(params.orgHandle, params.projectHandler),
                  apiCreationEntry('header-breadcrumb'),
                ),
            }
          : false,
      loading: apisLoading,
      // As for projects: REST loading while GraphQL fails (or the reverse) must
      // leave the APIs that did load selectable, so the blocking error is for
      // when none loaded, and a partial failure is flagged in the menu title.
      error: apisErrorValue && loadedApiOptions.length === 0 ? intl.formatMessage(messages.apisFailed) : undefined,
      openLabel: intl.formatMessage(messages.apiOpen),
      menuTitle: intl.formatMessage(
        apisErrorValue && loadedApiOptions.length > 0 ? messages.apisPartial : messages.apiMenuTitle,
      ),
      searchPlaceholder: intl.formatMessage(messages.apiSearch),
      emptyText: intl.formatMessage(messages.apiEmpty),
    },
  ];

  const selectAtLevel = (levelId: string, item: HeaderBreadcrumbsItem) => {
    if (levelId === ORGANIZATION_LEVEL) changeOrganization(item.id);
    else if (levelId === PROJECT_LEVEL) changeProject(item.id);
    else if (levelId === API_LEVEL) changeApi(item.id);
  };

  // A name in the trail goes up to that level. An ancestor keeps the current
  // page where that level has one, as the switch menus do; the last segment is
  // already the current level, so its name goes to that level's Home.
  const navigateToLevel = (levelId: string) => {
    const orgHandle = params.orgHandle;
    if (!orgHandle) return;
    if (levelId === ORGANIZATION_LEVEL) {
      if (params.projectHandler) clearProjectSelection();
      else navigate(routes.organizationHome(orgHandle));
    } else if (levelId === PROJECT_LEVEL && params.projectHandler) {
      if (selectedApiHandler) clearApiSelection();
      else navigate(routes.projectHome(orgHandle, params.projectHandler));
    } else if (levelId === API_LEVEL && params.projectHandler && selectedApiHandler) {
      navigate(
        params.graphqlApiHandler
          ? routes.graphqlApi(orgHandle, params.projectHandler, selectedApiHandler)
          : routes.api(orgHandle, params.projectHandler, selectedApiHandler),
      );
    }
  };

  return (
    <Header.Switchers showDivider={false}>
      <GlobalStyles styles={API_KIND_MENU_STYLES} />
      <HeaderBreadcrumbs
        aria-label={intl.formatMessage(messages.breadcrumbAria)}
        levels={levels}
        onSelect={selectAtLevel}
        onNavigate={navigateToLevel}
      />
    </Header.Switchers>
  );
}
