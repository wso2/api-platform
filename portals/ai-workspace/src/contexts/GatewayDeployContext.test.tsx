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
import { renderHook, waitFor, act } from '@testing-library/react';

import { getGateways } from '../apis/gatewayApis';
import * as agentDeploy from '../apis/agent/agentProxyDeployApis';
import { GatewayDeployProvider, useGatewayDeploy } from './GatewayDeployContext';

vi.mock('../apis/gatewayApis', () => ({ getGateways: vi.fn() }));
vi.mock('../apis/agent/agentProxyDeployApis');
vi.mock('../apis/llmProviderApis');
vi.mock('../apis/llmProxiesApis');
vi.mock('../apis/MCP/mcpServerDeployApis');
vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));
vi.mock('./AppShellContext', () => ({
  useAppShell: () => ({ currentOrganization: { uuid: 'org-1' } }),
}));

let permitted = true;
vi.mock('./AppAuthContext', () => ({
  useAppAuth: () => ({ hasPermission: () => permitted }),
}));

const GATEWAY = { id: 'gw-1', name: 'default', displayName: 'Default Gateway' };

const renderDeploy = (readOnly = false) =>
  renderHook(() => useGatewayDeploy(), {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <GatewayDeployProvider apiId="proxy-1" resourceType="agent-proxy" readOnly={readOnly}>
        {children}
      </GatewayDeployProvider>
    ),
  });

const settled = async (result: { current: { isLoading: boolean } }) =>
  waitFor(() => expect(result.current.isLoading).toBe(false));

beforeEach(() => {
  permitted = true;
  vi.mocked(getGateways).mockResolvedValue({ count: 1, list: [GATEWAY] } as never);
  vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
    count: 0, list: [],
  } as never);
  vi.mocked(agentDeploy.getAgentProxyDeployment).mockResolvedValue({
    id: 'dep-1', status: 'DEPLOYED',
  } as never);
  vi.mocked(agentDeploy.deployAgentProxy).mockResolvedValue({ id: 'dep-1' } as never);
  vi.mocked(agentDeploy.undeployAgentProxyDeployment).mockResolvedValue({ id: 'dep-1' } as never);
  vi.mocked(agentDeploy.restoreAgentProxyDeployment).mockResolvedValue({ id: 'dep-1' } as never);
  vi.mocked(agentDeploy.deleteAgentProxyDeployment).mockResolvedValue(undefined as never);
});

describe('an agent proxy in the deploy context', () => {
  it('loads the gateways it could be deployed to', async () => {
    const { result } = renderDeploy();

    await settled(result);
    expect(result.current.gateways).toHaveLength(1);
  });

  it('reads deployments through the agent proxy endpoint, not another kind', async () => {
    const { result } = renderDeploy();

    await settled(result);
    await waitFor(() =>
      expect(agentDeploy.getAgentProxyDeployments).toHaveBeenCalled()
    );
  });

  it('deploys through the agent proxy endpoint', async () => {
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.deployToGateway('gw-1', 'host.test'); });

    expect(agentDeploy.deployAgentProxy).toHaveBeenCalledWith(
      'proxy-1', expect.anything(), expect.any(String)
    );
  });

  it('undeploys through the agent proxy endpoint', async () => {
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.undeployDeployment('dep-1', 'gw-1'); });

    expect(agentDeploy.undeployAgentProxyDeployment).toHaveBeenCalled();
  });

  it('redeploys through the agent proxy endpoint', async () => {
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.redeployDeployment('dep-1', 'gw-1'); });

    expect(agentDeploy.restoreAgentProxyDeployment).toHaveBeenCalled();
  });

  it('deletes a deployment through the agent proxy endpoint', async () => {
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.deleteDeployment('dep-1'); });

    expect(agentDeploy.deleteAgentProxyDeployment).toHaveBeenCalled();
  });

  it('records a failure to list gateways rather than leaving the caller loading', async () => {
    vi.mocked(getGateways).mockRejectedValue(new Error('control plane down'));
    const { result } = renderDeploy();

    await settled(result);
    expect(result.current.error).not.toBeNull();
  });
});

