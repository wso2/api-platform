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
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import {
  accepts,
  apiUrl,
  aGateway,
  aGraphQLApiDetail,
  aPublication,
  aPublicationDraftDetails,
  aSubscriptionPlan,
  collection,
  failure,
  recorder,
  resource,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlPortalPublishPage } from './GraphqlPortalPublishPage';

// Monaco does not run in jsdom; the editor is a textarea holding the same text.
vi.mock('@/components/CodeEditor/CodeEditor', () => ({
  CodeEditor: ({
    ariaLabel,
    onChange,
    readOnly,
    value,
  }: {
    ariaLabel?: string;
    onChange?: (next: string) => void;
    readOnly?: boolean;
    value: string;
  }) => (
    <textarea
      aria-label={ariaLabel}
      onChange={(event) => onChange?.(event.target.value)}
      readOnly={readOnly}
      value={value}
    />
  ),
}));

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';
const PORTAL = 'acme-portal';
const SDL = 'type Query { countries: [String] }';

const api = aGraphQLApiDetail({
  displayName: 'Countries GraphQL API',
  id: API,
  context: '/countries',
  projectId: PROJECT,
  upstream: { main: { url: 'https://countries.example.com/graphql' } },
  version: '1.0',
});

/** The one gateway the API is deployed to; its URL is the Production URL's default. */
const deployedGateway = {
  ...aGateway({ displayName: 'Gateway One', endpoints: ['https://gw-one.test'], id: 'gw-1' }),
  associatedAt: '2026-01-01T00:00:00Z',
  isDeployed: true,
};

const BASE = `/api-portals/${PORTAL}/apis/graphql-api/${API}`;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<GraphqlPortalPublishPage />} path={routes.graphqlApiPortalPublish()} />
        <Route element={<div>portals listing</div>} path={routes.graphqlApiPublish()} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: routes.graphqlApiPortalPublish(ORG, PROJECT, API, PORTAL),
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

/** Nothing saved for this portal yet: the page opens on the API's own details and SDL. */
function serveFreshApi() {
  server.use(
    mswHttp.get(apiUrl(`/apis/graphql-api/${API}/thumbnail`), () => new HttpResponse(null, { status: 204 })),
    resource('/graphql-apis/:graphqlApiId', api),
    resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SDL }),
    collection(`/graphql-apis/${API}/gateways`, [deployedGateway]),
    collection('/subscription-plans', []),
    failure('get', `${BASE}/draft`, 404, 'DRAFT_NOT_FOUND'),
    failure('get', `${BASE}/draft/definition`, 404, 'DRAFT_NOT_FOUND'),
    failure('get', `${BASE}/publication`, 404, 'PUBLICATION_NOT_FOUND'),
    failure('get', `${BASE}/publication/definition`, 404, 'PUBLICATION_NOT_FOUND'),
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('GraphqlPortalPublishPage', () => {
  it('opens on the GraphQL API’s own details, with its SDL as the specification', async () => {
    serveFreshApi();
    const { user } = renderPage();

    expect(await screen.findByDisplayValue('Countries GraphQL API')).toBeInTheDocument();
    // As on REST, the Production URL defaults to where the API is served, never its backend.
    expect(screen.getByDisplayValue('https://gw-one.test/countries')).toBeInTheDocument();
    expect(
      screen.queryByDisplayValue('https://countries.example.com/graphql'),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: 'Specification' }));

    expect(await screen.findByRole('textbox', { name: 'API definition (GraphQL)' })).toHaveValue(
      SDL,
    );
    // An SDL has one serialization, so there is no JSON/YAML switch.
    expect(screen.queryByRole('group', { name: 'Source format' })).not.toBeInTheDocument();
  });

  it('saves the SDL as application/graphql and publishes through the graphql-api actions', async () => {
    serveFreshApi();
    const definitionRequests = recorder();
    const publishRequests = recorder();
    server.use(
      accepts('put', `${BASE}/draft`, aPublicationDraftDetails()),
      accepts('put', `${BASE}/draft/definition`, undefined, { record: definitionRequests }),
      accepts('post', `${BASE}/publish`, aPublication(), { record: publishRequests }),
    );
    const { user } = renderPage();

    await screen.findByDisplayValue('Countries GraphQL API');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(definitionRequests.last()?.headers.get('Content-Type')).toBe('application/graphql');
    expect(definitionRequests.last()?.body).toBe(SDL);
    await waitFor(() => expect(publishRequests.count()).toBe(1));
    expect(await screen.findByText('portals listing')).toBeInTheDocument();
  });

  it('does not publish an SDL that does not parse', async () => {
    serveFreshApi();
    const publishRequests = recorder();
    server.use(
      accepts('put', `${BASE}/draft`, aPublicationDraftDetails()),
      accepts('post', `${BASE}/publish`, aPublication(), { record: publishRequests }),
    );
    const { user } = renderPage();

    await screen.findByDisplayValue('Countries GraphQL API');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));
    const editor = await screen.findByRole('textbox', { name: 'API definition (GraphQL)' });
    await user.clear(editor);
    await user.type(editor, 'type Query {{ broken');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    expect(await screen.findByText(/This is not valid GraphQL/)).toBeInTheDocument();
    expect(publishRequests.count()).toBe(0);
  });

  it('saves the selected subscription plans with the GraphQL draft', async () => {
    serveFreshApi();
    const bronze = aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze' });
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    const draftRequests = recorder();
    server.use(
      collection('/subscription-plans', [bronze, gold]),
      accepts('put', `${BASE}/draft`, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', `${BASE}/draft/definition`, undefined),
    );
    const { user } = renderPage();

    await screen.findByDisplayValue('Countries GraphQL API');
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
    await user.click(await screen.findByRole('checkbox', { name: /Gold/ }));
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({
      subscriptionPlanIds: ['gold'],
    });
  });

  it('saves the selected documents with the GraphQL draft', async () => {
    serveFreshApi();
    const docsPath = `/apis/graphql-api/${API}/docs`;
    const draftRequests = recorder();
    server.use(
      collection(docsPath, [{ contentType: 'text/markdown', displayName: 'Schema guide', id: 'schema-guide', type: 'HowTo' }]),
      mswHttp.get(
        apiUrl(`${docsPath}/:docId/content`),
        () => new HttpResponse('Body', { headers: { 'Content-Type': 'text/markdown' } }),
      ),
      accepts('put', `${BASE}/draft`, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', `${BASE}/draft/definition`, undefined),
    );
    const { user } = renderPage();

    await screen.findByDisplayValue('Countries GraphQL API');
    await user.click(screen.getByRole('tab', { name: 'Documentation' }));
    await user.click(await screen.findByRole('checkbox', { name: 'Schema guide' }));
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ docIds: ['schema-guide'] });
  });
});
