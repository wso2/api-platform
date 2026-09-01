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
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import type { RestApi } from '@/api/resources/restApis';
import {
  ExtensionsProvider,
  type ApiControlPlaneExtension,
} from '../extensions';
import { routes } from '@/routes/paths';
import {
  ConsoleScopeContext,
  type ConsoleScope,
} from '../scope/ConsoleScopeContext';
import { makeConsoleScope } from '@/test/mockScope';
import { renderHook } from '@/test/utils';
import { useNavigationItems } from './useNavigationItems';
import { anOrganization, aProject } from '@/test/msw/fixtures';

const ORG = anOrganization().id;
const PROJECT = aProject().id;
const API = 'api-1';

// A REST API that supports every capability the API-level items gate on, so a
// hidden item in these tests means the scope rules hid it — not the fixture.
// Lowercase transports, as the spec documents them — see AppSidebar.test.tsx.
const COMPONENT = {
  displayName: 'Orders API',
  id: API,
  kind: 'RestApi',
  transport: ['http', 'https'],
} as RestApi;

const atOrg = () =>
  makeConsoleScope({
    component: COMPONENT,
    isApiScope: false,
    isProjectScope: false,
    params: { orgHandle: ORG },
    project: undefined,
  });

const atApi = () =>
  makeConsoleScope({
    component: COMPONENT,
    isApiScope: true,
    params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
  });

const itemsAt = (scope: ConsoleScope, route: string) => {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[route]}>
      <ConsoleScopeContext.Provider value={scope}>
        {children}
      </ConsoleScopeContext.Provider>
    </MemoryRouter>
  );
  const { result } = renderHook(() => useNavigationItems(), { wrapper });
  return result.current;
};

const itemFor = (scope: ConsoleScope, route: string, id: string) => {
  const item = itemsAt(scope, route).find((entry) => entry.id === id);
  if (!item) throw new Error(`No item ${id} at ${route}`);
  return item;
};

/*
 * A submenu parent is two different things depending on scope, and the switch
 * lives here rather than in the sidebar: withholding `children` is what makes
 * Oxygen treat the row as a link instead of a disclosure.
 */
describe('submenu children follow API scope', () => {
  it.each(['develop', 'test', 'insights', 'observability', 'manage'])(
    '%s offers its children once an API is in scope',
    (id) => {
      const item = itemFor(atApi(), routes.api(ORG, PROJECT, API), id);

      expect(item.children?.length).toBeGreaterThan(0);
      // Each child points at its own page under the API.
      for (const child of item.children ?? []) {
        expect(child.to).toContain(`/apis/${API}/`);
      }
    }
  );

  it.each(['develop', 'test', 'insights', 'observability', 'manage'])(
    '%s withholds them outside API scope, and links to the first instead',
    (id) => {
      const item = itemFor(atOrg(), routes.organizationHome(ORG), id);

      expect(item.children).toBeUndefined();
      // The scope-less alias of the first child — where its ScopeGate prompts.
      expect(item.to).toContain('/select-scope/');
    }
  );

  it('marks the child of the open page active, not its parent', () => {
    const route = routes.apiObservabilityLogs(ORG, PROJECT, API);
    const parent = itemFor(atApi(), route, 'observability');

    expect(parent.isActive).toBe(false);
    expect(
      parent.children?.find((child) => child.id === 'observability-logs')
        ?.isActive
    ).toBe(true);
  });

  it('marks the parent active while its scope gate is open', () => {
    const route = routes.apiObservabilityAlerts(ORG, null, null);
    const parent = itemFor(atOrg(), route, 'observability');

    expect(parent.isActive).toBe(true);
    expect(parent.children).toBeUndefined();
  });

  it('leaves items without children untouched', () => {
    const items = itemsAt(atApi(), routes.api(ORG, PROJECT, API));
    const leaves = ['overview', 'gateways', 'deploy', 'admin'];

    for (const id of leaves) {
      expect(items.find((item) => item.id === id)?.children).toBeUndefined();
    }
  });
});

/*
 * Host-injected extensions run through this same pipeline, so the two things
 * that can only go wrong here are covered: which entries reach the sidebar at
 * all, and when one counts as active.
 */
const itemsWithExtensions = (
  scope: ConsoleScope,
  route: string,
  extensions: ApiControlPlaneExtension[]
) => {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[route]}>
      <ConsoleScopeContext.Provider value={scope}>
        <ExtensionsProvider extensions={extensions}>
          {children}
        </ExtensionsProvider>
      </ConsoleScopeContext.Provider>
    </MemoryRouter>
  );
  const { result } = renderHook(() => useNavigationItems(), { wrapper });
  return result.current;
};

