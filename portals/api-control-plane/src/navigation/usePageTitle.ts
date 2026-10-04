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

import { defineMessages, useIntl, type MessageDescriptor } from 'react-intl';
import { matchPath, useLocation } from 'react-router-dom';

import { apiScopedPaths, projectScopedPaths, routes } from '../routes/paths';
import { useNavigationItems } from './useNavigationItems';
import type { NavigationItem } from './navigationTypes';

const messages = defineMessages({
  apis: {
    id: 'apiControlPlane.navigation.usePageTitle.apis',
    defaultMessage: 'APIs',
    description: 'Browser tab title for the API listing page.',
  },
  newApi: {
    id: 'apiControlPlane.navigation.usePageTitle.newApi',
    defaultMessage: 'Create API',
    description: 'Browser tab title for the API creation wizard.',
  },
  apiEdit: {
    id: 'apiControlPlane.navigation.usePageTitle.apiEdit',
    defaultMessage: 'Edit API',
    description: "Browser tab title for the page editing an API's basic information.",
  },
  settings: {
    id: 'apiControlPlane.navigation.usePageTitle.settings',
    defaultMessage: 'Settings',
    description: 'Browser tab title for any Settings tab.',
  },
  monetize: {
    id: 'apiControlPlane.navigation.usePageTitle.monetize',
    defaultMessage: 'Monetize',
    description: 'Browser tab title for the API monetization page.',
  },
  lifecycle: {
    id: 'apiControlPlane.navigation.usePageTitle.lifecycle',
    defaultMessage: 'Lifecycle',
    description: 'Browser tab title for the API lifecycle page.',
  },
  alerts: {
    id: 'apiControlPlane.navigation.usePageTitle.alerts',
    defaultMessage: 'Alerts',
    description: 'Browser tab title for the API observability alerts page.',
  },
  admin: {
    id: 'apiControlPlane.navigation.usePageTitle.admin',
    defaultMessage: 'Admin',
    description: 'Browser tab title for the API admin page.',
  },
});

/**
 * Pages the sidebar cannot name, listed most-specific first.
 *
 * Two kinds end up here: a page with no sidebar item at all (the API listing,
 * the create wizard, the routes behind a pending sidebar entry), and a page
 * whose URL an item's `match` claims but whose title would then be wrong —
 * `/apis/new` matches Overview's `.../apis/:apiHandler` pattern, so without an
 * entry here the creation wizard would read "Overview".
 *
 * Patterns come from the same `routes.*` builders `AppRoutes` registers, via the
 * same `*ScopedPaths` helpers, so a page reached at a scope-less alias is named
 * there too and a renamed route cannot silently stop matching.
 */
const ROUTE_TITLES: readonly { message: MessageDescriptor; patterns: readonly string[] }[] = [
  { message: messages.newApi, patterns: [routes.newApi()] },
  { message: messages.apiEdit, patterns: [routes.apiEdit()] },
  { message: messages.apis, patterns: projectScopedPaths(routes.apis) },
  // Every tab below Settings, including the ones extensions add. The sidebar's
  // own Settings item matches the bare `/settings` path only.
  {
    message: messages.settings,
    patterns: [`${routes.settings()}/*`, `${routes.projectSettings()}/*`],
  },
  { message: messages.monetize, patterns: apiScopedPaths(routes.apiManageMonetize) },
  { message: messages.lifecycle, patterns: apiScopedPaths(routes.apiManageLifecycle) },
  { message: messages.alerts, patterns: apiScopedPaths(routes.apiObservabilityAlerts) },
  { message: messages.admin, patterns: apiScopedPaths(routes.apiAdmin) },
];

/**
 * The deepest active sidebar item's label.
 *
 * A submenu child is preferred over its parent: out of scope the parent owns the
 * highlight (it matches its children's scope-less aliases), so taking the first
 * active item would title every Develop page "Develop".
 */
const activeItemLabel = (items: readonly NavigationItem[]): string | undefined => {
  for (const item of items) {
    const fromChild = item.children ? activeItemLabel(item.children) : undefined;
    if (fromChild) return fromChild;
    if (item.isActive) return item.label;
  }
  return undefined;
};

/**
 * What to call the current page in the browser tab, or `undefined` for a page
 * with no name of its own (the tab then shows the bare product name).
 *
 * Resolution order is explicit-route-title, then sidebar label. The sidebar
 * covers most pages for free — including ones extensions inject — which is what
 * keeps this from becoming a second route table to maintain alongside
 * `AppRoutes`.
 *
 * Sidebar labels are plain strings in `navigationRegistry`, not translated
 * messages, so a title taken from one is as translated as the sidebar itself.
 * Translating the registry fixes both at once.
 */
export const usePageTitle = (): string | undefined => {
  const intl = useIntl();
  const { pathname } = useLocation();
  const items = useNavigationItems();

  const routeTitle = ROUTE_TITLES.find((entry) =>
    entry.patterns.some((pattern) => matchPath(pattern, pathname) !== null),
  );
  if (routeTitle) return intl.formatMessage(routeTitle.message);

  return activeItemLabel(items);
};
