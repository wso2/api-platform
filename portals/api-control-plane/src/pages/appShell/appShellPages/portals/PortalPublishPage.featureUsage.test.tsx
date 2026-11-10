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

import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { http as mswHttp, HttpResponse } from 'msw';
import { Route, Routes } from 'react-router-dom';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import { mockDataGridLayout } from '@/test/dataGridLayout';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { makeConsoleScope } from '@/test/mockScope';
import {
  accepts,
  aGraphQLApiDetail,
  apiUrl,
  aPublication,
  aPublicationDraftDetails,
  aRestApi,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type PublicationDraftDetailsFixture,
  type PublicationFixture,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { GraphqlPortalPublishPage } from '../graphqlApis/publish/GraphqlPortalPublishPage';
import { PortalPublishPage } from './PortalPublishPage';

/*
 * What the publish page reports to feature usage, driven through its real
 * controls. PortalPublishPage.test.tsx covers what each action sends.
 */

vi.mock('@/featureUsage/moesif');

// Monaco does not run in jsdom; a textarea stands in for it.
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
const API = 'loan-mgmt';
const PORTAL = 'acme-portal';
const API_NAME = 'Loan Management Service';

const api = aRestApi({
  displayName: API_NAME,
  id: API,
  projectId: PROJECT,
  context: '/loans',
  version: '1.0.0',
});

const BASE = `/api-portals/${PORTAL}/apis/rest-api/${API}`;
const DRAFT_PATH = `${BASE}/draft`;
const DRAFT_DEFINITION_PATH = `${DRAFT_PATH}/definition`;
const DRAFT_THUMBNAIL_PATH = `${DRAFT_PATH}/thumbnail`;
const DRAFT_LANDING_PATH = `${DRAFT_PATH}/landing-page`;
const PUBLICATION_PATH = `${BASE}/publication`;
const PUBLISH_PATH = `${BASE}/publish`;
const UNPUBLISH_PATH = `${BASE}/unpublish`;
const DEPRECATE_PATH = `${BASE}/deprecate`;

const YAML_DEFINITION =
  'openapi: 3.0.3\ninfo:\n  title: Loan Management Service\n  version: 1.0.0\npaths: {}\n';

const scope = makeConsoleScope({
  component: api,
  params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
});

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<PortalPublishPage />} path={routes.apiPortalPublish()} />
        <Route element={<div>portals listing</div>} path={routes.apiPortals()} />
      </Routes>
    </ApiScopeProvider>,
    { route: routes.apiPortalPublish(ORG, PROJECT, API, PORTAL), scope },
  );
}

/** The reads the page makes before it shows the form; the draft holds a YAML definition. */
function servePublicationState({
  draft = aPublicationDraftDetails(),
  publication,
}: { draft?: PublicationDraftDetailsFixture; publication?: PublicationFixture } = {}) {
  server.use(
    mswHttp.get(
      apiUrl(`/apis/rest-api/${API}/thumbnail`),
      () => new HttpResponse(null, { status: 204 }),
    ),
    collection('/subscription-plans', []),
    resource('/rest-apis/:restApiId', api),
    collection(`/rest-apis/${API}/gateways`, []),
    resource(DRAFT_PATH, draft),
    mswHttp.get(
      apiUrl(DRAFT_DEFINITION_PATH),
      () => new HttpResponse(YAML_DEFINITION, { headers: { 'Content-Type': 'application/yaml' } }),
    ),
    publication
      ? resource(PUBLICATION_PATH, publication)
      : failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'),
    failure('get', `${PUBLICATION_PATH}/definition`, 404, 'PUBLICATION_NOT_FOUND'),
    failure('get', `/rest-apis/${API}/openapi`, 404, 'REST_API_SPEC_NOT_FOUND'),
  );
}

/** Every draft save and the publish call succeed. */
const acceptSaves = () =>
  server.use(
    accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
    accepts('put', DRAFT_DEFINITION_PATH, undefined),
    noContent('put', DRAFT_THUMBNAIL_PATH),
    noContent('put', DRAFT_LANDING_PATH),
  );

const nameField = () => screen.getByRole('textbox', { name: /^Name/ });

/** Types the API name the dialog asks for, then confirms. */
async function confirmInDialog(user: ReturnType<typeof renderPage>['user']) {
  const dialog = await screen.findByRole('dialog');
  await user.type(within(dialog).getByRole('textbox'), API_NAME);
  await user.click(within(dialog).getByRole('button', { name: 'Confirm' }));
}

