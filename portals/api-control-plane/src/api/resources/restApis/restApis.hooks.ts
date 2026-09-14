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

import {
  keepPreviousData,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { HANDLED_LOCALLY } from '../../core/queryClient';
import { useApiScope } from '../../core/scope';
import {
  createRestApi,
  deleteRestApi,
  updateRestApi,
  type CreateRestApiBody,
  type ListRestApisQuery,
  type RestApi,
  type RestApiListResponse,
  type UpdateRestApiBody,
} from './restApis.endpoints';
import { restApiKeys, restApiQueries } from './restApis.queries';

/**
 * The public hook surface for REST APIs — the only thing components import.
 *
 * Two conventions run through all of them:
 *
 * - **Scope is implicit but overridable.** Hooks default to the route's org and
 *   project via `useApiScope()`; an explicit override is accepted for the rare
 *   cross-scope case. Components pass nothing in the common path.
 * - **Errors are `ApiError`.** Because the transport normalizes everything,
 *   `query.error` is always an `ApiError` with `.code`, `.fieldErrors` and
 *   `.status` — components never see an `AxiosError` and never parse a body.
 */

/**
 * Everything the caller may vary on a list request — i.e. the spec's own query
 * parameters minus `projectId`, which comes from scope rather than the caller.
 *
 * Derived, never hand-written: the spec constrains `sortBy` to a `name |
 * createdAt` enum, and a hand-written `sortBy?: string` here would compile
 * happily and then 400 at runtime. This is the codegen guardrail doing its job.
 */
export type RestApiListFilters = Omit<ListRestApisQuery, 'projectId'>;

/**
 * Paginated list of REST APIs in the active project.
 *
 * The query does not run until both org and project are known, so this never
 * fires a request that the server would reject — and, critically, never writes
 * into a cache entry keyed by an empty scope.
 *
 * `keepPreviousData` is the one pagination-specific exception the query client
 * deliberately leaves to individual list queries: paging, sorting or searching
 * changes the key, and without it the grid would unmount into a loading state
 * on every page change. The previous page stays on screen (flagged by
 * `isPlaceholderData`) until the next one arrives.
 */
export const useRestApis = (
  filters: RestApiListFilters = {},
  overrides: { orgId?: string; projectId?: string } = {},
) => {
  const { org, projectId } = useApiScope(overrides);

  return useQuery({
    ...restApiQueries.list(org!, { projectId: projectId!, ...filters }),
    enabled: Boolean(org && projectId),
    placeholderData: keepPreviousData,
  });
};

/**
 * REST API totals for several projects in one organization.
 *
 * The list endpoint is project-scoped, so an organization overview has to
 * request the first item from each project and read the authoritative
 * `pagination.total` value. Keeping this fan-out here prevents pages from
 * reaching through the hook boundary or reimplementing query keys.
 */
export const useRestApiCounts = (
  projectIds: readonly string[],
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);
  const queries = useQueries({
    queries: projectIds.map((projectId) => ({
      ...restApiQueries.list(org!, { limit: 1, offset: 0, projectId }),
      enabled: Boolean(org),
    })),
  });

  const counts = Object.fromEntries(
    projectIds.map((projectId, index) => [projectId, queries[index]?.data?.pagination.total]),
  );

  return {
    counts,
    error: queries.find((query) => query.error)?.error,
    isPending: queries.some((query) => query.isPending),
    total: queries.reduce((sum, query) => sum + (query.data?.pagination.total ?? 0), 0),
  };
};

/** The server-side cap on `limit` for `ListRESTAPIs` (see openapi.yaml `limit-Q`). */
const LIST_MAX_PAGE_SIZE = 100;

/**
 * Every REST API in scope, fetched across as many pages as required.
 *
 * `ListRESTAPIs` caps `limit` at {@link LIST_MAX_PAGE_SIZE} server-side, so a
 * project with more APIs than that cannot be summarized from a single
 * request: `pagination.total` counts every API, but `list` only holds the
 * first page. This hook fetches page one to learn `total`, fans the remaining
 * pages out in parallel via `useQueries`, and concatenates them — so a caller
 * computing metrics (status/type breakdowns) from `list` sees the full
 * collection instead of silently truncating at the first page.
 */
export const useAllRestApis = (
  filters: Omit<RestApiListFilters, 'limit' | 'offset'> = {},
  overrides: { orgId?: string; projectId?: string } = {},
) => {
  const { org, projectId } = useApiScope(overrides);
  const enabled = Boolean(org && projectId);

  const firstPage = useQuery({
    ...restApiQueries.list(org!, {
      projectId: projectId!,
      ...filters,
      limit: LIST_MAX_PAGE_SIZE,
      offset: 0,
    }),
    enabled,
  });

  const total = firstPage.data?.pagination.total ?? 0;
  const remainingOffsets = Array.from(
    { length: Math.max(0, Math.ceil(total / LIST_MAX_PAGE_SIZE) - 1) },
    (_, index) => (index + 1) * LIST_MAX_PAGE_SIZE,
  );

  const remainingPages = useQueries({
    queries: remainingOffsets.map((offset) => ({
      ...restApiQueries.list(org!, {
        projectId: projectId!,
        ...filters,
        limit: LIST_MAX_PAGE_SIZE,
        offset,
      }),
      enabled: enabled && firstPage.isSuccess,
    })),
  });

  const isPending = firstPage.isPending || remainingPages.some((page) => page.isPending);
  const error = firstPage.error ?? remainingPages.find((page) => page.error)?.error;
  const list = firstPage.isSuccess
    ? [...firstPage.data.list, ...remainingPages.flatMap((page) => page.data?.list ?? [])]
    : undefined;

  return {
    data: list ? { list, pagination: firstPage.data!.pagination } : undefined,
    error,
    isPending,
  };
};

