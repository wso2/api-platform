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
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aDeployment, aGateway, aRestApi, apiUrl, listEnvelope, neverResponds } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { useLocation } from 'react-router-dom';

import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { resetTestApiKeys } from './utils/useTestApiKey';
import { TestPage } from './TestPage';

/**
 * Page-level tests for the test console.
 *
 * The page is the cURL builder and nothing else — the interactive console view
 * it used to toggle between has been removed — so what is under test here is
 * the page's own wiring: which of the loading, error and empty states it picks,
 * whether a test key is minted only when there is somewhere to send it, and
 * whether the command comes out addressed to the deployed gateway.
 *
 * The page derives the request from the definition and the gateway on every
 * render, so a render loop is a live risk. It surfaces as React's "Maximum
 * update depth exceeded", which React reports through `console.error` rather
 * than throwing — the render would otherwise look like a success — so it is
 * captured and asserted on explicitly.
 */

const API = aRestApi({ context: '/payments', id: 'payments-api' });

/**
 * The definition `GET /openapi` returns. Served as a string, the way the
 * endpoint does — the page parses it itself, so a wrapper object reaching the
 * builder instead of the document is exactly the bug this guards.
 */
const SPEC = {
  info: { title: 'Payments', version: '1.0.0' },
  openapi: '3.0.1',
  paths: {
    '/payments': { get: { responses: { '200': { description: 'ok' } } } },
    '/payments/{id}': { delete: { responses: { '204': { description: 'gone' } } } },
  },
};

/** The definition endpoint, in the shape the page consumes. */
const openApiHandler = (spec: object = SPEC) =>
  http.get(apiUrl('/rest-apis/payments-api/openapi'), () =>
    HttpResponse.json({ content: JSON.stringify(spec) }),
  );

/** The same API, secured by `api-key-auth` with the given params. */
const securedApi = (params?: Record<string, unknown>) =>
  aRestApi({
    context: '/payments',
    id: 'payments-api',
    policies: [{ name: 'api-key-auth', version: 'v1', ...(params ? { params } : {}) }],
  } as Parameters<typeof aRestApi>[0]);

/** Counts key-mint calls, so "never minted" can be asserted rather than assumed. */
let mintCalls = 0;

const GATEWAY = aGateway({
  displayName: 'Default gateway',
  endpoints: ['https://gw.example.com'],
  id: 'default-gateway',
});

/** The four mount-time requests, with the API document swapped in. */
const pathFor = (api: object) => [
  http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(api)),
  http.get(apiUrl('/rest-apis/payments-api/gateways'), () =>
    HttpResponse.json(
      listEnvelope([{ ...GATEWAY, associatedAt: '2026-01-01T00:00:00Z', isDeployed: true }]),
    ),
  ),
  http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
    HttpResponse.json(listEnvelope([aDeployment({ gatewayId: 'default-gateway' })])),
  ),
  openApiHandler(),
  http.post(apiUrl('/rest-apis/payments-api/api-keys'), () => {
    mintCalls += 1;
    return HttpResponse.json({
      apiKey: 'live-test-credential',
      keyId: 'k1',
      message: 'ok',
      status: 'success',
    });
  }),
];

/** Every request the page makes when the API is deployed nowhere. */
const undeployed = () => [
  http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(API)),
  http.get(apiUrl('/rest-apis/payments-api/gateways'), () => HttpResponse.json(listEnvelope([]))),
  http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
    HttpResponse.json(listEnvelope([])),
  ),
  openApiHandler(),
  http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
    HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' }),
  ),
];

/** Surfaces the router's current path, so a navigation can be asserted. */
function LocationProbe() {
  return <span data-testid="location">{useLocation().pathname}</span>;
}

const ORG = 'acme-org';
const PROJECT = 'retail';

/**
 * `renderWithProviders` mounts no `ApiScopeProvider`, and every scoped query is
 * gated on `enabled: Boolean(org)` — so without one nothing fetches and the
 * page sits on its loading state forever. Same wrapper as `DeployPage.test`.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <>
        <TestPage />
        <LocationProbe />
      </>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/apis/payments-api/test`,
      scope: makeConsoleScope({
        isApiScope: true,
        params: { apiHandler: 'payments-api', orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

let consoleErrors: string[] = [];

beforeEach(() => {
  mintCalls = 0;
  // Test keys are remembered for the browser session in a module-scoped map, so
  // without this a key minted by one test would satisfy the next and "mints a
  // key" assertions would pass on a request that never happened.
  resetTestApiKeys();
  resetHttpClient();
  consoleErrors = [];
  vi.spyOn(console, 'error').mockImplementation((...args: unknown[]) => {
    consoleErrors.push(String(args[0]));
  });
});

/**
 * The rendered curl command as plain text.
 *
 * `CodeBlock` highlights through Prism, which splits the command across many
 * `<span>`s — so `findByText` cannot see it as one string. Reading the `<pre>`'s
 * `textContent` asks the question the test actually means.
 */
