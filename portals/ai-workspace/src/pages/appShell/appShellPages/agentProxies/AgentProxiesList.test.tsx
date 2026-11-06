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
import AgentProxiesList from './AgentProxiesList';

const navigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigate };
});

const showSnackbar = vi.fn();
const deleteAgentProxy = vi.fn();
const getAgentProxies = vi.fn();
let permitted = true;
let proxies: Record<string, unknown>[] = [];

const proxy = (over: Record<string, unknown> = {}) => ({
  id: 'trip-planner',
  displayName: 'Trip Planner',
  description: 'plans multi-city trips',
  context: '/trips',
  version: '1.0.0',
  ...over,
});

// Mimics the real backend's `handleSearchClause` (platform-api/internal/repository/pagination.go):
// a case-insensitive substring match against display name and id/handle only —
// NOT description, context, or version. The mocked "server" this component
// now talks to instead of a pre-fetched in-memory list must match that exact
// contract, or these tests would pass against behavior the real API doesn't have.
const matches = (p: Record<string, unknown>, search: string) => {
  if (!search) return true;
  const haystack = [p.displayName, p.id]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
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
vi.mock('../../../../contexts/agentProxy', () => ({
  AgentProxiesProvider: ({ children }: { children: React.ReactNode }) => children,
  useAgentProxies: () => ({ deleteAgentProxy }),
}));
vi.mock('../../../../apis/agent/agentProxiesApis', () => ({
  getAgentProxies: (...args: unknown[]) => getAgentProxies(...args),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));

const renderList = () => renderWithProviders(<AgentProxiesList />);
const search = () => screen.getByPlaceholderText('Search Agent Proxies...');

beforeEach(() => {
  permitted = true;
  currentProject = { id: 'proj-1', name: 'Project One' };
  setCurrentProject.mockReset();
  proxies = [proxy(), proxy({ id: 'weather', displayName: 'Weather Bot', description: 'forecasts', context: '/weather', version: '2.0.0' })];
  showSnackbar.mockReset();
  navigate.mockReset();
  deleteAgentProxy.mockReset();
  getAgentProxies.mockReset();
  getAgentProxies.mockImplementation(async (_projectId, _baseUrl, limit, offset, options) => {
    const filtered = proxies.filter((p) => matches(p, options?.search ?? ''));
    return {
      count: filtered.length,
      list: filtered,
      pagination: { total: filtered.length, offset, limit },
    };
  });
});

describe('listing agent proxies', () => {
  it('shows every proxy in the project', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('says so plainly when the project has none', async () => {
    proxies = [];
    renderList();

    await waitFor(() =>
      expect(
        screen.getByText(/Create your first agent proxy/)
      ).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });
});

describe('searching', () => {
  it.each([
    ['the display name', 'Trip'],
    ['the id/handle', 'trip-planner'],
  ])('matches on %s', async (_label, term) => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), term);

    await waitFor(() => expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument());
    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
  });

  // Regression guard: the backend's search deliberately covers only display
  // name and id/handle (handleSearchClause), not description/context/version
  // — a prior client-side filter covered all four, and a test mock that still
  // simulated that wider match would mask this narrower real contract.
  it.each([
    ['the description', 'multi-city'],
    ['the context', '/trips'],
    ['the version', '1.0.0'],
  ])('does not match on %s', async (_label, term) => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), term);

    await waitFor(() =>
      expect(
        screen.getByText(/No agent proxies match your search/)
      ).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });

  it('ignores case', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'TRIP PLANNER');

    await waitFor(() => expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument());
    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
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

  it('matches nothing when no proxy carries the term', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), 'nothing-matches-this');

    await waitFor(() =>
      expect(
        screen.getByText(/No agent proxies match your search/)
      ).toBeInTheDocument()
    );
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument();
  });
});

