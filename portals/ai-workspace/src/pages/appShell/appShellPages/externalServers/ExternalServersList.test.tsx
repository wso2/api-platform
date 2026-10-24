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
import ExternalServersList from './ExternalServersList';

const navigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigate };
});

const showSnackbar = vi.fn();
const getMCPServers = vi.fn();
const deleteMCPServer = vi.fn();
let permitted = true;
let orgUuid = 'org-1';
let quotaOk = true;
let servers: Record<string, unknown>[] = [];

const server = (over: Record<string, unknown> = {}) => ({
  id: 'trip-planner',
  displayName: 'Trip Planner',
  description: 'plans multi-city trips',
  context: '/trips',
  version: '1.0.0',
  ...over,
});

// Mimics the real backend's `handleSearchClause`: a case-insensitive
// substring match against display name and id/handle only — NOT description,
// context, or version.
const matches = (s: Record<string, unknown>, search: string) => {
  if (!search) return true;
  const haystack = [s.displayName, s.id].filter(Boolean).join(' ').toLowerCase();
  return haystack.includes(search.toLowerCase());
};

let currentProject: { id: string; name: string } | null = { id: 'proj-1', name: 'Project One' };
const setCurrentProject = vi.fn();
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject,
    currentOrganization: { id: 'org-1', uuid: orgUuid, handle: 'acme' },
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
vi.mock('../../../../hooks/useResourceLimits', () => ({
  useResourceLimits: () => ({ canCreate: () => quotaOk, limitMessage: () => 'Quota reached' }),
}));
vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({
  mcpProxiesApis: {
    getMCPServers: (...args: unknown[]) => getMCPServers(...args),
    deleteMCPServer: (...args: unknown[]) => deleteMCPServer(...args),
  },
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));

const renderList = () => renderWithProviders(<ExternalServersList />);
const search = () => screen.getByPlaceholderText('Search MCP Proxies...');

beforeEach(() => {
  permitted = true;
  orgUuid = 'org-1';
  quotaOk = true;
  navigate.mockReset();
  currentProject = { id: 'proj-1', name: 'Project One' };
  setCurrentProject.mockReset();
  servers = [
    server(),
    server({ id: 'weather-bot', displayName: 'Weather Bot', description: 'forecasts', context: '/weather', version: '2.0.0' }),
  ];
  showSnackbar.mockReset();
  deleteMCPServer.mockReset();
  getMCPServers.mockReset();
  getMCPServers.mockImplementation(async (_projectId, _baseUrl, limit, offset, options) => {
    const filtered = servers.filter((s) => matches(s, options?.search ?? ''));
    return {
      count: filtered.length,
      list: filtered,
      pagination: { total: filtered.length, offset, limit },
    };
  });
});

describe('listing MCP Proxies', () => {
  it('shows every server in the project', async () => {
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('says so plainly when the project has none', async () => {
    servers = [];
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Create your first MCP Proxy/)).toBeInTheDocument()
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

  it.each([
    ['the description', 'multi-city'],
    ['the context', '/trips'],
    ['the version', '1.0.0'],
  ])('does not match on %s (narrower than the old client-side filter)', async (_label, term) => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), term);

    await waitFor(() =>
      expect(screen.getByText(/No MCP proxies match your search/)).toBeInTheDocument()
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

describe('deleting a server', () => {
  it('reports success once the server is gone', async () => {
    deleteMCPServer.mockResolvedValue(undefined);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    expect(screen.getByText('Delete external server')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('MCP Proxy deleted successfully.', 'success')
    );
    expect(deleteMCPServer).toHaveBeenCalledWith('trip-planner', expect.anything());
  });

  it('reports the reason when the delete is refused', async () => {
    deleteMCPServer.mockRejectedValue(new Error('still in use'));
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('still in use', 'error')
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
    currentProject = null;
  });

  it('explains that MCP proxies live inside a project', () => {
    renderList();

    expect(
      screen.getByText(/MCP proxies are created and managed at the project level/)
    ).toBeInTheDocument();
  });

  it('does not list any server while no project is in scope', () => {
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(getMCPServers).not.toHaveBeenCalled();
  });
});

describe('while the catalogue is still loading', () => {
  it('shows a loading indicator instead of an empty listing', () => {
    // A promise that never resolves during the test keeps the component in
    // its initial "first fetch in flight" state.
    getMCPServers.mockImplementation(() => new Promise(() => {}));
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(screen.queryByText(/Create your first MCP Proxy/)).not.toBeInTheDocument();
  });
});

describe('when the list cannot be loaded', () => {
  it('shows the error and refetches on retry', async () => {
    getMCPServers.mockRejectedValueOnce(new Error('network down'));
    const { user } = renderList();

    await waitFor(() => expect(screen.getByText(/network down/)).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /Retry/ }));

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(getMCPServers).toHaveBeenCalledTimes(2);
  });

  it('wraps a non-Error rejection in a generic error', async () => {
    getMCPServers.mockRejectedValueOnce('boom');
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/Failed to fetch MCP proxies|Something went wrong/)).toBeInTheDocument()
    );
  });
});

