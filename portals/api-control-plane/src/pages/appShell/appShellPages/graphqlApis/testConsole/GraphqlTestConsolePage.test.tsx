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

import { act } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aDeployment, aGateway, aGraphQLApiDetail, apiUrl, collection, resource } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { resetTestApiKeys } from '../../test/utils/useTestApiKey';
import { GraphqlTestConsolePage } from './GraphqlTestConsolePage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const SAMPLE_SDL = 'type Query { country(code: ID!): String }';

const api = aGraphQLApiDetail({
  context: '/countries-graphql-api/v1.0.0',
  displayName: 'Countries GraphQL API',
  id: API,
  projectId: PROJECT,
});

const gateway = aGateway({
  displayName: 'Edge Gateway',
  endpoints: ['https://gw.example.com'],
  id: 'edge-gateway',
  isActive: true,
});

// GraphiQL is a real, Monaco-backed third-party component this page doesn't
// own — it needs a canvas and web workers jsdom can't provide, so it's
// replaced with a stub that surfaces the props this page is responsible for
// wiring correctly (fetcher target, parsed schema), the same way
// `DefineApiPanel.test.tsx` stubs the wizard's own Monaco-backed editor.
let lastGraphiQLProps:
  | { defaultQuery?: string; fetcher?: unknown; schema?: unknown; shouldPersistHeaders?: boolean }
  | undefined;
vi.mock('graphiql', () => ({
  GraphiQL: (props: {
    defaultQuery?: string;
    fetcher?: unknown;
    schema?: unknown;
    shouldPersistHeaders?: boolean;
  }) => {
    lastGraphiQLProps = props;
    return <div>{props.schema ? 'GraphiQL ready with schema' : 'GraphiQL ready, no schema'}</div>;
  },
}));
vi.mock('graphiql/setup-workers/vite', () => ({}));
vi.mock('graphiql/style.css', () => ({}));

