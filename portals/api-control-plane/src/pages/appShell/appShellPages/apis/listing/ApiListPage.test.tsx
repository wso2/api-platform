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
  failure,
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

/** Enough APIs to force a second page at either default size (10 in the list, 12 in the grid). */
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

  server.use(
    http.get(
      apiUrl('/apis/:apiType/:apiId/thumbnail'),
      () => new HttpResponse(null, { status: 204 }),
    ),
  );
  serveNoGraphQLApis();
});

describe('ApiListPage', () => {
  it('fetches only the first window of the collection, not all of it', async () => {
    server.use(collection('/rest-apis', apiFixtures, { record: requests }));
    renderPage();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Inventory API')).toBeInTheDocument();
    // Paging is server-side: page one needs only the newest rows-per-page items
    // (10 in the list view) from each source.
    expect(requests.last()?.params.get('limit')).toBe('10');
    expect(requests.last()?.params.get('offset')).toBe('0');
  });

  it('counts every match from pagination.total, not the rows on screen', async () => {
    server.use(collection('/rest-apis', manyApis));
    renderPage();

    // 14 APIs exist; only 10 fit the first list page.
    expect(await screen.findByText(/1–10 of 14/)).toBeInTheDocument();
  });

  it('includes a GraphQL API in the same list as REST APIs', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    server.use(collection('/graphql-apis', [aGraphQLApi({ displayName: 'Countries API' })]));
    renderPage();

    expect(await screen.findByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Countries API')).toBeInTheDocument();
    // 2 REST + 1 GraphQL.
    expect(
      within(screen.getByTestId('api-list-view')).getAllByRole('button', { name: /^Open / }),
    ).toHaveLength(3);
  });

  it('matches the GraphQL type filter even when the server omits `kind` on the list item', async () => {
    // Regression test: GraphQLAPIListItem.kind was never populated by the
    // real backend (server-side bug, since fixed) — matching on it here
    // meant selecting the "GraphQL" filter always showed "No matching APIs"
    // even though the API existed. `kind: undefined` reproduces that
    // response shape; the filter must not depend on `kind` for this branch.
    server.use(collection('/rest-apis', apiFixtures));
    server.use(
      collection('/graphql-apis', [aGraphQLApi({ displayName: 'Countries API', kind: undefined })]),
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

  it('fetches each page from the server with its own limit and offset', async () => {
    server.use(collection('/rest-apis', manyApis, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('API 1');
    expect(screen.queryByText('API 13')).not.toBeInTheDocument();
    // The list view's first page: the newest 10 only, not the whole collection.
    expect(requests.last()?.params.get('limit')).toBe('10');
    expect(requests.last()?.params.get('offset')).toBe('0');
    const requestCountBeforePaging = requests.count();

    await user.click(screen.getByRole('button', { name: /next page/i }));

    expect(await screen.findByText('API 13')).toBeInTheDocument();
    // Page two needs the newest 20 to slice 11-14 out of the merged list.
    expect(requests.count()).toBeGreaterThan(requestCountBeforePaging);
    expect(requests.last()?.params.get('limit')).toBe('20');
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
    await user.click(screen.getByRole('button', { name: 'Clear search' }));
    expect(screen.getByPlaceholderText('Search APIs')).toHaveValue('');
    expect(await screen.findByText('Orders API')).toBeInTheDocument();
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
    // Two APIs against the list view's page size of 10 — a pager here would be dead weight.
    expect(screen.queryByRole('button', { name: /next page/i })).not.toBeInTheDocument();
  });

  it('widens the page and fits every API without a page turn', async () => {
    server.use(collection('/rest-apis', manyApis, { record: requests }));
    const { user } = renderPage();

    await screen.findByText('API 1');
    expect(screen.queryByText('API 13')).not.toBeInTheDocument();

    await user.click(screen.getByRole('combobox', { name: /APIs per page/i }));
    await user.click(screen.getByRole('option', { name: '20' }));

    // All 14 now fit on one page, fetched with the wider window.
    expect(await screen.findByText('API 13')).toBeInTheDocument();
    expect(requests.last()?.params.get('limit')).toBe('20');
  });

  it('falls back a page when a delete empties the last one', async () => {
    // Mutated by the delete handler below, so the refetch sees a shorter
    // collection — the only way the clamp has anything to clamp to.
    const apis = manyRestApis(11);
    server.use(
      collection('/rest-apis', apis, { record: requests }),
      http.delete(apiUrl('/rest-apis/:restApiId'), ({ params }) => {
        const index = apis.findIndex((api) => api.id === params.restApiId);
        if (index >= 0) apis.splice(index, 1);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = renderPage();

    // 11 APIs over the list view's page size of 10 leaves API 11 alone on the second page.
    await screen.findByText('API 1');
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await screen.findByText('API 11');

    await user.click(screen.getByRole('button', { name: 'Delete API 11' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByRole('textbox'), 'API 11');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    // Page 2 no longer exists once the delete's refetch lands — the clamp
    // effect must fall back to the last page that still does.
    expect(await screen.findByText('API 1')).toBeInTheDocument();
    expect(screen.queryByText('API 11')).not.toBeInTheDocument();
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

  it('opens an API from a grid card with Enter', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    // The grid card is Oxygen's `ListingCard` action area, named by its content.
    screen.getByRole('button', { name: /^Orders API/ }).focus();
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

  it('defaults to the list and switches between list and grid views', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    expect(screen.getByTestId('api-list-view')).toBeInTheDocument();
    expect(screen.getByText('Description')).toBeInTheDocument();
    expect(screen.getByText('Last Updated')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    expect(screen.queryByTestId('api-list-view')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'List view' }));

    expect(await screen.findByTestId('api-list-view')).toBeInTheDocument();
    expect(screen.getByText('Orders API')).toBeInTheDocument();
  });
});

describe('ApiListPage — paging', () => {
  it('pages the list view 10 at a time and the grid view 12 at a time', async () => {
    server.use(collection('/rest-apis', manyApis));
    const { user } = renderPage();

    expect(await screen.findByText(/1–10 of 14/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Grid view' }));

    expect(await screen.findByText(/1–12 of 14/)).toBeInTheDocument();
  });

  it('keeps each view on its own page when switching between them', async () => {
    server.use(collection('/rest-apis', manyApis));
    const { user } = renderPage();

    await screen.findByText(/1–10 of 14/);
    await user.click(screen.getByRole('button', { name: /next page/i }));
    expect(await screen.findByText(/11–14 of 14/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    expect(await screen.findByText(/1–12 of 14/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'List view' }));
    expect(await screen.findByText(/11–14 of 14/)).toBeInTheDocument();
  });

  it('shows the pager only when the view has more than one page', async () => {
    // 11 APIs: a second page in the list (10 per page), but one page in the grid (12).
    server.use(collection('/rest-apis', manyRestApis(11)));
    const { user } = renderPage();

    await screen.findByText(/1–10 of 11/);
    expect(screen.getByRole('button', { name: /next page/i })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Grid view' }));

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /next page/i })).not.toBeInTheDocument(),
    );
  });

  it('offers each view its own page sizes', async () => {
    server.use(collection('/rest-apis', manyApis));
    const { user } = renderPage();

    await screen.findByText(/1–10 of 14/);
    await user.click(screen.getByRole('combobox', { name: /APIs per page/i }));
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      '10',
      '20',
      '50',
    ]);
    await user.keyboard('{Escape}');

    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    await user.click(await screen.findByRole('combobox', { name: /APIs per page/i }));
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      '12',
      '24',
      '48',
    ]);
  });

  it('requests the same newest window from both resource types, and a wider one for the next page', async () => {
    const graphqlRequests = recorder();
    server.use(
      collection('/rest-apis', manyApis, { record: requests }),
      collection('/graphql-apis', [aGraphQLApi({ displayName: 'Countries API' })], {
        record: graphqlRequests,
      }),
    );
    const { user } = renderPage();

    await screen.findByText(/1–10 of 15/);
    expect(requests.last()?.params.get('limit')).toBe('10');
    expect(graphqlRequests.last()?.params.get('limit')).toBe('10');

    await user.click(screen.getByRole('button', { name: /next page/i }));

    await screen.findByText(/11–15 of 15/);
    // Page two is rows 11-20 of the merged list, which lie within the newest 20
    // of each source.
    expect(requests.last()?.params.get('limit')).toBe('20');
    expect(graphqlRequests.last()?.params.get('limit')).toBe('20');
  });

  it('adds the totals of both resource types', async () => {
    server.use(
      collection('/rest-apis', manyRestApis(8)),
      collection(
        '/graphql-apis',
        Array.from({ length: 5 }, (_, index) =>
          aGraphQLApi({ id: `gql-${index}`, displayName: `GQL ${index}` }),
        ),
      ),
    );
    renderPage();

    // 8 REST + 5 GraphQL.
    expect(await screen.findByText(/1–10 of 13/)).toBeInTheDocument();
  });

  it('does not request REST APIs at all when filtered to GraphQL', async () => {
    server.use(
      collection('/rest-apis', apiFixtures, { record: requests }),
      collection('/graphql-apis', [aGraphQLApi({ displayName: 'Countries API' })]),
    );
    renderList('graphql');

    expect(await screen.findByText('Countries API')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('fetches the whole collection when filtering by a kind the server cannot filter on', async () => {
    server.use(collection('/rest-apis', apiFixtures, { record: requests }));
    renderList('grpc');

    expect(await screen.findByText('No matching APIs')).toBeInTheDocument();
    // `kind` is not a query parameter, so every item is needed to filter locally.
    expect(requests.last()?.params.get('limit')).toBe('100');
  });
});

describe('ApiListPage — row and card actions', () => {
  it('puts a labelled Delete button on a list row that does not open the API', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    const rows = await screen.findByTestId('api-list-view');
    const deleteButton = within(rows).getByRole('button', { name: 'Delete Orders API' });
    expect(deleteButton).toHaveTextContent('Delete');

    await user.click(deleteButton);

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.queryByText('api overview')).not.toBeInTheDocument();
  });

  it('puts a Delete button on a grid card that does not open the API', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    const { user } = renderPage();

    await screen.findByText('Orders API');
    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    await user.click(await screen.findByRole('button', { name: 'Delete Orders API' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.queryByText('api overview')).not.toBeInTheDocument();
  });

  it('shows the list view as a header row over the API rows', async () => {
    server.use(collection('/rest-apis', apiFixtures));
    renderPage();

    await screen.findByText('Orders API');
    const rows = await screen.findByTestId('api-list-view');
    for (const heading of ['Name', 'Description', 'Type', 'Last Updated']) {
      expect(within(rows).getByText(heading)).toBeInTheDocument();
    }
  });
});

describe('ApiListPage — API type and version display', () => {
  const versions = [
    aRestApi({ id: 'orders-v1', displayName: 'Orders API', version: '1.0.0', kind: 'RestApi' }),
    aRestApi({ id: 'orders-v2', displayName: 'Orders API', version: '2.0.0', kind: 'RestApi' }),
  ];

  it('shows the API type as a chip under the name on a grid card, without the version', async () => {
    server.use(collection('/rest-apis', versions));
    const { user } = renderPage();

    await screen.findAllByText('Orders API');
    await user.click(screen.getByRole('button', { name: 'Grid view' }));

    expect(await screen.findAllByText('REST API')).toHaveLength(versions.length);
    expect(screen.queryByText('v1.0.0')).not.toBeInTheDocument();
  });

  it('shows the API type, not the version, in the list view', async () => {
    server.use(collection('/rest-apis', versions));
    renderPage();

    const rows = await screen.findByTestId('api-list-view');
    expect(within(rows).getAllByText('REST API')).toHaveLength(versions.length);
    expect(within(rows).queryByText('v1.0.0')).not.toBeInTheDocument();
  });
});

describe('ApiList — when the list fails to load', () => {
  it('keeps the sidebar beside the error', async () => {
    server.use(failure('get', '/rest-apis', 500, 'INTERNAL_SERVER_ERROR'));
    renderWithProviders(
      <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
        <Routes>
          <Route
            element={<ApiList sidebar={<div>type statistics</div>} />}
            path="/organizations/:orgHandle/projects/:projectHandler/apis"
          />
        </Routes>
      </ApiScopeProvider>,
      {
        route: `/organizations/${ORG}/projects/${PROJECT}/apis`,
        scope: makeConsoleScope(),
      },
    );

    expect(await screen.findByText('type statistics')).toBeInTheDocument();
    expect(screen.queryByText('Orders API')).not.toBeInTheDocument();
  });
});
