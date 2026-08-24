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
  getApplication,
  listApplicationApiKeys,
  listApplicationAssociations,
  listApplications,
  listAssociationApiKeys,
  type ListApplicationApiKeysQuery,
  type ListApplicationAssociationsQuery,
  type ListApplicationsQuery,
  type ListAssociationApiKeysQuery,
} from './applications.endpoints';

export const applicationKeys = createResourceKeys('applications');

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks, so
 * the same definition drives `useQuery`, a router loader's `ensureQueryData`,
 * a prefetch on hover and `getQueryData` inside a mutation — all sharing one
 * key, one fetcher and one staleTime.
 *
 * Keys and associations are filed with `child()`, beneath the application's own
 * detail entry, so deleting an application evicts everything under it in one
 * call. Association keys nest a level deeper still, which the child segment
 * expresses as a path rather than by adding another factory method.
 */
export const applicationQueries = {
  list: (org: OrgScope, query: ListApplicationsQuery) =>
    queryOptions({
      queryKey: applicationKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away or changing filters
      // aborts the in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listApplications({ orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  detail: (org: OrgScope, applicationId: string) =>
    queryOptions({
      queryKey: applicationKeys.detail(org, applicationId),
      queryFn: ({ signal }) =>
        getApplication(applicationId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  apiKeys: (
    org: OrgScope,
    applicationId: string,
    query: ListApplicationApiKeysQuery = {}
  ) =>
    queryOptions({
      queryKey: applicationKeys.child(org, applicationId, 'api-keys', query),
      queryFn: ({ signal }) =>
        listApplicationApiKeys(applicationId, { orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  associations: (
    org: OrgScope,
    applicationId: string,
    query: ListApplicationAssociationsQuery = {}
  ) =>
    queryOptions({
      queryKey: applicationKeys.child(org, applicationId, 'associations', query),
      queryFn: ({ signal }) =>
        listApplicationAssociations(applicationId, { orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  associationApiKeys: (
    org: OrgScope,
    applicationId: string,
    associationId: string,
    query: ListAssociationApiKeysQuery = {}
  ) =>
    queryOptions({
      queryKey: applicationKeys.child(
        org,
        applicationId,
        `associations/${associationId}/api-keys`,
        query
      ),
      queryFn: ({ signal }) =>
        listAssociationApiKeys(applicationId, associationId, {
          orgId: org,
          signal,
          query,
        }),
      staleTime: staleTimes.standard,
    }),
};
