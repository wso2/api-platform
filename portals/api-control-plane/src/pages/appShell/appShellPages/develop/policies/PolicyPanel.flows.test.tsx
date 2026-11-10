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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import type { Policy } from '@/api/resources/restApis';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { accepts, aRestApi, failure, recorder } from '@/test/msw';
import { policyHubHandlers, stubHubFetch, type HubPolicy } from '@/test/policyHub';
import { server } from '@/test/server';
import { fireEvent, renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { setDraggedPolicy } from './policyDnd';
import { PolicyPanel } from './PolicyPanel';

vi.mock('@/featureUsage/moesif');

const ORG = 'api-platform-demo';

const definition = [
  'name: api-key-auth',
  'version: 1.2',
  'parameters:',
  '  type: object',
  '  required: [key]',
  '  properties:',
  '    key:',
  '      type: string',
].join('\n');

const catalog: HubPolicy[] = [
  { definition, displayName: 'API Key Auth', name: 'api-key-auth', versions: ['1.2', '1.10'] },
  { displayName: 'CORS', name: 'cors', versions: ['1.0'] },
];

const operations = [
  { description: 'List orders', name: 'List orders', request: { method: 'GET', path: '/orders' } },
  {
    description: 'Create order',
    name: 'Create order',
    request: { method: 'POST', path: '/orders' },
  },
  { description: 'Get item', name: 'Get item', request: { method: 'GET', path: '/items' } },
];

const renderPanel = (
  options: { policies?: Policy[]; operations?: unknown[]; readOnly?: boolean } = {},
) => {
  const api = aRestApi({
    operations: (options.operations ?? operations) as never,
    policies: options.policies ?? [],
    readOnly: options.readOnly ?? false,
  });
  return {
    api,
    ...renderWithProviders(
      <ApiScopeProvider orgId={ORG}>
        <PolicyPanel api={api} />
      </ApiScopeProvider>,
    ),
  };
};

beforeEach(() => {
  resetHttpClient();
  resetTrackedFeatures();
  stubHubFetch();
  server.use(...policyHubHandlers(catalog));
});

afterEach(() => {
  vi.restoreAllMocks();
  setDraggedPolicy(null);
});

describe('PolicyPanel — global policies', () => {
  it('removes a policy, marks the page unsaved and saves the rest with major versions', async () => {
    const record = recorder();
    const { api, user } = renderPanel({
      policies: [
        { name: 'cors', params: {}, version: '1.0' },
        { name: 'api-key-auth', params: { key: 'API-Key' }, version: 'v1' },
      ],
    });
    server.use(accepts('put', '/rest-apis/:id', api, { record }));

    await user.click(screen.getAllByLabelText('Remove policy')[0]);
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(record.count()).toBe(1));
    expect(JSON.parse(record.last()!.body).policies).toEqual([
      { name: 'api-key-auth', params: { key: 'API-Key' }, version: 'v1' },
    ]);
  });

  it('counts each unsaved edit on the floating bar', async () => {
    const { user } = renderPanel({
      policies: [
        { name: 'cors', params: {}, version: 'v1' },
        { name: 'api-key-auth', params: {}, version: 'v1' },
      ],
    });
    expect(screen.queryByRole('region')).not.toBeInTheDocument();

    await user.click(screen.getAllByLabelText('Remove policy')[0]);
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();

    await user.click(screen.getAllByLabelText('Remove policy')[0]);
    expect(screen.getByText('2 unsaved changes')).toBeInTheDocument();
  });

  it('discards unsaved edits on Discard', async () => {
    const { user } = renderPanel({ policies: [{ name: 'cors', params: {}, version: 'v1' }] });

    await user.click(screen.getByLabelText('Remove policy'));
    expect(screen.queryByLabelText('Edit policy')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Discard' }));
    expect(await screen.findByLabelText('Edit policy')).toBeInTheDocument();
  });

  it('attaches a policy picked from the catalog once its parameters are filled', async () => {
    const { user } = renderPanel();

    await user.click(await screen.findByText('API Key Auth'));
    const submit = await screen.findByTestId('policy-param-submit');
    expect(submit).toBeDisabled();

    await user.type(screen.getByTestId('field-key'), 'X-Api-Key');
    await user.click(submit);

    expect(await screen.findByLabelText('Edit policy')).toBeInTheDocument();
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();
  });

  it('reopens an attached policy with its saved values and applies the edit', async () => {
    const { user } = renderPanel({
      policies: [{ name: 'api-key-auth', params: { key: 'Saved-Key' }, version: 'v1' }],
    });

    await user.click(await screen.findByLabelText('Edit policy'));
    const field = await screen.findByTestId('field-key');
    expect(field).toHaveValue('Saved-Key');

    await user.clear(field);
    await user.type(field, 'New-Key');
    await user.click(screen.getByTestId('policy-param-submit'));

    await waitFor(() => expect(screen.queryByTestId('field-key')).not.toBeInTheDocument());
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();
  });

  it('opens a dropped catalog policy for configuration', async () => {
    renderPanel();
    await screen.findByText('API Key Auth');
    setDraggedPolicy({ displayName: 'API Key Auth', name: 'api-key-auth', version: '1.2' });

    const zone = screen.getByText('Drag and drop policies here to apply across all resources');
    fireEvent.dragOver(zone, { dataTransfer: { dropEffect: 'none' } });
    fireEvent.drop(zone);

    expect(await screen.findByTestId('field-key')).toBeInTheDocument();
  });

  it('ignores a drop that is not a catalog policy', () => {
    renderPanel();

    const zone = screen.getByText('Drag and drop policies here to apply across all resources');
    fireEvent.drop(zone);

    expect(screen.queryByTestId('policy-param-submit')).not.toBeInTheDocument();
  });

  it('refuses drops on a read-only API', async () => {
    renderPanel({ readOnly: true });
    await screen.findByText('API Key Auth');
    setDraggedPolicy({ displayName: 'API Key Auth', name: 'api-key-auth', version: '1.2' });

    // A read-only API has no drop hint, only the plain "nothing attached" note.
    fireEvent.drop(screen.getByText('No policies attached.'));
    expect(screen.queryByTestId('policy-param-submit')).not.toBeInTheDocument();
  });
});

describe('PolicyPanel — resources', () => {
  it('filters the resources by path and by method', async () => {
    const { user } = renderPanel();

    expect(screen.getByText('List orders')).toBeInTheDocument();
    expect(screen.getByText('Get item')).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('Search resources by path'), 'items');
    expect(screen.queryByText('List orders')).not.toBeInTheDocument();
    expect(screen.getByText('Get item')).toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText('Search resources by path'));
    await user.click(screen.getByRole('combobox', { name: 'Filter resources by method' }));
    await user.click(within(await screen.findByRole('listbox')).getByText('POST'));
    expect(screen.getByText('Create order')).toBeInTheDocument();
    expect(screen.queryByText('Get item')).not.toBeInTheDocument();
  });

  it('says so when no resource matches or the API has none', async () => {
    const { user, unmount } = renderPanel();
    await user.type(screen.getByPlaceholderText('Search resources by path'), 'zzz');
    expect(screen.getByText('No resources match the filters.')).toBeInTheDocument();
    unmount();

    renderPanel({ operations: [] });
    expect(screen.getByText('No resources. Add them in the Routing tab.')).toBeInTheDocument();
  });

  it('lists the global policies as inherited on a resource and lets its own be removed', async () => {
    const { user } = renderPanel({
      operations: [
        {
          name: 'List orders',
          request: { method: 'GET', path: '/orders', policies: [{ name: 'cors', version: '1.0' }] },
        },
      ],
      policies: [{ name: 'api-key-auth', params: {}, version: 'v1' }],
    });

    await user.click(screen.getByRole('button', { name: 'Show details for GET /orders' }));

    expect(await screen.findByText('Inherited from global')).toBeInTheDocument();
    // API-level chip + the resource's own chip each carry their number.
    expect(screen.getByText('2')).toBeInTheDocument();

    const removeButtons = screen.getAllByLabelText('Remove policy');
    await user.click(removeButtons[removeButtons.length - 1]);
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();
  });

  it('opens a policy dropped on a resource', async () => {
    const { user } = renderPanel();
    await user.click(screen.getByRole('button', { name: 'Show details for GET /orders' }));
    await screen.findByText('API Key Auth');
    setDraggedPolicy({ displayName: 'API Key Auth', name: 'api-key-auth', version: '1.2' });

    const zone = await screen.findByText('Drag and drop policies here');
    fireEvent.dragOver(zone, { dataTransfer: { dropEffect: 'none' } });
    fireEvent.drop(zone);

    expect(await screen.findByTestId('field-key')).toBeInTheDocument();
  });

  it('attaches a clicked catalog policy to the resource that is open, and says so', async () => {
    const { user } = renderPanel();
    await user.click(screen.getByRole('button', { name: 'Show details for GET /orders' }));

    await user.click(await screen.findByText('API Key Auth'));

    expect(await screen.findByText('Applies to GET /orders')).toBeInTheDocument();
    await user.type(await screen.findByTestId('field-key'), 'X-Key');
    await user.click(screen.getByTestId('policy-param-submit'));

    // The chip lands in the resource's own list, which says how to reorder it.
    expect(await screen.findByText(/Drag to reorder/)).toBeInTheDocument();
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();
    expect(screen.getAllByLabelText('Edit policy')).toHaveLength(1);
  });

  it('targets Global policies when no resource is open', async () => {
    const { user } = renderPanel();

    await user.click(await screen.findByText('API Key Auth'));

    expect(await screen.findByText('Applies to Global policies')).toBeInTheDocument();
  });
});