const curlText = (): string =>
  Array.from(document.querySelectorAll('pre'))
    .map((element) => element.textContent ?? '')
    .join('\n');

/** Fails the test on a React error, which a render loop reports rather than throws. */
const expectNoRenderLoop = () => {
  expect(consoleErrors.filter((message) => /Maximum update depth/.test(message))).toEqual([]);
};

describe('TestPage — the cURL builder', () => {
  it('builds a command against the deployed gateway from the first operation', async () => {
    server.use(...pathFor(API));

    renderPage();

    // The builder seeds itself from the document's first operation, so the
    // command is addressed before anyone touches a control.
    await waitFor(() => expect(curlText()).toMatch(/https:\/\/gw\.example\.com/));
    expect(curlText()).toMatch(/curl/);
    expect(await screen.findByText(/cURL command/i)).toBeInTheDocument();
    expectNoRenderLoop();
  });

  it('reports a definition that cannot be parsed rather than an empty builder', async () => {
    server.use(
      ...pathFor(API).slice(0, 3),
      http.get(apiUrl('/rest-apis/payments-api/openapi'), () =>
        HttpResponse.json({ content: 'not: [valid' }),
      ),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
        HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' }),
      ),
    );

    renderPage();

    expect(await screen.findByText(/definition could not be loaded/i)).toBeInTheDocument();
    expect(curlText()).toBe('');
    expectNoRenderLoop();
  });

  // A `404` is the API simply never having had a definition uploaded, which is
  // a prompt to add one — not the failure the message above describes.
  it('shows the add-a-definition empty state when the API has no definition', async () => {
    server.use(
      ...pathFor(API).slice(0, 3),
      http.get(apiUrl('/rest-apis/payments-api/openapi'), () =>
        HttpResponse.json({ code: '404', message: 'not found' }, { status: 404 }),
      ),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
        HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' }),
      ),
    );

    renderPage();

    expect(
      await screen.findByText(/You must add an API definition to start testing/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/cURL command/i)).not.toBeInTheDocument();
    expectNoRenderLoop();
  });
});

describe('TestPage — the api-key-auth gate', () => {
  it('shows the test key panel and mints a key for a secured API', async () => {
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    renderPage();

    expect(await screen.findByText(/Test key/i)).toBeInTheDocument();
    await waitFor(() => expect(mintCalls).toBe(1));
    expectNoRenderLoop();
  });

  it('finds the policy when it is attached to a single operation', async () => {
    const perOperation = aRestApi({
      context: '/payments',
      id: 'payments-api',
      operations: [
        {
          name: 'listBooks',
          request: {
            method: 'GET',
            path: '/books',
            policies: [{ name: 'api-key-auth', version: 'v1', params: { key: 'Op-Key' } }],
          },
        },
      ],
    } as Parameters<typeof aRestApi>[0]);
    server.use(...pathFor(perOperation));

    renderPage();

    expect(await screen.findByText(/Test key/i)).toBeInTheDocument();
    await waitFor(() => expect(mintCalls).toBe(1));
    expectNoRenderLoop();
  });

  it('renders no key panel for an API that needs no key', async () => {
    server.use(...pathFor(API));

    renderPage();

    await screen.findByText(/cURL command/i);
    expect(screen.queryByText(/Test key/i)).not.toBeInTheDocument();
    expect(mintCalls).toBe(0);
    expectNoRenderLoop();
  });
});

