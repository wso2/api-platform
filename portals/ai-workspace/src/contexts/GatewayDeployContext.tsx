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
import { PLATFORM_API_BASE_URL } from '../paths';
import type { HybridGateway, GatewayDeployment } from '../apis/gatewayTypes';
import type {
  DeploymentListResponse,
  DeploymentResponse,
  DeploymentStatus,
} from '../utils/types';
import {
  trackHybridGatewayDeploymentCreate,
  trackHybridGatewayDeploymentRedeploy,
  trackHybridGatewayDeploymentUndeploy,
  trackHybridGatewayDeploymentDelete,
} from '../utils/app-insights';

export type { HybridGateway, GatewayDeployment };

type GatewayDeployResourceType = 'provider' | 'proxy' | 'mcp-server';

const POLL_INTERVAL_MS = 3000;

/**
 * How long a deployment keeps being polled after a deploy/redeploy/undeploy call
 * already answered with a terminal status.
 *
 * The platform API answers optimistically when transitional deployment statuses
 * are disabled (its default): the create call returns DEPLOYED before the gateway
 * has accepted the artifact, and a failure acknowledgement flips the record to
 * FAILED a moment later. Without this verification window the card would keep
 * claiming the deployment is active until the user reloads the page.
 */
const ACK_VERIFY_WINDOW_MS = 20000;

const TERMINAL_STATUSES: DeploymentStatus[] = [
  'DEPLOYED',
  'UNDEPLOYED',
  'ARCHIVED',
  'FAILED',
];

const isTransitionalStatus = (status: string): boolean =>
  status === 'DEPLOYING' || status === 'UNDEPLOYING';

interface PollingEntry {
  deploymentId: string;
  gatewayId: string;
  /** Last status observed for this deployment — a change is what polling waits for. */
  lastStatus: string;
  /**
   * Epoch ms after which polling gives up. `null` means poll until a terminal
   * status is reached (used for genuinely transitional deployments).
   */
  expiresAt: number | null;
}

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
  /**
   * True while a gateway's latest deployment reports a terminal status that the
   * gateway has not confirmed yet — the status shown is provisional and may still
   * flip to FAILED.
   */
  isVerifyingGateway: (gatewayId: string) => boolean;

  /**
   * When true, the artifact is read-only (e.g. gateway-originated): its deployment
   * lifecycle is owned by the gateway. The deployments remain viewable, but deploy/
   * redeploy/restore/undeploy actions are disabled.
   */
  readOnly: boolean;
}

const GatewayDeployContext = createContext<GatewayDeployContextValue | null>(
  null
);

interface GatewayDeployProviderProps {
  apiId: string;
  resourceType?: GatewayDeployResourceType;
  /** Disable deploy/redeploy/restore/undeploy actions while keeping deployments visible. */
  readOnly?: boolean;
  children: ReactNode;
}

