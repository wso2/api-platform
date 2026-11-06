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

describe('when the proxies fetch fails', () => {
  const failOnce = (error: unknown) => {
    const happy = getProxies.getMockImplementation()!;
    getProxies.mockRejectedValueOnce(error);
    getProxies.mockImplementation(happy);
  };

  it('shows the error alert instead of the table', async () => {
    getProxies.mockRejectedValue(Object.assign(new Error('Forbidden here'), { status: 403 }));
    renderList();

    expect(await screen.findByText('Forbidden here')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Search App LLM Proxies...')).not.toBeInTheDocument();
    expect(screen.queryByText(/Create your first App LLM Proxy/)).not.toBeInTheDocument();
  });

  it('refetches on Retry and recovers', async () => {
    failOnce(Object.assign(new Error('boom'), { status: 500 }));
    const { user } = renderList();

    expect(await screen.findByText(/Something went wrong\./)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(getProxies).toHaveBeenCalledTimes(2);
  });

  it('offers Retry for a statusless network error', async () => {
    failOnce(new Error('Network Error'));
    const { user } = renderList();

    expect(await screen.findByText('Network Error')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => expect(screen.getByText('Weather Bot')).toBeInTheDocument());
  });

  it('wraps a non-Error rejection in a generic message', async () => {
    failOnce('plain string');
    renderList();

    expect(await screen.findByText('Failed to fetch proxies')).toBeInTheDocument();
  });

  it('shows the empty state for a message-encoded 404', async () => {
    getProxies.mockRejectedValue(new Error('Request failed, status: 404'));
    renderList();

    expect(await screen.findByText(/Create your first App LLM Proxy/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
  });

  it('shows the empty state for a response-encoded 404', async () => {
    getProxies.mockRejectedValue(Object.assign(new Error('gone'), { response: { status: 404 } }));
    renderList();

    expect(await screen.findByText(/Create your first App LLM Proxy/)).toBeInTheDocument();
  });
});

describe('searching (request shape)', () => {
  it('sends the project and the trimmed, debounced term', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), '  Weather ');

    await waitFor(() =>
      expect(getProxies).toHaveBeenLastCalledWith('org-1', 'proj-1', expect.anything(), 20, 0, {
        search: 'Weather',
      })
    );
    expect(getProxies.mock.calls.length).toBeLessThan(6);
  });

  it('keeps the search box visible when a search matches nothing', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'zzz');

    await waitFor(() =>
      expect(screen.getByText(/No App LLM Proxies match your search/)).toBeInTheDocument()
    );
    expect(search()).toBeInTheDocument();
    expect(screen.queryByText(/Create your first App LLM Proxy/)).not.toBeInTheDocument();
  });
});

describe('pagination', () => {
  beforeEach(() => {
    const many = Array.from({ length: 45 }, (_, i) =>
      proxy({ id: `x-${i}`, displayName: `Proxy ${String(i).padStart(2, '0')}` })
    );
    getProxies.mockImplementation(async (_o, _p, _b, limit, offset) => ({
      count: Math.min(limit, many.length - offset),
      list: many.slice(offset, offset + limit),
      pagination: { total: many.length, offset, limit },
    }));
  });

  it('requests the next and previous pages', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Proxy 00')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(screen.getByText('Proxy 20')).toBeInTheDocument());
    expect(getProxies).toHaveBeenLastCalledWith('org-1', 'proj-1', expect.anything(), 20, 20, undefined);

    await user.click(screen.getByRole('button', { name: /previous page/i }));
    await waitFor(() => expect(screen.getByText('Proxy 00')).toBeInTheDocument());
  });

  it('refetches from the first page when the page size changes', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Proxy 00')).toBeInTheDocument());

    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByRole('option', { name: '50' }));

    await waitFor(() =>
      expect(getProxies).toHaveBeenLastCalledWith('org-1', 'proj-1', expect.anything(), 50, 0, undefined)
    );
    await waitFor(() => expect(screen.getByText('Proxy 44')).toBeInTheDocument());
  });

  it('hides the pager when the total is under one page', async () => {
    getProxies.mockImplementation(async (_o, _p, _b, limit, offset) => ({
      count: 2,
      list: proxies,
      pagination: { total: 2, offset, limit },
    }));
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /next page/i })).not.toBeInTheDocument();
  });
});

describe('deleting a proxy (more paths)', () => {
  it('refetches the list after a successful delete', async () => {
    deleteProxy.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    const before = getProxies.mock.calls.length;

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(getProxies.mock.calls.length).toBeGreaterThan(before));
  });

  it('reports the failure and closes the dialog when delete fails', async () => {
    deleteProxy.mockRejectedValue(new Error('proxy is deployed'));
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('proxy is deployed', 'error'));
    await waitFor(() =>
      expect(screen.queryByText(/Are you sure you want to delete/)).not.toBeInTheDocument()
    );
  });

  it('closes the dialog on Cancel without deleting', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    expect(screen.getByText(/Are you sure you want to delete Trip Planner/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/Are you sure you want to delete/)).not.toBeInTheDocument()
    );
    expect(deleteProxy).not.toHaveBeenCalled();
  });

  it('shows the read-only warning for a gateway-managed proxy', async () => {
    proxies = [proxy({ readOnly: true })];
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    expect(screen.getByText(/read-only|gateway/i)).toBeInTheDocument();
  });

  it('disables the delete control without permission', async () => {
    permitted = false;
    renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    expect(screen.getByRole('button', { name: 'Delete Trip Planner' })).toBeDisabled();
  });
});

describe('row rendering', () => {
  it('falls back to placeholders for a proxy with no description, version or provider', async () => {
    proxies = [proxy({ description: '', version: '', provider: undefined })];
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(3);
  });

  it('truncates a long description', async () => {
    proxies = [proxy({ description: 'a'.repeat(100) })];
    renderList();

    await waitFor(() => expect(screen.getByText('a'.repeat(60) + '…')).toBeInTheDocument());
  });

  it('shows the provider id when the provider is not in the loaded list (string and object refs)', async () => {
    proxies = [
      proxy({ id: 'a', displayName: 'Alpha', provider: 'prov-a' }),
      proxy({ id: 'b', displayName: 'Beta', provider: { id: 'prov-b' } }),
      proxy({ id: 'c', displayName: 'Gamma', provider: { id: 42 } }),
    ];
    renderList();

    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument());
    expect(screen.getByText('prov-a')).toBeInTheDocument();
    expect(screen.getByText('prov-b')).toBeInTheDocument();
  });

  it('shows the Create button only when the project has proxies', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByRole('link', { name: 'Create App LLM Proxy' })).toBeInTheDocument();
  });
});

describe('at organization level (project picker)', () => {
  beforeEach(() => {
    currentProject = null;
  });

  it('switches to project level after choosing a project', async () => {
    const { user } = renderList();
    const goButton = screen.getByRole('button', { name: 'Go to Project Level' });
    expect(goButton).toBeDisabled();

    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByRole('option', { name: 'Project Two' }));
    expect(goButton).not.toBeDisabled();
    await user.click(goButton);

    expect(setCurrentProject).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'proj-2' })
    );
  });
});
