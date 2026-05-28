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

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useMemo,
  type ReactNode,
} from 'react';
import { logger } from '../../utils/logger';
import { useAppShell } from '../AppShellContext';
import { getGateways } from '../../apis/gatewayApis';
import {
  getMCPServerDeployments,
  deployMCPServer,
  restoreMCPServerDeployment,
  undeployMCPServerDeployment,
  deleteMCPServerDeployment,
} from '../../apis/MCP/mcpServerDeployApis';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import type { HybridGateway, GatewayDeployment } from '../../apis/gatewayTypes';
import type { DeploymentListResponse } from '../../utils/types';

export type { HybridGateway, GatewayDeployment };

const normalizeGatewayNameForDeployment = (name: string): string =>
  name.trim().replace(/\s+/g, '_') || 'gateway';

const getDeploymentDateString = (): string =>
  new Date().toISOString().slice(0, 10);

const parseDeploymentNumber = (name: string | undefined): number | null => {
  if (!name || typeof name !== 'string') return null;
  const idSuffixMatch = name.match(/_(\d+)$/);
  if (idSuffixMatch) return parseInt(idSuffixMatch[1], 10);
  const legacyMatch = name.match(/^Deployment\s+(\d+)$/i);
  return legacyMatch ? parseInt(legacyMatch[1], 10) : null;
};

const isDeploymentNameForDate = (
  name: string | undefined,
  dateStr: string
): boolean =>
  !!name &&
  typeof name === 'string' &&
  name.includes(`_${dateStr}_`) &&
  /_\d+$/.test(name);

const getNextDeploymentName = (
  namePrefix: string,
  gatewayId: string,
  deployments: DeploymentListResponse | null
): string => {
  const prefix = normalizeGatewayNameForDeployment(namePrefix);
  const dateStr = getDeploymentDateString();
  if (!deployments?.list) {
    return `${prefix}_${dateStr}_1`;
  }
  const gatewayDeployments = deployments.list.filter(
    (d) => d.gatewayId === gatewayId
  );
  const deploymentsOnDate = gatewayDeployments.filter((d) =>
    isDeploymentNameForDate(d.name, dateStr)
  );
  let maxNumber = 0;
  for (const d of deploymentsOnDate) {
    const num = parseDeploymentNumber(d.name);
    if (num !== null && num > maxNumber) {
      maxNumber = num;
    }
  }
  return `${prefix}_${dateStr}_${maxNumber + 1}`;
};

interface MCPServerDeployContextValue {
  /** All available gateways */
  gateways: HybridGateway[];
  isLoading: boolean;
  error: Error | null;
  refetchGateways: () => Promise<void>;

  /** Deployments for the current MCP server */
  deployments: DeploymentListResponse | null;
  isLoadingDeployments: boolean;
  deploymentsError: Error | null;
  refetchDeployments: () => Promise<void>;

  /** Deploy to a gateway */
  deployToGateway: (gatewayId: string, host: string) => Promise<boolean>;
  /** Undeploy from a gateway */
  undeployDeployment: (
    deploymentId: string,
    gatewayId: string
  ) => Promise<boolean>;
  /** Redeploy a deployment */
  redeployDeployment: (
    deploymentId: string,
    gatewayId: string
  ) => Promise<boolean>;
  /** Delete a deployment record */
  deleteDeployment: (deploymentId: string) => Promise<boolean>;

  deployingGatewayId: string | null;
  isDeployingToGateway: boolean;
}

const MCPServerDeployContext = createContext<MCPServerDeployContextValue | null>(
  null
);

interface MCPServerDeployProviderProps {
  mcpServerId: string;
  children: ReactNode;
}

