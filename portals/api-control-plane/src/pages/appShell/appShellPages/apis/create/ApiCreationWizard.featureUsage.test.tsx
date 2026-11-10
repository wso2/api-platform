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

import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig } from '@/config/runtime';
import { accepts, collection, failure, recorder, type Recorder } from '@/test/msw';
import { makeAuthState } from '@/test/mockAuthState';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ApiCreationWizard } from './ApiCreationWizard';
import { apiCreationEntry } from './entryPoint';

/*
 * The REST "start from scratch" funnel as it reaches Moesif, through the real
 * wizard screens. Unlike ApiCreationWizard.test.tsx the type and source steps
 * are not stubbed: "Start from scratch" being the default tab, which the user
 * never clicks, is exactly what has to be counted.
 */

/** Stands in for moesif-browser-js, the boundary the console publishes through. */
const sdk = vi.hoisted(() => {
  const client = {
    // The SDK's network write, which the PII filter wraps; without it
    // tracking stays off rather than sending unfiltered data.
    _executeRequest: vi.fn(),
    identifyCompany: vi.fn(),
    identifyUser: vi.fn(),
    init: vi.fn(),
    reset: vi.fn(),
    track: vi.fn(),
  };
  client.init.mockImplementation(() => client);
  return client;
});
vi.mock('moesif-browser-js', () => ({ default: sdk }));
// The contract preview's swagger-ui bundles its own React; it isn't what is measured here.
vi.mock('@/components/SwaggerSpecViewer', () => ({ default: () => null }));

const scope = makeConsoleScope();
const path = '/organizations/api-platform-demo/projects/retail-apis/apis/new';

const signedInAs = (userId: string, orgId: string) =>
  makeAuthState({
    user: {
      email: 'alice@example.com',
      id: userId,
      name: 'Alice',
      org: { handle: 'api-platform-demo', id: orgId, name: 'API Platform Demo' },
    },
  });
const authState = signedInAs(
  '0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b',
  'b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f',
);

let createRequests: Recorder;

beforeAll(() => {
  // Read when the first action is tracked, as a deployment's runtime config is.
  Object.assign(runtimeConfig, {
    featureUsageTrackingEnabled: true,
    moesifPublishableKey: 'publishable-key',
  });
});

beforeEach(() => {
  resetHttpClient();
  vi.clearAllMocks();
  createRequests = recorder();
  server.use(collection('/rest-apis', []), collection('/graphql-apis', []));
});

/** Moesif actions sent so far, as [action, feature dimensions] pairs. */
const tracked = () =>
  sdk.track.mock.calls.map((call) => {
    const [action, metadata] = call as [string, Record<string, unknown>];
    const dimensions = Object.fromEntries(
      Object.entries(metadata).filter(
        ([key]) =>
          ![
            'action',
            'category',
            'tier',
            'event_category',
            'initiating_product',
            'platform',
          ].includes(key),
      ),
    );
    return [action, dimensions];
  });

const createFromScratch = async (options: Parameters<typeof renderWithProviders>[1]) => {
  const { user } = renderWithProviders(<ApiCreationWizard />, { authState, scope, ...options });

  // Step 1: REST is preselected.
  await user.click(screen.getByRole('button', { name: 'Continue' }));
  // Step 2: "Start from scratch" is the default tab; only the endpoint is typed.
  await user.type(screen.getByLabelText('Endpoint URL'), 'https://orders.example.com');
  await user.click(screen.getByRole('button', { name: 'Continue' }));
  // Step 3: the form arrives filled in from the skeleton.
  await user.click(screen.getByRole('button', { name: 'Create' }));
};