describe('PolicyPanel — feature usage', () => {
  const browse = ['apip.policies.policy-hub-browse', { artifact_kind: 'rest-api' }];
  const apiKeyAuth = (policyScope: 'api' | 'operation') => [
    'apip.policies.client-authentication-attach',
    { artifact_kind: 'rest-api', policy_name: 'api-key-auth', policy_scope: policyScope },
  ];

  const attachApiKeyAuth = async (user: ReturnType<typeof renderPanel>['user']) => {
    // The catalog card is the last match once the policy's chip also shows its name.
    await user.click((await screen.findAllByText('API Key Auth')).at(-1)!);
    await user.type(await screen.findByTestId('field-key'), 'X-Key');
    await user.click(screen.getByTestId('policy-param-submit'));
    await waitFor(() => expect(screen.queryByTestId('field-key')).not.toBeInTheDocument());
  };

  it('reports opening the catalog, then each policy a save newly attaches with its scope', async () => {
    const record = recorder();
    const { api, user } = renderPanel({ policies: [{ name: 'cors', params: {}, version: 'v1' }] });
    server.use(accepts('put', '/rest-apis/:id', api, { record }));

    await attachApiKeyAuth(user);
    await user.click(screen.getByRole('button', { name: 'Show details for GET /orders' }));
    await attachApiKeyAuth(user);
    // Removing a saved policy is not an attachment.
    await user.click(screen.getAllByLabelText('Remove policy')[0]);
    // Attaching only stages the change; nothing is reported until Save.
    expect(trackedFeatures()).toEqual([browse]);

    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(record.count()).toBe(1));
    expect(trackedFeatures()).toEqual([browse, apiKeyAuth('api'), apiKeyAuth('operation')]);
  });

  it('reports nothing when a save only edits or removes saved policies', async () => {
    const record = recorder();
    const { api, user } = renderPanel({
      policies: [
        { name: 'api-key-auth', params: { key: 'Saved-Key' }, version: 'v1' },
        { name: 'cors', params: {}, version: 'v1' },
      ],
    });
    server.use(accepts('put', '/rest-apis/:id', api, { record }));

    await user.click((await screen.findAllByLabelText('Edit policy'))[0]);
    const field = await screen.findByTestId('field-key');
    await user.clear(field);
    await user.type(field, 'New-Key');
    await user.click(screen.getByTestId('policy-param-submit'));
    await waitFor(() => expect(screen.queryByTestId('field-key')).not.toBeInTheDocument());
    await user.click(screen.getAllByLabelText('Remove policy')[1]);

    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(record.count()).toBe(1));
    expect(trackedFeatures()).toEqual([browse]);
  });

  it('does not report an attachment again when a rejected save is retried', async () => {
    const rejected = recorder();
    const record = recorder();
    const { api, user } = renderPanel();
    server.use(failure('put', '/rest-apis/:id', 500, 'INTERNAL_ERROR', { record: rejected }));

    await attachApiKeyAuth(user);
    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(rejected.count()).toBe(1));

    server.use(accepts('put', '/rest-apis/:id', api, { record }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(record.count()).toBe(1));
    expect(trackedFeatures()).toEqual([browse, apiKeyAuth('api')]);
  });
});
