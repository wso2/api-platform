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

import { queryOptions } from '@tanstack/react-query';

import { staleTimes } from '../../core/queryClient';
import { createResourceKeys, type OrgScope } from '../../core/queryKeys';
import {
  getSubscriptionPlan,
  listSubscriptionPlans,
  type ListSubscriptionPlansQuery,
  type SubscriptionPlan,
  type SubscriptionPlanListResponse,
} from './subscriptionPlans.endpoints';

export const subscriptionPlanKeys = createResourceKeys('subscriptionPlans');

/** The server's maximum page size. */
const PLANS_PAGE_SIZE = 100;
/** Bounds the loop so a wrong `pagination.total` cannot make it run forever. */
const PLANS_MAX_PAGES = 50;

/**
 * Pages through `/subscription-plans` until `pagination.total` plans are held.
 * Duplicates are dropped: a plan created mid-fetch can shift a later page.
 */
const fetchAllSubscriptionPlans = async (
  org: OrgScope,
  signal: AbortSignal
): Promise<SubscriptionPlanListResponse> => {
  const plans: SubscriptionPlan[] = [];
  const seen = new Set<string>();
  let received = 0;

  for (let page = 0; page < PLANS_MAX_PAGES; page += 1) {
    const response = await listSubscriptionPlans({
      orgId: org,
      signal,
      query: { limit: PLANS_PAGE_SIZE, offset: received },
    });
    const items = response.list ?? [];
    received += items.length;
    for (const plan of items) {
      if (plan.id && seen.has(plan.id)) continue;
      if (plan.id) seen.add(plan.id);
      plans.push(plan);
    }
    if (items.length === 0 || received >= (response.pagination?.total ?? 0)) break;
  }

  return {
    list: plans,
    count: plans.length,
    pagination: { total: plans.length, offset: 0, limit: plans.length },
  };
};

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks, so
 * the same definition drives `useQuery`, a router loader's `ensureQueryData`,
 * a prefetch on hover and `getQueryData` inside a mutation.
 *
 * Both use the `stable` tier. Plans are configuration an operator edits
 * occasionally, and the plan picker on a subscription form reads this list on
 * every open — refetching it each time would be pure waste.
 */
export const subscriptionPlanQueries = {
  list: (org: OrgScope, query: ListSubscriptionPlansQuery = {}) =>
    queryOptions({
      queryKey: subscriptionPlanKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) =>
        listSubscriptionPlans({ orgId: org, signal, query }),
      staleTime: staleTimes.stable,
    }),

  /** Every plan in the org, across all pages, in the shape of a single list response. */
  listAll: (org: OrgScope) =>
    queryOptions({
      queryKey: subscriptionPlanKeys.list(org, { pages: 'all' }),
      queryFn: ({ signal }) => fetchAllSubscriptionPlans(org, signal),
      staleTime: staleTimes.stable,
    }),

  detail: (org: OrgScope, subscriptionPlanId: string) =>
    queryOptions({
      queryKey: subscriptionPlanKeys.detail(org, subscriptionPlanId),
      queryFn: ({ signal }) =>
        getSubscriptionPlan(subscriptionPlanId, { orgId: org, signal }),
      staleTime: staleTimes.stable,
    }),
};
