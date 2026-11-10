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

import { http as mswHttp, HttpResponse } from 'msw';
import { Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  accepts,
  aGraphQLApiDetail,
  apiUrl,
  aPublication,
  aPublicationDraftDetails,
  aPublicationSummary,
  collection,
  failure,
  recorder,
  resource,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlPublishPage } from './GraphqlPublishPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const api = aGraphQLApiDetail({ displayName: 'Countries GraphQL API', id: API, projectId: PROJECT });

/** Renders the router's current path, so a test can see where the card navigated. */
const LocationProbe = () => <output data-testid="location">{useLocation().pathname}</output>;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlPublishPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/publish"
        />
        <Route
          element={<LocationProbe />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/publish/:apiPortalId"
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

    const { user } = renderPage();

    expect(
      await screen.findByRole('heading', { name: 'Publish Countries GraphQL API' }),
    ).toBeInTheDocument();
    expect(await screen.findByText('Partner Portal')).toBeInTheDocument();
    expect(Object.fromEntries(requests.last()!.params)).toMatchObject({
      apiId: API,
      apiType: 'graphql-api',
    });

    await user.click(screen.getByRole('button', { name: 'More publish options' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Configure & Publish' }));
    await user.click(screen.getByRole('button', { name: 'Configure & Publish' }));

    expect(await screen.findByTestId('location')).toHaveTextContent(
      `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/publish/partner-portal`,
    );
  });

  describe('one-click Publish', () => {
    const BASE = `/api-portals/partner-portal/apis/graphql-api/${API}`;

    /** Nothing saved for the portal; the API's own SDL is `sdl`. */
    function serveFreshApi(sdl: string) {
      server.use(
        resource('/graphql-apis/:graphqlApiId', api),
        collection('/api-publications', [aPublicationSummary({ apiPortalId: 'partner-portal' })]),
        resource('/graphql-apis/:graphqlApiId/sdl', { sdl }),
        collection(`/graphql-apis/${API}/gateways`, []),
        collection(`/apis/graphql-api/${API}/docs`, []),
        mswHttp.get(apiUrl(`/apis/graphql-api/${API}/thumbnail`), () => new HttpResponse(null, { status: 204 })),
        failure('get', `${BASE}/draft`, 404, 'DRAFT_NOT_FOUND'),
        failure('get', `${BASE}/draft/definition`, 404, 'DRAFT_NOT_FOUND'),
        failure('get', `${BASE}/publication`, 404, 'PUBLICATION_NOT_FOUND'),
        failure('get', `${BASE}/publication/definition`, 404, 'PUBLICATION_NOT_FOUND'),
      );
      const writes = { definition: recorder(), draft: recorder(), publish: recorder() };
      server.use(
        accepts('put', `${BASE}/draft`, aPublicationDraftDetails(), { record: writes.draft }),
        accepts('put', `${BASE}/draft/definition`, undefined, { record: writes.definition }),
        accepts('post', `${BASE}/publish`, aPublication(), { record: writes.publish }),
      );
      return writes;
    }

    it('publishes the API’s own SDL', async () => {
      const writes = serveFreshApi('type Query { country: String }');
      const { user } = renderPage();

      await user.click(await screen.findByRole('button', { name: 'Publish' }));

      await waitFor(() => expect(writes.publish.count()).toBe(1));
      expect(writes.definition.last()?.body).toBe('type Query { country: String }');
    });

    it.each([
      ['does not parse', 'type Query {'],
      ['is empty', ''],
      ['has no Query type', 'type Country { name: String }'],
    ])('opens the publish page instead when the API’s SDL %s', async (_case, sdl) => {
      const writes = serveFreshApi(sdl);
      const { user } = renderPage();

      await user.click(await screen.findByRole('button', { name: 'Publish' }));

      expect(await screen.findByTestId('location')).toHaveTextContent(`/graphql-apis/${API}/publish/partner-portal`);
      expect(writes.draft.count() + writes.definition.count() + writes.publish.count()).toBe(0);
    });
  });

  it('shows an error state when the API cannot be found', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }));

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
