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
  useApiPublication,
  useApiPublicationDefinition,
  useApiPublicationDraft,
  useApiPublicationDraftDefinition,
  useApiPublicationDraftLandingPage,
  useApiPublicationDraftThumbnail,
  useApiPublicationLandingPage,
  useApiPublicationThumbnail,
} from '@/api/resources/apiPublications';
import { isApiError } from '@/api/core/errors';
import { useGraphQLApi, useGraphQLApiSdl } from '@/api/resources/graphqlApis';
import { useApiThumbnail } from '@/api/resources/apiThumbnail';
import { useRestApi, useRestApiOpenApi } from '@/api/resources/restApis';
import { useGraphQLApiGateways } from '@/api/resources/graphqlApis/apiGateways';
import { useRestApiGateways } from '@/api/resources/restApis/apiGateways/apiGateways.hooks';
import type { PublishTarget } from './publishTarget';
import {
  buildProductionUrlOptions,
  resolveDraftFormValues,
  type DraftFormValues,
} from './utils/publicationForm';
import { readStoredDefinition, readStoredSdl, type StoredDefinition } from './utils/storedDefinition';

type ThumbnailSeed = { blob: Blob; inDraft: boolean };

type LandingPageSeed = { text: string; inDraft: boolean };

/**
 * What the editor opens with, from the freshest tier that has each part.
 * `inDraft`: the draft already stores it, so saving need not send it.
 * `landingPageFailed`: a tier has a landing page but reading it failed.
 */
export type PublishSeed = {
  definition?: StoredDefinition;
  landingPage?: LandingPageSeed;
  landingPageFailed: boolean;
  thumbnail?: ThumbnailSeed;
  values: DraftFormValues;
};

const isNotFound = (error: unknown): boolean => isApiError(error) && error.isNotFound;

/**
 * A query's data, unless the server has since said it is gone. A refetch that
 * 404s keeps the previous `data` beside the error, so a draft that a publish
 * just consumed would otherwise still read as a draft.
 */
const liveData = <T,>(query: { data: T | undefined; error: unknown }): T | undefined =>
  isNotFound(query.error) ? undefined : query.data;

/**
 * Every read behind the publish page, and the editor's seed derived from them.
 * The draft and the publication load in parallel; each definition tier is fetched once the tier
 * before it is confirmed absent (`undefined` keeps a query disabled).
 * The thumbnail and landing page pick their tier by the `hasThumbnail` / `hasLandingPage` flags, never by
 * whether a read worked: a failed read must not fall through to another tier, or saving would overwrite the draft's own.
 * Nothing is seeded from a copy being revalidated, which still holds the old draft until the next refetch.
 */
