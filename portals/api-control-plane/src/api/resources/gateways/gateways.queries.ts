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
  getGateway,
  getGatewayManifest,
  listGatewayTokens,
  listGateways,
  type ListGatewaysQuery,
} from './gateways.endpoints';

export const gatewayKeys = createResourceKeys('gateways');

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks, so
 * the same definition drives `useQuery`, a router loader's `ensureQueryData`,
 * a prefetch on hover and `getQueryData` inside a mutation — all sharing one
 * key, one fetcher and one staleTime.
 *
 * Tokens and the manifest are keyed with `child()`, which files them beneath
 * the gateway's own detail entry: deleting a gateway then evicts its tokens and
 * manifest in the same call, with no separate bookkeeping.
 */
export const gatewayQueries = {
  list: (org: OrgScope, query: ListGatewaysQuery = {}) =>
    queryOptions({
      queryKey: gatewayKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away or changing filters
      // aborts the in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listGateways({ orgId: org, signal, query }),
      staleTime: staleTimes.stable,
    }),

  /**
   * A single gateway.
   *
   * Uses the `realtime` tier rather than `stable`: `isActive` flips when a
   * self-hosted agent connects, and this is the query the setup flow watches
   * for that transition.
   */
  detail: (org: OrgScope, gatewayId: string) =>
    queryOptions({
      queryKey: gatewayKeys.detail(org, gatewayId),
      queryFn: ({ signal }) => getGateway(gatewayId, { orgId: org, signal }),
      staleTime: staleTimes.realtime,
    }),

  tokens: (org: OrgScope, gatewayId: string) =>
    queryOptions({
      queryKey: gatewayKeys.child(org, gatewayId, 'tokens'),
      queryFn: ({ signal }) => listGatewayTokens(gatewayId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  manifest: (org: OrgScope, gatewayId: string) =>
    queryOptions({
      queryKey: gatewayKeys.child(org, gatewayId, 'manifest'),
      queryFn: ({ signal }) => getGatewayManifest(gatewayId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),
};
