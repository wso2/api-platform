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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Outlet, Route, Routes } from 'react-router-dom';

import { resetHttpClient } from '../api/core/http';
import { routes } from '../routes/paths';
import { anOrganization, collection } from '../test/msw';
import { OrganizationSwitchError, type AuthState } from '../contexts/auth/authTypes';
import { authStatePresets, makeAuthState } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen, waitFor } from '../test/utils';
import { OrganizationRedirectPage } from '../pages/appShell/appShellPages/system/SystemPages';
import { ConsoleScopeProvider, useConsoleScope } from './ConsoleScopeProvider';

function OrganizationProbe() {
  const { organization, params } = useConsoleScope();
  return <p>{`${params.orgHandle}:${organization?.displayName ?? 'none'}`}</p>;
}

function renderScope(route: string, authState: AuthState = authStatePresets.authenticated()) {
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
    { route, authState }
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

describe('ConsoleScopeProvider organization switching', () => {
  const orgA = { id: 'ou-a', handle: 'org-a', name: 'Org A' };
  const orgB = { id: 'ou-b', handle: 'org-b', name: 'Org B' };

  /** A session in `org` that can move between organizations. */
  const switchingSession = (org: typeof orgA, exchangeOrgToken: AuthState['exchangeOrgToken']) =>
    makeAuthState({
      canSwitchOrganization: true,
      exchangeOrgToken,
      organizations: [orgA, orgB],
      user: { name: 'Test User', email: 'test.user@example.com', org },
    });

  beforeEach(() => {
    resetHttpClient();
    // The Platform API has not listed org-b for this user yet.
    server.use(
      collection('/organizations', [anOrganization({ id: 'org-a', displayName: 'Org A' })]),
      collection('/projects', []),
    );
  });

  it('renders the organization the session is in, without switching', async () => {
    const exchangeOrgToken = vi.fn().mockResolvedValue(true);
    renderScope(routes.organizationHome('org-a'), switchingSession(orgA, exchangeOrgToken));

    expect(await screen.findByText('org-a:Org A')).toBeInTheDocument();
    expect(exchangeOrgToken).not.toHaveBeenCalled();
  });

  it('moves the session into the URL organization before rendering it', async () => {
    const exchangeOrgToken = vi.fn(() => new Promise<boolean>(() => {}));
    renderScope(routes.organizationHome('org-b'), switchingSession(orgA, exchangeOrgToken));

    expect(await screen.findByText('Switching organization')).toBeInTheDocument();
    expect(exchangeOrgToken).toHaveBeenCalledWith('org-b');
    expect(screen.queryByText(/^org-b:/)).not.toBeInTheDocument();
  });

  // Membership comes from the identity provider: the Platform API's list
  // lacking org-b must not bounce a session that is already in it.
  it('keeps an organization the Platform API has not listed yet', async () => {
    renderScope(routes.organizationHome('org-b'), switchingSession(orgB, vi.fn()));

    expect(await screen.findByText(/^org-b:/)).toBeInTheDocument();
  });

  it('sends the user to the organization picker when the session may not move there', async () => {
    const exchangeOrgToken = vi.fn().mockRejectedValue(new OrganizationSwitchError('forbidden'));
    renderScope(routes.organizationHome('org-c'), switchingSession(orgA, exchangeOrgToken));

    expect(await screen.findByText('organization picker')).toBeInTheDocument();
  });

  it('reports a switch that did not complete', async () => {
    const exchangeOrgToken = vi.fn().mockRejectedValue(new OrganizationSwitchError('unavailable'));
    renderScope(routes.organizationHome('org-b'), switchingSession(orgA, exchangeOrgToken));

    expect(await screen.findByText('Unable to switch organization')).toBeInTheDocument();
    expect(screen.queryByText(/^org-b:/)).not.toBeInTheDocument();
  });
});

describe('ConsoleScopeProvider and the organization picker after a refused switch', () => {
  const STALE = { id: 'ou-stale', handle: 'org-stale', name: 'Stale' };
  const OTHER = { id: 'ou-b', handle: 'org-b', name: 'Org B' };

  function renderWithPicker(authState: AuthState) {
    return renderWithProviders(
      <Routes>
        <Route
          element={
            <ConsoleScopeProvider>
              <Outlet />
            </ConsoleScopeProvider>
          }
        >
          <Route path={routes.organizations} element={<OrganizationRedirectPage />} />
          <Route path={routes.organizationHome()} element={<OrganizationProbe />} />
        </Route>
      </Routes>,
      { route: routes.organizations, authState },
    );
  }

  beforeEach(() => {
    resetHttpClient();
    server.use(collection('/projects', []));
  });

  // The Platform API still lists an organization the user can no longer use and
  // not yet the session's own: the picker must not keep choosing the stale one.
  it("lands on the session's own organization, which needs no switch", async () => {
    server.use(collection('/organizations', [anOrganization({ id: 'org-stale', displayName: 'Stale' })]));
    const exchangeOrgToken = vi.fn().mockRejectedValue(new OrganizationSwitchError('forbidden'));
    renderWithPicker(
      makeAuthState({
        canSwitchOrganization: true,
        exchangeOrgToken,
        organizations: [STALE, OTHER],
        user: { name: 'Test User', email: 'test.user@example.com', org: { id: 'ou-a', handle: 'org-a', name: 'Org A' } },
      }),
    );

    expect(await screen.findByText(/^org-a:/)).toBeInTheDocument();
    expect(exchangeOrgToken).not.toHaveBeenCalled();
  });

  it('never picks an organization the session was refused again', async () => {
    server.use(
      collection('/organizations', [
        anOrganization({ id: 'org-stale', displayName: 'Stale' }),
        anOrganization({ id: 'org-b', displayName: 'Org B' }),
      ]),
    );
    const exchangeOrgToken = vi.fn((handle: string) =>
      handle === 'org-stale'
        ? Promise.reject(new OrganizationSwitchError('forbidden'))
        : new Promise<boolean>(() => {}),
    );
    // No organization claim, so there is no session organization to fall back on.
    renderWithPicker(makeAuthState({ canSwitchOrganization: true, exchangeOrgToken, organizations: [STALE, OTHER] }));

    await waitFor(() => expect(exchangeOrgToken).toHaveBeenCalledWith('org-b'));
    expect(exchangeOrgToken.mock.calls.filter(([handle]) => handle === 'org-stale')).toHaveLength(1);
    expect(screen.getByText('Switching organization')).toBeInTheDocument();
  });
});
