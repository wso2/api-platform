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
  createGraphQLApi,
  deleteGraphQLApi,
  updateGraphQLApi,
  validateGraphQLSchema,
  type CreateGraphQLApiBody,
  type GraphQLApi,
  type GraphQLApiDetail,
  type GraphQLApiListResponse,
  type ListGraphQLApisQuery,
  type UpdateGraphQLApiBody,
  type ValidateGraphQLSchemaBody,
  type ValidateGraphQLSchemaResponse,
} from './graphqlApis.endpoints';
import { graphQLApiKeys, graphQLApiQueries } from './graphqlApis.queries';

/**
 * Everything the caller may vary on a list request — the spec's own query
 * parameters minus `projectId`, which comes from scope. Mirrors
 * `RestApiListFilters` exactly; see that type for why this is derived rather
 * than hand-written.
 */
export type GraphQLApiListFilters = Omit<ListGraphQLApisQuery, 'projectId'>;

/**
 * Paginated list of GraphQL APIs in the active project. Mirrors `useRestApis`
 * — same scope-gating, same `keepPreviousData` reasoning.
 */
export const useGraphQLApis = (
  filters: GraphQLApiListFilters = {},
  overrides: { orgId?: string; projectId?: string } = {},
) => {
  const { org, projectId } = useApiScope(overrides);

  return useQuery({
    ...graphQLApiQueries.list(org!, { projectId: projectId!, ...filters }),
    enabled: Boolean(org && projectId),
    placeholderData: keepPreviousData,
  });
};

/**
 * GraphQL API totals for several projects in one organization. Mirrors
 * `useRestApiCounts` exactly — see that hook for why this fans out per
 * project rather than trusting a single request.
 */
export const useGraphQLApiCounts = (
  projectIds: readonly string[],
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);
  const queries = useQueries({
    queries: projectIds.map((projectId) => ({
      ...graphQLApiQueries.list(org!, { limit: 1, offset: 0, projectId }),
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

/** The server-side cap on `limit` for `ListGraphQLAPIs` (see openapi.yaml `limit-Q`). */
const LIST_MAX_PAGE_SIZE = 100;

/**
 * Every GraphQL API in scope, fetched across as many pages as required.
 * Mirrors `useAllRestApis` exactly — see that hook for why this fans out
 * rather than trusting a single page.
 */
export const useAllGraphQLApis = (
  filters: Omit<GraphQLApiListFilters, 'limit' | 'offset'> = {},
  overrides: { orgId?: string; projectId?: string } = {},
) => {
  const { org, projectId } = useApiScope(overrides);
  const enabled = Boolean(org && projectId);

  const firstPage = useQuery({
    ...graphQLApiQueries.list(org!, {
      projectId: projectId!,
      ...filters,
      limit: LIST_MAX_PAGE_SIZE,
      offset: 0,
    }),
    enabled,
    // See `useAllRestApis` for why: keeps a filter change from flashing back
    // to a full loading state.
    placeholderData: keepPreviousData,
  });

  const total = firstPage.data?.pagination.total ?? 0;
  const remainingOffsets = Array.from(
    { length: Math.max(0, Math.ceil(total / LIST_MAX_PAGE_SIZE) - 1) },
    (_, index) => (index + 1) * LIST_MAX_PAGE_SIZE,
  );

  const remainingPages = useQueries({
    queries: remainingOffsets.map((offset) => ({
      ...graphQLApiQueries.list(org!, {
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
    isPlaceholderData: firstPage.isPlaceholderData,
  };
};

/** A single GraphQL API by handle. */
export const useGraphQLApi = (
  graphqlApiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...graphQLApiQueries.detail(org!, graphqlApiId!),
    enabled: Boolean(org && graphqlApiId),
  });
};

/** The API's resolved SDL, fetched separately since it can be large. */
export const useGraphQLApiSdl = (
  graphqlApiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...graphQLApiQueries.sdl(org!, graphqlApiId!),
    enabled: Boolean(org && graphqlApiId),
    select: (data) => data.sdl,
  });
};

/**
 * The public hook surface for GraphQL APIs — the only thing components
 * import. Follows the same two conventions as `restApis.hooks.ts`: scope is
 * implicit but overridable via `useApiScope()`, and errors are always
 * `ApiError`.
 */

/**
 * Invalidation helper shared by every GraphQL API mutation. Mirrors
 * `useInvalidateRestApis` — see that helper for why this invalidates the
 * whole resource root rather than one list key.
 */
const useInvalidateGraphQLApis = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: graphQLApiKeys.all(org) });
  };
};

/**
 * When `handlesErrors` is true, errors are handled locally and won't trigger
 * the global snackbar. Otherwise, errors reach the snackbar by default.
 */
export const useCreateGraphQLApi = (
  overrides: { handlesErrors?: boolean; orgId?: string } = {},
) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateGraphQLApis(orgId);

  return useMutation<GraphQLApi, ApiError, CreateGraphQLApiBody>({
    meta: overrides.handlesErrors ? HANDLED_LOCALLY : undefined,
    mutationFn: (body) => createGraphQLApi(body, { orgId }),
    onSuccess: invalidate,
  });
};

/**
 * Updates a GraphQL API. Mirrors `useUpdateRestApi`'s shape, without the
 * optimistic cache write — the mutation variable is a multipart envelope
 * (`{ metadata, sdlFile }`), not the flat resource shape cached under the
 * detail key, so there is nothing to merge in optimistically; a plain
 * invalidate-on-settle keeps this simple and still correct.
 */
export const useUpdateGraphQLApi = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateGraphQLApis(orgId);

  return useMutation<
    GraphQLApiDetail,
    ApiError,
    { graphqlApiId: string; body: UpdateGraphQLApiBody }
  >({
    mutationFn: ({ graphqlApiId, body }) => updateGraphQLApi(graphqlApiId, body, { orgId }),
    onSuccess: invalidate,
  });
};

/** Deletes a GraphQL API. Mirrors `useDeleteRestApi` exactly. */
export const useDeleteGraphQLApi = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateGraphQLApis(orgId);

  return useMutation<void, ApiError, { graphqlApiId: string }>({
    mutationFn: ({ graphqlApiId }) => deleteGraphQLApi(graphqlApiId, { orgId }),
    onSuccess: (_result, { graphqlApiId }) => {
      if (org) {
        queryClient.removeQueries({ queryKey: graphQLApiKeys.detail(org, graphqlApiId) });
      }
      invalidate();
    },
  });
};

