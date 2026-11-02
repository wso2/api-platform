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

import { waitFor } from '@testing-library/react';
import type { QueryClient } from '@tanstack/react-query';
import { beforeEach, describe, expect, it } from 'vitest';

import { aMeter, aSubscriptionPlan, accepts, noContent } from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import type { OrgScope } from '../../core/queryKeys';
import { meterKeys } from '../meters/meters.queries';
import {
  useCreateSubscriptionPlan,
  useDeleteSubscriptionPlan,
  useUpdateSubscriptionPlan,
} from './subscriptionPlans.hooks';

/**
 * A plan's prices name the meters they count with, so creating or deleting a plan
 * changes those meters' `planCount`: the meter list is read again after either.
 * A plan's pricing can't be updated, so an update leaves the meter list alone.
 */

beforeEach(() => {
  resetHttpClient();
});

/** Caches a meter list, as the Meters page leaves it, and returns its key. */
const cacheMeterList = (queryClient: QueryClient, org: OrgScope) => {
  const key = meterKeys.list(org, {});
  queryClient.setQueryData(key, { count: 1, list: [aMeter()], pagination: {} });
  return key;
};

describe('subscription plan mutations and the meter list', () => {
  it('reads the meters again after a plan is created', async () => {
    server.use(accepts('post', '/subscription-plans', aSubscriptionPlan({ id: 'platinum' })));

    const { result, queryClient, org } = renderApiHook(() => useCreateSubscriptionPlan());
    const meterList = cacheMeterList(queryClient, org);

    result.current.mutate({ displayName: 'Platinum', status: 'ACTIVE' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() => expect(queryClient.getQueryState(meterList)?.isInvalidated).toBe(true));
  });

  it('reads the meters again after a plan is deleted', async () => {
    server.use(noContent('delete', '/subscription-plans/gold'));

    const { result, queryClient, org } = renderApiHook(() => useDeleteSubscriptionPlan());
    const meterList = cacheMeterList(queryClient, org);

    result.current.mutate({ subscriptionPlanId: 'gold' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() => expect(queryClient.getQueryState(meterList)?.isInvalidated).toBe(true));
  });

  it('leaves the meter list alone after a plan is updated', async () => {
    server.use(accepts('put', '/subscription-plans/gold', aSubscriptionPlan({ displayName: 'Gold plus' })));

    const { result, queryClient, org } = renderApiHook(() => useUpdateSubscriptionPlan());
    const meterList = cacheMeterList(queryClient, org);

    result.current.mutate({ subscriptionPlanId: 'gold', body: { displayName: 'Gold plus' } });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    await settle();

    expect(queryClient.getQueryState(meterList)?.isInvalidated).toBe(false);
  });
});
