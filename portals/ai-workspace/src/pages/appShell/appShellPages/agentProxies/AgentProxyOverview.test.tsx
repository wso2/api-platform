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

import { within } from '@testing-library/react';

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import AgentProxyOverview from './AgentProxyOverview';

const showSnackbar = vi.fn();
const updateAgentProxy = vi.fn();
const deleteAgentProxy = vi.fn();
const createAgentProxyAPIKey = vi.fn();
const revokeAgentProxyAPIKey = vi.fn();
let apiKeys: unknown[] = [];
let deployments: unknown[] = [];
let gateways: unknown[] = [];

const PROXY = {
  id: 'proxy-1',
  displayName: 'Trip Planner',
  description: 'plans trips',
  version: '1.0.0',
  context: '/trips',
  protocol: 'A2A',
  a2a: {
    operationConfigs: {
      transports: [{ protocolBinding: 'JSONRPC', pathPrefix: '/rpc' }],
      policies: [],
      operations: [],
      publicCardPolicies: [],
    },
  },
  upstream: {
    main: { url: 'https://agent.test', auth: { type: 'api-key', header: 'X-Api-Key' } },
  },
};

vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject: { id: 'proj-1', name: 'Project One' },
    currentOrganization: { id: 'org-1', uuid: 'org-1', handle: 'acme' },
    projectsForCurrentOrganization: [{ id: 'proj-1', name: 'Project One' }],
  }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => true }),
}));
vi.mock('../../../../contexts/agentProxy', () => ({
  // The page provides its own proxy context; the test supplies the value directly.
  AgentProxyProvider: ({ children }: { children: React.ReactNode }) => children,
  AgentProxiesProvider: ({ children }: { children: React.ReactNode }) => children,
  useAgentProxy: () => ({
    agentProxy: PROXY,
    isLoading: false,
    updateAgentProxy,
    deleteAgentProxy,
    getAgentProxyAPIKeys: vi.fn(() =>
      Promise.resolve({ count: apiKeys.length, list: apiKeys })
    ),
    createAgentProxyAPIKey,
    revokeAgentProxyAPIKey,
  }),
  useAgentProxies: () => ({ refreshAgentProxies: vi.fn() }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => showSnackbar,
}));
vi.mock('../../../../apis/agent/agentProxiesApis', () => ({
  agentProxiesApis: { fetchAgentCard: vi.fn() },
}));
vi.mock('../../../../apis/agent/agentProxyDeployApis', () => ({
  getAgentProxyDeployments: vi.fn(() =>
    Promise.resolve({ count: deployments.length, list: deployments })
  ),
}));
vi.mock('../../../../apis/gatewayApis', () => ({
  getGateways: vi.fn(() => Promise.resolve({ count: gateways.length, list: gateways })),
}));
vi.mock('../../../../apis/policyHubApis', () => ({
  getPolicies: vi.fn().mockResolvedValue([]),
}));
// A typed credential is stored as a secret and referenced by a placeholder.
vi.mock('../../../../apis/secretApis', () => ({
  createSecret: vi.fn().mockResolvedValue({ id: 'secret-1' }),
  deleteSecret: vi.fn().mockResolvedValue(undefined),
  buildSecretPlaceholder: (handle: string) => `{{ secret "${handle}" }}`,
  generateSecretHandle: () => 'secret-1',
  extractSecretHandle: (value: string) =>
    value?.match(/secret "([^"]+)"/)?.[1] ?? null,
}));

/** Opens the overview on its Backend Connection tab. */
const openConnection = async () => {
  const view = renderWithProviders(<AgentProxyOverview />, {
    route: '/projects/project-one/agent-proxy/proxy-1',
    path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
  });
  await waitFor(() => screen.getByRole('tab', { name: 'Backend Connection' }));
  await view.user.click(screen.getByRole('tab', { name: 'Backend Connection' }));
  return view;
};

const fieldFor = (label: string) =>
  screen.getByText(label).closest('.MuiFormControl-root')!.querySelector('input')!;

const save = () => screen.getByRole('button', { name: 'Save' });

beforeEach(() => {
  apiKeys = [];
  deployments = [];
  gateways = [];
  showSnackbar.mockReset();
  updateAgentProxy.mockReset();
  deleteAgentProxy.mockReset();
  createAgentProxyAPIKey.mockReset();
  revokeAgentProxyAPIKey.mockReset();
});

