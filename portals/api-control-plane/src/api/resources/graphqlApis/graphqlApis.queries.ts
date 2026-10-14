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
  getGraphQLApi,
  getGraphQLApiSdl,
  listGraphQLApis,
  type ListGraphQLApisQuery,
} from './graphqlApis.endpoints';

export const graphQLApiKeys = createResourceKeys('graphqlApis');

/** Query definitions as `queryOptions` objects — see `restApiQueries` for why. */
export const graphQLApiQueries = {
  list: (org: OrgScope, query: ListGraphQLApisQuery) =>
    queryOptions({
      queryKey: graphQLApiKeys.list(org, query),
      queryFn: ({ signal }) => listGraphQLApis({ orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),

  detail: (org: OrgScope, graphqlApiId: string) =>
    queryOptions({
      queryKey: graphQLApiKeys.detail(org, graphqlApiId),
      queryFn: ({ signal }) => getGraphQLApi(graphqlApiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  /** Kept under the detail's own `sdl` child so deleting the API evicts it too. */
  sdl: (org: OrgScope, graphqlApiId: string) =>
    queryOptions({
      queryKey: graphQLApiKeys.child(org, graphqlApiId, 'sdl'),
      queryFn: ({ signal }) => getGraphQLApiSdl(graphqlApiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),
};
