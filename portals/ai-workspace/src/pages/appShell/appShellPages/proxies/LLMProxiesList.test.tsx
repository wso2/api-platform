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
import LLMProxiesList from './LLMProxiesList';

const showSnackbar = vi.fn();
const deleteProxy = vi.fn();
const getProxies = vi.fn();
let permitted = true;
let proxies: Record<string, unknown>[] = [];

const proxy = (over: Record<string, unknown> = {}) => ({
  id: 'trip-planner',
  displayName: 'Trip Planner',
  description: 'plans multi-city trips',
  version: '1.0.0',
  ...over,
});

// Mimics the real backend's `handleSearchClause`: a case-insensitive
// substring match against display name and id/handle only — NOT description
// or version, which the old client-side filter also matched on.
const matches = (p: Record<string, unknown>, search: string) => {
  if (!search) return true;
  const haystack = [p.displayName, p.id].filter(Boolean).join(' ').toLowerCase();
  return haystack.includes(search.toLowerCase());
};

let currentProject: { id: string; name: string } | null = { id: 'proj-1', name: 'Project One' };
const setCurrentProject = vi.fn();
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject,
    currentOrganization: { id: 'org-1', uuid: 'org-1', handle: 'acme' },
    projectsForCurrentOrganization: [
      { id: 'proj-1', name: 'project-one', displayName: 'Project One' },
      { id: 'proj-2', name: 'project-two', displayName: 'Project Two' },
    ],
    setCurrentProject,
    isProjectsLoading: false,
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => permitted }),
}));
vi.mock('../../../../contexts/proxy', () => ({
  useProxies: () => ({ deleteProxy }),
}));
vi.mock('../../../../contexts/llmProvider', () => ({
  formatRelativeTime: () => 'just now',
  useLLMProviders: () => ({ providersResponse: { list: [] } }),
}));
vi.mock('../../../../apis/proxyApis', () => ({
  getProxies: (...args: unknown[]) => getProxies(...args),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
  useAIWorkspaceSnackbar: () => showSnackbar,
}));

const renderList = () => renderWithProviders(<LLMProxiesList />);
const search = () => screen.getByPlaceholderText('Search App LLM Proxies...');

beforeEach(() => {
  permitted = true;
  currentProject = { id: 'proj-1', name: 'Project One' };
  setCurrentProject.mockReset();
  proxies = [
    proxy(),
    proxy({ id: 'weather-bot', displayName: 'Weather Bot', description: 'forecasts', version: '2.0.0' }),
  ];
  showSnackbar.mockReset();
  deleteProxy.mockReset();
  getProxies.mockReset();
  getProxies.mockImplementation(async (_orgId, _projectId, _baseUrl, limit, offset, options) => {
    const filtered = proxies.filter((p) => matches(p, options?.search ?? ''));
    return {
      count: filtered.length,
      list: filtered,
      pagination: { total: filtered.length, offset, limit },
    };
  });
});

describe('listing App LLM Proxies', () => {
  it('shows every proxy in the project', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('says so plainly when the project has none', async () => {
    proxies = [];
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Create your first App LLM Proxy/)).toBeInTheDocument()
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
      expect(screen.getByText(/No App LLM Proxies match your search/)).toBeInTheDocument()
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

describe('deleting a proxy', () => {
  it('reports success once the proxy is gone', async () => {
    deleteProxy.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Successfully deleted App LLM Proxy.', 'success')
    );
    expect(deleteProxy).toHaveBeenCalledWith('trip-planner');
  });
});

describe('without permission', () => {
  it('offers no delete control', async () => {
    permitted = false;
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    const deleteButton = screen.queryByRole('button', { name: 'Delete Trip Planner' });
    if (deleteButton) expect(deleteButton).toBeDisabled();
  });
});

describe('at organization level', () => {
  beforeEach(() => {
    currentProject = null;
  });

  it('explains that proxies live inside a project', () => {
    renderList();

    expect(
      screen.getByText(/App LLM Proxies are created and managed at the project level/)
    ).toBeInTheDocument();
  });

  it('does not list any proxy while no project is in scope', () => {
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(getProxies).not.toHaveBeenCalled();
  });
});
