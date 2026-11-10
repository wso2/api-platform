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
import { useState } from 'react';
import { Route, Routes, useLocation } from 'react-router-dom';

import type { QueryClient } from '@tanstack/react-query';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { orgScope } from '@/api/core/queryKeys';
import { apiDocumentQueries } from '@/api/resources/apiDocuments/apiDocuments.queries';
import { apiPublicationKeys } from '@/api/resources/apiPublications/apiPublications.queries';
import { apiThumbnailQueries } from '@/api/resources/apiThumbnail/apiThumbnail.queries';
import { routes } from '@/routes/paths';
import {
  accepts,
  apiUrl,
  aPublication,
  aPublicationDraftDetails,
  aPublicationSummary,
  aRestApi,
  collection,
  failure,
  recorder,
  resource,
  type PublicationSummaryFixture,
  type Recorder,
} from '@/test/msw';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { makeAuthState } from '@/test/mockAuthState';
import { ConsoleScopeContext } from '@/scope/ConsoleScopeContext';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { makeTestQueryClient, renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { ApiPortalPublicationsList } from './ApiPortalPublicationsList';

vi.mock('@/featureUsage/moesif');

const ORG = 'api-platform-demo';
const OTHER_ORG = 'api-platform-staging';
const PROJECT = 'retail-apis';
const API = 'loan-mgmt';
const OTHER_API = 'payments';

const api = aRestApi({ displayName: 'Loan Management Service', id: API, projectId: PROJECT });

let requests: Recorder;

/** Stands in for the publish page and exposes the navigation state the card passed it. */
function PublishFlow() {
  const state = useLocation().state as unknown;
  return (
    <>
      <div>publish flow</div>
      <output data-testid="flow-state">{JSON.stringify(state)}</output>
    </>
  );
}

/**
 * The page's hook reads `ApiScopeContext` for the org (the endpoint takes no
 * `projectId`), and the console scope for the API's own handle and display
 * name — hence both providers, same as `DeployPage.test.tsx`.
 */
function renderPage(queryClient?: QueryClient, scopes?: string[]) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<ApiPortalPublicationsList />} path={routes.apiPortals()} />
        {/* Stands in for the per-portal publish flow, so the split button's navigation is observable. */}
        <Route element={<PublishFlow />} path={routes.apiPortalPublish()} />
      </Routes>
    </ApiScopeProvider>,
    {
      ...(scopes && {
        authState: makeAuthState({ user: { email: 'test.user@example.com', name: 'Test User', scopes } }),
        permissionMode: 'enforce' as const,
      }),
      queryClient,
      route: `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/portals`,
      scope: makeConsoleScope({
        component: api,
        params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

/** The navigation state the card handed the publish flow. */
const flowState = () => JSON.parse(screen.getByTestId('flow-state').textContent ?? '{}') as unknown;

const servePublications = (items: PublicationSummaryFixture[]) => {
  server.use(collection('/api-publications', items, { record: requests }));
};

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  URL.createObjectURL = vi.fn(() => 'blob:test/thumbnail');
  URL.revokeObjectURL = vi.fn();
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
    // Published, so there's a live listing to link to.
    expect(screen.getByRole('link', { name: 'View in Portal' })).toBeInTheDocument();
  });

  it('keeps a pending draft off the card, whatever the status', async () => {
    servePublications([
      aPublicationSummary({ status: 'PUBLISHED', draftUpdatedAt: '2026-02-01T00:00:00Z' }),
    ]);

    renderPage();

    expect(await screen.findByText('Published')).toBeInTheDocument();
    expect(screen.queryByText('Draft')).not.toBeInTheDocument();
    expect(screen.queryByText('Unpublished changes')).not.toBeInTheDocument();
  });

  it('shows Not published for an untouched portal', async () => {
    servePublications([aPublicationSummary({ status: 'NOT_PUBLISHED', draftUpdatedAt: null })]);

    renderPage();

    expect(await screen.findByText('Not published')).toBeInTheDocument();
    // Never published, so there's no listing on the portal to link to yet.
    expect(screen.queryByRole('link', { name: 'View in Portal' })).not.toBeInTheDocument();
    expect(screen.getByText('View in Portal').closest('a')).toHaveAttribute('aria-disabled', 'true');
  });

  it('summarises how many portals the API is published to', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'one', status: 'PUBLISHED' }),
      aPublicationSummary({ apiPortalId: 'two', status: 'DEPRECATED' }),
      aPublicationSummary({ apiPortalId: 'three', status: 'NOT_PUBLISHED' }),
    ]);

    renderPage();

    expect(await screen.findByText('Published to 1 of 3 portals')).toBeInTheDocument();
  });

  it('shows the shared empty state when the organization has no portals', async () => {
    servePublications([]);

    renderPage();

    expect(await screen.findByText('Create your first portal')).toBeInTheDocument();
    // The portal preview animation stands in for an icon.
    expect(screen.getByText('API catalog')).toBeInTheDocument();
    // No CTA when the managed-portals extension isn't registered (the empty-state
    // link points at that extension's route, so a naked button would 404 in OSS).
    expect(screen.queryByRole('button', { name: 'Add Portal' })).not.toBeInTheDocument();
  });

  it('opens the publish flow for the portal the card names', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' }),
    ]);
    const { user } = renderPage();

    // The split button's default action is Publish; "Configure & Publish" sits behind its arrow.
    await user.click(await screen.findByRole('button', { name: 'More publish options' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Configure & Publish' }));
    await user.click(screen.getByRole('button', { name: 'Configure & Publish' }));

    expect(await screen.findByText('publish flow')).toBeInTheDocument();
    expect(flowState()).toEqual({ portalName: 'API Portal 1' });
  });

  describe('one-click Publish', () => {
    const PORTAL_BASE = `/api-portals/acme-portal/apis/rest-api/${API}`;
    const DRAFT_PATH = `${PORTAL_BASE}/draft`;
    const DOCS_PATH = `/apis/rest-api/${API}/docs`;
    const TWO_DOCS = [
      { contentType: 'text/markdown', displayName: 'Getting started', id: 'guide', type: 'HowTo' },
      { contentType: 'text/markdown', displayName: 'Common questions', id: 'faq', type: 'FAQ' },
    ];
    const SPEC = '{"openapi":"3.0.3","info":{"title":"Loans","version":"1.0.0"},"paths":{}}';

    /** What the card reads before it publishes: nothing saved for the portal, and the API's own content. */
    function serveNothingSaved({ draft = false, thumbnail = false } = {}) {
      server.use(
        resource('/rest-apis/:restApiId', api),
        collection(`/rest-apis/${API}/gateways`, []),
        draft ? resource(DRAFT_PATH, aPublicationDraftDetails()) : failure('get', DRAFT_PATH, 404, 'DRAFT_NOT_FOUND'),
        failure('get', `${DRAFT_PATH}/definition`, 404, 'DRAFT_NOT_FOUND'),
        failure('get', `${PORTAL_BASE}/publication`, 404, 'PUBLICATION_NOT_FOUND'),
        failure('get', `${PORTAL_BASE}/publication/definition`, 404, 'PUBLICATION_NOT_FOUND'),
        resource(`/rest-apis/${API}/openapi`, { content: SPEC }),
        mswHttp.get(apiUrl(`/apis/rest-api/${API}/thumbnail`), () =>
          thumbnail
            ? new HttpResponse(new Uint8Array([0x89, 0x50]), { headers: { 'Content-Type': 'image/png' } })
            : new HttpResponse(null, { status: 204 }),
        ),
        collection(DOCS_PATH, TWO_DOCS),
      );
    }

    /** A promise that settles only when `release` is called, to hold a request open. */
    function heldOpen() {
      let release: () => void = () => {};
      const until = new Promise<void>((resolve) => {
        release = resolve;
      });
      return { release, until };
    }

    /**
     * Records each write the publish makes. The publish itself answers with `publish`;
     * with `draftUntil`, the draft save is held open until that settles.
     */
    function recordWrites({
      draftUntil,
      publish = () => HttpResponse.json(aPublication()) as Response,
    }: { draftUntil?: Promise<void>; publish?: () => Response | Promise<Response> } = {}) {
      const writes = { draft: recorder(), definition: recorder(), thumbnail: recorder(), publish: recorder() };
      server.use(
        mswHttp.put(apiUrl(DRAFT_PATH), async ({ request }) => {
          await writes.draft.capture(request);
          await draftUntil;
          return HttpResponse.json(aPublicationDraftDetails());
        }),
        accepts('put', `${DRAFT_PATH}/definition`, undefined, { record: writes.definition }),
        accepts('put', `${DRAFT_PATH}/thumbnail`, undefined, { record: writes.thumbnail }),
        mswHttp.post(apiUrl(`${PORTAL_BASE}/publish`), async ({ request }) => {
          await writes.publish.capture(request);
          return publish();
        }),
      );
      return writes;
    }

    const clickPublish = async (user: ReturnType<typeof renderPage>['user']) =>
      user.click(await screen.findByRole('button', { name: 'Publish' }));

    /**
     * The list as the API switcher, or Back and Forward, reuse it between two Portals pages: the
     * organization or the API changes and the list stays mounted.
     */
    function renderPageWithSwitches() {
      function PageWithSwitches() {
        const [org, setOrg] = useState(ORG);
        const [apiHandler, setApiHandler] = useState(API);
        const scope = makeConsoleScope({
          component: api,
          params: { apiHandler, orgHandle: org, projectHandler: PROJECT },
        });
        return (
          <ConsoleScopeContext.Provider value={scope}>
            <button onClick={() => setOrg(OTHER_ORG)} type="button">
              Switch to the other org
            </button>
            <button onClick={() => setApiHandler(OTHER_API)} type="button">
              Switch to the other API
            </button>
            <button
              onClick={() => {
                setOrg(ORG);
                setApiHandler(API);
              }}
              type="button"
            >
              Switch back
            </button>
            <ApiScopeProvider orgId={org} projectId={PROJECT}>
              <Routes>
                <Route element={<ApiPortalPublicationsList />} path={routes.apiPortals()} />
                <Route element={<PublishFlow />} path={routes.apiPortalPublish()} />
              </Routes>
            </ApiScopeProvider>
          </ConsoleScopeContext.Provider>
        );
      }
      return renderWithProviders(<PageWithSwitches />, {
        route: `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/portals`,
      });
    }

    const orgsWrittenTo = (...recorders: Recorder[]) =>
      recorders.flatMap((record) => record.calls.map((call) => call.headers.get('X-Org-Id')));

    it('publishes from the card with the API’s details and specification, every document and no plan', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' })]);
      serveNothingSaved();
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('Published Loan Management Service to API Portal 1.')).toBeInTheDocument();
      const draft = JSON.parse(writes.draft.last()?.body ?? '{}');
      expect(draft).toMatchObject({ displayName: 'Loan Management Service', subscriptionPlanIds: [] });
      expect([...draft.docIds].sort()).toEqual(['faq', 'guide']);
      expect(JSON.parse(writes.definition.last()?.body ?? '{}')).toMatchObject({ openapi: '3.0.3' });
      expect(writes.thumbnail.count()).toBe(0);
      expect(writes.publish.count()).toBe(1);
      expect(screen.queryByText('publish flow')).not.toBeInTheDocument();
    });

    it('reports the one-click publish as a publish click', async () => {
      resetTrackedFeatures();
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' })]);
      serveNothingSaved();
      recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('Published Loan Management Service to API Portal 1.')).toBeInTheDocument();
      expect(trackedFeatures()).toEqual([['apip.lifecycle.api-publish-click', { api_type: 'rest-api' }]]);
    });

    it('sends the API’s own thumbnail when it has one', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved({ thumbnail: true });
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      await waitFor(() => expect(writes.publish.count()).toBe(1));
      expect(writes.thumbnail.count()).toBe(1);
    });

    it('waits for the API’s thumbnail to be read again over an older cached copy', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      const apiThumbnailRead = heldOpen();
      server.use(
        mswHttp.get(apiUrl(`/apis/rest-api/${API}/thumbnail`), async () => {
          await apiThumbnailRead.until;
          return new HttpResponse(new Uint8Array([0x89, 0x50]), { headers: { 'Content-Type': 'image/png' } });
        }),
      );
      const writes = recordWrites();
      const queryClient = makeTestQueryClient();
      queryClient.setQueryData(apiThumbnailQueries.blob(orgScope(ORG)!, 'rest-api', API).queryKey, null, {
        updatedAt: 0,
      });
      const { user } = renderPage(queryClient);

      await clickPublish(user);
      await screen.findByRole('button', { name: 'Publishing…' });
      await new Promise((resolve) => setTimeout(resolve, 200));
      expect(writes.draft.count()).toBe(0);

      apiThumbnailRead.release();
      await waitFor(() => expect(writes.publish.count()).toBe(1));
      expect(writes.thumbnail.count()).toBe(1);
    });

    it('shows progress on the button and holds the menu until the publish ends', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      const publish = heldOpen();
      const writes = recordWrites({
        publish: async () => {
          await publish.until;
          return HttpResponse.json(aPublication());
        },
      });
      const { user } = renderPage();

      await clickPublish(user);

      const running = await screen.findByRole('button', { name: 'Publishing…' });
      expect(within(running).getByRole('progressbar')).toBeInTheDocument();
      expect(running).toBeDisabled();
      expect(screen.getByRole('button', { name: 'More publish options' })).toBeDisabled();
      await waitFor(() => expect(writes.draft.count()).toBe(1));
      expect(screen.getByRole('button', { name: 'Publishing…' })).toBeDisabled();

      publish.release();
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Publishing…' })).not.toBeInTheDocument());
    });

    it('keeps a publish running, and its card busy, while the search hides the card', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' })]);
      serveNothingSaved();
      const publish = heldOpen();
      const writes = recordWrites({
        publish: async () => {
          await publish.until;
          return HttpResponse.json(aPublication());
        },
      });
      const { user } = renderPage();

      await clickPublish(user);
      await screen.findByRole('button', { name: 'Publishing…' });
      await user.type(screen.getByPlaceholderText('Search portals'), 'zzz');
      await screen.findByText('No matching portals');
      await user.clear(screen.getByPlaceholderText('Search portals'));

      expect(await screen.findByRole('button', { name: 'Publishing…' })).toBeDisabled();
      expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();

      publish.release();
      expect(await screen.findByText('Published Loan Management Service to API Portal 1.')).toBeInTheDocument();
      expect(writes.publish.count()).toBe(1);
    });

    it('finishes a publish in the organization it started in after the page moves to another', async () => {
      servePublications([
        aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' }),
      ]);
      serveNothingSaved();
      const draftSave = heldOpen();
      const writes = recordWrites({ draftUntil: draftSave.until });
      const { user } = renderPageWithSwitches();

      await clickPublish(user);
      await waitFor(() => expect(writes.draft.count()).toBe(1));
      await user.click(screen.getByRole('button', { name: 'Switch to the other org' }));

      // The other organization's portal of the same name is not the one being published.
      expect(await screen.findByRole('button', { name: 'Publish' })).toBeEnabled();
      draftSave.release();
      expect(await screen.findByText('Published Loan Management Service to API Portal 1.')).toBeInTheDocument();
      expect(orgsWrittenTo(writes.draft, writes.definition, writes.publish)).toEqual([
        ORG,
        ORG,
        ORG,
      ]);

      // Configure & Publish there opens that organization's publish page as usual.
      await user.click(screen.getByRole('button', { name: 'More publish options' }));
      await user.click(await screen.findByRole('menuitem', { name: 'Configure & Publish' }));
      await user.click(screen.getByRole('button', { name: 'Configure & Publish' }));
      expect(await screen.findByText('publish flow')).toBeInTheDocument();
    });

    it.each([
      ['organization', 'Switch to the other org'],
      ['API', 'Switch to the other API'],
    ])('drops a publish that has written nothing when the page moves to another %s', async (_, move) => {
      servePublications([
        aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' }),
      ]);
      serveNothingSaved();
      const apiThumbnailRead = heldOpen();
      server.use(
        mswHttp.get(apiUrl(`/apis/rest-api/${API}/thumbnail`), async () => {
          await apiThumbnailRead.until;
          return new HttpResponse(null, { status: 204 });
        }),
      );
      const writes = recordWrites();
      const { user } = renderPageWithSwitches();

      await clickPublish(user);
      await screen.findByRole('button', { name: 'Publishing…' });
      await user.click(screen.getByRole('button', { name: move }));
      await user.click(screen.getByRole('button', { name: 'Switch back' }));

      expect(await screen.findByRole('button', { name: 'Publish' })).toBeEnabled();
      apiThumbnailRead.release();
      await new Promise((resolve) => setTimeout(resolve, 200));
      expect(
        orgsWrittenTo(writes.draft, writes.definition, writes.thumbnail, writes.publish),
      ).toEqual([]);
      expect(screen.queryByText('publish flow')).not.toBeInTheDocument();
      expect(screen.queryByText(/Nothing was published/)).not.toBeInTheDocument();
    });

    it('keeps a publish running, once, while the portal list fails to load and loads again', async () => {
      const rollup = [aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' })];
      servePublications(rollup);
      serveNothingSaved();
      const draftSave = heldOpen();
      const writes = recordWrites({ draftUntil: draftSave.until });
      const queryClient = makeTestQueryClient();
      const reloadList = () => queryClient.invalidateQueries({ queryKey: apiPublicationKeys.lists(orgScope(ORG)!) });
      const { user } = renderPage(queryClient);

      await clickPublish(user);
      await waitFor(() => expect(writes.draft.count()).toBe(1));
      server.use(failure('get', '/api-publications', 500, 'INTERNAL_ERROR'));
      await reloadList();
      expect(await screen.findByText('Unable to load portals')).toBeInTheDocument();
      servePublications(rollup);
      await reloadList();
      expect(await screen.findByRole('button', { name: 'Publishing…' })).toBeDisabled();
      // Time for a publish started over again to make its reads and save the draft a second time.
      await new Promise((resolve) => setTimeout(resolve, 200));
      expect(writes.draft.count()).toBe(1);

      draftSave.release();
      expect(await screen.findByText('Published Loan Management Service to API Portal 1.')).toBeInTheDocument();
      expect(writes.draft.count()).toBe(1);
      expect(writes.publish.count()).toBe(1);
    });

    it('keeps showing progress when the list is read again mid-publish and shows the saved draft', async () => {
      const rollup = [aPublicationSummary({ apiPortalId: 'acme-portal' })];
      servePublications(rollup);
      serveNothingSaved();
      const publish = heldOpen();
      const writes = recordWrites({
        publish: async () => {
          await publish.until;
          return HttpResponse.json(aPublication());
        },
      });
      const queryClient = makeTestQueryClient();
      const { user } = renderPage(queryClient);

      await clickPublish(user);
      await waitFor(() => expect(writes.publish.count()).toBe(1));
      rollup[0] = { ...rollup[0], draftUpdatedAt: '2026-02-01T00:00:00Z' };
      await queryClient.invalidateQueries({ queryKey: apiPublicationKeys.lists(orgScope(ORG)!) });
      await waitFor(() => expect(requests.count()).toBeGreaterThan(1));

      expect(screen.getByRole('button', { name: 'Publishing…' })).toBeDisabled();

      publish.release();
      expect(await screen.findByRole('button', { name: 'Configure & Publish' })).toBeEnabled();
    });

    it('stays on the list and offers Configure & Publish for the saved draft when the publish fails', async () => {
      const rollup = [aPublicationSummary({ apiPortalId: 'acme-portal' })];
      servePublications(rollup);
      serveNothingSaved();
      const writes = recordWrites({
        publish: () => {
          // The draft was saved before the publish failed, and the rollup read after the failure says so.
          rollup[0] = { ...rollup[0], draftUpdatedAt: '2026-02-01T00:00:00Z' };
          return HttpResponse.json(
            { code: 'PUBLICATION_VALIDATION_FAILED', message: 'Invalid.', status: 'error' } as never,
            { status: 400 },
          );
        },
      });
      const { user } = renderPage();

      await clickPublish(user);

      await waitFor(() => expect(writes.publish.count()).toBe(1));
      await waitFor(() => expect(screen.getByRole('button', { name: 'Configure & Publish' })).toBeEnabled());
      expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
      expect(screen.queryByText('publish flow')).not.toBeInTheDocument();
    });

    it('offers Configure & Publish for the saved draft when a step after the draft save fails', async () => {
      const rollup = [aPublicationSummary({ apiPortalId: 'acme-portal' })];
      servePublications(rollup);
      serveNothingSaved();
      const writes = recordWrites();
      server.use(
        mswHttp.put(apiUrl(`${DRAFT_PATH}/definition`), () => {
          // The draft is saved by now, and the rollup read after the failure says so.
          rollup[0] = { ...rollup[0], draftUpdatedAt: '2026-02-01T00:00:00Z' };
          return HttpResponse.json(
            { code: 'INTERNAL_ERROR', message: 'Failed.', status: 'error' } as never,
            { status: 500 },
          );
        }),
      );
      const { user } = renderPage();

      await clickPublish(user);

      await waitFor(() => expect(writes.draft.count()).toBe(1));
      await waitFor(() => expect(screen.getByRole('button', { name: 'Configure & Publish' })).toBeEnabled());
      expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
      expect(writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when a draft was saved since the card was drawn', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1' })]);
      serveNothingSaved({ draft: true });
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow')).toBeInTheDocument();
      expect(flowState()).toEqual({ portalName: 'API Portal 1' });
      expect(
        screen.getByText('Nothing was published to API Portal 1. Check the details here, then publish.'),
      ).toBeInTheDocument();
      expect(writes.draft.count() + writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when the documents cannot be listed', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(failure('get', DOCS_PATH, 500, 'INTERNAL_ERROR'));
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow', undefined, { timeout: 5000 })).toBeInTheDocument();
      expect(writes.draft.count() + writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when the documents cannot be listed again over an earlier list', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(failure('get', DOCS_PATH, 500, 'INTERNAL_ERROR'));
      const writes = recordWrites();
      const queryClient = makeTestQueryClient();
      queryClient.setQueryData(
        apiDocumentQueries.listAll(orgScope(ORG)!, 'rest-api', API).queryKey,
        { count: 2, list: TWO_DOCS, pagination: { limit: 2, offset: 0, total: 2 } },
        { updatedAt: 0 },
      );
      const { user } = renderPage(queryClient);

      await clickPublish(user);

      expect(await screen.findByText('publish flow')).toBeInTheDocument();
      expect(writes.draft.count() + writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when the document list comes back short', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(
        mswHttp.get(apiUrl(DOCS_PATH), () =>
          HttpResponse.json({ count: 1, list: [TWO_DOCS[0]], pagination: { limit: 1, offset: 0, total: 2 } }),
        ),
      );
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow')).toBeInTheDocument();
      expect(writes.draft.count() + writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when the API’s thumbnail cannot be read', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(failure('get', `/apis/rest-api/${API}/thumbnail`, 500, 'INTERNAL_ERROR'));
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow', undefined, { timeout: 5000 })).toBeInTheDocument();
      expect(writes.draft.count() + writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when the API’s thumbnail is not an image the portal takes', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(
        mswHttp.get(apiUrl(`/apis/rest-api/${API}/thumbnail`), () =>
          new HttpResponse(new Uint8Array([0x47, 0x49]), { headers: { 'Content-Type': 'image/gif' } }),
        ),
      );
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow')).toBeInTheDocument();
      expect(writes.draft.count() + writes.thumbnail.count() + writes.publish.count()).toBe(0);
    });

    it('opens the publish page instead when the gateways cannot be read for the Production URL', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(failure('get', `/rest-apis/${API}/gateways`, 500, 'INTERNAL_ERROR'));
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow', undefined, { timeout: 5000 })).toBeInTheDocument();
      expect(writes.draft.count() + writes.publish.count()).toBe(0);
    });

    it.each([
      ['has no specification', null],
      ['has an empty specification', ''],
    ])('opens the publish page instead when the API %s', async (_case, content) => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
      serveNothingSaved();
      server.use(
        content === null
          ? failure('get', `/rest-apis/${API}/openapi`, 404, 'NOT_FOUND')
          : resource(`/rest-apis/${API}/openapi`, { content }),
      );
      const writes = recordWrites();
      const { user } = renderPage();

      await clickPublish(user);

      expect(await screen.findByText('publish flow')).toBeInTheDocument();
      expect(writes.draft.count() + writes.definition.count() + writes.publish.count()).toBe(0);
    });

    it('publishes from a portal whose id is also an object property name', async () => {
      servePublications([aPublicationSummary({ apiPortalId: 'constructor' })]);

      renderPage();

      expect(await screen.findByRole('button', { name: 'Publish' })).toBeEnabled();
    });
  });

  it('offers only Configure & Publish to a caller who may publish but not save a draft', async () => {
    servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);

    renderPage(undefined, ['ap:api_portal:rest_api:manage']);

    expect(await screen.findByRole('button', { name: 'Configure & Publish' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
  });

  it('offers only Configure & Publish once a draft is saved, even before the first publish', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'acme-portal', draftUpdatedAt: '2026-02-01T00:00:00Z', status: 'NOT_PUBLISHED' }),
    ]);

    renderPage();

    expect(await screen.findByRole('button', { name: 'Configure & Publish' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'More publish options' })).not.toBeInTheDocument();
  });

  it.each(['PUBLISHED', 'DEPRECATED'] as const)(
    'offers only Configure & Publish once the API is %s on the portal',
    async (status) => {
      servePublications([aPublicationSummary({ apiPortalId: 'acme-portal', apiPortalName: 'API Portal 1', status })]);
      const { user } = renderPage();

      const configure = await screen.findByRole('button', { name: 'Configure & Publish' });
      expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'More publish options' })).not.toBeInTheDocument();
      expect(configure).toHaveAccessibleDescription('API Portal 1');

      await user.click(configure);

      expect(await screen.findByText('publish flow')).toBeInTheDocument();
      expect(flowState()).toEqual({ portalName: 'API Portal 1' });
    },
  );

  it('keeps Publish as the button label after the menu is opened and closed', async () => {
    servePublications([aPublicationSummary({ apiPortalId: 'acme-portal' })]);
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'More publish options' }));
    expect(await screen.findAllByRole('menuitem')).toHaveLength(2);
    await user.keyboard('{Escape}');

    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument();
  });

  it('names each portal for assistive tech on its otherwise identical button', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'one', apiPortalName: 'API Portal 1' }),
      aPublicationSummary({ apiPortalId: 'two', apiPortalName: 'API Portal 2' }),
    ]);

    renderPage();

    await screen.findByText('API Portal 1');
    const buttons = screen.getAllByRole('button', { name: 'Publish' });
    expect(buttons.map((button) => button.getAttribute('aria-describedby'))).toHaveLength(2);
    expect(buttons[0]).toHaveAccessibleDescription('API Portal 1');
    expect(buttons[1]).toHaveAccessibleDescription('API Portal 2');
  });

  it('shows Deprecated for a deprecated listing', async () => {
    servePublications([aPublicationSummary({ status: 'DEPRECATED', draftUpdatedAt: null })]);

    renderPage();

    expect(await screen.findByText('Deprecated')).toBeInTheDocument();
    // Deprecated listings are still live on the portal, so the link stays.
    expect(screen.getByRole('link', { name: 'View in Portal' })).toHaveAttribute('href');
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

    await screen.findByText('Create your first portal');
    expect(screen.queryByPlaceholderText('Search portals')).not.toBeInTheDocument();
  });

  it('says the user lacks permission when the portals cannot be listed (403)', async () => {
    server.use(failure('get', '/api-publications', 403, 'FORBIDDEN'));

    renderPage();

    expect(await screen.findByText('You don’t have permission')).toBeInTheDocument();
    expect(screen.queryByText('Unable to load portals')).not.toBeInTheDocument();
  });

  it('keeps the generic message for a failure that is not a permission problem', async () => {
    server.use(failure('get', '/api-publications', 500, 'INTERNAL_ERROR'));

    renderPage();

    expect(await screen.findByText('Unable to load portals')).toBeInTheDocument();
    expect(screen.queryByText('You don’t have permission')).not.toBeInTheDocument();
  });

  it('shows when the publication last changed, beside its status', async () => {
    servePublications([
      aPublicationSummary({
        status: 'PUBLISHED',
        publicationUpdatedAt: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
      }),
    ]);

    renderPage();

    expect(await screen.findByText('2 days ago')).toBeInTheDocument();
  });

  it('names a portal by its handle when it has no display name', async () => {
    servePublications([aPublicationSummary({ apiPortalId: 'fallback-handle', apiPortalName: '' })]);

    renderPage();

    expect(await screen.findByText('fallback-handle')).toBeInTheDocument();
  });

  it('marks each card with the developer portal logo', async () => {
    servePublications([
      aPublicationSummary({ apiPortalId: 'one' }),
      aPublicationSummary({ apiPortalId: 'two' }),
    ]);

    const { container } = renderPage();

    await screen.findAllByRole('button', { name: 'Publish' });
    const logos = container.querySelectorAll('img');
    expect(logos).toHaveLength(2);
    logos.forEach((logo) => expect(logo.getAttribute('src')).toContain('devportal-logo'));
  });
});
