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
import { routes } from '@/routes/paths';
import {
  accepts,
  apiUrl,
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
  upstream: { main: { url: 'https://backend.internal/loans' } },
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
const API_OPENAPI_PATH = `/rest-apis/${API}/openapi`;

/** The definition chain's three tiers, each with its own call recorder. */
function definitionTierRecorders() {
  return {
    draftDefinition: recorder(),
    publicationDefinition: recorder(),
    openApi: recorder(),
  };
}

/** The five reads the page makes before it can render the form. */
function servePublicationState({
  draft,
  publication,
  definitionRecorders = definitionTierRecorders(),
}: {
  draft?: PublicationDraftDetailsFixture;
  publication?: PublicationFixture;
  definitionRecorders?: ReturnType<typeof definitionTierRecorders>;
} = {}) {
  server.use(
    resource('/rest-apis/:restApiId', api),
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
});

const API_NAME = 'Loan Management Service';

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
    // Falls back to the API's own upstream URL, not a draft/publication one.
    expect(screen.getByDisplayValue('https://backend.internal/loans')).toBeInTheDocument();
  });

  it('disables Subscription Plans, Documentation and Landing Page for this alpha', async () => {
    servePublicationState();

    renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.getByRole('tab', { name: 'Subscription Plans' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'Documentation' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'Landing Page' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'API Details' })).toBeEnabled();
    expect(screen.getByRole('tab', { name: 'Specification' })).toBeEnabled();
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
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    await waitFor(() => expect(draftRequests.count()).toBe(1));
    await waitFor(() => expect(definitionRequests.count()).toBe(1));
    expect(JSON.parse(draftRequests.last()?.body ?? '{}')).toMatchObject({
      displayName: 'Loan Management Service',
      version: '1.0.0',
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
    await user.click(await screen.findByRole('button', { name: 'Edit' }));
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
    await user.click(await screen.findByRole('button', { name: 'Edit' }));
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
    expect(screen.getByText('Draft version')).toBeInTheDocument();
  });

  it('shows the live details read-only from what is already loaded, and brings the draft back untouched', async () => {
    const publicationRequests = recorder();
    servePublicationState({ draft, publication: published });
    server.use(resource(PUBLICATION_PATH, published, { record: publicationRequests }));

    const { user } = renderPage();

    const draftName = await screen.findByDisplayValue('Draft Name');
    await user.type(draftName, ' edited');
    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(screen.getByRole('button', { name: 'Published' })).toHaveAttribute('aria-pressed', 'true');
    const publishedName = screen.getByDisplayValue('Published Name');
    expect(publishedName).toHaveAttribute('readonly');
    expect(screen.getByText('Published version')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save Draft' })).not.toBeInTheDocument();

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

    await screen.findByDisplayValue('Draft Name');
    await user.click(screen.getByRole('tab', { name: 'Specification' }));
    await user.click(screen.getByRole('button', { name: 'Published' }));

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

    expect(await screen.findByText('v3.0.0 · edited 2 hours ago')).toBeInTheDocument();
    expect(screen.getByText('Draft version')).toBeInTheDocument();
  });

  it('shows no banner for a draft that was never saved', async () => {
    servePublicationState();

    renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();
    expect(screen.queryByText(/not saved yet/)).not.toBeInTheDocument();
  });

  it('shows no banner on the draft side when only the published version exists, but does on the published side', async () => {
    servePublicationState({ publication: aPublication({ version: '2.0.0' }) });

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(await screen.findByText('Published version')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Draft' }));

    expect(screen.queryByText('Published version')).not.toBeInTheDocument();
    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();
  });

  it('dates the published banner from the live publication', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails(),
      publication: aPublication({ version: '2.0.0', updatedAt: hoursAgo(48) }),
    });

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(await screen.findByText('v2.0.0 · updated 2 days ago')).toBeInTheDocument();
    expect(screen.getByText('Published version')).toBeInTheDocument();
  });

  it('calls a deprecated listing deprecated, not published', async () => {
    servePublicationState({
      draft: aPublicationDraftDetails(),
      publication: aPublication({ status: 'DEPRECATED' }),
    });

    const { user } = renderPage();
    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'Published' }));

    expect(await screen.findByText('Deprecated version')).toBeInTheDocument();
    expect(screen.queryByText('Published version')).not.toBeInTheDocument();
  });
});

describe('PortalPublishPage — published specification', () => {
  const draft = aPublicationDraftDetails({ displayName: 'Draft Name' });
  const published = aPublication({ displayName: 'Published Name' });
  const publishedSpec = { openapi: '3.0.3', info: { title: 'Published Name' }, paths: {} };

  async function openPublishedSpecification() {
    const view = renderPage();
    await screen.findByDisplayValue('Draft Name');
    await view.user.click(screen.getByRole('tab', { name: 'Specification' }));
    await view.user.click(screen.getByRole('button', { name: 'Published' }));
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
    await screen.findByDisplayValue('Loan Management Service');
    expect(screen.getByRole('button', { name: 'Published' })).toBeEnabled();

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
    expect(screen.getByText('Draft version')).toBeInTheDocument();
    first.unmount();

    // The publish promoted the draft: the server no longer has one, and the
    // cache still holds the old copy until it is revalidated on the next visit.
    servePublicationState({ publication: aPublication({ displayName: 'Live Name' }) });
    await queryClient.invalidateQueries();
    renderPage(queryClient);

    expect(await screen.findByDisplayValue('Live Name')).toBeInTheDocument();
    expect(screen.queryByDisplayValue('Old Draft Name')).not.toBeInTheDocument();
    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();
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
    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Publish' }));
    // Both saves have gone through; the publish call is still in flight.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish' })).toBeDisabled());
    await new Promise((resolve) => setTimeout(resolve, 150));

    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();
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
    expect(screen.queryByText('Draft version')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText('Draft version')).toBeInTheDocument();
    expect(draftReads.count()).toBe(1);
  });
});
