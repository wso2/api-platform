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

import { beforeEach, describe, expect, it } from 'vitest';
import { Route, Routes } from 'react-router-dom';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import {
  aPublicationSummary,
  aRestApi,
  collection,
  recorder,
  type PublicationSummaryFixture,
  type Recorder,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { ApiPortalPublicationsList } from './ApiPortalPublicationsList';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'loan-mgmt';

const api = aRestApi({ displayName: 'Loan Management Service', id: API, projectId: PROJECT });

let requests: Recorder;

/**
 * The page's hook reads `ApiScopeContext` for the org (the endpoint takes no
 * `projectId`), and the console scope for the API's own handle and display
 * name — hence both providers, same as `DeployPage.test.tsx`.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<ApiPortalPublicationsList />} path={routes.apiPortals()} />
        {/* Stands in for the per-portal publish flow, so "Go To Publish" is observable. */}
        <Route element={<div>publish flow</div>} path={routes.apiPortalPublish()} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/portals`,
      scope: makeConsoleScope({
        component: api,
        params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

const servePublications = (items: PublicationSummaryFixture[]) => {
  server.use(collection('/api-publications', items, { record: requests }));
};

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('ApiPortalPublicationsList', () => {
  it('lists every portal from the rollup, scoped to this API', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' }),
    ]);

    renderPage();

    expect(await screen.findByText('API Portal 1')).toBeInTheDocument();
    expect(requests.last()?.params.get('apiType')).toBe('rest-api');
    expect(requests.last()?.params.get('apiId')).toBe(API);
    expect(requests.last()?.headers.get('X-Org-Id')).toBe(ORG);
  });

  it('shows Published for a portal with no pending draft', async () => {
    servePublications([
      aPublicationSummary({ status: 'PUBLISHED', draftUpdatedAt: null }),
    ]);

    renderPage();

    expect(await screen.findByText('Published')).toBeInTheDocument();
  });

  it('shows Draft instead of Published when a draft is pending, even once live', async () => {
    servePublications([
      aPublicationSummary({ status: 'PUBLISHED', draftUpdatedAt: '2026-02-01T00:00:00Z' }),
    ]);

    renderPage();

    expect(await screen.findByText('Draft')).toBeInTheDocument();
    expect(screen.queryByText('Published')).not.toBeInTheDocument();
  });

  it('shows no status mark for an untouched portal', async () => {
    servePublications([aPublicationSummary({ status: 'NOT_PUBLISHED', draftUpdatedAt: null })]);

    renderPage();

    await screen.findByText('API Portal 1');
    expect(screen.queryByText('Published')).not.toBeInTheDocument();
    expect(screen.queryByText('Draft')).not.toBeInTheDocument();
    expect(screen.queryByText('Deprecated')).not.toBeInTheDocument();
  });

  it('shows the shared empty state when the organization has no portals', async () => {
    servePublications([]);

    renderPage();

    expect(await screen.findByText('No Portals Available')).toBeInTheDocument();
    // No CTA when the managed-portals extension isn't registered (the empty-state
    // link points at that extension's route, so a naked button would 404 in OSS).
    expect(screen.queryByRole('button', { name: 'Add Portal' })).not.toBeInTheDocument();
  });

  it('opens the publish flow for the portal the card names', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' }),
    ]);
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Go To Publish' }));

    expect(await screen.findByText('publish flow')).toBeInTheDocument();
  });

  it('names each portal for assistive tech on its otherwise identical button', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'one', apiPortalName: 'API Portal 1' }),
      aPublicationSummary({ apiPortalId: 'two', apiPortalName: 'API Portal 2' }),
    ]);

    renderPage();

    await screen.findByText('API Portal 1');
    const buttons = screen.getAllByRole('button', { name: 'Go To Publish' });
    expect(buttons.map((button) => button.getAttribute('aria-describedby'))).toHaveLength(2);
    expect(buttons[0]).toHaveAccessibleDescription('API Portal 1');
    expect(buttons[1]).toHaveAccessibleDescription('API Portal 2');
  });

  it('shows Deprecated for a deprecated listing', async () => {
    servePublications([aPublicationSummary({ status: 'DEPRECATED', draftUpdatedAt: null })]);

    renderPage();

    expect(await screen.findByText('Deprecated')).toBeInTheDocument();
  });

  it('titles the page with the API handle until its display name is known', async () => {
    servePublications([aPublicationSummary()]);

    renderWithProviders(
      <ApiScopeProvider orgId={ORG}>
        <ApiPortalPublicationsList />
      </ApiScopeProvider>,
      {
        route: `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/portals`,
        scope: makeConsoleScope({
          params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
        }),
      },
    );

    expect(await screen.findByText(`Publish ${API}`)).toBeInTheDocument();
  });

  it('filters the cards by the search text and explains an empty result', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'one', apiPortalName: 'Partner Portal', apiPortalDescription: 'For partners.' }),
      aPublicationSummary({ apiPortalId: 'two', apiPortalName: 'Internal Portal', apiPortalDescription: 'For staff.' }),
    ]);
    const { user } = renderPage();

    await screen.findByText('Partner Portal');
    await user.type(screen.getByPlaceholderText('Search portals'), 'partner');

    expect(screen.getByText('Partner Portal')).toBeInTheDocument();
    expect(screen.queryByText('Internal Portal')).not.toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText('Search portals'));
    await user.type(screen.getByPlaceholderText('Search portals'), 'zzz');

    expect(await screen.findByText('No matching portals')).toBeInTheDocument();
  });

  it('does not offer a search when the organization has no portals', async () => {
    servePublications([]);

    renderPage();

    await screen.findByText('No Portals Available');
    expect(screen.queryByPlaceholderText('Search portals')).not.toBeInTheDocument();
  });
});
