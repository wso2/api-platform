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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { http as mswHttp, HttpResponse } from 'msw';
import { Route, Routes } from 'react-router-dom';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { orgScope } from '@/api/core/queryKeys';
import { apiDocumentQueries } from '@/api/resources/apiDocuments/apiDocuments.queries';
import type { ApiDocumentListResponse } from '@/api/resources/apiDocuments/apiDocuments.endpoints';
import { apiGatewayQueries } from '@/api/resources/restApis/apiGateways/apiGateways.queries';
import type { RestApiGatewayListResponse } from '@/api/resources/restApis/apiGateways/apiGateways.endpoints';
import { routes } from '@/routes/paths';
import {
  accepts,
  apiUrl,
  aGateway,
  aPublication,
  aPublicationDraftDetails,
  aRestApi,
  aSubscriptionPlan,
  collection,
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
import { makeTestQueryClient, renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { PortalPublishPage } from './PortalPublishPage';

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

const api = aRestApi({
  description: 'Manage loan applications and repayments.',
  displayName: 'Loan Management Service',
  id: API,
  projectId: PROJECT,
  context: '/loans',
  upstream: { main: { url: 'https://backend.internal/loans' }, sandbox: { url: 'https://sandbox.backend.internal/loans' } },
  version: '1.0.0',
});

let requests: Recorder;

function renderPage(queryClient?: QueryClient) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<PortalPublishPage />} path={routes.apiPortalPublish()} />
        {/* Stands in for the Portals listing, so "Back" is observable. */}
        <Route element={<div>portals listing</div>} path={routes.apiPortals()} />
      </Routes>
    </ApiScopeProvider>,
    {
      queryClient,
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
const DEPRECATE_PATH = `/api-portals/${PORTAL}/apis/rest-api/${API}/deprecate`;
const DOCS_PATH = `/apis/rest-api/${API}/docs`;
const API_THUMBNAIL_PATH = `/apis/rest-api/${API}/thumbnail`;
const DRAFT_THUMBNAIL_PATH = `${DRAFT_PATH}/thumbnail`;
const PUBLICATION_THUMBNAIL_PATH = `${PUBLICATION_PATH}/thumbnail`;
const API_OPENAPI_PATH = `/rest-apis/${API}/openapi`;
const DRAFT_LANDING_PATH = `${DRAFT_PATH}/landing-page`;
const PUBLICATION_LANDING_PATH = `${PUBLICATION_PATH}/landing-page`;
const GATEWAYS_PATH = `/rest-apis/${API}/gateways`;

/** A gateway the API is deployed to, as the API's gateway list returns it. */
const deployedGateway = (id: string, displayName: string, endpoint: string) => ({
  ...aGateway({ displayName, endpoints: [endpoint], id }),
  associatedAt: '2026-01-01T00:00:00Z',
  isDeployed: true,
});

/** The definition chain's three tiers, each with its own call recorder. */
function definitionTierRecorders() {
  return {
    draftDefinition: recorder(),
    publicationDefinition: recorder(),
    openApi: recorder(),
  };
}

/** The reads the page makes before it can render the form; `plans` is the org's catalog. */
function servePublicationState({
  draft,
  publication,
  definitionRecorders = definitionTierRecorders(),
  gateways = [],
  plans = [],
  apiThumbnail = apiWithoutThumbnail(),
}: {
  draft?: PublicationDraftDetailsFixture;
  publication?: PublicationFixture;
  definitionRecorders?: ReturnType<typeof definitionTierRecorders>;
  gateways?: ReturnType<typeof deployedGateway>[];
  plans?: ReturnType<typeof aSubscriptionPlan>[];
  apiThumbnail?: ReturnType<typeof apiWithoutThumbnail>;
} = {}) {
  server.use(
    apiThumbnail,
    collection('/subscription-plans', plans),
    resource('/rest-apis/:restApiId', api),
    collection(GATEWAYS_PATH, gateways),
    draft ? resource(DRAFT_PATH, draft) : failure('get', DRAFT_PATH, 404, 'DRAFT_NOT_FOUND'),
    failure('get', DRAFT_DEFINITION_PATH, 404, 'DRAFT_NOT_FOUND', {
      record: definitionRecorders.draftDefinition,
    }),
    publication
      ? resource(PUBLICATION_PATH, publication)
      : failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'),
    failure('get', PUBLICATION_DEFINITION_PATH, 404, 'PUBLICATION_NOT_FOUND', {
      record: definitionRecorders.publicationDefinition,
    }),
    // The third definition fallback tier — no spec uploaded for this API.
    failure('get', API_OPENAPI_PATH, 404, 'REST_API_SPEC_NOT_FOUND', {
      record: definitionRecorders.openApi,
    }),
  );
  return definitionRecorders;
}

/** An image as the thumbnail endpoints serve it. */
const pngResponse = () => new HttpResponse(new Uint8Array([0x89, 0x50]), { headers: { 'Content-Type': 'image/png' } });

/** Serves a thumbnail at `path`, and counts the reads. */
const thumbnailAt = (path: string, reads: Recorder) =>
  mswHttp.get(apiUrl(path), async ({ request }) => {
    await reads.capture(request);
    return pngResponse();
  });

/** The API has no thumbnail of its own: the endpoint answers 204. */
const apiWithoutThumbnail = () =>
  mswHttp.get(apiUrl(API_THUMBNAIL_PATH), () => new HttpResponse(null, { status: 204 }));

/** A stored definition served as text in the given serialization, as the backend returns it. */
const definitionText = (path: string, text: string, contentType: string) =>
  mswHttp.get(
    apiUrl(path),
    () => new HttpResponse(text, { headers: { 'Content-Type': contentType } }),
  );

const YAML_DEFINITION =
  'openapi: 3.0.3\ninfo:\n  title: Loan Management Service\n  version: 1.0.0\npaths: {}\n';

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  URL.createObjectURL = vi.fn(() => 'blob:test/thumbnail');
  URL.revokeObjectURL = vi.fn();
});

const API_NAME = 'Loan Management Service';

type ThumbnailSaveAnswer = { code: string; status: number } | 'network' | undefined;

/**
 * Answers each thumbnail save: the nth save gets the nth answer (the last one repeats), a failure
 * to reject with or `undefined` to succeed.
 */
function answerThumbnailSaves(...answers: ThumbnailSaveAnswer[]) {
  let saves = 0;
  server.use(
    mswHttp.put(apiUrl(DRAFT_THUMBNAIL_PATH), () => {
      const answer = answers[Math.min(saves, answers.length - 1)];
      saves += 1;
      if (answer === 'network') return HttpResponse.error();
      if (!answer) return new HttpResponse(null, { status: 204 });
      return HttpResponse.json(
        { code: answer.code, message: 'The request could not be completed.', status: 'error' } as never,
        { status: answer.status },
      );
    }),
  );
  return { saves: () => saves };
}

/** Save Draft is off until something changes; a trailing space does, and is trimmed away on save. */
const makeEdit = (user: ReturnType<typeof renderPage>['user']) =>
  user.type(screen.getByRole('textbox', { name: /^Name/ }), ' ');

/** A live API opens on its published version; the actions and the editable form are on the draft. */
const showDraft = async (user: ReturnType<typeof renderPage>['user']) =>
  user.click(await screen.findByRole('button', { name: 'Draft' }));

