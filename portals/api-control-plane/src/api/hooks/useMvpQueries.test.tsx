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

import { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { makeTestQueryClient } from '../../test/utils';
import type { Api, ApiDetail } from '../../types/domain';

// Stub the network layer; we assert on cache invalidation, not transport.
vi.mock('../mvpApi', async (importActual) => ({
  ...(await importActual<typeof import('../mvpApi')>()),
  updateApi: vi.fn(),
  createApi: vi.fn(),
  deleteApi: vi.fn(),
  getApi: vi.fn(),
  listProjects: vi.fn(),
}));

import { createApi, deleteApi, getApi, listProjects, updateApi } from '../mvpApi';
import {
  useApi,
  useCreateApi,
  useDeleteApi,
  useProjects,
  useUpdateApi,
} from './useMvpQueries';

const ORG = 'api-platform-demo';
const PROJ = 'retail-apis';
const HANDLER = 'orders-api';

const detail = { handler: HANDLER } as ApiDetail;

function makeWrapper() {
  const queryClient = makeTestQueryClient();
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries');
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { wrapper, invalidate };
}

const invalidatedKeys = (spy: { mock: { calls: unknown[][] } }) =>
  spy.mock.calls.map((c) => (c[0] as { queryKey: unknown }).queryKey);

describe('useMvpQueries cache invalidation', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.restoreAllMocks());

  it('useUpdateApi invalidates detail, single-API, and list keys', async () => {
    vi.mocked(updateApi).mockResolvedValue(detail);
    const { wrapper, invalidate } = makeWrapper();
    const { result } = renderHook(() => useUpdateApi(ORG, PROJ), { wrapper });

    await act(async () => {
      await result.current.mutateAsync(detail);
    });

    const keys = invalidatedKeys(invalidate);
    // Regression guard: the single-API key (['api', ...]) read by useApi must
    // be invalidated alongside detail + list, or breadcrumb/capabilities/
    // Deploy/Test/Manage serve stale data after an edit.
    expect(keys).toContainEqual(['componentDetail', ORG, PROJ, HANDLER]);
    expect(keys).toContainEqual(['api', ORG, PROJ, HANDLER]);
    expect(keys).toContainEqual(['components', ORG, PROJ]);
  });

  it('useDeleteApi invalidates the list key', async () => {
    vi.mocked(deleteApi).mockResolvedValue(undefined);
    const { wrapper, invalidate } = makeWrapper();
    const { result } = renderHook(() => useDeleteApi(ORG, PROJ), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ handler: HANDLER } as Api);
    });

    expect(invalidatedKeys(invalidate)).toContainEqual(['components', ORG, PROJ]);
  });

  it('useCreateApi invalidates the list key on success', async () => {
    vi.mocked(createApi).mockResolvedValue({ handler: 'new-api' } as Api);
    const { wrapper, invalidate } = makeWrapper();
    const { result } = renderHook(() => useCreateApi(ORG, PROJ), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        kind: 'API_PROXY',
        displayName: 'New API',
      } as Parameters<typeof result.current.mutateAsync>[0]);
    });

    await waitFor(() =>
      expect(invalidatedKeys(invalidate)).toContainEqual([
        'components',
        ORG,
        PROJ,
      ])
    );
  });
});

describe('useMvpQueries enabled gating', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.restoreAllMocks());

  it('useApi does not fetch when required args are missing', () => {
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useApi(undefined, undefined, undefined), {
      wrapper,
    });
    expect(result.current.fetchStatus).toBe('idle');
    expect(getApi).not.toHaveBeenCalled();
  });

  it('useApi fetches once all args are present', async () => {
    vi.mocked(getApi).mockResolvedValue({ handler: HANDLER } as Api);
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useApi(ORG, PROJ, HANDLER), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(getApi).toHaveBeenCalledWith(ORG, PROJ, HANDLER);
    expect(result.current.data).toMatchObject({ handler: HANDLER });
  });

  it('useProjects stays idle without an orgHandle', () => {
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useProjects(undefined), { wrapper });
    expect(result.current.fetchStatus).toBe('idle');
    expect(listProjects).not.toHaveBeenCalled();
  });
});
