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

import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { makeAuthState } from '@/test/mockAuthState';
import { aUserApiKey, collection, noContent, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import type { ApiKeyApiKind } from './apiKeyKinds';
import { ApiKeysPanel } from './ApiKeysPanel';

const ORG = 'api-platform-demo';
const REST_API = 'pizza-shack';
const GRAPHQL_API = 'countries-graphql-api';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const renderPanel = (
  apiId: string,
  apiKind?: ApiKeyApiKind,
  options: Parameters<typeof renderWithProviders>[1] = {},
) =>
  renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <ApiKeysPanel apiId={apiId} apiKind={apiKind} />
    </ApiScopeProvider>,
    options,
  );

/** The `aria-label` sits on the `Tooltip`'s wrapping `<span>`, not the
 * `IconButton` itself, so the button is found within that labelled span. */
const revokeButton = () => within(screen.getByLabelText('Revoke API key')).getByRole('button');

describe('ApiKeysPanel — reading the shared key list', () => {
  it('narrows the caller’s key inventory to the kind of API it is showing', async () => {
    const reads = recorder();
    server.use(collection('/me/api-keys', [], { record: reads }));

    renderPanel(GRAPHQL_API, 'graphql');

    await waitFor(() => expect(reads.count()).toBe(1));
    expect(reads.last()?.url.searchParams.getAll('type')).toEqual(['GraphQLApi']);
  });

  it('defaults to REST API keys when no kind is given', async () => {
    const reads = recorder();
    server.use(collection('/me/api-keys', [], { record: reads }));

    renderPanel(REST_API);

    await waitFor(() => expect(reads.count()).toBe(1));
    expect(reads.last()?.url.searchParams.getAll('type')).toEqual(['RestApi']);
  });

  it('shows only this API’s own active keys', async () => {
    server.use(
      collection('/me/api-keys', [
        aUserApiKey({
          artifactId: GRAPHQL_API,
          artifactType: 'GraphQLApi',
          displayName: 'Prod Key',
        }),
        // This API's own key, but revoked — must not render as active.
        aUserApiKey({
          artifactId: GRAPHQL_API,
          artifactType: 'GraphQLApi',
          displayName: 'Revoked Key',
          status: 'revoked',
        }),
        // A different GraphQL API's key.
        aUserApiKey({
          artifactId: 'other-api',
          artifactType: 'GraphQLApi',
          displayName: 'Other Key',
        }),
      ]),
    );

    renderPanel(GRAPHQL_API, 'graphql');

    expect(await screen.findByText('Prod Key')).toBeInTheDocument();
    expect(screen.queryByText('Revoked Key')).not.toBeInTheDocument();
    expect(screen.queryByText('Other Key')).not.toBeInTheDocument();
  });
});

describe('ApiKeysPanel — revoking a key', () => {
  it.each([
    { apiId: REST_API, kind: 'rest' as const, endpoint: `/rest-apis/${REST_API}/api-keys/key-1` },
    {
      apiId: GRAPHQL_API,
      kind: 'graphql' as const,
      endpoint: `/graphql-apis/${GRAPHQL_API}/api-keys/key-1`,
    },
  ])('revokes a $kind API’s key at its own endpoint', async ({ apiId, kind, endpoint }) => {
    server.use(
      collection('/me/api-keys', [
        aUserApiKey({
          artifactId: apiId,
          artifactType: kind === 'graphql' ? 'GraphQLApi' : 'RestApi',
          displayName: 'Prod Key',
          id: 'key-1',
        }),
      ]),
      noContent('delete', '/rest-apis/:restApiId/api-keys/:apiKeyId', { record: requests }),
      noContent('delete', '/graphql-apis/:graphqlApiId/api-keys/:apiKeyId', { record: requests }),
    );

    const { user } = renderPanel(apiId, kind);
    await screen.findByText('Prod Key');

    await user.click(revokeButton());
    expect(await screen.findByText('Revoke API Key')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Revoke' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(endpoint);
  });
});

describe('ApiKeysPanel — permissions', () => {
  const enforcedAs = (scopes: string[]) => ({
    authState: makeAuthState({
      user: { email: 'test.user@example.com', name: 'Test User', scopes },
    }),
    permissionMode: 'enforce' as const,
  });

  it('gates a GraphQL API’s Add button on the GraphQL key scope, not the REST one', async () => {
    server.use(collection('/me/api-keys', []));

    renderPanel(GRAPHQL_API, 'graphql', enforcedAs(['ap:rest_api:api_key:create']));
    expect(await screen.findByRole('button', { name: 'Add' })).toBeDisabled();
  });

  it('enables a GraphQL API’s Add button for a caller holding the GraphQL key scope', async () => {
    server.use(collection('/me/api-keys', []));

    renderPanel(GRAPHQL_API, 'graphql', enforcedAs(['ap:graphql_api:api_key:create']));
    expect(await screen.findByRole('button', { name: 'Add' })).toBeEnabled();
  });
});