/** Types the API name the dialog asks for, then confirms. */
async function confirmInDialog(user: ReturnType<typeof renderPage>['user']) {
  const dialog = await screen.findByRole('dialog');
  await user.type(within(dialog).getByRole('textbox'), API_NAME);
  await user.click(within(dialog).getByRole('button', { name: 'Confirm' }));
}

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
    // No gateway to default to, and the backend URLs are not exposed.
    expect(screen.getByRole('combobox', { name: 'Production URL' })).toHaveValue('');
    expect(screen.getByRole('combobox', { name: 'Sandbox URL' })).toHaveValue('');
    expect(screen.queryByDisplayValue('https://backend.internal/loans')).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue('https://sandbox.backend.internal/loans')).not.toBeInTheDocument();
  });

  it('still opens, with no Production URL default, when the gateways cannot be read', async () => {
    servePublicationState();
    server.use(failure('get', GATEWAYS_PATH, 500, 'INTERNAL_ERROR'));

    renderPage();

    expect(await screen.findByDisplayValue('Loan Management Service')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Production URL' })).toHaveValue('');
    expect(screen.queryByRole('button', { name: 'Show gateway URLs' })).not.toBeInTheDocument();
  });

  it('ignores an older cached gateway list when the gateways cannot be read again', async () => {
    servePublicationState();
    server.use(failure('get', GATEWAYS_PATH, 500, 'INTERNAL_ERROR'));
    const queryClient = makeTestQueryClient();
    queryClient.setQueryData(
      apiGatewayQueries.list(orgScope(ORG)!, API).queryKey,
      { list: [deployedGateway('gw-1', 'Gateway One', 'https://gw-one.test')] } as RestApiGatewayListResponse,
      { updatedAt: 0 },
    );

    renderPage(queryClient);

    expect(await screen.findByDisplayValue('Loan Management Service')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Production URL' })).toHaveValue('');
    expect(screen.queryByRole('button', { name: 'Show gateway URLs' })).not.toBeInTheDocument();
  });

  it('defaults the Production URL to the first gateway the API is deployed to', async () => {
    servePublicationState({
      gateways: [
        deployedGateway('gw-1', 'Gateway One', 'https://gw-one.test/'),
        deployedGateway('gw-2', 'Gateway Two', 'https://gw-two.test'),
      ],
    });
    const { user } = renderPage();

    const production = await screen.findByRole('combobox', { name: 'Production URL' });
    expect(production).toHaveValue('https://gw-one.test/loans');

    await user.click(screen.getByRole('button', { name: 'Show gateway URLs' }));
    expect(await screen.findByText('https://gw-two.test/loans')).toBeInTheDocument();
  });

  it('offers only gateways the API is deployed to', async () => {
    servePublicationState({
      gateways: [
        { ...deployedGateway('gw-1', 'Gateway One', 'https://gw-one.test'), isDeployed: false },
        deployedGateway('gw-2', 'Gateway Two', 'https://gw-two.test'),
      ],
    });
    const { user } = renderPage();

    expect(await screen.findByRole('combobox', { name: 'Production URL' })).toHaveValue('https://gw-two.test/loans');

    await user.click(screen.getByRole('button', { name: 'Show gateway URLs' }));
    expect(await screen.findByText('Gateway Two')).toBeInTheDocument();
    expect(screen.queryByText('Gateway One')).not.toBeInTheDocument();
  });

  it('keeps the saved Production URL over a gateway default', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails(),
      gateways: [deployedGateway('gw-1', 'Gateway One', 'https://gw-one.test')],
    });

    renderPage();

    expect(await screen.findByDisplayValue('https://api.example.com/loans')).toBeInTheDocument();
  });

  it('rejects a Production URL that is not a full URL', async () => {
    servePublicationState();
    const { user } = renderPage();

    const production = await screen.findByRole('combobox', { name: 'Production URL' });
    await user.clear(production);
    await user.type(production, 'gw.example.com');
    await user.tab();

    expect(await screen.findByText('Enter a full URL, for example https://api.example.com.')).toBeInTheDocument();
  });

  it('shows a URL error only after the field is left, not while it is being edited', async () => {
    servePublicationState();
    const { user } = renderPage();

    const production = await screen.findByRole('combobox', { name: 'Production URL' });
    await user.clear(production);
    await user.type(production, 'gw');
    await user.tab();
    expect(await screen.findByText('Enter a full URL, for example https://api.example.com.')).toBeInTheDocument();

    await user.type(production, '.');
    expect(screen.queryByText('Enter a full URL, for example https://api.example.com.')).not.toBeInTheDocument();

    await user.tab();
    expect(await screen.findByText('Enter a full URL, for example https://api.example.com.')).toBeInTheDocument();
  });

  it('enables every tab', async () => {
    servePublicationState();

    renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    for (const name of ['API Details', 'Specification', 'Subscription Plans', 'Documentation', 'Landing Page']) {
      expect(screen.getByRole('tab', { name })).toBeEnabled();
    }
  });

  it('Documentation tab saves the selection as part of the draft, and reads it back from the publication in the published view', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails({ docIds: [] }),
      publication: aPublication({ docIds: ['guide'] }),
    });
    server.use(
      collection(DOCS_PATH, [
        { contentType: 'text/markdown', displayName: 'Getting started', id: 'guide', type: 'HowTo' },
        { contentType: 'text/markdown', displayName: 'Common questions', id: 'faq', type: 'FAQ' },
      ]),
      mswHttp.get(
        apiUrl(`${DOCS_PATH}/:docId/content`),
        () => new HttpResponse('Body', { headers: { 'Content-Type': 'text/markdown' } }),
      ),
    );
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await showDraft(user);
    await user.click(screen.getByRole('tab', { name: 'Documentation' }));
    await user.click(await screen.findByRole('checkbox', { name: 'Common questions' }));
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ docIds: ['faq'] });

    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(await screen.findByText('Getting started')).toBeInTheDocument();
    expect(screen.queryByText('Common questions')).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('Subscription Plans tab saves the selection as part of the draft, and reads it back from the publication in the published view', async () => {
    const bronze = aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze' });
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    servePublicationState({
      draft: aPublicationDraftDetails({ subscriptionPlanIds: [] }),
      publication: aPublication({ subscriptionPlanIds: ['bronze'] }),
    });
    server.use(collection('/subscription-plans', [bronze, gold]));
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await showDraft(user);
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
    await user.click(await screen.findByRole('checkbox', { name: /Gold/ }));
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({
      subscriptionPlanIds: ['gold'],
    });

    await user.click(screen.getByRole('button', { name: 'Published' }));

    const bronzeCard = await screen.findByRole('checkbox', { name: /Bronze/ });
    expect(bronzeCard).toHaveAttribute('aria-checked', 'true');
    expect(screen.queryByRole('button', { name: 'Select all' })).not.toBeInTheDocument();
  });

  it('is back to no unsaved changes once the plan selection matches the saved draft again, whatever the order', async () => {
    const bronze = aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze' });
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    const silver = aSubscriptionPlan({ displayName: 'Silver', id: 'silver' });
    servePublicationState({
      draft: aPublicationDraftDetails({ subscriptionPlanIds: ['bronze', 'gold'] }),
      plans: [bronze, gold, silver],
    });

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
    const silverCard = await screen.findByRole('checkbox', { name: /Silver/ });
    const bronzeCard = screen.getByRole('checkbox', { name: /Bronze/ });
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();

    await user.click(silverCard);
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
    await user.click(silverCard);
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

    // Toggled off and on, a plan lands last in the list; it is still the saved selection.
    await user.click(bronzeCard);
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
    await user.click(bronzeCard);
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
  });

  it('offers an inactive plan only while it is selected, and blocks Save Draft and Publish until an inactive selected plan is cleared', async () => {
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold', status: 'ACTIVE' });
    const retired = aSubscriptionPlan({ displayName: 'Retired', id: 'retired', status: 'INACTIVE' });
    const unused = aSubscriptionPlan({ displayName: 'Unused', id: 'unused', status: 'INACTIVE' });
    servePublicationState({
      draft: aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'retired'] }),
      plans: [gold, retired, unused],
    });
    const draftRequests = recorder();
    const publishRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      accepts('post', PUBLISH_PATH, aPublication(), { record: publishRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));

    // Nothing selected is dropped behind the user's back; an unselected inactive plan isn't offered.
    const retiredCard = await screen.findByRole('checkbox', { name: /Retired/ });
    expect(retiredCard).toHaveAttribute('aria-checked', 'true');
    expect(screen.queryByRole('checkbox', { name: /Unused/ })).not.toBeInTheDocument();
    expect(screen.getByText('1 inactive plan selected')).toBeInTheDocument();

    // Both actions stop at the plans, without a request.
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    expect(await screen.findByText('Clear inactive plans to save or publish: "Retired".')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Publish' }));
    expect(draftRequests.count()).toBe(0);
    expect(publishRequests.count()).toBe(0);

    await user.click(retiredCard);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ subscriptionPlanIds: ['gold'] });
  });

  it('drops a selected plan that no longer exists, says so, and does not send it', async () => {
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    servePublicationState({
      draft: aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'deleted'] }),
      plans: [gold],
    });
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(
      await screen.findByText('A selected subscription plan no longer exists and was removed from the selection.'),
    ).toBeInTheDocument();

    // It was never an edit of the user's, so nothing is left to save until they make one.
    expect(screen.queryByText('Unsaved changes', { exact: false })).not.toBeInTheDocument();
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ subscriptionPlanIds: ['gold'] });
  });

  it('lets a deleted plan be cleared from the draft, which turns on Save Draft and drops it on save', async () => {
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    servePublicationState({
      draft: aPublicationDraftDetails({
        subscriptionPlanIds: ['gold', 'old'],
        subscriptionPlans: [
          { deleted: false, displayName: 'Gold', edited: false, id: 'gold' },
          { deleted: true, displayName: 'Platinum', edited: false, id: 'old' },
        ],
      }),
      plans: [gold],
    });
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

    await user.click(await screen.findByRole('checkbox', { name: /Platinum/ }));
    expect(screen.queryByRole('checkbox', { name: /Platinum/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ subscriptionPlanIds: ['gold'] });
  });

  it('stops Save Draft and Publish at a deleted plan until it is cleared', async () => {
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    servePublicationState({
      draft: aPublicationDraftDetails({
        subscriptionPlanIds: ['gold', 'old'],
        subscriptionPlans: [
          { deleted: false, displayName: 'Gold', edited: false, id: 'gold' },
          { deleted: true, displayName: 'Platinum', edited: false, id: 'old' },
        ],
      }),
      plans: [gold],
    });
    const draftRequests = recorder();
    const publishRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      accepts('post', PUBLISH_PATH, aPublication(), { record: publishRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);

    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    expect(await screen.findByText('Clear deleted plans from this listing to save or publish: "Platinum".')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: /Platinum/ })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Publish' }));
    expect(draftRequests.count()).toBe(0);
    expect(publishRequests.count()).toBe(0);

    await user.click(screen.getByRole('checkbox', { name: /Platinum/ }));
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ subscriptionPlanIds: ['gold'] });
  });

  it('drops a selected document that no longer exists, says so, and does not send it', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ docIds: ['guide', 'deleted'] }) });
    server.use(
      collection(DOCS_PATH, [{ contentType: 'text/markdown', displayName: 'Getting started', id: 'guide', type: 'HowTo' }]),
    );
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(
      await screen.findByText('A selected document no longer exists and was removed from the selection.'),
    ).toBeInTheDocument();

    // It was never an edit of the user's, so nothing is left to save until they make one.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled());
    expect(screen.queryByText('Unsaved changes', { exact: false })).not.toBeInTheDocument();
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ docIds: ['guide'] });
  });

  it('keeps a selected document it cannot check because the document list was cut short', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ docIds: ['doc-0', 'beyond'] }) });
    server.use(
      mswHttp.get(apiUrl(DOCS_PATH), ({ request }) => {
        const offset = Number(new URL(request.url).searchParams.get('offset') ?? 0);
        const list = Array.from({ length: 100 }, (_, i) => ({
          contentType: 'text/markdown',
          displayName: `Doc ${offset + i}`,
          id: `doc-${offset + i}`,
          type: 'HowTo',
        }));
        return HttpResponse.json({ count: list.length, list, pagination: { limit: 100, offset, total: 6000 } });
      }),
    );
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    // Past the 50 pages read, so the list cannot say whether it was deleted.
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(
      screen.queryByText('A selected document no longer exists and was removed from the selection.'),
    ).not.toBeInTheDocument();
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ docIds: ['doc-0', 'beyond'] });
  });

  it('keeps a selected document that an older cached document list predates, once the list is read again', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ docIds: ['guide', 'added'] }) });
    server.use(
      collection(DOCS_PATH, [
        { contentType: 'text/markdown', displayName: 'Getting started', id: 'guide', type: 'HowTo' },
        { contentType: 'text/markdown', displayName: 'Added elsewhere', id: 'added', type: 'HowTo' },
      ]),
    );
    const draftRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );
    const queryClient = makeTestQueryClient();
    queryClient.setQueryData(
      apiDocumentQueries.listAll(orgScope(ORG)!, 'rest-api', API).queryKey,
      {
        count: 1,
        list: [{ contentType: 'text/markdown', displayName: 'Getting started', id: 'guide', type: 'HowTo' }],
        pagination: { limit: 1, offset: 0, total: 1 },
      } as ApiDocumentListResponse,
      { updatedAt: 0 },
    );

    const { user } = renderPage(queryClient);

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Documentation' }));
    expect(await screen.findByText('Added elsewhere')).toBeInTheDocument();

    expect(
      screen.queryByText('A selected document no longer exists and was removed from the selection.'),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: 'API Details' }));
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ docIds: ['guide', 'added'] });
  });

  it("sends the draft's selected documents with every save, so saving other fields keeps them", async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ docIds: ['guide'] }) });
    const draftRequests = recorder();
    server.use(
      collection(DOCS_PATH, [{ contentType: 'text/markdown', displayName: 'Getting started', id: 'guide', type: 'HowTo' }]),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue(API_NAME);
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({ docIds: ['guide'] });
  });

  it("shows the draft's thumbnail, without reading the publication's or the API's own", async () => {
    const apiReads = recorder();
    const publicationReads = recorder();
    const draftReads = recorder();
    servePublicationState({
      draft: aPublicationDraftDetails({ hasThumbnail: true }),
      publication: aPublication({ hasThumbnail: true }),
      apiThumbnail: thumbnailAt(API_THUMBNAIL_PATH, apiReads),
    });
    server.use(
      thumbnailAt(DRAFT_THUMBNAIL_PATH, draftReads),
      thumbnailAt(PUBLICATION_THUMBNAIL_PATH, publicationReads),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await showDraft(user);
    expect(await screen.findByRole('img', { name: 'Loan Management Service' })).toHaveAttribute('src', 'blob:test/thumbnail');
    expect(draftReads.count()).toBe(1);
    expect(apiReads.count()).toBe(0);
  });

  it("falls back to the publication's thumbnail, then to the API's own", async () => {
    const apiReads = recorder();
    servePublicationState({
      publication: aPublication({ hasThumbnail: true }),
      apiThumbnail: thumbnailAt(API_THUMBNAIL_PATH, apiReads),
    });
    server.use(thumbnailAt(PUBLICATION_THUMBNAIL_PATH, recorder()));

    const first = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await showDraft(first.user);
    expect(await screen.findByRole('img', { name: 'Loan Management Service' })).toBeInTheDocument();
    expect(apiReads.count()).toBe(0);
    first.unmount();

    resetHttpClient();
    servePublicationState({ apiThumbnail: thumbnailAt(API_THUMBNAIL_PATH, apiReads) });
    renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    expect(await screen.findByRole('img', { name: 'Loan Management Service' })).toBeInTheDocument();
    expect(apiReads.count()).toBe(1);
  });

  it('shows the published thumbnail read-only in the published view', async () => {
    servePublicationState({
      publication: aPublication({ hasThumbnail: true }),
    });
    server.use(thumbnailAt(PUBLICATION_THUMBNAIL_PATH, recorder()));

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(await screen.findByRole('img', { name: 'Loan Management Service' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Change thumbnail' })).not.toBeInTheDocument();

    await showDraft(user);
    expect(screen.getByRole('button', { name: 'Change thumbnail' })).toBeInTheDocument();
  });

  it('counts a picked thumbnail as an unsaved change, and refuses one that is too large', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    const { container, user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

    const tooLarge = new File([new Uint8Array(2 * 1024 * 1024 + 1)], 'big.png', { type: 'image/png' });
    await user.upload(input, tooLarge);
    expect(await screen.findByText('The image must be 2 MB or smaller.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

    await user.upload(input, new File(['png'], 'logo.png', { type: 'image/png' }));
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeEnabled();
    expect(screen.getByRole('img', { name: 'Loan Management Service' })).toHaveAttribute('src', 'blob:test/thumbnail');
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
  });

  it('drops a thumbnail the server refuses, so the next save does not send it again', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );
    const thumbnail = answerThumbnailSaves({ code: 'PAYLOAD_TOO_LARGE', status: 413 });

    const { container, user } = renderPage();

    await screen.findByDisplayValue(API_NAME);
    await user.upload(
      container.querySelector<HTMLInputElement>('input[type="file"]')!,
      new File(['png'], 'logo.png', { type: 'image/png' }),
    );
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(thumbnail.saves()).toBe(1));
    await waitFor(() => expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument());

    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText('Draft saved.')).toBeInTheDocument();
    expect(thumbnail.saves()).toBe(1);
  });

  it.each([
    ['a server error', { code: 'INTERNAL_ERROR', status: 500 }],
    ['a network failure', 'network'],
    ['a draft that is gone', { code: 'DRAFT_NOT_FOUND', status: 404 }],
  ] as [string, ThumbnailSaveAnswer][])('keeps a picked thumbnail when its save fails with %s, so the next save sends it again', async (_name, failure) => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );
    const thumbnail = answerThumbnailSaves(failure, undefined);

    const { container, user } = renderPage();

    await screen.findByDisplayValue(API_NAME);
    await user.upload(
      container.querySelector<HTMLInputElement>('input[type="file"]')!,
      new File(['png'], 'logo.png', { type: 'image/png' }),
    );
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(thumbnail.saves()).toBe(1));
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: API_NAME })).toHaveAttribute('src', 'blob:test/thumbnail');
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeEnabled();

    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText('Draft saved.')).toBeInTheDocument();
    expect(thumbnail.saves()).toBe(2);
  });

  it('lists the saved inactive plan first, then the saved selection, then the other plans, and keeps that order when a plan is selected', async () => {
    const bronze = aSubscriptionPlan({ displayName: 'Bronze', id: 'bronze' });
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' });
    const retired = aSubscriptionPlan({ displayName: 'Retired', id: 'retired', status: 'INACTIVE' });
    servePublicationState({
      draft: aPublicationDraftDetails({ subscriptionPlanIds: ['gold', 'retired'] }),
      plans: [bronze, gold, retired],
    });

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
    await screen.findByRole('checkbox', { name: /Retired/ });

    const names = ['Retired', 'Gold', 'Bronze'];
    const order = () =>
      screen.getAllByRole('checkbox').map((card) => names.find((name) => card.textContent?.startsWith(name)));
    expect(order()).toEqual(names);

    // Selecting a plan does not move it: the order follows what is saved, not the clicks.
    await user.click(screen.getByRole('checkbox', { name: /Bronze/ }));
    expect(order()).toEqual(names);
  });

  it('names every inactive plan in the Save Draft warning, in one fixed sentence', async () => {
    const retired = aSubscriptionPlan({ displayName: 'Retired', id: 'retired', status: 'INACTIVE' });
    const legacy = aSubscriptionPlan({ displayName: 'Legacy', id: 'legacy', status: 'INACTIVE' });
    servePublicationState({
      draft: aPublicationDraftDetails({ subscriptionPlanIds: ['retired', 'legacy'] }),
      plans: [retired, legacy],
    });
    const draftRequests = recorder();
    server.use(accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }));

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(
      await screen.findByText('Clear inactive plans to save or publish: "Retired", "Legacy".'),
    ).toBeInTheDocument();
    expect(draftRequests.count()).toBe(0);
  });

  it('shows exactly the plans the live listing holds in the published view, an inactive one badged', async () => {
    const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold', status: 'ACTIVE' });
    const silver = aSubscriptionPlan({ displayName: 'Silver', id: 'silver', status: 'ACTIVE' });
    const retired = aSubscriptionPlan({ displayName: 'Retired', id: 'retired', status: 'INACTIVE' });
    servePublicationState({
      publication: aPublication({ subscriptionPlanIds: ['gold', 'retired'] }),
      plans: [gold, silver, retired],
    });

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
    await user.click(screen.getByRole('button', { name: 'Published' }));

    const retiredCard = await screen.findByRole('checkbox', { name: /Retired/ });
    expect(retiredCard).toHaveAttribute('aria-checked', 'true');
    expect(retiredCard).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('checkbox', { name: /Gold/ })).toHaveAttribute('aria-checked', 'true');
    expect(screen.queryByRole('checkbox', { name: /Silver/ })).not.toBeInTheDocument();
    expect(screen.getByText('Inactive')).toBeInTheDocument();
  });

  it('explains a missing draft once, without retrying, when the definition save 404s', async () => {
    servePublicationState();
    const draftRequests = recorder();
    server.use(
      failure('put', DRAFT_DEFINITION_PATH, 404, 'DRAFT_NOT_FOUND', { message: 'raw server text' }),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText(/Unable to save the draft/)).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    expect(draftRequests.count()).toBe(1);
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
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({
      displayName: 'Loan Management Service',
      version: '1.0.0',
      agentVisibility: 'VISIBLE',
    });
  });

  it('does not query publication/definition or the API’s own spec once a draft definition is found', async () => {
    const definitionRecorders = servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(
      resource(
        DRAFT_DEFINITION_PATH,
        { openapi: '3.0.3', paths: {} },
        {
          record: definitionRecorders.draftDefinition,
        },
      ),
    );

    renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await waitFor(() => expect(definitionRecorders.draftDefinition.count()).toBe(1));
    expect(definitionRecorders.publicationDefinition.count()).toBe(0);
    expect(definitionRecorders.openApi.count()).toBe(0);
  });

  it.each([
    ['draft', DRAFT_DEFINITION_PATH],
    ['published', PUBLICATION_DEFINITION_PATH],
  ])('opens with a %s definition that was saved as YAML', async (_tier, path) => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(path, YAML_DEFINITION, 'application/yaml'));
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText('portals listing')).not.toBeInTheDocument();
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? '{}')).toMatchObject({
      openapi: '3.0.3',
      info: { title: 'Loan Management Service', version: '1.0.0' },
    });
  });

  it('shows a definition saved as YAML in YAML, exactly as stored', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(DRAFT_DEFINITION_PATH, YAML_DEFINITION, 'application/yaml'));

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));

    expect(await screen.findByRole('textbox', { name: 'API definition (YAML)' })).toHaveValue(
      YAML_DEFINITION,
    );
    expect(screen.getByRole('button', { name: 'YAML' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('shows a definition saved as JSON in JSON, pretty-printed', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(
      definitionText(DRAFT_DEFINITION_PATH, '{"openapi":"3.0.3","paths":{}}', 'application/json'),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));

    expect(await screen.findByRole('textbox', { name: 'API definition (JSON)' })).toHaveValue(
      '{\n  "openapi": "3.0.3",\n  "paths": {}\n}',
    );
  });

  it('saves the definition as the format shown once it has been switched and edited', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(DRAFT_DEFINITION_PATH, YAML_DEFINITION, 'application/yaml'));
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));
    await user.click(await screen.findByRole('button', { name: 'JSON' }));
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? '{}')).toMatchObject({
      openapi: '3.0.3',
      info: { title: 'Loan Management Service' },
    });
  });

  it('still saves the details when the definition cannot be read, and reports the partial save', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(DRAFT_DEFINITION_PATH, YAML_DEFINITION, 'application/yaml'));
    const draftRequests = recorder();
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));
    await user.type(
      await screen.findByRole('textbox', { name: 'API definition (YAML)' }),
      '\n  bad: [[',
    );
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText(/This is not valid YAML:/)).toBeInTheDocument();
    // Details and definition are separate endpoints, so an unparseable
    // definition doesn't cost the user their unrelated Details edits.
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(definitionRequests.count()).toBe(0);
    expect(
      await screen.findByText('Details saved. The specification has an error and was not saved.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Draft saved.')).not.toBeInTheDocument();
  });

  it('Save Draft succeeds with an empty definition — only Publish checks validity', async () => {
    // No draft/publication/own-spec definition is served, so definitionText
    // stays "" and definitionFormat stays its default, 'json'.
    servePublicationState({ draft: aPublicationDraftDetails() });
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? 'null')).toEqual({});
  });

  it('Save Draft succeeds with a syntactically valid but incomplete OpenAPI object', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(DRAFT_DEFINITION_PATH, '{"foo":1}', 'application/json'));
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? 'null')).toEqual({ foo: 1 });
  });

  it('still opens with a definition that was saved as JSON', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(
      definitionText(
        DRAFT_DEFINITION_PATH,
        JSON.stringify({ openapi: '3.0.3', paths: {} }),
        'application/json',
      ),
    );
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? '{}')).toMatchObject({ openapi: '3.0.3' });
  });

  it('falls all the way through to the API’s own real stored spec when no draft/publication definition exists', async () => {
    servePublicationState();
    server.use(
      resource(API_OPENAPI_PATH, {
        content:
          'openapi: 3.0.3\ninfo:\n  title: Loan Management Service\n  version: 1.0.0\npaths: {}\n',
      }),
    );
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? '{}')).toMatchObject({
      openapi: '3.0.3',
      info: { title: 'Loan Management Service', version: '1.0.0' },
    });
  });

  it('Publish saves the draft, then calls the publish action', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(DRAFT_DEFINITION_PATH, YAML_DEFINITION, 'application/yaml'));
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
      accepts('post', PUBLISH_PATH, aPublication(), { record: requests }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    // Publish only ever calls the publish action once the definition it
    // carries has actually been saved — not the empty-draft placeholder.
    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(definitionRequests.last()?.body ?? '{}')).toMatchObject({
      openapi: '3.0.3',
      info: { title: 'Loan Management Service', version: '1.0.0' },
    });
    await waitFor(() => expect(requests.count()).toBe(1));
    expect(await screen.findByText('Published to acme-portal.')).toBeInTheDocument();
    // Nothing left to do here once the action succeeds — back to the listing.
    expect(await screen.findByText('portals listing')).toBeInTheDocument();
  });

  it('Publish saves the details but does not call publish when the definition cannot be read', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    server.use(definitionText(DRAFT_DEFINITION_PATH, YAML_DEFINITION, 'application/yaml'));
    const draftRequests = recorder();
    const definitionRequests = recorder();
    const publishRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails(), { record: draftRequests }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
      accepts('post', PUBLISH_PATH, aPublication(), { record: publishRequests }),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));
    await user.type(
      await screen.findByRole('textbox', { name: 'API definition (YAML)' }),
      '\n  bad: [[',
    );
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    expect(await screen.findByText(/This is not valid YAML:/)).toBeInTheDocument();
    // Details still saves — it's a separate, unaffected endpoint — but
    // publishing a stale definition under a broken edit is never allowed.
    await waitFor(() => expect(draftRequests.count()).toBe(1));
    expect(definitionRequests.count()).toBe(0);
    expect(publishRequests.count()).toBe(0);
    expect(screen.queryByText('Published to acme-portal.')).not.toBeInTheDocument();
  });

  it('Unpublish is disabled until the API is actually live, then asks for confirmation', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(noContent('post', UNPUBLISH_PATH, { record: requests }));

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    const unpublishItem = await screen.findByRole('menuitem', { name: 'Unpublish' });
    expect(unpublishItem).not.toHaveAttribute('aria-disabled', 'true');
    await user.click(unpublishItem);

    // Choosing "Unpublish" from the menu arms the primary button with that
    // action — it doesn't unpublish on its own, the armed button still has to
    // be clicked (and confirmed) to actually do it.
    expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));

    await confirmInDialog(user);

    await waitFor(() => expect(requests.count()).toBe(1));
  });

  it('Deprecate asks for confirmation, then marks the live listing deprecated', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(
      accepts('post', DEPRECATE_PATH, aPublication({ status: 'DEPRECATED' }), { record: requests }),
    );

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    // Published, so the primary button reads Republish, not Publish.
    expect(screen.getByRole('button', { name: 'Republish' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    const deprecateItem = await screen.findByRole('menuitem', { name: 'Deprecate' });
    expect(deprecateItem).not.toHaveAttribute('aria-disabled', 'true');
    await user.click(deprecateItem);

    // Like Unpublish, picking it from the menu only arms the primary button.
    expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: 'Deprecate' }));

    await confirmInDialog(user);

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(await screen.findByText('Deprecated on acme-portal.')).toBeInTheDocument();
  });

  it('Deprecate is disabled when the API is not published', async () => {
    servePublicationState();

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    // Never published, so the primary button still reads Publish.
    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    expect(await screen.findByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
  });

  it('Deprecate is disabled once already deprecated, while Unpublish stays available', async () => {
    servePublicationState({ publication: aPublication({ status: 'DEPRECATED' }) });

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    // Deprecated, not published, so the primary button reads Publish, not Republish.
    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    expect(await screen.findByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    expect(screen.getByRole('menuitem', { name: 'Unpublish' })).not.toHaveAttribute(
      'aria-disabled',
      'true',
    );
  });

  it('returns to the portals list once the API has been unpublished', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(noContent('post', UNPUBLISH_PATH));

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));
    await confirmInDialog(user);

    // Nothing left to do here once the action succeeds — back to the listing.
    expect(await screen.findByText('portals listing')).toBeInTheDocument();
    expect(screen.getByText('Unpublished from acme-portal.')).toBeInTheDocument();
  });

  it('does not report success when the API was already unpublished elsewhere, and refreshes to Publish', async () => {
    servePublicationState({ publication: aPublication() });
    const message = 'This API is already unpublished from this API Portal. No changes were made.';
    server.use(failure('post', UNPUBLISH_PATH, 409, 'PUBLICATION_STATE_CONFLICT', { message }));

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));

    server.use(failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'));
    await confirmInDialog(user);

    expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument();
    expect(screen.queryByText('Unpublished from acme-portal.')).not.toBeInTheDocument();
  });

  it('returns to the portals list once the API has been deprecated', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(accepts('post', DEPRECATE_PATH, aPublication({ status: 'DEPRECATED' })));

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Deprecate' }));
    await user.click(await screen.findByRole('button', { name: 'Deprecate' }));
    await confirmInDialog(user);

    // Nothing left to do here once the action succeeds — back to the listing.
    expect(await screen.findByText('portals listing')).toBeInTheDocument();
    expect(screen.getByText('Deprecated on acme-portal.')).toBeInTheDocument();
  });

  it('lists Deprecate before Unpublish in the dropdown', async () => {
    servePublicationState({ publication: aPublication() });

    const { user } = renderPage();
    await showDraft(user);

    await screen.findByDisplayValue(API_NAME);
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));

    const items = await screen.findAllByRole('menuitem');
    expect(items.map((item) => item.textContent)).toEqual(['Deprecate', 'Unpublish']);
  });

  it.each(['Unpublish', 'Deprecate'] as const)(
    '%s only confirms once the API name has been typed',
    async (action) => {
      servePublicationState({ publication: aPublication() });
      const path = action === 'Unpublish' ? UNPUBLISH_PATH : DEPRECATE_PATH;
      server.use(
        action === 'Unpublish'
          ? noContent('post', path, { record: requests })
          : accepts('post', path, aPublication({ status: 'DEPRECATED' }), { record: requests }),
      );

      const { user } = renderPage();
      await showDraft(user);

      await screen.findByDisplayValue(API_NAME);
      await user.click(screen.getByRole('button', { name: 'More publish actions' }));
      await user.click(await screen.findByRole('menuitem', { name: action }));
      await user.click(await screen.findByRole('button', { name: action }));

      const dialog = await screen.findByRole('dialog');
      const confirm = within(dialog).getByRole('button', { name: 'Confirm' });
      expect(within(dialog).getByText(`Type "${API_NAME}" to confirm`)).toBeInTheDocument();
      expect(
        within(dialog).getByText(
          action === 'Unpublish'
            ? `This removes the API "${API_NAME}" from acme-portal. You can publish it again later.`
            : `The API "${API_NAME}" will be marked as deprecated on acme-portal. It will remain listed.`,
        ),
      ).toBeInTheDocument();
      expect(confirm).toBeDisabled();

      await user.type(within(dialog).getByRole('textbox'), 'Wrong name');
      expect(confirm).toBeDisabled();

      await user.clear(within(dialog).getByRole('textbox'));
      await user.type(within(dialog).getByRole('textbox'), API_NAME);
      expect(confirm).toBeEnabled();
      expect(requests.count()).toBe(0);
    },
  );

  it('keeps the armed Unpublish button when the confirmation is cancelled', async () => {
    servePublicationState({ publication: aPublication() });

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }),
    );

    expect(await screen.findByRole('button', { name: 'Unpublish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
  });

  it('refreshes the actions when unpublish finds the listing already removed (409 PUBLICATION_STATE_CONFLICT)', async () => {
    servePublicationState({ publication: aPublication() });

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));

    // Another session already unpublished it: the action conflicts and a re-read finds nothing live.
    server.use(
      failure('post', UNPUBLISH_PATH, 409, 'PUBLICATION_STATE_CONFLICT'),
      failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'),
    );
    await confirmInDialog(user);

    expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Unpublish' })).not.toBeInTheDocument();
  });

  it('refreshes the actions when deprecate finds the listing no longer published (409 PUBLICATION_STATE_CONFLICT)', async () => {
    servePublicationState({ publication: aPublication() });

    const { user } = renderPage();

    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Deprecate' }));
    await user.click(await screen.findByRole('button', { name: 'Deprecate' }));

    // Another session already deprecated it, so a re-read reports DEPRECATED.
    server.use(
      failure('post', DEPRECATE_PATH, 409, 'PUBLICATION_STATE_CONFLICT'),
      resource(PUBLICATION_PATH, aPublication({ status: 'DEPRECATED' })),
    );
    await confirmInDialog(user);

    expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    expect(await screen.findByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
  });

  it.each([
    ['409 PUBLICATION_PORTAL_CONFLICT', 409, 'PUBLICATION_PORTAL_CONFLICT'],
    ['503 PUBLICATION_PORTAL_UNAVAILABLE', 503, 'PUBLICATION_PORTAL_UNAVAILABLE'],
  ])(
    'keeps the actions unchanged on %s, since the listing itself did not change',
    async (_label, status, code) => {
      servePublicationState({ publication: aPublication() });

      const { user } = renderPage();
      await showDraft(user);

      await screen.findByDisplayValue('Loan Management Service');
      await user.click(screen.getByRole('button', { name: 'More publish actions' }));
      await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
      await user.click(await screen.findByRole('button', { name: 'Unpublish' }));

      server.use(failure('post', UNPUBLISH_PATH, status, code, { record: requests }));
      await confirmInDialog(user);

      await waitFor(() => expect(requests.count()).toBe(1));
      // The re-read still finds it live, so Unpublish stays armed and usable.
      expect(await screen.findByRole('button', { name: 'Unpublish' })).toBeEnabled();
      expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
    },
  );

  it('stays usable after Save Draft fails with 413', async () => {
    servePublicationState();
    server.use(failure('put', DRAFT_PATH, 413, 'REQUEST_TOO_LARGE'));

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Save Draft' })).toBeEnabled());
    expect(screen.queryByText('Draft saved.')).not.toBeInTheDocument();
  });

  it('stays usable after Publish fails with a portal conflict', async () => {
    servePublicationState();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      failure('post', PUBLISH_PATH, 409, 'PUBLICATION_PORTAL_CONFLICT'),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled());
    expect(screen.queryByText('Published to acme-portal.')).not.toBeInTheDocument();
  });

  it('stays usable when Publish rejects the definition as invalid, and sends the user to the Specification tab', async () => {
    // No client-side check exists any more: the draft (with whatever
    // definition it holds) saves unconditionally, and only the publish call
    // itself can reject it. The message itself is surfaced by the ordinary
    // global error snackbar, not any bespoke handling here — this only
    // checks that the user lands where they'd actually fix the problem.
    servePublicationState();
    const definitionRequests = recorder();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: definitionRequests }),
      failure('post', PUBLISH_PATH, 400, 'PUBLICATION_VALIDATION_FAILED'),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.getByRole('tab', { name: 'API Details' })).toHaveAttribute('aria-selected', 'true');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: 'Specification' })).toHaveAttribute('aria-selected', 'true'),
    );
    expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled();
    expect(screen.queryByText('Published to acme-portal.')).not.toBeInTheDocument();
  });

  it('sends the user to the Specification tab from any tab when the server names the definition', async () => {
    servePublicationState();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      failure('post', PUBLISH_PATH, 400, 'PUBLICATION_VALIDATION_FAILED', {
        errors: [{ field: 'definition', message: 'A definition is required to publish this API' }],
      }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() =>
      expect(screen.getByRole('tab', { name: 'Specification' })).toHaveAttribute('aria-selected', 'true'),
    );
  });

  it('stays on the current tab when the server names a field that is not the definition', async () => {
    servePublicationState();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      failure('post', PUBLISH_PATH, 400, 'PUBLICATION_VALIDATION_FAILED', {
        errors: [{ field: 'thumbnail', message: 'thumbnail must be a PNG or JPEG image' }],
      }),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled());
    expect(screen.getByRole('tab', { name: 'Landing Page' })).toHaveAttribute('aria-selected', 'true');
  });

  it('stays on the Documentation tab when Publish fails validation', async () => {
    servePublicationState();
    server.use(
      collection(DOCS_PATH, []),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      failure('post', PUBLISH_PATH, 400, 'PUBLICATION_VALIDATION_FAILED'),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Documentation' }));
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled());
    expect(screen.getByRole('tab', { name: 'Documentation' })).toHaveAttribute('aria-selected', 'true');
  });

  it('does not switch tabs when Publish fails for a reason unrelated to the definition', async () => {
    servePublicationState();
    server.use(
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      failure('post', PUBLISH_PATH, 409, 'PUBLICATION_PORTAL_CONFLICT'),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled());
    expect(screen.getByRole('tab', { name: 'API Details' })).toHaveAttribute('aria-selected', 'true');
  });

  it('says the user lacks permission when the draft cannot be read (403)', async () => {
    servePublicationState();
    server.use(failure('get', DRAFT_PATH, 403, 'FORBIDDEN'));

    renderPage();

    expect(await screen.findByText('You don’t have permission')).toBeInTheDocument();
    expect(screen.queryByText(/Unable to load the publish details/)).not.toBeInTheDocument();
  });

  it('keeps the generic message when loading fails for a reason other than permission', async () => {
    servePublicationState();
    server.use(failure('get', DRAFT_PATH, 500, 'INTERNAL_ERROR'));

    renderPage();

    expect(await screen.findByText(/Unable to load the publish details/)).toBeInTheDocument();
    expect(screen.queryByText('You don’t have permission')).not.toBeInTheDocument();
  });
});

