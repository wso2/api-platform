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
  type DeployApiBody,
  type DeploymentListResponse,
  type ListDeploymentsQuery,
} from './deployments.endpoints';
import { deploymentQueries, hasTransitioningDeployment } from './deployments.queries';
import { restApiKeys } from '../restApis.queries';

/**
 * The public hook surface for an API's deployments.
 *
 * Every hook here takes `restApiId` explicitly rather than reading it from
 * scope. The route's `apiHandler` is not always the API being acted on — a
 * deploy dialog opened from a list acts on a row, not on the page — so making
 * the parent an argument keeps that case honest.
 */

export type DeploymentListFilters = ListDeploymentsQuery;

/* -------------------------------------------------------------------------- */
/* Polling                                                                     */
/* -------------------------------------------------------------------------- */

const POLL_START_MS = 2_000;
const POLL_FACTOR = 1.5;
const POLL_CAP_MS = 30_000;

/**
 * Stop polling after this many rounds. At the backoff above, that is roughly
 * ten minutes — past which a deployment stuck in DEPLOYING is a backend problem
 * the UI should surface rather than keep waiting on.
 */
const MAX_POLL_ROUNDS = 25;

/**
 * Backing-off poll interval, used while any deployment is still settling.
 *
 * Three properties matter, and the previous layer had none of them: it backs
 * off rather than hammering a fixed four seconds, it stops at a bound rather
 * than polling for the life of the tab, and it pauses while the tab is hidden
 * (`refetchIntervalInBackground` is left at its default of false).
 *
 * The round count is approximated by `dataUpdateCount`, which also counts
 * fetches from before polling began. That errs toward backing off sooner, which
 * is the safe direction.
 */
const pollWhileTransitioning = (
  updateCount: number,
  transitioning: boolean
): number | false => {
  if (!transitioning) return false;
  if (updateCount > MAX_POLL_ROUNDS) return false;
  return Math.min(POLL_START_MS * POLL_FACTOR ** updateCount, POLL_CAP_MS);
};

/* -------------------------------------------------------------------------- */
/* Queries                                                                     */
/* -------------------------------------------------------------------------- */

/**
 * Deployments of one API, polling while any of them is mid-transition.
 *
 * platform-api flips the status once the gateway acknowledges, so the list
 * settles on its own and polling stops without the caller doing anything.
 */
export const useDeployments = (
  restApiId: string | undefined,
  filters: DeploymentListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...deploymentQueries.list(org!, restApiId!, filters),
    enabled: Boolean(org && restApiId),
    refetchInterval: (query) =>
      pollWhileTransitioning(
        query.state.dataUpdateCount,
        hasTransitioningDeployment(query.state.data?.list)
      ),
  });
};

/** A single deployment, polling while it is mid-transition. */
export const useDeployment = (
  restApiId: string | undefined,
  deploymentId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...deploymentQueries.detail(org!, restApiId!, deploymentId!),
    enabled: Boolean(org && restApiId && deploymentId),
    refetchInterval: (query) =>
      pollWhileTransitioning(
        query.state.dataUpdateCount,
        hasTransitioningDeployment(
          query.state.data ? [query.state.data] : undefined
        )
      ),
  });
};

/* -------------------------------------------------------------------------- */
/* Mutations                                                                   */
/* -------------------------------------------------------------------------- */

/**
 * Invalidation helper shared by every deployment mutation.
 *
 * Invalidates the parent API's `deployments` child key, which covers every
 * filtered variant of the list plus the individual detail entries beneath it.
 * The API's own detail is invalidated too, because its summary carries
 * deployment state the list change has just made stale.
 */
const useInvalidateDeployments = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return (restApiId: string) => {
    if (!org) return;
    void queryClient.invalidateQueries({
      queryKey: restApiKeys.child(org, restApiId, 'deployments'),
    });
    void queryClient.invalidateQueries({
      queryKey: restApiKeys.detail(org, restApiId),
    });
  };
};

export const useDeployApi = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<
    Deployment,
    ApiError,
    { restApiId: string; body: DeployApiBody }
  >({
    mutationFn: ({ restApiId, body }) => deployApi(restApiId, body, { orgId }),
    // No optimistic write: the server assigns the deployment id and its initial
    // status, and the refetch that follows starts the poll that watches it
    // settle.
    onSuccess: (_deployment, { restApiId }) => invalidate(restApiId),
  });
};

export const useUndeployDeployment = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<
    Deployment,
    ApiError,
    { restApiId: string; deploymentId: string }
  >({
    mutationFn: ({ restApiId, deploymentId }) =>
      undeployDeployment(restApiId, deploymentId, { orgId }),
    onSuccess: (_deployment, { restApiId }) => invalidate(restApiId),
  });
};

export const useRestoreDeployment = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<
    Deployment,
    ApiError,
    { restApiId: string; deploymentId: string }
  >({
    mutationFn: ({ restApiId, deploymentId }) =>
      restoreDeployment(restApiId, deploymentId, { orgId }),
    onSuccess: (_deployment, { restApiId }) => invalidate(restApiId),
  });
};

export const useDeleteDeployment = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateDeployments(orgId);

  return useMutation<void, ApiError, { restApiId: string; deploymentId: string }>({
    mutationFn: ({ restApiId, deploymentId }) =>
      deleteDeployment(restApiId, deploymentId, { orgId }),
    onSuccess: (_result, { restApiId, deploymentId }) => {
      if (org) {
        // Drop the detail entry outright — refetching a deleted deployment just
        // to receive a 404 is a wasted round trip and an error the user would
        // briefly see.
        queryClient.removeQueries({
          queryKey: restApiKeys.child(org, restApiId, `deployments/${deploymentId}`),
        });
      }
      invalidate(restApiId);
    },
  });
};

/**
 * Selector example: a status badge only needs the counts, so it should not
 * re-render when an unrelated field changes.
 */
export const useDeploymentSummary = (
  restApiId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...deploymentQueries.list(org!, restApiId!),
    enabled: Boolean(org && restApiId),
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
