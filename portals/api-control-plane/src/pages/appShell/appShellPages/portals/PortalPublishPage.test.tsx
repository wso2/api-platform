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
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
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

  it('sends the user back to the Specification tab, naming the format, when the definition cannot be read', async () => {
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
    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    await user.type(
      await screen.findByRole('textbox', { name: 'API definition (YAML)' }),
      '\n  bad: [[',
    );
    await user.click(screen.getByRole('button', { name: 'Save Draft' }));

    expect(await screen.findByText(/This is not valid YAML:/)).toBeInTheDocument();
    expect(definitionRequests.count()).toBe(0);
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

  it('goes back to Publish once the API has been unpublished', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(noContent('post', UNPUBLISH_PATH));

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));

    // The refetch that follows the unpublish now finds no live listing.
    server.use(failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'));
    await confirmInDialog(user);

    expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Unpublish' })).not.toBeInTheDocument();
  });

  it('goes back to Publish once the API has been deprecated', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(accepts('post', DEPRECATE_PATH, aPublication({ status: 'DEPRECATED' })));

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Deprecate' }));
    await user.click(await screen.findByRole('button', { name: 'Deprecate' }));

    server.use(resource(PUBLICATION_PATH, aPublication({ status: 'DEPRECATED' })));
    await confirmInDialog(user);

    expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Deprecate' })).not.toBeInTheDocument();
  });

  it('keeps Publish as the button after unpublishing and publishing again', async () => {
    servePublicationState({ publication: aPublication() });
    server.use(
      noContent('post', UNPUBLISH_PATH),
      accepts('put', DRAFT_PATH, aPublicationDraftDetails()),
      accepts('put', DRAFT_DEFINITION_PATH, undefined),
      accepts('post', PUBLISH_PATH, aPublication()),
    );

    const { user } = renderPage();

    await screen.findByDisplayValue('Loan Management Service');
    await user.click(screen.getByRole('button', { name: 'More publish actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Unpublish' }));
    await user.click(await screen.findByRole('button', { name: 'Unpublish' }));
    server.use(failure('get', PUBLICATION_PATH, 404, 'PUBLICATION_NOT_FOUND'));
    await confirmInDialog(user);
    await screen.findByText('Unpublished from acme-portal.');

    server.use(resource(PUBLICATION_PATH, aPublication()));
    await user.click(await screen.findByRole('button', { name: 'Publish' }));
    await screen.findByText('Published to acme-portal.');

    // Live again — the primary side must still say Publish, not fall back to the old Unpublish.
    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Unpublish' })).not.toBeInTheDocument();
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
            : `This marks the API "${API_NAME}" as deprecated on acme-portal. It stays visible there.`,
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
});
