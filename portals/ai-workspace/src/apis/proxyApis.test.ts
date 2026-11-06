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
import { getProxies } from './proxyApis';

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

describe('getProxies', () => {
  it('filters by project and defaults to limit=20, offset=0', async () => {
    const result = await getProxies('org-1', 'proj-1', BASE);

    const { path, params } = lastQuery();
    expect(path).toBe('/llm-proxies');
    expect(Object.fromEntries(params)).toEqual({ projectId: 'proj-1', limit: '20', offset: '0' });
    expect(vi.mocked(get).mock.calls[0][1]).toBeUndefined();
    expect(vi.mocked(get).mock.calls[0][2]).toBe(BASE);
    expect(result).toBe(page);
  });

  it('sends the requested limit and offset', async () => {
    await getProxies('org-1', 'proj-1', BASE, 100, 200);

    const params = lastQuery().params;
    expect(params.get('limit')).toBe('100');
    expect(params.get('offset')).toBe('200');
  });

  it('maps the search option to the query parameter, safely encoded', async () => {
    await getProxies('org-1', 'proj-1', BASE, 20, 0, { search: 'trip&projectId=evil' });

    const { params } = lastQuery();
    expect(params.get('query')).toBe('trip&projectId=evil');
    expect(params.get('projectId')).toBe('proj-1');
  });

  it('includes sortBy and sortOrder when given', async () => {
    await getProxies('org-1', 'proj-1', BASE, 20, 0, { sortBy: 'name', sortOrder: 'asc' });

    const params = lastQuery().params;
    expect(params.get('sortBy')).toBe('name');
    expect(params.get('sortOrder')).toBe('asc');
  });

  it('omits optional parameters that were not provided', async () => {
    await getProxies('org-1', 'proj-1', BASE, 20, 0, { search: '' });

    const params = lastQuery().params;
    expect(params.has('query')).toBe(false);
    expect(params.has('sortBy')).toBe(false);
    expect(params.has('sortOrder')).toBe(false);
  });

  it('propagates request failures', async () => {
    const failure = Object.assign(new Error('boom'), { status: 503 });
    vi.mocked(get).mockRejectedValue(failure);

    await expect(getProxies('org-1', 'proj-1', BASE)).rejects.toBe(failure);
  });
});
