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
import { listMyApiKeys, type ListMyApiKeysQuery } from './apiKeys.endpoints';

/**
 * Keyed as its own org-scoped resource rather than as a child of `restApis`,
 * because the only read the spec offers spans artifacts: `/me/api-keys` returns
 * the caller's keys across REST APIs, LLM providers and LLM proxies at once.
 * Filing that under one parent API would be a lie about what the entry holds.
 *
 * Consequence worth knowing: revoking a key from an API screen invalidates this
 * list, not a per-API one, because a per-API list does not exist.
 */
export const apiKeyKeys = createResourceKeys('apiKeys');

export const apiKeyQueries = {
  /**
   * The signed-in user's keys. `standard` rather than `stable`: keys are
   * created and revoked interactively, so a stale list is immediately visible
   * as a wrong answer.
   */
  mine: (org: OrgScope, query: ListMyApiKeysQuery = {}) =>
    queryOptions({
      queryKey: apiKeyKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listMyApiKeys({ orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),
};