describe('PortalPublishPage — viewing the published version', () => {
  const published = aPublication({ displayName: 'Published Name', version: '2.0.0' });
  const draft = aPublicationDraftDetails({ displayName: 'Draft Name', version: '3.0.0' });

  it('cannot move to Published while nothing is live', async () => {
    servePublicationState({ draft });

    renderPage();

    await screen.findByDisplayValue('Draft Name');
    expect(screen.getByRole('button', { name: 'Published' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Draft' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('opens on the published version when the API is live, with the draft one click away', async () => {
    servePublicationState({ draft, publication: published });

    const { user } = renderPage();

    expect(await screen.findByDisplayValue('Published Name')).toHaveAttribute('readonly');
    expect(screen.getByRole('button', { name: 'Published' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Draft' })).toBeEnabled();

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(screen.getByDisplayValue('Draft Name')).not.toHaveAttribute('readonly');
  });

  it('can switch to the draft of a live API that has none saved yet', async () => {
    servePublicationState({ publication: published });

    const { user } = renderPage();

    await screen.findByDisplayValue('Published Name');
    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(screen.getByDisplayValue('Published Name')).not.toHaveAttribute('readonly');
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeInTheDocument();
  });

  it('shows the live details read-only from what is already loaded, and brings the draft back untouched', async () => {
    const publicationRequests = recorder();
    servePublicationState({ draft, publication: published });
    server.use(resource(PUBLICATION_PATH, published, { record: publicationRequests }));

    const { user } = renderPage();

    expect(await screen.findByDisplayValue('Published Name')).toHaveAttribute('readonly');
    expect(screen.queryByRole('button', { name: 'Save Draft' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Draft' }));
    await user.type(screen.getByDisplayValue('Draft Name'), ' edited');
    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(screen.getByRole('button', { name: 'Published' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByDisplayValue('Published Name')).toHaveAttribute('readonly');

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(screen.getByDisplayValue('Draft Name edited')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeInTheDocument();
    // Neither flip asked the server for anything the page already held.
    expect(publicationRequests.count()).toBe(1);
  });

  it('reads the published definition once, on first view, and reuses it on later flips', async () => {
    const definitionRecorders = servePublicationState({ draft, publication: published });
    server.use(
      resource(DRAFT_DEFINITION_PATH, { openapi: '3.0.3', info: { title: 'Draft Name' }, paths: {} }),
      resource(
        PUBLICATION_DEFINITION_PATH,
        { openapi: '3.0.3', info: { title: 'Published Name' }, paths: {} },
        { record: definitionRecorders.publicationDefinition },
      ),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Published Name');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));

    const editor = await screen.findByLabelText(/API definition/);
    await waitFor(() => expect((editor as HTMLTextAreaElement).value).toContain('Published Name'));
    expect(editor).toHaveAttribute('readonly');
    expect(definitionRecorders.publicationDefinition.count()).toBe(1);

    await user.click(screen.getByRole('button', { name: 'Draft' }));
    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(((await screen.findByLabelText(/API definition/)) as HTMLTextAreaElement).value).toContain('Published Name');
    expect(definitionRecorders.publicationDefinition.count()).toBe(1);
  });
});

describe('PortalPublishPage — version banner', () => {
  const hoursAgo = (hours: number) => new Date(Date.now() - hours * 60 * 60 * 1000).toISOString();

  it('dates the draft banner from the saved draft', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ version: '3.0.0', updatedAt: hoursAgo(2) }) });

    renderPage();

    expect(await screen.findByText('Edited 2 hours ago')).toBeInTheDocument();
  });

  it('shows no banner for a draft that was never saved', async () => {
    servePublicationState();

    renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();
    expect(screen.queryByText(/not saved yet/)).not.toBeInTheDocument();
  });

  it('shows the published banner on the published side only, and no draft banner when none is saved', async () => {
    servePublicationState({ publication: aPublication({ version: '2.0.0' }) });

    const { user } = renderPage();

    expect(await screen.findByText(/^Updated/)).toBeInTheDocument();
    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(screen.queryByText(/^Updated/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();
  });

  it('dates the published banner from the live publication, and the draft one from the draft', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails({ version: '3.0.0', updatedAt: hoursAgo(2) }),
      publication: aPublication({ version: '2.0.0', updatedAt: hoursAgo(48) }),
    });

    const { user } = renderPage();

    expect(await screen.findByText('Updated 2 days ago')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(screen.getByText('Edited 2 hours ago')).toBeInTheDocument();
    expect(screen.queryByText(/^Updated/)).not.toBeInTheDocument();
  });

  it('calls a deprecated listing deprecated, not updated', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails(),
      publication: aPublication({ status: 'DEPRECATED', version: '2.0.0' }),
    });

    renderPage();

    expect(await screen.findByText(/^Deprecated/)).toBeInTheDocument();
    expect(screen.queryByText(/^Updated/)).not.toBeInTheDocument();
  });
});

describe('PortalPublishPage — published specification', () => {
  const draft = aPublicationDraftDetails({ displayName: 'Draft Name' });
  const published = aPublication({ displayName: 'Published Name' });
  const publishedSpec = { openapi: '3.0.3', info: { title: 'Published Name' }, paths: {} };

  async function openPublishedSpecification() {
    const view = renderPage();
    await screen.findByDisplayValue('Published Name');
    await view.user.click(screen.getByRole('tab', { name: 'Specification' }));
    return view;
  }

  it('re-prints the published definition as YAML for reading, still read-only', async () => {
    servePublicationState({ draft, publication: published });
    server.use(
      resource(DRAFT_DEFINITION_PATH, { openapi: '3.0.3', info: { title: 'Draft Name' }, paths: {} }),
      resource(PUBLICATION_DEFINITION_PATH, publishedSpec),
    );

    const { user } = await openPublishedSpecification();
    await waitFor(() =>
      expect((screen.getByLabelText(/API definition/) as HTMLTextAreaElement).value).toContain('Published Name'),
    );
    await user.click(screen.getByRole('button', { name: 'YAML' }));

    const yaml = (await screen.findByRole('textbox', { name: 'API definition (YAML)' })) as HTMLTextAreaElement;
    expect(yaml.value).toContain('title: Published Name');
    expect(yaml).toHaveAttribute('readonly');
  });

  it('reports a published definition that cannot be read beside the toggle, keeping the page usable', async () => {
    servePublicationState({ draft, publication: published });
    server.use(
      resource(DRAFT_DEFINITION_PATH, { openapi: '3.0.3', info: { title: 'Draft Name' }, paths: {} }),
      failure('get', PUBLICATION_DEFINITION_PATH, 500, 'INTERNAL_ERROR'),
    );

    const { user } = await openPublishedSpecification();

    expect(await screen.findByText('Unable to load the published specification.')).toBeInTheDocument();
    expect(screen.queryByText('Unable to load the publish details.')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    const editor = (await screen.findByRole('textbox', { name: 'API definition (JSON)' })) as HTMLTextAreaElement;
    expect(editor.value).toContain('Draft Name');
  });
});

describe('PortalPublishPage — version toggle while an action runs', () => {
  it('stays disabled until the save finishes', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails(),
      publication: aPublication(),
    });
    server.use(
      mswHttp.put(apiUrl(DRAFT_PATH), async () => {
        await new Promise((resolve) => setTimeout(resolve, 300));
        return HttpResponse.json(aPublicationDraftDetails());
      }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();
    await showDraft(user);
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.getByRole('button', { name: 'Published' })).toBeEnabled();

    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Published' })).toBeDisabled());
    await waitFor(() => expect(screen.getByRole('button', { name: 'Published' })).toBeEnabled());
  });
});

describe('PortalPublishPage — after a publish consumed the draft', () => {
  it('drops the draft it remembers: no draft banner, and the form opens on what is live', async () => {
    const queryClient = makeTestQueryClient();
    const draft = aPublicationDraftDetails({ displayName: 'Old Draft Name', updatedAt: new Date().toISOString() });
    servePublicationState({ draft });
    const first = renderPage(queryClient);
    await screen.findByDisplayValue('Old Draft Name');
    expect(screen.getByText(/^Edited/)).toBeInTheDocument();
    first.unmount();

    // The publish promoted the draft: the server no longer has one, and the
    // cache still holds the old copy until it is revalidated on the next visit.
    servePublicationState({ publication: aPublication({ displayName: 'Live Name' }) });
    await queryClient.invalidateQueries();
    renderPage(queryClient);

    expect(await screen.findByDisplayValue('Live Name')).toBeInTheDocument();
    expect(screen.queryByDisplayValue('Old Draft Name')).not.toBeInTheDocument();
    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();
  });
});

describe('PortalPublishPage — while a publish runs', () => {
  const notFound = (code: string) =>
    HttpResponse.json({ status: 'error', code, message: 'Not found.' } as never, { status: 404 });

  /** A portal that has never been published: the draft appears once saved, the publication once published. */
  function serveFirstPublish() {
    servePublicationState();
    let draftSaved = false;
    let published = false;
    const reads = { draft: recorder(), publication: recorder() };
    server.use(
      mswHttp.get(apiUrl(DRAFT_PATH), async ({ request }) => {
        await reads.draft.capture(request);
        return draftSaved
          ? HttpResponse.json(aPublicationDraftDetails({ updatedAt: new Date().toISOString() }))
          : notFound('DRAFT_NOT_FOUND');
      }),
      mswHttp.get(apiUrl(PUBLICATION_PATH), async ({ request }) => {
        await reads.publication.capture(request);
        return published ? HttpResponse.json(aPublication()) : notFound('PUBLICATION_NOT_FOUND');
      }),
      mswHttp.put(apiUrl(DRAFT_PATH), () => {
        draftSaved = true;
        return HttpResponse.json(aPublicationDraftDetails({ updatedAt: new Date().toISOString() }));
      }),
      accepts('put', DRAFT_DEFINITION_PATH, undefined, { record: recorder() }),
      mswHttp.post(apiUrl(PUBLISH_PATH), async () => {
        await new Promise((resolve) => setTimeout(resolve, 400));
        published = true;
        return HttpResponse.json(aPublication());
      }),
    );
    return reads;
  }

  it('holds the banner and the actions steady, instead of redrawing as each save lands', async () => {
    const reads = serveFirstPublish();

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Publish' }));
    // Both saves have gone through; the publish call is still in flight.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish' })).toBeDisabled());
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Republish' })).not.toBeInTheDocument();
    expect(await screen.findByText('portals listing')).toBeInTheDocument();
    expect(reads.publication.count()).toBe(1);
  });

  it('reads nothing of its own again on the way out', async () => {
    const reads = serveFirstPublish();

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    expect(await screen.findByText('portals listing')).toBeInTheDocument();
    // One read each when the page opened, none for the three writes after it.
    expect(reads.draft.count()).toBe(1);
    expect(reads.publication.count()).toBe(1);
  });
});

describe('PortalPublishPage — unsaved changes', () => {
  it('keeps Save Draft off until something is edited, and says so in the banner', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ updatedAt: new Date().toISOString() }) });
    server.use(accepts('put', DRAFT_PATH, aPublicationDraftDetails()), accepts('put', DRAFT_DEFINITION_PATH, undefined));

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();

    await makeEdit(user);

    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeEnabled();
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
    expect(screen.getByText(/^Edited .* ·/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled());
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();
  });

  it('reports unsaved changes alone when no draft has been saved yet, and again once an edit is undone', async () => {
    servePublicationState();

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();

    await makeEdit(user);
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();

    await user.type(screen.getByRole('textbox', { name: /^Name/ }), '{Backspace}');
    expect(screen.queryByText(/Unsaved changes/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
  });
});

describe('PortalPublishPage — saving a draft', () => {
  it('shows the saved draft from the server’s reply, without reading it again', async () => {
    servePublicationState();
    const draftReads = recorder();
    server.use(
      mswHttp.get(apiUrl(DRAFT_PATH), async ({ request }) => {
        await draftReads.capture(request);
        return HttpResponse.json(
          { status: 'error', code: 'DRAFT_NOT_FOUND', message: 'Not found.' } as never,
          { status: 404 },
        );
      }),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails({ updatedAt: new Date().toISOString() })),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
    );

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText(/^Edited/)).not.toBeInTheDocument();

    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText(/^Edited/)).toBeInTheDocument();
    expect(draftReads.count()).toBe(1);
  });
});