export function usePublishPageData(apiPortalId: string, target: PublishTarget, wantPublishedDefinition: boolean) {
  const { apiHandle: apiHandler, apiType } = target;
  const isGraphQL = target.definitionKind === 'graphql';
  // Both kinds' API queries are declared, and the other kind's stays disabled
  // on an `undefined` handle, so the hooks run in the same order either way.
  const restApiQuery = useRestApi(isGraphQL ? undefined : apiHandler);
  const graphqlApiQuery = useGraphQLApi(isGraphQL ? apiHandler : undefined);
  const apiQuery = isGraphQL ? graphqlApiQuery : restApiQuery;
  const restGatewaysQuery = useRestApiGateways(isGraphQL ? undefined : apiHandler);
  const graphqlGatewaysQuery = useGraphQLApiGateways(isGraphQL ? apiHandler : undefined);
  const gatewaysQuery = isGraphQL ? graphqlGatewaysQuery : restGatewaysQuery;
  const draftQuery = useApiPublicationDraft(apiPortalId, apiType, apiHandler);
  const publicationQuery = useApiPublication(apiPortalId, apiType, apiHandler);
  const draftDefinitionQuery = useApiPublicationDraftDefinition(apiPortalId, apiType, apiHandler);
  const draftDefinitionAbsent = isNotFound(draftDefinitionQuery.error);

  // A refetch that 404s (after an unpublish) keeps the previous `data` beside
  // the error, so the 404 itself marks the listing as gone. Any other failure
  // says nothing about the listing, so the last known state is kept.
  const publication = liveData(publicationQuery);
  const draft = liveData(draftQuery);

  const draftHasThumbnail = Boolean(draft?.hasThumbnail);
  const publicationHasThumbnail = Boolean(publication?.hasThumbnail);
  const draftThumbnailQuery = useApiPublicationDraftThumbnail(
    apiPortalId,
    apiType,
    draftHasThumbnail ? apiHandler : undefined,
  );
  const publicationThumbnailQuery = useApiPublicationThumbnail(
    apiPortalId,
    apiType,
    publicationHasThumbnail ? apiHandler : undefined,
  );
  const draftHasLandingPage = Boolean(draft?.hasLandingPage);
  const publicationHasLandingPage = Boolean(publication?.hasLandingPage);
  const draftLandingPageQuery = useApiPublicationDraftLandingPage(
    apiPortalId,
    apiType,
    draftHasLandingPage ? apiHandler : undefined,
  );
  const publicationLandingPageQuery = useApiPublicationLandingPage(
    apiPortalId,
    apiType,
    publicationHasLandingPage ? apiHandler : undefined,
  );
  // Last tier, asked for only once the two above are known to have nothing.
  const apiOwnThumbnailNeeded =
    !draftQuery.isPending && !publicationQuery.isPending && !draftHasThumbnail && !publicationHasThumbnail;
  const apiThumbnailQuery = useApiThumbnail(apiType, apiOwnThumbnailNeeded ? apiHandler : undefined);

  const publicationDefinitionQuery = useApiPublicationDefinition(
    apiPortalId,
    apiType,
    draftDefinitionAbsent || (wantPublishedDefinition && publication) ? apiHandler : undefined,
  );
  const publicationDefinitionAbsent = draftDefinitionAbsent && isNotFound(publicationDefinitionQuery.error);

  // Last fallback tier: the API's own stored definition — a REST API's OpenAPI
  // document, which 404s when none has ever been uploaded, or a GraphQL API's SDL.
  const restDefinitionQuery = useRestApiOpenApi(
    publicationDefinitionAbsent && !isGraphQL ? apiHandler : undefined,
  );
  const sdlQuery = useGraphQLApiSdl(publicationDefinitionAbsent && isGraphQL ? apiHandler : undefined);
  const apiOpenApiQuery = isGraphQL ? sdlQuery : restDefinitionQuery;

  // Where the API is served; the first is the Production URL's default.
  // A failed read leaves the list empty, even over an older cached one, and never blocks the page.
  const productionUrlOptions = useMemo(
    () => buildProductionUrlOptions(gatewaysQuery.error ? [] : (gatewaysQuery.data?.list ?? []), apiQuery.data?.context),
    [apiQuery.data?.context, gatewaysQuery.data, gatewaysQuery.error],
  );

  // A disabled query stays `isPending` forever, so a definition tier only blocks
  // the page once its predecessor is confirmed absent and it is actually running.
  const isPending =
    apiQuery.isPending ||
    gatewaysQuery.isPending ||
    draftQuery.isPending ||
    publicationQuery.isPending ||
    draftDefinitionQuery.isPending ||
    (draftDefinitionAbsent && publicationDefinitionQuery.isPending) ||
    (publicationDefinitionAbsent && apiOpenApiQuery.isPending) ||
    (draftHasThumbnail && draftThumbnailQuery.isPending) ||
    (!draftHasThumbnail && publicationHasThumbnail && publicationThumbnailQuery.isPending) ||
    (draftHasLandingPage && draftLandingPageQuery.isPending) ||
    (!draftHasLandingPage && publicationHasLandingPage && publicationLandingPageQuery.isPending) ||
    (apiOwnThumbnailNeeded && apiThumbnailQuery.isPending);
  const isRevalidating = [
    apiQuery,
    gatewaysQuery,
    draftQuery,
    publicationQuery,
    draftDefinitionQuery,
    publicationDefinitionQuery,
    apiOpenApiQuery,
    draftThumbnailQuery,
    publicationThumbnailQuery,
    draftLandingPageQuery,
    publicationLandingPageQuery,
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

  const draftDefinitionData = liveData(draftDefinitionQuery);
  const publicationDefinitionData = liveData(publicationDefinitionQuery);
  const restDefinitionData = liveData(restDefinitionQuery);
  const sdlData = liveData(sdlQuery);

  const publishedDefinition = useMemo(
    () =>
      publicationDefinitionData
        ? readStoredDefinition(publicationDefinitionData.text, publicationDefinitionData.contentType)
        : undefined,
    [publicationDefinitionData],
  );

  /** The API's own definition as the editor opens it, whichever kind it is. */
  const apiDefinition = useMemo<StoredDefinition | undefined>(() => {
    if (isGraphQL) return sdlData === undefined ? undefined : readStoredSdl(sdlData);
    return restDefinitionData ? readStoredDefinition(restDefinitionData.content) : undefined;
  }, [isGraphQL, restDefinitionData, sdlData]);

  const draftThumbnail = liveData(draftThumbnailQuery)?.blob;
  const publishedThumbnail = liveData(publicationThumbnailQuery)?.blob;

  const draftLandingPage = liveData(draftLandingPageQuery)?.text;
  const publishedLandingPage = liveData(publicationLandingPageQuery)?.text;

  // The freshest tier that has a definition wins: draft, publication, the API's own.
  const seed = useMemo<PublishSeed | undefined>(() => {
    if (isLoading) return undefined;
    const definition =
      (draftDefinitionData && readStoredDefinition(draftDefinitionData.text, draftDefinitionData.contentType)) ??
      publishedDefinition ??
      apiDefinition;
    const apiOwnThumbnail = apiThumbnailQuery.blob;
    const thumbnailBlob = draftHasThumbnail
      ? draftThumbnail
      : publicationHasThumbnail
        ? publishedThumbnail
        : apiOwnThumbnail;
    const landingPageText = draftHasLandingPage
      ? draftLandingPage
      : publicationHasLandingPage
        ? publishedLandingPage
        : undefined;
    return {
      definition,
      landingPage: landingPageText === undefined ? undefined : { text: landingPageText, inDraft: draftHasLandingPage },
      landingPageFailed: (draftHasLandingPage || publicationHasLandingPage) && landingPageText === undefined,
      thumbnail: thumbnailBlob && { blob: thumbnailBlob, inDraft: draftHasThumbnail },
      values: resolveDraftFormValues(draft, publication, apiQuery.data, productionUrlOptions[0]?.url),
    };
  }, [
    isLoading,
    apiQuery.data,
    apiThumbnailQuery.blob,
    draft,
    draftHasLandingPage,
    draftHasThumbnail,
    draftLandingPage,
    draftThumbnail,
    publication,
    publicationHasLandingPage,
    publicationHasThumbnail,
    publishedLandingPage,
    publishedThumbnail,
    draftDefinitionData,
    publishedDefinition,
    apiDefinition,
    productionUrlOptions,
  ]);

  return {
    api: apiQuery.data,
    draft,
    error,
    isLoading,
    productionUrlOptions,
    publication,
    publishedDefinition: {
      definition: publishedDefinition,
      failed: isApiError(publicationDefinitionQuery.error) && !publicationDefinitionQuery.error.isNotFound,
      isLoading: publicationDefinitionQuery.isLoading,
    },
    publishedLandingPage: {
      failed: publicationHasLandingPage && publicationLandingPageQuery.isError,
      isLoading: publicationLandingPageQuery.isLoading,
      text: publishedLandingPage,
    },
    publishedThumbnail,
    seed,
  };
}
