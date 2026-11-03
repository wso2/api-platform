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
import EditAgentProxy from './EditAgentProxy';

const showSnackbar = vi.fn();
const updateAgentProxy = vi.fn();

const PROXY = {
  id: 'proxy-1',
  displayName: 'Trip Planner',
  description: 'plans trips',
  context: '/trips',
  version: '1.0.0',
};

vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject: { id: 'proj-1', name: 'Project One' },
    currentOrganization: { id: 'org-1', uuid: 'org-1', handle: 'acme' },
    projectsForCurrentOrganization: [{ id: 'proj-1', name: 'Project One' }],
  }),
}));
vi.mock('../../../../contexts/agentProxy', () => ({
  AgentProxyProvider: ({ children }: { children: React.ReactNode }) => children,
  useAgentProxy: () => ({ agentProxy: PROXY, isLoading: false, updateAgentProxy }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));

const renderEdit = (route = '/projects/project-one/agent-proxy/proxy-1') =>
  renderWithProviders(<EditAgentProxy />, {
    route,
    path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
  });

const fieldFor = (label: string) =>
  screen.getByText(label).closest('.MuiFormControl-root')!.querySelector('input, textarea')!;

beforeEach(() => {
  showSnackbar.mockReset();
  updateAgentProxy.mockReset();
});

describe('the edit form', () => {
  it('opens seeded with what is stored', async () => {
    renderEdit();

    await waitFor(() => expect(fieldFor('Name')).toHaveValue('Trip Planner'));
    expect(fieldFor('Description')).toHaveValue('plans trips');
  });

  it('saves the edited name and says so', async () => {
    updateAgentProxy.mockResolvedValue(PROXY);
    const { user } = renderEdit();
    await waitFor(() => screen.getByText('Name'));

    await user.clear(fieldFor('Name'));
    await user.type(fieldFor('Name'), 'Renamed');
    await user.click(screen.getByRole('button', { name: 'Update' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Agent Proxy updated successfully', 'success')
    );
    expect(updateAgentProxy).toHaveBeenCalledWith(
      expect.objectContaining({ displayName: 'Renamed' })
    );
  });

  it('reports a refused save rather than claiming success', async () => {
    updateAgentProxy.mockRejectedValue(new Error('name already taken'));
    const { user } = renderEdit();
    await waitFor(() => screen.getByText('Name'));

    await user.clear(fieldFor('Name'));
    await user.type(fieldFor('Name'), 'Renamed');
    await user.click(screen.getByRole('button', { name: 'Update' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith(expect.any(String), 'error')
    );
  });
});

describe('without an id in the route', () => {
  it('says the proxy id is missing instead of rendering a form', () => {
    renderWithProviders(<EditAgentProxy />, { route: '/', path: '/' });

    expect(screen.getByText('Agent Proxy ID is missing')).toBeInTheDocument();
  });
});

describe('leaving the form', () => {
  it('abandons the edit without saving', async () => {
    const { user } = renderEdit();
    await waitFor(() => screen.getByText('Name'));

    await user.clear(fieldFor('Name'));
    await user.type(fieldFor('Name'), 'Abandoned');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(updateAgentProxy).not.toHaveBeenCalled();
  });

  it('will not submit an empty name', async () => {
    const { user } = renderEdit();
    await waitFor(() => screen.getByText('Name'));

    await user.clear(fieldFor('Name'));

    expect(screen.getByRole('button', { name: 'Update' })).toBeDisabled();
  });

  it('carries the edited description through to the update', async () => {
    updateAgentProxy.mockResolvedValue(PROXY);
    const { user } = renderEdit();
    await waitFor(() => screen.getByText('Description'));

    await user.clear(fieldFor('Description'));
    await user.type(fieldFor('Description'), 'now plans cities too');
    await user.click(screen.getByRole('button', { name: 'Update' }));

    await waitFor(() =>
      expect(updateAgentProxy).toHaveBeenCalledWith(
        expect.objectContaining({ description: 'now plans cities too' })
      )
    );
  });
});