/** Arms the publish bar's primary button with a menu action, then clicks it. */
async function armAndClick(user: ReturnType<typeof renderPage>['user'], action: string) {
  await user.click(await screen.findByRole('button', { name: 'Draft' }));
  await screen.findByDisplayValue(API_NAME);
  await user.click(screen.getByRole('button', { name: 'More publish actions' }));
  await user.click(await screen.findByRole('menuitem', { name: action }));
  await user.click(await screen.findByRole('button', { name: action }));
}

beforeAll(mockDataGridLayout);

beforeEach(() => {
  resetHttpClient();
  resetTrackedFeatures();
  URL.createObjectURL = vi.fn(() => 'blob:test/thumbnail');
  URL.revokeObjectURL = vi.fn();
});

describe('PortalPublishPage feature usage', () => {
  describe('Publish', () => {
    it('reports one publish, and none of the draft saves it makes on the way', async () => {
      servePublicationState();
      acceptSaves();
      const publishRequests = recorder();
      server.use(accepts('post', PUBLISH_PATH, aPublication(), { record: publishRequests }));
      const { user } = renderPage();

      await screen.findByDisplayValue(API_NAME);
      await user.click(screen.getByRole('button', { name: 'Publish' }));

      await waitFor(() => expect(publishRequests.count()).toBe(1));
      expect(trackedFeatures()).toEqual([['apip.lifecycle.api-publish-click', { api_type: 'rest-api' }]]);
    });

    it('reports nothing when the form is invalid and nothing is sent', async () => {
      servePublicationState();
      const draftRequests = recorder();
      server.use(accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }));
      const { user } = renderPage();

      await screen.findByDisplayValue(API_NAME);
      await user.clear(nameField());
      await user.click(screen.getByRole('button', { name: 'Publish' }));

      await waitFor(() => expect(nameField()).toHaveAttribute('aria-invalid', 'true'));
      expect(draftRequests.count()).toBe(0);
      expect(trackedFeatures()).toEqual([]);
    });

    it('reports a GraphQL API as graphql-api', async () => {
      const GRAPHQL_API = 'countries-graphql-api';
      const GRAPHQL_BASE = `/api-portals/${PORTAL}/apis/graphql-api/${GRAPHQL_API}`;
      const publishRequests = recorder();
      server.use(
        mswHttp.get(
          apiUrl(`/apis/graphql-api/${GRAPHQL_API}/thumbnail`),
          () => new HttpResponse(null, { status: 204 }),
        ),
        resource(
          '/graphql-apis/:graphqlApiId',
          aGraphQLApiDetail({
            displayName: 'Countries GraphQL API',
            id: GRAPHQL_API,
            version: '1.0',
          }),
        ),
        resource('/graphql-apis/:graphqlApiId/sdl', { sdl: 'type Query { countries: [String] }' }),
        collection(`/graphql-apis/${GRAPHQL_API}/gateways`, []),
        collection('/subscription-plans', []),
        failure('get', `${GRAPHQL_BASE}/draft`, 404, 'DRAFT_NOT_FOUND'),
        failure('get', `${GRAPHQL_BASE}/draft/definition`, 404, 'DRAFT_NOT_FOUND'),
        failure('get', `${GRAPHQL_BASE}/publication`, 404, 'PUBLICATION_NOT_FOUND'),
        failure('get', `${GRAPHQL_BASE}/publication/definition`, 404, 'PUBLICATION_NOT_FOUND'),
        accepts('put', `${GRAPHQL_BASE}/draft`, aPublicationDraftDetails()),
        accepts('put', `${GRAPHQL_BASE}/draft/definition`, undefined),
        accepts('post', `${GRAPHQL_BASE}/publish`, aPublication(), { record: publishRequests }),
      );
      const { user } = renderWithProviders(
        <ApiScopeProvider orgId={ORG}>
          <Routes>
            <Route element={<GraphqlPortalPublishPage />} path={routes.graphqlApiPortalPublish()} />
            <Route element={<div>portals listing</div>} path={routes.graphqlApiPublish()} />
          </Routes>
        </ApiScopeProvider>,
        {
          route: routes.graphqlApiPortalPublish(ORG, PROJECT, GRAPHQL_API, PORTAL),
          scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
        },
      );

      await screen.findByDisplayValue('Countries GraphQL API');
      await user.click(screen.getByRole('button', { name: 'Publish' }));

      await waitFor(() => expect(publishRequests.count()).toBe(1));
      expect(trackedFeatures()).toEqual([
        ['apip.lifecycle.api-publish-click', { api_type: 'graphql-api' }],
      ]);
    });
  });

  describe('Save Draft', () => {
    it('reports the details and the definition, the parts every save sends', async () => {
      servePublicationState();
      acceptSaves();
      const { user } = renderPage();

      await screen.findByDisplayValue(API_NAME);
      await user.type(nameField(), ' ');
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      expect(await screen.findByText('Draft saved.')).toBeInTheDocument();
      expect(trackedFeatures()).toEqual([
        [
          'apip.lifecycle.publication-draft-save-click',
          {
            api_type: 'rest-api',
            definition_saved: 'yes',
            landing_page_changed: 'no',
            thumbnail_changed: 'no',
          },
        ],
      ]);
    });

    it('leaves out a definition that does not parse, since it is not sent', async () => {
      servePublicationState();
      acceptSaves();
      const { user } = renderPage();

      await screen.findByDisplayValue(API_NAME);
      await user.click(screen.getByRole('tab', { name: 'Specification' }));
      await user.type(
        await screen.findByRole('textbox', { name: 'API definition (YAML)' }),
        '\n  bad: [[',
      );
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      expect(
        await screen.findByText('Details saved. The specification has an error and was not saved.'),
      ).toBeInTheDocument();
      expect(trackedFeatures()).toEqual([
        [
          'apip.lifecycle.publication-draft-save-click',
          {
            api_type: 'rest-api',
            definition_saved: 'no',
            landing_page_changed: 'no',
            thumbnail_changed: 'no',
          },
        ],
      ]);
    });

    it('reports a changed thumbnail and landing page as their own parts', async () => {
      servePublicationState({ draft: aPublicationDraftDetails({ hasLandingPage: true }) });
      server.use(
        mswHttp.get(
          apiUrl(DRAFT_LANDING_PATH),
          () => new HttpResponse('# Draft page', { headers: { 'Content-Type': 'text/markdown' } }),
        ),
      );
      acceptSaves();
      const { container, user } = renderPage();

      await screen.findByDisplayValue(API_NAME);
      await user.upload(
        container.querySelector<HTMLInputElement>('input[type="file"]')!,
        new File(['png'], 'logo.png', { type: 'image/png' }),
      );
      await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
      await user.type(
        await screen.findByRole('textbox', { name: 'Landing page (Markdown)' }),
        ' edited',
      );
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      expect(await screen.findByText('Draft saved.')).toBeInTheDocument();
      expect(trackedFeatures()).toEqual([
        [
          'apip.lifecycle.publication-draft-save-click',
          {
            api_type: 'rest-api',
            definition_saved: 'yes',
            landing_page_changed: 'yes',
            thumbnail_changed: 'yes',
          },
        ],
      ]);
    });
  });

  describe('publication management', () => {
    it('reports an unpublish once it is confirmed', async () => {
      servePublicationState({ publication: aPublication() });
      const requests = recorder();
      server.use(noContent('post', UNPUBLISH_PATH, { record: requests }));
      const { user } = renderPage();

      await armAndClick(user, 'Unpublish');
      await confirmInDialog(user);

      await waitFor(() => expect(requests.count()).toBe(1));
      expect(trackedFeatures()).toEqual([
        ['apip.lifecycle.api-unpublish-click', { api_type: 'rest-api' }],
      ]);
    });

    it('reports a deprecation once it is confirmed', async () => {
      servePublicationState({ publication: aPublication() });
      const requests = recorder();
      server.use(
        accepts('post', DEPRECATE_PATH, aPublication({ status: 'DEPRECATED' }), {
          record: requests,
        }),
      );
      const { user } = renderPage();

      await armAndClick(user, 'Deprecate');
      await confirmInDialog(user);

      await waitFor(() => expect(requests.count()).toBe(1));
      expect(trackedFeatures()).toEqual([
        ['apip.lifecycle.api-deprecate-click', { api_type: 'rest-api' }],
      ]);
    });

    it('reports nothing when the confirmation is cancelled', async () => {
      servePublicationState({ publication: aPublication() });
      const { user } = renderPage();

      await armAndClick(user, 'Unpublish');
      await user.click(
        within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }),
      );

      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(trackedFeatures()).toEqual([]);
    });
  });
});