describe('who may deploy', () => {
  it('withholds the write actions from a caller without the scope', async () => {
    permitted = false;
    const { result } = renderDeploy();

    await settled(result);
    expect(result.current.canDeploy).toBe(false);
    expect(result.current.readOnly).toBe(true);
  });

  it('treats a gateway-owned artifact as read only even for a permitted caller', async () => {
    const { result } = renderDeploy(true);

    await settled(result);
    expect(result.current.readOnly).toBe(true);
  });
});

describe('naming a new deployment', () => {
  const today = new Date().toISOString().slice(0, 10);

  it('continues the numbering already used for today', async () => {
    vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
      count: 2,
      list: [
        { deploymentId: 'd1', gatewayId: 'gw-1', status: 'DEPLOYED', name: `proxy-1_${today}_1` },
        { deploymentId: 'd2', gatewayId: 'gw-1', status: 'DEPLOYED', name: `proxy-1_${today}_2` },
      ],
    } as never);
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.deployToGateway('gw-1', 'host.test'); });

    const [, body] = vi.mocked(agentDeploy.deployAgentProxy).mock.calls[0];
    expect(JSON.stringify(body)).toContain(`${today}_3`);
  });

  it('starts from one when nothing was deployed today', async () => {
    vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
      count: 1,
      list: [{ deploymentId: 'd1', gatewayId: 'gw-1', status: 'DEPLOYED', name: 'proxy-1_2020-01-01_7' }],
    } as never);
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.deployToGateway('gw-1', 'host.test'); });

    const [, body] = vi.mocked(agentDeploy.deployAgentProxy).mock.calls[0];
    expect(JSON.stringify(body)).toContain(`${today}_1`);
  });

  it('ignores a deployment whose name carries no number', async () => {
    vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
      count: 1,
      list: [{ deploymentId: 'd1', gatewayId: 'gw-1', status: 'DEPLOYED', name: 'hand-named' }],
    } as never);
    const { result } = renderDeploy();
    await settled(result);

    await act(async () => { await result.current.deployToGateway('gw-1', 'host.test'); });

    expect(agentDeploy.deployAgentProxy).toHaveBeenCalled();
  });
});

describe('a deployment still settling', () => {
  it('is watched until its status stops being transitional', async () => {
    vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
      count: 1,
      list: [{ deploymentId: 'dep-1', gatewayId: 'gw-1', status: 'DEPLOYING', name: 'proxy-1_x_1' }],
    } as never);

    const { result } = renderDeploy();
    await settled(result);

    await waitFor(() =>
      expect(agentDeploy.getAgentProxyDeployment).toHaveBeenCalledWith(
        'proxy-1', 'dep-1', expect.any(String)
      )
    );
  });

  it('reports the gateway as still settling while it is watched', async () => {
    vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
      count: 1,
      list: [{ deploymentId: 'dep-2', gatewayId: 'gw-1', status: 'UNDEPLOYING', name: 'proxy-1_x_2' }],
    } as never);

    const { result } = renderDeploy();
    await settled(result);

    await waitFor(() => expect(agentDeploy.getAgentProxyDeployment).toHaveBeenCalled());
  });

  it('keeps watching when a status read fails rather than giving up at once', async () => {
    vi.mocked(agentDeploy.getAgentProxyDeployments).mockResolvedValue({
      count: 1,
      list: [{ deploymentId: 'dep-3', gatewayId: 'gw-1', status: 'DEPLOYING', name: 'proxy-1_x_3' }],
    } as never);
    vi.mocked(agentDeploy.getAgentProxyDeployment).mockRejectedValue(new Error('blip'));

    const { result } = renderDeploy();
    await settled(result);

    await waitFor(() => expect(agentDeploy.getAgentProxyDeployment).toHaveBeenCalled());
    expect(result.current.error).toBeNull();
  });
});
