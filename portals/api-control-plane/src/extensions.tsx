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

import type { ReactNode } from 'react';

import { apiPath, projectPath, type ScopeHandle } from './routes/paths';
import type { ConsoleScope } from './scope/ConsoleScopeProvider';
import type { NavigationLevel } from './navigation/navigationTypes';
import type { CloudHostPort } from './hostPort';
import { SlotEntriesProvider, useSlotEntries, type SlotEntry } from './slots';

/**
 * A host-injected feature. `routePath` is relative to the same route group the
 * built-in pages live in (e.g. `"billing"` or `"settings/environments"`, never
 * an absolute `/organizations/...` path), and `level` decides the URL shape
 * (organization/project/api) the same way the built-in pages' own `level` does.
 *
 * `slot` is the named extension point this entry attaches to (see
 * `slots/index.tsx`) — e.g. `"sidebar.project"` for a top-level project nav
 * item, or `"settings.project.tabs"` to appear as a Settings sub-nav tab. New
 * slot names can be introduced by core without changing this type.
 *
 * `render` receives the small, portable `CloudHostPort` (org/project handle,
 * navigate, notify) instead of a pre-built element, so the same feature
 * component can be reused by another host app without depending on this
 * portal's own hooks — see `hostPort.tsx`.
 */
export type ApiControlPlaneExtension = SlotEntry & {
  routePath: string;
  render: (port: CloudHostPort) => ReactNode;
  label: string;
  icon?: ReactNode;
  level: NavigationLevel;
  /** Sidebar section heading. Defaults to the level's own section (e.g. "Organization"). */
  group?: string;
  isVisible?: (scope: ConsoleScope) => boolean;
};

/**
 * Slot names core knows about. Both live here rather than being spelled out at
 * each use site, so the sidebar route builder and the nav pipeline (and the
 * Settings tab list and its routes) can never drift apart on a string literal.
 */
const SIDEBAR_SLOT_PREFIX = 'sidebar.';

/** The slot a Settings sub-nav tab for `level` attaches to. */
export const settingsTabSlot = (level: NavigationLevel): string =>
  `settings.${level}.tabs`;

/** Whether this entry is a top-level sidebar item rather than a nested one. */
export const isSidebarExtension = (
  extension: ApiControlPlaneExtension
): boolean => extension.slot.startsWith(SIDEBAR_SLOT_PREFIX);

/**
 * Entries for `settingsTabSlot(level)`, sorted by `order`.
 *
 * `slot` and `level` must agree: a type-valid but inconsistent descriptor
 * (`slot: 'settings.organization.tabs'` with `level: 'project'`) would
 * otherwise render against the wrong scope's Port, so it is dropped here and
 * in the matching route pass rather than half-honoured.
 */
export const settingsTabExtensions = (
  extensions: readonly ApiControlPlaneExtension[],
  level: NavigationLevel
): ApiControlPlaneExtension[] =>
  extensions
    .filter(
      (extension) =>
        extension.slot === settingsTabSlot(level) && extension.level === level
    )
    .sort((left, right) => left.order - right.order);

export function ExtensionsProvider({
  extensions,
  children,
}: {
  extensions: readonly ApiControlPlaneExtension[];
  children: ReactNode;
}) {
  return (
    <SlotEntriesProvider entries={extensions}>{children}</SlotEntriesProvider>
  );
}

export function useExtensions(): readonly ApiControlPlaneExtension[] {
  return useSlotEntries<ApiControlPlaneExtension>();
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
