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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { renderWithProviders, screen, waitFor, within } from '../../../../test/utils';
import CustomPoliciesList from './CustomPoliciesList';

const showSnackbar = vi.fn();
const getGatewayCustomPolicies = vi.fn();
const deleteGatewayCustomPolicy = vi.fn();
let permitted = true;
let policies: Record<string, unknown>[] = [];

const policy = (over: Record<string, unknown> = {}) => ({
  uuid: 'uuid-1',
  organizationUuid: 'org-1',
  name: 'rate-limit',
  version: 'v1.0.0',
  description: 'limits requests',
  policyDefinition: {},
  createdAt: '2026-01-02T03:04:05Z',
  updatedAt: '2026-02-03T04:05:06Z',
  ...over,
});

vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => permitted }),
}));
vi.mock('../../../../apis/gatewayPolicyApis', () => ({
  getGatewayCustomPolicies: (...args: unknown[]) => getGatewayCustomPolicies(...args),
  deleteGatewayCustomPolicy: (...args: unknown[]) => deleteGatewayCustomPolicy(...args),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  useAIWorkspaceSnackbar: () => showSnackbar,
  default: () => showSnackbar,
}));

const renderList = (props: { embedded?: boolean } = {}) =>
  renderWithProviders(<CustomPoliciesList {...props} />);
const search = () => screen.getByPlaceholderText('Search policies...');

beforeEach(() => {
  permitted = true;
  policies = [
    policy(),
    policy({ uuid: 'uuid-2', name: 'auth-check', version: '2.0.0', description: undefined, createdAt: undefined, updatedAt: undefined }),
  ];
  showSnackbar.mockReset();
  deleteGatewayCustomPolicy.mockReset();
  getGatewayCustomPolicies.mockReset();
  getGatewayCustomPolicies.mockImplementation(async () => ({
    count: policies.length,
    list: policies,
  }));
});

const rowNames = () =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((r) => within(r).getAllByRole('cell')[0].textContent);

describe('listing custom policies', () => {
  it('renders each policy with a normalised version and placeholders for missing values', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
    expect(screen.getByText('auth-check')).toBeInTheDocument();
    expect(screen.getByText('v1.0.0')).toBeInTheDocument();
    expect(screen.getByText('v2.0.0')).toBeInTheDocument();
    expect(screen.getByText('limits requests')).toBeInTheDocument();
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(3);
  });

  it('shows a loading row while fetching', () => {
    getGatewayCustomPolicies.mockImplementation(() => new Promise(() => {}));
    renderList();

    expect(screen.getByText('Loading custom policies...')).toBeInTheDocument();
  });

  it('renders embedded without the page wrapper', async () => {
    renderList({ embedded: true });

    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
  });

  it('tolerates a response with no list', async () => {
    getGatewayCustomPolicies.mockResolvedValue({ count: 0 });
    renderList();

    await waitFor(() => expect(screen.getByText('No custom policies found')).toBeInTheDocument());
  });
});

describe('empty states', () => {
  it('shows the empty state and no search box when there are no policies', async () => {
    policies = [];
    renderList();

    await waitFor(() => expect(screen.getByText('No custom policies found')).toBeInTheDocument());
    expect(screen.getByText(/Custom policies synced from your gateways will appear here/)).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Search policies...')).not.toBeInTheDocument();
  });

  it('shows the no-results message (not the empty state) when a search matches nothing', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());

    await user.type(search(), 'zzz');

    await waitFor(() =>
      expect(screen.getByText(/No custom policies match your search/)).toBeInTheDocument()
    );
    expect(screen.queryByText('No custom policies found')).not.toBeInTheDocument();
    expect(screen.queryByText(/will appear here/)).not.toBeInTheDocument();
  });

  it('restores the list when the search is cleared', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());

    await user.type(search(), 'zzz');
    await waitFor(() =>
      expect(screen.getByText(/No custom policies match your search/)).toBeInTheDocument()
    );
    await user.clear(search());

    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
  });
});