describe('PortalPublishPage — landing page', () => {
  const EDITOR = 'Landing page (Markdown)';

  /** A landing page served as Markdown text at `path`, with its reads counted. */
  const landingPageAt = (path: string, text: string, reads: Recorder = recorder()) =>
    mswHttp.get(apiUrl(path), async ({ request }) => {
      await reads.capture(request);
      return new HttpResponse(text, { headers: { 'Content-Type': 'text/markdown' } });
    });

  const openLandingPage = async (user: ReturnType<typeof renderPage>['user']) => {
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
  };

  it('offers to create one from the API details when none is stored, and reads nothing for it', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    const reads = recorder();
    server.use(
      landingPageAt(DRAFT_LANDING_PATH, '# draft', reads),
      landingPageAt(PUBLICATION_LANDING_PATH, '# published', reads),
      definitionText(
        DRAFT_DEFINITION_PATH,
        JSON.stringify({ openapi: '3.0.3', paths: { '/loans': { get: { summary: 'List loans' } } } }),
        'application/json',
      ),
    );
    const { user } = renderPage();

    await openLandingPage(user);
    expect(screen.getByText('No landing page')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: EDITOR })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

    await user.click(screen.getByRole('button', { name: 'Create from API details' }));

    const editor = screen.getByRole('textbox', { name: EDITOR }) as HTMLTextAreaElement;
    expect(editor.value).toBe(
      [
        '## Overview',
        '',
        'Manage loan applications and repayments.',
        '',
        '## Endpoints',
        '',
        '- **Production:** https://api.example.com/loans',
        '',
        '## Resources',
        '',
        '- **GET** `/loans` — List loans',
        '',
      ].join('\n'),
    );
    expect(screen.getByRole('region', { name: 'Preview' })).toHaveTextContent('List loans');
    expect(screen.getByText(/Publishing replaces the portal’s default section/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeEnabled();
    expect(reads.count()).toBe(0);

    await user.type(editor, '!');
    expect(screen.queryByText(/Publishing replaces the portal’s default section/)).not.toBeInTheDocument();
  });

  it('drops the generated-page notice once the page is edited, and never shows it for a page that was already there', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ hasLandingPage: true }) });
    server.use(landingPageAt(DRAFT_LANDING_PATH, '# Existing'));
    const { user } = renderPage();

    await openLandingPage(user);
    expect(await screen.findByRole('textbox', { name: EDITOR })).toHaveValue('# Existing');
    expect(screen.queryByText(/Publishing replaces the portal’s default section/)).not.toBeInTheDocument();
  });

  it('shows what is typed in the preview as it is typed', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ hasLandingPage: true }) });
    server.use(landingPageAt(DRAFT_LANDING_PATH, ''));
    const { user } = renderPage();

    await openLandingPage(user);
    expect(await screen.findByText('Your landing page appears here as you write.')).toBeInTheDocument();
    await user.type(screen.getByRole('textbox', { name: EDITOR }), '# Loans');

    expect(await within(screen.getByRole('region', { name: 'Preview' })).findByRole('heading', { name: 'Loans' })).toBeInTheDocument();
  });

  it('opens on the draft’s own page rather than the published one, and does not save it again', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails({ hasLandingPage: true }),
      publication: aPublication({ hasLandingPage: true }),
    });
    const draftReads = recorder();
    const saves = recorder();
    server.use(
      landingPageAt(DRAFT_LANDING_PATH, '# Draft page', draftReads),
      landingPageAt(PUBLICATION_LANDING_PATH, '# Published page'),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      noContent('put', DRAFT_LANDING_PATH, { record: saves }),
    );
    const { user } = renderPage();

    await showDraft(user);
    await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
    expect(await screen.findByDisplayValue('# Draft page')).toBeInTheDocument();
    expect(draftReads.count()).toBe(1);

    await user.click(screen.getByRole('tab', { name: 'API Details' }));
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));
    expect(await screen.findByText('Draft saved.')).toBeInTheDocument();
    expect(saves.count()).toBe(0);
  });

  it('opens on the published page when the draft has none, and saves it into the draft', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails(),
      publication: aPublication({ hasLandingPage: true }),
    });
    const saves = recorder();
    server.use(
      landingPageAt(PUBLICATION_LANDING_PATH, '# Published page'),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      noContent('put', DRAFT_LANDING_PATH, { record: saves }),
    );
    const { user } = renderPage();

    await showDraft(user);
    await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
    expect(await screen.findByDisplayValue('# Published page')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

    await user.click(screen.getByRole('tab', { name: 'API Details' }));
    await makeEdit(user);
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(saves.count()).toBe(1));
    expect(saves.last()?.body).toBe('# Published page');
    expect(saves.last()?.headers.get('content-type')).toContain('text/markdown');
  });

  it('saves the edited page with Save Draft', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ hasLandingPage: true }) });
    const saves = recorder();
    server.use(
      landingPageAt(DRAFT_LANDING_PATH, '# Draft page'),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      noContent('put', DRAFT_LANDING_PATH, { record: saves }),
    );
    const { user } = renderPage();

    await openLandingPage(user);
    await user.type(await screen.findByRole('textbox', { name: EDITOR }), ' edited');
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(saves.count()).toBe(1));
    expect(saves.last()?.body).toBe('# Draft page edited');
    expect(await screen.findByText('Draft saved.')).toBeInTheDocument();
    expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
  });

  it('still saves the landing page when the thumbnail fails, and does not say the draft was saved', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ hasLandingPage: true }) });
    const saves = recorder();
    server.use(
      landingPageAt(DRAFT_LANDING_PATH, '# Draft page'),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      noContent('put', DRAFT_LANDING_PATH, { record: saves }),
    );
    const thumbnail = answerThumbnailSaves({ code: 'INTERNAL_ERROR', status: 500 });
    const { container, user } = renderPage();

    await screen.findByDisplayValue(API_NAME);
    await user.upload(
      container.querySelector<HTMLInputElement>('input[type="file"]')!,
      new File(['png'], 'logo.png', { type: 'image/png' }),
    );
    await openLandingPage(user);
    await user.type(await screen.findByRole('textbox', { name: EDITOR }), ' edited');
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(thumbnail.saves()).toBe(1));
    await waitFor(() => expect(saves.count()).toBe(1));
    expect(saves.last()?.body).toBe('# Draft page edited');
    expect(screen.queryByText('Draft saved.')).not.toBeInTheDocument();
  });

  it('Publish saves the landing page before it publishes', async () => {
    servePublicationState({ draft: aPublicationDraftDetails() });
    const order: string[] = [];
    server.use(
      definitionText(DRAFT_DEFINITION_PATH, YAML_DEFINITION, 'application/yaml'),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      mswHttp.put(apiUrl(DRAFT_LANDING_PATH), () => {
        order.push('landing page');
        return new HttpResponse(null, { status: 204 });
      }),
      mswHttp.post(apiUrl(PUBLISH_PATH), () => {
        order.push('publish');
        return HttpResponse.json(aPublication());
      }),
    );
    const { user } = renderPage();

    await openLandingPage(user);
    await user.click(screen.getByRole('button', { name: 'Create from API details' }));
    await user.click(screen.getByRole('button', { name: 'Publish' }));

    expect(await screen.findByText('portals listing')).toBeInTheDocument();
    expect(order).toEqual(['landing page', 'publish']);
  });

  it('opens no editor when a stored page could not be read, so it is not written over', async () => {
    servePublicationState({ draft: aPublicationDraftDetails({ hasLandingPage: true }) });
    server.use(failure('get', DRAFT_LANDING_PATH, 500, 'INTERNAL_ERROR'));
    const { user } = renderPage();

    await openLandingPage(user);

    expect(await screen.findByText('Unable to load the landing page.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create from API details' })).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: EDITOR })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
  });

  it('shows the published page as rendered text only in the published view', async () => {
    servePublicationState({ publication: aPublication({ hasLandingPage: true }) });
    server.use(landingPageAt(PUBLICATION_LANDING_PATH, '# Live page'));
    const { user } = renderPage();

    await openLandingPage(user);

    expect(await screen.findByRole('heading', { name: 'Live page' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: EDITOR })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create from API details' })).not.toBeInTheDocument();
  });

  it('says the portal shows its default page when none is published, with nothing to create', async () => {
    servePublicationState({ publication: aPublication() });
    const { user } = renderPage();

    await openLandingPage(user);

    expect(await screen.findByText('The portal shows its default page.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create from API details' })).not.toBeInTheDocument();
  });
});
