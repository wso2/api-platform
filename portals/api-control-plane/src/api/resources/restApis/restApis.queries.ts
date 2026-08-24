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
  getRestApi,
  listRestApis,
  type ListRestApisQuery,
} from './restApis.endpoints';

export const restApiKeys = createResourceKeys('restApis');

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks.
 *
 * This is the pattern that pays off at scale. A `queryOptions` object is a
 * plain value, so the same definition drives `useQuery`, `useSuspenseQuery`,
 * a router loader's `queryClient.ensureQueryData`, a prefetch on hover, and
 * `getQueryData` inside a mutation's optimistic update — all sharing one key,
 * one fetcher and one staleTime. The alternative (defining these inside hooks)
 * forces every non-component caller to re-derive the key by hand, which is
 * exactly how key drift and stale-cache bugs start.
 */
export const restApiQueries = {
  list: (org: OrgScope, query: ListRestApisQuery) =>
    queryOptions({
      queryKey: restApiKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away or changing filters
      // aborts the in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listRestApis({ orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  detail: (org: OrgScope, restApiId: string) =>
    queryOptions({
      queryKey: restApiKeys.detail(org, restApiId),
      queryFn: ({ signal }) => getRestApi(restApiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),
};
