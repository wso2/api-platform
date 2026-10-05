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

import { http } from 'msw';
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aGraphQLApiDetail, apiUrl, resource } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { GraphqlDevelopPoliciesPage } from './GraphqlDevelopPoliciesPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const api = aGraphQLApiDetail({
  displayName: 'Countries GraphQL API',
  id: API,
  policies: [{ name: 'cors', version: 'v1', params: {} }],
  projectId: PROJECT,
});

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlDevelopPoliciesPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/develop/policies"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/develop/policies`,
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('GraphqlDevelopPoliciesPage', () => {
  it('lists the attached policy', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: 'type Query { a: String }' }),
    );

    renderPage();

    expect(await screen.findByText('cors')).toBeInTheDocument();
  });

  // Regression test: Save used to be gated on the SDL fetch's loading state
  // (sdlQuery.isPending) unconditionally, even though save() only reads
  // sdlQuery.data for a non-introspection API. For this fixture's
  // introspection-sourced (schemaSource undefined) API, that fetch is never
  // actually needed to save — but a slow/still-pending SDL request used to
  // leave Save stuck disabled even after a policy edit made the form dirty,
  // which looked exactly like "cannot add policies" from the outside.
  it('keeps Save enabled once dirty, even while the (unneeded) SDL fetch is still pending', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', api));
    // Deliberately never resolves within the test — stands in for a slow SDL
    // request. If Save depended on it, the button would still be disabled by
    // the time the assertion below runs.
    server.use(http.get(apiUrl('/graphql-apis/:graphqlApiId/sdl'), () => new Promise(() => {})));

    const { user } = renderPage();

    await user.click(await screen.findByLabelText('Remove policy'));

    expect(await screen.findByRole('button', { name: /Save/ })).toBeEnabled();
  });

  it('lists a gateway-managed API’s policies without any way to change them', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', { ...api, readOnly: true }),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: 'type Query { a: String }' }),
    );

    renderPage();

    expect(await screen.findByText('cors')).toBeInTheDocument();
    expect(screen.getByText('Policies cannot be changed here')).toBeInTheDocument();
    expect(screen.queryByLabelText('Edit policy')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Remove policy')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Save/ })).not.toBeInTheDocument();
  });

  it('shows an error state when the API cannot be found', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }));

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
