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
  getServiceAccount,
  listServiceAccountRoles,
  listServiceAccounts,
  type ListServiceAccountsQuery,
} from './serviceAccounts.endpoints';

export const serviceAccountKeys = createResourceKeys('serviceAccounts');

/** Only reads are cached, and no read returns a secret. */
export const serviceAccountQueries = {
  list: (org: OrgScope, query: ListServiceAccountsQuery = {}) =>
    queryOptions({
      queryKey: serviceAccountKeys.list(org, query),
      queryFn: ({ signal }) => listServiceAccounts({ orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  detail: (org: OrgScope, id: string) =>
    queryOptions({
      queryKey: serviceAccountKeys.detail(org, id),
      queryFn: ({ signal }) => getServiceAccount(id, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  // The role list comes from the server's mapping file, read once at startup.
  roles: (org: OrgScope) =>
    queryOptions({
      queryKey: [...serviceAccountKeys.all(org), 'roles'] as const,
      queryFn: ({ signal }) => listServiceAccountRoles({ orgId: org, signal }),
      staleTime: staleTimes.stable,
    }),
};
