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

import { Outlet, Route, Routes, useParams } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import { ConsoleScopeProvider } from '@/scope/ConsoleScopeProvider';
import { anOrganization, aProject, collection, resource } from '@/test/msw';
import { makeAuthState } from '@/test/mockAuthState';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';

import { OrganizationRedirectPage } from './SystemPages';

const STORAGE_KEY = 'oxygen-ui:header-breadcrumbs';

const remember = (organization: string, project: string) =>
  window.localStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({ version: 1, value: { organization, project }, updatedAt: 1 }),
  );

function ProjectProbe() {
  const { orgHandle, projectHandler } = useParams();
  return <p>{`project-home ${orgHandle}/${projectHandler}`}</p>;
}

function OrganizationProbe() {
  const { orgHandle } = useParams();
  return <p>{`organization-home ${orgHandle}`}</p>;
}

const exchangeOrgToken = vi.fn().mockResolvedValue(true);

function renderLanding() {
  return renderWithProviders(
    <Routes>
      <Route
        element={
          <ConsoleScopeProvider>
            <Outlet />
          </ConsoleScopeProvider>
        }
      >
        <Route element={<OrganizationRedirectPage />} path={routes.organizations} />
        <Route element={<ProjectProbe />} path={routes.projectHome()} />
        <Route element={<OrganizationProbe />} path={routes.organizationHome()} />
      </Route>
    </Routes>,
    {
      route: routes.organizations,
      authState: makeAuthState({
        canSwitchOrganization: true,
        exchangeOrgToken,
        organizations: [
          { id: 'ou-a', handle: 'org-a', name: 'Org A' },
          { id: 'ou-b', handle: 'org-b', name: 'Org B' },
        ],
        user: {
          name: 'Test User',
          email: 'test.user@example.com',
          org: { id: 'ou-a', handle: 'org-a', name: 'Org A' },
        },
      }),
    },
  );
}

const serveProjects = (...projects: ReturnType<typeof aProject>[]) =>
  server.use(
    collection('/organizations', [
      anOrganization({ id: 'org-a', displayName: 'Org A' }),
      anOrganization({ id: 'org-b', displayName: 'Org B' }),
    ]),
    collection('/projects', projects),
    // The project in the URL is read by the scope provider once landed.
    resource('/projects/:projectId', projects[0] ?? aProject()),
  );

beforeEach(() => {
  resetHttpClient();
  window.localStorage.clear();
  exchangeOrgToken.mockClear();
});
afterEach(() => {
  window.localStorage.clear();
  vi.unstubAllGlobals();
});

describe('OrganizationRedirectPage — where users land', () => {
  it('lands on the project the header last remembered', async () => {
    serveProjects(
      aProject({ id: 'retail', updatedAt: '2026-03-01T00:00:00Z' }),
      aProject({ id: 'billing', updatedAt: '2026-01-01T00:00:00Z' }),
    );
    remember('org-a', 'billing');
    renderLanding();

    expect(await screen.findByText('project-home org-a/billing')).toBeInTheDocument();
  });

  it('lands on the most recently changed project when nothing is remembered', async () => {
    serveProjects(
      aProject({ id: 'old', createdAt: '2026-01-01T00:00:00Z' }),
      aProject({
        id: 'fresh',
        createdAt: '2026-01-02T00:00:00Z',
        updatedAt: '2026-04-01T00:00:00Z',
      }),
      aProject({ id: 'newer', createdAt: '2026-02-01T00:00:00Z' }),
    );
    renderLanding();

    expect(await screen.findByText('project-home org-a/fresh')).toBeInTheDocument();
  });

  it('falls back to the latest project when the remembered one is gone', async () => {
    serveProjects(aProject({ id: 'retail', updatedAt: '2026-03-01T00:00:00Z' }));
    remember('org-a', 'deleted-long-ago');
    renderLanding();

    expect(await screen.findByText('project-home org-a/retail')).toBeInTheDocument();
  });

  it('ignores a stored value that is not a project of the organization it names', async () => {
    serveProjects(aProject({ id: 'retail' }));
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ value: { organization: 'org-a' } }));
    renderLanding();

    expect(await screen.findByText('project-home org-a/retail')).toBeInTheDocument();
  });

  it('lands on the organization home when the organization has no projects', async () => {
    serveProjects();
    renderLanding();

    expect(await screen.findByText('organization-home org-a')).toBeInTheDocument();
  });

  it("moves the session to a remembered project's own organization", async () => {
    serveProjects(aProject({ id: 'retail' }));
    remember('org-b', 'billing');
    vi.stubGlobal('location', { ...window.location, replace: vi.fn() });
    renderLanding();

    await vi.waitFor(() => expect(exchangeOrgToken).toHaveBeenCalledWith('org-b'));
  });

  it('does not follow a remembered organization the user does not belong to', async () => {
    serveProjects(aProject({ id: 'retail' }));
    remember('org-elsewhere', 'billing');
    renderLanding();

    expect(await screen.findByText('project-home org-a/retail')).toBeInTheDocument();
    expect(exchangeOrgToken).not.toHaveBeenCalled();
  });
});
