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
import { aGraphQLApiDetail, resource } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlDevelopDocumentsPage } from './GraphqlDevelopDocumentsPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const api = aGraphQLApiDetail({ displayName: 'Countries GraphQL API', id: API, projectId: PROJECT });

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlDevelopDocumentsPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/develop/documents"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/develop/documents`,
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('GraphqlDevelopDocumentsPage', () => {
  it('shows the coming-soon placeholder', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', api));

    renderPage();

    expect(
      await screen.findByText('Documents for this API will be available soon.'),
    ).toBeInTheDocument();
  });

  it('shows an error state when the API cannot be found', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }));

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
