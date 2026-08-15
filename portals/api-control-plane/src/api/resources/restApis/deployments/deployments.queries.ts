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
import { restApiKeys } from '../restApis.queries';

/**
 * Deployments are keyed with `restApiKeys.child(...)`, which files them beneath
 * their API's own detail entry. That nesting is what makes deleting an API
 * evict its deployments in the same call, with no separate bookkeeping — and it
 * is why this module reuses the parent's key factory rather than declaring one.
 *
 * Both queries use the `realtime` tier: a deployment's status is the thing the
 * user is actively watching change.
 */
export const deploymentQueries = {
  list: (org: OrgScope, restApiId: string, query: ListDeploymentsQuery = {}) =>
    queryOptions({
      queryKey: restApiKeys.child(org, restApiId, 'deployments', query),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) =>
        listDeployments(restApiId, { orgId: org, signal, query }),
      staleTime: staleTimes.realtime,
    }),

  detail: (org: OrgScope, restApiId: string, deploymentId: string) =>
    queryOptions({
      // Keeps the detail query under the deployments prefix so invalidations
      // reach it, while avoiding collisions with the list key.
      queryKey: restApiKeys.child(org, restApiId, 'deployments', { deploymentId }),
      queryFn: ({ signal }) =>
        getDeployment(restApiId, deploymentId, { orgId: org, signal }),
      staleTime: staleTimes.realtime,
    }),
};

/** Statuses the gateway has not finished acting on yet. */
const TRANSITIONAL: ReadonlySet<Deployment['status']> = new Set([
  'DEPLOYING',
  'UNDEPLOYING',
]);

/** True while any deployment in the list is still settling. */
export const hasTransitioningDeployment = (
  deployments: readonly Deployment[] | undefined
): boolean => Boolean(deployments?.some((d) => TRANSITIONAL.has(d.status)));

export const isTransitioning = (deployment: Deployment | undefined): boolean =>
  Boolean(deployment && TRANSITIONAL.has(deployment.status));
