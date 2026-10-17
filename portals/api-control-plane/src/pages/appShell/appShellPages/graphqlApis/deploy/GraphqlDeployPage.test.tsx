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
  accepts,
  aDeployment,
  aGateway,
  aGraphQLApiDetail,
  collection,
  recorder,
  resource,
  type DeploymentFixture,
  type Recorder,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlDeployPage } from './GraphqlDeployPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const api = aGraphQLApiDetail({ displayName: 'Countries GraphQL API', id: API, projectId: PROJECT });

const gateway = aGateway({ displayName: 'Edge Gateway', id: 'edge-gateway', isActive: true });

/** Settled on purpose — see `DeployPage.test.tsx` for why a DEPLOYING fixture is avoided. */
const deployment: DeploymentFixture = aDeployment({
  createdAt: '2026-01-01T00:00:00Z',
  deploymentId: 'deployment-1',
  gatewayId: 'edge-gateway',
  name: 'Edge_Gateway_2026-01-01_1',
  status: 'DEPLOYED',
});

let requests: Recorder;

/**
 * This page reads `graphqlApiHandler` from `useParams()` directly rather than
 * from the console scope (see `graphqlApiPath` for why: the route lives
 * outside `ConsoleScopeProvider`'s REST-only `apis` segment matching), so the
 * route has to actually match through real `Route`s, not just an injected
 * scope object.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlDeployPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/deploy"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/deploy`,
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

function serveDeployState(deployments: DeploymentFixture[] = [deployment]) {
  server.use(
    resource('/graphql-apis/:graphqlApiId', api),
    collection('/gateways', [gateway]),
    collection('/graphql-apis/:graphqlApiId/deployments', deployments, { record: requests }),
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('GraphqlDeployPage', () => {
  it('reads deployments from the GraphQL-API-scoped sub-resource and renders the current one', async () => {
    serveDeployState();

    renderPage();

    expect(await screen.findByText('Edge Gateway')).toBeInTheDocument();
    expect(await screen.findAllByText('Edge_Gateway_2026-01-01_1')).not.toHaveLength(0);

    // Deployments hang off the GraphQL API, at its own `graphql-apis` path —
    // never off the REST `rest-apis` sub-resource.
    expect(requests.last()?.url.pathname).toContain(`/graphql-apis/${API}/deployments`);
    expect(requests.last()?.headers.get('X-Org-Id')).toBe(ORG);
  });

  it('deploys to the GraphQL-API-scoped endpoint with the gateway handle in the body', async () => {
    serveDeployState([]);
    const deploys = recorder();
    server.use(
      accepts('post', '/graphql-apis/:graphqlApiId/deployments', deployment, { record: deploys }),
    );

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Deploy' }));

    await expect.poll(() => deploys.count()).toBe(1);
    expect(deploys.last()?.url.pathname).toContain(`/graphql-apis/${API}/deployments`);
    expect(JSON.parse(deploys.last()?.body ?? '{}')).toMatchObject({
      base: 'current',
      gatewayId: 'edge-gateway',
    });
  });

  it('offers gateway creation when the organization has none', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      collection('/gateways', []),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    expect(await screen.findByText('No gateway added yet')).toBeInTheDocument();
  });
});
