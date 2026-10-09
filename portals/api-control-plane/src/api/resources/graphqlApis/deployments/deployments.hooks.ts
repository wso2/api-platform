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

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '../../../core/errors';
import { useApiScope } from '../../../core/scope';
import {
  deleteDeployment,
  deployApi,
  restoreDeployment,
  undeployDeployment,
  type Deployment,
  type DeployGraphQLApiBody,
  type DeploymentListResponse,
  type ListDeploymentsQuery,
} from './deployments.endpoints';
import { deploymentQueries, hasTransitioningDeployment } from './deployments.queries';
import { graphQLApiKeys } from '../graphqlApis.queries';

/**
 * The public hook surface for a GraphQL API's deployments — mirrors
 * `restApis/deployments/deployments.hooks.ts` exactly, including the
 * backing-off poll while a deployment is settling. See that file for the
 * reasoning behind the poll shape.
 */

export type DeploymentListFilters = ListDeploymentsQuery;

const POLL_START_MS = 2_000;
const POLL_FACTOR = 1.5;
const POLL_CAP_MS = 30_000;
const MAX_POLL_ROUNDS = 25;

const pollWhileTransitioning = (
  updateCount: number,
  transitioning: boolean,
): number | false => {
  if (!transitioning) return false;
  if (updateCount > MAX_POLL_ROUNDS) return false;
  return Math.min(POLL_START_MS * POLL_FACTOR ** updateCount, POLL_CAP_MS);
};

export const useDeployments = (
  graphqlApiId: string | undefined,
  filters: DeploymentListFilters = {},
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...deploymentQueries.list(org!, graphqlApiId!, filters),
    enabled: Boolean(org && graphqlApiId),
    refetchInterval: (query) =>
      pollWhileTransitioning(
        query.state.dataUpdateCount,
        hasTransitioningDeployment(query.state.data?.list),
      ),
  });
};

export const useDeployment = (
  graphqlApiId: string | undefined,
  deploymentId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...deploymentQueries.detail(org!, graphqlApiId!, deploymentId!),
    enabled: Boolean(org && graphqlApiId && deploymentId),
    refetchInterval: (query) =>
      pollWhileTransitioning(
        query.state.dataUpdateCount,
        hasTransitioningDeployment(query.state.data ? [query.state.data] : undefined),
      ),
  });
};

const useInvalidateDeployments = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return (graphqlApiId: string) => {
    if (!org) return;
    void queryClient.invalidateQueries({
      queryKey: graphQLApiKeys.children(org, graphqlApiId, 'deployments'),
    });
    void queryClient.invalidateQueries({
      queryKey: graphQLApiKeys.detail(org, graphqlApiId),
    });
  };
};

export const useDeployApi = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<
    Deployment,
    ApiError,
    { graphqlApiId: string; body: DeployGraphQLApiBody }
  >({
    mutationFn: ({ graphqlApiId, body }) => deployApi(graphqlApiId, body, { orgId }),
    onSuccess: (_deployment, { graphqlApiId }) => invalidate(graphqlApiId),
  });
};

export const useUndeployDeployment = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<Deployment, ApiError, { graphqlApiId: string; deploymentId: string }>({
    mutationFn: ({ graphqlApiId, deploymentId }) =>
      undeployDeployment(graphqlApiId, deploymentId, { orgId }),
    onSuccess: (_deployment, { graphqlApiId }) => invalidate(graphqlApiId),
  });
};

export const useRestoreDeployment = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<Deployment, ApiError, { graphqlApiId: string; deploymentId: string }>({
    mutationFn: ({ graphqlApiId, deploymentId }) =>
      restoreDeployment(graphqlApiId, deploymentId, { orgId }),
    onSuccess: (_deployment, { graphqlApiId }) => invalidate(graphqlApiId),
  });
};

export const useDeleteDeployment = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<void, ApiError, { graphqlApiId: string; deploymentId: string }>({
    mutationFn: ({ graphqlApiId, deploymentId }) =>
      deleteDeployment(graphqlApiId, deploymentId, { orgId }),
    onSuccess: (_result, { graphqlApiId, deploymentId }) => {
      if (org) {
        queryClient.removeQueries({
          queryKey: graphQLApiKeys.child(org, graphqlApiId, 'deployments', { deploymentId }),
        });
      }
      invalidate(graphqlApiId);
    },
  });
};

export const useDeploymentSummary = (
  graphqlApiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...deploymentQueries.list(org!, graphqlApiId!),
    enabled: Boolean(org && graphqlApiId),
    select: (data: DeploymentListResponse) => {
      const deployments = data.list ?? [];
      return {
        total: data.pagination?.total ?? deployments.length,
        deployed: deployments.filter((d) => d.status === 'DEPLOYED').length,
        failed: deployments.filter((d) => d.status === 'FAILED').length,
        isSettling: hasTransitioningDeployment(deployments),
      };
    },
  });
};
