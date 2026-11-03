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
import { listGraphQLApiGateways, type ListGraphQLApiGatewaysQuery } from './apiGateways.endpoints';
import { graphQLApiKeys } from '../graphqlApis.queries';

/**
 * Keyed with `graphQLApiKeys.child(...)`, beneath the API's own detail entry —
 * the same nesting `deployments` uses, so deleting the API evicts its gateway
 * associations in the same call.
 */
export const apiGatewayQueries = {
  list: (org: OrgScope, graphqlApiId: string, query: ListGraphQLApiGatewaysQuery = {}) =>
    queryOptions({
      queryKey: graphQLApiKeys.child(org, graphqlApiId, 'gateways', query),
      queryFn: ({ signal }) => listGraphQLApiGateways(graphqlApiId, { orgId: org, signal, query }),
      staleTime: staleTimes.standard,
    }),
};