/** A single REST API by handle. */
export const useRestApi = (restApiId: string | undefined, overrides: { orgId?: string } = {}) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...restApiQueries.detail(org!, restApiId!),
    enabled: Boolean(org && restApiId),
  });
};

/**
 * Invalidation helper shared by every REST API mutation.
 *
 * It invalidates the resource root rather than a specific list key, because a
 * create or delete shifts pagination and counts on list pages the user has not
 * visited yet — invalidating only the current page leaves those stale. Prefix
 * invalidation covers all of them in one call, and TanStack Query only refetches
 * the queries that are actually mounted.
 */
const useInvalidateRestApis = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: restApiKeys.all(org) });
  };
};

/**
 * When `handlesErrors` is true, errors are handled locally and won't trigger
 * the global snackbar. Otherwise, errors reach the snackbar by default.
 */
export const useCreateRestApi = (
  overrides: { handlesErrors?: boolean; orgId?: string } = {},
) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateRestApis(orgId);

  return useMutation<RestApi, ApiError, CreateRestApiBody>({
    meta: overrides.handlesErrors ? HANDLED_LOCALLY : undefined,
    mutationFn: (body) => createRestApi(body, { orgId }),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so navigating straight
      // to the new API renders instantly instead of showing a loading state
      // for data the server already gave us.
      if (org && created.id) {
        queryClient.setQueryData(restApiKeys.detail(org, created.id), created);
      }
      invalidate();
    },
  });
};

export const useUpdateRestApi = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateRestApis(orgId);

  return useMutation<
    RestApi,
    ApiError,
    { restApiId: string; body: UpdateRestApiBody },
    { previous?: RestApi }
  >({
    mutationFn: ({ restApiId, body }) => updateRestApi(restApiId, body, { orgId }),

    // Optimistic update: the edit appears immediately, and rolls back exactly
    // if the server rejects it. `cancelQueries` first is not optional — an
    // in-flight refetch that resolves after this write would otherwise clobber
    // the optimistic value with stale server data.
    onMutate: async ({ restApiId, body }) => {
      if (!org) return {};
      const key = restApiKeys.detail(org, restApiId);
      await queryClient.cancelQueries({ queryKey: key });

      const previous = queryClient.getQueryData<RestApi>(key);
      if (previous) {
        queryClient.setQueryData<RestApi>(key, { ...previous, ...body });
      }
      return { previous };
    },

    onError: (_error, { restApiId }, context) => {
      if (org && context?.previous) {
        queryClient.setQueryData(restApiKeys.detail(org, restApiId), context.previous);
      }
    },

    // Reconcile with the server regardless of outcome: the response may carry
    // server-computed fields (updatedAt, lifeCycleStatus) the optimistic merge
    // could not know.
    onSettled: () => invalidate(),
  });
};

export const useDeleteRestApi = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateRestApis(orgId);

  return useMutation<void, ApiError, { restApiId: string }>({
    mutationFn: ({ restApiId }) => deleteRestApi(restApiId, { orgId }),
    onSuccess: (_result, { restApiId }) => {
      if (org) {
        // Drop the detail entry outright — refetching a deleted resource just
        // to receive a 404 is a wasted round trip and an error the user would
        // briefly see.
        queryClient.removeQueries({ queryKey: restApiKeys.detail(org, restApiId) });
      }
      invalidate();
    },
  });
};

/**
 * How many ids are inspected before deciding a handle is free. The filter below
 * is a *substring* match, so this bounds the superstring case ("orders-api"
 * also matching "orders-api-v2"), not the number of APIs in the project.
 */
const AVAILABILITY_PROBE_LIMIT = 100;

/**
 * Whether `candidateId` is still free as a REST API handle in the active
 * project: `data === true` means free, `false` means taken, `undefined` means
 * not answered yet (no scope, blank candidate, still loading, or failed).
 *
 * Built on the list query rather than a detail probe, so a free handle is a
 * plain 200 rather than a deliberate 404 sitting in the detail cache. The
 * spec's `query` filter matches a *substring* of the handle, so the exact
 * comparison happens here: `orders` is free even when `orders-v2` exists.
 *
 * Debounce the candidate at the call site (`useDebouncedValue`): this hook
 * issues a request for every distinct value it is handed.
 */
export const useRestApiIdAvailability = (
  candidateId: string | undefined,
  overrides: { orgId?: string; projectId?: string } = {},
) => {
  const { org, projectId } = useApiScope(overrides);
  const candidate = candidateId?.trim().toLowerCase() ?? '';

  return useQuery({
    ...restApiQueries.list(org!, {
      projectId: projectId!,
      query: candidate,
      limit: AVAILABILITY_PROBE_LIMIT,
    }),
    enabled: Boolean(org && projectId && candidate),
    select: (data: RestApiListResponse) =>
      !(data.list ?? []).some((api) => api.id?.toLowerCase() === candidate),
  });
};

/**
 * Selector example: components that only need a name→id lookup should not
 * re-render when an unrelated field changes. `select` runs after the cache
 * read, so this component re-renders only when the derived value changes.
 */
export const useRestApiOptions = (filters: RestApiListFilters = {}) => {
  const { org, projectId } = useApiScope();

  return useQuery({
    ...restApiQueries.list(org!, { projectId: projectId!, ...filters }),
    enabled: Boolean(org && projectId),
    select: (data: RestApiListResponse) =>
      (data.list ?? []).map((api) => ({
        id: api.id,
        label: api.displayName ?? api.id,
      })),
  });
};
