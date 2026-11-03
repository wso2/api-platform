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

import { get, post, put, del } from '../../clients/choreoApiClient';
import {
  createAgentProxy,
  getAgentProxies,
  getAgentProxy,
  updateAgentProxy,
  deleteAgentProxy,
  fetchAgentCard,
  getAgentProxyAPIKeys,
  createAgentProxyAPIKey,
  revokeAgentProxyAPIKey,
} from './agentProxiesApis';

vi.mock('../../clients/choreoApiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const BASE = 'https://control-plane.test';

beforeEach(() => {
  vi.mocked(get).mockReset();
  vi.mocked(post).mockReset();
  vi.mocked(put).mockReset();
  vi.mocked(del).mockReset();
});

describe('agent proxy requests', () => {
  it('posts a new proxy and returns what the server stored', async () => {
    const stored = { id: 'proxy-1' };
    vi.mocked(post).mockResolvedValue(stored);

    const result = await createAgentProxy({ displayName: 'Trips' } as never, BASE);

    expect(post).toHaveBeenCalledWith('/agent-proxies', { displayName: 'Trips' }, BASE);
    expect(result).toBe(stored);
  });

  it('carries the project and the paging window into the list query', async () => {
    vi.mocked(get).mockResolvedValue({ list: [], count: 0 });

    await getAgentProxies('proj-1', BASE, 50, 100);

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies?projectId=proj-1&limit=50&offset=100',
      undefined,
      BASE
    );
  });

  it('defaults the paging window when the caller omits it', async () => {
    vi.mocked(get).mockResolvedValue({ list: [], count: 0 });

    await getAgentProxies('proj-1', BASE);

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies?projectId=proj-1&limit=20&offset=0',
      undefined,
      BASE
    );
  });

  it('escapes a project id that would otherwise alter the query', async () => {
    vi.mocked(get).mockResolvedValue({ list: [], count: 0 });

    await getAgentProxies('a&b=c', BASE);

    expect(get).toHaveBeenCalledWith(
      '/agent-proxies?projectId=a%26b%3Dc&limit=20&offset=0',
      undefined,
      BASE
    );
  });

  it('escapes an id that would otherwise add a path segment', async () => {
    vi.mocked(get).mockResolvedValue({ id: 'x' });

    await getAgentProxy('a/b', BASE);

    expect(get).toHaveBeenCalledWith('/agent-proxies/a%2Fb', undefined, BASE);
  });

  it('puts the updated proxy against its own id', async () => {
    const updated = { id: 'proxy-1' };
    vi.mocked(put).mockResolvedValue(updated);

    const result = await updateAgentProxy('proxy-1', { displayName: 'Trips' } as never, BASE);

    expect(put).toHaveBeenCalledWith('/agent-proxies/proxy-1', { displayName: 'Trips' }, BASE);
    expect(result).toBe(updated);
  });

  it('deletes a proxy by id', async () => {
    vi.mocked(del).mockResolvedValue(undefined);

    await deleteAgentProxy('proxy-1', BASE);

    expect(del).toHaveBeenCalledWith('/agent-proxies/proxy-1', undefined, BASE);
  });

  it('asks the server to fetch the card rather than reaching the agent directly', async () => {
    const card = { name: 'Trip Planner' };
    vi.mocked(post).mockResolvedValue(card);

    const result = await fetchAgentCard({ url: 'https://agent.test' } as never, BASE);

    expect(post).toHaveBeenCalledWith(
      '/agent-proxies/fetch-agent-card',
      { url: 'https://agent.test' },
      BASE
    );
    expect(result).toBe(card);
  });
});

describe('agent proxy API keys', () => {
  it('lists the keys under the owning proxy', async () => {
    vi.mocked(get).mockResolvedValue({ list: [], count: 0 });

    await getAgentProxyAPIKeys('proxy-1', BASE);

    expect(get).toHaveBeenCalledWith('/agent-proxies/proxy-1/api-keys', undefined, BASE);
  });

  it('creates a key under the owning proxy', async () => {
    const created = { apiKey: 'secret' };
    vi.mocked(post).mockResolvedValue(created);

    const result = await createAgentProxyAPIKey('proxy-1', { displayName: 'ci' } as never, BASE);

    expect(post).toHaveBeenCalledWith(
      '/agent-proxies/proxy-1/api-keys',
      { displayName: 'ci' },
      BASE
    );
    expect(result).toBe(created);
  });

  it('revokes a key by its own id, both ids escaped', async () => {
    vi.mocked(del).mockResolvedValue(undefined);

    await revokeAgentProxyAPIKey('a/b', 'c d', BASE);

    expect(del).toHaveBeenCalledWith('/agent-proxies/a%2Fb/api-keys/c%20d', undefined, BASE);
  });
});

describe('transport failures', () => {
  const failure = new Error('upstream unavailable');

  it.each([
    ['createAgentProxy', () => createAgentProxy({} as never, BASE), post],
    ['getAgentProxies', () => getAgentProxies('p', BASE), get],
    ['getAgentProxy', () => getAgentProxy('a', BASE), get],
    ['updateAgentProxy', () => updateAgentProxy('a', {} as never, BASE), put],
    ['deleteAgentProxy', () => deleteAgentProxy('a', BASE), del],
    ['fetchAgentCard', () => fetchAgentCard({} as never, BASE), post],
    ['getAgentProxyAPIKeys', () => getAgentProxyAPIKeys('a', BASE), get],
    ['createAgentProxyAPIKey', () => createAgentProxyAPIKey('a', {} as never, BASE), post],
    ['revokeAgentProxyAPIKey', () => revokeAgentProxyAPIKey('a', 'k', BASE), del],
  ])('%s surfaces the failure to its caller', async (_name, call, client) => {
    vi.mocked(client).mockRejectedValue(failure);

    await expect(call()).rejects.toThrow('upstream unavailable');
  });
});
