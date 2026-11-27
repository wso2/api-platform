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

import { delay, http, HttpResponse } from 'msw';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { NO_RESOURCE_LIMITS, PortProvider, type CloudHostPort } from '@/hostPort';
import { routes } from '@/routes/paths';
import { aProject, aRestApi, apiUrl, collection, resource } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';

import { ProjectHomePage } from './ProjectHomePage';

const ORG = 'acme';
const PROJECT = 'retail';

const port: CloudHostPort = {
  orgHandle: ORG,
  navigate: () => {},
  notify: () => {},
  apiFetch: async () => undefined as never,
  billing: { organization: async () => null },
  resourceLimits: NO_RESOURCE_LIMITS,
  resources: { invalidate: () => {} },
  permissions: { canAny: () => true },
  trackFeature: () => undefined,
};

function renderHome() {
  return renderWithProviders(
    <PortProvider value={port}>
      <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
        <Routes>
          <Route element={<ProjectHomePage />} path={routes.projectHome()} />
        </Routes>
      </ApiScopeProvider>
    </PortProvider>,
    {
      route: routes.projectHome(ORG, PROJECT),
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

const serveProject = (apis: ReturnType<typeof aRestApi>[]) =>
  server.use(
    resource(`/projects/${PROJECT}`, aProject({ id: PROJECT, displayName: 'Retail APIs' })),
    collection('/rest-apis', apis),
    collection('/graphql-apis', []),
    collection('/gateways', []),
    collection('/api-portals', []),
    http.get(
      apiUrl('/apis/:apiType/:apiId/thumbnail'),
      () => new HttpResponse(null, { status: 204 }),
    ),
  );

beforeEach(() => resetHttpClient());

describe('ProjectHomePage', () => {
  it('shows the project heading and its APIs for a project that has some', async () => {
    serveProject([aRestApi({ id: 'orders-api', displayName: 'Orders API' })]);
    renderHome();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Retail APIs')).toBeInTheDocument();
    expect(screen.queryByText('Govern and manage every API')).not.toBeInTheDocument();
  });

  it('shows the welcome view in place of the project heading for an empty project', async () => {
    serveProject([]);
    renderHome();

    expect(await screen.findByText('Govern and manage every API')).toBeInTheDocument();
    expect(screen.getByText(/Nothing is published yet/)).toBeInTheDocument();
    expect(screen.queryByText(/Owned by/)).not.toBeInTheDocument();
  });

  it('shows a single skeleton, not two loaders, while the project and its APIs load', async () => {
    server.use(
      http.get(apiUrl(`/projects/${PROJECT}`), async () => {
        await delay(150);
        return HttpResponse.json(aProject({ id: PROJECT, displayName: 'Retail APIs' }));
      }),
      collection('/rest-apis', [aRestApi({ id: 'orders-api', displayName: 'Orders API' })]),
      collection('/graphql-apis', []),
      collection('/gateways', []),
      collection('/api-portals', []),
      http.get(
        apiUrl('/apis/:apiType/:apiId/thumbnail'),
        () => new HttpResponse(null, { status: 204 }),
      ),
    );
    const { container } = renderHome();

    // While anything is still loading: one skeleton, and neither spinner label.
    expect(container.querySelector('[aria-busy="true"]')).not.toBeNull();
    expect(screen.queryByText('Loading project')).not.toBeInTheDocument();
    expect(screen.queryByText('Loading APIs')).not.toBeInTheDocument();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(await screen.findByText('Retail APIs')).toBeInTheDocument();
    await waitFor(() => expect(container.querySelector('[aria-busy="true"]')).toBeNull());
  });

  it('shows the welcome view, never an empty project name, when the APIs load before the project', async () => {
    server.use(
      http.get(apiUrl(`/projects/${PROJECT}`), async () => {
        await delay(150);
        return HttpResponse.json(aProject({ id: PROJECT, displayName: 'Retail APIs' }));
      }),
      collection('/rest-apis', []),
      collection('/graphql-apis', []),
      collection('/gateways', []),
      collection('/api-portals', []),
    );
    const { container } = renderHome();

    // The list is empty but the project is not known yet: a skeleton, not "You're in the  project".
    await waitFor(() => expect(container.querySelector('[aria-busy="true"]')).not.toBeNull());
    expect(screen.queryByText(/Nothing is published yet/)).not.toBeInTheDocument();

    expect(await screen.findByText('Govern and manage every API')).toBeInTheDocument();
    expect(screen.getByText('Retail APIs')).toBeInTheDocument();
  });

  it('says the project was not found when it fails to load', async () => {
    server.use(
      http.get(apiUrl(`/projects/${PROJECT}`), () =>
        HttpResponse.json({ message: 'not found' }, { status: 404 }),
      ),
      collection('/rest-apis', []),
      collection('/graphql-apis', []),
    );
    renderHome();

    expect(await screen.findByText('Project not found')).toBeInTheDocument();
  });

  it('forgets the API type filter when the user moves to another project', async () => {
    server.use(
      resource('/projects/retail', aProject({ id: 'retail', displayName: 'Retail APIs' })),
      resource('/projects/empty', aProject({ id: 'empty', displayName: 'Empty project' })),
      // The list is scoped by project, as the real endpoint is.
      http.get(apiUrl('/rest-apis'), ({ request }) => {
        const projectId = new URL(request.url).searchParams.get('projectId');
        const list =
          projectId === 'retail' ? [aRestApi({ id: 'orders-api', displayName: 'Orders API' })] : [];
        return HttpResponse.json({
          list,
          pagination: { total: list.length, offset: 0, limit: 20 },
        });
      }),
      collection('/graphql-apis', []),
      collection('/gateways', []),
      collection('/api-portals', []),
      http.get(
        apiUrl('/apis/:apiType/:apiId/thumbnail'),
        () => new HttpResponse(null, { status: 204 }),
      ),
    );

    function ScopedHome() {
      const { projectHandler } = useParams();
      return (
        <ApiScopeProvider orgId={ORG} projectId={projectHandler}>
          <Link to={routes.projectHome(ORG, 'empty')}>switch project</Link>
          <ProjectHomePage />
        </ApiScopeProvider>
      );
    }
    const { user } = renderWithProviders(
      <PortProvider value={port}>
        <Routes>
          <Route element={<ScopedHome />} path={routes.projectHome()} />
        </Routes>
      </PortProvider>,
      {
        route: routes.projectHome(ORG, 'retail'),
        scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: 'retail' } }),
      },
    );

    // Filter to REST in the project that has APIs…
    await screen.findByText('Orders API');
    await user.click(screen.getByRole('button', { name: /REST/ }));
    // …then move to a project with none: the welcome view, not "No matching APIs".
    await user.click(screen.getByRole('link', { name: 'switch project' }));

    expect(await screen.findByText('Govern and manage every API')).toBeInTheDocument();
    expect(screen.queryByText('No matching APIs')).not.toBeInTheDocument();
  });
});
