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
import AgentProxiesNew from './AgentProxiesNew';
import { agentProxiesApis } from '../../../../apis/agent/agentProxiesApis';

vi.mock('../../../../apis/agent/agentProxiesApis', () => ({
  agentProxiesApis: { fetchAgentCard: vi.fn(), createAgentProxy: vi.fn() },
}));
vi.mock('../../../../contexts/AppShellContext', () => ({
  useAppShell: () => ({
    currentProject: { id: 'proj-1', name: 'Project One' },
    currentOrganization: { id: 'org-1', uuid: 'org-1', handle: 'acme' },
    projectsForCurrentOrganization: [{ id: 'proj-1', name: 'Project One' }],
  }),
}));
const createAgentProxy = vi.fn();
vi.mock('../../../../contexts/agentProxy', () => ({
  useAgentProxies: () => ({ createAgentProxy }),
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => true }),
}));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({
  default: () => vi.fn(),
}));

const cardWith = (interfaces: unknown[]) => ({
  name: 'Trip Planner',
  description: 'plans trips',
  version: '1.0.0',
  supportedInterfaces: interfaces,
});

/** Types the agent URL and runs the card probe. */
const probe = async (url: string) => {
  const view = renderWithProviders(<AgentProxiesNew />);
  await view.user.type(
    screen.getByPlaceholderText('Enter the URL of your A2A agent'),
    url
  );
  await view.user.click(screen.getByRole('button', { name: 'Fetch Agent Info' }));
  return view;
};

/** Probes, then advances to the step that shows the seeded transports. */
const probeThenContinue = async (url: string) => {
  const view = await probe(url);
  await waitFor(() => screen.getByRole('button', { name: 'Next' }));
  await view.user.click(screen.getByRole('button', { name: 'Next' }));
  return view;
};

beforeEach(() => {
  vi.mocked(agentProxiesApis.fetchAgentCard).mockReset();
  createAgentProxy.mockReset();
});

describe('seeding transports from the advertised card', () => {
  it('keeps only the part of the interface path that follows the agent url', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([
        { protocolBinding: 'JSONRPC', url: 'https://agent.test/v1.0/rpc' },
        { protocolBinding: 'HTTP+JSON', url: 'https://agent.test/v1.0/rest' },
      ]) as never
    );

    await probeThenContinue('https://agent.test/v1.0');

    await waitFor(() => expect(screen.getByText('/rpc')).toBeInTheDocument());
    expect(screen.getByText('/rest')).toBeInTheDocument();
  });

  it('uses the whole interface path when it does not sit under the agent url', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([{ protocolBinding: 'JSONRPC', url: 'https://agent.test/other/rpc' }]) as never
    );

    await probeThenContinue('https://agent.test/v1.0');

    await waitFor(() => expect(screen.getByText('/other/rpc')).toBeInTheDocument());
  });

  it('selects only the bindings the card actually advertises', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([
        { protocolBinding: 'JSONRPC', url: 'https://agent.test/rpc' },
        { protocolBinding: 'GRPC', url: 'https://agent.test/grpc' },
      ]) as never
    );

    await probeThenContinue('https://agent.test');

    await waitFor(() => expect(screen.getByText('/rpc')).toBeInTheDocument());
    expect(screen.queryByText('/grpc')).not.toBeInTheDocument();
  });

  it('leaves the defaults alone for an entry missing its binding or url', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([
        { url: 'https://agent.test/nameless' },
        { protocolBinding: 'JSONRPC' },
        null,
      ]) as never
    );

    await probeThenContinue('https://agent.test');

    await waitFor(() =>
      expect(screen.queryByText('/nameless')).not.toBeInTheDocument()
    );
  });

  it('ignores an interface whose url cannot be parsed', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([{ protocolBinding: 'JSONRPC', url: 'not-a-url' }]) as never
    );

    await probeThenContinue('https://agent.test');

    await waitFor(() => expect(screen.queryByText('not-a-url')).not.toBeInTheDocument());
  });

  it('tolerates a card with no interfaces at all', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      { name: 'Bare', description: '', version: '1' } as never
    );

    await probe('https://agent.test');

    await waitFor(() => expect(agentProxiesApis.fetchAgentCard).toHaveBeenCalled());
  });
});

