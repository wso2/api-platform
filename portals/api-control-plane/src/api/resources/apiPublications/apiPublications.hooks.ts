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

import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  deprecateRestApiOnApiPortal,
  publishRestApiToApiPortal,
  saveApiPublicationDraft,
  saveApiPublicationDraftDefinition,
  unpublishRestApiFromApiPortal,
  type DraftDefinitionDocument,
  type ListApiPublicationsQuery,
  type Publication,
  type PublicationDraftDetails,
  type PublicationDraftDetailsInput,
} from './apiPublications.endpoints';
import { apiPublicationKeys, apiPublicationQueries } from './apiPublications.queries';

/**
 * Everything the caller may vary on `listApiPublications`, minus `apiType`/
 * `apiId` — those identify *which* API this rollup is for, not a filter on it,
 * so they are required parameters on the hook rather than optional filters.
 */
export type ApiPublicationListFilters = Omit<ListApiPublicationsQuery, 'apiType' | 'apiId'>;

/**
 * Every API Portal an API can be published to, each annotated with that API's
 * own publication status and whether a draft is pending — the rollup behind
 * the API-level Portals page.
 *
 * Org-scoped only (the endpoint takes no `projectId`), so this stays enabled
 * once the organization resolves and the API identity (`apiType` + `apiId`) is
 * known — it does not wait on the project the way `useRestApis` does.
 */
export const useApiPublications = (
  apiType: string | undefined,
  apiId: string | undefined,
  filters: ApiPublicationListFilters = {},
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPublicationQueries.list(org!, { apiType: apiType!, apiId: apiId!, ...filters }),
    enabled: Boolean(org && apiType && apiId),
    placeholderData: keepPreviousData,
  });
};

/**
 * One API's draft on one portal. 404s (`DRAFT_NOT_FOUND`) whenever nothing has
 * been saved yet — an expected, common outcome (a portal never drafted against
 * before), not a failure: callers branch on `query.error?.isNotFound` rather
 * than rendering it as an error state.
 */
export const useApiPublicationDraft = (
  apiPortalId: string | undefined,
  apiType: string | undefined,
  apiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPublicationQueries.draft(org!, apiPortalId!, apiType!, apiId!),
    enabled: Boolean(org && apiPortalId && apiType && apiId),
  });
};

/** The draft's own definition. Same 404-is-expected shape as {@link useApiPublicationDraft}. */
export const useApiPublicationDraftDefinition = (
  apiPortalId: string | undefined,
  apiType: string | undefined,
  apiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPublicationQueries.draftDefinition(org!, apiPortalId!, apiType!, apiId!),
    enabled: Boolean(org && apiPortalId && apiType && apiId),
  });
};

/** The live listing on one portal. 404s (`PUBLICATION_NOT_FOUND`) when not currently published. */
export const useApiPublication = (
  apiPortalId: string | undefined,
  apiType: string | undefined,
  apiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPublicationQueries.publication(org!, apiPortalId!, apiType!, apiId!),
    enabled: Boolean(org && apiPortalId && apiType && apiId),
  });
};

/** The published definition — the fallback tier once no draft definition exists. */
export const useApiPublicationDefinition = (
  apiPortalId: string | undefined,
  apiType: string | undefined,
  apiId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPublicationQueries.publicationDefinition(org!, apiPortalId!, apiType!, apiId!),
    enabled: Boolean(org && apiPortalId && apiType && apiId),
  });
};

/**
 * Invalidation shared by every write below: a save/publish/unpublish/deprecate can shift
 * the draft, the publication, and the rollup's status/timestamps all at once,
 * so the whole resource is invalidated rather than one specific key.
 */
const useInvalidateApiPublications = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: apiPublicationKeys.all(org) });
  };
};

export const useSaveApiPublicationDraft = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiPublications(orgId);

  return useMutation<
    PublicationDraftDetails,
    ApiError,
    { apiPortalId: string; apiType: string; apiId: string; body: PublicationDraftDetailsInput }
  >({
    mutationFn: ({ apiPortalId, apiType, apiId, body }) =>
      saveApiPublicationDraft(apiPortalId, apiType, apiId, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useSaveApiPublicationDraftDefinition = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiPublications(orgId);

  return useMutation<
    void,
    ApiError,
    { apiPortalId: string; apiType: string; apiId: string; body: DraftDefinitionDocument }
  >({
    mutationFn: ({ apiPortalId, apiType, apiId, body }) =>
      saveApiPublicationDraftDefinition(apiPortalId, apiType, apiId, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const usePublishRestApiToApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiPublications(orgId);

  return useMutation<Publication, ApiError, { apiPortalId: string; apiId: string }>({
    mutationFn: ({ apiPortalId, apiId }) =>
      publishRestApiToApiPortal(apiPortalId, apiId, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useUnpublishRestApiFromApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiPublications(orgId);

  return useMutation<void, ApiError, { apiPortalId: string; apiId: string }>({
    mutationFn: ({ apiPortalId, apiId }) =>
      unpublishRestApiFromApiPortal(apiPortalId, apiId, { orgId }),
    // Settled, not success-only: a failed unpublish (e.g. 409 PUBLICATION_STATE_CONFLICT
    // after another session already changed the listing) must re-read the real status.
    onSettled: () => invalidate(),
  });
};

export const useDeprecateRestApiOnApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiPublications(orgId);

  return useMutation<Publication, ApiError, { apiPortalId: string; apiId: string }>({
    mutationFn: ({ apiPortalId, apiId }) =>
      deprecateRestApiOnApiPortal(apiPortalId, apiId, { orgId }),
    // Settled, not success-only, for the same reason as unpublish: a 409
    // PUBLICATION_STATE_CONFLICT must re-read the real status.
    onSettled: () => invalidate(),
  });
};
