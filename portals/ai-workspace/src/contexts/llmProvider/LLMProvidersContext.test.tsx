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

import * as llmProviderApis from '../../apis/llmProviderApis';
import { LLMProvidersProvider, useLLMProviders } from './LLMProvidersContext';

vi.mock('../../apis/llmProviderApis');
vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const currentOrganization = { uuid: 'org-1' };
vi.mock('../AppShellContext', () => ({
  useAppShell: () => ({ currentOrganization }),
}));

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <LLMProvidersProvider>{children}</LLMProvidersProvider>
);

const renderProviders = () => renderHook(() => useLLMProviders(), { wrapper });

beforeEach(() => {
  vi.mocked(llmProviderApis.getLLMProviders).mockReset();
});

describe('loading the LLM Providers list', () => {
  it('does not truncate an organization with more providers than the server default page size', async () => {
    // Reproduces the WSO2 Summit bug report: an organization with 28 LLM
    // Providers per provider type only ever showed 20, because the list
    // fetch never passed an explicit limit and the server defaults an
    // unpaged request to 20.
    const allProviders = {
      count: 28,
      list: Array.from({ length: 28 }, (_, i) => ({ id: `provider-${i}`, displayName: `provider-${i}` })),
      pagination: { total: 28, offset: 0, limit: 100 },
    };
    vi.mocked(llmProviderApis.getLLMProviders).mockResolvedValue(allProviders as never);

    const { result } = renderProviders();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.providersResponse.list).toHaveLength(28);
    expect(llmProviderApis.getLLMProviders).toHaveBeenCalledWith(
      'org-1',
      expect.any(String),
      100,
      0
    );
  });

  it('walks a second page when an organization holds more than one full page of providers', async () => {
    const firstPage = {
      count: 100,
      list: Array.from({ length: 100 }, (_, i) => ({ id: `provider-${i}`, displayName: `provider-${i}` })),
      pagination: { total: 115, offset: 0, limit: 100 },
    };
    const secondPage = {
      count: 15,
      list: Array.from({ length: 15 }, (_, i) => ({ id: `provider-${100 + i}`, displayName: `provider-${100 + i}` })),
      pagination: { total: 115, offset: 100, limit: 100 },
    };
    vi.mocked(llmProviderApis.getLLMProviders)
      .mockResolvedValueOnce(firstPage as never)
      .mockResolvedValueOnce(secondPage as never);

    const { result } = renderProviders();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.providersResponse.list).toHaveLength(115);
    expect(llmProviderApis.getLLMProviders).toHaveBeenCalledTimes(2);
    expect(llmProviderApis.getLLMProviders).toHaveBeenNthCalledWith(
      2,
      'org-1',
      expect.any(String),
      100,
      100
    );
  });
});
