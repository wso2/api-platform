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
  createSubscriptionPlan,
  deleteSubscriptionPlan,
  updateSubscriptionPlan,
  type CreateSubscriptionPlanBody,
  type ListSubscriptionPlansQuery,
  type SubscriptionPlan,
  type SubscriptionPlanListResponse,
  type UpdateSubscriptionPlanBody,
} from './subscriptionPlans.endpoints';
import {
  subscriptionPlanKeys,
  subscriptionPlanQueries,
} from './subscriptionPlans.queries';

/**
 * The public hook surface for subscription plans — the only thing components
 * import. Scope is implicit but overridable, and `query.error` is always an
 * `ApiError` with `.code`, `.fieldErrors` and `.status`.
 */

export type SubscriptionPlanListFilters = ListSubscriptionPlansQuery;

/** Plans available in the active organization. */
export const useSubscriptionPlans = (
  filters: SubscriptionPlanListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...subscriptionPlanQueries.list(org!, filters),
    enabled: Boolean(org),
  });
};

/** A single plan. */
export const useSubscriptionPlan = (
  subscriptionPlanId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...subscriptionPlanQueries.detail(org!, subscriptionPlanId!),
    enabled: Boolean(org && subscriptionPlanId),
  });
};

/**
 * Invalidation helper shared by every plan mutation.
 *
 * Invalidates the resource root rather than a specific list key, because a
 * create or delete shifts pagination and counts on list pages the user has not
 * visited yet.
 */
const useInvalidateSubscriptionPlans = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: subscriptionPlanKeys.all(org) });
  };
};

export const useCreateSubscriptionPlan = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSubscriptionPlans(orgId);

  return useMutation<SubscriptionPlan, ApiError, CreateSubscriptionPlanBody>({
    mutationFn: (body) => createSubscriptionPlan(body, { orgId }),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so navigating straight
      // to the new plan renders instantly instead of showing a loading state
      // for data the server already gave us.
      if (org && created.id) {
        queryClient.setQueryData(
          subscriptionPlanKeys.detail(org, created.id),
          created
        );
      }
      invalidate();
    },
  });
};

export const useUpdateSubscriptionPlan = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSubscriptionPlans(orgId);

  return useMutation<
    SubscriptionPlan,
    ApiError,
    { subscriptionPlanId: string; body: UpdateSubscriptionPlanBody },
    { previous?: SubscriptionPlan }
  >({
    mutationFn: ({ subscriptionPlanId, body }) =>
      updateSubscriptionPlan(subscriptionPlanId, body, { orgId }),

    // Optimistic update: the edit appears immediately, and rolls back exactly
    // if the server rejects it. `cancelQueries` first is not optional — an
    // in-flight refetch that resolves after this write would otherwise clobber
    // the optimistic value with stale server data.
    onMutate: async ({ subscriptionPlanId, body }) => {
      if (!org) return {};
      const key = subscriptionPlanKeys.detail(org, subscriptionPlanId);
      await queryClient.cancelQueries({ queryKey: key });

      const previous = queryClient.getQueryData<SubscriptionPlan>(key);
      if (previous) {
        queryClient.setQueryData<SubscriptionPlan>(key, { ...previous, ...body });
      }
      return { previous };
    },

    onError: (_error, { subscriptionPlanId }, context) => {
      if (org && context?.previous) {
        queryClient.setQueryData(
          subscriptionPlanKeys.detail(org, subscriptionPlanId),
          context.previous
        );
      }
    },

    onSettled: () => invalidate(),
  });
};

/**
 * Deletes a plan.
 *
 * The backend refuses while subscriptions still reference it, answering with a
 * specific `code`. Callers should branch on `ApiError.code` to explain which
 * subscriptions are blocking rather than showing a generic failure.
 */
export const useDeleteSubscriptionPlan = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSubscriptionPlans(orgId);

  return useMutation<void, ApiError, { subscriptionPlanId: string }>({
    mutationFn: ({ subscriptionPlanId }) =>
      deleteSubscriptionPlan(subscriptionPlanId, { orgId }),
    onSuccess: (_result, { subscriptionPlanId }) => {
      if (org) {
        queryClient.removeQueries({
          queryKey: subscriptionPlanKeys.detail(org, subscriptionPlanId),
        });
      }
      invalidate();
    },
  });
};

/**
 * Selector example: a plan picker on a subscription form only needs id/label,
 * so it should not re-render when an unrelated field changes.
 */
export const useSubscriptionPlanOptions = (
  filters: SubscriptionPlanListFilters = {}
) => {
  const { org } = useApiScope();

  return useQuery({
    ...subscriptionPlanQueries.list(org!, filters),
    enabled: Boolean(org),
    select: (data: SubscriptionPlanListResponse) =>
      (data.list ?? []).map((plan) => ({
        id: plan.id,
        label: plan.displayName,
      })),
  });
};