const atProject = () =>
  makeConsoleScope({
    component: undefined,
    isApiScope: false,
    isProjectScope: true,
    params: { orgHandle: ORG, projectHandler: PROJECT },
  });

// `routePath: 'environments'` is also the tail segment of an unrelated
// settings-tab route: the collision a substring matcher cannot tell apart,
// since `.../settings/environments` contains `/environments` too even though
// that route belongs to a different feature.
const sidebarExtension: ApiControlPlaneExtension = {
  id: 'environments-sidebar',
  label: 'Environments',
  level: 'project',
  order: 50,
  render: () => <div>Sidebar Environments</div>,
  routePath: 'environments',
  slot: 'sidebar.project',
};

const PROJECT_BASE = `/organizations/${ORG}/projects/${PROJECT}`;

describe('host-injected sidebar extensions', () => {
  it('is active at its own destination', () => {
    const [item] = itemsWithExtensions(
      atProject(),
      `${PROJECT_BASE}/environments`,
      [sidebarExtension]
    ).filter((entry) => entry.id === sidebarExtension.id);

    expect(item?.isActive).toBe(true);
  });

  it('is not active on an unrelated route ending with the same segment', () => {
    const [item] = itemsWithExtensions(
      atProject(),
      `${PROJECT_BASE}/settings/environments`,
      [sidebarExtension]
    ).filter((entry) => entry.id === sidebarExtension.id);

    expect(item?.isActive).toBe(false);
  });

  it('keeps a nested-slot extension out of the sidebar entirely', () => {
    // A `settings.*` entry is rendered by the Settings sub-nav; surfacing it
    // here too would show the same feature in two places.
    const settingsTab: ApiControlPlaneExtension = {
      ...sidebarExtension,
      id: 'environments-settings-tab',
      routePath: 'settings/environments',
      slot: 'settings.project.tabs',
    };

    const items = itemsWithExtensions(
      atProject(),
      `${PROJECT_BASE}/settings/environments`,
      [settingsTab]
    );

    expect(items.find((entry) => entry.id === settingsTab.id)).toBeUndefined();
  });

  it('hides built-in Insights outside API scope when cloud Insights extensions load', () => {
    const orgInsights: ApiControlPlaneExtension = {
      id: 'organization-insights',
      label: 'Insights',
      level: 'organization',
      order: 900,
      render: () => <div>Cloud Insights</div>,
      routePath: 'insights',
      slot: 'sidebar.organization',
      isVisible: (scope) => {
        const typed = scope as {
          isOrganizationScope?: boolean;
          isProjectScope?: boolean;
          isApiScope?: boolean;
        };
        return (
          Boolean(typed.isOrganizationScope) &&
          !typed.isProjectScope &&
          !typed.isApiScope
        );
      },
    };

    const atOrg = () =>
      makeConsoleScope({
        isApiScope: false,
        isProjectScope: false,
        params: { orgHandle: ORG },
        project: undefined,
      });

    const items = itemsWithExtensions(
      atOrg(),
      `/organizations/${ORG}/home`,
      [orgInsights]
    );
    expect(items.find((entry) => entry.id === 'insights')).toBeUndefined();
    expect(items.find((entry) => entry.id === 'organization-insights')).toBeDefined();

    const insightsIndex = items.findIndex(
      (entry) => entry.id === 'organization-insights'
    );
    const observabilityIndex = items.findIndex(
      (entry) => entry.id === 'observability'
    );
    expect(insightsIndex).toBeGreaterThan(-1);
    expect(observabilityIndex).toBeGreaterThan(-1);
    expect(insightsIndex).toBeLessThan(observabilityIndex);
  });

  it('keeps built-in Insights submenu in API scope with cloud extensions loaded', () => {
    const cloudInsights: ApiControlPlaneExtension = {
      id: 'organization-insights',
      label: 'Insights',
      level: 'organization',
      order: 900,
      render: () => <div>Cloud Insights</div>,
      routePath: 'insights',
      slot: 'sidebar.organization',
      isVisible: (scope) => {
        const typed = scope as {
          isOrganizationScope?: boolean;
          isProjectScope?: boolean;
          isApiScope?: boolean;
        };
        return (
          Boolean(typed.isOrganizationScope) &&
          !typed.isProjectScope &&
          !typed.isApiScope
        );
      },
    };

    const atApi = () =>
      makeConsoleScope({
        isApiScope: true,
        isProjectScope: true,
        params: {
          apiHandler: API,
          orgHandle: ORG,
          projectHandler: PROJECT,
        },
        component: COMPONENT,
      });

    const items = itemsWithExtensions(
      atApi(),
      `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/insights/api`,
      [cloudInsights]
    );

    expect(items.find((entry) => entry.id === 'insights')).toBeDefined();
    expect(items.find((entry) => entry.id === 'organization-insights')).toBeUndefined();
  });
});
