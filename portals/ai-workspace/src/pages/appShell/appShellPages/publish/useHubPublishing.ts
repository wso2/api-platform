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
import { isFeatureEnabled } from '../../../../config.env';
import { usePublishProject } from '../../../../hooks/usePublishProject';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { logger } from '../../../../utils/logger';
import { isPublished } from './publicationDisplay';

/**
 * Where one MCP proxy stands with publishing to MCP Hubs, for the pages that
 * only need to show it: whether it is published on any hub, and where the publish
 * flow starts.
 *
 * Reads nothing unless publishing to MCP Hubs is on. `isPublished` stays false
 * until it is known, and when the read fails, which does not block the publish
 * flow: it is reachable either way.
 */
export function useHubPublishing(mcpProxyId?: string) {
  const { organizationId, portalsPath } = usePublishProject();
  const [published, setPublished] = useState(false);

  useEffect(() => {
    if (!isFeatureEnabled('api-portal-integration') || !mcpProxyId || !organizationId) return;
    let cancelled = false;
    setPublished(false);
    mcpProxiesApis
      .listMcpProxyApiPortalPublications(mcpProxyId, PLATFORM_API_BASE_URL)
      .then((summary) => {
        if (!cancelled) setPublished(summary.list.some(({ status }) => isPublished(status)));
      })
      .catch((error) => {
        logger.error('Failed to check the MCP Hub publish status:', error);
      });
    return () => {
      cancelled = true;
    };
  }, [mcpProxyId, organizationId]);

  return {
    isPublished: published,
    /** The hub list for this proxy. */
    publishUrl: portalsPath(mcpProxyId ?? ''),
  };
}
