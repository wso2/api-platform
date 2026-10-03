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

import { beforeEach, describe, expect, it } from 'vitest';
import { Outlet, Route, Routes } from 'react-router-dom';

import { resetHttpClient } from '../api/core/http';
import { routes } from '../routes/paths';
import { anOrganization, collection } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';
import { ConsoleScopeProvider, useConsoleScope } from './ConsoleScopeProvider';

function OrganizationProbe() {
  const { organization, params } = useConsoleScope();
  return <p>{`${params.orgHandle}:${organization?.displayName ?? 'none'}`}</p>;
}

function renderScope(route: string) {
  return renderWithProviders(
    <Routes>
      <Route
        element={
          <ConsoleScopeProvider>
            <Outlet />
          </ConsoleScopeProvider>
        }
      >
        <Route path={routes.organizations} element={<p>organization picker</p>} />
        <Route path={routes.organizationHome()} element={<OrganizationProbe />} />
      </Route>
    </Routes>,
    { route, authState: authStatePresets.authenticated() }
  );
}

describe('ConsoleScopeProvider organization membership', () => {
  const org = anOrganization({ id: 'org-b', displayName: 'Org B' });

  beforeEach(() => {
    resetHttpClient();
    server.use(collection('/organizations', [org]), collection('/projects', []));
  });

  it('keeps an organization the signed-in user belongs to', async () => {
    renderScope(routes.organizationHome('org-b'));

    expect(await screen.findByText('org-b:Org B')).toBeInTheDocument();
  });

  // An account switch can land the new user on the previous user's org URL
  // (login return path, bookmark, another tab); it must not render as theirs.
  it('redirects away from an organization the signed-in user does not belong to', async () => {
    renderScope(routes.organizationHome('org-a'));

    expect(await screen.findByText('organization picker')).toBeInTheDocument();
    expect(screen.queryByText(/^org-a:/)).not.toBeInTheDocument();
  });
});
