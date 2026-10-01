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
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  createApiDocument,
  deleteApiDocument,
  updateApiDocument,
  type CreateApiDocumentBody,
  type CreateApiDocumentResponse,
  type ListApiDocumentsQuery,
  type UpdateApiDocumentBody,
  type UpdateApiDocumentResponse,
} from './apiDocuments.endpoints';
import {
  apiDocumentKeys,
  apiDocumentParentId,
  apiDocumentQueries,
  type ApiDocumentPagesQuery,
} from './apiDocuments.queries';

/**
 * The public hook surface for API documents.
 *
 * Every hook takes the parent API explicitly (`apiType` + `apiId`, the API's
 * handle) rather than reading it from route scope, mirroring deployments: the
 * page decides which API it is showing, the hook does not guess.
 */

type Overrides = { orgId?: string };

/** One explicit page of document metadata, keeping the previous page on screen while the next loads. */
export const useApiDocuments = (
  apiType: string,
  apiId: string | undefined,
  query: ListApiDocumentsQuery = {},
  overrides: Overrides = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiDocumentQueries.list(org!, apiType, apiId!, query),
    enabled: Boolean(org && apiId),
    placeholderData: keepPreviousData,
  });
};

/** Document metadata loaded a page at a time; call `fetchNextPage` to append the next one. */
export const useApiDocumentPages = (
  apiType: string,
  apiId: string | undefined,
  query: ApiDocumentPagesQuery = {},
  overrides: Overrides = {}
) => {
  const { org } = useApiScope(overrides);

  return useInfiniteQuery({
    ...apiDocumentQueries.pages(org!, apiType, apiId!, query),
    enabled: Boolean(org && apiId),
  });
};

/** One document's metadata. */
export const useApiDocument = (
  apiType: string,
  apiId: string | undefined,
  docId: string | undefined,
  overrides: Overrides = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiDocumentQueries.detail(org!, apiType, apiId!, docId!),
    enabled: Boolean(org && apiId && docId),
  });
};

/** One document's body as text, with its stored content type. */
export const useApiDocumentContent = (
  apiType: string,
  apiId: string | undefined,
  docId: string | undefined,
  overrides: Overrides = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiDocumentQueries.content(org!, apiType, apiId!, docId!),
    enabled: Boolean(org && apiId && docId),
  });
};

/**
 * Invalidates everything cached under one API's documents: every page and
 * filter of both lists (a write changes counts and, since the server orders by
 * last update, page membership) plus every open document.
 */
const useInvalidateApiDocuments = (overrides: Overrides) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope(overrides);

  return (apiType: string, apiId: string) => {
    if (!org) return;
    void queryClient.invalidateQueries({
      queryKey: apiDocumentKeys.detail(org, apiDocumentParentId(apiType, apiId)),
    });
  };
};

type ParentArgs = { apiType: string; apiId: string };

export const useCreateApiDocument = (overrides: Overrides = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiDocuments(overrides);

  return useMutation<
    CreateApiDocumentResponse,
    ApiError,
    ParentArgs & { body: CreateApiDocumentBody }
  >({
    mutationFn: ({ apiType, apiId, body }) => createApiDocument(apiType, apiId, body, { orgId }),
    onSuccess: (_data, { apiType, apiId }) => invalidate(apiType, apiId),
  });
};

export const useUpdateApiDocument = (overrides: Overrides = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiDocuments(overrides);

  return useMutation<
    UpdateApiDocumentResponse,
    ApiError,
    ParentArgs & { docId: string; body: UpdateApiDocumentBody }
  >({
    mutationFn: ({ apiType, apiId, docId, body }) =>
      updateApiDocument(apiType, apiId, docId, body, { orgId }),
    // The response is metadata only and the content is cached separately, so
    // both are refetched rather than patched.
    onSuccess: (_data, { apiType, apiId }) => invalidate(apiType, apiId),
  });
};

export const useDeleteApiDocument = (overrides: Overrides = {}) => {
  const queryClient = useQueryClient();
  const { org, orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiDocuments(overrides);

  return useMutation<void, ApiError, ParentArgs & { docId: string }>({
    mutationFn: ({ apiType, apiId, docId }) => deleteApiDocument(apiType, apiId, docId, { orgId }),
    onSuccess: (_data, { apiType, apiId, docId }) => {
      // Drop the deleted document outright so nothing can render it from cache
      // while the lists refetch.
      if (org) {
        queryClient.removeQueries({
          queryKey: apiDocumentQueries.detail(org, apiType, apiId, docId).queryKey,
        });
        queryClient.removeQueries({
          queryKey: apiDocumentQueries.content(org, apiType, apiId, docId).queryKey,
        });
      }
      invalidate(apiType, apiId);
    },
  });
};
