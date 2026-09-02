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
import { createGlobalResourceKeys } from '../../core/queryKeys';
import {
  getPolicyDefinition,
  listPolicies,
  listPolicyCategories,
  listPolicyVersions,
} from './policyHub.endpoints';

/**
 * Global keys, not org-scoped: the Policy Hub is a shared catalog served from
 * its own origin with no `X-Org-Id`, so filing it under `scopeKey(org)` would
 * both misdescribe it and throw the catalog away on every org switch.
 */
export const policyHubKeys = createGlobalResourceKeys('policyHub');

/**
 * Every query here sits on the `static` tier. A published policy version is
 * immutable by construction — a change to a policy ships as a new version — so
 * the catalog is as close to fixed-per-session as anything in this app gets.
 */
export const policyHubQueries = {
  policies: (page: number, pageSize: number, categories: string[] = []) =>
    queryOptions({
      queryKey: policyHubKeys.list({ page, pageSize, categories }),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listPolicies(page, pageSize, categories, { signal }),
      staleTime: staleTimes.static,
    }),

  categories: () =>
    queryOptions({
      queryKey: [...policyHubKeys.all(), 'categories'] as const,
      queryFn: ({ signal }) => listPolicyCategories({ signal }),
      staleTime: staleTimes.static,
    }),

  versions: (name: string) =>
    queryOptions({
      queryKey: [...policyHubKeys.detail(name), 'versions'] as const,
      queryFn: ({ signal }) => listPolicyVersions(name, { signal }),
      staleTime: staleTimes.static,
    }),

  definition: (name: string, version: string) =>
    queryOptions({
      queryKey: [...policyHubKeys.detail(name), 'definition', version] as const,
      queryFn: ({ signal }) => getPolicyDefinition(name, version, { signal }),
      staleTime: staleTimes.static,
    }),
};
