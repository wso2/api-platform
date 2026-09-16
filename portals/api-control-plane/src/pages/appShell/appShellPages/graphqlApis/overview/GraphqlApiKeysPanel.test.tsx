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
import { accepts, aUserApiKey, collection, noContent, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { GraphqlApiKeysPanel } from './GraphqlApiKeysPanel';

const ORG = 'api-platform-demo';
const API = 'countries-graphql-api';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const renderPanel = () =>
  renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <GraphqlApiKeysPanel graphqlApiId={API} />
    </ApiScopeProvider>,
  );

const openAddDialog = async (user: ReturnType<typeof renderPanel>['user']) => {
  await user.click(screen.getByRole('button', { name: 'Add' }));
  const dialog = await screen.findByRole('dialog');
  return {
    dialog,
    nameField: within(dialog).getByLabelText('Key Name'),
    valueField: within(dialog).getByLabelText('API Key Value'),
  };
};

describe('GraphqlApiKeysPanel — reading the shared key list', () => {
  it('shows only this API’s own active keys', async () => {
    server.use(
      collection('/me/api-keys', [
        aUserApiKey({ artifactId: API, artifactType: 'GraphQLApi', displayName: 'Prod Key' }),
        // This API's own key, but revoked — must not render as active.
        aUserApiKey({
          artifactId: API,
          artifactType: 'GraphQLApi',
          displayName: 'Revoked Key',
          status: 'revoked',
        }),
        // A different GraphQL API's key.
        aUserApiKey({ artifactId: 'other-api', artifactType: 'GraphQLApi', displayName: 'Other Key' }),
      ]),
    );

    renderPanel();

    expect(await screen.findByText('Prod Key')).toBeInTheDocument();
    expect(screen.queryByText('Revoked Key')).not.toBeInTheDocument();
    expect(screen.queryByText('Other Key')).not.toBeInTheDocument();
  });

  it('shows nothing extra when there are no keys for this API', async () => {
    server.use(collection('/me/api-keys', []));

    renderPanel();

    expect(await screen.findByText('API Keys')).toBeInTheDocument();
    expect(screen.queryByLabelText('Revoke API key')).not.toBeInTheDocument();
  });
});

describe('GraphqlApiKeysPanel — issuing a key', () => {
  it('creates a key at the GraphQL-API-scoped endpoint, not the shared REST one', async () => {
    server.use(collection('/me/api-keys', []));
    const creates = recorder();
    server.use(
      accepts(
        'post',
        '/graphql-apis/:graphqlApiId/api-keys',
        { displayName: 'New Key', maskedApiKey: '••••1234' },
        { record: creates },
      ),
    );

    const { user } = renderPanel();
    await screen.findByText('API Keys');

    const { dialog, nameField, valueField } = await openAddDialog(user);
    await user.type(nameField, 'New Key');
    await user.type(valueField, 'secret-value');
    await user.click(within(dialog).getByRole('button', { name: 'Add' }));

    await waitFor(() => expect(creates.count()).toBe(1));
    expect(creates.last()?.url.pathname).toContain(`/graphql-apis/${API}/api-keys`);
    expect(JSON.parse(creates.last()?.body ?? '{}')).toEqual({
      displayName: 'New Key',
      apiKey: 'secret-value',
    });
  });

  it('disables the Add button until both name and value are filled in', async () => {
    server.use(collection('/me/api-keys', []));

    const { user } = renderPanel();
    await screen.findByText('API Keys');

    const { dialog, nameField, valueField } = await openAddDialog(user);
    const addInDialog = within(dialog).getByRole('button', { name: 'Add' });
    expect(addInDialog).toBeDisabled();

    await user.type(nameField, 'New Key');
    expect(addInDialog).toBeDisabled();

    await user.type(valueField, 'secret-value');
    expect(addInDialog).toBeEnabled();
  });
});

describe('GraphqlApiKeysPanel — revoking a key', () => {
  it('revokes at the GraphQL-API-scoped endpoint and removes the row', async () => {
    server.use(
      collection('/me/api-keys', [
        aUserApiKey({
          artifactId: API,
          artifactType: 'GraphQLApi',
          displayName: 'Prod Key',
          id: 'key-1',
        }),
      ]),
    );
    server.use(
      noContent('delete', '/graphql-apis/:graphqlApiId/api-keys/:apiKeyId', { record: requests }),
    );

    const { user } = renderPanel();
    await screen.findByText('Prod Key');

    // The `aria-label` sits on the `Tooltip`'s own wrapping `<span>`, not the
    // `IconButton` itself, so the button is found within that labelled span.
    const revokeButton = within(screen.getByLabelText('Revoke API key')).getByRole('button');
    await user.click(revokeButton);
    expect(await screen.findByText('Revoke API Key')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Revoke' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(`/graphql-apis/${API}/api-keys/key-1`);
  });
});
