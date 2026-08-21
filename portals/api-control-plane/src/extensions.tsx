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

import { createContext, useContext, type ReactNode } from 'react';

import { apiPath, projectPath, type ScopeHandle } from './routes/paths';
import type { ConsoleScope } from './scope/ConsoleScopeProvider';
import type { NavigationLevel } from './navigation/navigationTypes';

/**
 * A host-injected feature: a route plus its sidebar entry. `routePath` is
 * relative to the same route group the built-in nav items live in (e.g.
 * `"billing"`, not `"/organizations/:orgHandle/billing"`), and `level`
 * decides which sidebar section it's grouped under — mirrors
 * `NavigationDefinition` so it can be merged straight into the existing
 * nav pipeline in `navigation/useNavigationItems.ts`.
 */
export type ApiControlPlaneExtension = {
  id: string;
  routePath: string;
  element: ReactNode;
  label: string;
  icon?: ReactNode;
  level: NavigationLevel;
  /** Sidebar section heading. Defaults to the level's own section (e.g. "Organization"). */
  group?: string;
  order: number;
  isVisible?: (scope: ConsoleScope) => boolean;
};

const ExtensionsContext = createContext<readonly ApiControlPlaneExtension[]>(
  []
);

export function ExtensionsProvider({
  extensions,
  children,
}: {
  extensions: readonly ApiControlPlaneExtension[];
  children: ReactNode;
}) {
  return (
    <ExtensionsContext.Provider value={extensions}>
      {children}
    </ExtensionsContext.Provider>
  );
}

export function useExtensions(): readonly ApiControlPlaneExtension[] {
  return useContext(ExtensionsContext);
}

/**
 * Prefixes an extension's `routePath` with the URL shape for its `level`
 * (organization/project/api), so both `AppRoutes` (route patterns, `orgHandle`
 * etc. as `:param` placeholders) and the nav pipeline (concrete scope values)
 * build the same URL shape from one place.
 *
 * A `null` project/API handle drops that scope's segments, exactly as it does
 * for the built-in `routes.*` builders — see `ScopeHandle` in `routes/paths.ts`.
 * That is what lets an extension page be reached from a shallower scope and
 * render `ScopeGate` to ask for the rest.
 */
export function buildScopedExtensionPath(
  level: NavigationLevel,
  routeSuffix: string,
  params: {
    orgHandle: string;
    projectHandler?: ScopeHandle;
    apiHandler?: ScopeHandle;
  }
): string {
  if (level === 'organization') {
    return `/organizations/${params.orgHandle}/${routeSuffix}`;
  }
  if (level === 'project') {
    return projectPath(params.orgHandle, params.projectHandler ?? null, routeSuffix);
  }
  return apiPath(
    params.orgHandle,
    params.projectHandler ?? null,
    params.apiHandler ?? null,
    routeSuffix
  );
}

/**
 * Every route pattern an extension page answers on: its fully-scoped path plus
 * the scope-less aliases for whichever scopes its level requires. Mirrors
 * `projectScopedPaths`/`apiScopedPaths` for the built-in pages.
 */
export function extensionScopedPaths(
  level: NavigationLevel,
  routeSuffix: string
): string[] {
  const build = (projectHandler: ScopeHandle, apiHandler: ScopeHandle) =>
    buildScopedExtensionPath(level, routeSuffix, {
      apiHandler,
      orgHandle: ':orgHandle',
      projectHandler,
    });

  if (level === 'organization') return [build(null, null)];
  if (level === 'project') {
    return [build(':projectHandler', null), build(null, null)];
  }
  return [
    build(':projectHandler', ':apiHandler'),
    build(':projectHandler', null),
    build(null, null),
  ];
}