describe('ApiCreationWizard — feature usage', () => {
  it('reports each screen of the REST from-scratch funnel, in order', async () => {
    server.use(
      accepts(
        'post',
        '/rest-apis/import-openapi',
        { id: 'untitled-api' },
        { record: createRequests },
      ),
    );

    await createFromScratch({
      routerEntries: [{ pathname: path, ...apiCreationEntry('api-list') }],
    });

    await waitFor(() => expect(sdk.track).toHaveBeenCalledTimes(4));
    expect(tracked()).toEqual([
      ['apip.manage-apis.api-creation-wizard-open', { entry_point: 'api-list' }],
      ['apip.manage-apis.api-creation-wizard-api-type-select', { api_type: 'rest-api' }],
      [
        'apip.manage-apis.api-creation-wizard-creation-method-select',
        { api_type: 'rest-api', creation_method: 'scratch' },
      ],
      ['apip.manage-apis.rest-api-create-click', { creation_method: 'scratch' }],
    ]);
    expect(createRequests.count()).toBe(1);
  });

  it('identifies the signed-in user and organization by ID only, and sends nothing identifying', async () => {
    server.use(accepts('post', '/rest-apis/import-openapi', { id: 'untitled-api' }));
    const userId = '5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f';
    const orgId = '9a3f4b2c-1d2e-4f5a-8b6c-7d8e9f0a1b2c';

    await createFromScratch({ authState: signedInAs(userId, orgId), route: path });

    await waitFor(() => expect(sdk.track).toHaveBeenCalledTimes(4));
    expect(sdk.identifyCompany).toHaveBeenCalledWith(orgId);
    expect(sdk.identifyUser).toHaveBeenCalledWith(userId);
    // Names, emails, handles and typed values never reach Moesif.
    const sent = JSON.stringify([
      sdk.track.mock.calls,
      sdk.identifyUser.mock.calls,
      sdk.identifyCompany.mock.calls,
    ]);
    for (const forbidden of [
      'api-platform-demo',
      'retail-apis',
      'Untitled',
      'orders.example.com',
      'alice',
      'Alice',
    ]) {
      expect(sent).not.toContain(forbidden);
    }
  });

  it('reports a typed or bookmarked URL as a direct entry', async () => {
    renderWithProviders(<ApiCreationWizard />, { authState, route: path, scope });

    await waitFor(() =>
      expect(tracked()).toEqual([
        ['apip.manage-apis.api-creation-wizard-open', { entry_point: 'direct' }],
      ]),
    );
  });

  it('counts the Create click even when the server then rejects the API', async () => {
    server.use(
      failure('post', '/rest-apis/import-openapi', 409, 'CONFLICT', {
        message: 'Identifier is taken',
      }),
    );

    await createFromScratch({ route: path });

    await waitFor(() =>
      expect(tracked().map(([action]) => action)).toContain('apip.manage-apis.rest-api-create-click'),
    );
  });
});

describe('ApiCreationWizard — feature usage, other paths', () => {
  const ORDERS_SPEC = [
    'openapi: 3.0.0',
    'info:',
    '  title: Orders',
    "  version: '1.0'",
    'servers:',
    '  - url: https://orders.example.com',
    'paths:',
    '  /orders:',
    '    get:',
    '      responses:',
    "        '200':",
    '          description: ok',
  ].join('\n');

  it('reports a REST API imported from a contract URL, with its source', async () => {
    server.use(
      accepts('post', '/rest-apis/validate-openapi', {
        content: ORDERS_SPEC,
        errors: [],
        isValid: true,
      }),
      accepts('post', '/rest-apis/import-openapi', { id: 'orders' }),
    );
    const { user } = renderWithProviders(<ApiCreationWizard />, { authState, route: path, scope });

    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(screen.getByRole('button', { name: /Start with a contract/ }));
    await user.type(
      screen.getByLabelText(/URL for API Contract/),
      'https://example.com/orders.yaml',
    );
    await user.tab();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Continue' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(await screen.findByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(tracked().map(([action]) => action)).toContain('apip.manage-apis.rest-api-create-click'),
    );
    expect(tracked().slice(2)).toEqual([
      [
        'apip.manage-apis.api-creation-wizard-creation-method-select',
        { api_type: 'rest-api', creation_method: 'contract' },
      ],
      ['apip.manage-apis.rest-api-create-click', { contract_source: 'url', creation_method: 'contract' }],
    ]);
    // Nothing the user supplied rides along: not the URL, not the title.
    expect(JSON.stringify(sdk.track.mock.calls)).not.toMatch(/example\.com|Orders/);
  });

  it('reports a GraphQL API created from scratch, in the same terms as REST', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', { resolved: false }),
      accepts('post', '/graphql-apis', {
        context: '/backend/v1.0.0',
        displayName: 'Backend',
        id: 'backend',
        upstream: { main: { url: 'https://backend.example.com/graphql' } },
      }),
    );
    const { user } = renderWithProviders(<ApiCreationWizard />, { authState, route: path, scope });

    await user.click(screen.getByRole('button', { name: /GraphQL API/ }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(screen.getByRole('button', { name: /Start from scratch/ }));
    await user.type(screen.getByLabelText(/Endpoint URL/), 'https://backend.example.com/graphql');
    await screen.findByText(/introspection may be disabled/);
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(await screen.findByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(tracked().map(([action]) => action)).toContain('apip.manage-apis.graphql-api-create-click'),
    );
    expect(tracked().slice(1)).toEqual([
      ['apip.manage-apis.api-creation-wizard-api-type-select', { api_type: 'graphql-api' }],
      [
        'apip.manage-apis.api-creation-wizard-creation-method-select',
        { api_type: 'graphql-api', creation_method: 'scratch' },
      ],
      ['apip.manage-apis.graphql-api-create-click', { creation_method: 'scratch' }],
    ]);
  });
});
