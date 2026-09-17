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

import { act, renderWithProviders, screen, waitFor } from '@/test/utils';
import { getSampleRestApiDefinition } from '@/api/resources/restApis/restApis.endpoints';
import { NO_SAMPLE_DEFINITION, sampleDefinitionIdFor } from '@/api/resources/restApis/mocks';
import { firstOperationOf } from './utils/operationRequest';
import { resetTestApiKeys } from './utils/useTestApiKey';
import { TEST_KEY_TTL_HOURS } from './utils/testApiKey';
import { TestPage } from './TestPage';

/**
 * Page-level tests for the test console.
 *
 * The console view's viewer is mocked: mounting real swagger-ui in jsdom is
 * slow and it is covered on its own in `console/TestConsoleSpecViewer.test.tsx`.
 * What these tests are for is the page's own wiring — and specifically the two
 * failures reported against the first version, both of which came from the page
 * correcting a request from an effect while the console reported it back:
 *
 * - the cURL view sat on a loading spinner indefinitely, and
 * - the render loop stopped clicks landing, so operations would not expand.
 *
 * A loop surfaces here as React's "Maximum update depth exceeded", so any test
 * that renders the page to completion is a guard against it. `console.error` is
 * failed on explicitly, because React reports that as an error rather than a
 * thrown exception and the render would otherwise appear to succeed.
 */

vi.mock('./console/TestConsoleSpecViewer', () => ({
  default: ({ baseUrl }: { baseUrl: string }) => <div data-testid="spec-viewer">{baseUrl}</div>,
}));

const API = aRestApi({ context: '/payments', id: 'payments-api' });

/** The same API, secured by `api-key-auth` with the given params. */
const securedApi = (params?: Record<string, unknown>) =>
  aRestApi({
    context: '/payments',
    id: 'payments-api',
    policies: [{ name: 'api-key-auth', version: 'v1', ...(params ? { params } : {}) }],
  } as Parameters<typeof aRestApi>[0]);

/** Counts key-mint calls, so "never minted" can be asserted rather than assumed. */
let mintCalls = 0;

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
const GATEWAY = aGateway({
  displayName: 'Default gateway',
  endpoints: ['https://gw.example.com'],
  id: 'default-gateway',
});

/** Every request the page makes on mount. */
const happyPath = () => [
  http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(API)),
  http.get(apiUrl('/rest-apis/payments-api/gateways'), () =>
    HttpResponse.json(
      listEnvelope([{ ...GATEWAY, associatedAt: '2026-01-01T00:00:00Z', isDeployed: true }]),
    ),
  ),
  http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
    HttpResponse.json(listEnvelope([aDeployment({ gatewayId: 'default-gateway' })])),
  ),
  http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
    HttpResponse.json({
      apiKey: 'live-test-credential',
      keyId: 'k1',
      message: 'ok',
      status: 'success',
    }),
  ),
];

/** Every request the page makes when the API is deployed nowhere. */
const undeployed = () => [
  http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(API)),
  http.get(apiUrl('/rest-apis/payments-api/gateways'), () => HttpResponse.json(listEnvelope([]))),
  http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
    HttpResponse.json(listEnvelope([])),
  ),
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

