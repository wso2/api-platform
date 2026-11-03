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
  getDeployment,
  listDeployments,
  type Deployment,
  type ListDeploymentsQuery,
} from './deployments.endpoints';
import { graphQLApiKeys } from '../graphqlApis.queries';

/**
 * Deployments are keyed with `graphQLApiKeys.child(...)`, filing them beneath
 * their API's own detail entry — see `restApis/deployments/deployments.queries.ts`,
 * which this mirrors exactly.
 */
export const deploymentQueries = {
  list: (org: OrgScope, graphqlApiId: string, query: ListDeploymentsQuery = {}) =>
    queryOptions({
      queryKey: graphQLApiKeys.child(org, graphqlApiId, 'deployments', query),
      queryFn: ({ signal }) => listDeployments(graphqlApiId, { orgId: org, signal, query }),
      staleTime: staleTimes.realtime,
    }),

  detail: (org: OrgScope, graphqlApiId: string, deploymentId: string) =>
    queryOptions({
      queryKey: graphQLApiKeys.child(org, graphqlApiId, 'deployments', { deploymentId }),
      queryFn: ({ signal }) =>
        getDeployment(graphqlApiId, deploymentId, { orgId: org, signal }),
      staleTime: staleTimes.realtime,
    }),
};

/** Statuses the gateway has not finished acting on yet. */
const TRANSITIONAL: ReadonlySet<Deployment['status']> = new Set(['DEPLOYING', 'UNDEPLOYING']);

/** True while any deployment in the list is still settling. */
export const hasTransitioningDeployment = (
  deployments: readonly Deployment[] | undefined,
): boolean => Boolean(deployments?.some((d) => TRANSITIONAL.has(d.status)));

export const isTransitioning = (deployment: Deployment | undefined): boolean =>
  Boolean(deployment && TRANSITIONAL.has(deployment.status));