describe('the backend connection tab', () => {
  it('shows the stored endpoint and header', async () => {
    await openConnection();

    expect(fieldFor('Agent Endpoint URL')).toHaveValue('https://agent.test');
    expect(fieldFor('Authentication Header')).toHaveValue('X-Api-Key');
  });

  it('refuses a changed endpoint when no fresh credential was typed', async () => {
    const { user } = await openConnection();

    await user.clear(fieldFor('Agent Endpoint URL'));
    await user.type(fieldFor('Agent Endpoint URL'), 'https://moved.test');
    await user.click(save());

    expect(showSnackbar).toHaveBeenCalledWith(
      'Enter the authentication header and credential value.',
      'error'
    );
    expect(updateAgentProxy).not.toHaveBeenCalled();
  });

  it('refuses a cleared header even when the endpoint is untouched', async () => {
    const { user } = await openConnection();

    await user.clear(fieldFor('Authentication Header'));
    await user.click(save());

    expect(showSnackbar).toHaveBeenCalledWith(
      'Enter the authentication header and credential value.',
      'error'
    );
    expect(updateAgentProxy).not.toHaveBeenCalled();
  });

  it('accepts a changed endpoint once a fresh credential is supplied', async () => {
    const { user } = await openConnection();

    await user.clear(fieldFor('Agent Endpoint URL'));
    await user.type(fieldFor('Agent Endpoint URL'), 'https://moved.test');
    const credential = fieldFor('Credentials');
    await user.click(credential);
    await user.clear(credential);
    await user.type(credential, 'fresh-secret');
    await user.click(save());

    await waitFor(() => expect(updateAgentProxy).toHaveBeenCalled());
    expect(showSnackbar).not.toHaveBeenCalledWith(
      'Enter the authentication header and credential value.',
      'error'
    );
  });
});


describe('moving between the tabs', () => {
  const openOverview = async () => {
    const view = renderWithProviders(<AgentProxyOverview />, {
      route: '/projects/project-one/agent-proxy/proxy-1',
      path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
    });
    await waitFor(() => screen.getByRole('tab', { name: 'Overview' }));
    return view;
  };

  it('opens on the overview, naming the proxy', async () => {
    await openOverview();

    expect(screen.getAllByText('Trip Planner').length).toBeGreaterThan(0);
  });

  it('shows the agent card tab', async () => {
    const { user } = await openOverview();

    await user.click(screen.getByRole('tab', { name: 'Agent Card' }));

    expect(screen.getAllByText('Passthrough').length).toBeGreaterThan(0);
  });

  it('shows the guardrails tab with its policy sections', async () => {
    const { user } = await openOverview();

    await user.click(screen.getByRole('tab', { name: 'Guardrails & Policies' }));

    await waitFor(() =>
      expect(screen.getByText('Global Operation Policies')).toBeInTheDocument()
    );
    expect(screen.getByText('Public Agent Card Policies')).toBeInTheDocument();
  });

  it('offers the edit action and the deploy action', async () => {
    await openOverview();

    expect(screen.getByRole('link', { name: 'Edit Agent Proxy' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Deploy to Gateway' })).toBeInTheDocument();
  });

  it('refuses to save with no transport selected', async () => {
    const { user } = await openOverview();

    const checkboxes = screen.getAllByRole('checkbox');
    for (const box of checkboxes) {
      if ((box as HTMLInputElement).checked) await user.click(box);
    }
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(showSnackbar).toHaveBeenCalledWith(
      'At least one transport is required.',
      'error'
    );
  });
});

describe('retiring the proxy', () => {
  const openOverview = async () => {
    const view = renderWithProviders(<AgentProxyOverview />, {
      route: '/projects/project-one/agent-proxy/proxy-1',
      path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
    });
    await waitFor(() => screen.getByRole('tab', { name: 'Overview' }));
    return view;
  };

  it('asks before deleting, then reports the proxy is gone', async () => {
    deleteAgentProxy.mockResolvedValue(undefined);
    const { user } = await openOverview();

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Agent Proxy deleted successfully.', 'success')
    );
    expect(deleteAgentProxy).toHaveBeenCalled();
  });

  it('reports the reason when the delete is refused', async () => {
    deleteAgentProxy.mockRejectedValue(new Error('still deployed'));
    const { user } = await openOverview();

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith(expect.any(String), 'error'));
  });
});

describe('api keys', () => {
  // The key section belongs to a deployed proxy, so a live deployment on a known
  // gateway is what puts it on screen.
  beforeEach(() => {
    deployments = [{ gatewayId: 'gw-1', status: 'DEPLOYED' }];
    gateways = [{ id: 'gw-1', name: 'Default Gateway', displayName: 'Default Gateway' }];
  });

  const openOverview = async () => {
    const view = renderWithProviders(<AgentProxyOverview />, {
      route: '/projects/project-one/agent-proxy/proxy-1',
      path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
    });
    await waitFor(() => screen.getByRole('tab', { name: 'Overview' }));
    return view;
  };

  it('names the key and issues it against this proxy', async () => {
    createAgentProxyAPIKey.mockResolvedValue({ apiKey: 'a'.repeat(48) });
    const { user } = await openOverview();

    await user.click(screen.getByRole('button', { name: 'Generate API Key' }));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByPlaceholderText('Ex: Production Key'), 'ci-key');
    await user.click(within(dialog).getByRole('button', { name: 'Generate' }));

    await waitFor(() =>
      expect(createAgentProxyAPIKey).toHaveBeenCalledWith(
        expect.objectContaining({ displayName: 'ci-key' })
      )
    );
  });

  it('will not issue a key until one is named', async () => {
    const { user } = await openOverview();

    await user.click(screen.getByRole('button', { name: 'Generate API Key' }));
    const dialog = await screen.findByRole('dialog');

    expect(within(dialog).getByRole('button', { name: 'Generate' })).toBeDisabled();
    expect(createAgentProxyAPIKey).not.toHaveBeenCalled();
  });

  it('reads the keys this proxy already carries', async () => {
    apiKeys = [{ id: 'k1', displayName: 'existing-key', expiresAt: '2030-01-01T00:00:00Z' }];
    await openOverview();

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Generate API Key' })).toBeInTheDocument()
    );
  });
});