describe('TestPage — the api-key-auth gate', () => {
  it('shows no test key panel, and mints no key, for an unsecured API', async () => {
    server.use(...pathFor(API));

    renderPage();

    await screen.findByTestId('spec-viewer');

    // Without the policy the gateway accepts unauthenticated calls, so a key
    // would change nothing about whether a request succeeds — and minting one
    // anyway would leave a real, persisted API key nothing asked for.
    expect(screen.queryByText(/Test key/i)).not.toBeInTheDocument();
    await waitFor(() => expect(mintCalls).toBe(0));
    expectNoRenderLoop();
  });

  it('attaches no credential to the curl command for an unsecured API', async () => {
    server.use(...pathFor(API));

    const { user } = renderPage();

    await screen.findByTestId('spec-viewer');
    await user.click(screen.getByRole('button', { name: /cURL view/i }));

    await waitFor(() => expect(curlText()).toContain('curl -X'));
    expect(curlText()).not.toMatch(/API-Key|X-API-Key|live-test-credential/i);
    expectNoRenderLoop();
  });

  it('shows the test key panel and mints a key for a secured API', async () => {
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    renderPage();

    expect(await screen.findByText(/Test key/i)).toBeInTheDocument();
    await waitFor(() => expect(mintCalls).toBe(1));
    expectNoRenderLoop();
  });

  it('sends the credential in the header the policy names', async () => {
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    const { user } = renderPage();

    await screen.findByTestId('spec-viewer');
    await user.click(screen.getByRole('button', { name: /cURL view/i }));

    await waitFor(() => expect(curlText()).toContain('X-API-Key'));
    // Masked until revealed, even though it is a real credential.
    expect(curlText()).toContain('-H');
    expect(curlText()).not.toContain('live-test-credential');
    expectNoRenderLoop();
  });

  it('stops attaching the credential once the key expires', async () => {
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    // Installed before rendering, because the page's countdown timer is created
    // during render: a clock swapped in afterwards would not own it.
    // `shouldAdvanceTime` keeps msw and react-query progressing on real time.
    vi.useFakeTimers({ shouldAdvanceTime: true });

    try {
      const { user } = renderPage();

      await screen.findByTestId('spec-viewer');
      await user.click(screen.getByRole('button', { name: /cURL view/i }));
      await waitFor(() => expect(curlText()).toContain('X-API-Key'));

      // A test key lasts an hour. Nothing re-renders this page because time
      // passed, so without a clock of its own the page would go on injecting a
      // credential the gateway has already stopped honouring.
      vi.setSystemTime(new Date(Date.now() + (TEST_KEY_TTL_HOURS * 60 + 1) * 60 * 1000));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });

      await waitFor(() => expect(curlText()).not.toContain('X-API-Key'));
    } finally {
      vi.useRealTimers();
    }
    expectNoRenderLoop();
  });

  it('falls back to the gateway default header when the policy has no params', async () => {
    server.use(...pathFor(securedApi()));

    const { user } = renderPage();

    await screen.findByTestId('spec-viewer');
    await user.click(screen.getByRole('button', { name: /cURL view/i }));

    await waitFor(() => expect(curlText()).toContain('API-Key'));
    expectNoRenderLoop();
  });

  it('places the credential in the query string when the policy says query', async () => {
    server.use(...pathFor(securedApi({ in: 'query', key: 'apiKey' })));

    const { user } = renderPage();

    await screen.findByTestId('spec-viewer');
    await user.click(screen.getByRole('button', { name: /cURL view/i }));

    // In the URL, not as a -H line — and masked there too, which an earlier
    // buildRequestUrl would not have done.
    await waitFor(() => expect(curlText()).toContain('apiKey='));
    expect(curlText()).not.toContain("-H 'apiKey");
    expect(curlText()).not.toContain('live-test-credential');
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
});

