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

import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { listSubscriptionPlans } from '../apis/subscriptionPlanApis';
import { useSubscriptionPlans } from './useSubscriptionPlans';

vi.mock('../apis/subscriptionPlanApis', () => ({ listSubscriptionPlans: vi.fn() }));
vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const gold = { id: 'gold', displayName: 'Gold', status: 'ACTIVE' as const };

beforeEach(() => {
  vi.mocked(listSubscriptionPlans).mockReset();
});

describe('useSubscriptionPlans', () => {
  it('reads nothing until it is enabled', () => {
    const { result } = renderHook(() => useSubscriptionPlans(false));

    expect(listSubscriptionPlans).not.toHaveBeenCalled();
    expect(result.current).toEqual({ plans: undefined, failed: false, upToDate: false });
  });

  it('holds no plans until they are in, then all of them', async () => {
    vi.mocked(listSubscriptionPlans).mockResolvedValue([gold]);

    const { result } = renderHook(() => useSubscriptionPlans(true));
    expect(result.current.plans).toBeUndefined();

    await waitFor(() => expect(result.current.plans).toEqual([gold]));
    expect(result.current.failed).toBe(false);
    expect(result.current.upToDate).toBe(true);
  });

  it('keeps the earlier list while it is read again, but not up to date until the new one is in', async () => {
    const silver = { id: 'silver', displayName: 'Silver', status: 'ACTIVE' as const };
    vi.mocked(listSubscriptionPlans).mockResolvedValueOnce([gold]);
    const { result, rerender } = renderHook(({ enabled }) => useSubscriptionPlans(enabled), {
      initialProps: { enabled: true },
    });
    await waitFor(() => expect(result.current.upToDate).toBe(true));

    rerender({ enabled: false });
    vi.mocked(listSubscriptionPlans).mockResolvedValueOnce([gold, silver]);
    rerender({ enabled: true });

    expect(result.current.plans).toEqual([gold]);
    expect(result.current.upToDate).toBe(false);
    await waitFor(() => expect(result.current.plans).toEqual([gold, silver]));
    expect(result.current.upToDate).toBe(true);
  });

  it('reads when it becomes enabled', async () => {
    vi.mocked(listSubscriptionPlans).mockResolvedValue([gold]);

    const { result, rerender } = renderHook(({ enabled }) => useSubscriptionPlans(enabled), {
      initialProps: { enabled: false },
    });
    rerender({ enabled: true });

    await waitFor(() => expect(result.current.plans).toEqual([gold]));
    expect(listSubscriptionPlans).toHaveBeenCalledTimes(1);
  });

  it('reports a failed read instead of plans', async () => {
    vi.mocked(listSubscriptionPlans).mockRejectedValue(new Error('forbidden'));

    const { result } = renderHook(() => useSubscriptionPlans(true));

    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(result.current.plans).toBeUndefined();
  });
});
