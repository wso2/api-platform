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
  addApplicationApiKeys,
  addApplicationAssociations,
  createApplication,
  deleteApplication,
  removeApplicationApiKey,
  removeApplicationAssociation,
  updateApplication,
  type AddApplicationApiKeysBody,
  type AddApplicationAssociationsBody,
  type Application,
  type ApplicationAssociationListResponse,
  type ApplicationListResponse,
  type CreateApplicationBody,
  type ListApplicationsQuery,
  type MappedApiKeyListResponse,
  type UpdateApplicationBody,
} from './applications.endpoints';
import { applicationKeys, applicationQueries } from './applications.queries';

/**
 * The public hook surface for applications — the only thing components import.
 *
 * Two conventions run through all of them:
 *
 * - **Scope is implicit but overridable.** Hooks default to the route's
 *   organization and project via `useApiScope()`; an explicit override is
 *   accepted for the rare cross-scope case.
 * - **Errors are `ApiError`.** `query.error` always has `.code`,
 *   `.fieldErrors` and `.status`; components never see an `AxiosError`.
 */

/**
 * Everything a caller may vary on the list request — the spec's own query
 * parameters minus `projectId`, which comes from scope rather than the caller.
 */
export type ApplicationListFilters = Omit<ListApplicationsQuery, 'projectId'>;

/**
 * Applications in the active project.
 *
 * The query does not run until both organization and project are known, so it
 * never fires a request the server would reject — and never writes into a cache
 * entry keyed by an empty scope.
 */
export const useApplications = (
  filters: ApplicationListFilters = {},
  overrides: { orgId?: string; projectId?: string } = {}
) => {
  const { org, projectId } = useApiScope(overrides);

  return useQuery({
    ...applicationQueries.list(org!, { projectId: projectId!, ...filters }),
    enabled: Boolean(org && projectId),
  });
};

/** A single application by handle. */
export const useApplication = (
  applicationId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...applicationQueries.detail(org!, applicationId!),
    enabled: Boolean(org && applicationId),
  });
};

/** API keys mapped to an application. */
export const useApplicationApiKeys = (
  applicationId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...applicationQueries.apiKeys(org!, applicationId!),
    enabled: Boolean(org && applicationId),
  });
};

/** Providers this application is associated with. */
export const useApplicationAssociations = (
  applicationId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...applicationQueries.associations(org!, applicationId!),
    enabled: Boolean(org && applicationId),
  });
};

/** Keys issued for one association of an application. */
export const useAssociationApiKeys = (
  applicationId: string | undefined,
  associationId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...applicationQueries.associationApiKeys(org!, applicationId!, associationId!),
    enabled: Boolean(org && applicationId && associationId),
  });
};

/**
 * Invalidation helper shared by every application mutation.
 *
 * Invalidates the resource root rather than a specific list key, because a
 * create or delete shifts pagination and counts on list pages the user has not
 * visited yet. Prefix invalidation covers all of them in one call, and
 * TanStack Query only refetches the queries that are actually mounted.
 */
const useInvalidateApplications = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: applicationKeys.all(org) });
  };
};

/** Invalidates one application's sub-resources without touching the lists. */
const useInvalidateApplicationChildren = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return (applicationId: string) => {
    if (!org) return;
    // The detail key is the prefix of every child, so one call covers keys,
    // associations and association keys alike.
    void queryClient.invalidateQueries({
      queryKey: applicationKeys.detail(org, applicationId),
    });
  };
};

export const useCreateApplication = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateApplications(orgId);

  return useMutation<Application, ApiError, CreateApplicationBody>({
    mutationFn: (body) => createApplication(body, { orgId }),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so navigating straight
      // to the new application renders instantly instead of showing a loading
      // state for data the server already gave us.
      if (org && created.id) {
        queryClient.setQueryData(applicationKeys.detail(org, created.id), created);
      }
      invalidate();
    },
  });
};

