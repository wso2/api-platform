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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { get } from '../clients/choreoApiClient';
import { listSubscriptionPlans } from './subscriptionPlanApis';

vi.mock('../clients/choreoApiClient', () => ({ get: vi.fn() }));

const BASE = 'https://control-plane.test';

const plan = (id: string) => ({ id, displayName: id, status: 'ACTIVE' as const });
const page = (ids: string[], total: number, offset = 0) => ({
  list: ids.map(plan),
  count: ids.length,
  pagination: { total, offset, limit: 100 },
});

beforeEach(() => {
  vi.mocked(get).mockReset();
});

describe('listSubscriptionPlans', () => {
  it('reads one page when it holds every plan', async () => {
    vi.mocked(get).mockResolvedValue(page(['gold', 'silver'], 2));

    const plans = await listSubscriptionPlans(BASE);

    expect(plans.map(({ id }) => id)).toEqual(['gold', 'silver']);
    expect(get).toHaveBeenCalledTimes(1);
    expect(get).toHaveBeenCalledWith('/subscription-plans', { limit: 100, offset: 0 }, BASE);
  });

  it('keeps reading, from where the last page ended, until the total is held', async () => {
    vi.mocked(get)
      .mockResolvedValueOnce(page(['a', 'b'], 3))
      .mockResolvedValueOnce(page(['c'], 3, 2));

    const plans = await listSubscriptionPlans(BASE);

    expect(plans.map(({ id }) => id)).toEqual(['a', 'b', 'c']);
    expect(get).toHaveBeenNthCalledWith(2, '/subscription-plans', { limit: 100, offset: 2 }, BASE);
  });

  it('stops on an empty page even when the total says more exist', async () => {
    vi.mocked(get).mockResolvedValueOnce(page(['a'], 5)).mockResolvedValueOnce(page([], 5, 1));

    const plans = await listSubscriptionPlans(BASE);

    expect(plans).toHaveLength(1);
    expect(get).toHaveBeenCalledTimes(2);
  });

  it('returns no plans for an organization without any', async () => {
    vi.mocked(get).mockResolvedValue(page([], 0));

    expect(await listSubscriptionPlans(BASE)).toEqual([]);
  });

  it('propagates a failed read', async () => {
    vi.mocked(get).mockRejectedValue(new Error('forbidden'));

    await expect(listSubscriptionPlans(BASE)).rejects.toThrow('forbidden');
  });
});