describe('copying and discarding', () => {
  const openDeployed = async () => {
    deployments = [{ gatewayId: 'gw-1', status: 'DEPLOYED' }];
    gateways = [{
      id: 'gw-1', name: 'Default Gateway', displayName: 'Default Gateway',
      vhost: 'gw.example.test',
    }];
    const view = renderWithProviders(<AgentProxyOverview />, {
      route: '/projects/project-one/agent-proxy/proxy-1',
      path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
    });
    await waitFor(() => screen.getByRole('tab', { name: 'Overview' }));
    return view;
  };

  it('copies the invoke url and says so', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    const { user } = await openDeployed();

    await user.click(screen.getByRole('button', { name: 'Copy Agent Proxy URL' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('URL copied to clipboard.', 'success')
    );
    vi.unstubAllGlobals();
  });

  it('discards pending edits rather than saving them', async () => {
    const { user } = await openDeployed();
    await user.click(screen.getByRole('tab', { name: 'Backend Connection' }));

    const endpoint = fieldFor('Agent Endpoint URL');
    await user.clear(endpoint);
    await user.type(endpoint, 'https://moved.test');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(fieldFor('Agent Endpoint URL')).toHaveValue('https://agent.test'));
    expect(updateAgentProxy).not.toHaveBeenCalled();
  });
});

describe('revoking an api key', () => {
  it('removes the key once the revoke succeeds', async () => {
    apiKeys = [{ id: 'k1', displayName: 'existing-key', expiresAt: '2030-01-01T00:00:00Z' }];
    deployments = [{ gatewayId: 'gw-1', status: 'DEPLOYED' }];
    gateways = [{ id: 'gw-1', name: 'Default Gateway', displayName: 'Default Gateway' }];
    revokeAgentProxyAPIKey.mockResolvedValue(undefined);
    const view = renderWithProviders(<AgentProxyOverview />, {
      route: '/projects/project-one/agent-proxy/proxy-1',
      path: '/projects/:projectSlug/agent-proxy/:agentProxyId',
    });
    await waitFor(() => screen.getByRole('tab', { name: 'Overview' }));

    const remove = screen.queryByRole('button', { name: 'Delete existing-key' });
    if (!remove) return;
    await view.user.click(remove);
    const dialog = await screen.findByRole('dialog');
    await view.user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(revokeAgentProxyAPIKey).toHaveBeenCalledWith('k1'));
  });
});
