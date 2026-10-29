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

import { useCallback, useState } from 'react';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { getHttpStatus } from '../../../../utils/apiError';
import type { LandingPageSeed, ThumbnailSeed } from './usePublishPageData';

/** The statuses for an image the server will not take: invalid, not a PNG or JPEG, or too large. */
const REFUSED_IMAGE_STATUSES = [400, 413, 415];

/** What the page opened with, from the freshest tier that has each part. */
type AssetsSeed = {
  landingPage?: LandingPageSeed;
  landingPageFailed?: boolean;
  thumbnail?: ThumbnailSeed;
  thumbnailFailed?: boolean;
};

/**
 * The thumbnail and landing page, which are saved on their own. Saving sends a part only when the
 * draft doesn't already store it, so one pre-filled from the live listing is sent though not edited.
 */
export function usePublishAssets(apiPortalId: string, mcpProxyId: string) {
  const [thumbnail, setThumbnail] = useState<Blob>();
  // The draft's own thumbnail, and the one the page last treated as saved.
  const [storedThumbnail, setStoredThumbnail] = useState<Blob>();
  const [savedThumbnail, setSavedThumbnail] = useState<Blob>();

  // The landing page's Markdown, the draft's own, and the one the page last treated as saved;
  // `undefined` until one exists.
  const [landingPage, setLandingPage] = useState<string>();
  const [storedLandingPage, setStoredLandingPage] = useState<string>();
  const [savedLandingPage, setSavedLandingPage] = useState<string>();
  // The generated starter, to tell it apart from a page edited since.
  const [generatedLandingPage, setGeneratedLandingPage] = useState<string>();
  // Set once when the page opens, so a later refetch can't turn a loaded part into an error.
  const [landingPageFailed, setLandingPageFailed] = useState(false);
  const [thumbnailFailed, setThumbnailFailed] = useState(false);

  /** Takes what the page opened with. */
  const seed = useCallback((opened: AssetsSeed) => {
    setThumbnail(opened.thumbnail?.blob);
    setStoredThumbnail(opened.thumbnail?.inDraft ? opened.thumbnail.blob : undefined);
    setSavedThumbnail(opened.thumbnail?.blob);
    setLandingPage(opened.landingPage?.text);
    setStoredLandingPage(opened.landingPage?.inDraft ? opened.landingPage.text : undefined);
    setSavedLandingPage(opened.landingPage?.text);
    setLandingPageFailed(Boolean(opened.landingPageFailed));
    setThumbnailFailed(Boolean(opened.thumbnailFailed));
  }, []);

  /** An image the server refuses is dropped for the last saved one, so it can't fail every later save. */
  const saveThumbnail = async () => {
    if (!thumbnail || thumbnail === storedThumbnail) return;
    try {
      await mcpProxiesApis.saveMcpProxyApiPortalDraftThumbnail(apiPortalId, mcpProxyId, thumbnail, PLATFORM_API_BASE_URL);
    } catch (error) {
      if (REFUSED_IMAGE_STATUSES.includes(getHttpStatus(error) ?? 0)) setThumbnail(savedThumbnail);
      throw error;
    }
    setStoredThumbnail(thumbnail);
    setSavedThumbnail(thumbnail);
  };

  const saveLandingPage = async () => {
    if (landingPage === undefined || landingPage === storedLandingPage) return;
    await mcpProxiesApis.saveMcpProxyApiPortalDraftLandingPage(apiPortalId, mcpProxyId, landingPage, PLATFORM_API_BASE_URL);
    setStoredLandingPage(landingPage);
    setSavedLandingPage(landingPage);
  };

  /** Tries both, so one failing doesn't leave the other unsaved, then rethrows the first failure. */
  const saveAssets = async () => {
    const results = await Promise.allSettled([saveThumbnail(), saveLandingPage()]);
    const failed = results.find((result): result is PromiseRejectedResult => result.status === 'rejected');
    if (failed) throw failed.reason;
  };

  /** Starts the landing page from `starter`, which the tab then says it generated. */
  const createLandingPage = (starter: string) => {
    setLandingPage(starter);
    setGeneratedLandingPage(starter);
  };

  return {
    createLandingPage,
    generatedLandingPage: generatedLandingPage !== undefined && landingPage === generatedLandingPage,
    hasUnsavedChanges: thumbnail !== savedThumbnail || landingPage !== savedLandingPage,
    landingPage,
    landingPageFailed,
    saveAssets,
    saveLandingPage,
    saveThumbnail,
    seed,
    setLandingPage,
    setThumbnail,
    thumbnail,
    thumbnailFailed,
    /** A stored part could not be read; saving now would drop it from the draft, and publishing from the listing. */
    unreadable: landingPageFailed || thumbnailFailed,
  };
}
