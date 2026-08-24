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
 * Transport layer for `/subscriptions`. One thin function per spec operation:
 * no branching, no adapters, no cache awareness — just "call this endpoint
 * with these arguments and get the spec's response type back".
 *
 * One asymmetry to know about: `GetSubscription` needs only the subscription
 * id, but update and delete additionally require `subscriberId` as a **query**
 * parameter. That is the spec's shape, not a wrapper convention, so the
 * argument is threaded explicitly rather than hidden.
 */

export type Subscription = Schema<'Subscription'>;
export type SubscriptionListResponse = ResponseOf<'ListSubscriptions'>;
export type ListSubscriptionsQuery = QueryOf<'ListSubscriptions'>;
export type CreateSubscriptionBody = BodyOf<'CreateSubscription'>;
export type UpdateSubscriptionBody = BodyOf<'UpdateSubscription'>;

/** The `subscriberId` that update and delete require alongside the path id. */
export type SubscriberQuery = QueryOf<'UpdateSubscription'>;

const BASE = '/subscriptions';

const resourcePath = (
  subscriptionId: PathOf<'GetSubscription'>['subscriptionId']
): string => `${BASE}/${encodeURIComponent(subscriptionId)}`;

export const listSubscriptions = async (
  options?: RequestOptions
): Promise<SubscriptionListResponse> => {
  return http.get<SubscriptionListResponse>(BASE, {
    ...options,
    operationName: 'ListSubscriptions',
  });
};

export const getSubscription = async (
  subscriptionId: string,
  options?: RequestOptions
): Promise<Subscription> => {
  return http.get<Subscription>(resourcePath(subscriptionId), {
    ...options,
    operationName: 'GetSubscription',
  });
};

export const createSubscription = async (
  body: CreateSubscriptionBody,
  options?: RequestOptions
): Promise<Subscription> => {
  return http.post<Subscription>(BASE, body, {
    ...options,
    operationName: 'CreateSubscription',
  });
};

type UpdateSubscriptionOptions = Omit<RequestOptions, 'query'> & {
  query: QueryOf<'UpdateSubscription'>;
};

type DeleteSubscriptionOptions = Omit<RequestOptions, 'query'> & {
  query: QueryOf<'DeleteSubscription'>;
};

/** Requires `subscriberId` in `options.query`; omitting it is a 400. */
export const updateSubscription = async (
  subscriptionId: string,
  body: UpdateSubscriptionBody,
  options: UpdateSubscriptionOptions
): Promise<Subscription> => {
  return http.put<Subscription>(resourcePath(subscriptionId), body, {
    ...options,
    operationName: 'UpdateSubscription',
  });
};

/** Requires `subscriberId` in `options.query`; omitting it is a 400. */
export const deleteSubscription = async (
  subscriptionId: string,
  options: DeleteSubscriptionOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(subscriptionId), {
    ...options,
    operationName: 'DeleteSubscription',
  });
};
