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
import AgentProxiesDeploy from './AgentProxiesDeploy';

let deployments: { status: string }[] = [];
let proxy: Record<string, unknown> | null = {
  id: 'proxy-1',
  displayName: 'Trip Planner',
  a2a: { operationConfigs: { policies: [], operations: [] } },
};

vi.mock('../../../../contexts/agentProxy', () => ({
  AgentProxyProvider: ({ children }: { children: React.ReactNode }) => children,
  useAgentProxy: () => ({ agentProxy: proxy, isLoading: false, error: null }),
}));
vi.mock('../../../../contexts/GatewayDeployContext', () => ({
  GatewayDeployProvider: ({ children }: { children: React.ReactNode }) => children,
  useGatewayDeploy: () => ({ deployments: { list: deployments } }),
}));
vi.mock('../../../../Components/GatewayDeploy', () => ({
  GatewayDeployMainSection: () => <div>gateway deploy section</div>,
}));

const renderDeploy = (route = '/projects/project-one/agent-proxy/proxy-1/deploy') =>
  renderWithProviders(<AgentProxiesDeploy />, {
    route,
    path: '/projects/:projectSlug/agent-proxy/:agentProxyId/deploy',
  });

beforeEach(() => {
  deployments = [];
  proxy = {
    id: 'proxy-1',
    displayName: 'Trip Planner',
    a2a: { operationConfigs: { policies: [], operations: [] } },
  };
});

describe('the deploy page', () => {
  it('offers the gateway deployment section for the proxy', async () => {
    renderDeploy();

    await waitFor(() => expect(screen.getByText('gateway deploy section')).toBeInTheDocument());
  });

  it('names the proxy being deployed', async () => {
    renderDeploy();

    await waitFor(() => expect(screen.getAllByText(/Trip Planner/).length).toBeGreaterThan(0));
  });

  it('offers a way back to the proxy', async () => {
    renderDeploy();

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /Back to Agent Proxy/ })).toBeInTheDocument()
    );
  });

  it('still renders once the proxy carries a live deployment', async () => {
    deployments = [{ status: 'DEPLOYED' }];
    renderDeploy();

    await waitFor(() => expect(screen.getByText('gateway deploy section')).toBeInTheDocument());
  });

  it('still renders for a proxy that carries policies', async () => {
    proxy = {
      id: 'proxy-1',
      displayName: 'Trip Planner',
      a2a: { operationConfigs: { policies: [{ policyName: 'guard' }], operations: [] } },
    };
    renderDeploy();

    await waitFor(() => expect(screen.getByText('gateway deploy section')).toBeInTheDocument());
  });
});

describe('without an id in the route', () => {
  it('says the proxy id is required instead of rendering the page', () => {
    renderWithProviders(<AgentProxiesDeploy />, { route: '/', path: '/' });

    expect(screen.getByText('Agent proxy ID is required')).toBeInTheDocument();
  });
});
