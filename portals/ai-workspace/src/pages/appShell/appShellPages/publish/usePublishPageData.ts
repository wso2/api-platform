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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';

import { getApiPortal, type ApiPortal } from '../../../../apis/apiPortalApis';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { useMcpDeployedGateways } from '../../../../hooks/useMcpDeployedGateways';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { getHttpStatus } from '../../../../utils/apiError';
import { logger } from '../../../../utils/logger';
import type { MCPServer, Publication, PublicationDraftDetails } from '../../../../utils/types';
import {
  buildProductionUrlOptions,
  definitionToText,
  resolveDraftFormValues,
  type DraftFormValues,
  type McpDefinition,
} from './mcpPublishForm';

/** `inDraft`: the draft already stores it, so saving need not send it. */
export type ThumbnailSeed = { blob: Blob; inDraft: boolean };

/** `inDraft`: the draft already stores it, so saving need not send it. */
export type LandingPageSeed = { text: string; inDraft: boolean };

/**
 * What the editor opens with, from the freshest tier that has each part.
 * `landingPageFailed` / `thumbnailFailed`: a tier has that part but reading it failed.
 */
type PublishSeed = {
  definitionText: string;
  landingPage?: LandingPageSeed;
  landingPageFailed: boolean;
  thumbnail?: ThumbnailSeed;
  thumbnailFailed: boolean;
  values: DraftFormValues;
};

type Definition = McpDefinition | undefined;

/** A 404 is "nothing saved yet", not a failure. */
async function orUndefinedOn404<T>(call: Promise<T>): Promise<T | undefined> {
  try {
    return await call;
  } catch (error) {
    if (getHttpStatus(error) === 404) return undefined;
    throw error;
  }
}

/** A stored part of a listing, read when the listing's `has…` flag says it exists. */
type ReadAsset<T> = { value?: T; failed: boolean };

/** A failed read is reported as failed, not as missing, and does not fail the page. */
async function readAsset<T>(exists: boolean | undefined, read: () => Promise<T>): Promise<ReadAsset<T>> {
  if (!exists) return { failed: false };
  try {
    return { value: await read(), failed: false };
  } catch (error) {
    logger.error('Failed to read a part of the publication:', error);
    return { failed: true };
  }
}

type Loaded = {
  server: MCPServer;
  portal?: ApiPortal;
  draft?: PublicationDraftDetails;
  publication?: Publication;
  draftDefinition: Definition;
  publicationDefinition: Definition;
  draftLandingPage: ReadAsset<string>;
  publicationLandingPage: ReadAsset<string>;
  draftThumbnail: ReadAsset<Blob>;
  publicationThumbnail: ReadAsset<Blob>;
};

/**
 * Every read behind the publish page. Each part opens from the freshest tier that has it: the draft,
 * then the live listing, then the proxy itself. The thumbnail and landing page pick their tier by the
 * `has…` flags, never by whether a read worked, so a failed read can't fall through and be saved over
 * the draft's own. `isLoading` is latched; a save hands its draft back through `setDraft`.
 */
