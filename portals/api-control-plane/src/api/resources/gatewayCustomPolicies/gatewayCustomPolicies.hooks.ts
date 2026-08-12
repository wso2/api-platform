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

import type { ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  deleteGatewayCustomPolicy,
  syncCustomPolicy,
  type CustomPolicy,
  type CustomPolicyListResponse,
  type ListCustomPoliciesQuery,
} from './gatewayCustomPolicies.endpoints';
import {
  gatewayCustomPolicyKeys,
  gatewayCustomPolicyQueries,
  policyVersionId,
} from './gatewayCustomPolicies.queries';

/**
 * The public hook surface for gateway custom policies — the only thing
 * components import.
 *
 * Scope is implicit but overridable, and `query.error` is always an `ApiError`
 * with `.code`, `.fieldErrors` and `.status`.
 *
 * There is no create or update hook, because the API has no such operation:
 * policies enter the control plane by being synced from a gateway.
 */

export type CustomPolicyListFilters = ListCustomPoliciesQuery;

/** Policies published to gateways in the active organization. */
export const useGatewayCustomPolicies = (
  filters: CustomPolicyListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...gatewayCustomPolicyQueries.list(org!, filters),
    enabled: Boolean(org),
  });
};

/**
 * One version of one policy.
 *
 * Both the id and the version are required — a policy id alone does not
 * identify a definition.
 */
export const useGatewayCustomPolicy = (
  gatewayCustomPolicyId: string | undefined,
  version: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...gatewayCustomPolicyQueries.detail(org!, gatewayCustomPolicyId!, version!),
    enabled: Boolean(org && gatewayCustomPolicyId && version),
  });
};

/**
 * Invalidation helper shared by every policy mutation.
 *
 * Invalidates the resource root rather than a specific list key, because a sync
 * or delete shifts pagination and counts on list pages the user has not visited
 * yet.
 */
const useInvalidateGatewayCustomPolicies = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({
      queryKey: gatewayCustomPolicyKeys.all(org),
    });
  };
};

/**
 * Pulls a policy definition from a gateway into the control plane.
 *
 * All three arguments are required by the spec as query parameters. This
 * replaces the create/update pair other resources have — a policy is never
 * authored here, only imported.
 */
export const useSyncCustomPolicy = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateGatewayCustomPolicies(orgId);

  return useMutation<
    CustomPolicy,
    ApiError,
    { gatewayId: string; policyName: string; policyVersion: string }
  >({
    mutationFn: ({ gatewayId, policyName, policyVersion }) =>
      syncCustomPolicy({
        orgId,
        query: { gatewayId, policyName, policyVersion },
      }),
    onSuccess: (synced) => {
      // Seed the detail cache from the sync response so opening the policy
      // renders instantly instead of showing a loading state for data the
      // server already gave us.
      if (org) {
        queryClient.setQueryData(
          gatewayCustomPolicyKeys.detail(
            org,
            policyVersionId(synced.name, synced.version)
          ),
          synced
        );
      }
      invalidate();
    },
  });
};

/**
 * Deletes one version of a policy.
 *
 * Other versions of the same policy are unaffected, so only that version's
 * cache entry is removed.
 */
export const useDeleteGatewayCustomPolicy = (
  overrides: { orgId?: string } = {}
) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateGatewayCustomPolicies(orgId);

  return useMutation<
    void,
    ApiError,
    { gatewayCustomPolicyId: string; version: string }
  >({
    mutationFn: ({ gatewayCustomPolicyId, version }) =>
      deleteGatewayCustomPolicy(gatewayCustomPolicyId, version, { orgId }),
    onSuccess: (_result, { gatewayCustomPolicyId, version }) => {
      if (org) {
        // Drop the detail entry outright — refetching a deleted version just to
        // receive a 404 is a wasted round trip and an error the user would
        // briefly see.
        queryClient.removeQueries({
          queryKey: gatewayCustomPolicyKeys.detail(
            org,
            policyVersionId(gatewayCustomPolicyId, version)
          ),
        });
      }
      invalidate();
    },
  });
};

/**
 * Selector example: a policy picker only needs name/version, so it should not
 * re-render when an unrelated field changes.
 */
export const useGatewayCustomPolicyOptions = (
  filters: CustomPolicyListFilters = {}
) => {
  const { org } = useApiScope();

  return useQuery({
    ...gatewayCustomPolicyQueries.list(org!, filters),
    enabled: Boolean(org),
    select: (data: CustomPolicyListResponse) =>
      (data.list ?? []).map((policy) => ({
        id: policyVersionId(policy.name, policy.version),
        label: `${policy.name} ${policy.version}`,
        name: policy.name,
        version: policy.version,
      })),
  });
};
