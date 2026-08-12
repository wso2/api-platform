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
  createProject,
  deleteProject,
  updateProject,
  type CreateProjectBody,
  type ListProjectsQuery,
  type Project,
  type ProjectListResponse,
  type UpdateProjectBody,
} from './projects.endpoints';
import { projectKeys, projectQueries } from './projects.queries';

/**
 * The public hook surface for projects — the only thing components import.
 *
 * Two conventions run through all of them:
 *
 * - **Scope is implicit but overridable.** Hooks default to the route's
 *   organization via `useApiScope()`; an explicit override is accepted for the
 *   rare cross-org case. Components pass nothing in the common path.
 * - **Errors are `ApiError`.** Because the transport normalizes everything,
 *   `query.error` always has `.code`, `.fieldErrors` and `.status` — components
 *   never see an `AxiosError` and never parse a body.
 */

/** Everything a caller may vary on the list request. */
export type ProjectListFilters = ListProjectsQuery;

/**
 * Projects in the active organization.
 *
 * The query does not run until the organization is known, so this never fires a
 * request the server would reject — and, critically, never writes into a cache
 * entry keyed by an empty scope.
 */
export const useProjects = (
  filters: ProjectListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...projectQueries.list(org!, filters),
    enabled: Boolean(org),
  });
};

/** A single project by handle. */
export const useProject = (
  projectId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...projectQueries.detail(org!, projectId!),
    enabled: Boolean(org && projectId),
  });
};

/**
 * Invalidation helper shared by every project mutation.
 *
 * It invalidates the resource root rather than a specific list key, because a
 * create or delete shifts pagination and counts on list pages the user has not
 * visited yet — invalidating only the current page leaves those stale. Prefix
 * invalidation covers all of them in one call, and TanStack Query only
 * refetches the queries that are actually mounted.
 */
const useInvalidateProjects = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: projectKeys.all(org) });
  };
};

export const useCreateProject = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateProjects(orgId);

  return useMutation<Project, ApiError, CreateProjectBody>({
    mutationFn: (body) => createProject(body, { orgId }),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so navigating straight
      // to the new project renders instantly instead of showing a loading
      // state for data the server already gave us. `id` is required by the
      // schema, so only the scope needs guarding.
      if (org) {
        queryClient.setQueryData(projectKeys.detail(org, created.id), created);
      }
      invalidate();
    },
  });
};

export const useUpdateProject = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateProjects(orgId);

  return useMutation<
    Project,
    ApiError,
    { projectId: string; body: UpdateProjectBody },
    { previous?: Project }
  >({
    mutationFn: ({ projectId, body }) => updateProject(projectId, body, { orgId }),

    // Optimistic update: the edit appears immediately, and rolls back exactly
    // if the server rejects it. `cancelQueries` first is not optional — an
    // in-flight refetch that resolves after this write would otherwise clobber
    // the optimistic value with stale server data.
    onMutate: async ({ projectId, body }) => {
      if (!org) return {};
      const key = projectKeys.detail(org, projectId);
      await queryClient.cancelQueries({ queryKey: key });

      const previous = queryClient.getQueryData<Project>(key);
      if (previous) {
        queryClient.setQueryData<Project>(key, { ...previous, ...body });
      }
      return { previous };
    },

    onError: (_error, { projectId }, context) => {
      if (org && context?.previous) {
        queryClient.setQueryData(projectKeys.detail(org, projectId), context.previous);
      }
    },

    // Reconcile with the server regardless of outcome: the response carries
    // server-managed fields (updatedAt, updatedBy) the optimistic merge could
    // not know.
    onSettled: () => invalidate(),
  });
};

/**
 * Deletes a project.
 *
 * The backend refuses to delete the last project in an organization, or one
 * that still owns APIs, answering 400 with a specific `code`. Callers should
 * branch on `ApiError.code` to explain which guard blocked the delete rather
 * than showing a generic failure.
 */
export const useDeleteProject = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateProjects(orgId);

  return useMutation<void, ApiError, { projectId: string }>({
    mutationFn: ({ projectId }) => deleteProject(projectId, { orgId }),
    onSuccess: (_result, { projectId }) => {
      if (org) {
        // Drop the detail entry outright — refetching a deleted resource just
        // to receive a 404 is a wasted round trip and an error the user would
        // briefly see.
        queryClient.removeQueries({ queryKey: projectKeys.detail(org, projectId) });
      }
      invalidate();
    },
  });
};

/**
 * Selector example: a project picker only needs id/label, so it should not
 * re-render when an unrelated field changes. `select` runs after the cache
 * read, so this component re-renders only when the derived value changes.
 */
export const useProjectOptions = (filters: ProjectListFilters = {}) => {
  const { org } = useApiScope();

  return useQuery({
    ...projectQueries.list(org!, filters),
    enabled: Boolean(org),
    select: (data: ProjectListResponse) =>
      (data.list ?? []).map((project) => ({
        id: project.id,
        label: project.displayName,
      })),
  });
};