export function GatewayDeployProvider({
  apiId,
  resourceType = 'provider',
  readOnly = false,
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
    Map<string, PollingEntry>
  >(new Map());

  const pollingDeploymentsRef = useRef(pollingDeployments);
  pollingDeploymentsRef.current = pollingDeployments;

  const fetchSingleDeploymentStatus = useCallback(
    async (deploymentId: string): Promise<DeploymentResponse> => {
      if (resourceType === 'proxy') {
        return getLLMProxyDeployment(apiId, deploymentId, organizationId, PLATFORM_API_BASE_URL);
      } else if (resourceType === 'mcp-server') {
        return getMCPServerDeployment(apiId, deploymentId, PLATFORM_API_BASE_URL);
      }
      return getLLMProviderDeployment(apiId, deploymentId, organizationId, PLATFORM_API_BASE_URL);
    },
    [apiId, organizationId, resourceType]
  );

  const startPolling = useCallback(
    (
      deploymentId: string,
      gatewayId: string,
      lastStatus: string,
      verifyWindowMs: number | null
    ) => {
      setPollingDeployments((prev) => {
        const next = new Map(prev);
        next.set(deploymentId, {
          deploymentId,
          gatewayId,
          lastStatus,
          expiresAt:
            verifyWindowMs === null ? null : Date.now() + verifyWindowMs,
        });
        return next;
      });
    },
    []
  );

  /**
   * Starts polling after a deploy/redeploy/undeploy call. A transitional response
   * is polled until it settles; an already-terminal response is only provisional
   * (see ACK_VERIFY_WINDOW_MS) and is polled for a bounded window so a late
   * gateway failure acknowledgement surfaces without a page reload.
   */
  const startPostActionPolling = useCallback(
    (deploymentId: string, gatewayId: string, status: string) => {
      startPolling(
        deploymentId,
        gatewayId,
        status,
        isTransitionalStatus(status) ? null : ACK_VERIFY_WINDOW_MS
      );
    },
    [startPolling]
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

  const isVerifyingGateway = useCallback(
    (gatewayId: string): boolean => {
      for (const entry of pollingDeployments.values()) {
        if (entry.gatewayId === gatewayId && entry.expiresAt !== null) {
          return true;
        }
      }
      return false;
    },
    [pollingDeployments]
  );

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
        isTransitionalStatus(d.status) &&
        !pollingDeploymentsRef.current.has(d.deploymentId)
      ) {
        startPolling(d.deploymentId, d.gatewayId, d.status, null);
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
      const observed = new Map<string, string>();
      results.forEach((result, idx) => {
        const [key, entry] = entries[idx];
        if (result.status !== 'fulfilled') {
          resolved.push(key);
          return;
        }

        const status = result.value.status;
        const statusChanged = status !== entry.lastStatus;
        if (statusChanged) {
          observed.set(key, status);
        }

        if (statusChanged && TERMINAL_STATUSES.includes(status)) {
          // The gateway has reported the outcome — nothing further to wait for.
          resolved.push(key);
        } else if (entry.expiresAt !== null && Date.now() >= entry.expiresAt) {
          // Verification window elapsed with no contradicting acknowledgement:
          // treat the status the API already reported as final.
          resolved.push(key);
        }
      });

      if (resolved.length > 0 || observed.size > 0) {
        setPollingDeployments((prev) => {
          const next = new Map(prev);
          for (const [key, status] of observed) {
            const entry = next.get(key);
            if (entry) {
              next.set(key, { ...entry, lastStatus: status });
            }
          }
          for (const key of resolved) {
            next.delete(key);
          }
          return next;
        });
        refetchDeployments();
      }
    }, POLL_INTERVAL_MS);

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

        // Keep watching the deployment: a terminal status here is still provisional
        // until the gateway acknowledges (or rejects) the artifact.
        startPostActionPolling(result.deploymentId, gatewayId, result.status);

        await refetchDeployments();

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
      startPostActionPolling,
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

        // Keep watching the deployment until the gateway confirms the undeploy —
        // the status read back here may still be overwritten with FAILED.
        try {
          const updated = await fetchSingleDeploymentStatus(deploymentId);
          startPostActionPolling(deploymentId, gatewayId, updated.status);
        } catch {
          // Ignore — the refetch below still reconciles the list.
        }

        await refetchDeployments();

        return true;
      } catch (err) {
        logger.error(`Failed to undeploy LLM ${resourceType}:`, err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [
      apiId,
      organizationId,
      refetchDeployments,
      fetchSingleDeploymentStatus,
      startPostActionPolling,
      resourceType,
    ]
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

        // A terminal status on the restore response is provisional as well.
        startPostActionPolling(result.deploymentId, gatewayId, result.status);

        await refetchDeployments();

        return true;
      } catch (err) {
        logger.error(`Failed to restore LLM ${resourceType} deployment:`, err);
        return false;
      } finally {
        setDeployingGatewayId(null);
      }
    },
    [
      apiId,
      organizationId,
      refetchDeployments,
      startPostActionPolling,
      resourceType,
    ]
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
      isVerifyingGateway,
      readOnly,
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
      isVerifyingGateway,
      readOnly,
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
