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
import { getMeter, listMeters, type ListMetersQuery } from './meters.endpoints';

export const meterKeys = createResourceKeys('meters');

/**
 * Query definitions. Meters are configuration an administrator edits
 * occasionally, and the plan dialog reads the list on every open, so both use
 * the `stable` tier.
 */
export const meterQueries = {
  list: (org: OrgScope, query: ListMetersQuery = {}) =>
    queryOptions({
      queryKey: meterKeys.list(org, query),
      queryFn: ({ signal }) => listMeters({ orgId: org, signal, query }),
      staleTime: staleTimes.stable,
    }),

  detail: (org: OrgScope, meterId: string) =>
    queryOptions({
      queryKey: meterKeys.detail(org, meterId),
      queryFn: ({ signal }) => getMeter(meterId, { orgId: org, signal }),
      staleTime: staleTimes.stable,
    }),
};
