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

import { AppShell } from '@wso2/oxygen-ui';
import { describe, expect, it } from 'vitest';

import type { RestApi } from '@/api/resources/restApis';
import { routes } from '@/routes/paths';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen } from '@/test/utils';
import { AppSidebar } from './AppSidebar';
import { anOrganization, aProject } from '@/test/msw/fixtures';

const ORG = anOrganization().id;
const PROJECT = aProject().id;
const API = 'api-1';

// Lowercase transports, exactly as the API sends them: an upper-case-only
// reachability check made `canTest` false and hid the whole Test menu once the
// scope gate had been passed. Keeping the fixture in the spec's casing means
// these tests fail if that returns.
const COMPONENT = {
  displayName: 'Orders API',
  id: API,
  kind: 'RestApi',
  transport: ['http', 'https'],
} as RestApi;

const renderSidebar = (route: string, isApiScope: boolean) =>
  renderWithProviders(
    <AppShell>
      <AppShell.Sidebar>
        <AppSidebar />
      </AppShell.Sidebar>
    </AppShell>,
    {
      route,
      scope: makeConsoleScope({
        component: COMPONENT,
        isApiScope,
        isProjectScope: isApiScope,
        params: isApiScope
          ? { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT }
          : { orgHandle: ORG },
        // Left unset either way: the sidebar reads scope from the flags and
        // params above, never from the loaded project object.
        project: undefined,
      }),
    },
  );

/*
 * The two halves of a submenu parent, as the user meets them. Which one applies
 * is decided entirely by whether the item has children — see `renderItem`.
 */
describe('AppSidebar submenus', () => {
  it('opens Test as a submenu in API scope rather than navigating', async () => {
    const { user } = renderSidebar(routes.api(ORG, PROJECT, API), true);

    const test = screen.getByRole('button', { name: /^Test$/ });
    // A disclosure, not a link: no href to follow.
    expect(test.closest('a')).toBeNull();
    expect(test).toHaveAttribute('aria-expanded', 'false');

    await user.click(test);

    expect(test).toHaveAttribute('aria-expanded', 'true');
    for (const label of ['API Console', 'Curl', 'API Chat']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it('links Test at its first child scope gate when no API is open', () => {
    renderSidebar(routes.organizationHome(ORG), false);

    const link = screen.getByRole('button', { name: /^Test$/ }).closest('a');
    expect(link).toHaveAttribute('href', routes.apiTestConsole(ORG, null, null));
    // Nothing to disclose, so no submenu entries and no chevron state.
    expect(screen.queryByText('API Console')).not.toBeInTheDocument();
  });

  it('opens Develop onto the three panels lifted off the overview page', async () => {
    const { user } = renderSidebar(routes.api(ORG, PROJECT, API), true);

    await user.click(screen.getByRole('button', { name: /^Develop$/ }));

    for (const label of ['Policies', 'Routing', 'Documents']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it('auto-opens the submenu holding the active page', () => {
    renderSidebar(routes.apiObservabilityLogs(ORG, PROJECT, API), true);

    expect(screen.getByRole('button', { name: /^Observability$/ })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(screen.getByText('Logs')).toBeInTheDocument();
  });
});
