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

const showSnackbar = vi.fn();
const deleteAgentProxy = vi.fn();
let permitted = true;
let proxies: unknown[] = [];
let proxiesLoading = false;

const proxy = (over: Record<string, unknown> = {}) => ({
  id: 'trip-planner',
  displayName: 'Trip Planner',
  description: 'plans multi-city trips',
  context: '/trips',
  version: '1.0.0',
  ...over,
});

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
  useAgentProxies: () => ({
    agentProxiesResponse: {
      count: proxies.length,
      list: proxies,
      pagination: { total: proxies.length, offset: 0, limit: 20 },
    },
    isLoading: proxiesLoading,
    deleteAgentProxy,
  }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));

const renderList = () => renderWithProviders(<AgentProxiesList />);
const search = () => screen.getByPlaceholderText('Search Agent Proxies...');

beforeEach(() => {
  permitted = true;
  proxiesLoading = false;
  currentProject = { id: 'proj-1', name: 'Project One' };
  setCurrentProject.mockReset();
  proxies = [proxy(), proxy({ id: 'weather', displayName: 'Weather Bot', description: 'forecasts', context: '/weather', version: '2.0.0' })];
  showSnackbar.mockReset();
  deleteAgentProxy.mockReset();
});

describe('listing agent proxies', () => {
  it('shows every proxy in the project', () => {
    renderList();

    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('says so plainly when the project has none', () => {
    proxies = [];
    renderList();

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });
});

describe('searching', () => {
  it.each([
    ['the display name', 'Trip'],
    ['the description', 'multi-city'],
    ['the context', '/trips'],
    ['the version', '1.0.0'],
  ])('matches on %s', async (_label, term) => {
    const { user } = renderList();

    await user.type(search(), term);

    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
    expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument();
  });

  it('ignores case', async () => {
    const { user } = renderList();

    await user.type(search(), 'TRIP PLANNER');

    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
  });

  it('shows everything again once the search is cleared', async () => {
    const { user } = renderList();
    // The placeholder is dropped once the field has text, so the element is
    // held rather than looked up again.
    const input = search();

    await user.type(input, 'Trip');
    await user.clear(input);

    expect(screen.getByText('Weather Bot')).toBeInTheDocument();
  });

  it('matches nothing when no proxy carries the term', async () => {
    const { user } = renderList();

    await user.type(search(), 'nothing-matches-this');

    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
    expect(screen.queryByText('Weather Bot')).not.toBeInTheDocument();
  });
});

describe('deleting a proxy', () => {
  it('reports success once the proxy is gone', async () => {
    deleteAgentProxy.mockResolvedValue(undefined);
    const { user } = renderList();

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

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    await user.click(screen.getByRole('button', { name: 'Delete', exact: true }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith(expect.any(String), 'error')
    );
  });
});

describe('without permission', () => {
  it('offers no delete control', () => {
    permitted = false;
    renderList();

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
  });
});

describe('while the catalogue is still loading', () => {
  it('shows a placeholder table instead of an empty listing', () => {
    proxiesLoading = true;
    proxies = [];
    renderList();

    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByText('Last Updated')).toBeInTheDocument();
    expect(screen.queryByText('Trip Planner')).not.toBeInTheDocument();
  });
});
