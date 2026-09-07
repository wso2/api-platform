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

import { lazy, type ReactNode } from 'react';
import { Navigate, Route, Routes, useLocation, useParams } from 'react-router-dom';

import { AuthCallbackPage } from '@/pages/auth/AuthCallbackPage';
import { LoginPage } from '@/pages/auth/LoginPage';
import {
  NotFoundPage,
  OrganizationRedirectPage,
  ServerErrorPage,
  SessionExpiredPage,
  UnauthorizedPage,
} from '@/pages/appShell/appShellPages/system/SystemPages';
import { ConsoleScopeProvider } from '@/scope/ConsoleScopeProvider';
import AppLayout from '@/pages/appShell/AppLayout';
import {
  extensionScopedPaths,
  isSidebarExtension,
  PAGE_GATEWAYS_SLOT,
  settingsTabExtensions,
  type ApiControlPlaneExtension,
} from '@/extensions';
import type { NavigationLevel } from '@/navigation/navigationTypes';
import { usePort } from '@/hostPort';
import { Hideable, useSlot } from '@/slots';
import { ProtectedRoute } from './ProtectedRoute';
import { apiScopedPaths, projectScopedPaths, routes } from './paths';

// Code-split the authenticated feature pages so they are not pulled into the
// initial (login) bundle.
const OrganizationHomePage = lazy(() =>
  import('../pages/appShell/appShellPages/organizations/OrganizationHomePage').then((m) => ({
    default: m.OrganizationHomePage,
  })),
);
const ProjectListPage = lazy(() =>
  import('../pages/appShell/appShellPages/projects').then((m) => ({
    default: m.ProjectListPage,
  })),
);
const GatewaysPage = lazy(() =>
  import('../pages/appShell/appShellPages/gateways/GatewaysPage').then((m) => ({
    default: m.GatewaysPage,
  })),
);
const GatewayCreatePage = lazy(() =>
  import('../pages/appShell/appShellPages/gateways/GatewayCreatePage').then((m) => ({
    default: m.GatewayCreatePage,
  })),
);
const GatewayDetailPage = lazy(() =>
  import('../pages/appShell/appShellPages/gateways/GatewayDetailPage').then((m) => ({
    default: m.GatewayDetailPage,
  })),
);
const ProjectHomePage = lazy(() =>
  import('../pages/appShell/appShellPages/projects').then((m) => ({
    default: m.ProjectHomePage,
  })),
);
const ApiListPage = lazy(() =>
  import('../pages/appShell/appShellPages/apis/listing').then((m) => ({
    default: m.ApiListPage,
  })),
);
const ApiCreatePage = lazy(() =>
  import('../pages/appShell/appShellPages/apis/create/ApiCreationWizard').then((m) => ({
    default: m.ApiCreationWizard,
  })),

);
const ApiDetailPage = lazy(() =>
  import('../pages/appShell/appShellPages/apis/overview').then((m) => ({
    default: m.ApiDetailPage,
  })),
);
const ApiEditPage = lazy(() =>
  import('../pages/appShell/appShellPages/apis/edit').then((m) => ({
    default: m.ApiEditPage,
  })),
);
const DeployPage = lazy(() =>
  import('../pages/appShell/appShellPages/deploy/DeployPage').then((m) => ({
    default: m.DeployPage,
  })),
);
const TestPage = lazy(() =>
  import('../pages/appShell/appShellPages/test/TestPage').then((m) => ({ default: m.TestPage })),
);
const PoliciesPage = lazy(() =>
  import('../pages/appShell/appShellPages/develop/policies/PoliciesPage').then((m) => ({
    default: m.PoliciesPage,
  })),
);
const RoutingPage = lazy(() =>
  import('../pages/appShell/appShellPages/develop/routings/RoutingPage').then((m) => ({
    default: m.RoutingPage,
  })),
);
const DocumentsPage = lazy(() =>
  import('../pages/appShell/appShellPages/develop/documents/DocumentsPage').then((m) => ({
    default: m.DocumentsPage,
  })),
);
const ApiConsolePage = lazy(() =>
  import('../pages/appShell/appShellPages/test/ApiConsolePage').then((m) => ({
    default: m.ApiConsolePage,
  })),
);
const ApiChatPage = lazy(() =>
  import('../pages/appShell/appShellPages/test/ApiChatPage').then((m) => ({
    default: m.ApiChatPage,
  })),
);
const AlertsPage = lazy(() =>
  import('../pages/appShell/appShellPages/observability/AlertsPage').then((m) => ({
    default: m.AlertsPage,
  })),
);
const MetricsPage = lazy(() =>
  import('../pages/appShell/appShellPages/observability/MetricsPage').then((m) => ({
    default: m.MetricsPage,
  })),
);
const MonetizePage = lazy(() =>
  import('../pages/appShell/appShellPages/manage/MonetizePage').then((m) => ({
    default: m.MonetizePage,
  })),
);
const LifeCyclePage = lazy(() =>
  import('../pages/appShell/appShellPages/manage/LifeCyclePage').then((m) => ({
    default: m.LifeCyclePage,
  })),
);
const InsightsPage = lazy(() =>
  import('../pages/appShell/appShellPages/insights/InsightsPage').then((m) => ({
    default: m.InsightsPage,
  })),
);
const CompliancePage = lazy(() =>
  import('../pages/appShell/appShellPages/insights/CompliancePage').then((m) => ({
    default: m.CompliancePage,
  })),
);
const AdminPage = lazy(() =>
  import('../pages/appShell/appShellPages/admin/AdminPage').then((m) => ({
    default: m.AdminPage,
  })),
);
const RuntimeLogsPage = lazy(() =>
  import('../pages/appShell/appShellPages/observability/RuntimeLogsPage').then((m) => ({
    default: m.RuntimeLogsPage,
  })),
);
const SettingsLayout = lazy(() =>
  import('../pages/appShell/appShellPages/settings/SettingsLayout').then((m) => ({
    default: m.SettingsLayout,
  })),
);
const GeneralSettingsPage = lazy(() =>
  import('../pages/appShell/appShellPages/settings/GeneralSettingsPage').then((m) => ({
    default: m.GeneralSettingsPage,
  })),
);

