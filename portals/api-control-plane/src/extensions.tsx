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
type ExtensionRender = (port: CloudHostPort) => ReactNode;

type ApiControlPlaneExtensionBase = SlotEntry & {
  routePath: string;
  label: string;
  icon?: ReactNode;
  level: NavigationLevel;
  /** Sidebar section heading. Defaults to the level's own section (e.g. "Organization"). */
  group?: string;
  isVisible?: (scope: ConsoleScope) => boolean;
  /**
   * A built-in sidebar item this entry stands in for, named by its id.
   *
   * A built-in may be scoped differently from what a host needs, and two items
   * of the same name side by side would be worse than either. Declaring the
   * claim drops the built-in for as long as this entry is itself visible, so a
   * claim can never remove both.
   */
  claims?: string;
  /**
   * Extra segments, at this entry's `level`, that redirect to its `routePath` —
   * e.g. to keep a page's former URL working after it moved. Sidebar entries
   * only. Query string and hash are carried over.
   */
  aliases?: readonly string[];
};

export type ApiControlPlaneExtension = ApiControlPlaneExtensionBase &
  (
    | {
        render: ExtensionRender;
        /**
         * Sub-items, which turn this entry into a disclosure the way a built-in
         * parent's `children` do. Each is a page of its own, mounted under the
         * parent's `routePath` and at the parent's `level`.
         */
        children?: readonly ApiControlPlaneExtensionChild[];
      }
    | {
        /**
         * Omitted on a parent: a direct hit on its own `routePath` redirects to
         * its first visible child, so the URL always names the page shown and
         * the sidebar highlights it.
         */
        render?: undefined;
        children: readonly [ApiControlPlaneExtensionChild, ...ApiControlPlaneExtensionChild[]];
      }
  );

/** An entry that renders a page of its own — every entry but a childful parent that omits `render`. */
export type RenderableExtension = ApiControlPlaneExtension & { render: ExtensionRender };

export const hasRender = (
  extension: ApiControlPlaneExtension
): extension is RenderableExtension => typeof extension.render === 'function';

/**
 * One page under an extension parent. It carries no `level`, `group` or `order`:
 * all three come from the parent, so a sub-item cannot drift into another scope,
 * another divider cluster, or out of the order it was declared in.
 *
 * `routePath` is its own segment only. The parent's is prefixed for it — see
 * `childRoutePath`.
 */
export type ApiControlPlaneExtensionChild = {
  id: string;
  routePath: string;
  render: ExtensionRender;
  label: string;
  icon?: ReactNode;
  isVisible?: (scope: ConsoleScope) => boolean;
};

/**
 * Where a child page is mounted. Composed here rather than written out at the
 * registration site, so a child cannot name a path outside its parent — a bare
 * `gateways` would otherwise register a top-level route that shadows the
 * built-in page of that name.
 */
export const childRoutePath = (parentRoutePath: string, segment: string): string =>
  `${parentRoutePath.replace(/\/\*$/, '')}/${segment}`;

/**
 * Slot names core knows about. Both live here rather than being spelled out at
 * each use site, so the sidebar route builder and the nav pipeline (and the
 * Settings tab list and its routes) can never drift apart on a string literal.
 */
const SIDEBAR_SLOT_PREFIX = 'sidebar.';

/** The slot a Settings sub-nav tab for `level` attaches to. */
export const settingsTabSlot = (level: NavigationLevel): string =>
  `settings.${level}.tabs`;

/**
 * Slot for overriding the built-in Gateways page (list + create/edit) without
 * changing anything under `pages/appShell/appShellPages/gateways`. Consumed
 * directly by the `gateways/*` route wrapper in `AppRoutes` (via `useSlot`) —
 * not by the sidebar or Settings-tab filters, which only match `sidebar.*` /
 * `settings.*.tabs`. A `page.*` entry therefore rides the same
 * `ApiControlPlaneExtension` shape with no new nav plumbing; only its `render`
 * is used, and its `routePath`/`level` are inert.
 */
export const PAGE_GATEWAYS_SLOT = 'page.gateways';

/**
 * Slot for overriding the built-in API Deploy page. Same arrangement as
 * `PAGE_GATEWAYS_SLOT`: consumed by the `apiDeploy` route wrapper in
 * `AppRoutes`, with `routePath`/`level` inert. The override needs the API in
 * scope, which the Port carries as `apiHandle`.
 */
export const PAGE_API_DEPLOY_SLOT = 'page.apiDeploy';

/**
 * Slot for overriding the built-in per-API Logs page under Observability. Same
 * arrangement as `PAGE_API_DEPLOY_SLOT`: consumed by the
 * `apiObservabilityLogs` route wrapper in `AppRoutes`, with `routePath`/`level`
 * inert.
 */
export const PAGE_API_OBSERVABILITY_LOGS_SLOT = 'page.apiObservabilityLogs';

/** Whether this entry is a top-level sidebar item rather than a nested one. */
export const isSidebarExtension = (
  extension: ApiControlPlaneExtension
): boolean => extension.slot.startsWith(SIDEBAR_SLOT_PREFIX);

/** Prefix for a slot that overrides a built-in page (e.g. `page.gateways`). */
const PAGE_SLOT_PREFIX = 'page.';

/**
 * Whether this entry overrides a built-in page rather than adding a sidebar or
 * Settings-tab item. Consumed by the built-in route wrapper (via `useSlot`) to
 * render in place of the native page; the nav pipeline also uses it to let an
 * override reposition the built-in item it replaces (its `group`/`order`),
 * without changing the item for hosts that register no override.
 */
export const isPageOverride = (extension: ApiControlPlaneExtension): boolean =>
  extension.slot.startsWith(PAGE_SLOT_PREFIX);

/**
 * Entries for `settingsTabSlot(level)`, sorted by `order`.
 *
 * `slot` and `level` must agree: a type-valid but inconsistent descriptor
 * (`slot: 'settings.organization.tabs'` with `level: 'project'`) would
 * otherwise render against the wrong scope's Port, so it is dropped here and
 * in the matching route pass rather than half-honoured. An entry with no
 * `render` has nothing to show in a tab and is dropped the same way.
 */
export const settingsTabExtensions = (
  extensions: readonly ApiControlPlaneExtension[],
  level: NavigationLevel
): RenderableExtension[] =>
  extensions
    .filter(hasRender)
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
