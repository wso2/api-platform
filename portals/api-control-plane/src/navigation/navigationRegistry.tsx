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
  Boxes,
  ClipboardList,
  Home,
  Network,
  Rocket,
  Settings,
  Terminal,
} from '@wso2/oxygen-ui-icons-react';

import { routes } from '../routes/paths';
import type { NavigationDefinition } from './navigationTypes';

export const navigationRegistry: NavigationDefinition[] = [
  {
    id: 'organization-home',
    label: 'Home',
    level: 'organization',
    order: 10,
    icon: <Home />,
    to: ({ params }) =>
      params.orgHandle ? routes.organizationHome(params.orgHandle) : undefined,
    match: (pathname) => /\/organizations\/[^/]+\/home$/.test(pathname),
  },
  {
    id: 'projects',
    label: 'Projects',
    level: 'organization',
    order: 20,
    icon: <Boxes />,
    to: ({ params }) =>
      params.orgHandle ? routes.projects(params.orgHandle) : undefined,
    match: (pathname) => /\/organizations\/[^/]+\/projects$/.test(pathname),
  },
  {
    id: 'gateways',
    label: 'API Gateways',
    level: 'organization',
    order: 30,
    icon: <Network />,
    to: ({ params }) =>
      params.orgHandle ? routes.gateways(params.orgHandle) : undefined,
    match: (pathname) => /\/organizations\/[^/]+\/gateways(\/[^/]+)?$/.test(pathname),
  },
  {
    id: 'org-settings',
    label: 'Settings',
    level: 'organization',
    order: 40,
    icon: <Settings />,
    pinned: true,
    // Only while not inside a project — the project-level `settings` entry
    // takes over once a project is selected, so there's always exactly one
    // "Settings" link pinned to the sidebar bottom, never two at once.
    isVisible: (scope) => !scope.isProjectScope,
    to: ({ params }) =>
      params.orgHandle ? routes.orgSettings(params.orgHandle) : undefined,
    match: (pathname) => /\/organizations\/[^/]+\/settings(\/[^/]+)?$/.test(pathname),
  },
  {
    id: 'project-home',
    label: 'Project Home',
    level: 'project',
    order: 100,
    icon: <Home />,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler
        ? routes.projectHome(params.orgHandle, params.projectHandler)
        : undefined,
    match: (pathname) =>
      /\/organizations\/[^/]+\/projects\/[^/]+\/home$/.test(pathname),
  },
  {
    id: 'apis',
    label: 'APIs',
    level: 'project',
    order: 110,
    icon: <Boxes />,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler
        ? routes.apis(params.orgHandle, params.projectHandler)
        : undefined,
    match: (pathname) =>
      /\/projects\/[^/]+\/apis(\/new)?$/.test(pathname),
  },
  {
    id: 'runtime-logs',
    label: 'Runtime Logs',
    level: 'project',
    order: 120,
    icon: <Terminal />,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler
        ? routes.runtimeLogs(params.orgHandle, params.projectHandler)
        : undefined,
    match: (pathname) => /\/observe\/runtimelogs$/.test(pathname),
  },
  {
    id: 'settings',
    label: 'Settings',
    level: 'project',
    order: 130,
    icon: <Settings />,
    pinned: true,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler
        ? routes.settings(params.orgHandle, params.projectHandler)
        : undefined,
    // Also active on a settings tab (e.g. /settings/general), but not deeper.
    // The `/projects/[^/]+/` prefix is required so a project literally
    // handled "settings" (e.g. `/projects/settings/home`) can't false-match.
    match: (pathname) => /\/projects\/[^/]+\/settings(\/[^/]+)?$/.test(pathname),
  },
  {
    id: 'api-overview',
    label: 'API Overview',
    level: 'api',
    order: 200,
    icon: <Boxes />,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler && params.apiHandler
        ? routes.api(
            params.orgHandle,
            params.projectHandler,
            params.apiHandler
          )
        : undefined,
    match: (pathname) => /\/apis\/[^/]+$/.test(pathname),
  },
  {
    id: 'deploy',
    label: 'Deploy',
    level: 'api',
    order: 210,
    icon: <Rocket />,
    isVisible: ({ capabilities }) => capabilities.canDeploy,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler && params.apiHandler
        ? routes.apiDeploy(
            params.orgHandle,
            params.projectHandler,
            params.apiHandler
          )
        : undefined,
    match: (pathname) => /\/deploy$/.test(pathname),
  },
  {
    id: 'test',
    label: 'Test',
    level: 'api',
    order: 220,
    icon: <Terminal />,
    isVisible: ({ capabilities }) => capabilities.canTest,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler && params.apiHandler
        ? routes.apiTest(
            params.orgHandle,
            params.projectHandler,
            params.apiHandler
          )
        : undefined,
    match: (pathname) => /\/test$/.test(pathname),
  },
  {
    id: 'manage',
    label: 'Manage',
    level: 'api',
    order: 230,
    icon: <ClipboardList />,
    isVisible: ({ capabilities }) => capabilities.canManage,
    to: ({ params }) =>
      params.orgHandle && params.projectHandler && params.apiHandler
        ? routes.apiManage(
            params.orgHandle,
            params.projectHandler,
            params.apiHandler
          )
        : undefined,
    match: (pathname) => /\/manage$/.test(pathname),
  },
];
