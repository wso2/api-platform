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
 * A project or API handle for a path builder.
 *
 * `null` means "this scope is not selected yet". The builder then replaces the
 * unresolved scope's segments with `SELECT_SCOPE_SEGMENT`, producing the
 * **scope-less alias** of the same page — the URL the sidebar links to when the
 * user clicks, say, Deploy while no API is in scope. The page still mounts at
 * that URL; its `ScopeGate` renders a scope picker instead of the page body
 * until the handle is filled in.
 *
 * Omitting the argument entirely keeps the `:param` placeholder, which is what
 * route registration and the sidebar's `match` predicates want.
 */
export type ScopeHandle = string | null;

/**
 * Marks where a page's path runs out of resolved scope.
 *
 * Reserved: no organization, project or API handle may take this value, and no
 * page suffix may start with it.
 */
export const SELECT_SCOPE_SEGMENT = 'select-scope';

const join = (...segments: (string | undefined)[]) =>
  `/${segments.filter(Boolean).join('/')}`;

/**
 * Builds a **project-level** page's path, marking an unresolved project:
 *
 * ```
 * projectPath('acme', 'orders', 'apis') -> /organizations/acme/projects/orders/apis
 * projectPath('acme', null,     'apis') -> /organizations/acme/select-scope/apis
 * ```
 *
 * The marker earns its place by keeping the alias unambiguous in both
 * directions:
 *
 * - `ConsoleScopeProvider` reads scope back out of the pathname positionally —
 *   the segment after `projects` is the project handle, the one after `apis` is
 *   the API handle. Merely dropping the handle would leave
 *   `/organizations/acme/projects/apis`, read as *project `apis`*, so the gate
 *   would believe a project was selected and the page would query one that
 *   doesn't exist. The marker keeps `projects`/`apis` out of the alias entirely,
 *   so those lookups correctly find nothing.
 * - Nothing can collide. Dropping the segments instead would put project
 *   Settings at `/organizations/:orgHandle/settings` and project Home at
 *   `/organizations/:orgHandle/home`, which is `organizationHome`.
 */
export const projectPath = (
  orgHandle: string,
  projectHandler: ScopeHandle,
  suffix?: string
): string =>
  projectHandler
    ? join('organizations', orgHandle, 'projects', projectHandler, suffix)
    : join('organizations', orgHandle, SELECT_SCOPE_SEGMENT, suffix);

/**
 * Builds an **API-level** page's path. The marker replaces whichever scope runs
 * out first, so one alias covers "no project yet" and another "project known,
 * API still to pick":
 *
 * ```
 * apiPath('acme', 'orders', 'a-1', 'deploy') -> /organizations/acme/projects/orders/apis/a-1/deploy
 * apiPath('acme', 'orders', null,  'deploy') -> /organizations/acme/projects/orders/select-scope/deploy
 * apiPath('acme', null,     null,  'deploy') -> /organizations/acme/select-scope/deploy
 * ```
 */
export const apiPath = (
  orgHandle: string,
  projectHandler: ScopeHandle,
  apiHandler: ScopeHandle,
  suffix?: string
): string => {
  if (!projectHandler) return projectPath(orgHandle, null, suffix);
  if (!apiHandler) {
    return join(
      'organizations',
      orgHandle,
      'projects',
      projectHandler,
      SELECT_SCOPE_SEGMENT,
      suffix
    );
  }
  return join(
    'organizations',
    orgHandle,
    'projects',
    projectHandler,
    'apis',
    apiHandler,
    suffix
  );
};

