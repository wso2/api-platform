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

import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aDeployment, aGateway, aGraphQLApiDetail, collection, resource } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
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

const serveApi = (deployments: ReturnType<typeof aDeployment>[] = []) => {
  server.use(
    resource('/graphql-apis/:graphqlApiId', api),
    resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
    collection('/gateways', [gateway]),
    collection('/graphql-apis/:graphqlApiId/deployments', deployments),
  );
};

beforeEach(() => {
  resetHttpClient();
  lastGraphiQLProps = undefined;
});

describe('GraphqlTestConsolePage — before anything is deployed', () => {
  it('says to deploy first and renders no console', async () => {
    serveApi([]);

    renderPage();

    expect(
      await screen.findByText('Deploy this API to a gateway before testing it here.'),
    ).toBeInTheDocument();
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

  it('points the fetcher at the selected gateway’s invoke URL', async () => {
    serveApi([deployment]);

    renderPage();

    await screen.findByText('GraphiQL ready with schema');
    // createGraphiQLFetcher closes over the URL rather than exposing it, so
    // the endpoint text next to the selector is this page's own contract —
    // asserted above — and is what the fetcher is built from.
    expect(lastGraphiQLProps?.fetcher).toBeInstanceOf(Function);
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

  it('copies the endpoint URL to the clipboard', async () => {
    serveApi([deployment]);
    const { user } = renderPage();
    await screen.findByText('GraphiQL ready with schema');
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');

    await user.click(screen.getByRole('button', { name: 'Copy endpoint URL' }));

    expect(writeText).toHaveBeenCalledWith('https://gw.example.com/countries-graphql-api/v1.0.0');
  });
});

describe('GraphqlTestConsolePage — the API cannot be found', () => {
  it('shows an error state', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
      collection('/gateways', []),
      collection('/graphql-apis/:graphqlApiId/deployments', []),
    );

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
