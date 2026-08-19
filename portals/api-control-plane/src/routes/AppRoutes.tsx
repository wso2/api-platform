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

import { lazy } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';

import { AuthCallbackPage } from '../features/auth/AuthCallbackPage';
import { LoginPage } from '../features/auth/LoginPage';
import {
  NotFoundPage,
  OrganizationRedirectPage,
  ServerErrorPage,
  SessionExpiredPage,
  UnauthorizedPage,
} from '../features/system/SystemPages';
import { ConsoleScopeProvider } from '../scope/ConsoleScopeProvider';
import AppLayout from '../layouts/AppLayout';
import {
  buildScopedExtensionPath,
  type ApiControlPlaneExtension,
} from '../extensions';
import { usePort } from '../hostPort';
import { ProtectedRoute } from './ProtectedRoute';
import { routes } from './paths';

/** Resolves the real `CloudHostPort` and hands it to the extension's `render`. */
function ExtensionRoute({ extension }: { extension: ApiControlPlaneExtension }) {
  const port = usePort();
  return <>{extension.render(port)}</>;
}

// Code-split the authenticated feature pages so they are not pulled into the
// initial (login) bundle.
const OrganizationHomePage = lazy(() =>
  import('../features/organizations/OrganizationHomePage').then((m) => ({
    default: m.OrganizationHomePage,
  }))
);
const ProjectListPage = lazy(() =>
  import('../features/projects/ProjectListPage').then((m) => ({
    default: m.ProjectListPage,
  }))
);
const GatewaysPage = lazy(() =>
  import('../features/gateways/GatewaysPage').then((m) => ({
    default: m.GatewaysPage,
  }))
);
const GatewayCreatePage = lazy(() =>
  import('../features/gateways/GatewayCreatePage').then((m) => ({
    default: m.GatewayCreatePage,
  }))
);
const GatewayDetailPage = lazy(() =>
  import('../features/gateways/GatewayDetailPage').then((m) => ({
    default: m.GatewayDetailPage,
  }))
);
const ProjectHomePage = lazy(() =>
  import('../features/projects/ProjectHomePage').then((m) => ({
    default: m.ProjectHomePage,
  }))
);
const ApiListPage = lazy(() =>
  import('../features/apis/ApiListPage').then((m) => ({
    default: m.ApiListPage,
  }))
);
const ApiCreatePage = lazy(() =>
  import('../features/apis/ApiCreatePage').then((m) => ({
    default: m.ApiCreatePage,
  }))
);
const ApiDetailPage = lazy(() =>
  import('../features/apis/ApiDetailPage').then((m) => ({
    default: m.ApiDetailPage,
  }))
);
const DeployPage = lazy(() =>
  import('../features/deploy/DeployPage').then((m) => ({ default: m.DeployPage }))
);
const TestPage = lazy(() =>
  import('../features/test/TestPage').then((m) => ({ default: m.TestPage }))
);
const ManagePage = lazy(() =>
  import('../features/manage/ManagePage').then((m) => ({ default: m.ManagePage }))
);
const RuntimeLogsPage = lazy(() =>
  import('../features/logs/RuntimeLogsPage').then((m) => ({
    default: m.RuntimeLogsPage,
  }))
);
const SettingsLayout = lazy(() =>
  import('../features/settings/SettingsLayout').then((m) => ({
    default: m.SettingsLayout,
  }))
);
const GeneralSettingsPage = lazy(() =>
  import('../features/settings/GeneralSettingsPage').then((m) => ({
    default: m.GeneralSettingsPage,
  }))
);

export type AppRoutesProps = {
  extensions?: readonly ApiControlPlaneExtension[];
};

export function AppRoutes({ extensions = [] }: AppRoutesProps) {
  const topLevelExtensions = extensions.filter((ext) =>
    ext.slot.startsWith('sidebar.')
  );

  const extensionRoutes = topLevelExtensions.map((extension) => (
    <Route
      key={extension.id}
      path={buildScopedExtensionPath(extension.scope, extension.routePath, {
        apiHandler: ':apiHandler',
        orgHandle: ':orgHandle',
        projectHandler: ':projectHandler',
      })}
      element={<ExtensionRoute extension={extension} />}
    />
  ));

  // Extensions registered against a `settings.<scope>.tabs` slot render
  // nested under the matching (org- or project-level) Settings layout
  // instead of as a sibling top-level route — the path is relative to
  // `/settings/`.
  const settingsTabRoutesFor = (scope: 'organization' | 'project') =>
    extensions
      .filter((ext) => ext.slot === `settings.${scope}.tabs`)
      .map((extension) => (
        <Route
          key={extension.id}
          path={extension.routePath.replace(/^settings\//, '')}
          element={<ExtensionRoute extension={extension} />}
        />
      ));

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
          <Route path={routes.orgSettings()} element={<SettingsLayout scope="organization" />}>
            <Route index element={<Navigate to="general" replace />} />
            <Route path="general" element={<GeneralSettingsPage />} />
            {settingsTabRoutesFor('organization')}
          </Route>
          <Route path={routes.projects()} element={<ProjectListPage />} />
          <Route path={routes.gateways()} element={<GatewaysPage />} />
          <Route path={routes.newGateway()} element={<GatewayCreatePage />} />
          <Route path={routes.gateway()} element={<GatewayDetailPage />} />
          <Route path={routes.projectHome()} element={<ProjectHomePage />} />
          <Route path={routes.apis()} element={<ApiListPage />} />
          <Route path={routes.newApi()} element={<ApiCreatePage />} />
          <Route path={routes.api()} element={<ApiDetailPage />} />
          <Route path={routes.apiDeploy()} element={<DeployPage />} />
          <Route path={routes.apiTest()} element={<TestPage />} />
          <Route path={routes.apiManage()} element={<ManagePage />} />
          <Route path={routes.runtimeLogs()} element={<RuntimeLogsPage />} />
          <Route path={routes.settings()} element={<SettingsLayout scope="project" />}>
            <Route index element={<Navigate to="general" replace />} />
            <Route path="general" element={<GeneralSettingsPage />} />
            {settingsTabRoutesFor('project')}
          </Route>
          {extensionRoutes}
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  );
}