describe('probing the agent url', () => {
  it('reports a malformed url without calling the server', async () => {
    await probe('not a url');

    expect(agentProxiesApis.fetchAgentCard).not.toHaveBeenCalled();
  });

  it('warns but does not block when the agent cannot be reached', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockRejectedValue(new Error('unreachable'));

    await probe('https://agent.test');

    await waitFor(() =>
      expect(screen.getByText(/Could not reach the upstream agent/)).toBeInTheDocument()
    );
  });
});

describe('the built-in sample agent', () => {
  it('fills the url and probes it without the user typing anything', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([{ protocolBinding: 'JSONRPC', url: 'https://sample.test/rpc' }]) as never
    );
    const view = renderWithProviders(<AgentProxiesNew />);

    await view.user.click(screen.getByRole('button', { name: 'Try with Sample Agent' }));

    await waitFor(() => expect(agentProxiesApis.fetchAgentCard).toHaveBeenCalled());
    const [request] = vi.mocked(agentProxiesApis.fetchAgentCard).mock.calls[0];
    expect((request as { url: string }).url).toMatch(/^https?:\/\//);
  });
});

describe('choosing transports', () => {
  it('selects every binding the card advertised', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([
        { protocolBinding: 'JSONRPC', url: 'https://agent.test/rpc' },
        { protocolBinding: 'HTTP+JSON', url: 'https://agent.test/rest' },
      ]) as never
    );
    await probeThenContinue('https://agent.test');

    await waitFor(() => expect(screen.getByText('/rpc')).toBeInTheDocument());
    expect(screen.getByText('/rest')).toBeInTheDocument();
    screen.getAllByRole('checkbox').forEach((box) => expect(box).toBeChecked());
  });

  it('adds a binding the card never advertised when it is turned on', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([{ protocolBinding: 'JSONRPC', url: 'https://agent.test/rpc' }]) as never
    );
    const view = await probeThenContinue('https://agent.test');
    await waitFor(() => screen.getByText('/rpc'));

    await view.user.click(screen.getByText('HTTP+JSON'));

    await waitFor(() => expect(screen.getByText('/rest')).toBeInTheDocument());
  });
});

describe('creating the proxy', () => {
  const fillAndSubmit = async (view: Awaited<ReturnType<typeof probeThenContinue>>) => {
    await view.user.type(
      screen.getByPlaceholderText('Trip Planning Agent'),
      'My Agent'
    );
    await view.user.click(screen.getByRole('button', { name: 'Create', exact: true }));
  };

  it('submits the named proxy to the control plane', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([{ protocolBinding: 'JSONRPC', url: 'https://agent.test/rpc' }]) as never
    );
    createAgentProxy.mockResolvedValue({ id: 'my-agent' });
    const view = await probeThenContinue('https://agent.test');
    await waitFor(() => screen.getByPlaceholderText('Trip Planning Agent'));

    await fillAndSubmit(view);

    await waitFor(() => expect(createAgentProxy).toHaveBeenCalled());
  });

  it('keeps the user on the form when the control plane refuses', async () => {
    vi.mocked(agentProxiesApis.fetchAgentCard).mockResolvedValue(
      cardWith([{ protocolBinding: 'JSONRPC', url: 'https://agent.test/rpc' }]) as never
    );
    createAgentProxy.mockRejectedValue(new Error('name already taken'));
    const view = await probeThenContinue('https://agent.test');
    await waitFor(() => screen.getByPlaceholderText('Trip Planning Agent'));

    await fillAndSubmit(view);

    await waitFor(() => expect(createAgentProxy).toHaveBeenCalled());
    expect(screen.getByPlaceholderText('Trip Planning Agent')).toBeInTheDocument();
  });
});
