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

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';

import * as proxyApis from '../../apis/proxyApis';
import { ProxiesProvider, useProxies } from './ProxiesContext';

vi.mock('../../apis/proxyApis');
vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const currentOrganization = { uuid: 'org-1' };
const currentProject = { id: 'proj-1' };
vi.mock('../AppShellContext', () => ({
  useAppShell: () => ({ currentOrganization, currentProject }),
}));

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <ProxiesProvider>{children}</ProxiesProvider>
);

const renderProxies = () => renderHook(() => useProxies(), { wrapper });

beforeEach(() => {
  vi.mocked(proxyApis.getProxies).mockReset();
});

describe('loading the App LLM Proxies list', () => {
  it('does not truncate a project with more proxies than the server default page size', async () => {
    // Reproduces the WSO2 Summit bug report: a project with 25-28 App LLM
    // Proxies only ever showed 20, because the list fetch never passed an
    // explicit limit and the server defaults an unpaged request to 20.
    const allProxies = {
      count: 25,
      list: Array.from({ length: 25 }, (_, i) => ({ id: `proxy-${i}`, displayName: `proxy-${i}` })),
      pagination: { total: 25, offset: 0, limit: 100 },
    };
    vi.mocked(proxyApis.getProxies).mockResolvedValue(allProxies as never);

    const { result } = renderProxies();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.proxiesResponse.list).toHaveLength(25);
    expect(proxyApis.getProxies).toHaveBeenCalledWith(
      'org-1',
      'proj-1',
      expect.any(String),
      100,
      0
    );
  });

  it('walks a second page when a project holds more than one full page of proxies', async () => {
    const firstPage = {
      count: 100,
      list: Array.from({ length: 100 }, (_, i) => ({ id: `proxy-${i}`, displayName: `proxy-${i}` })),
      pagination: { total: 128, offset: 0, limit: 100 },
    };
    const secondPage = {
      count: 28,
      list: Array.from({ length: 28 }, (_, i) => ({ id: `proxy-${100 + i}`, displayName: `proxy-${100 + i}` })),
      pagination: { total: 128, offset: 100, limit: 100 },
    };
    vi.mocked(proxyApis.getProxies)
      .mockResolvedValueOnce(firstPage as never)
      .mockResolvedValueOnce(secondPage as never);

    const { result } = renderProxies();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.proxiesResponse.list).toHaveLength(128);
    expect(proxyApis.getProxies).toHaveBeenCalledTimes(2);
    expect(proxyApis.getProxies).toHaveBeenNthCalledWith(
      2,
      'org-1',
      'proj-1',
      expect.any(String),
      100,
      100
    );
  });

  it('clears the list and stops loading when no project is selected', async () => {
    currentProject.id = '';

    const { result } = renderProxies();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.proxiesResponse.list).toEqual([]);
    expect(proxyApis.getProxies).not.toHaveBeenCalled();

    currentProject.id = 'proj-1';
  });
});
