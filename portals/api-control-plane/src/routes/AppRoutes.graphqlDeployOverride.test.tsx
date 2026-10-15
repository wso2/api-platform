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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ExtensionsProvider,
  PAGE_GRAPHQL_API_DEPLOY_SLOT,
  type ApiControlPlaneExtension,
} from '../extensions';
import { AppRoutes } from './AppRoutes';
import { routes } from './paths';
import { aGraphQLApiDetail, anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// Covers the `page.graphqlApiDeploy` slot: a host that replaces the REST Deploy
// page replaces the GraphQL one with the same feature, and since the Port's
// `apiHandle` is REST-only, the override is handed the GraphQL API's handle.
describe('AppRoutes GraphQL deploy page override', () => {
  const org = anOrganization({ id: 'api-platform-demo', displayName: 'API Platform Demo' });
  const project = aProject({ id: 'retail-apis', displayName: 'Retail APIs' });
  const api = aGraphQLApiDetail({ id: 'countries-graphql-api', projectId: 'retail-apis' });

  beforeEach(() => {
    vi.stubEnv('VITE_USE_MOCK_API', 'true');
    server.use(
      collection('/organizations', [org]),
      resource('/organizations/:organizationId', org),
      collection('/projects', [project]),
      resource('/projects/:projectId', project),
      collection('/rest-apis', []),
      resource('/graphql-apis/:graphqlApiId', api),
    );
  });
  afterEach(() => vi.unstubAllEnvs());

  const override: ApiControlPlaneExtension = {
    id: 'graphql-api-deploy',
    label: 'Deploy',
    level: 'api',
    order: 0,
    render: (_port, artifactHandle) => <div>Mock cloud deploy for {artifactHandle}</div>,
    routePath: 'deploy',
    slot: PAGE_GRAPHQL_API_DEPLOY_SLOT,
  };

  it('renders the override with the GraphQL API handle from the URL', async () => {
    renderWithProviders(
      <ExtensionsProvider extensions={[override]}>
        <AppRoutes extensions={[override]} />
      </ExtensionsProvider>,
      {
        authState: authStatePresets.authenticated(),
        route: routes.graphqlApiDeploy('api-platform-demo', 'retail-apis', 'countries-graphql-api'),
      },
    );

    expect(
      await screen.findByText('Mock cloud deploy for countries-graphql-api'),
    ).toBeInTheDocument();
  });
});
