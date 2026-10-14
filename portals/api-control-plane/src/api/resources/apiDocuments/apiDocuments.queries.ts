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

import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';

import { staleTimes } from '../../core/queryClient';
import { createResourceKeys, type OrgScope } from '../../core/queryKeys';
import {
  getApiDocument,
  getApiDocumentContent,
  listApiDocuments,
  type ApiDocumentListResponse,
  type ListApiDocumentsQuery,
} from './apiDocuments.endpoints';

/**
 * Documents are a sub-resource of one API, keyed under that API's detail
 * entry so a single invalidation of the parent covers every page, every filter
 * and every open document.
 *
 * The parent id joins type and handle: a handle is unique only within its own
 * type, so the handle alone could let a REST API and a WebSub API with the same
 * handle share cache entries.
 */
export const apiDocumentKeys = createResourceKeys('apiDocuments');

export const apiDocumentParentId = (apiType: string, apiId: string): string =>
  `${apiType}/${apiId}`;

/** Query shape the paged (infinite) list is keyed on — offset is the page param, not part of the key. */
export type ApiDocumentPagesQuery = Omit<ListApiDocumentsQuery, 'offset'>;

/**
 * Offset of the page after `last`, or `undefined` once every document has been
 * loaded. Reads `pagination.total` rather than guessing from a short page.
 */
export const nextDocumentsOffset = (last: ApiDocumentListResponse): number | undefined => {
  const { offset, total } = last.pagination;
  const next = offset + last.list.length;
  return last.list.length > 0 && next < total ? next : undefined;
};

export const apiDocumentQueries = {
  /** One explicit page — the overview's numbered pagination. */
  list: (org: OrgScope, apiType: string, apiId: string, query: ListApiDocumentsQuery = {}) =>
    queryOptions({
      queryKey: apiDocumentKeys.child(org, apiDocumentParentId(apiType, apiId), 'documents', query),
      queryFn: ({ signal }) => listApiDocuments(apiType, apiId, query, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  /** Pages appended on demand — the Documents page's "View more" list. */
  pages: (org: OrgScope, apiType: string, apiId: string, query: ApiDocumentPagesQuery = {}) =>
    infiniteQueryOptions({
      queryKey: apiDocumentKeys.child(
        org,
        apiDocumentParentId(apiType, apiId),
        'documentPages',
        query
      ),
      queryFn: ({ pageParam, signal }) =>
        listApiDocuments(apiType, apiId, { ...query, offset: pageParam }, { orgId: org, signal }),
      initialPageParam: 0,
      getNextPageParam: nextDocumentsOffset,
      staleTime: staleTimes.standard,
    }),

  /** One document's metadata. */
  detail: (org: OrgScope, apiType: string, apiId: string, docId: string) =>
    queryOptions({
      queryKey: apiDocumentKeys.child(org, apiDocumentParentId(apiType, apiId), 'document', {
        docId,
      }),
      queryFn: ({ signal }) => getApiDocument(apiType, apiId, docId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  /**
   * One document's body. Keyed beside its metadata under the same parent, so
   * the parent-level invalidation every write performs refreshes both.
   */
  content: (org: OrgScope, apiType: string, apiId: string, docId: string) =>
    queryOptions({
      queryKey: apiDocumentKeys.child(org, apiDocumentParentId(apiType, apiId), 'documentContent', {
        docId,
      }),
      queryFn: ({ signal }) => getApiDocumentContent(apiType, apiId, docId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),
};
