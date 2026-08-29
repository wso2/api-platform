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

import { staleTimes } from '../../../core/queryClient';
import { type OrgScope } from '../../../core/queryKeys';
import {
  listRestApiGateways,
  type ListRestApiGatewaysQuery,
} from './apiGateways.endpoints';
import { restApiKeys } from '../restApis.queries';

/**
 * Keyed with `restApiKeys.child(...)`, beneath the API's own detail entry —
 * the same nesting `deployments` uses, and for the same reason: deleting an
 * API evicts its gateway associations in the same call, with no separate
 * bookkeeping.
 */
export const apiGatewayQueries = {
  list: (org: OrgScope, restApiId: string, query: ListRestApiGatewaysQuery = {}) =>
    queryOptions({
      queryKey: restApiKeys.child(org, restApiId, 'gateways', query),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) =>
        listRestApiGateways(restApiId, { orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),
};
