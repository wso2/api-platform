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

import { beforeEach, describe, expect, it } from 'vitest';

import {
  aPlanPricing,
  aSubscriptionPlan,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  createSubscriptionPlan,
  deleteSubscriptionPlan,
  getSubscriptionPlan,
  getSubscriptionPlanPricing,
  listSubscriptionPlans,
  updateSubscriptionPlan,
} from './subscriptionPlans.endpoints';

/**
 * Contract tests for `/subscription-plans` — the rate-limit tiers a
 * subscription attaches to.
 *
 * A plain CRUD resource, so most of this is URL and method pinning. A paid
 * plan is created with its pricing in the same request; afterwards the pricing
 * is only read. The delete guard is the one failure specific to it: a plan still
 * in use cannot be removed, and only `code` distinguishes that from any other.
 */

const PLAN_ID = 'gold';
const RESOURCE = `/subscription-plans/${PLAN_ID}`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listSubscriptionPlans', () => {
  it('GETs the collection at the hyphenated path', async () => {
    // The path is `/subscription-plans`, not `/subscriptionPlans` — the module
    // is camelCase but the URL is not.
    server.use(collection('/subscription-plans', [aSubscriptionPlan()], { record: requests }));

    await listSubscriptionPlans();

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscription-plans');
  });

  it('passes paging parameters through', async () => {
    server.use(collection('/subscription-plans', [], { record: requests }));

    await listSubscriptionPlans({ query: { limit: 10, offset: 20 } });

    expect(Object.fromEntries(requests.last()!.params)).toEqual({
      limit: '10',
      offset: '20',
    });
  });

  it('scopes the request to the organization', async () => {
    server.use(collection('/subscription-plans', [], { record: requests }));

    await listSubscriptionPlans({ orgId: 'acme-org' });

    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
  });
});

describe('plan CRUD', () => {
  it('GETs one plan by id', async () => {
    server.use(resource(RESOURCE, aSubscriptionPlan(), { record: requests }));

    await getSubscriptionPlan(PLAN_ID);

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscription-plans/gold');
  });

  it('POSTs a new plan with the request body', async () => {
    server.use(accepts('post', '/subscription-plans', aSubscriptionPlan(), { record: requests }));

    await createSubscriptionPlan({ displayName: 'Platinum' } as never);

    expect(requests.last()?.method).toBe('POST');
    expect(JSON.parse(requests.last()!.body)).toMatchObject({
      displayName: 'Platinum',
    });
  });

  it('PUTs an update to the resource path', async () => {
    server.use(accepts('put', RESOURCE, aSubscriptionPlan(), { record: requests }));

    await updateSubscriptionPlan(PLAN_ID, aSubscriptionPlan({ displayName: 'Gold+' }));

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscription-plans/gold');
  });

  it('DELETEs and resolves to nothing on 204', async () => {
    server.use(noContent('delete', RESOURCE, { record: requests }));

    await expect(deleteSubscriptionPlan(PLAN_ID)).resolves.toBeUndefined();
    expect(requests.last()?.method).toBe('DELETE');
  });

  it('percent-encodes the id', async () => {
    server.use(
      resource('/subscription-plans/:subscriptionPlanId', aSubscriptionPlan(), {
        record: requests,
      }),
    );

    await getSubscriptionPlan('weird/id');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscription-plans/weird%2Fid');
  });
});

describe('failures', () => {
  it('surfaces the in-use guard that blocked a delete', async () => {
    server.use(
      failure('delete', RESOURCE, 409, 'SUBSCRIPTION_PLAN_IN_USE', {
        details: { subscriptions: 4 },
        message: 'Subscriptions still reference this plan.',
      }),
    );

    const error = (await deleteSubscriptionPlan(PLAN_ID).catch((e: unknown) => e)) as ApiError;

    expect(error.code).toBe('SUBSCRIPTION_PLAN_IN_USE');
    expect(error.details).toEqual({ subscriptions: 4 });
  });

  it('labels the failing operation', async () => {
    server.use(failure('get', RESOURCE, 404, 'NOT_FOUND'));

    const error = (await getSubscriptionPlan(PLAN_ID).catch((e: unknown) => e)) as ApiError;

    expect(error.operation).toBe('GetSubscriptionPlan');
  });
});

describe('plan pricing', () => {
  const PRICING = `${RESOURCE}/pricing`;

  it('POSTs a paid plan with its pricing in the one request', async () => {
    server.use(
      accepts('post', '/subscription-plans', aSubscriptionPlan({ pricing: aPlanPricing() }), {
        record: requests,
      }),
    );

    const pricing = {
      billingPeriod: 'MONTH' as const,
      currency: 'USD',
      prices: [
        {
          filters: [
            { field: 'API_TYPE' as const, operator: 'EQUALS' as const, values: ['LlmProvider'] },
          ],
          meterId: 'llm-tokens',
          pricingModel: 'PER_UNIT' as const,
          tiers: null,
          unitAmount: '2.50',
          unitQuantity: 1000000,
        },
      ],
      pricingType: 'USAGE_BASED' as const,
    };

    const created = await createSubscriptionPlan({
      displayName: 'Gold',
      id: PLAN_ID,
      pricing,
      status: 'ACTIVE',
    });

    expect(requests.calls.map((r) => [r.method, r.url.pathname])).toEqual([
      ['POST', '/api/v0.9/subscription-plans'],
    ]);
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Gold',
      id: PLAN_ID,
      pricing,
      status: 'ACTIVE',
    });
    expect(created.pricing?.pricingType).toBe('USAGE_BASED');
  });

  it('GETs the pricing under the plan', async () => {
    server.use(resource(PRICING, aPlanPricing(), { record: requests }));

    await getSubscriptionPlanPricing(PLAN_ID);

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/subscription-plans/gold/pricing');
  });

  it('surfaces the refusal of an update that carries pricing', async () => {
    server.use(
      failure('put', RESOURCE, 400, 'VALIDATION_FAILED', {
        message:
          "Pricing can't change once a plan is created. Create a new plan with the new pricing.",
      }),
    );

    const error = (await updateSubscriptionPlan(
      PLAN_ID,
      aSubscriptionPlan({ pricing: aPlanPricing() }),
    ).catch((e: unknown) => e)) as ApiError;

    expect(error.isValidation).toBe(true);
    expect(error.message).toBe(
      "Pricing can't change once a plan is created. Create a new plan with the new pricing.",
    );
    expect(error.operation).toBe('UpdateSubscriptionPlan');
  });
});
