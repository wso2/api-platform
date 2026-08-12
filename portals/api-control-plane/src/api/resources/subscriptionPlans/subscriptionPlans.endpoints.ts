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

export const createSubscriptionPlan = async (
  body: CreateSubscriptionPlanBody,
  options?: RequestOptions
): Promise<SubscriptionPlan> => {
  return http.post<SubscriptionPlan>(BASE, body, {
    ...options,
    operationName: 'CreateSubscriptionPlan',
  });
};

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
 * Deletes a plan. The backend refuses while subscriptions still reference it,
 * answering with a specific `code` that reaches the caller as `ApiError.code`.
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
