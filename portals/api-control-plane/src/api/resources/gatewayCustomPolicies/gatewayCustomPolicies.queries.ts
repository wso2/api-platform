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
  getGatewayCustomPolicy,
  listGatewayCustomPolicies,
  type CustomPolicy,
  type CustomPolicyListResponse,
  type ListCustomPoliciesQuery,
} from './gatewayCustomPolicies.endpoints';

export const gatewayCustomPolicyKeys = createResourceKeys('gatewayCustomPolicies');

/** The server's maximum page size. */
const POLICIES_PAGE_SIZE = 100;
/** Caps the requests if `pagination.total` is wrong. */
const POLICIES_MAX_PAGES = 50;

/**
 * Reads every page of the organization's custom policies, skipping records already read.
 * Fails rather than return a partial list when the page cap is reached first.
 */
const fetchAllGatewayCustomPolicies = async (
  org: OrgScope,
  signal: AbortSignal
): Promise<CustomPolicyListResponse> => {
  const byUuid = new Map<string, CustomPolicy>();
  let received = 0;
  let total = 0;
  let complete = false;

  for (let page = 0; page < POLICIES_MAX_PAGES && !complete; page += 1) {
    const response = await listGatewayCustomPolicies({
      orgId: org,
      signal,
      query: { limit: POLICIES_PAGE_SIZE, offset: received },
    });
    // `list` is null when the organization has none.
    const items = response.list ?? [];
    received += items.length;
    total = response.pagination?.total ?? 0;
    items.forEach((policy) => byUuid.set(policy.uuid, policy));
    complete = items.length === 0 || received >= total;
  }

  if (!complete) {
    throw new Error(`Read ${received} of ${total} custom policies before the page limit.`);
  }

  const list = [...byUuid.values()];
  return {
    list,
    count: list.length,
    pagination: { total: Math.max(total, list.length), offset: 0, limit: list.length },
  };
};

/**
 * Composite id for a policy version.
 *
 * A policy is identified by id *and* version, so the detail key has to carry
 * both. Joining them into one segment keeps the key factory's `detail(org, id)`
 * shape intact rather than adding a two-argument variant used by one resource.
 */
export const policyVersionId = (
  gatewayCustomPolicyId: string,
  version: string
): string => {
    const escape = (s: string) => s.replace(/([,()])/g, '\\$1');
    return `(${escape(gatewayCustomPolicyId)},${escape(version)})`;
}

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks, so
 * the same definition drives `useQuery`, a router loader's `ensureQueryData`,
 * a prefetch on hover and `getQueryData` inside a mutation.
 *
 * Both use the `static` tier — the first resource to do so. A published policy
 * version is immutable: it cannot be edited, only synced afresh or deleted, so
 * refetching it during a session is pure waste.
 */
export const gatewayCustomPolicyQueries = {
  list: (org: OrgScope, query: ListCustomPoliciesQuery = {}) =>
    queryOptions({
      queryKey: gatewayCustomPolicyKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) =>
        listGatewayCustomPolicies({ orgId: org, signal, query }),
      staleTime: staleTimes.static,
    }),

  /** Every policy in the org, as one list response. */
  listAll: (org: OrgScope) =>
    queryOptions({
      queryKey: gatewayCustomPolicyKeys.list(org, { pages: 'all' }),
      queryFn: ({ signal }) => fetchAllGatewayCustomPolicies(org, signal),
      staleTime: staleTimes.static,
    }),

  detail: (org: OrgScope, gatewayCustomPolicyId: string, version: string) =>
    queryOptions({
      queryKey: gatewayCustomPolicyKeys.detail(
        org,
        policyVersionId(gatewayCustomPolicyId, version)
      ),
      queryFn: ({ signal }) =>
        getGatewayCustomPolicy(gatewayCustomPolicyId, version, {
          orgId: org,
          signal,
        }),
      staleTime: staleTimes.static,
    }),
};
