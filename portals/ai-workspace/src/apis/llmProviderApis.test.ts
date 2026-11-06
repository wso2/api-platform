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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';

import { get } from '../clients/choreoApiClient';
import { getLLMProviders, getLLMProviderProxies } from './llmProviderApis';

vi.mock('../clients/choreoApiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const BASE = 'https://platform.test';
const page = { count: 0, list: [], pagination: { total: 0, offset: 0, limit: 20 } };

const lastQuery = () => {
  const url = vi.mocked(get).mock.calls[0][0] as string;
  const [path, qs] = url.split('?');
  return { path, params: new URLSearchParams(qs) };
};

beforeEach(() => {
  vi.mocked(get).mockReset();
  vi.mocked(get).mockResolvedValue(page);
});

describe('getLLMProviders', () => {
  it('defaults to limit=20 and offset=0 with no filters', async () => {
    const result = await getLLMProviders('org-1', BASE);

    const { path, params } = lastQuery();
    expect(path).toBe('/llm-providers');
    expect(Object.fromEntries(params)).toEqual({ limit: '20', offset: '0' });
    expect(vi.mocked(get).mock.calls[0][1]).toBeUndefined();
    expect(vi.mocked(get).mock.calls[0][2]).toBe(BASE);
    expect(result).toBe(page);
  });

  it('sends the requested limit and offset', async () => {
    await getLLMProviders('org-1', BASE, 50, 100);

    expect(Object.fromEntries(lastQuery().params)).toEqual({ limit: '50', offset: '100' });
  });

  it('maps the search option to the query parameter', async () => {
    await getLLMProviders('org-1', BASE, 20, 0, { search: 'open ai&x=1' });

    const { params } = lastQuery();
    expect(params.get('query')).toBe('open ai&x=1');
    expect(params.get('x')).toBeNull();
  });

  it('includes sortBy and sortOrder when given', async () => {
    await getLLMProviders('org-1', BASE, 20, 0, { sortBy: 'createdAt', sortOrder: 'desc' });

    expect(Object.fromEntries(lastQuery().params)).toEqual({
      limit: '20',
      offset: '0',
      sortBy: 'createdAt',
      sortOrder: 'desc',
    });
  });

  it('omits an empty search term', async () => {
    await getLLMProviders('org-1', BASE, 20, 0, { search: '' });

    expect(lastQuery().params.has('query')).toBe(false);
  });

  it('propagates request failures', async () => {
    const failure = Object.assign(new Error('boom'), { status: 500 });
    vi.mocked(get).mockRejectedValue(failure);

    await expect(getLLMProviders('org-1', BASE)).rejects.toBe(failure);
  });
});

describe('getLLMProviderProxies', () => {
  it('URL-encodes the provider id and passes limit/offset', async () => {
    await getLLMProviderProxies('a/b', 'org-1', BASE, 1, 0);

    const { path, params } = lastQuery();
    expect(path).toBe('/llm-providers/a%2Fb/llm-proxies');
    expect(Object.fromEntries(params)).toEqual({ limit: '1', offset: '0' });
    expect(vi.mocked(get).mock.calls[0][2]).toBe(BASE);
  });

  it('defaults to limit=20 and offset=0', async () => {
    await getLLMProviderProxies('p1', 'org-1', BASE);

    expect(Object.fromEntries(lastQuery().params)).toEqual({ limit: '20', offset: '0' });
  });

  it('propagates request failures', async () => {
    vi.mocked(get).mockRejectedValue(new Error('nope'));

    await expect(getLLMProviderProxies('p1', 'org-1', BASE)).rejects.toThrow('nope');
  });
});
