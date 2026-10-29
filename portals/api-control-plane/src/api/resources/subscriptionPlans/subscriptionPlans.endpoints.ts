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

import { http, type RequestOptions } from '../../core/http';
import type {
  BodyOf,
  PathOf,
  QueryOf,
  ResponseOf,
  Schema,
} from '../../core/spec';

/**
 * Transport layer for `/subscription-plans` — the rate-limit and quota tiers a
 * subscription can be attached to. One thin function per spec operation: no
 * branching, no adapters, no cache awareness.
 */

export type SubscriptionPlan = Schema<'SubscriptionPlan'>;
export type SubscriptionPlanListResponse = ResponseOf<'ListSubscriptionPlans'>;
export type ListSubscriptionPlansQuery = QueryOf<'ListSubscriptionPlans'>;
export type CreateSubscriptionPlanBody = BodyOf<'CreateSubscriptionPlan'>;
export type UpdateSubscriptionPlanBody = BodyOf<'UpdateSubscriptionPlan'>;
export type SubscriptionPlanPricing = Schema<'SubscriptionPlanPricing'>;
export type SubscriptionPlanPricingBody = Schema<'SubscriptionPlanPricingRequest'>;
export type PriceTier = Schema<'PriceTier'>;
export type SubscriptionPlanPrice = Schema<'SubscriptionPlanPrice'>;
export type PriceFilter = Schema<'PriceFilter'>;

const BASE = '/subscription-plans';

const resourcePath = (
  subscriptionPlanId: PathOf<'GetSubscriptionPlan'>['subscriptionPlanId']
): string => `${BASE}/${encodeURIComponent(subscriptionPlanId)}`;

export const listSubscriptionPlans = async (
  options?: RequestOptions
): Promise<SubscriptionPlanListResponse> => {
  return http.get<SubscriptionPlanListResponse>(BASE, {
    ...options,
    operationName: 'ListSubscriptionPlans',
  });
};

export const getSubscriptionPlan = async (
  subscriptionPlanId: string,
  options?: RequestOptions
): Promise<SubscriptionPlan> => {
  return http.get<SubscriptionPlan>(resourcePath(subscriptionPlanId), {
    ...options,
    operationName: 'GetSubscriptionPlan',
  });
};

/** Creates a plan; a paid plan is created with its `pricing`, which can't be updated afterwards. */
export const createSubscriptionPlan = async (
  body: CreateSubscriptionPlanBody,
  options?: RequestOptions
): Promise<SubscriptionPlan> => {
  return http.post<SubscriptionPlan>(BASE, body, {
    ...options,
    operationName: 'CreateSubscriptionPlan',
  });
};

/**
 * Updates a plan's name, limits, status or expiry. The name and request limit
 * can't change while anything offers or uses the plan (409
 * `SUBSCRIPTION_PLAN_IN_USE`), and a body with `pricing` is refused (409
 * `SUBSCRIPTION_PLAN_PRICING_LOCKED`). The change is not sent to API Portals.
 */
export const updateSubscriptionPlan = async (
  subscriptionPlanId: string,
  body: UpdateSubscriptionPlanBody,
  options?: RequestOptions
): Promise<SubscriptionPlan> => {
  return http.put<SubscriptionPlan>(resourcePath(subscriptionPlanId), body, {
    ...options,
    operationName: 'UpdateSubscriptionPlan',
  });
};

/**
 * Deletes a plan, only while nothing offers or uses it: otherwise 409
 * `SUBSCRIPTION_PLAN_IN_USE`, with a message that counts what does. The delete
 * is not sent to API Portals.
 */
export const deleteSubscriptionPlan = async (
  subscriptionPlanId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(subscriptionPlanId), {
    ...options,
    operationName: 'DeleteSubscriptionPlan',
  });
};

const pricingPath = (subscriptionPlanId: string): string => `${resourcePath(subscriptionPlanId)}/pricing`;

/**
 * A paid plan's pricing, set when the plan was created. A free plan has none
 * (404).
 */
export const getSubscriptionPlanPricing = async (
  subscriptionPlanId: string,
  options?: RequestOptions
): Promise<SubscriptionPlanPricing> => {
  return http.get<SubscriptionPlanPricing>(pricingPath(subscriptionPlanId), {
    ...options,
    operationName: 'GetSubscriptionPlanPricing',
  });
};