export type AppRoutesProps = {
  extensions?: readonly ApiControlPlaneExtension[];
};

/**
 * One `<Route>` per path a scoped page answers on — its fully-scoped path plus
 * the scope-less aliases the sidebar links to when the project (or API) isn't
 * selected yet. The same element renders at all of them; the `ScopeGate` inside
 * it shows a picker instead of the page body until scope is complete.
 */
const scopedRoutes = (paths: string[], element: ReactNode) =>
  paths.map((path) => <Route key={path} path={path} element={element} />);

/**
 * Resolves the real `CloudHostPort` and hands it to the extension's `render`.
 *
 * The port is read from context here, inside the router, rather than passed in
 * from the registration site — an extension only ever receives it as a plain
 * value, so it never imports this portal's hooks itself (see `hostPort.tsx`).
 */
function ExtensionRoute({ extension }: { extension: ApiControlPlaneExtension }) {
  const port = usePort();
  return <>{extension.render(port)}</>;
}

/**
 * The gateways route subtree. Renders a cloud override registered against
 * `PAGE_GATEWAYS_SLOT` when one is present, otherwise the built-in gateways
 * pages — the same Slot-adds / Hideable-suppresses split the ai-workspace host
 * uses, so the cloud build swaps in the managed-gateways plugin while the public
 * build keeps its native pages. Mounted at `gateways/*` so either branch owns
 * the nested `create`/`new`/`:gatewayId` routes.
 */
function GatewaysRoute() {
  const port = usePort();
  const [override] = useSlot<ApiControlPlaneExtension>(PAGE_GATEWAYS_SLOT);
  // This route's own `gateways` path, derived by stripping the matched splat
  // off the current URL. Deliberately not `useResolvedPath('')`: from a splat
  // route react-router 7 resolves relative to the *full* matched pathname, so
  // that would hand back the current URL and redirect it to itself. Stripping
  // the splat also keeps the scope-less `select-scope` aliases working, which
  // rebuilding the path from `routes.gateways()` would not.
  const { pathname } = useLocation();
  const splat = useParams()['*'] ?? '';
  const indexPath = splat
    ? pathname.slice(0, pathname.length - splat.length).replace(/\/+$/, '')
    : pathname;
  if (override) {
    // An override replaces the whole subtree with one self-contained flow that
    // keeps its view in local state (a feature package holds no router of its
    // own, so it cannot own nested routes). There is therefore no `new` or
    // `:gatewayId` URL under it: send those to the index instead of rendering
    // the list at a URL claiming to be a create or detail page.
    return (
      <Routes>
        <Route index element={<>{override.render(port)}</>} />
        <Route element={<Navigate replace to={indexPath} />} path="*" />
      </Routes>
    );
  }
  return (
    <Hideable name={PAGE_GATEWAYS_SLOT}>
      <Routes>
        <Route index element={<GatewaysPage />} />
        <Route path="new" element={<GatewayCreatePage />} />
        <Route path=":gatewayId" element={<GatewayDetailPage />} />
      </Routes>
    </Hideable>
  );
}

