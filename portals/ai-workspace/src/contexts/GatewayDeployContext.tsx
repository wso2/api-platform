/*
 * Copyright (c) 2026, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from 'react';
import { logger } from '../utils/logger';
import { useAppShell } from './AppShellContext';
import { getGateways } from '../apis/gatewayApis';
import {
  getLLMProviderDeployments,
  getLLMProviderDeployment,
  deployLLMProvider,
  restoreLLMProviderDeployment,
  undeployLLMProviderDeployment,
  deleteLLMProviderDeployment,
} from '../apis/llmProviderApis';
import {
  getLLMProxyDeployments,
  getLLMProxyDeployment,
  deployLLMProxy,
  restoreLLMProxyDeployment,
  undeployLLMProxyDeployment,
  deleteLLMProxyDeployment,
} from '../apis/llmProxiesApis';
import {
  getMCPServerDeployments,
  getMCPServerDeployment,
  deployMCPServer,
  restoreMCPServerDeployment,
  undeployMCPServerDeployment,
  deleteMCPServerDeployment,
} from '../apis/MCP/mcpServerDeployApis';
import { PLATFORM_API_BASE_URL } from '../config.env';
import type { HybridGateway, GatewayDeployment } from '../apis/gatewayTypes';
import type { DeploymentListResponse, DeploymentResponse } from '../utils/types';
import {
  trackHybridGatewayDeploymentCreate,
  trackHybridGatewayDeploymentRedeploy,
  trackHybridGatewayDeploymentUndeploy,
  trackHybridGatewayDeploymentDelete,
} from '../utils/app-insights';

export type { HybridGateway, GatewayDeployment };

type GatewayDeployResourceType = 'provider' | 'proxy' | 'mcp-server';

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

interface GatewayDeployContextValue {
  /** All available gateways */
  gateways: HybridGateway[];
  isLoading: boolean;
  error: Error | null;
  refetchGateways: () => Promise<void>;

  /** Deployments for the current API */
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
  isPollingGateway: (gatewayId: string) => boolean;
}

const GatewayDeployContext = createContext<GatewayDeployContextValue | null>(
  null
);

interface GatewayDeployProviderProps {
  apiId: string;
  resourceType?: GatewayDeployResourceType;
  children: ReactNode;
}