describe('TestPage', () => {
  it('renders the console view against the resolved gateway endpoint', async () => {
    server.use(...happyPath());

    renderPage();

    // The endpoint is the gateway URL joined with the API context — proof the
    // viewer received a resolved base URL rather than the mount-time empty one.
    expect(await screen.findByTestId('spec-viewer')).toHaveTextContent(
      'https://gw.example.com/payments',
    );
    expectNoRenderLoop();
  });

  it('shows a curl command in the cURL view rather than a spinner', async () => {
    server.use(...happyPath());

    const { user } = renderPage();

    await screen.findByTestId('spec-viewer');
    await user.click(screen.getByRole('button', { name: /cURL view/i }));

    // The reported bug: the builder was gated on a request that an effect
    // never got around to setting, so this view showed "Loading test console"
    // forever.
    await waitFor(() => expect(curlText()).toContain('curl -X'));
    expect(screen.queryByText(/Loading test console/i)).not.toBeInTheDocument();
    expectNoRenderLoop();
  });

  it('seeds the builder from the first operation of the definition', async () => {
    server.use(...happyPath());

    // Derived rather than hardcoded: which bundled sample an API gets is a
    // deterministic function of its id, so asserting a specific verb here
    // would pin the test to that mapping instead of to the behaviour.
    const choice = sampleDefinitionIdFor('payments-api');
    // Some APIs are selected to have no definition at all. This test is about
    // seeding from one that does, so an unusable fixture fails loudly here
    // rather than through a confusing assertion further down.
    if (choice === NO_SAMPLE_DEFINITION) {
      throw new Error('fixture API must select a sample with a definition');
    }

    const definition = await getSampleRestApiDefinition(choice);
    const first = firstOperationOf(definition.spec);

    const { user } = renderPage();

    await screen.findByTestId('spec-viewer');
    await user.click(screen.getByRole('button', { name: /cURL view/i }));

    await waitFor(() => expect(curlText()).toContain(`curl -X ${first!.method}`));
    expect(curlText()).toContain(`https://gw.example.com/payments${first!.path}`);
    expectNoRenderLoop();
  });

  it('renders nothing but the empty state when the API is deployed nowhere', async () => {
    server.use(...undeployed());

    renderPage();

    expect(
      await screen.findByText(/You must deploy the API to start testing/i),
    ).toBeInTheDocument();

    // "Only the banner": with no gateway there is no endpoint, no key worth
    // issuing and no request to build, so every one of these controls would be
    // inert if it rendered.
    expect(screen.queryByTestId('spec-viewer')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /cURL view/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Console view/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Test key/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Endpoint/i)).not.toBeInTheDocument();
    expect(curlText()).toBe('');
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

  it('does not show the empty state for a deployed API', async () => {
    server.use(...happyPath());

    renderPage();

    await screen.findByTestId('spec-viewer');
    expect(screen.queryByText(/You must deploy the API to start testing/i)).not.toBeInTheDocument();
  });

  it('shows neither console nor empty state while deployment is unknown', async () => {
    // An unresolved gateway query reports zero gateways exactly like a
    // genuinely undeployed API. The page holds its loading state until it
    // knows, so the empty state cannot appear on the strength of a pending
    // request — and the console cannot flash before being replaced by it.
    server.use(
      http.get(apiUrl('/rest-apis/payments-api'), () => HttpResponse.json(API)),
      neverResponds('get', '/rest-apis/payments-api/gateways'),
      http.get(apiUrl('/rest-apis/payments-api/deployments'), () =>
        HttpResponse.json(listEnvelope([aDeployment({ gatewayId: 'default-gateway' })])),
      ),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () =>
        HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' }),
      ),
    );

    renderPage();

    expect(await screen.findByText(/Loading test console/i)).toBeInTheDocument();
    expect(screen.queryByText(/You must deploy the API to start testing/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId('spec-viewer')).not.toBeInTheDocument();
  });

  it('surfaces the minted test key and the header it travels in', async () => {
    // Secured explicitly: the panel only exists for an API whose api-key-auth
    // policy demands a key, and the header name now comes from that policy.
    server.use(...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })));

    renderPage();

    // The header name is its own <code> element inside the sentence, so the
    // sentence and the name are asserted separately rather than as one string.
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

  it('keeps the console usable when the test key cannot be minted', async () => {
    // Secured deliberately: an unsecured API never attempts a mint, so this
    // would assert nothing about the failure path it is named for.
    server.use(
      ...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })).slice(0, 3),
      http.post(apiUrl('/rest-apis/payments-api/api-keys'), () => {
        mintCalls += 1;
        return HttpResponse.json({ code: '500', message: 'nope' }, { status: 500 });
      }),
    );

    renderPage();

    // A failed key must not take the page down with it: the contract is still
    // readable and a request can still be built, unauthenticated.
    expect(await screen.findByTestId('spec-viewer')).toBeInTheDocument();
    await waitFor(() => expect(mintCalls).toBe(1));
    expectNoRenderLoop();
  });

  it('reports a failed mint rather than masking it', async () => {
    server.use(
      ...pathFor(securedApi({ in: 'header', key: 'X-API-Key' })).slice(0, 3),
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
