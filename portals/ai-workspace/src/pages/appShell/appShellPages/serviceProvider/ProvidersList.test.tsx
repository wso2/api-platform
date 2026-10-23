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

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import ProvidersList from './ProvidersList';

const showSnackbar = vi.fn();
const deleteProvider = vi.fn();
const getLLMProviders = vi.fn();
const getLLMProviderProxies = vi.fn();
let permitted = true;
let providers: Record<string, unknown>[] = [];

const provider = (over: Record<string, unknown> = {}) => ({
  id: 'trip-planner',
  displayName: 'Trip Planner',
  description: 'plans multi-city trips',
  version: '1.0.0',
  status: 'Active',
  ...over,
});

// Mimics the real backend's `handleSearchClause`: a case-insensitive
// substring match against display name and id/handle only — NOT description
// or version.
const matches = (p: Record<string, unknown>, search: string) => {
  if (!search) return true;
  const haystack = [p.displayName, p.id].filter(Boolean).join(' ').toLowerCase();
  return haystack.includes(search.toLowerCase());
};

let currentProject: { id: string; name: string } | null = null;
const setCurrentProject = vi.fn();
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject,
    currentOrganization: { id: 'org-1', uuid: 'org-1', handle: 'acme' },
    setCurrentProject,
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => permitted }),
}));
vi.mock('../../../../contexts/llmProvider', () => ({
  formatRelativeTime: () => 'just now',
  useLLMProviders: () => ({ deleteProvider }),
}));
vi.mock('../../../../contexts/llmProvider/providerTemplate', () => ({
  useProviderTemplates: () => ({ templatesResponse: { list: [] } }),
}));
vi.mock('../../../../contexts/ApplicationsContext', () => ({
  useApplications: () => ({ applications: [] }),
}));
vi.mock('../../../../hooks/useResourceLimits', () => ({
  useResourceLimits: () => ({ canCreate: () => true, limitMessage: () => '' }),
}));
vi.mock('../../../../apis/llmProviderApis', () => ({
  getLLMProviders: (...args: unknown[]) => getLLMProviders(...args),
  getLLMProviderProxies: (...args: unknown[]) => getLLMProviderProxies(...args),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));

const renderList = () => renderWithProviders(<ProvidersList />);
const search = () => screen.getByPlaceholderText('Search Service Providers...');

beforeEach(() => {
  permitted = true;
  currentProject = null;
  setCurrentProject.mockReset();
  providers = [
    provider(),
    provider({ id: 'weather-bot', displayName: 'Weather Bot', description: 'forecasts', version: '2.0.0' }),
  ];
  showSnackbar.mockReset();
  deleteProvider.mockReset();
  getLLMProviders.mockReset();
  getLLMProviderProxies.mockReset();
  getLLMProviderProxies.mockResolvedValue({ count: 0, list: [], pagination: { total: 0, offset: 0, limit: 1 } });
  getLLMProviders.mockImplementation(async (_orgUuid, _baseUrl, limit, offset, options) => {
    const filtered = providers.filter((p) => matches(p, options?.search ?? ''));
    return {
      count: filtered.length,
      list: filtered,
      pagination: { total: filtered.length, offset, limit },
    };
  });
});

describe('listing LLM Providers', () => {
  it('shows every provider in the organization', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('says so plainly when the organization has none', async () => {
    providers = [];
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Create your first LLM Provider/)).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });
});

describe('searching', () => {
  it('matches on the display name', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'Trip');

    await waitFor(() => expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument());
    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
  });

  it('does not match on the description or version (narrower than the old client-side filter)', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'multi-city');

    await waitFor(() =>
      expect(screen.getByText('No providers match your search.')).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });

  it('shows everything again once the search is cleared', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    const input = search();

    await user.type(input, 'Trip');
    await waitFor(() => expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument());

    await user.clear(input);

    await waitFor(() => expect(screen.getByText('Weather Bot')).toBeInTheDocument());
  });
});

describe('deleting a provider', () => {
  it('reports success once the provider is gone, after confirming its name', async () => {
    deleteProvider.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await waitFor(() =>
      expect(getLLMProviderProxies).toHaveBeenCalledWith('trip-planner', 'org-1', expect.anything(), 1, 0)
    );

    const confirmButton = await screen.findByRole('button', { name: 'Delete' });
    expect(confirmButton).toBeDisabled();

    await user.type(screen.getByPlaceholderText('Trip Planner'), 'Trip Planner');
    expect(confirmButton).not.toBeDisabled();

    await user.click(confirmButton);

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Provider deleted successfully.', 'success')
    );
    expect(deleteProvider).toHaveBeenCalledWith('trip-planner');
  });

  it('blocks deletion when the provider is still in use by a proxy', async () => {
    getLLMProviderProxies.mockResolvedValue({ count: 2, list: [], pagination: { total: 2, offset: 0, limit: 1 } });
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith(
        expect.stringContaining('Cannot delete "Trip Planner"'),
        'error'
      )
    );
    expect(screen.queryByPlaceholderText('Trip Planner')).not.toBeInTheDocument();
  });
});

describe('without permission', () => {
  it('offers no delete control', async () => {
    permitted = false;
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: 'Delete Trip Planner' })).not.toBeInTheDocument();
  });
});

describe('at project level', () => {
  beforeEach(() => {
    currentProject = { id: 'proj-1', name: 'Project One' };
  });

  it('explains that provider management is an organization-level feature', async () => {
    renderList();

    expect(
      screen.getByText(/Provider management is available at the organization level/)
    ).toBeInTheDocument();
    // The fetch still runs in the background (it isn't gated on project level),
    // but its result has nowhere to render — this project-level view is a dead
    // end regardless of what the organization's provider list looks like.
    await waitFor(() => expect(getLLMProviders).toHaveBeenCalled());
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });
});

describe('while the catalogue is still loading', () => {
  it('shows a loading indicator instead of an empty listing', () => {
    // A promise that never resolves during the test keeps the component in
    // its initial "first fetch in flight" state.
    getLLMProviders.mockImplementation(() => new Promise(() => {}));
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(screen.queryByText(/Create your first LLM Provider/)).not.toBeInTheDocument();
  });
});
