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

import { http, HttpResponse } from 'msw';
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  aRestApi,
  apiUrl,
  collection,
  manyRestApis,
  recorder,
  type RestApiFixture,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { makeConsoleScope } from '@/test/mockScope';
import { ApiListPage } from './ApiListPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';

const apiFixtures: RestApiFixture[] = [
  aRestApi({ id: 'orders-api', displayName: 'Orders API' }),
  aRestApi({ id: 'inventory-api', displayName: 'Inventory API' }),
];

/** Enough APIs to force a second page at the default size of 12. */
const manyApis = manyRestApis(14);

/**
 * The spec's `query` parameter is a substring match on the API's id (handle),
 * not its display name — the handler has to mirror that or the search test
 * would pass against behaviour the server does not have.
 */
const matchesHandle = (api: RestApiFixture, term: string) =>
  (api.id ?? '').toLowerCase().includes(term);

let requests: Recorder;

/**
 * The page's hooks read `ApiScopeContext`, not the console scope, so the
 * provider has to be mounted here — without it the query stays
 * `enabled: false` and the page renders its loading state forever.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <Routes>
        <Route
          element={<ApiListPage />}
          path="/organizations/:orgHandle/projects/:projectHandler/apis"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/apis`,
      scope: makeConsoleScope(),
    },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('ApiListPage', () => {
  it('renders the first page and asks the server for the paging window', async () => {
    server.use(collection('/rest-apis', apiFixtures, { record: requests }));
    renderPage();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Inventory API')).toBeInTheDocument();
    expect(requests.last()?.params.get('limit')).toBe('12');
    expect(requests.last()?.params.get('offset')).toBe('0');
  });

  it('counts every match from pagination.total, not the cards on screen', async () => {
    server.use(collection('/rest-apis', manyApis));
    renderPage();

    // 14 APIs exist; only 12 fit the first page.
    expect(await screen.findByText('14')).toBeInTheDocument();
  });

  it('searches server-side rather than filtering the current page', async () => {
    server.use(
      collection('/rest-apis', apiFixtures, {
        matches: matchesHandle,
        record: requests,
      }),
    );
    const { user } = renderPage();

    await screen.findByText('Orders API');
    await user.type(screen.getByPlaceholderText('Search APIs'), 'inventory');

    await waitFor(() => expect(requests.last()?.params.get('query')).toBe('inventory'));
    await waitFor(() => expect(screen.queryByText('Orders API')).not.toBeInTheDocument());
    expect(screen.getByText('Inventory API')).toBeInTheDocument();
  });

  it('requests APIs newest-first', async () => {
    server.use(collection('/rest-apis', apiFixtures, { record: requests }));
    renderPage();

    await screen.findByText('Orders API');
    expect(requests.last()?.params.get('sortBy')).toBe('createdAt');
    expect(requests.last()?.params.get('sortOrder')).toBe('desc');
  });

  it('requests the next page when the pagination control advances', async () => {
    server.use(collection('/rest-apis', manyApis, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('API 1');
    expect(screen.queryByText('API 13')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /next page/i }));

    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('12'));
    expect(await screen.findByText('API 13')).toBeInTheDocument();
  });

  it('returns to the first page when the search changes', async () => {
    server.use(
      collection('/rest-apis', manyApis, {
        matches: matchesHandle,
        record: requests,
      }),
    );
    const { user } = renderPage();

    await screen.findByText('API 1');
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('12'));

    await user.type(screen.getByPlaceholderText('Search APIs'), 'api');

    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('0'));
  });

  it('keeps the create prompt for an empty project but not for a missed search', async () => {
    server.use(collection('/rest-apis', apiFixtures, { matches: matchesHandle }));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    await user.type(screen.getByPlaceholderText('Search APIs'), 'nothing');

    expect(await screen.findByText('No matching APIs')).toBeInTheDocument();
    expect(screen.queryByText('Create your first API')).not.toBeInTheDocument();
  });

  it('shows the empty state when the project has no APIs', async () => {
    server.use(collection('/rest-apis', []));
    renderPage();

    expect(await screen.findByText('Create your first API')).toBeInTheDocument();
  });

  it('offers exactly one create button on the empty state', async () => {
    server.use(collection('/rest-apis', []));
    renderPage();

    await screen.findByText('Create your first API');
    // The header's own action is suppressed here, so the prompt's button is
    // the only one on the page.
    expect(screen.getAllByRole('button', { name: 'Create API' })).toHaveLength(1);
  });

  it('hides the pagination bar when every API fits on one page', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    renderPage();

    await screen.findByText('Orders API');
    // Two APIs against a page size of 12 — a pager here would be dead weight.
    expect(screen.queryByRole('button', { name: /next page/i })).not.toBeInTheDocument();
  });

  it('widens the paging window and returns to the first page on rows-per-page', async () => {
    server.use(collection('/rest-apis', manyApis, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('API 1');
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('12'));

    await user.click(screen.getByRole('combobox', { name: /APIs per page/i }));
    await user.click(screen.getByRole('option', { name: '24' }));

    await waitFor(() => {
      expect(requests.last()?.params.get('limit')).toBe('24');
      // The old offset of 12 would land mid-collection under the wider window.
      expect(requests.last()?.params.get('offset')).toBe('0');
    });
    expect(await screen.findByText('API 13')).toBeInTheDocument();
  });

  it('falls back a page when a delete empties the last one', async () => {
    // Mutated by the delete handler below, so the refetch sees a shorter
    // collection — the only way the clamp has anything to clamp to.
    const apis = manyRestApis(13);
    server.use(
      collection('/rest-apis', apis, { record: requests }),
      http.delete(apiUrl('/rest-apis/:restApiId'), ({ params }) => {
        const index = apis.findIndex((api) => api.id === params.restApiId);
        if (index >= 0) apis.splice(index, 1);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = renderPage();

    // 13 APIs over a page size of 12 leaves API 13 alone on the second page.
    await screen.findByText('API 1');
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('12'));
    await screen.findByText('API 13');

    await user.click(screen.getByRole('button', { name: 'Delete API 13' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByRole('textbox'), 'API 13');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    // Page 2 no longer exists; the next request must ask for one that does.
    await waitFor(() => expect(requests.last()?.params.get('offset')).toBe('0'));
    expect(await screen.findByText('API 1')).toBeInTheDocument();
  });

  it('switches between the card grid and the compact list', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    expect(screen.queryByTestId('api-list-view')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'List view' }));

    expect(await screen.findByTestId('api-list-view')).toBeInTheDocument();
    expect(screen.getByText('Orders API')).toBeInTheDocument();
  });
});