export const useUpdateApplication = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateApplications(orgId);

  return useMutation<
    Application,
    ApiError,
    { applicationId: string; body: UpdateApplicationBody },
    { previous?: Application }
  >({
    mutationFn: ({ applicationId, body }) =>
      updateApplication(applicationId, body, { orgId }),

    // Optimistic update: the edit appears immediately, and rolls back exactly
    // if the server rejects it. `cancelQueries` first is not optional — an
    // in-flight refetch that resolves after this write would otherwise clobber
    // the optimistic value with stale server data.
    onMutate: async ({ applicationId, body }) => {
      if (!org) return {};
      const key = applicationKeys.detail(org, applicationId);
      await queryClient.cancelQueries({ queryKey: key });

      const previous = queryClient.getQueryData<Application>(key);
      if (previous) {
        queryClient.setQueryData<Application>(key, { ...previous, ...body });
      }
      return { previous };
    },

    onError: (_error, { applicationId }, context) => {
      if (org && context?.previous) {
        queryClient.setQueryData(
          applicationKeys.detail(org, applicationId),
          context.previous
        );
      }
    },

    onSettled: () => invalidate(),
  });
};

export const useDeleteApplication = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateApplications(orgId);

  return useMutation<void, ApiError, { applicationId: string }>({
    mutationFn: ({ applicationId }) => deleteApplication(applicationId, { orgId }),
    onSuccess: (_result, { applicationId }) => {
      if (org) {
        // Removing the detail entry also removes its keys and associations,
        // which are filed beneath it.
        queryClient.removeQueries({
          queryKey: applicationKeys.detail(org, applicationId),
        });
      }
      invalidate();
    },
  });
};

/* -------------------------------------------------------------------------- */
/* API key mappings                                                           */
/* -------------------------------------------------------------------------- */

export const useAddApplicationApiKeys = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidateChildren = useInvalidateApplicationChildren(orgId);

  return useMutation<
    MappedApiKeyListResponse,
    ApiError,
    { applicationId: string; body: AddApplicationApiKeysBody }
  >({
    mutationFn: ({ applicationId, body }) =>
      addApplicationApiKeys(applicationId, body, { orgId }),
    onSuccess: (_result, { applicationId }) => invalidateChildren(applicationId),
  });
};

/**
 * Unmaps a key from an application.
 *
 * `entityID` identifies the artifact the mapping points at and is a **required
 * query parameter** on this DELETE — the key id alone does not identify the
 * mapping. Callers must supply it.
 */
export const useRemoveApplicationApiKey = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidateChildren = useInvalidateApplicationChildren(orgId);

  return useMutation<
    void,
    ApiError,
    { applicationId: string; apiKeyId: string; entityID: string }
  >({
    mutationFn: ({ applicationId, apiKeyId, entityID }) =>
      removeApplicationApiKey(applicationId, apiKeyId, {
        orgId,
        query: { entityID },
      }),
    onSuccess: (_result, { applicationId }) => invalidateChildren(applicationId),
  });
};

/* -------------------------------------------------------------------------- */
/* Associations                                                               */
/* -------------------------------------------------------------------------- */

export const useAddApplicationAssociations = (
  overrides: { orgId?: string } = {}
) => {
  const { orgId } = useApiScope(overrides);
  const invalidateChildren = useInvalidateApplicationChildren(orgId);

  return useMutation<
    ApplicationAssociationListResponse,
    ApiError,
    { applicationId: string; body: AddApplicationAssociationsBody }
  >({
    mutationFn: ({ applicationId, body }) =>
      addApplicationAssociations(applicationId, body, { orgId }),
    onSuccess: (_result, { applicationId }) => invalidateChildren(applicationId),
  });
};

export const useRemoveApplicationAssociation = (
  overrides: { orgId?: string } = {}
) => {
  const { orgId } = useApiScope(overrides);
  const invalidateChildren = useInvalidateApplicationChildren(orgId);

  return useMutation<
    void,
    ApiError,
    { applicationId: string; associationId: string }
  >({
    mutationFn: ({ applicationId, associationId }) =>
      removeApplicationAssociation(applicationId, associationId, { orgId }),
    onSuccess: (_result, { applicationId }) => invalidateChildren(applicationId),
  });
};

/**
 * Selector example: a picker only needs id/label, so it should not re-render
 * when an unrelated field changes.
 */
export const useApplicationOptions = (filters: ApplicationListFilters = {}) => {
  const { org, projectId } = useApiScope();

  return useQuery({
    ...applicationQueries.list(org!, { projectId: projectId!, ...filters }),
    enabled: Boolean(org && projectId),
    select: (data: ApplicationListResponse) =>
      (data.list ?? []).map((application) => ({
        id: application.id,
        label: application.displayName,
      })),
  });
};