describe('create controls', () => {
  it('offers Create MCP Proxy once the project has servers', async () => {
    renderList();

    const link = await screen.findByRole('link', { name: 'Create MCP Proxy' });
    expect(link).toHaveAttribute('href', expect.stringContaining('/mcp-proxy/create'));
  });

  it('disables creation without permission', async () => {
    permitted = false;
    renderList();

    const label = await screen.findByText('Create MCP Proxy');
    expect(label.closest('a')).toHaveAttribute('aria-disabled', 'true');
  });

  it('disables creation when the quota is reached', async () => {
    quotaOk = false;
    renderList();

    const label = await screen.findByText('Create MCP Proxy');
    expect(label.closest('a')).toHaveAttribute('aria-disabled', 'true');
  });

  it('disables the empty-state create button without permission', async () => {
    permitted = false;
    servers = [];
    renderList();

    const label = await screen.findByText('Create MCP Proxy');
    expect(label.closest('a')).toHaveAttribute('aria-disabled', 'true');
  });

  it('disables row delete buttons without permission', async () => {
    permitted = false;
    renderList();

    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Delete Trip Planner' })).toBeDisabled();
  });
});

describe('opening a server', () => {
  it('navigates when a row is clicked, but not when delete is clicked', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    expect(navigate).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    await user.click(screen.getByText('Trip Planner'));

    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/mcp-proxy/trip-planner'));
  });

  it('shows placeholders for missing fields', async () => {
    servers = [
      server({ displayName: '', description: undefined, context: undefined, version: undefined }),
    ];
    renderList();

    await waitFor(() => expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(4));
  });
});

describe('deleting a server (dialog)', () => {
  it('closes without deleting on Cancel', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() =>
      expect(screen.queryByText(/Are you sure you want to delete/)).not.toBeInTheDocument()
    );
    expect(deleteMCPServer).not.toHaveBeenCalled();
  });

  it('warns when the server is read-only (gateway artifact)', async () => {
    servers = [server({ readOnly: true })];
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    expect(screen.getByText(/Are you sure you want to delete Trip Planner/)).toBeInTheDocument();
    expect(within(screen.getByRole('dialog')).getAllByText(/MCP Proxy/i).length).toBeGreaterThan(0);
  });
});

describe('pagination', () => {
  const many = (n: number) =>
    Array.from({ length: n }, (_, i) =>
      server({ id: `srv-${i}`, displayName: `Srv ${String(i).padStart(2, '0')}` })
    );

  beforeEach(() => {
    getMCPServers.mockImplementation(async (_p, _b, limit, offset) => ({
      count: servers.length,
      list: servers.slice(offset, offset + limit),
      pagination: { total: servers.length, offset, limit },
    }));
  });

  it('hides the pager when there is less than one page', async () => {
    servers = many(5);
    renderList();

    await waitFor(() => expect(screen.getByText('Srv 00')).toBeInTheDocument());
    expect(screen.queryByText(/Rows per page/)).not.toBeInTheDocument();
  });

  it('requests the next page with the matching offset', async () => {
    servers = many(25);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Srv 00')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /next page/i }));

    await waitFor(() => expect(screen.getByText('Srv 20')).toBeInTheDocument());
    const last = getMCPServers.mock.calls.at(-1)!;
    expect(last[2]).toBe(20);
    expect(last[3]).toBe(20);
  });

  it('resets to the first page when rows per page changes', async () => {
    servers = many(60);
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Srv 00')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(screen.getByText('Srv 20')).toBeInTheDocument());

    await user.click(screen.getByRole('combobox', { name: /Rows per page/i }));
    await user.click(await screen.findByRole('option', { name: '50' }));

    await waitFor(() => expect(screen.getByText('Srv 49')).toBeInTheDocument());
    const last = getMCPServers.mock.calls.at(-1)!;
    expect(last[2]).toBe(50);
    expect(last[3]).toBe(0);
  });

  it('falls back a page when the last row of the last page is deleted', async () => {
    servers = many(21);
    deleteMCPServer.mockImplementation(async (id: string) => {
      servers = servers.filter((s) => s.id !== id);
    });
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Srv 00')).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: /next page/i }));
    await waitFor(() => expect(screen.getByText('Srv 20')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Delete Srv 20' }));
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(screen.getByText('Srv 00')).toBeInTheDocument());
    expect(screen.queryByText('Srv 20')).not.toBeInTheDocument();
    expect(getMCPServers.mock.calls.at(-1)![3]).toBe(0);
  });
});

describe('searching (edge cases)', () => {
  it('sends a trimmed query and keeps the create button while filtered to nothing', async () => {
    const { user } = renderList();
    await waitFor(() => expect(screen.getByText('Trip Planner')).toBeInTheDocument());

    await user.type(search(), '  zzz  ');

    await waitFor(() =>
      expect(screen.getByText(/No MCP proxies match your search/)).toBeInTheDocument()
    );
    expect(getMCPServers.mock.calls.at(-1)![4]).toEqual({ search: 'zzz' });
    expect(screen.getByText('Create MCP Proxy')).toBeInTheDocument();
  });
});

describe('without an organization', () => {
  it('does not fetch and shows an empty table', async () => {
    orgUuid = '';
    renderList();

    await waitFor(() =>
      expect(screen.getByText(/No MCP proxies match your search/)).toBeInTheDocument()
    );
    expect(getMCPServers).not.toHaveBeenCalled();
  });
});

describe('choosing a project at organization level', () => {
  beforeEach(() => {
    currentProject = null;
  });

  it('keeps Go to Project Level disabled until a project is chosen', () => {
    renderList();

    expect(screen.getByRole('button', { name: 'Go to Project Level' })).toBeDisabled();
  });

  it('switches to the chosen project and navigates to its MCP proxies', async () => {
    const { user } = renderList();

    await user.click(screen.getByRole('combobox'));
    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('Project Two'));
    await user.click(screen.getByRole('button', { name: 'Go to Project Level' }));

    expect(setCurrentProject).toHaveBeenCalledWith(expect.objectContaining({ id: 'proj-2' }));
    expect(navigate).toHaveBeenCalledWith(expect.stringContaining('/mcp-proxy'));
  });
});