export const routes = {
  login: '/login',
  authCallback: '/login/callback',
  signInCallback: '/signin',
  unauthorized: '/unauthorized',
  sessionExpired: '/session-expired',
  serverError: '/server-error',
  organizations: '/organizations',
  organizationHome: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/home`,
  projects: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/projects`,
  gateways: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/gateways`,
  newGateway: (orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/gateways/new`,
  gateway: (orgHandle = ':orgHandle', gatewayId = ':gatewayId') =>
    `/organizations/${orgHandle}/gateways/${gatewayId}`,
  // Project and API overview are the deeper tiers of the sidebar's Overview
  // item, which degrades to a shallower tier rather than gating (see
  // `adaptive` in navigation/navigationRegistry.tsx) — so neither is ever built
  // without its handle, and neither needs a scope-less alias.
  projectHome: (orgHandle = ':orgHandle', projectHandler = ':projectHandler') =>
    projectPath(orgHandle, projectHandler, 'home'),
  apis: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler'
  ) => projectPath(orgHandle, projectHandler, 'apis'),
  // Only reachable from inside a project (the APIs page's own create button),
  // so it has no scope-less alias.
  newApi: (orgHandle = ':orgHandle', projectHandler = ':projectHandler') =>
    projectPath(orgHandle, projectHandler, 'apis/new'),
  api: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler',
    apiHandler = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler),
  apiEdit: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler',
    apiHandler = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'edit'),
  // Develop's own submenu: the three panels that used to be tabs on the API
  // overview page.
  apiDevelopPolicies: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'develop/policies'),
  apiDevelopRouting: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'develop/routing'),
  apiDevelopDocuments: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'develop/documents'),
  apiDeploy: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'deploy'),
  // Test, Observability and Manage are sidebar *parents*: in API scope they open
  // a submenu rather than a page, so only their children have paths. There is no
  // bare `.../test` route — nothing links to one.
  apiTestConsole: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'test/console'),
  apiTestCurl: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'test/curl'),
  apiTestChat: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'test/chat'),
  apiManageMonetize: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'manage/monetize'),
  apiManageLifecycle: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'manage/lifecycle'),
  // The doubled `api` is the child's own label ("API Insights") under the
  // Insights parent, not a stutter in the naming scheme.
  apiInsightsApi: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'insights/api'),
  apiInsightsCompliance: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'insights/compliance'),
  apiObservabilityAlerts: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'observability/alerts'),
  apiObservabilityMetrics: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'observability/metrics'),
  // Runtime logs, scoped to one API. Was project-wide (`observe/runtimelogs`)
  // when the sidebar had a project section; it now sits under Observability.
  apiObservabilityLogs: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'observability/logs'),
  apiAdmin: (
    orgHandle = ':orgHandle',
    projectHandler: ScopeHandle = ':projectHandler',
    apiHandler: ScopeHandle = ':apiHandler'
  ) => apiPath(orgHandle, projectHandler, apiHandler, 'admin'),
  // Settings is the one page with no scope requirement, so the sidebar links to
  // the organization-level path and it renders whatever the scope. It needs no
  // scope-less alias for the same reason: there is nothing to select.
  settings: (orgHandle = ':orgHandle') => `/organizations/${orgHandle}/settings`,
  // The same page, deep-linked for one project — the gear on a project card.
  // Kept as its own builder rather than a `ScopeHandle` on `settings` because
  // these two are alternative entry points, not a scoped/scope-less pair.
  projectSettings: (
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler'
  ) => projectPath(orgHandle, projectHandler, 'settings'),
  // One Settings sub-nav tab, org- and project-scoped. `tab` is the segment
  // below `/settings/` ("general", or an extension's own `routePath` with the
  // `settings/` prefix stripped) — the sub-nav and the routes are built from
  // these two, so a tab can never link somewhere no route answers.
  settingsTab: (tab: string, orgHandle = ':orgHandle') =>
    `/organizations/${orgHandle}/settings/${tab}`,
  projectSettingsTab: (
    tab: string,
    orgHandle = ':orgHandle',
    projectHandler = ':projectHandler'
  ) => projectPath(orgHandle, projectHandler, `settings/${tab}`),
};

export type ProjectPathBuilder = (
  orgHandle: string,
  projectHandler: ScopeHandle
) => string;

export type ApiPathBuilder = (
  orgHandle: string,
  projectHandler: ScopeHandle,
  apiHandler: ScopeHandle
) => string;

/**
 * Any `routes.*` page builder, whatever depth it takes — an org-level one
 * ignores the trailing arguments.
 *
 * This is the shape the sidebar's scope-adaptive items need: one item pointing at
 * an org-, project- and API-level page, each called with as many handles as its
 * own level takes. Deliberately `string | undefined` rather than `ScopeHandle`:
 * every builder here accepts it, and the adaptive item only ever calls a tier
 * whose handles it already has, so `null` never comes up.
 */
export type ScopedPathBuilder = (
  orgHandle: string,
  projectHandler?: string,
  apiHandler?: string
) => string;

/**
 * Every route pattern a project-level page answers on: its fully-scoped path
 * plus the scope-less alias the sidebar links to when no project is selected.
 *
 * `AppRoutes` registers these and `navigationRegistry` matches against them, so
 * the route table and the sidebar highlight can't drift apart — both are
 * generated from the same builder.
 */
export const projectScopedPaths = (build: ProjectPathBuilder): string[] => [
  build(':orgHandle', ':projectHandler'),
  build(':orgHandle', null),
];

/**
 * Same as `projectScopedPaths`, for API-level pages. Three patterns, because an
 * API-level page can be missing the API alone or both the API and the project.
 */
export const apiScopedPaths = (build: ApiPathBuilder): string[] => [
  build(':orgHandle', ':projectHandler', ':apiHandler'),
  ...apiScopeSelectPaths(build),
];

/**
 * Just the scope-less aliases of an API-level page — `apiScopedPaths` without the
 * fully-scoped path.
 *
 * This is what a submenu *parent* matches on. A parent has no page of its own, so
 * out of API scope it links to its first child's alias, and matching those
 * aliases is what keeps it highlighted while the `ScopeGate` asks for an API.
 * Deliberately not the fully-scoped path: once scope resolves, the child owns the
 * highlight (Oxygen leaves an expanded parent unhighlighted by design), and
 * matching both would make parent and child claim active at once.
 */
export const apiScopeSelectPaths = (build: ApiPathBuilder): string[] => [
  build(':orgHandle', ':projectHandler', null),
  build(':orgHandle', null, null),
];
