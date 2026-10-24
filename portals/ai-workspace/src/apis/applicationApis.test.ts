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

import { get } from '../clients/choreoApiClient';
import { applicationApis, getApplicationAPIKeys } from './applicationApis';

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

beforeEach(() => {
  vi.mocked(get).mockReset();
  vi.mocked(get).mockResolvedValue({ list: [], pagination: { total: 0, offset: 0, limit: 20 } });
});

describe('getApplications query string', () => {
  it('sends only the project id when no options are given', async () => {
    await applicationApis.getApplications('proj-1', BASE);

    expect(get).toHaveBeenCalledWith('/applications?projectId=proj-1', undefined, BASE);
  });

  it('includes type, paging, search and sort options', async () => {
    await applicationApis.getApplications('proj-1', BASE, {
      type: 'genai',
      limit: 50,
      offset: 100,
      query: 'trip planner',
      sortBy: 'name',
      sortOrder: 'desc',
    } as never);

    const url = vi.mocked(get).mock.calls[0][0] as string;
    const params = new URLSearchParams(url.split('?')[1]);
    expect(url.startsWith('/applications?')).toBe(true);
    expect(Object.fromEntries(params)).toEqual({
      projectId: 'proj-1',
      type: 'genai',
      limit: '50',
      offset: '100',
      query: 'trip planner',
      sortBy: 'name',
      sortOrder: 'desc',
    });
  });

  it('keeps a zero offset but drops empty and undefined values', async () => {
    await applicationApis.getApplications('proj-1', BASE, { offset: 0, query: '' } as never);

    expect(get).toHaveBeenCalledWith('/applications?projectId=proj-1&offset=0', undefined, BASE);
  });

  it('rethrows request failures', async () => {
    vi.mocked(get).mockRejectedValue(new Error('boom'));

    await expect(applicationApis.getApplications('proj-1', BASE)).rejects.toThrow('boom');
  });
});

describe('getApplicationAPIKeys query string', () => {
  it('omits the query string without options and encodes the id', async () => {
    await getApplicationAPIKeys('a/b', BASE);

    expect(get).toHaveBeenCalledWith('/applications/a%2Fb/api-keys', undefined, BASE);
  });

  it('adds limit and offset', async () => {
    await getApplicationAPIKeys('app', BASE, { limit: 10, offset: 20 });

    expect(get).toHaveBeenCalledWith('/applications/app/api-keys?limit=10&offset=20', undefined, BASE);
  });

  it('rethrows request failures', async () => {
    vi.mocked(get).mockRejectedValue(new Error('boom'));

    await expect(getApplicationAPIKeys('app', BASE)).rejects.toThrow('boom');
  });
});