describe('searching', () => {
  it('matches name, version and description', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());

    await user.type(search(), 'limits req');
    await waitFor(() => expect(screen.queryByText('auth-check')).not.toBeInTheDocument());
    expect(screen.getByText('rate-limit')).toBeInTheDocument();

    await user.clear(search());
    await user.type(search(), '2.0');
    await waitFor(() => expect(screen.queryByText('rate-limit')).not.toBeInTheDocument());
    expect(screen.getByText('auth-check')).toBeInTheDocument();
  });
});

describe('sorting', () => {
  it('sorts by name ascending by default and toggles direction on a second click', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
    expect(rowNames()).toEqual(['auth-check', 'rate-limit']);

    await user.click(screen.getByText('Name'));
    expect(rowNames()).toEqual(['rate-limit', 'auth-check']);

    await user.click(screen.getByText('Name'));
    expect(rowNames()).toEqual(['auth-check', 'rate-limit']);
  });

  it('switches column and resets to ascending', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
    await user.click(screen.getByText('Name'));

    await user.click(screen.getByText('Version'));

    // 'v1.0.0' sorts before '2.0.0'? localeCompare: digits before letters.
    expect(rowNames()).toEqual(['auth-check', 'rate-limit']);
    await user.click(screen.getByText('Created At'));
    expect(rowNames()).toHaveLength(2);
    await user.click(screen.getByText('Updated At'));
    expect(rowNames()).toHaveLength(2);
  });
});

describe('pagination', () => {
  beforeEach(() => {
    policies = Array.from({ length: 12 }, (_, i) =>
      policy({ uuid: `u-${i}`, name: `policy-${String(i).padStart(2, '0')}` })
    );
  });

  it('pages client-side', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('policy-00')).toBeInTheDocument());
    expect(screen.queryByText('policy-10')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /next page/i }));

    expect(screen.getByText('policy-10')).toBeInTheDocument();
    expect(screen.queryByText('policy-00')).not.toBeInTheDocument();
  });

  it('changes rows per page', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('policy-00')).toBeInTheDocument());

    await user.click(screen.getByRole('combobox', { name: /Rows per page/i }));
    await user.click(await screen.findByRole('option', { name: '25' }));

    await waitFor(() => expect(screen.getByText('policy-11')).toBeInTheDocument());
  });
});

describe('load failure', () => {
  it('shows the error and refetches on retry', async () => {
    getGatewayCustomPolicies.mockRejectedValueOnce(new Error('network down'));
    const { user } = renderList();

    await waitFor(() => expect(screen.getByText(/network down/)).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /Retry/ }));

    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
  });

  it('wraps a non-Error rejection', async () => {
    getGatewayCustomPolicies.mockRejectedValueOnce('boom');
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Failed to load custom policies|Something went wrong/)).toBeInTheDocument()
    );
  });
});

describe('deleting a policy', () => {
  it('deletes by uuid and version, reports success and refetches', async () => {
    deleteGatewayCustomPolicy.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete rate-limit' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Custom policy deleted successfully', 'success')
    );
    expect(deleteGatewayCustomPolicy).toHaveBeenCalledWith('uuid-1', 'v1.0.0');
    expect(getGatewayCustomPolicies).toHaveBeenCalledTimes(2);
  });

  it('reports the failure', async () => {
    deleteGatewayCustomPolicy.mockRejectedValue(new Error('still attached'));
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete rate-limit' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith(expect.any(String), 'error'));
  });

  it('cancels without deleting', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete rate-limit' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(deleteGatewayCustomPolicy).not.toHaveBeenCalled();
  });

  it('hides the actions column without permission', async () => {
    permitted = false;
    renderList();

    await waitFor(() => expect(screen.getByText('rate-limit')).toBeInTheDocument());
    expect(screen.queryByText('Actions')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete rate-limit' })).not.toBeInTheDocument();
  });
});
