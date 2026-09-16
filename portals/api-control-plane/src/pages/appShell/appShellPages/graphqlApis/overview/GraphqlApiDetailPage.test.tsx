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

import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  aDeployment,
  aGateway,
  aGraphQLApiDetail,
  collection,
  resource,
  type DeploymentFixture,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlApiDetailPage } from './GraphqlApiDetailPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const SAMPLE_SDL = 'type Query { countries: [String] }';

const api = aGraphQLApiDetail({
  context: `/${API}/v1.0.0`,
  displayName: 'Countries GraphQL API',
  id: API,
  projectId: PROJECT,
  version: '1.0.0',
});

const gateway = aGateway({ displayName: 'Edge Gateway', id: 'edge-gateway', isActive: true });

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlApiDetailPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}`,
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('GraphqlApiDetailPage', () => {
  it('renders the header and the resolved schema', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/gateways', []),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    expect(
      await screen.findByRole('heading', { name: 'Countries GraphQL API' }),
    ).toBeInTheDocument();
    expect(screen.getByText('v1.0.0')).toBeInTheDocument();
    expect(screen.getByText('GraphQL API')).toBeInTheDocument();
    // From the resolved SDL, via the shared schema explorer.
    expect(await screen.findByText('Query', { exact: false })).toBeInTheDocument();

    const deployLink = screen.getByRole('link', { name: /Deploy to Gateway/ });
    expect(deployLink).toHaveAttribute(
      'href',
      `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/deploy`,
    );

    const editLink = screen.getByRole('link', { name: 'Edit API details' });
    expect(editLink).toHaveAttribute(
      'href',
      `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/edit`,
    );
  });

  it('hides the edit button and shows a Gateway-managed chip for a read-only API', async () => {
    const readOnlyApi = { ...api, readOnly: true };
    server.use(
      resource('/graphql-apis/:graphqlApiId', readOnlyApi),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/gateways', []),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    await screen.findByRole('heading', { name: 'Countries GraphQL API' });
    expect(screen.getByText('Gateway-managed')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Edit API details' })).not.toBeInTheDocument();
  });

  it('shows only the endpoint, not invoke URL or API keys, before anything is deployed', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/gateways', [gateway]),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    await screen.findByRole('heading', { name: 'Countries GraphQL API' });

    expect(screen.getByText('https://upstream.test/graphql')).toBeInTheDocument();
    expect(screen.queryByText('Invoke URL')).not.toBeInTheDocument();
    expect(screen.queryByText('API Keys')).not.toBeInTheDocument();
    expect(screen.queryByText('Deployed gateways')).not.toBeInTheDocument();
  });

  it('shows the invoke URL, API keys and deployed-gateways panels once deployed', async () => {
    const deployment: DeploymentFixture = aDeployment({
      deploymentId: 'deployment-1',
      gatewayId: 'edge-gateway',
      name: 'Edge_Gateway_2026-01-01_1',
      status: 'DEPLOYED',
    });

    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/gateways', [gateway]),
      collection('/graphql-apis/:graphqlApiId/deployments', [deployment]),
      collection('/me/api-keys', []),
    );

    renderPage();

    expect(await screen.findByText('Invoke URL')).toBeInTheDocument();
    expect(screen.getByText('API Keys')).toBeInTheDocument();
    expect(screen.getByText('Deployed gateways')).toBeInTheDocument();
    expect(screen.getAllByText('Edge Gateway').length).toBeGreaterThan(0);
  });

  it('shows an error state when the API cannot be found', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }),
      // `useGraphQLApiSdl`/`useGateways`/`useDeployments` are unconditional
      // hooks — they fire regardless of whether the detail query itself
      // errored, so each still needs a handler here.
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/gateways', []),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
