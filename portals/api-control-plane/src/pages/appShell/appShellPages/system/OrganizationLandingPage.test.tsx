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

import { Route, Routes, useParams } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import { aProject, collection, failure } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';

import { OrganizationLandingPage } from './OrganizationLandingPage';

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

function renderLanding(org = 'org-b') {
  return renderWithProviders(
    <ApiScopeProvider orgId={org}>
      <Routes>
        <Route element={<OrganizationLandingPage />} path={routes.organizationLanding()} />
        <Route element={<ProjectProbe />} path={routes.projectHome()} />
        <Route element={<OrganizationProbe />} path={routes.organizationHome()} />
      </Routes>
    </ApiScopeProvider>,
    { route: routes.organizationLanding(org), scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  resetHttpClient();
  window.localStorage.clear();
});
afterEach(() => window.localStorage.clear());

/** More projects than one page holds, so the whole set cannot arrive at once. */
const manyProjects = (count: number) =>
  Array.from({ length: count }, (_, index) =>
    aProject({
      id: `project-${index}`,
      createdAt: new Date(Date.UTC(2026, 0, 1, 0, index)).toISOString(),
    }),
  );

describe('OrganizationLandingPage', () => {
  it('lands on the remembered project when it is still there', async () => {
    server.use(collection('/projects', [aProject({ id: 'retail' }), aProject({ id: 'billing' })]));
    remember('org-b', 'billing');
    renderLanding();

    expect(await screen.findByText('project-home org-b/billing')).toBeInTheDocument();
  });

  it('falls back to the latest project when the remembered one was deleted', async () => {
    server.use(
      collection('/projects', [
        aProject({ id: 'retail', updatedAt: '2026-03-01T00:00:00Z' }),
        aProject({ id: 'older', updatedAt: '2026-01-01T00:00:00Z' }),
      ]),
    );
    remember('org-b', 'billing');
    renderLanding();

    expect(await screen.findByText('project-home org-b/retail')).toBeInTheDocument();
  });

  it('ignores a remembered project of another organization', async () => {
    server.use(collection('/projects', [aProject({ id: 'billing' }), aProject({ id: 'retail' })]));
    remember('org-a', 'billing');
    renderLanding();

    // Not "billing" by name alone: the one it was remembered for is org-a's.
    expect(await screen.findByText(/^project-home org-b\//)).toBeInTheDocument();
  });

  it('finds a remembered project beyond the first page of projects', async () => {
    server.use(collection('/projects', manyProjects(230)));
    remember('org-b', 'project-215');
    renderLanding();

    expect(await screen.findByText('project-home org-b/project-215')).toBeInTheDocument();
  });

  it('picks the latest project from every page, not only the first', async () => {
    const projects = manyProjects(230);
    projects[205] = { ...projects[205], updatedAt: '2030-01-01T00:00:00Z' };
    server.use(collection('/projects', projects));
    renderLanding();

    expect(await screen.findByText('project-home org-b/project-205')).toBeInTheDocument();
  });

  it('lands on the organization home when it has no projects', async () => {
    server.use(collection('/projects', []));
    renderLanding();

    expect(await screen.findByText('organization-home org-b')).toBeInTheDocument();
  });

  it('lands on the organization home when the projects cannot be read', async () => {
    server.use(failure('get', '/projects', 500, 'INTERNAL_ERROR'));
    renderLanding();

    expect(await screen.findByText('organization-home org-b')).toBeInTheDocument();
  });
});