export function usePublishPageData(
  apiPortalId: string,
  serverId: string,
  organizationId: string,
  wantPublishedDefinition: boolean
) {
  const gateways = useMcpDeployedGateways(organizationId, serverId);
  const [loaded, setLoaded] = useState<Loaded>();
  const [error, setError] = useState<Error>();
  const [reloadKey, setReloadKey] = useState(0);
  const [publishedDefinition, setPublishedDefinition] = useState<{
    definition: Definition;
    failed: boolean;
    isLoading: boolean;
  }>({ definition: undefined, failed: false, isLoading: false });

  useEffect(() => {
    if (!organizationId || !serverId || !apiPortalId) return;
    let cancelled = false;
    void (async () => {
      try {
        const [server, draft, publication, portal, draftDefinition] = await Promise.all([
          mcpProxiesApis.getMCPServer(serverId, PLATFORM_API_BASE_URL),
          orUndefinedOn404(mcpProxiesApis.getMcpProxyApiPortalDraft(apiPortalId, serverId, PLATFORM_API_BASE_URL)),
          orUndefinedOn404(mcpProxiesApis.getMcpProxyApiPortalPublication(apiPortalId, serverId, PLATFORM_API_BASE_URL)),
          getApiPortal(apiPortalId, PLATFORM_API_BASE_URL).catch(() => undefined),
          orUndefinedOn404(
            mcpProxiesApis.getMcpProxyApiPortalDraftDefinition(apiPortalId, serverId, PLATFORM_API_BASE_URL)
          ),
        ]);
        // Only read when the draft has nothing of its own to open the editor with.
        const publicationDefinition =
          draftDefinition === undefined && publication
            ? await orUndefinedOn404(
                mcpProxiesApis.getMcpProxyApiPortalPublicationDefinition(apiPortalId, serverId, PLATFORM_API_BASE_URL)
              )
            : undefined;
        const [draftLandingPage, publicationLandingPage, draftThumbnail, publicationThumbnail] = await Promise.all([
          readAsset(draft?.hasLandingPage, () =>
            mcpProxiesApis.getMcpProxyApiPortalDraftLandingPage(apiPortalId, serverId, PLATFORM_API_BASE_URL)
          ),
          readAsset(publication?.hasLandingPage, () =>
            mcpProxiesApis.getMcpProxyApiPortalPublicationLandingPage(apiPortalId, serverId, PLATFORM_API_BASE_URL)
          ),
          readAsset(draft?.hasThumbnail, () =>
            mcpProxiesApis.getMcpProxyApiPortalDraftThumbnail(apiPortalId, serverId, PLATFORM_API_BASE_URL)
          ),
          readAsset(publication?.hasThumbnail, () =>
            mcpProxiesApis.getMcpProxyApiPortalPublicationThumbnail(apiPortalId, serverId, PLATFORM_API_BASE_URL)
          ),
        ]);
        if (cancelled) return;
        setError(undefined);
        setLoaded({
          server,
          portal,
          draft,
          publication,
          draftDefinition,
          publicationDefinition,
          draftLandingPage,
          publicationLandingPage,
          draftThumbnail,
          publicationThumbnail,
        });
      } catch (loadError) {
        logger.error('Failed to load the MCP publish page:', loadError);
        if (!cancelled) setError(loadError instanceof Error ? loadError : new Error(String(loadError)));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [organizationId, serverId, apiPortalId, reloadKey]);

  // The live definition is read for the viewer only when it is actually shown.
  const isLive = Boolean(loaded?.publication);
  useEffect(() => {
    if (!wantPublishedDefinition || !isLive) return;
    let cancelled = false;
    setPublishedDefinition((current) => ({ ...current, failed: false, isLoading: true }));
    void (async () => {
      try {
        const definition = await orUndefinedOn404(
          mcpProxiesApis.getMcpProxyApiPortalPublicationDefinition(apiPortalId, serverId, PLATFORM_API_BASE_URL)
        );
        if (!cancelled) setPublishedDefinition({ definition, failed: false, isLoading: false });
      } catch (readError) {
        logger.error('Failed to load the published definition:', readError);
        if (!cancelled) setPublishedDefinition({ definition: undefined, failed: true, isLoading: false });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [wantPublishedDefinition, isLive, apiPortalId, serverId, reloadKey]);

  const productionUrlOptions = useMemo(
    () => buildProductionUrlOptions(gateways.gateways, loaded?.server.context),
    [gateways.gateways, loaded?.server.context]
  );

  // Latched: nothing is shown until the page's first full read has settled.
  const [isReady, setIsReady] = useState(false);
  if (!isReady && !gateways.isLoading && (loaded || error)) setIsReady(true);

  const seed = useMemo<PublishSeed | undefined>(() => {
    if (!isReady || !loaded) return undefined;
    const definition = loaded.draftDefinition ?? loaded.publicationDefinition ?? (loaded.server.capabilities as Definition);
    const landingPage = loaded.draft?.hasLandingPage
      ? { read: loaded.draftLandingPage, inDraft: true }
      : loaded.publication?.hasLandingPage
        ? { read: loaded.publicationLandingPage, inDraft: false }
        : undefined;
    const thumbnail = loaded.draft?.hasThumbnail
      ? { read: loaded.draftThumbnail, inDraft: true }
      : loaded.publication?.hasThumbnail
        ? { read: loaded.publicationThumbnail, inDraft: false }
        : undefined;
    return {
      definitionText: definitionToText(definition),
      landingPage:
        landingPage?.read.value === undefined ? undefined : { text: landingPage.read.value, inDraft: landingPage.inDraft },
      landingPageFailed: Boolean(landingPage?.read.failed),
      thumbnail: thumbnail?.read.value === undefined ? undefined : { blob: thumbnail.read.value, inDraft: thumbnail.inDraft },
      thumbnailFailed: Boolean(thumbnail?.read.failed),
      values: resolveDraftFormValues(loaded.draft, loaded.publication, loaded.server, productionUrlOptions[0]?.url),
    };
  }, [isReady, loaded, productionUrlOptions]);

  const refetch = useCallback(() => setReloadKey((key) => key + 1), []);
  const setDraft = useCallback(
    (draft: PublicationDraftDetails) => setLoaded((current) => current && { ...current, draft }),
    []
  );

  return {
    server: loaded?.server,
    portal: loaded?.portal,
    draft: loaded?.draft,
    publication: loaded?.publication,
    error,
    isLoading: !isReady,
    productionUrlOptions,
    publishedDefinition,
    publishedLandingPage: {
      text: loaded?.publicationLandingPage.value,
      failed: Boolean(loaded?.publicationLandingPage.failed),
    },
    publishedThumbnail: loaded?.publicationThumbnail.value,
    seed,
    refetch,
    setDraft,
  };
}
