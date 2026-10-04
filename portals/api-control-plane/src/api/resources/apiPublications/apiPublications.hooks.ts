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
import { HANDLED_LOCALLY } from '../../core/queryClient';
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
 * What the writes below do to the cache. A save, publish, unpublish or deprecate
 * can shift the draft, the publication and the rollup's status and timestamps
 * all at once, so the whole resource is marked stale rather than one key — but
 * marking stale and reading again are separate, and a screen that is about to
 * leave, or is already showing the answer, has no use for the second.
 */
const useApiPublicationCache = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return {
    /**
     * Marks every publication query stale. `active` also reads the mounted ones
     * again; `none` leaves them as shown and lets the next visit revalidate.
     */
    markStale: (refetchType: 'active' | 'none') => {
      if (!org) return Promise.resolve();
      return queryClient.invalidateQueries({ queryKey: apiPublicationKeys.all(org), refetchType });
    },

    /**
     * Reads the rollups again now, mounted or not, so the listing that opens
     * next is already current rather than showing the old status and then
     * changing. Awaited by the write, which therefore settles once it is. A
     * failure here is not the write's failure: the listing revalidates itself.
     */
    refreshRollups: async () => {
      if (!org) return;
      await queryClient
        .refetchQueries({ queryKey: apiPublicationKeys.lists(org), type: 'all' })
        .catch(() => undefined);
    },

    /** Stores a draft the server just returned, so reading it again is unnecessary. */
    setDraft: async (apiPortalId: string, apiType: string, apiId: string, draft: PublicationDraftDetails) => {
      if (!org) return;
      const { queryKey } = apiPublicationQueries.draft(org, apiPortalId, apiType, apiId);
      // A read still in flight would otherwise land after this and replace the saved draft.
      await queryClient.cancelQueries({ queryKey });
      queryClient.setQueryData(queryKey, draft);
    },
  };
};

/**
 * Settles a publish, unpublish or deprecate. On success the screen leaves for the
 * listing, so only the listing is brought up to date; on failure the screen
 * stays, and what it shows is read again to find out where things stand.
 */
const settleStatusChange = async (cache: ReturnType<typeof useApiPublicationCache>, error: ApiError | null) => {
  if (error) {
    await cache.markStale('active');
    return;
  }
  await cache.markStale('none');
  await cache.refreshRollups();
};

export const useSaveApiPublicationDraft = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const cache = useApiPublicationCache(orgId);

  return useMutation<
    PublicationDraftDetails,
    ApiError,
    { apiPortalId: string; apiType: string; apiId: string; body: PublicationDraftDetailsInput }
  >({
    mutationFn: ({ apiPortalId, apiType, apiId, body }) =>
      saveApiPublicationDraft(apiPortalId, apiType, apiId, body, { orgId }),
    onSuccess: async (draft, { apiPortalId, apiType, apiId }) => {
      // The response is the draft as saved, so it replaces the cached one outright.
      await cache.setDraft(apiPortalId, apiType, apiId, draft);
      await cache.markStale('none');
    },
  });
};

export const useSaveApiPublicationDraftDefinition = (
  overrides: { handlesErrors?: boolean; orgId?: string } = {},
) => {
  const { orgId } = useApiScope(overrides);
  const cache = useApiPublicationCache(orgId);

  return useMutation<
    void,
    ApiError,
    { apiPortalId: string; apiType: string; apiId: string; body: DraftDefinitionDocument }
  >({
    meta: overrides.handlesErrors ? HANDLED_LOCALLY : undefined,
    mutationFn: ({ apiPortalId, apiType, apiId, body }) =>
      saveApiPublicationDraftDefinition(apiPortalId, apiType, apiId, body, { orgId }),
    onSuccess: () => cache.markStale('none'),
  });
};

export const usePublishRestApiToApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const cache = useApiPublicationCache(orgId);

  return useMutation<Publication, ApiError, { apiPortalId: string; apiId: string }>({
    mutationFn: ({ apiPortalId, apiId }) =>
      publishRestApiToApiPortal(apiPortalId, apiId, { orgId }),
    // On success the publish consumed the draft and the screen is on its way
    // out; on failure (e.g. the draft was already consumed elsewhere) it stays
    // and re-reads what it shows.
    onSettled: (_data, error) => settleStatusChange(cache, error),
  });
};

export const useUnpublishRestApiFromApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const cache = useApiPublicationCache(orgId);

  return useMutation<void, ApiError, { apiPortalId: string; apiId: string }>({
    mutationFn: ({ apiPortalId, apiId }) =>
      unpublishRestApiFromApiPortal(apiPortalId, apiId, { orgId }),
    // Settled, not success-only: a failed unpublish (e.g. 409 PUBLICATION_STATE_CONFLICT
    // after another session already changed the listing) must re-read the real status.
    onSettled: (_data, error) => settleStatusChange(cache, error),
  });
};

export const useDeprecateRestApiOnApiPortal = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const cache = useApiPublicationCache(orgId);

  return useMutation<Publication, ApiError, { apiPortalId: string; apiId: string }>({
    mutationFn: ({ apiPortalId, apiId }) =>
      deprecateRestApiOnApiPortal(apiPortalId, apiId, { orgId }),
    // Settled, not success-only, for the same reason as unpublish: a 409
    // PUBLICATION_STATE_CONFLICT must re-read the real status.
    onSettled: (_data, error) => settleStatusChange(cache, error),
  });
};