export function AppRoutes({ extensions = [] }: AppRoutesProps) {
  // Extensions registered against a `settings.<level>.tabs` slot render nested
  // under the matching Settings layout, at a path relative to it — so the tab's
  // own route and the sub-nav entry `useSettingsTabs` builds stay in step.
  const settingsTabRoutes = (level: NavigationLevel) =>
    settingsTabExtensions(extensions, level).map((extension) => (
      <Route
        key={extension.id}
        path={extension.routePath.replace(/^settings\//, '')}
        element={<ExtensionRoute extension={extension} />}
      />
    ));

  // Only `sidebar.*` entries become top-level routes; a Settings tab extension
  // is routed by `settingsTabRoutes` above, nested under the Settings layout.
  const extensionRoutes = extensions
    .filter(isSidebarExtension)
    .flatMap((extension) =>
      extensionScopedPaths(extension.level, extension.routePath).map((path) => (
        <Route
          key={`${extension.id}:${path}`}
          path={path}
          element={<ExtensionRoute extension={extension} />}
        />
      )),
    );

  return (
    <Routes>
      <Route path={routes.login} element={<LoginPage />} />
      <Route path={routes.authCallback} element={<AuthCallbackPage />} />
      <Route path={routes.signInCallback} element={<AuthCallbackPage />} />
      <Route path={routes.unauthorized} element={<UnauthorizedPage />} />
      <Route path={routes.sessionExpired} element={<SessionExpiredPage />} />
      <Route path={routes.serverError} element={<ServerErrorPage />} />
      <Route element={<ProtectedRoute />}>
        <Route
          element={
            <ConsoleScopeProvider>
              <AppLayout />
            </ConsoleScopeProvider>
          }
        >
          <Route path="/" element={<OrganizationRedirectPage />} />
          <Route path={routes.organizations} element={<OrganizationRedirectPage />} />
          <Route path={routes.organizationHome()} element={<OrganizationHomePage />} />
          <Route path={routes.projects()} element={<ProjectListPage />} />
          <Route path={`${routes.gateways()}/*`} element={<GatewaysRoute />} />
          {/*
            Project and API overview take a single fully-scoped path each: they
            are the deeper tiers of the sidebar's Overview item, which degrades
            to a shallower tier instead of linking here un-scoped, so there is no
            alias to register.
          */}
          <Route path={routes.projectHome()} element={<ProjectHomePage />} />
          <Route path={routes.api()} element={<ApiDetailPage />} />
          {scopedRoutes(projectScopedPaths(routes.apis), <ApiListPage />)}
          <Route path={routes.newApi()} element={<ApiCreatePage />} />
          {/*
            Editing basic information is reached from the detail page's own edit
            button, so — like the create page — it takes the fully-scoped path
            alone.
          */}
          <Route path={routes.apiEdit()} element={<ApiEditPage />} />
          {/*
            Test, Observability and Manage are sidebar parents with no page of
            their own — only their children are routed. Out of API scope a parent
            links to its first child's alias, so `.../test` and friends are never
            produced and are not registered.
          */}
          {scopedRoutes(apiScopedPaths(routes.apiDevelopPolicies), <PoliciesPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiDevelopRouting), <RoutingPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiDevelopDocuments), <DocumentsPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiTestConsole), <ApiConsolePage />)}
          {scopedRoutes(apiScopedPaths(routes.apiTestCurl), <TestPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiTestChat), <ApiChatPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiDeploy), <DeployPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiInsightsApi), <InsightsPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiInsightsCompliance), <CompliancePage />)}
          {scopedRoutes(apiScopedPaths(routes.apiObservabilityAlerts), <AlertsPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiObservabilityMetrics), <MetricsPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiObservabilityLogs), <RuntimeLogsPage />)}
          {scopedRoutes(apiScopedPaths(routes.apiManageMonetize), <MonetizePage />)}
          {scopedRoutes(apiScopedPaths(routes.apiManageLifecycle), <LifeCyclePage />)}
          {scopedRoutes(apiScopedPaths(routes.apiAdmin), <AdminPage />)}
          {/* Two entry points, one page, no scope requirement either way. The
              index route renders the same content as `general`, so `/settings`
              is never a blank pane. */}
          <Route path={routes.settings()} element={<SettingsLayout level="organization" />}>
            <Route index element={<GeneralSettingsPage />} />
            <Route path="general" element={<GeneralSettingsPage />} />
            {settingsTabRoutes('organization')}
          </Route>
          <Route path={routes.projectSettings()} element={<SettingsLayout level="project" />}>
            <Route index element={<GeneralSettingsPage />} />
            <Route path="general" element={<GeneralSettingsPage />} />
            {settingsTabRoutes('project')}
          </Route>
          {extensionRoutes}
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  );
}
