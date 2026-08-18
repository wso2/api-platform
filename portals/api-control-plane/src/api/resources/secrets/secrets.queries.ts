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
  getSecret,
  listSecrets,
  type ListSecretsQuery,
} from './secrets.endpoints';

export const secretKeys = createResourceKeys('secrets');

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks, so
 * the same definition drives `useQuery`, a router loader's `ensureQueryData`,
 * a prefetch on hover and `getQueryData` inside a mutation.
 *
 * Only metadata is ever cached here. Neither read returns a secret's value, so
 * nothing sensitive enters the store — which is also why the rotate mutation
 * does not seed its response into the cache.
 */
export const secretQueries = {
  list: (org: OrgScope, query: ListSecretsQuery = {}) =>
    queryOptions({
      queryKey: secretKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away or changing filters
      // aborts the in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listSecrets({ orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  detail: (org: OrgScope, secretId: string) =>
    queryOptions({
      queryKey: secretKeys.detail(org, secretId),
      queryFn: ({ signal }) => getSecret(secretId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),
};