describe('deleting a proxy', () => {
  it('reports success once the proxy is gone', async () => {
    deleteAgentProxy.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete', exact: true }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Agent Proxy deleted successfully.', 'success')
    );
    expect(deleteAgentProxy).toHaveBeenCalledWith('trip-planner');
  });

  it('reports the reason when the delete is refused', async () => {
    deleteAgentProxy.mockRejectedValue(new Error('still deployed'));
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete', exact: true }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith(expect.any(String), 'error')
    );
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
    // No project in scope, so the list cannot show proxies and offers a chooser instead.
    currentProject = null;
  });

  it('explains that proxies live inside a project', () => {
    renderList();

    expect(
      screen.getByText(/Agent proxies are created and managed at the project level/)
    ).toBeInTheDocument();
  });

  it('offers the projects in the organization to switch into', async () => {
    const { user } = renderList();

    await user.click(screen.getByRole('combobox'));

    expect(screen.getByRole('option', { name: 'Project One' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Project Two' })).toBeInTheDocument();
  });

  it('cannot switch until a project is chosen', () => {
    renderList();

    expect(screen.getByRole('button', { name: /Go to Project Level/ })).toBeDisabled();
  });

  it('switches into the project that was picked', async () => {
    const { user } = renderList();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: 'Project Two' }));
    await user.click(screen.getByRole('button', { name: /Go to Project Level/ }));

    expect(setCurrentProject).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'proj-2' })
    );
  });

  it('does not list any proxy while no project is in scope', () => {
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(getAgentProxies).not.toHaveBeenCalled();
  });
});

describe('while the catalogue is still loading', () => {
  it('shows a placeholder table instead of an empty listing', () => {
    // A promise that never resolves during the test keeps the component in
    // its initial "first fetch in flight" state.
    getAgentProxies.mockImplementation(() => new Promise(() => {}));
    renderList();

    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByText('Last Updated')).toBeInTheDocument();
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });
});

describe('fetch failure', () => {
  it('shows an error and refetches on retry', async () => {
    getAgentProxies.mockRejectedValueOnce(new Error('boom'));
    const { user } = renderList();

    const retry = await screen.findAllByRole('button', { name: /Retry/ });
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();

    await user.click(retry[0]);

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(getAgentProxies).toHaveBeenCalledTimes(2);
  });

  it('wraps a non-Error rejection', async () => {
    getAgentProxies.mockRejectedValueOnce('nope');
    renderList();

    expect((await screen.findAllByRole('button', { name: /Retry/ })).length).toBeGreaterThan(0);
  });
});

describe('row interaction', () => {
  it('opens the proxy when its row is clicked', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByText('Trip Planner'));

    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/agent-proxy/trip-planner'));
  });

  it('does not open the proxy when delete is clicked', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    expect(navigate).not.toHaveBeenCalled();
  });

  it('keeps the proxy when the delete dialog is cancelled', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByText(/Are you sure you want to delete/)).not.toBeInTheDocument());
    expect(deleteAgentProxy).not.toHaveBeenCalled();
  });

  it('switches project and navigates from the org-level chooser', async () => {
    currentProject = null;
    const { user } = renderList();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: 'Project Two' }));
    await user.click(screen.getByRole('button', { name: /Go to Project Level/ }));

    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/agent-proxy'));
  });
});

describe('pagination', () => {
  const many = (n: number) =>
    Array.from({ length: n }, (_, i) => proxy({ id: `p-${i}`, displayName: `Proxy ${i}` }));

  beforeEach(() => {
    const all = many(45);
    getAgentProxies.mockImplementation(async (_p, _b, limit, offset) => ({
      count: all.length,
      list: all.slice(offset, offset + limit),
      pagination: { total: all.length, offset, limit },
    }));
  });

  it('requests the next page and changes page size', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Proxy 0')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(getAgentProxies).toHaveBeenLastCalledWith('proj-1', expect.anything(), 20, 20, undefined));
    await waitFor(() => expect(screen.getByText('Proxy 20')).toBeInTheDocument());

    await user.click(screen.getByRole('combobox', { name: /rows per page/i }));
    await user.click(screen.getByRole('option', { name: '50' }));
    await waitFor(() => expect(getAgentProxies).toHaveBeenLastCalledWith('proj-1', expect.anything(), 50, 0, undefined));
  });
});

describe('read-only proxy deletion', () => {
  it('warns when the proxy is gateway-managed', async () => {
    proxies = [proxy({ readOnly: true })];
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    expect(screen.getByText(/Are you sure you want to delete Trip Planner/)).toBeInTheDocument();
  });
});
