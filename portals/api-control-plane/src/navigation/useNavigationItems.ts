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
import { useConsoleScope } from '../scope/ConsoleScopeProvider';
import { buildScopedExtensionPath, useExtensions } from '../extensions';
import { navigationRegistry } from './navigationRegistry';
import {
  NAVIGATION_GROUP_BY_LEVEL,
  type NavigationDefinition,
  type NavigationGroup,
  type NavigationItem,
} from './navigationTypes';

// const isLevelAvailable = (
//   definition: NavigationDefinition,
//   scope: ReturnType<typeof useConsoleScope>
// ) => {
//   if (definition.level === 'organization') return scope.isOrganizationScope;
//   if (definition.level === 'project') return scope.isProjectScope;
//   return scope.isApiScope;
// };

const isFeatureEnabled = (definition: NavigationDefinition) =>
  !definition.featureKey ||
  runtimeConfig.featureFlags.includes(definition.featureKey);

export const useNavigationItems = (): NavigationItem[] => {
  const scope = useConsoleScope();
  const location = useLocation();
  const extensions = useExtensions();

  return useMemo(() => {
    // Host-injected extensions are converted to the same NavigationDefinition
    // shape the built-in registry uses, so they run through one filter/sort
    // pipeline instead of a parallel "Cloud category" implementation.
    const extensionDefinitions: NavigationDefinition[] = extensions.map(
      (extension) => {
        const isDescendantRoute = extension.routePath.endsWith('/*');
        const routeSuffix = extension.routePath.replace(/\/\*$/, '');
        const routeSegment = `/${routeSuffix}`;
        return {
          group: extension.group,
          icon: extension.icon,
          id: extension.id,
          isVisible: extension.isVisible,
          label: extension.label,
          level: extension.level,
          match: (pathname) => {
            const index = pathname.indexOf(routeSegment);
            if (index === -1) return false;
            const charAfter = pathname[index + routeSegment.length];
            // Match only a complete path segment: nothing after it, or (for
            // a `/*` route) a further `/` continuing into a descendant path.
            return charAfter === undefined || (isDescendantRoute && charAfter === '/');
          },
          order: extension.order,
          to: (navScope) => {
            const { orgHandle, projectHandler, apiHandler } = navScope.params;
            if (!orgHandle) return undefined;
            if (extension.level === 'project' && !projectHandler) return undefined;
            if (
              extension.level === 'api' &&
              (!projectHandler || !apiHandler)
            ) {
              return undefined;
            }
            return buildScopedExtensionPath(extension.level, routeSuffix, {
              apiHandler,
              orgHandle,
              projectHandler,
            });
          },
        };
      }
    );
    const combinedRegistry = [...navigationRegistry, ...extensionDefinitions];

    return combinedRegistry
      // .filter((definition) => isLevelAvailable(definition, scope))
      .filter(isFeatureEnabled)
      .filter((definition) => definition.isVisible?.(scope) ?? true)
      .map((definition) => {
        const to = definition.to(scope);
        // if (!to) return undefined;
        return {
          group: definition.group ?? NAVIGATION_GROUP_BY_LEVEL[definition.level],
          icon: definition.icon,
          id: definition.id,
          isActive: definition.match
            ? definition.match(location.pathname)
            : location.pathname === to,
          label: definition.label,
          to,
        };
      })
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
 * Same items as `useNavigationItems`, bucketed into ordered sidebar sections by
 * their `group`. Group order follows first appearance in the (order-sorted)
 * item list, so Organization → Project → Api falls out naturally.
 */
export const useNavigationGroups = (): NavigationGroup[] => {
  const items = useNavigationItems();

  return useMemo(() => {
    const groups: NavigationGroup[] = [];
    const byLabel = new Map<string, NavigationGroup>();

    for (const item of items) {
      let group = byLabel.get(item.group);
      if (!group) {
        group = { label: item.group, items: [] };
        byLabel.set(item.group, group);
        groups.push(group);
      }
      group.items.push(item);
    }

    return groups;
  }, [items]);
};
