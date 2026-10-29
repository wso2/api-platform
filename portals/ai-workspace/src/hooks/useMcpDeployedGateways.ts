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

import { getGateways } from '../apis/gatewayApis';
import type { Gateway } from '../apis/gatewayTypes';
import { getMCPServerDeployments } from '../apis/MCP/mcpServerDeployApis';
import { PLATFORM_API_BASE_URL } from '../paths';
import { logger } from '../utils/logger';

/**
 * The gateways an MCP proxy is currently deployed to, newest deployment first.
 * A gateway whose deployments can't be read is treated as having none rather
 * than failing the whole list.
 */
export function useMcpDeployedGateways(
  organizationId: string,
  mcpProxyId: string
): { gateways: Gateway[]; isLoading: boolean } {
  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (!organizationId || !mcpProxyId) {
      setGateways([]);
      setIsLoading(false);
      return;
    }
    let cancelled = false;
    void (async () => {
      setIsLoading(true);
      try {
        const { list: all = [] } = await getGateways(organizationId);
        const responses = await Promise.all(
          all.map((gateway) =>
            getMCPServerDeployments(mcpProxyId, PLATFORM_API_BASE_URL, gateway.id).catch((error) => {
              logger.error(`Failed to fetch deployments for gateway ${gateway.id}:`, error);
              return { list: [] };
            })
          )
        );
        const latest = new Map<string, number>();
        responses
          .flatMap((response) => response.list)
          .filter((deployment) => deployment.status === 'DEPLOYED')
          .forEach((deployment) => {
            const time = new Date(deployment.createdAt || 0).getTime();
            if (time >= (latest.get(deployment.gatewayId) ?? -1)) latest.set(deployment.gatewayId, time);
          });
        if (cancelled) return;
        setGateways(
          all
            .filter((gateway) => latest.has(gateway.id))
            .sort((a, b) => (latest.get(b.id) ?? 0) - (latest.get(a.id) ?? 0))
        );
      } catch (error) {
        if (cancelled) return;
        logger.error('Failed to fetch deployed gateways for publishing:', error);
        setGateways([]);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [organizationId, mcpProxyId]);

  return { gateways, isLoading };
}