// createGraphiQLFetcher closes over its options with no getter, so the only
// way to assert what URL/headers this page builds it with is to intercept the
// call itself — everything else about the real toolkit stays intact.
let lastFetcherOptions: import('@graphiql/toolkit').CreateFetcherOptions | undefined;
vi.mock('@graphiql/toolkit', async () => {
  const actual = await vi.importActual<typeof import('@graphiql/toolkit')>('@graphiql/toolkit');
  return {
    ...actual,
    createGraphiQLFetcher: (options: import('@graphiql/toolkit').CreateFetcherOptions) => {
      lastFetcherOptions = options;
      return actual.createGraphiQLFetcher(options);
    },
  };
});

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route
          element={<GraphqlTestConsolePage />}
          path="/organizations/:orgHandle/projects/:projectHandler/graphql-apis/:graphqlApiHandler/test/console"
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/graphql-apis/${API}/test/console`,
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

const serveApi = (
  deployments: ReturnType<typeof aDeployment>[] = [],
  served: ReturnType<typeof aGraphQLApiDetail> = api,
) => {
  server.use(
    resource('/graphql-apis/:graphqlApiId', served),
    resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
    collection('/graphql-apis/:graphqlApiId/gateways', [gateway]),
    collection('/graphql-apis/:graphqlApiId/deployments', deployments),
  );
};

beforeEach(() => {
  resetHttpClient();
  lastGraphiQLProps = undefined;
  lastFetcherOptions = undefined;
  resetTestApiKeys();
});

describe('GraphqlTestConsolePage — before anything is deployed', () => {
  it('says to deploy first, offers the Deploy page, and renders no console', async () => {
    serveApi([]);

    renderPage();

    expect(
      await screen.findByText('You must deploy the API to start testing.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deploy API' })).toBeInTheDocument();
    expect(screen.queryByText(/GraphiQL ready/)).not.toBeInTheDocument();
  });
});

describe('GraphqlTestConsolePage — once deployed', () => {
  const deployment = aDeployment({ gatewayId: 'edge-gateway', status: 'DEPLOYED' });

  it('shows the gateway’s invoke URL and renders the console with the parsed schema', async () => {
    serveApi([deployment]);

    renderPage();

    expect(await screen.findByText('Edge Gateway')).toBeInTheDocument();
    const endpointField = screen.getByDisplayValue(
      'https://gw.example.com/countries-graphql-api/v1.0.0',
    );
    expect(endpointField).toBeInTheDocument();
    expect(endpointField).toHaveAttribute('readonly');
    expect(await screen.findByText('GraphiQL ready with schema')).toBeInTheDocument();
    expect(lastGraphiQLProps?.schema).toBeDefined();
  });

  // GraphiQL derives a tab's title from the query's own operation name —
  // there is no separate "tab title" prop — so the starter query passed here
  // must contain a named operation, or the first tab reads GraphiQL's own
  // default "<untitled>" instead of "Schema". GraphiQL's own welcome comment
  // is kept ahead of it (see DEFAULT_QUERY's comment), so this only asserts
  // the named operation is present, not that the string starts with it.
  it('starts the editor with a named "Schema" operation, not an anonymous one', async () => {
    serveApi([deployment]);

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    expect(lastGraphiQLProps?.defaultQuery).toMatch(/query Schema\b/);
  });

  it('keeps GraphiQL’s own welcome/example comment ahead of the starter query', async () => {
    serveApi([deployment]);

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    expect(lastGraphiQLProps?.defaultQuery).toContain('# Welcome to GraphiQL');
  });

  // The console calls the gateway directly — the API's CORS policy must allow
  // it — so the fetcher targets the same invoke URL the page shows, with no
  // console-internal headers such as the BFF's CSRF header.
  it('points the fetcher at the gateway’s invoke URL', async () => {
    serveApi([deployment]);

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    expect(lastGraphiQLProps?.fetcher).toBeInstanceOf(Function);
    expect(lastFetcherOptions?.url).toBe('https://gw.example.com/countries-graphql-api/v1.0.0');
    expect(lastFetcherOptions?.headers).toEqual({});
  });

  // GraphiQL only strips the Headers tab from what it writes to `storage`
  // (real localStorage here) when shouldPersistHeaders is falsy — pins the
  // fix for headers (including any Authorization value) being persisted to
  // disk in cleartext.
  it('does not opt in to persisting the Headers tab to storage', async () => {
    serveApi([deployment]);

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    expect(lastGraphiQLProps?.shouldPersistHeaders).toBeFalsy();
  });

  // Regression test: a network-level fetch failure (a CORS rejection, an
  // untrusted certificate or an unreachable gateway) used to surface as nothing but GraphiQL's own cryptic
  // "Failed to fetch" — no guidance anywhere in this page. The fetcher this
  // page builds now catches that specific failure class and shows an
  // actionable toast, while still rethrowing so GraphiQL's own panel keeps
  // reflecting the failure too.
  it('shows a helpful toast when the endpoint fetch fails at the network level', async () => {
    serveApi([deployment]);
    const originalFetch = global.fetch;
    global.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

    try {
      renderPage();
      await screen.findByText('GraphiQL ready with schema');

      // createGraphiQLFetcher's incremental-delivery path (the default) is an
      // async GENERATOR function: calling it only returns the generator
      // object — the body, including the actual httpFetch() call, doesn't
      // run until something iterates it (which is what the real, unmocked
      // GraphiQL does when it consumes the result). So this drives that same
      // iteration protocol directly, the way GraphiQL itself would, rather
      // than asserting the top-level call's own return value rejects.
      const fetcher = lastGraphiQLProps?.fetcher as (params: {
        query: string;
      }) => Promise<AsyncIterator<unknown> | unknown>;
      await act(async () => {
        const result = await fetcher({ query: '{ __typename }' });
        if (result && typeof (result as AsyncIterator<unknown>).next === 'function') {
          await (result as AsyncIterator<unknown>).next().catch(() => undefined);
        }
      });

      expect(await screen.findByText(/Could not reach the gateway/)).toBeInTheDocument();
    } finally {
      global.fetch = originalFetch;
    }
  });

  // Regression test: GraphiQL keeps the starter tab's "Schema" operation name
  // after the query is replaced with an anonymous one, and the gateway's
  // graphql-authz policy rejected every such request with "Unknown operation
  // named". The fetcher this page wires must drop the stale name.
  it('does not send a stale operationName for an anonymous query', async () => {
    serveApi([deployment]);
    const originalFetch = global.fetch;
    const fetchSpy = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { hello: 'hi' } }), {
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    global.fetch = fetchSpy;

    try {
      renderPage();
      await screen.findByText('GraphiQL ready with schema');

      // Same iteration protocol as the toast test above: the toolkit's
      // fetcher may return an async iterator whose body only runs when
      // consumed.
      const fetcher = lastGraphiQLProps?.fetcher as (params: {
        query: string;
        operationName?: string;
      }) => Promise<AsyncIterator<unknown> | unknown>;
      await act(async () => {
        const result = await fetcher({ query: '{ hello }', operationName: 'Schema' });
        if (result && typeof (result as AsyncIterator<unknown>).next === 'function') {
          await (result as AsyncIterator<unknown>).next().catch(() => undefined);
        }
      });

      expect(fetchSpy).toHaveBeenCalled();
      const body = JSON.parse(String(fetchSpy.mock.calls[0][1]?.body));
      expect(body.query).toBe('{ hello }');
      expect(body.operationName).toBeUndefined();
    } finally {
      global.fetch = originalFetch;
    }
  });

  it('copies the endpoint URL to the clipboard', async () => {
    serveApi([deployment]);
    const { user } = renderPage();
    await screen.findByText('GraphiQL ready with schema');
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');

    await user.click(screen.getByRole('button', { name: 'Copy endpoint URL' }));

    expect(writeText).toHaveBeenCalledWith('https://gw.example.com/countries-graphql-api/v1.0.0');
  });
});

describe('GraphqlTestConsolePage — with an api-key-auth policy', () => {
  const deployment = aDeployment({ gatewayId: 'edge-gateway', status: 'DEPLOYED' });
  const securedApi = (params?: Record<string, string>) =>
    aGraphQLApiDetail({
      ...api,
      policies: [{ name: 'api-key-auth', version: 'v1', ...(params ? { params } : {}) }],
    });
  const serveKeyMint = () => {
    let mints = 0;
    server.use(
      http.post(apiUrl('/graphql-apis/:graphqlApiId/api-keys'), () => {
        mints += 1;
        return HttpResponse.json({
          apiKey: 'graphql-test-credential',
          keyId: 'k1',
          message: 'ok',
          status: 'success',
        });
      }),
    );
    return () => mints;
  };

  it('mints a test key and sends it as the policy’s header', async () => {
    serveApi([deployment], securedApi({ in: 'header', key: 'X-Api-Key' }));
    const mints = serveKeyMint();

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    await vi.waitFor(() =>
      expect(lastFetcherOptions?.headers).toEqual({ 'X-Api-Key': 'graphql-test-credential' }),
    );
    expect(lastFetcherOptions?.url).toBe('https://gw.example.com/countries-graphql-api/v1.0.0');
    expect(mints()).toBe(1);
  });

  it('sends the key as a query parameter when the policy says so', async () => {
    serveApi([deployment], securedApi({ in: 'query', key: 'apikey' }));
    serveKeyMint();

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    await vi.waitFor(() =>
      expect(lastFetcherOptions?.url).toBe(
        'https://gw.example.com/countries-graphql-api/v1.0.0?apikey=graphql-test-credential',
      ),
    );
    expect(lastFetcherOptions?.headers).not.toHaveProperty('apikey');
  });
});

describe('GraphqlTestConsolePage — without an api-key-auth policy', () => {
  it('mints no key and titles the page after the API', async () => {
    serveApi([aDeployment({ gatewayId: 'edge-gateway', status: 'DEPLOYED' })]);
    let mints = 0;
    server.use(
      http.post(apiUrl('/graphql-apis/:graphqlApiId/api-keys'), () => {
        mints += 1;
        return HttpResponse.json({ apiKey: 'k', keyId: 'k1', message: 'ok', status: 'success' });
      }),
    );

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    expect(
      screen.getByRole('heading', { name: /Test\s+Countries GraphQL API/ }),
    ).toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(mints).toBe(0);
  });
});

describe('GraphqlTestConsolePage — the stored schema', () => {
  const deployment = aDeployment({ gatewayId: 'edge-gateway', status: 'DEPLOYED' });

  it('still opens the console when the API has no stored schema', async () => {
    serveApi([deployment]);
    server.use(
      resource(
        '/graphql-apis/:graphqlApiId/sdl',
        { code: 'NOT_FOUND', message: 'not found', status: 'error' },
        { status: 404 },
      ),
    );

    renderPage();

    expect(await screen.findByText('GraphiQL ready, no schema')).toBeInTheDocument();
  });

  it('shows an error state when the schema fails to load', async () => {
    serveApi([deployment]);
    server.use(
      resource(
        '/graphql-apis/:graphqlApiId/sdl',
        { code: 'INTERNAL_ERROR', message: 'boom', status: 'error' },
        { status: 500 },
      ),
    );

    renderPage();

    expect(
      await screen.findByText('This API’s definition could not be loaded.'),
    ).toBeInTheDocument();
    expect(screen.queryByText(/GraphiQL ready/)).not.toBeInTheDocument();
  });
});

describe('GraphqlTestConsolePage — the API cannot be found', () => {
  it('shows an error state', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/graphql-apis/:graphqlApiId/gateways', []),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