describe('TestPage', () => {
  it('renders nothing but the empty state when the API is deployed nowhere', async () => {
    server.use(...undeployed());

    renderPage();

    expect(
      await screen.findByText(/You must deploy the API to start testing/i),
    ).toBeInTheDocument();

    // "Only the banner": with no gateway there is no endpoint, no key worth
    // issuing and no request to build, so every one of these controls would be
    // inert if it rendered.
    expect(screen.queryByText(/Test key/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Endpoint/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/cURL command/i)).not.toBeInTheDocument();
    expect(curlText()).toBe('');
    expectNoRenderLoop();
  });

  // Both lookups fall back to an empty list, so a failed request is
  // indistinguishable from an undeployed API by gateway count alone. Telling
  // someone to deploy an API that is already deployed sends them to redo work
  // and hides the real fault.
  describe('when a deployment lookup fails', () => {
    const failing = (endpoint: 'gateways' | 'deployments') => [
      ...undeployed().filter((handler) => !handler.info.path.toString().endsWith(endpoint)),
      http.get(apiUrl(`/rest-apis/payments-api/${endpoint}`), () =>
        HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 }),
      ),
    ];

    it.each(['gateways', 'deployments'] as const)(
      'reports the failure rather than the deploy prompt when %s fails',
      async (endpoint) => {
        server.use(...failing(endpoint));

        renderPage();

        expect(await screen.findByText(/deployments could not be loaded/i)).toBeInTheDocument();
        expect(
          screen.queryByText(/You must deploy the API to start testing/i),
        ).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Deploy API/i })).not.toBeInTheDocument();
        expectNoRenderLoop();
      },
    );
  });

  // A test key is an ordinary, persisted API key: minting one for a page that
  // renders the deploy prompt instead of a builder spends a real credential and
  // leaves an entry in the user's key list for a console they never reached.
  it('mints no key for a secured API that is deployed nowhere', async () => {
    server.use(
      http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(securedApi())),
      http.get(apiUrl('/rest-apis/payments-api/gateways'), () =>
        HttpResponse.json(listEnvelope([])),
      ),
      http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
        HttpResponse.json(listEnvelope([])),
      ),
      openApiHandler(),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () => {
        mintCalls += 1;
        return HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' });
      }),
    );

    renderPage();

    await screen.findByText(/You must deploy the API to start testing/i);
    expect(mintCalls).toBe(0);
    expectNoRenderLoop();
  });

  it('mints no key when the deployment lookup fails', async () => {
    server.use(
      http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(securedApi())),
      http.get(apiUrl('/rest-apis/payments-api/gateways'), () =>
        HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 }),
      ),
      http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
        HttpResponse.json(listEnvelope([])),
      ),
      openApiHandler(),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () => {
        mintCalls += 1;
        return HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' });
      }),
    );

    renderPage();

    await screen.findByText(/deployments could not be loaded/i);
    expect(mintCalls).toBe(0);
    expectNoRenderLoop();
  });

  it('offers one way out of the empty state, to the Deploy page', async () => {
    server.use(...undeployed());

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: /Deploy API/i }));

    // Built from the same `routes.apiDeploy` builder the Overview page's Deploy
    // button uses, so asserting the destination guards the pairing.
    await waitFor(() =>
      expect(screen.getByTestId('location').textContent).toBe(
        `/organizations/${ORG}/projects/${PROJECT}/apis/payments-api/deploy`,
      ),
    );
  });

  it('shows neither builder nor empty state while deployment is unknown', async () => {
    // An unresolved gateway query reports zero gateways exactly like a
    // genuinely undeployed API. The page holds its loading state until it
    // knows, so the empty state cannot appear on the strength of a pending
    // request — and the builder cannot flash before being replaced by it.
    server.use(
      http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(API)),
      neverResponds('get', '/rest-apis/payments-api/gateways'),
      http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
        HttpResponse.json(listEnvelope([aDeployment({ gatewayId: 'default-gateway' })])),
      ),
      openApiHandler(),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
        HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' }),
      ),
    );

    renderPage();

    expect(await screen.findByText(/Loading test console/i)).toBeInTheDocument();
    expect(screen.queryByText(/You must deploy the API to start testing/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/cURL command/i)).not.toBeInTheDocument();
  });

  it('surfaces the minted test key and the header it travels in', async () => {
    // Secured explicitly: the panel only exists for an API whose api-key-auth
    // policy demands a key, and the header name now comes from that policy.
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    renderPage();

    // The header name is its own <code> element inside the sentence, so the
    // sentence and the name are asserted separately. `getNodeText` joins only
    // direct text nodes, which is why the sentence reads as gapless here.
    expect(
      await screen.findByText(/Sent as the header on requests to this gateway/i),
    ).toBeInTheDocument();
    expect(screen.getByText('X-API-Key')).toBeInTheDocument();
    // Queried by display value, not text: the key sits in a read-only
    // TextField, so the mask is an input's value rather than a text node.
    // Awaited because the key is minted asynchronously — the panel's sentence
    // renders before the value it describes arrives.
    expect(await screen.findByDisplayValue(/•/)).toBeInTheDocument();
    // Masked, not printed. This value is only 20 characters, which an earlier
    // masking rule left fully legible because it only masked values longer
    // than its 28-character prefix.
    expect(screen.queryByDisplayValue('live-test-credential')).toBeNull();
  });

  // Same reasoning one layer on: the command is what gets pasted somewhere, so
  // the key is masked there too until it is revealed deliberately.
  it('keeps the key out of the displayed command until it is revealed', async () => {
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    const { user } = renderPage();

    await waitFor(() => expect(curlText()).toMatch(/X-API-Key/));
    expect(curlText()).not.toMatch(/live-test-credential/);

    await user.click(await screen.findByRole('button', { name: /Key hidden/i }));

    await waitFor(() => expect(curlText()).toMatch(/live-test-credential/));
    expectNoRenderLoop();
  });

  it('reports a failed mint rather than masking it', async () => {
    server.use(
      ...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })).slice(0, 4),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
        HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 }),
      ),
    );

    renderPage();

    // A development-only placeholder key used to stand in here, which meant a
    // real failure read as a working credential. Nothing masks it now.
    expect(await screen.findByText(/Could not create a test key/i)).toBeInTheDocument();
    expect(screen.queryByDisplayValue(/•/)).toBeNull();
    expectNoRenderLoop();
  });
});