export function MCPServerDeployProvider({
  mcpServerId,
  children,
}: MCPServerDeployProviderProps) {
  const { currentOrganization } = useAppShell();
  const organizationId = currentOrganization?.uuid ?? '';

  const [gateways, setGateways] = useState<HybridGateway[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [deployments, setDeployments] = useState<DeploymentListResponse | null>(
    null
  );
  const [isLoadingDeployments, setIsLoadingDeployments] = useState(false);
  const [deploymentsError, setDeploymentsError] = useState<Error | null>(null);

  const [deployingGatewayId, setDeployingGatewayId] = useState<string | null>(
    null
  );
  const isDeployingToGateway = deployingGatewayId !== null;

  const fetchGateways = useCallback(async () => {
    if (!organizationId) {
      setIsLoading(false);
      return;
    }
    setIsLoading(true);
    setError(null);
    try {
      const response = await getGateways(organizationId);
      const fetchedGateways: HybridGateway[] = (response.list || []).map(
        (gateway) => ({
          ...gateway,
          status: gateway.isActive
            ? ('connected' as const)
            : ('disconnected' as const),
        })
      );
      setGateways(fetchedGateways);
    } catch (err) {
      logger.error('Failed to fetch hybrid gateways:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch gateways')
      );
      setGateways([]);
    } finally {
      setIsLoading(false);
    }
  }, [organizationId]);

  useEffect(() => {
    fetchGateways();
  }, [fetchGateways]);

  const refetchDeployments = useCallback(async () => {
    if (!mcpServerId || !organizationId) {
      setDeployments(null);
      return;
    }
    setIsLoadingDeployments(true);
    setDeploymentsError(null);
    try {
      // Fetch deployments for each gateway since gatewayId is required
      const deploymentPromises = gateways.map((gateway) =>
        getMCPServerDeployments(
          mcpServerId,
          organizationId,
          PLATFORM_API_BASE_URL,
          gateway.id
        ).catch((err) => {
          logger.error(
            `Failed to fetch deployments for gateway ${gateway.id}:`,
            err
          );
          return { list: [], count: 0 };
        })
      );

      const deploymentResponses = await Promise.all(deploymentPromises);
      const allDeployments = deploymentResponses.flatMap(
        (response) => response.list
      );

      setDeployments({ list: allDeployments, count: allDeployments.length });
    } catch (err) {
      logger.error('Failed to fetch MCP server deployments:', err);
      setDeploymentsError(
        err instanceof Error ? err : new Error('Failed to fetch deployments')
      );
    } finally {
      setIsLoadingDeployments(false);
    }
  }, [mcpServerId, organizationId, gateways]);

  useEffect(() => {
    if (mcpServerId) {
      refetchDeployments();
    } else {
      setDeployments(null);
    }
  }, [mcpServerId, refetchDeployments]);

  const deployToGateway = useCallback(
    async (gatewayId: string, host: string): Promise<boolean> => {
      if (!mcpServerId || !organizationId) return false;

      setDeployingGatewayId(gatewayId);
      try {
        const deploymentName = getNextDeploymentName(
          gatewayId,
          gatewayId,
          deployments
        );
        const result = await deployMCPServer(
          mcpServerId,
          organizationId,
          {
            name: deploymentName,
            base: 'current',
            gatewayId,
            metadata: {
              host,
            },
          },
          PLATFORM_API_BASE_URL
        );
        if (!result?.deploymentId) {
          throw new Error('Failed to deploy MCP server');
        }

        await refetchDeployments();
        return true;
      } catch (err) {
        logger.error('Deployment of MCP server failed:', err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [mcpServerId, organizationId, deployments, refetchDeployments]
  );

  const undeployDeployment = useCallback(
    async (deploymentId: string, gatewayId: string): Promise<boolean> => {
      if (!mcpServerId || !organizationId || !deploymentId) return false;

      setDeployingGatewayId(gatewayId);
      try {
        await undeployMCPServerDeployment(
          mcpServerId,
          deploymentId,
          organizationId,
          PLATFORM_API_BASE_URL,
          gatewayId
        );

        await refetchDeployments();
        return true;
      } catch (err) {
        logger.error('Failed to undeploy MCP server:', err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [mcpServerId, organizationId, refetchDeployments]
  );

  const redeployDeployment = useCallback(
    async (deploymentId: string, gatewayId: string): Promise<boolean> => {
      if (!mcpServerId || !organizationId || !deploymentId) return false;

      setDeployingGatewayId(gatewayId);
      try {
        const result = await restoreMCPServerDeployment(
          mcpServerId,
          deploymentId,
          organizationId,
          PLATFORM_API_BASE_URL,
          gatewayId
        );
        if (!result?.deploymentId) {
          throw new Error('Failed to restore MCP server deployment');
        }

        await refetchDeployments();
        return true;
      } catch (err) {
        logger.error('Failed to restore MCP server deployment:', err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [mcpServerId, organizationId, refetchDeployments]
  );

  const deleteDeployment = useCallback(
    async (deploymentId: string): Promise<boolean> => {
      if (!mcpServerId || !organizationId || !deploymentId) return false;

      // Block deletion when deployment is DEPLOYED
      const deployment = deployments?.list.find(
        (d) => d.deploymentId === deploymentId
      );
      if (deployment && deployment.status === 'DEPLOYED') {
        logger.warn('Cannot delete a DEPLOYED deployment. Undeploy first.');
        return false;
      }

      try {
        await deleteMCPServerDeployment(
          mcpServerId,
          deploymentId,
          organizationId,
          PLATFORM_API_BASE_URL
        );

        await refetchDeployments();
        return true;
      } catch (err) {
        logger.error('Failed to delete deployment:', err);
        return false;
      }
    },
    [mcpServerId, organizationId, deployments, refetchDeployments]
  );

  const value = useMemo<MCPServerDeployContextValue>(
    () => ({
      gateways,
      isLoading,
      error,
      refetchGateways: fetchGateways,
      deployments,
      isLoadingDeployments,
      deploymentsError,
      refetchDeployments,
      deployToGateway,
      undeployDeployment,
      redeployDeployment,
      deleteDeployment,
      deployingGatewayId,
      isDeployingToGateway,
    }),
    [
      gateways,
      isLoading,
      error,
      fetchGateways,
      deployments,
      isLoadingDeployments,
      deploymentsError,
      refetchDeployments,
      deployToGateway,
      undeployDeployment,
      redeployDeployment,
      deleteDeployment,
      deployingGatewayId,
      isDeployingToGateway,
    ]
  );

  return (
    <MCPServerDeployContext.Provider value={value}>
      {children}
    </MCPServerDeployContext.Provider>
  );
}

export function useMCPServerDeploy(): MCPServerDeployContextValue {
  const context = useContext(MCPServerDeployContext);
  if (!context) {
    throw new Error(
      'useMCPServerDeploy must be used within a MCPServerDeployProvider'
    );
  }
  return context;
}

export default MCPServerDeployContext;
