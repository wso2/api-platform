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

import { useEffect, useState } from 'react';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { MULTI_HUB_PUBLISHING } from '../../../../config.env';
import { usePublishProject } from '../../../../hooks/usePublishProject';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { logger } from '../../../../utils/logger';
import { isListedOnPortal } from './publicationDisplay';

/**
 * Where one MCP proxy stands with publishing to MCP Hubs, for the pages that
 * only need to show it: whether it is listed on any hub, and where the publish
 * flow starts.
 *
 * Reads nothing unless the deployment publishes to more than one hub. `isListed`
 * is `null` until it is known, and stays `null` when the read fails: a status that
 * could not be read is not reported as "not published", and does not block the
 * publish flow, which is reachable either way.
 */
export function useHubPublishing(mcpProxyId?: string) {
  const { organizationId, publishPath } = usePublishProject();
  const [isListed, setIsListed] = useState<boolean | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (!MULTI_HUB_PUBLISHING || !mcpProxyId || !organizationId) return;
    let cancelled = false;
    setIsLoading(true);
    mcpProxiesApis
      .listMcpProxyApiPortalPublications(mcpProxyId, PLATFORM_API_BASE_URL)
      .then((summary) => {
        if (!cancelled) setIsListed(summary.list.some(({ status }) => isListedOnPortal(status)));
      })
      .catch((error) => {
        logger.error('Failed to check the MCP Hub publish status:', error);
        if (!cancelled) setIsListed(null);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [mcpProxyId, organizationId]);

  return {
    isListed,
    isLoading,
    /** For the setup banner: `undefined` where publishing to hubs is not offered, so it shows no such step. */
    hasPublication: MULTI_HUB_PUBLISHING ? isListed === true : undefined,
    /** The hub list for this proxy. */
    publishUrl: publishPath(`/${encodeURIComponent(mcpProxyId ?? '')}`),
  };
}
