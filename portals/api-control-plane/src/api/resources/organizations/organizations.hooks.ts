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
import {
  registerOrganization,
  type ListOrganizationsQuery,
  type Organization,
  type OrganizationListResponse,
  type RegisterOrganizationBody,
} from './organizations.endpoints';
import { organizationKeys, organizationQueries } from './organizations.queries';

/**
 * The public hook surface for organizations
 *
 * These differ from every other resource in one respect: there is no scope to
 * resolve, because organizations *are* the scope. Nothing here reads
 * `useApiScope`, and no query is gated on an active organization — the list has
 * to load before one exists.
 *
 * As everywhere else, `query.error` is always an `ApiError` with `.code`,
 * `.fieldErrors` and `.status`; components never see an `AxiosError`.
 */

/** Everything a caller may vary on the list request. */
export type OrganizationListFilters = ListOrganizationsQuery;

/** Organizations the signed-in user can reach. Drives the org switcher. */
export const useOrganizations = (filters: OrganizationListFilters = {}) =>
  useQuery(organizationQueries.list(filters));

/** A single organization by id. */
export const useOrganization = (organizationId: string | undefined) =>
  useQuery({
    ...organizationQueries.detail(organizationId!),
    enabled: Boolean(organizationId),
  });

/**
 * Registers a new organization during onboarding.
 *
 * Invalidates the list root rather than a specific list key, because a new
 * organization shifts pagination on list pages the user has not visited yet.
 */
export const useRegisterOrganization = () => {
  const queryClient = useQueryClient();

  return useMutation<Organization, ApiError, RegisterOrganizationBody>({
    mutationFn: (body) => registerOrganization(body),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so navigating straight
      // into the new organization renders instantly instead of showing a
      // loading state for data the server already gave us. `id` is required by
      // the schema, so no guard is needed here.
      queryClient.setQueryData(organizationKeys.detail(created.id), created);
      void queryClient.invalidateQueries({ queryKey: organizationKeys.all() });
    },
  });
};

/**
 * Selector example: a switcher only needs id/label, so it should not re-render
 * when an unrelated field changes. `select` runs after the cache read, so this
 * component re-renders only when the derived value changes.
 */
export const useOrganizationOptions = (filters: OrganizationListFilters = {}) =>
  useQuery({
    ...organizationQueries.list(filters),
    select: (data: OrganizationListResponse) =>
      (data.list ?? []).map((organization) => ({
        id: organization.id,
        label: organization.displayName,
      })),
  });
