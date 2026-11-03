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

import { get } from '../clients/choreoApiClient';
import type { SubscriptionPlan, SubscriptionPlanListResponse } from '../utils/types';

/** The server's maximum page size for a plan list. */
const PLANS_PAGE_SIZE = 100;
/** Bounds the loop so a wrong `pagination.total` cannot make it run forever. */
const PLANS_MAX_PAGES = 50;

/**
 * Every subscription plan of the organization, page by page. A plan created while the
 * pages are read can come up twice; it is listed once. Scope: `ap:subscription_plan:read`.
 */
export async function listSubscriptionPlans(baseUrl: string): Promise<SubscriptionPlan[]> {
  const plans: SubscriptionPlan[] = [];
  const seen = new Set<string>();
  let received = 0;
  for (let pages = 0; pages < PLANS_MAX_PAGES; pages += 1) {
    const page = await get<SubscriptionPlanListResponse>(
      '/subscription-plans',
      { limit: PLANS_PAGE_SIZE, offset: received },
      baseUrl
    );
    received += page.list.length;
    for (const plan of page.list) {
      if (seen.has(plan.id)) continue;
      seen.add(plan.id);
      plans.push(plan);
    }
    if (page.list.length === 0 || received >= page.pagination.total) break;
  }
  return plans;
}