/**
 * Dry-runs schema resolution against the declared source (SDL/URL/file/
 * introspection) without creating anything — backs the wizard's "Check"
 * action and the schema explorer's preview.
 */
export const useValidateGraphQLSchema = () => {
  const { orgId } = useApiScope();

  return useMutation<ValidateGraphQLSchemaResponse, ApiError, ValidateGraphQLSchemaBody>({
    mutationFn: (body) => validateGraphQLSchema(body, { orgId }),
  });
};

/**
 * How many ids are inspected before deciding a handle is free. The filter
 * below is a *substring* match, so this bounds the superstring case
 * ("orders-api" also matching "orders-api-v2"), not the number of APIs in the
 * project — see `useRestApiIdAvailability`, which this mirrors exactly.
 */
const AVAILABILITY_PROBE_LIMIT = 100;

/**
 * Whether `candidateId` is still free as a GraphQL API handle in the active
 * project. `data === true` means free, `false` means taken, `undefined` means
 * not answered yet (no scope, blank candidate, still loading, or failed).
 *
 * Debounce the candidate at the call site (`useDebouncedValue`): this hook
 * issues a request for every distinct value it is handed.
 */
export const useGraphQLApiIdAvailability = (
  candidateId: string | undefined,
  overrides: { orgId?: string; projectId?: string } = {},
) => {
  const { org, projectId } = useApiScope(overrides);
  const candidate = candidateId?.trim().toLowerCase() ?? '';

  return useQuery({
    ...graphQLApiQueries.list(org!, {
      projectId: projectId!,
      query: candidate,
      limit: AVAILABILITY_PROBE_LIMIT,
    }),
    enabled: Boolean(org && projectId && candidate),
    select: (data: GraphQLApiListResponse) =>
      !(data.list ?? []).some((api) => api.id?.toLowerCase() === candidate),
  });
};
