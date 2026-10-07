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

import * as agentProxiesApis from '../../apis/agent/agentProxiesApis';
import { AgentProxyProvider, useAgentProxy } from './AgentProxyContext';

vi.mock('../../apis/agent/agentProxiesApis');
vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const PROXY = { id: 'proxy-1', displayName: 'Trips' };

const renderProxy = (agentProxyId = 'proxy-1') =>
  renderHook(() => useAgentProxy(), {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <AgentProxyProvider agentProxyId={agentProxyId}>{children}</AgentProxyProvider>
    ),
  });

const settled = async (result: { current: { isLoading: boolean } }) =>
  waitFor(() => expect(result.current.isLoading).toBe(false));

beforeEach(() => {
  vi.mocked(agentProxiesApis.getAgentProxy).mockResolvedValue(PROXY as never);
  vi.mocked(agentProxiesApis.updateAgentProxy).mockReset();
  vi.mocked(agentProxiesApis.deleteAgentProxy).mockReset();
  vi.mocked(agentProxiesApis.getAgentProxyAPIKeys).mockReset();
  vi.mocked(agentProxiesApis.createAgentProxyAPIKey).mockReset();
  vi.mocked(agentProxiesApis.revokeAgentProxyAPIKey).mockReset();
});

describe('loading one proxy', () => {
  it('fetches the proxy named by the provider', async () => {
    const { result } = renderProxy();

    await settled(result);
    expect(result.current.agentProxy).toEqual(PROXY);
    expect(agentProxiesApis.getAgentProxy).toHaveBeenCalledWith('proxy-1', expect.any(String));
  });

  it('holds nothing and stops loading when no id is given', async () => {
    const { result } = renderProxy('');

    await settled(result);
    expect(result.current.agentProxy).toBeNull();
    expect(result.current.error).toBeNull();
    expect(agentProxiesApis.getAgentProxy).not.toHaveBeenCalled();
  });

  it('clears the proxy as well as recording the failure', async () => {
    vi.mocked(agentProxiesApis.getAgentProxy).mockRejectedValue(new Error('not found'));

    const { result } = renderProxy();

    await settled(result);
    expect(result.current.error?.message).toBe('not found');
    expect(result.current.agentProxy).toBeNull();
  });

  it('lets a response that lost the race be discarded rather than written', async () => {
    let releaseFirst: (value: unknown) => void = () => {};
    vi.mocked(agentProxiesApis.getAgentProxy)
      .mockImplementationOnce(
        () => new Promise((resolve) => { releaseFirst = resolve; }) as never
      )
      .mockResolvedValue({ id: 'proxy-1', displayName: 'Fresh' } as never);

    const { result } = renderProxy();
    await act(async () => { await result.current.refetch(); });

    await act(async () => {
      releaseFirst({ id: 'proxy-1', displayName: 'Stale' });
      await Promise.resolve();
    });

    expect(result.current.agentProxy?.displayName).toBe('Fresh');
  });

  it('exposes a local setter that changes state without calling the API', async () => {
    const { result } = renderProxy();
    await settled(result);

    act(() => { result.current.setLocalAgentProxy({ ...PROXY, displayName: 'Edited' } as never); });

    expect(result.current.agentProxy?.displayName).toBe('Edited');
    expect(agentProxiesApis.updateAgentProxy).not.toHaveBeenCalled();
  });
});

describe('mutating the proxy', () => {
  it('stores what the update returned', async () => {
    vi.mocked(agentProxiesApis.updateAgentProxy).mockResolvedValue({
      ...PROXY, displayName: 'Renamed',
    } as never);

    const { result } = renderProxy();
    await settled(result);

    await act(async () => { await result.current.updateAgentProxy({} as never); });

    expect(result.current.agentProxy?.displayName).toBe('Renamed');
  });

  it('leaves nothing behind once the proxy is deleted', async () => {
    vi.mocked(agentProxiesApis.deleteAgentProxy).mockResolvedValue(undefined as never);

    const { result } = renderProxy();
    await settled(result);

    await act(async () => { await result.current.deleteAgentProxy(); });

    expect(result.current.agentProxy).toBeNull();
  });

  it('reads the keys belonging to this proxy', async () => {
    vi.mocked(agentProxiesApis.getAgentProxyAPIKeys).mockResolvedValue({
      count: 0, list: [],
    } as never);

    const { result } = renderProxy();
    await settled(result);

    await act(async () => { await result.current.getAgentProxyAPIKeys(); });

    expect(agentProxiesApis.getAgentProxyAPIKeys).toHaveBeenCalledWith(
      'proxy-1', expect.any(String)
    );
  });

  it('creates a key against this proxy', async () => {
    vi.mocked(agentProxiesApis.createAgentProxyAPIKey).mockResolvedValue({
      apiKey: 'secret',
    } as never);

    const { result } = renderProxy();
    await settled(result);

    await act(async () => { await result.current.createAgentProxyAPIKey({} as never); });

    expect(agentProxiesApis.createAgentProxyAPIKey).toHaveBeenCalledWith(
      'proxy-1', {}, expect.any(String)
    );
  });

  it('revokes a key against this proxy', async () => {
    vi.mocked(agentProxiesApis.revokeAgentProxyAPIKey).mockResolvedValue(undefined as never);

    const { result } = renderProxy();
    await settled(result);

    await act(async () => { await result.current.revokeAgentProxyAPIKey('key-1'); });

    expect(agentProxiesApis.revokeAgentProxyAPIKey).toHaveBeenCalledWith(
      'proxy-1', 'key-1', expect.any(String)
    );
  });
});

describe('without an id', () => {
  it.each([
    ['updateAgentProxy', (c: ReturnType<typeof useAgentProxy>) => c.updateAgentProxy({} as never)],
    ['deleteAgentProxy', (c: ReturnType<typeof useAgentProxy>) => c.deleteAgentProxy()],
    ['getAgentProxyAPIKeys', (c: ReturnType<typeof useAgentProxy>) => c.getAgentProxyAPIKeys()],
    ['createAgentProxyAPIKey', (c: ReturnType<typeof useAgentProxy>) => c.createAgentProxyAPIKey({} as never)],
    ['revokeAgentProxyAPIKey', (c: ReturnType<typeof useAgentProxy>) => c.revokeAgentProxyAPIKey('k')],
  ])('%s refuses rather than calling the API with an empty id', async (_label, call) => {
    const { result } = renderProxy('');
    await settled(result);

    await expect(call(result.current)).rejects.toThrow('Agent proxy ID is missing');
  });
});
