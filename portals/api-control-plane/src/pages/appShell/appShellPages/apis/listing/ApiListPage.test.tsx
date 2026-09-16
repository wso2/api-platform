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
  aGraphQLApi,
  aRestApi,
  apiUrl,
  collection,
  manyRestApis,
  recorder,
  type GraphQLApiFixture,
  type RestApiFixture,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { routes } from '@/routes/paths';
import { makeConsoleScope } from '@/test/mockScope';
import { ApiList } from './ApiList';
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
const matchesHandle = (api: RestApiFixture | GraphQLApiFixture, term: string) =>
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
        {/* Stands in for the API overview, so opening an API is observable. */}
        <Route element={<div>api overview</div>} path={routes.api()} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/apis`,
      scope: makeConsoleScope(),
    },
  );
}

/** Same harness as `renderPage`, but for `ApiList` directly with a `typeFilter` — `ApiListPage` itself never passes one; only `ProjectHomePage`'s stat-card click does. */
function renderList(typeFilter: 'rest' | 'graphql' | 'grpc' | 'async') {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <Routes>
        <Route
          element={<ApiList typeFilter={typeFilter} />}
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

/**
 * Every test needs a `/graphql-apis` handler too — `ApiList` fetches both
 * resource types unconditionally to merge them into one list (that merge is
 * the whole point: a GraphQL API is otherwise invisible here after creation).
 * Most tests are about REST-only behaviour, so this defaults to an empty
 * GraphQL collection; tests covering the merge itself override it.
 */
const serveNoGraphQLApis = () => server.use(collection('/graphql-apis', []));

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  serveNoGraphQLApis();
});

describe('ApiListPage', () => {
  it('fetches the full REST collection in one request, not the on-screen page', async () => {
    server.use(collection('/rest-apis', apiFixtures, { record: requests }));
    renderPage();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Inventory API')).toBeInTheDocument();
    // Pagination is client-side over the merged REST+GraphQL list — the
    // request always asks for the server's max page size, not the UI's
    // current rows-per-page.
    expect(requests.last()?.params.get('limit')).toBe('100');
    expect(requests.last()?.params.get('offset')).toBe('0');
  });

  it('counts every match from pagination.total, not the cards on screen', async () => {
    server.use(collection('/rest-apis', manyApis));
    renderPage();

    // 14 APIs exist; only 12 fit the first page.
    expect(await screen.findByText('14')).toBeInTheDocument();
  });

  it('includes a GraphQL API in the same list as REST APIs', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    server.use(collection('/graphql-apis', [aGraphQLApi({ displayName: 'Countries API' })]));
    renderPage();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Countries API')).toBeInTheDocument();
    // 2 REST + 1 GraphQL.
    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('matches the GraphQL type filter even when the server omits `kind` on the list item', async () => {
    // Regression test: GraphQLAPIListItem.kind was never populated by the
    // real backend (server-side bug, since fixed) — matching on it here
    // meant selecting the "GraphQL" filter always showed "No matching APIs"
    // even though the API existed. `kind: undefined` reproduces that
    // response shape; the filter must not depend on `kind` for this branch.
    server.use(collection('/rest-apis', apiFixtures));
    server.use(
      collection('/graphql-apis', [
        aGraphQLApi({ displayName: 'Countries API', kind: undefined }),
      ]),
    );

    renderList('graphql');

    expect(await screen.findByText('Countries API')).toBeInTheDocument();
    expect(screen.queryByText('Orders API')).not.toBeInTheDocument();
    expect(screen.queryByText('No matching APIs')).not.toBeInTheDocument();
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

  it('pages locally over the already-fetched collection, issuing no new request', async () => {
    server.use(collection('/rest-apis', manyApis, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('API 1');
    expect(screen.queryByText('API 13')).not.toBeInTheDocument();
    const requestCountBeforePaging = requests.count();

    await user.click(screen.getByRole('button', { name: /next page/i }));

    expect(await screen.findByText('API 13')).toBeInTheDocument();
    // The whole collection was already in hand — paging is a local slice.
    expect(requests.count()).toBe(requestCountBeforePaging);
  });

  it('returns to the first local page when the search changes', async () => {
    server.use(
      collection('/rest-apis', manyApis, {
        matches: matchesHandle,
        record: requests,
      }),
    );
    const { user } = renderPage();

    await screen.findByText('API 1');
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await screen.findByText('API 13');

    await user.type(screen.getByPlaceholderText('Search APIs'), 'api');

    await waitFor(() => expect(requests.last()?.params.get('query')).toBe('api'));
    expect(await screen.findByText('API 1')).toBeInTheDocument();
  });

  it('keeps the create prompt for an empty project but not for a missed search', async () => {
    server.use(collection('/rest-apis', apiFixtures, { matches: matchesHandle }));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    await user.type(screen.getByPlaceholderText('Search APIs'), 'nothing');

    expect(await screen.findByText('No matching APIs')).toBeInTheDocument();
    expect(screen.queryByText('Create your first API')).not.toBeInTheDocument();
  });

  it('shows the empty state when the project has no APIs of either kind', async () => {
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

  it('widens the local page and fits every API without a page turn', async () => {
    server.use(collection('/rest-apis', manyApis, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('API 1');
    expect(screen.queryByText('API 13')).not.toBeInTheDocument();
    const requestCountBeforeResize = requests.count();

    await user.click(screen.getByRole('combobox', { name: /APIs per page/i }));
    await user.click(screen.getByRole('option', { name: '24' }));

    // All 14 now fit in one local page — no server round trip needed for it.
    expect(await screen.findByText('API 13')).toBeInTheDocument();
    expect(requests.count()).toBe(requestCountBeforeResize);
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
    await screen.findByText('API 13');

    await user.click(screen.getByRole('button', { name: 'Delete API 13' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByRole('textbox'), 'API 13');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    // Page 2 no longer exists once the delete's refetch lands — the clamp
    // effect must fall back to the last page that still does.
    expect(await screen.findByText('API 1')).toBeInTheDocument();
    expect(screen.queryByText('API 13')).not.toBeInTheDocument();
  });

  it('opens an API from the keyboard in both views', async () => {
    // Each card/row carries a delete button, which was the only thing in it a
    // keyboard could reach — the card itself was not focusable at all.
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    screen.getByRole('button', { name: 'Open Orders API' }).focus();
    await user.keyboard('{Enter}');

    expect(await screen.findByText('api overview')).toBeInTheDocument();
  });

  it('opens an API from a table row with Space', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    await user.click(screen.getByRole('button', { name: 'List view' }));

    const rows = await screen.findByTestId('api-list-view');
    within(rows).getByRole('button', { name: 'Open Orders API' }).focus();
    await user.keyboard(' ');

    expect(await screen.findByText('api overview')).toBeInTheDocument();
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
