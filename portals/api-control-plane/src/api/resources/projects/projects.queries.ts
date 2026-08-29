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
  getProject,
  listProjects,
  type ListProjectsQuery,
} from './projects.endpoints';

export const projectKeys = createResourceKeys('projects');

/**
 * Query definitions, expressed as `queryOptions` objects rather than hooks, so
 * the same definition drives `useQuery`, a router loader's `ensureQueryData`,
 * a prefetch on hover and `getQueryData` inside a mutation — all sharing one
 * key, one fetcher and one staleTime.
 *
 * `org` is threaded into both the key and the request: into the key so two
 * organizations can never share an entry, and into the request so the
 * transport can attach `X-Org-Id`. Passing it explicitly rather than reading a
 * context is what keeps these usable outside a component.
 */
export const projectQueries = {
  list: (org: OrgScope, query: ListProjectsQuery = {}) =>
    queryOptions({
      queryKey: projectKeys.list(org, query),
      // `signal` comes from TanStack Query: navigating away or changing filters
      // aborts the in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) => listProjects({ orgId: org, signal, query }),
      // The `stable` tier: a project's name and description change far less
      // often than the APIs inside it.
      staleTime: staleTimes.stable,
    }),

  detail: (org: OrgScope, projectId: string) =>
    queryOptions({
      queryKey: projectKeys.detail(org, projectId),
      queryFn: ({ signal }) => getProject(projectId, { orgId: org, signal }),
      staleTime: staleTimes.stable,
    }),
};
