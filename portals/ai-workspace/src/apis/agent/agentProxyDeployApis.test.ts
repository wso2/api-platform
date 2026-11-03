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

import { describe, it, expect, vi, beforeEach } from 'vitest';

import { get, post, del } from '../../clients/choreoApiClient';
import {
  deployAgentProxy,
  getAgentProxyDeployments,
  getAgentProxyDeployment,
  deleteAgentProxyDeployment,
  undeployAgentProxyDeployment,
  restoreAgentProxyDeployment,
} from './agentProxyDeployApis';

vi.mock('../../clients/choreoApiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
}));

vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const BASE = 'https://control-plane.test';

beforeEach(() => {
  vi.mocked(get).mockReset();
  vi.mocked(post).mockReset();
  vi.mocked(del).mockReset();
});

describe('deploying an agent proxy', () => {
  it('posts the deployment under the owning proxy', async () => {
    const deployment = { id: 'dep-1' };
    vi.mocked(post).mockResolvedValue(deployment);

    const result = await deployAgentProxy('proxy-1', { gatewayId: 'gw-1' } as never, BASE);

    expect(post).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments',
      { gatewayId: 'gw-1' },
      BASE
    );
    expect(result).toBe(deployment);
  });

  it('reads one deployment by its own id', async () => {
    vi.mocked(get).mockResolvedValue({ id: 'dep-1' });

    await getAgentProxyDeployment('proxy-1', 'dep-1', BASE);

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments/dep-1',
      undefined,
      BASE
    );
  });

  it('deletes one deployment by its own id', async () => {
    vi.mocked(del).mockResolvedValue(undefined);

    await deleteAgentProxyDeployment('proxy-1', 'dep-1', BASE);

    expect(del).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments/dep-1',
      undefined,
      BASE
    );
  });
});

describe('listing deployments', () => {
  beforeEach(() => {
    vi.mocked(get).mockResolvedValue({ list: [], count: 0 });
  });

  it('omits the query entirely when no filter is given', async () => {
    await getAgentProxyDeployments('proxy-1', BASE);

    expect(get).toHaveBeenCalledWith('/agent-proxies/proxy-1/deployments', undefined, BASE);
  });

  it('filters by gateway alone', async () => {
    await getAgentProxyDeployments('proxy-1', BASE, 'gw-1');

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments?gatewayId=gw-1',
      undefined,
      BASE
    );
  });

  it('filters by status alone', async () => {
    await getAgentProxyDeployments('proxy-1', BASE, undefined, 'DEPLOYED');

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments?status=DEPLOYED',
      undefined,
      BASE
    );
  });

  it('carries both filters when both are given', async () => {
    await getAgentProxyDeployments('proxy-1', BASE, 'gw-1', 'DEPLOYED');

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments?gatewayId=gw-1&status=DEPLOYED',
      undefined,
      BASE
    );
  });

  it('treats an empty filter as absent rather than as a blank query', async () => {
    await getAgentProxyDeployments('proxy-1', BASE, '', '');

    expect(get).toHaveBeenCalledWith('/agent-proxies/proxy-1/deployments', undefined, BASE);
  });

  it('escapes a gateway id that would otherwise alter the query', async () => {
    await getAgentProxyDeployments('proxy-1', BASE, 'gw 1&x=2');

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments?gatewayId=gw+1%26x%3D2',
      undefined,
      BASE
    );
  });
});

describe('undeploy and restore', () => {
  it('names the gateway the deployment is being withdrawn from', async () => {
    vi.mocked(post).mockResolvedValue({ id: 'dep-1' });

    await undeployAgentProxyDeployment('proxy-1', 'dep-1', BASE, 'gw-1');

    expect(post).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments/dep-1/undeploy?gatewayId=gw-1',
      {},
      BASE
    );
  });

  it('names the gateway the deployment is being restored to', async () => {
    vi.mocked(post).mockResolvedValue({ id: 'dep-1' });

    await restoreAgentProxyDeployment('proxy-1', 'dep-1', BASE, 'gw-1');

    expect(post).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/deployments/dep-1/restore?gatewayId=gw-1',
      {},
      BASE
    );
  });
});

describe('transport failures', () => {
  const failure = new Error('gateway unreachable');

  it.each([
    ['deployAgentProxy', () => deployAgentProxy('a', {} as never, BASE), post],
    ['getAgentProxyDeployments', () => getAgentProxyDeployments('a', BASE), get],
    ['getAgentProxyDeployment', () => getAgentProxyDeployment('a', 'd', BASE), get],
    ['deleteAgentProxyDeployment', () => deleteAgentProxyDeployment('a', 'd', BASE), del],
    ['undeployAgentProxyDeployment', () => undeployAgentProxyDeployment('a', 'd', BASE, 'g'), post],
    ['restoreAgentProxyDeployment', () => restoreAgentProxyDeployment('a', 'd', BASE, 'g'), post],
  ])('%s surfaces the failure to its caller', async (_name, call, client) => {
    vi.mocked(client).mockRejectedValue(failure);

    await expect(call()).rejects.toThrow('gateway unreachable');
  });
});
