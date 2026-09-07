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

import { useMemo } from 'react';
import { useLocation } from 'react-router-dom';

import { runtimeConfig } from '../config/runtime';
import {
  useConsoleScope,
  type ConsoleScope,
} from '../scope/ConsoleScopeProvider';
import {
  buildScopedExtensionPath,
  isPageOverride,
  isSidebarExtension,
  useExtensions,
} from '../extensions';
import { navigationRegistry } from './navigationRegistry';
import {
  type NavigationDefinition,
  type NavigationItem,
} from './navigationTypes';

const isFeatureEnabled = (definition: NavigationDefinition) =>
  !definition.featureKey ||
  runtimeConfig.featureFlags.includes(definition.featureKey);

/**
 * Whether an item's `requires` scope holds — the gate on offering its children.
 *
 * An item with no `requires` has no scope condition and is treated as satisfied,
 * so only submenu parents ever consult this.
 */
const isScopeSatisfied = (
  definition: NavigationDefinition,
  scope: ConsoleScope
) => {
  if (definition.requires === 'api') return scope.isApiScope;
  if (definition.requires === 'project') return scope.isProjectScope;
  return true;
};

export const useNavigationItems = (): NavigationItem[] => {
  const scope = useConsoleScope();
  const location = useLocation();
  const extensions = useExtensions();

  return useMemo(() => {
    // Host-injected extensions are converted to the same NavigationDefinition
    // shape the built-in registry uses, so they run through one filter/sort
    // pipeline instead of a parallel "Cloud category" implementation.
    //
    // Only `sidebar.*` entries belong here: an extension registered against a
    // nested slot (e.g. `settings.project.tabs`) renders inside that slot's own
    // host and must not also appear as a top-level sidebar item.
    const extensionDefinitions: NavigationDefinition[] = extensions
      .filter(isSidebarExtension)
      .map((extension) => {
        const isDescendantRoute = extension.routePath.endsWith('/*');
        const routeSuffix = extension.routePath.replace(/\/\*$/, '');
        // The one destination this item points at in the current scope,
        // computed once and used for both `to` and `match`. A raw substring
        // search over the pathname would also fire on an unrelated route that
        // merely ends with the same segment name — a `settings/<name>` tab
        // route would light up a sidebar extension whose own destination is
        // `/<name>` at a different depth.
        const destination = scope.params.orgHandle
          ? buildScopedExtensionPath(extension.level, routeSuffix, {
              apiHandler: scope.params.apiHandler ?? null,
              orgHandle: scope.params.orgHandle,
              projectHandler: scope.params.projectHandler ?? null,
            })
          : undefined;
        return {
          group: extension.group,
          icon: extension.icon,
          id: extension.id,
          isVisible: extension.isVisible,
          label: extension.label,
          level: extension.level,
          match: (pathname) =>
            destination !== undefined &&
            // Exactly this destination, or (for a `/*` route) a path
            // continuing below it — never a partial segment match.
            (pathname === destination ||
              (isDescendantRoute && pathname.startsWith(`${destination}/`))),
          order: extension.order,
          // A missing project/API no longer makes the item unlinkable: the path
          // degrades to the extension page's scope-less alias, where its own
          // `ScopeGate` collects what's missing. Only a route with no
          // organization has nothing to link to.
          to: () => destination,
        };
      });
    // A `page.*` override renders in place of a built-in page (see the
    // `gateways/*` route wrapper); it may also carry the nav placement
    // (`group`/`order`) for the built-in item it replaces, keyed by shared `id`.
    // With no override registered (the open-source build), the built-in item
    // keeps its own placement untouched.
    const overridePlacements = new Map(
      extensions
        .filter(isPageOverride)
        .map((extension) => [extension.id, extension])
    );
    const registryWithOverrides = navigationRegistry.map((definition) => {
      const override = overridePlacements.get(definition.id);
      // `group` is optional on an override — one that only repositions within
      // its existing cluster sets `order` alone — so fall back to the built-in
      // group rather than clearing it and moving the item out of its cluster.
      return override
        ? { ...definition, group: override.group ?? definition.group, order: override.order }
        : definition;
    });
    const combinedRegistry = [...registryWithOverrides, ...extensionDefinitions];

    // A definition becomes an item unless it has no target at all. Children go
    // through the very same resolution — feature flag, visibility, `to`,
    // `isActive` — one level down, so a submenu entry can be flagged off or
    // capability-hidden exactly like a top-level one.
    const resolve = (
      definition: NavigationDefinition
    ): NavigationItem | undefined => {
      if (!isFeatureEnabled(definition)) return undefined;
      if (!(definition.isVisible?.(scope) ?? true)) return undefined;

      const to = definition.to(scope);
      if (!to) return undefined;

      // Children are withheld until their scope holds. That is what makes a
      // parent behave as two different things: a disclosure in scope (the
      // sidebar drops its link once children are present) and an ordinary link
      // to its first child's `ScopeGate` outside it.
      const children =
        definition.children && isScopeSatisfied(definition, scope)
          ? definition.children.reduce<NavigationItem[]>((kept, child) => {
              const item = resolve(child);
              if (item) kept.push(item);
              return kept;
            }, [])
          : undefined;

      return {
        group: definition.group,
        icon: definition.icon,
        id: definition.id,
        isActive: definition.match
          ? definition.match(location.pathname)
          : location.pathname === to,
        label: definition.label,
        to,
        ...(children?.length ? { children } : {}),
      };
    };

    // Items are not filtered by level. An API-level item stays in the sidebar at
    // every scope, linking to its page's scope-less alias so the page's
    // `ScopeGate` can prompt for the missing project/API; a scope-adaptive item
    // links to the deepest tier the route satisfies. The only remaining reason
    // `to` comes back undefined is a route with no organization at all (`/`,
    // `/organizations`), which has nothing to link to yet.
    return combinedRegistry
      .map(resolve)
      .filter(Boolean)
      .sort((left, right) => {
        const leftOrder =
          combinedRegistry.find((item) => item.id === left?.id)?.order ?? 0;
        const rightOrder =
          combinedRegistry.find((item) => item.id === right?.id)?.order ?? 0;
        return leftOrder - rightOrder;
      }) as NavigationItem[];
  }, [location.pathname, scope, extensions]);
};

/**
 * The same items, bucketed into the divider-separated clusters the sidebar
 * renders. Cluster order follows first appearance in the (order-sorted) item
 * list, so the registry's `order` alone decides both item and cluster order.
 *
 * No labels: the clusters exist to separate, not to title. See
 * `NavigationDefinition.group` for why an item can no longer carry a scope
 * heading.
 */
export const useNavigationClusters = (): NavigationItem[][] => {
  const items = useNavigationItems();

  return useMemo(() => {
    const clusters: NavigationItem[][] = [];
    const byKey = new Map<string, NavigationItem[]>();

    for (const item of items) {
      // Items with no cluster of their own share one, rather than each becoming
      // a divider of its own.
      const key = item.group ?? '';
      let cluster = byKey.get(key);
      if (!cluster) {
        cluster = [];
        byKey.set(key, cluster);
        clusters.push(cluster);
      }
      cluster.push(item);
    }

    return clusters;
  }, [items]);
};
