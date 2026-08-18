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
  createSubscription,
  deleteSubscription,
  updateSubscription,
  type CreateSubscriptionBody,
  type ListSubscriptionsQuery,
  type Subscription,
  type UpdateSubscriptionBody,
} from './subscriptions.endpoints';
import { subscriptionKeys, subscriptionQueries } from './subscriptions.queries';

/**
 * The public hook surface for subscriptions — the only thing components import.
 *
 * Scope is implicit but overridable, and `query.error` is always an `ApiError`
 * with `.code`, `.fieldErrors` and `.status`.
 *
 * The mutations here take `subscriberId` explicitly because the spec requires
 * it as a query parameter on update and delete — see the endpoints module.
 */

export type SubscriptionListFilters = ListSubscriptionsQuery;

/**
 * Subscriptions in the active organization.
 *
 * Pass `artifactId` or `applicationId` to scope the list to one API or one
 * application; the filter lands in the cache key, so each view keeps its own
 * entry rather than fighting over a shared one.
 */
export const useSubscriptions = (
  filters: SubscriptionListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...subscriptionQueries.list(org!, filters),
    enabled: Boolean(org),
  });
};

/** A single subscription. */
export const useSubscription = (
  subscriptionId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...subscriptionQueries.detail(org!, subscriptionId!),
    enabled: Boolean(org && subscriptionId),
  });
};

/**
 * Invalidation helper shared by every subscription mutation.
 *
 * Invalidates the resource root rather than a specific list key: a subscription
 * appears in several filtered lists at once (by artifact, by application, by
 * subscriber), and a create or delete changes all of them.
 */
const useInvalidateSubscriptions = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: subscriptionKeys.all(org) });
  };
};

export const useCreateSubscription = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSubscriptions(orgId);

  return useMutation<Subscription, ApiError, CreateSubscriptionBody>({
    mutationFn: (body) => createSubscription(body, { orgId }),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so navigating straight
      // to the new subscription renders instantly instead of showing a loading
      // state for data the server already gave us.
      if (org && created.id) {
        queryClient.setQueryData(subscriptionKeys.detail(org, created.id), created);
      }
      invalidate();
    },
  });
};

/**
 * Updates a subscription.
 *
 * `subscriberId` is required by the spec as a query parameter alongside the
 * path id, so callers must pass it.
 */
export const useUpdateSubscription = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSubscriptions(orgId);

  return useMutation<
    Subscription,
    ApiError,
    { subscriptionId: string; subscriberId: string; body: UpdateSubscriptionBody },
    { previous?: Subscription }
  >({
    mutationFn: ({ subscriptionId, subscriberId, body }) =>
      updateSubscription(subscriptionId, body, { orgId, query: { subscriberId } }),

    // Optimistic update: the edit appears immediately, and rolls back exactly
    // if the server rejects it. `cancelQueries` first is not optional — an
    // in-flight refetch that resolves after this write would otherwise clobber
    // the optimistic value with stale server data.
    onMutate: async ({ subscriptionId, body }) => {
      if (!org) return {};
      const key = subscriptionKeys.detail(org, subscriptionId);
      await queryClient.cancelQueries({ queryKey: key });

      const previous = queryClient.getQueryData<Subscription>(key);
      if (previous) {
        queryClient.setQueryData<Subscription>(key, { ...previous, ...body });
      }
      return { previous };
    },

    onError: (_error, { subscriptionId }, context) => {
      if (org && context?.previous) {
        queryClient.setQueryData(
          subscriptionKeys.detail(org, subscriptionId),
          context.previous
        );
      }
    },

    onSettled: () => invalidate(),
  });
};

/** Deletes a subscription. `subscriberId` is required by the spec. */
export const useDeleteSubscription = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSubscriptions(orgId);

  return useMutation<
    void,
    ApiError,
    { subscriptionId: string; subscriberId: string }
  >({
    mutationFn: ({ subscriptionId, subscriberId }) =>
      deleteSubscription(subscriptionId, { orgId, query: { subscriberId } }),
    onSuccess: (_result, { subscriptionId }) => {
      if (org) {
        // Drop the detail entry outright — refetching a deleted subscription
        // just to receive a 404 is a wasted round trip and an error the user
        // would briefly see.
        queryClient.removeQueries({
          queryKey: subscriptionKeys.detail(org, subscriptionId),
        });
      }
      invalidate();
    },
  });
};
