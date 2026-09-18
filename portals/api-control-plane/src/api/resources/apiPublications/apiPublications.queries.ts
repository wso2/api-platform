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

import { queryOptions } from '@tanstack/react-query';

import { staleTimes } from '../../core/queryClient';
import { createResourceKeys, type OrgScope } from '../../core/queryKeys';
import {
  getApiPublication,
  getApiPublicationDefinition,
  getApiPublicationDraft,
  getApiPublicationDraftDefinition,
  listApiPublications,
  type ListApiPublicationsQuery,
} from './apiPublications.endpoints';

export const apiPublicationKeys = createResourceKeys('apiPublications');

export const apiPublicationQueries = {
  list: (org: OrgScope, query: ListApiPublicationsQuery) =>
    queryOptions({
      queryKey: apiPublicationKeys.list(org, query),
      queryFn: ({ signal }) => listApiPublications(query, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  draft: (org: OrgScope, apiPortalId: string, apiType: string, apiId: string) =>
    queryOptions({
      queryKey: apiPublicationKeys.child(org, apiId, 'draft', { apiPortalId, apiType }),
      queryFn: ({ signal }) =>
        getApiPublicationDraft(apiPortalId, apiType, apiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  draftDefinition: (org: OrgScope, apiPortalId: string, apiType: string, apiId: string) =>
    queryOptions({
      queryKey: apiPublicationKeys.child(org, apiId, 'draftDefinition', { apiPortalId, apiType }),
      queryFn: ({ signal }) =>
        getApiPublicationDraftDefinition(apiPortalId, apiType, apiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  publication: (org: OrgScope, apiPortalId: string, apiType: string, apiId: string) =>
    queryOptions({
      queryKey: apiPublicationKeys.child(org, apiId, 'publication', { apiPortalId, apiType }),
      queryFn: ({ signal }) =>
        getApiPublication(apiPortalId, apiType, apiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),

  publicationDefinition: (org: OrgScope, apiPortalId: string, apiType: string, apiId: string) =>
    queryOptions({
      queryKey: apiPublicationKeys.child(org, apiId, 'publicationDefinition', {
        apiPortalId,
        apiType,
      }),
      queryFn: ({ signal }) =>
        getApiPublicationDefinition(apiPortalId, apiType, apiId, { orgId: org, signal }),
      staleTime: staleTimes.standard,
    }),
};
