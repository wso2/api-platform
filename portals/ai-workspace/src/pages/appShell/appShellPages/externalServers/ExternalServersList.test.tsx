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
import ExternalServersList from './ExternalServersList';

const showSnackbar = vi.fn();
const getMCPServers = vi.fn();
const deleteMCPServer = vi.fn();
let permitted = true;
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
vi.mock('../../../../hooks/useResourceLimits', () => ({
  useResourceLimits: () => ({ canCreate: () => true, limitMessage: () => '' }),
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
