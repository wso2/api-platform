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
import { AgentProxiesProvider, useAgentProxies } from './AgentProxiesContext';

vi.mock('../../apis/agent/agentProxiesApis');
vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

// The provider reads the active project from the app shell, so the test drives it
// through this value rather than rendering the shell itself.
let currentProject: { id: string } | null = { id: 'proj-1' };
vi.mock('../AppShellContext', () => ({
  useAppShell: () => ({ currentProject }),
}));

const listOf = (ids: string[]) => ({
  count: ids.length,
  list: ids.map((id) => ({ id, displayName: id })),
  pagination: { total: ids.length, offset: 0, limit: 20 },
});

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <AgentProxiesProvider>{children}</AgentProxiesProvider>
);

const renderProxies = () => renderHook(() => useAgentProxies(), { wrapper });

beforeEach(() => {
  currentProject = { id: 'proj-1' };
  vi.mocked(agentProxiesApis.getAgentProxies).mockReset();
  vi.mocked(agentProxiesApis.createAgentProxy).mockReset();
  vi.mocked(agentProxiesApis.updateAgentProxy).mockReset();
  vi.mocked(agentProxiesApis.deleteAgentProxy).mockReset();
});

describe('loading the list', () => {
  it('fetches the proxies belonging to the active project', async () => {
    vi.mocked(agentProxiesApis.getAgentProxies).mockResolvedValue(listOf(['a', 'b']) as never);

    const { result } = renderProxies();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.agentProxiesResponse.list).toHaveLength(2);
    expect(agentProxiesApis.getAgentProxies).toHaveBeenCalledWith(
      'proj-1',
      expect.any(String)
    );
  });

  it('clears the list and stops loading when no project is selected', async () => {
    currentProject = null;

    const { result } = renderProxies();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.agentProxiesResponse.list).toEqual([]);
    expect(result.current.error).toBeNull();
    expect(agentProxiesApis.getAgentProxies).not.toHaveBeenCalled();
  });

  it('records a failure instead of leaving the caller loading forever', async () => {
    vi.mocked(agentProxiesApis.getAgentProxies).mockRejectedValue(new Error('upstream down'));

    const { result } = renderProxies();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.error?.message).toBe('upstream down');
  });

  it('lets a response that lost the race be discarded rather than written', async () => {
    // The first fetch resolves only after a second one has been issued, which is
    // what switching project does; the stale answer must not reach the list.
    let releaseFirst: (value: unknown) => void = () => {};
    vi.mocked(agentProxiesApis.getAgentProxies)
      .mockImplementationOnce(
        () => new Promise((resolve) => { releaseFirst = resolve; }) as never
      )
      .mockResolvedValue(listOf(['fresh']) as never);

    const { result } = renderProxies();
    await act(async () => { await result.current.refreshAgentProxies(); });

    await act(async () => {
      releaseFirst(listOf(['stale']));
      await Promise.resolve();
    });

    expect(result.current.agentProxiesResponse.list.map((i) => i.id)).toEqual(['fresh']);
  });
});

describe('mutating the list', () => {
  beforeEach(() => {
    vi.mocked(agentProxiesApis.getAgentProxies).mockResolvedValue(listOf(['a']) as never);
  });

  it('puts a newly created proxy at the head and raises the totals', async () => {
    vi.mocked(agentProxiesApis.createAgentProxy).mockResolvedValue({
      id: 'new', displayName: 'New',
    } as never);

    const { result } = renderProxies();
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => { await result.current.createAgentProxy({} as never); });

    expect(result.current.agentProxiesResponse.list.map((i) => i.id)).toEqual(['new', 'a']);
    expect(result.current.agentProxiesResponse.count).toBe(2);
    expect(result.current.agentProxiesResponse.pagination.total).toBe(2);
  });

  it('replaces an updated proxy in place, leaving the order alone', async () => {
    vi.mocked(agentProxiesApis.getAgentProxies).mockResolvedValue(listOf(['a', 'b']) as never);
    vi.mocked(agentProxiesApis.updateAgentProxy).mockResolvedValue({
      id: 'a', displayName: 'Renamed',
    } as never);

    const { result } = renderProxies();
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => { await result.current.updateAgentProxy('a', {} as never); });

    expect(result.current.agentProxiesResponse.list.map((i) => i.id)).toEqual(['a', 'b']);
    expect(result.current.agentProxiesResponse.list[0].displayName).toBe('Renamed');
  });

  it('drops a deleted proxy and lowers the totals', async () => {
    vi.mocked(agentProxiesApis.deleteAgentProxy).mockResolvedValue(undefined as never);

    const { result } = renderProxies();
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => { await result.current.deleteAgentProxy('a'); });

    expect(result.current.agentProxiesResponse.list).toEqual([]);
    expect(result.current.agentProxiesResponse.count).toBe(0);
  });

  it('keeps the totals at zero rather than going negative', async () => {
    vi.mocked(agentProxiesApis.getAgentProxies).mockResolvedValue({
      count: 0, list: [{ id: 'a', displayName: 'a' }], pagination: { total: 0, offset: 0, limit: 20 },
    } as never);
    vi.mocked(agentProxiesApis.deleteAgentProxy).mockResolvedValue(undefined as never);

    const { result } = renderProxies();
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => { await result.current.deleteAgentProxy('a'); });

    expect(result.current.agentProxiesResponse.count).toBe(0);
    expect(result.current.agentProxiesResponse.pagination.total).toBe(0);
  });

  it.each([
    ['create', 'createAgentProxy'],
    ['update', 'updateAgentProxy'],
    ['delete', 'deleteAgentProxy'],
  ] as const)('surfaces a failed %s to the caller', async (_label, fn) => {
    vi.mocked(agentProxiesApis[fn]).mockRejectedValue(new Error('refused') as never);

    const { result } = renderProxies();
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await expect(
      fn === 'createAgentProxy'
        ? result.current.createAgentProxy({} as never)
        : fn === 'updateAgentProxy'
          ? result.current.updateAgentProxy('a', {} as never)
          : result.current.deleteAgentProxy('a')
    ).rejects.toThrow('refused');
  });
});

describe('looking a proxy up', () => {
  it('finds one already in the list and nothing for an unknown id', async () => {
    vi.mocked(agentProxiesApis.getAgentProxies).mockResolvedValue(listOf(['a']) as never);

    const { result } = renderProxies();
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.getAgentProxyById('a')?.id).toBe('a');
    expect(result.current.getAgentProxyById('missing')).toBeUndefined();
  });
});
