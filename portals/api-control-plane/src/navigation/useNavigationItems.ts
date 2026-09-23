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

import { useMemo, type ReactNode } from 'react';
import { useLocation } from 'react-router-dom';

import { runtimeConfig } from '../config/runtime';
import {
  useConsoleScope,
  type ConsoleScope,
} from '../scope/ConsoleScopeProvider';
import {
  buildScopedExtensionPath,
  childRoutePath,
  isPageOverride,
  isSidebarExtension,
  useExtensions,
  type ApiControlPlaneExtension,
} from '../extensions';
import { navigationRegistry } from './navigationRegistry';
import {
  type NavigationDefinition,
  type NavigationItem,
  type NavigationLevel,
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

const CLOUD_INSIGHTS_SIDEBAR_IDS = new Set([
  'organization-insights',
  'project-insights',
]);

const hasCloudInsightsSidebar = (extensions: readonly ApiControlPlaneExtension[]) =>
  extensions.some(
    (extension) =>
      isSidebarExtension(extension) &&
      CLOUD_INSIGHTS_SIDEBAR_IDS.has(extension.id)
  );

/**
 * The built-in Insights submenu and the cloud org/project Insights extensions
 * both link to Insights outside API scope — keep only the cloud entries then.
 */
const isBuiltinInsightsHiddenByCloudPlugin = (
  definition: NavigationDefinition,
  scope: ConsoleScope,
  cloudInsightsLoaded: boolean
) =>
  definition.id === 'insights' && cloudInsightsLoaded && !scope.isApiScope;

/**
 * Built-in ids that a *visible* sidebar extension has claimed (see
 * `ApiControlPlaneExtension.claims`).
 *
 * Gated on the claimant's own visibility, so an entry that stands down in some
 * scope hands the built-in back rather than removing both. Applied where the two
 * registries are merged: by resolution time a claim and its target can share an
 * id, and the rule could no longer tell them apart.
 */
const claimedBuiltinIds = (
  extensions: readonly ApiControlPlaneExtension[],
  scope: ConsoleScope
) =>
  new Set(
    extensions
      .filter(
        (extension) =>
          isSidebarExtension(extension) &&
          extension.claims &&
          (extension.isVisible?.(scope) ?? true)
      )
      .map((extension) => extension.claims as string)
  );

const warned = new Set<string>();
const warnOnce = (message: string) => {
  if (!import.meta.env.DEV || warned.has(message)) return;
  warned.add(message);
  console.warn(message);
};

/**
 * Dev-only checks on how sidebar extensions relate to the built-in registry: a
 * `claims` naming no built-in item does nothing (usually a typo), and an
 * extension sharing a built-in's id without claiming it puts two items with one
 * id in the sidebar.
 */
const warnOnRegistryConflicts = (extensions: readonly ApiControlPlaneExtension[]) => {
  const builtinIds = new Set(navigationRegistry.map((definition) => definition.id));
  for (const extension of extensions.filter(isSidebarExtension)) {
    if (extension.claims && !builtinIds.has(extension.claims)) {
      warnOnce(
        `Sidebar extension "${extension.id}" claims "${extension.claims}", which is not a built-in item; the claim has no effect.`
      );
    }
    if (builtinIds.has(extension.id) && extension.claims !== extension.id) {
      warnOnce(
        `Sidebar extension "${extension.id}" shares its id with a built-in item without claiming it; both will be shown. Set \`claims: '${extension.id}'\` to replace the built-in.`
      );
    }
  }
};

export const useNavigationItems = (): NavigationItem[] => {
  const scope = useConsoleScope();
  const location = useLocation();
  const extensions = useExtensions();

  return useMemo(() => {
    // Host-injected extensions are converted to the same NavigationDefinition
    // shape the built-in registry uses, so they run through one filter/sort
    // pipeline instead of a parallel implementation.
    //
    // `level` and `group` are arguments rather than fields of `entry`: a child
    // has neither of its own and must inherit the parent's.
    const toDefinition = (
      entry: {
        icon?: ReactNode;
        id: string;
        isVisible?: ApiControlPlaneExtension['isVisible'];
        label: string;
        routePath: string;
      },
      level: NavigationLevel,
      group: string | undefined,
      order: number
    ): NavigationDefinition => {
      const isDescendantRoute = entry.routePath.endsWith('/*');
      const routeSuffix = entry.routePath.replace(/\/\*$/, '');
      // The one destination this item points at in the current scope, computed
      // once and used for both `to` and `match`. A raw substring search over the
      // pathname would also fire on an unrelated route that merely ends with the
      // same segment name — a `settings/<name>` tab route would light up a
      // sidebar extension whose own destination is `/<name>` at a different depth.
      const destination = scope.params.orgHandle
        ? buildScopedExtensionPath(level, routeSuffix, {
            apiHandler: scope.params.apiHandler ?? null,
            orgHandle: scope.params.orgHandle,
            projectHandler: scope.params.projectHandler ?? null,
          })
        : undefined;
      return {
        group,
        icon: entry.icon,
        id: entry.id,
        isVisible: entry.isVisible,
        label: entry.label,
        level,
        match: (pathname) =>
          destination !== undefined &&
          // Exactly this destination, or (for a `/*` route) a path continuing
          // below it — never a partial segment match.
          (pathname === destination ||
            (isDescendantRoute && pathname.startsWith(`${destination}/`))),
        order,
        // A missing project/API no longer makes the item unlinkable: the path
        // degrades to the extension page's scope-less alias, where its own
        // `ScopeGate` collects what's missing. Only a route with no organization
        // has nothing to link to.
        to: () => destination,
      };
    };

    // Only `sidebar.*` entries belong here: an extension registered against a
    // nested slot (e.g. `settings.project.tabs`) renders inside that slot's own
    // host and must not also appear as a top-level sidebar item.
    const extensionDefinitions: NavigationDefinition[] = extensions
      .filter(isSidebarExtension)
      .map((extension) => ({
        ...toDefinition(extension, extension.level, extension.group, extension.order),
        // Ordered by declaration: `order` sorts top-level items only.
        ...(extension.children?.length
          ? {
              children: extension.children.map((child, index) =>
                toDefinition(
                  { ...child, routePath: childRoutePath(extension.routePath, child.routePath) },
                  extension.level,
                  extension.group,
                  index
                )
              ),
            }
          : {}),
      }));
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
    warnOnRegistryConflicts(extensions);
    const cloudInsightsLoaded = hasCloudInsightsSidebar(extensions);
    const claimed = claimedBuiltinIds(extensions, scope);
    const combinedRegistry = [
      ...registryWithOverrides.filter((definition) => !claimed.has(definition.id)),
      ...extensionDefinitions,
    ];

    // A definition becomes an item unless it has no target at all. Children go
    // through the very same resolution — feature flag, visibility, `to`,
    // `isActive` — one level down, so a submenu entry can be flagged off or
    // capability-hidden exactly like a top-level one.
    const resolve = (
      definition: NavigationDefinition
    ): NavigationItem | undefined => {
      if (!isFeatureEnabled(definition)) return undefined;
      if (
        isBuiltinInsightsHiddenByCloudPlugin(
          definition,
          scope,
          cloudInsightsLoaded
        )
      ) {
        return undefined;
      }
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