export function GatewayDeployProvider({
  apiId,
  resourceType = 'provider',
  children,
}: GatewayDeployProviderProps) {
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

  const [pollingDeployments, setPollingDeployments] = useState<
    Map<string, { deploymentId: string; gatewayId: string }>
  >(new Map());

  const pollingDeploymentsRef = useRef(pollingDeployments);
  pollingDeploymentsRef.current = pollingDeployments;

  const fetchSingleDeploymentStatus = useCallback(
    async (deploymentId: string): Promise<DeploymentResponse> => {
      if (resourceType === 'proxy') {
        return getLLMProxyDeployment(apiId, deploymentId, organizationId, PLATFORM_API_BASE_URL);
      } else if (resourceType === 'mcp-server') {
        return getMCPServerDeployment(apiId, deploymentId, organizationId, PLATFORM_API_BASE_URL);
      }
      return getLLMProviderDeployment(apiId, deploymentId, organizationId, PLATFORM_API_BASE_URL);
    },
    [apiId, organizationId, resourceType]
  );

  const startPolling = useCallback(
    (deploymentId: string, gatewayId: string) => {
      setPollingDeployments((prev) => {
        const next = new Map(prev);
        next.set(deploymentId, { deploymentId, gatewayId });
        return next;
      });
    },
    []
  );

  const isPollingGateway = useCallback(
    (gatewayId: string): boolean => {
      for (const entry of pollingDeployments.values()) {
        if (entry.gatewayId === gatewayId) return true;
      }
      return false;
    },
    [pollingDeployments]
  );

  const TERMINAL_STATUSES = ['DEPLOYED', 'UNDEPLOYED', 'ARCHIVED', 'FAILED'];

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
    if (!apiId || !organizationId) {
      setDeployments(null);
      return;
    }
    setIsLoadingDeployments(true);
    setDeploymentsError(null);
    try {
      if (resourceType === 'proxy' || resourceType === 'mcp-server') {
        // For proxies and MCP servers, fetch deployments per gateway (gatewayId scoped API)
        const deploymentPromises = gateways.map((gateway) =>
          (resourceType === 'proxy'
            ? getLLMProxyDeployments(
                apiId,
                organizationId,
                PLATFORM_API_BASE_URL,
                gateway.id
              )
            : getMCPServerDeployments(
                apiId,
                organizationId,
                PLATFORM_API_BASE_URL,
                gateway.id
              )
          ).catch((error) => {
            logger.error(
              `Failed to fetch deployments for gateway ${gateway.id}:`,
              error
            );
            return { list: [], count: 0 };
          })
        );

        const deploymentResponses = await Promise.all(deploymentPromises);
        const allDeployments = deploymentResponses.flatMap(
          (response) => response.list
        );

        setDeployments({ list: allDeployments, count: allDeployments.length });
      } else {
        const result = await getLLMProviderDeployments(
          apiId,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        setDeployments(result);
      }
    } catch (err) {
      logger.error(`Failed to fetch LLM ${resourceType} deployments:`, err);
      setDeploymentsError(
        err instanceof Error ? err : new Error('Failed to fetch deployments')
      );
    } finally {
      setIsLoadingDeployments(false);
    }
  }, [apiId, organizationId, resourceType, gateways]);

  useEffect(() => {
    if (apiId) {
      refetchDeployments();
    } else {
      setDeployments(null);
    }
  }, [apiId, refetchDeployments]);

  // Auto-start polling for any deployments already in transitional state
  useEffect(() => {
    if (!deployments?.list) return;
    for (const d of deployments.list) {
      if (
        (d.status === 'DEPLOYING' || d.status === 'UNDEPLOYING') &&
        !pollingDeploymentsRef.current.has(d.deploymentId)
      ) {
        startPolling(d.deploymentId, d.gatewayId);
      }
    }
  }, [deployments, startPolling]);

  // Polling effect — 3s interval for each entry in pollingDeployments
  useEffect(() => {
    if (pollingDeployments.size === 0) return;

    const intervalId = setInterval(async () => {
      const current = pollingDeploymentsRef.current;
      if (current.size === 0) return;

      const entries = Array.from(current.entries());
      const results = await Promise.allSettled(
        entries.map(([, { deploymentId }]) =>
          fetchSingleDeploymentStatus(deploymentId)
        )
      );

      const resolved: string[] = [];
      results.forEach((result, idx) => {
        if (result.status === 'fulfilled') {
          const resp = result.value;
          if (TERMINAL_STATUSES.includes(resp.status)) {
            resolved.push(entries[idx][0]);
          }
        } else {
          resolved.push(entries[idx][0]);
        }
      });

      if (resolved.length > 0) {
        setPollingDeployments((prev) => {
          const next = new Map(prev);
          for (const key of resolved) {
            next.delete(key);
          }
          return next;
        });
        refetchDeployments();
      }
    }, 3000);

    return () => clearInterval(intervalId);
  }, [pollingDeployments, fetchSingleDeploymentStatus, refetchDeployments]);

  const deployToGateway = useCallback(
    async (gatewayId: string, host: string): Promise<boolean> => {
      if (!apiId || !organizationId) return false;

      setDeployingGatewayId(gatewayId);
      try {
        const deploymentName = getNextDeploymentName(
          gatewayId,
          gatewayId,
          deployments
        );
        const result =
          resourceType === 'proxy'
            ? await deployLLMProxy(
                apiId,
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
              )
            : resourceType === 'mcp-server'
              ? await deployMCPServer(
                  apiId,
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
                )
              : await deployLLMProvider(
                  apiId,
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
          throw new Error(`Failed to deploy LLM ${resourceType}`);
        }

        // Track deployment create
        trackHybridGatewayDeploymentCreate({
          orgUuid: organizationId,
          gatewayId,
          apiId,
          deploymentId: result.deploymentId,
          base: 'current',
          hasBuildId: false,
          hasEndpointOverride: Boolean(host),
          resourceType,
        });

        await refetchDeployments();

        // If the response status is transitional, start polling
        if (result.status === 'DEPLOYING' || result.status === 'UNDEPLOYING') {
          startPolling(result.deploymentId, gatewayId);
        }

        return true;
      } catch (err) {
        logger.error(`Deployment of LLM ${resourceType} failed:`, err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [
      apiId,
      organizationId,
      gateways,
      deployments,
      refetchDeployments,
      startPolling,
      resourceType,
    ]
  );

  const undeployDeployment = useCallback(
    async (deploymentId: string, gatewayId: string): Promise<boolean> => {
      if (!apiId || !organizationId || !deploymentId) return false;

      setDeployingGatewayId(gatewayId);
      try {
        if (resourceType === 'proxy') {
          await undeployLLMProxyDeployment(
            apiId,
            deploymentId,
            organizationId,
            PLATFORM_API_BASE_URL,
            gatewayId
          );
        } else if (resourceType === 'mcp-server') {
          await undeployMCPServerDeployment(
            apiId,
            deploymentId,
            organizationId,
            PLATFORM_API_BASE_URL,
            gatewayId
          );
        } else {
          await undeployLLMProviderDeployment(
            apiId,
            deploymentId,
            gatewayId,
            organizationId,
            PLATFORM_API_BASE_URL
          );
        }

        // Track undeploy
        trackHybridGatewayDeploymentUndeploy({
          orgUuid: organizationId,
          gatewayId,
          apiId,
          deploymentId,
          resourceType,
        });

        await refetchDeployments();

        // Check if the deployment is now in a transitional state
        try {
          const updated = await fetchSingleDeploymentStatus(deploymentId);
          if (updated.status === 'DEPLOYING' || updated.status === 'UNDEPLOYING') {
            startPolling(deploymentId, gatewayId);
          }
        } catch {
          // Ignore — refetch already happened
        }

        return true;
      } catch (err) {
        logger.error(`Failed to undeploy LLM ${resourceType}:`, err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [apiId, organizationId, refetchDeployments, fetchSingleDeploymentStatus, startPolling, resourceType]
  );

  const redeployDeployment = useCallback(
    async (deploymentId: string, gatewayId: string): Promise<boolean> => {
      if (!apiId || !organizationId || !deploymentId) return false;

      setDeployingGatewayId(gatewayId);
      try {
        const result =
          resourceType === 'proxy'
            ? await restoreLLMProxyDeployment(
                apiId,
                deploymentId,
                organizationId,
                PLATFORM_API_BASE_URL,
                gatewayId
              )
            : resourceType === 'mcp-server'
              ? await restoreMCPServerDeployment(
                  apiId,
                  deploymentId,
                  organizationId,
                  PLATFORM_API_BASE_URL,
                  gatewayId
                )
              : await restoreLLMProviderDeployment(
                  apiId,
                  deploymentId,
                  gatewayId,
                  organizationId,
                  PLATFORM_API_BASE_URL
                );
        if (!result?.deploymentId) {
          throw new Error(`Failed to restore LLM ${resourceType} deployment`);
        }

        // Track redeploy
        trackHybridGatewayDeploymentRedeploy({
          orgUuid: organizationId,
          gatewayId,
          apiId,
          deploymentId,
          hasBuildId: false,
          resourceType,
        });

        await refetchDeployments();

        // If the restored deployment is transitional, start polling
        if (result.status === 'DEPLOYING' || result.status === 'UNDEPLOYING') {
          startPolling(result.deploymentId, gatewayId);
        }

        return true;
      } catch (err) {
        logger.error(`Failed to restore LLM ${resourceType} deployment:`, err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [apiId, organizationId, refetchDeployments, startPolling, resourceType]
  );

  const deleteDeployment = useCallback(
    async (deploymentId: string): Promise<boolean> => {
      if (!apiId || !organizationId || !deploymentId) return false;

      // Block deletion when deployment is DEPLOYED
      const deployment = deployments?.list.find(
        (d) => d.deploymentId === deploymentId
      );
      if (deployment && deployment.status === 'DEPLOYED') {
        logger.warn('Cannot delete a DEPLOYED deployment. Undeploy first.');
        return false;
      }

      try {
        if (resourceType === 'proxy') {
          await deleteLLMProxyDeployment(
            apiId,
            deploymentId,
            organizationId,
            PLATFORM_API_BASE_URL
          );
        } else if (resourceType === 'mcp-server') {
          await deleteMCPServerDeployment(
            apiId,
            deploymentId,
            organizationId,
            PLATFORM_API_BASE_URL
          );
        } else {
          await deleteLLMProviderDeployment(
            apiId,
            deploymentId,
            organizationId,
            PLATFORM_API_BASE_URL
          );
        }

        // Track delete
        trackHybridGatewayDeploymentDelete({
          orgUuid: organizationId,
          gatewayId: deployment?.gatewayId ?? '',
          apiId,
          deploymentId,
          resourceType,
        });

        await refetchDeployments();
        return true;
      } catch (err) {
        logger.error('Failed to delete deployment:', err);
        return false;
      }
    },
    [apiId, organizationId, deployments, refetchDeployments, resourceType]
  );

  const value = useMemo<GatewayDeployContextValue>(
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
      isPollingGateway,
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
      isPollingGateway,
    ]
  );

  return (
    <GatewayDeployContext.Provider value={value}>
      {children}
    </GatewayDeployContext.Provider>
  );
}

export function useGatewayDeploy(): GatewayDeployContextValue {
  const context = useContext(GatewayDeployContext);
  if (!context) {
    throw new Error(
      'useGatewayDeploy must be used within a GatewayDeployProvider'
    );
  }
  return context;
}

export default GatewayDeployContext;
