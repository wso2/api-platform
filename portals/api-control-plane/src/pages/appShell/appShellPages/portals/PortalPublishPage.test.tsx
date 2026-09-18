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
  accepts,
  aPublication,
  aPublicationDraftDetails,
  aRestApi,
  failure,
  noContent,
  recorder,
  resource,
  type PublicationDraftDetailsFixture,
  type PublicationFixture,
  type Recorder,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { PortalPublishPage } from './PortalPublishPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'loan-mgmt';
const PORTAL = 'acme-portal';

const api = aRestApi({
  description: 'Manage loan applications and repayments.',
  displayName: 'Loan Management Service',
  id: API,
  projectId: PROJECT,
  upstream: { main: { url: 'https://backend.internal/loans' } },
  version: '1.0.0',
});

let requests: Recorder;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<PortalPublishPage />} path={routes.apiPortalPublish()} />
        {/* Stands in for the Portals listing, so "Back" is observable. */}
        <Route element={<div>portals listing</div>} path={routes.apiPortals()} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/portals/${PORTAL}`,
      scope: makeConsoleScope({
        component: api,
        params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

const DRAFT_PATH = `/api-portals/${PORTAL}/apis/rest-api/${API}/draft`;
const DRAFT_DEFINITION_PATH = `${DRAFT_PATH}/definition`;
const PUBLICATION_PATH = `/api-portals/${PORTAL}/apis/rest-api/${API}/publication`;
const PUBLICATION_DEFINITION_PATH = `${PUBLICATION_PATH}/definition`;
const PUBLISH_PATH = `/api-portals/${PORTAL}/apis/rest-api/${API}/publish`;
const UNPUBLISH_PATH = `/api-portals/${PORTAL}/apis/rest-api/${API}/unpublish`;

/** The four reads the page makes before it can render the form. */
function servePublicationState({
  draft,
  publication,
}: {
  draft?: PublicationDraftDetailsFixture;
  publication?: PublicationFixture;
} = {}) {
  server.use(
    resource('/rest-apis/:restApiId', api),
    draft
      ? resource(DRAFT_PATH, draft)
      : failure('get', DRAFT_PATH, 404, 'DRAFT_NOT_FOUND'),
    failure('get', DRAFT_DEFINITION_PATH, 404, 'DRAFT_NOT_FOUND'),
    publication
      ? resource(PUBLICATION_PATH, publication)
      : failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'),
    failure('get', PUBLICATION_DEFINITION_PATH, 404, 'PUBLICATION_NOT_FOUND'),
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('PortalPublishPage', () => {
  it('pre-fills from an existing draft when one exists', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });

    renderPage();

    expect(await screen.findByDisplayValue('Loan Management Service')).toBeInTheDocument();
    expect(screen.getByDisplayValue('1.0.0')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://api.example.com/loans')).toBeInTheDocument();
  });

  it("falls back to the API's own basic info when nothing is saved yet", async () => {
    servePublicationState();

    renderPage();

    expect(await screen.findByDisplayValue('Loan Management Service')).toBeInTheDocument();
    expect(screen.getByDisplayValue('1.0.0')).toBeInTheDocument();
    // Falls back to the API's own upstream URL, not a draft/publication one.
    expect(screen.getByDisplayValue('https://backend.internal/loans')).toBeInTheDocument();
  });

  it('disables Subscription Plans, Documentations and Landing Page for this alpha', async () => {
    servePublicationState();

    renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.getByRole('tab', { name: 'Subscription Plans' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'Documentations' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'Landing Page' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'API Details' })).toBeEnabled();
    expect(screen.getByRole('tab', { name: 'Specification' })).toBeEnabled();
  });

  it('Save Draft writes the details before the definition', async () => {
    servePublicationState();
    const draftRequests = recorder();
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({
      displayName: 'Loan Management Service',
      version: '1.0.0',
    });
  });

  it('Publish saves the draft, then calls the publish action', async () => {
    servePublicationState();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      accepts('post', PUBLISH_PATH, aPublication(), { record: requests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(await screen.findByText('Published to acme-portal.')).toBeInTheDocument();
  });

  it('Unpublish is disabled until the API is actually live, then asks for confirmation', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(noContent('post', UNPUBLISH_PATH, { record: requests }));

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    const unpublishItem = await screen.findByRole('menuitem', { name: 'Unpublish' });
    expect(unpublishItem).not.toHaveAttribute('aria-disabled', 'true');
    await user.click(unpublishItem);

    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Unpublish' }));

    await waitFor(() => expect(requests.count()).toBe(1));
  });
});
