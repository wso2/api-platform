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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { useCallback } from 'react';

import type { ApiError } from '../../core/errors';
import { requireOrgScope, useApiScope } from '../../core/scope';
import { apiPublicationKeys } from '../apiPublications/apiPublications.queries';
import {
  createApiPortal,
  deleteApiPortal,
  updateApiPortal,
  type ApiPortal,
  type CreateApiPortalBody,
  type ListApiPortalsQuery,
  type UpdateApiPortalBody,
} from './apiPortals.endpoints';
import { apiPortalKeys, apiPortalQueries } from './apiPortals.queries';

/** Everything a caller may vary on the list request. */
export type ApiPortalListFilters = ListApiPortalsQuery;

/**
 * Org-scoped list of API Portals for the caller's organization.
 *
 * Returns the server's default first page unless `filters` narrows or pages it.
 * The org-overview count reads `data.pagination.total`, which the server
 * reports for the full result set and is therefore independent of the page
 * size chosen here.
 *
 * `keepPreviousData` matches the other list hooks; org-switch callers must
 * treat `isPlaceholderData` as unavailable so the prior org's data does not
 * leak into the new view.
 */
export const useApiPortals = (
  filters: ApiPortalListFilters = {},
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPortalQueries.list(org!, filters),
    enabled: Boolean(org),
    placeholderData: keepPreviousData,
  });
};

/**
 * Invalidates every cached API Portal query for the org, and the publications
 * that list the org's portals.
 *
 * The resource root rather than the list key, because a create or delete
 * shifts counts and pagination on pages the user has not visited yet. The
 * publish pages list the org's portals through the publications resource, so a
 * portal added, renamed or removed changes those lists too.
 */
export const useInvalidateApiPortals = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return useCallback(() => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: apiPortalKeys.all(org) });
    void queryClient.invalidateQueries({ queryKey: apiPublicationKeys.all(org) });
  }, [org, queryClient]);
};

/** A single registered API Portal by handle. */
export const useApiPortal = (
  apiPortalId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPortalQueries.detail(org!, apiPortalId!),
    enabled: Boolean(org && apiPortalId),
  });
};

export const useCreateApiPortal = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateApiPortals(orgId);

  return useMutation<ApiPortal, ApiError, CreateApiPortalBody>({
    mutationFn: async (body) =>
      createApiPortal(body, { orgId: requireOrgScope(orgId, 'CreateApiPortal') }),
    onSuccess: (created) => {
      // Seed the detail cache so the page the user lands on after registering
      // renders without a loading state for data the server just returned.
      if (org && created.id) {
        queryClient.setQueryData(apiPortalKeys.detail(org, created.id), created);
      }
      invalidate();
    },
  });
};

/**
 * Updates a portal. Passing `sharedKey` in the body rotates the key the
 * control plane sends to the portal; the key is write-only, so nothing about
 * it is cached here.
 */
export const useUpdateApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiPortals(orgId);

  return useMutation<
    ApiPortal,
    ApiError,
    { apiPortalId: string; body: UpdateApiPortalBody }
  >({
    mutationFn: async ({ apiPortalId, body }) =>
      updateApiPortal(apiPortalId, body, {
        orgId: requireOrgScope(orgId, 'UpdateApiPortal'),
      }),
    onSettled: () => invalidate(),
  });
};

export const useDeleteApiPortal = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateApiPortals(orgId);

  return useMutation<void, ApiError, { apiPortalId: string }>({
    mutationFn: async ({ apiPortalId }) =>
      deleteApiPortal(apiPortalId, { orgId: requireOrgScope(orgId, 'DeleteApiPortal') }),
    onSuccess: (_result, { apiPortalId }) => {
      if (org) {
        queryClient.removeQueries({ queryKey: apiPortalKeys.detail(org, apiPortalId) });
      }
      invalidate();
    },
  });
};
