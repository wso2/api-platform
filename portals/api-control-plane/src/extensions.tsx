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
 */
export function buildScopedExtensionPath(
  level: NavigationLevel,
  routeSuffix: string,
  params: { orgHandle: string; projectHandler?: string; apiHandler?: string }
): string {
  if (level === 'organization') {
    return `/organizations/${params.orgHandle}/${routeSuffix}`;
  }
  if (level === 'project') {
    return `/organizations/${params.orgHandle}/projects/${params.projectHandler}/${routeSuffix}`;
  }
  return `/organizations/${params.orgHandle}/projects/${params.projectHandler}/apis/${params.apiHandler}/${routeSuffix}`;
}
