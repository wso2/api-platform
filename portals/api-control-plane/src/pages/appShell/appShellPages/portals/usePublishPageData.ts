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

import { useMemo, useState } from 'react';

import {
  REST_API_TYPE,
  useApiPublication,
  useApiPublicationDefinition,
  useApiPublicationDraft,
  useApiPublicationDraftDefinition,
} from '@/api/resources/apiPublications';
import { isApiError } from '@/api/core/errors';
import { useRestApi, useRestApiOpenApi } from '@/api/resources/restApis';
import { resolveDraftFormValues, type DraftFormValues } from './utils/publicationForm';
import { readStoredDefinition, type StoredDefinition } from './utils/storedDefinition';

/** What the editor opens with: the details and the definition, from the freshest tier that has them. */
export type PublishSeed = { definition?: StoredDefinition; values: DraftFormValues };

const isNotFound = (error: unknown): boolean => isApiError(error) && error.isNotFound;

/**
 * A query's data, unless the server has since said it is gone. A refetch that
 * 404s keeps the previous `data` beside the error, so a draft that a publish
 * just consumed would otherwise still read as a draft.
 */
const liveData = <T,>(query: { data: T | undefined; error: unknown }): T | undefined =>
  isNotFound(query.error) ? undefined : query.data;

/**
 * Every read behind the publish page, and what can be derived from them without
 * the user's edits.
 *
 * The draft and the publication load in parallel: a portal can be live and have
 * a draft edit in progress, and the publication decides whether Unpublish is
 * enabled either way. The Specification tab's definition tiers only pre-fill
 * that tab, so each is fetched once the tier before it is confirmed absent
 * (draft definition, then publication definition, then the API's own spec);
 * passing `undefined` for the API handle keeps a tier's query disabled.
 *
 * The published definition is also fetched, on demand, when `wantPublishedDefinition`
 * is set — the same query as the fallback tier, so a definition already read
 * for the pre-fill is reused, and one read for the viewer is not read again
 * (a write invalidates all of them).
 *
 * Nothing is read from a copy that is being revalidated: after a write, the
 * cache still holds the old draft and definitions until the next visit
 * refetches them, and seeding from those would show what is no longer there.
 */
export function usePublishPageData(apiPortalId: string, apiHandler: string, wantPublishedDefinition: boolean) {
  const apiQuery = useRestApi(apiHandler);
  const draftQuery = useApiPublicationDraft(apiPortalId, REST_API_TYPE, apiHandler);
  const publicationQuery = useApiPublication(apiPortalId, REST_API_TYPE, apiHandler);
  const draftDefinitionQuery = useApiPublicationDraftDefinition(apiPortalId, REST_API_TYPE, apiHandler);
  const draftDefinitionAbsent = isNotFound(draftDefinitionQuery.error);

  const publicationDefinitionQuery = useApiPublicationDefinition(
    apiPortalId,
    REST_API_TYPE,
    draftDefinitionAbsent || wantPublishedDefinition ? apiHandler : undefined,
  );
  const publicationDefinitionAbsent = draftDefinitionAbsent && isNotFound(publicationDefinitionQuery.error);

  // Last fallback tier: the API's own stored definition, which 404s when none
  // has ever been uploaded.
  const apiOpenApiQuery = useRestApiOpenApi(publicationDefinitionAbsent ? apiHandler : undefined);

  // A disabled query stays `isPending` forever, so a definition tier only blocks
  // the page once its predecessor is confirmed absent and it is actually running.
  const isPending =
    apiQuery.isPending ||
    draftQuery.isPending ||
    publicationQuery.isPending ||
    draftDefinitionQuery.isPending ||
    (draftDefinitionAbsent && publicationDefinitionQuery.isPending) ||
    (publicationDefinitionAbsent && apiOpenApiQuery.isPending);
  const isRevalidating = [
    apiQuery,
    draftQuery,
    publicationQuery,
    draftDefinitionQuery,
    publicationDefinitionQuery,
    apiOpenApiQuery,
  ].some((query) => query.isFetching);

  // Latched: the page is loading until everything it opens on is current, and
  // not again for the refetches that follow its own saves.
  const [isReady, setIsReady] = useState(false);
  if (!isReady && !isPending && !isRevalidating) setIsReady(true);
  const isLoading = !isReady;

  // A 404 on these tiers just means nothing is saved yet; only another error,
  // or the API itself not resolving, is worth an error screen. The published
  // definition read on demand reports its own failure, beside the viewer.
  const error =
    apiQuery.error ??
    [
      draftQuery,
      publicationQuery,
      draftDefinitionQuery,
      ...(draftDefinitionAbsent ? [publicationDefinitionQuery] : []),
      apiOpenApiQuery,
    ].find((query) => isApiError(query.error) && !query.error.isNotFound)?.error;

  // A refetch that 404s (after an unpublish) keeps the previous `data` beside
  // the error, so the 404 itself marks the listing as gone. Any other failure
  // says nothing about the listing, so the last known state is kept.
  const publication = liveData(publicationQuery);
  const draft = liveData(draftQuery);

  const draftDefinitionData = liveData(draftDefinitionQuery);
  const publicationDefinitionData = liveData(publicationDefinitionQuery);
  const apiOpenApiData = liveData(apiOpenApiQuery);

  const publishedDefinition = useMemo(
    () =>
      publicationDefinitionData
        ? readStoredDefinition(publicationDefinitionData.text, publicationDefinitionData.contentType)
        : undefined,
    [publicationDefinitionData],
  );

  // The freshest tier that has a definition wins: draft, publication, the API's own.
  const seed = useMemo<PublishSeed | undefined>(() => {
    if (isLoading) return undefined;
    const definition =
      (draftDefinitionData && readStoredDefinition(draftDefinitionData.text, draftDefinitionData.contentType)) ??
      publishedDefinition ??
      (apiOpenApiData ? readStoredDefinition(apiOpenApiData.content) : undefined);
    return { definition, values: resolveDraftFormValues(draft, publication, apiQuery.data) };
  }, [isLoading, apiQuery.data, draft, publication, draftDefinitionData, publishedDefinition, apiOpenApiData]);

  return {
    api: apiQuery.data,
    draft,
    error,
    isLoading,
    publication,
    publishedDefinition: {
      definition: publishedDefinition,
      failed: isApiError(publicationDefinitionQuery.error) && !publicationDefinitionQuery.error.isNotFound,
      isLoading: publicationDefinitionQuery.isLoading,
    },
    seed,
  };
}
