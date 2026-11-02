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
  aGraphQLApiDetail,
  aPublicationSummary,
  collection,
  recorder,
  resource,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlPublishPage } from './GraphqlPublishPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const api = aGraphQLApiDetail({ displayName: 'Countries GraphQL API', id: API, projectId: PROJECT });

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlPublishPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/publish"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/publish`,
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('GraphqlPublishPage', () => {
  it('lists the API Portals with this GraphQL API’s own publication status', async () => {
    const requests = recorder();
    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      collection(
        '/api-publications',
        [aPublicationSummary({ apiPortalId: 'partner-portal', apiPortalName: 'Partner Portal' })],
        { record: requests },
      ),
    );

    renderPage();

    expect(
      await screen.findByRole('heading', { name: 'Publish Countries GraphQL API' }),
    ).toBeInTheDocument();
    expect(await screen.findByText('Partner Portal')).toBeInTheDocument();
    expect(Object.fromEntries(requests.last()!.params)).toMatchObject({
      apiId: API,
      apiType: 'graphql-api',
    });
    expect(screen.getByRole('link', { name: 'Go to Publish' })).toHaveAttribute(
      'href',
      `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/publish/partner-portal`,
    );
  });

  it('shows an error state when the API cannot be found', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }));

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
