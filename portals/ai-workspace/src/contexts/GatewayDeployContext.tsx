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
import { useAppAuth } from './AppAuthContext';
import {
  DEPLOYMENT_SCOPES,
  type DeployableResourceType,
} from '../auth/permissions';
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

type GatewayDeployResourceType = DeployableResourceType;

const POLL_INTERVAL_MS = 2000;

/**
 * Upper bound on polling a transitional (DEPLOYING/UNDEPLOYING) deployment,
 * mirroring platform-api's own deployment timeout: its sweeper marks a stuck
 * deployment FAILED after `timeout_duration` (60s default) and runs every
 * `timeout_interval` (20s default), so detection lands by 80s. Past that the
 * record cannot still be transitional, and continuing to poll only keeps the tab
 * busy — a lost acknowledgement would otherwise be polled forever.
 *
 * Keep in step with [platform_api.deployments] in platform-api's config template;
 * the values are not exposed over the API, so they cannot be read at runtime.
 */
const TRANSITIONAL_POLL_TIMEOUT_MS = 80000;

const isTransitionalStatus = (status: string): boolean =>
  status === 'DEPLOYING' || status === 'UNDEPLOYING';

interface PollingEntry {
  deploymentId: string;
  gatewayId: string;
  /** Epoch ms after which polling gives up regardless of the status observed. */
  expiresAt: number;
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
   * When true, the artifact's deployment lifecycle is not writable from here —
   * either because the artifact is gateway-originated (owned by the gateway) or
   * because the signed-in user lacks the deployment-create scope for this
   * resource kind. The deployments remain viewable, but deploy/redeploy/restore/
   * undeploy actions are disabled.
   */
  readOnly: boolean;
  /** True when the user holds the deployment-create scope for this resource kind. */
  canDeploy: boolean;
  /** True when the user holds the deployment-read scope for this resource kind. */
  canViewDeployments: boolean;
}

const GatewayDeployContext = createContext<GatewayDeployContextValue | null>(
  null
);

interface GatewayDeployProviderProps {
  apiId: string;
  resourceType?: GatewayDeployResourceType;
  /**
   * Disable deploy/redeploy/restore/undeploy actions while keeping deployments
   * visible. The user's scopes are checked independently and can force this on
   * even when the caller passes `false`.
   */
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
  const { hasPermission } = useAppAuth();
  const organizationId = currentOrganization?.uuid ?? '';

  // Permission is the floor, not an override: a caller passing readOnly={false}
  // (or omitting it) still cannot get writable actions without the scope, so a
  // viewer sees the same read-only deploy surface as a gateway-owned artifact.
  const canViewDeployments = hasPermission(DEPLOYMENT_SCOPES[resourceType].read);
  const canDeploy = hasPermission(DEPLOYMENT_SCOPES[resourceType].create);
  const isReadOnly = readOnly || !canDeploy;

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

  /**
   * Deployments whose poll window expired while still transitional — the gateway's
   * acknowledgement never arrived, so they are surfaced as FAILED. The override only
   * applies while platform-api still reports a transitional status, so a late
   * acknowledgement replaces it with the real outcome on the next refetch.
   */
  const [timedOutDeployments, setTimedOutDeployments] = useState<Set<string>>(
    new Set()
  );

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

  /**
   * Watches a deployment whose status is still transitional (DEPLOYING/UNDEPLOYING)
   * until platform-api reports a status that is no longer transitional, bounded by
   * the server's own timeout (TRANSITIONAL_POLL_TIMEOUT_MS). A status that is
   * already non-transitional is taken at face value — whatever platform-api
   * reports is what the card shows.
   */
  const startPolling = useCallback(
    (deploymentId: string, gatewayId: string, status: string) => {
      if (!isTransitionalStatus(status)) return;
      // A new transitional phase gets a clean slate: drop any timed-out mark from
      // an earlier phase of this same deploymentId, or the stale mark would paint
      // it FAILED immediately and suppress the watch that resolves it.
      setTimedOutDeployments((prev) => {
        if (!prev.has(deploymentId)) return prev;
        const next = new Set(prev);
        next.delete(deploymentId);
        return next;
      });
      setPollingDeployments((prev) => {
        if (prev.has(deploymentId)) return prev;
        const next = new Map(prev);
        next.set(deploymentId, {
          deploymentId,
          gatewayId,
          expiresAt: Date.now() + TRANSITIONAL_POLL_TIMEOUT_MS,
        });
        return next;
      });
    },
    []
  );

  /** True while a deployment on this gateway is still in a transitional status. */
  const isPollingGateway = useCallback(
    (gatewayId: string): boolean => {
      for (const entry of pollingDeployments.values()) {
        if (entry.gatewayId === gatewayId) return true;
      }
      return false;
    },
    [pollingDeployments]
  );

  const fetchGateways = useCallback(async () => {
    // A user without the deployment-read scope has no readable deploy surface,
    // so don't issue the gateway/deployment reads at all — they would only 403.
    if (!organizationId || !canViewDeployments) {
      setGateways([]);
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
  }, [organizationId, canViewDeployments]);

  useEffect(() => {
    fetchGateways();
  }, [fetchGateways]);

  const refetchDeployments = useCallback(async () => {
    if (!apiId || !organizationId || !canViewDeployments) {
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
  }, [apiId, organizationId, resourceType, gateways, canViewDeployments]);

  useEffect(() => {
    if (apiId) {
      refetchDeployments();
    } else {
      setDeployments(null);
    }
  }, [apiId, refetchDeployments]);

  /**
   * Deployments as consumers see them: a record still reported as transitional after
   * its poll window expired is shown as FAILED, since the status was never received.
   */
  const effectiveDeployments = useMemo<DeploymentListResponse | null>(() => {
    if (!deployments?.list || timedOutDeployments.size === 0) return deployments;
    return {
      ...deployments,
      list: deployments.list.map((d) =>
        timedOutDeployments.has(d.deploymentId) && isTransitionalStatus(d.status)
          ? { ...d, status: 'FAILED' as DeploymentStatus }
          : d
      ),
    };
  }, [deployments, timedOutDeployments]);

  // Auto-start polling for any deployments already in transitional state
  useEffect(() => {
    if (!effectiveDeployments?.list) return;
    for (const d of effectiveDeployments.list) {
      if (
        isTransitionalStatus(d.status) &&
        !pollingDeploymentsRef.current.has(d.deploymentId)
      ) {
        startPolling(d.deploymentId, d.gatewayId, d.status);
      }
    }
  }, [effectiveDeployments, startPolling]);

  /**
   * Polling effect — one pass immediately when a deployment enters the watch set,
   * then every POLL_INTERVAL_MS, so the first status read always lands well inside
   * five seconds of the deploy/undeploy call rather than after an initial idle tick.
   * An entry is dropped as soon as its status is no longer transitional, or once its
   * poll window expires.
   */
  useEffect(() => {
    if (pollingDeployments.size === 0) return;

    let cancelled = false;

    const poll = async () => {
      const current = pollingDeploymentsRef.current;
      if (current.size === 0) return;

      const entries = Array.from(current.entries());
      const results = await Promise.allSettled(
        entries.map(([, { deploymentId }]) =>
          fetchSingleDeploymentStatus(deploymentId)
        )
      );
      if (cancelled) return;

      const resolved: string[] = [];
      const timedOut: string[] = [];
      results.forEach((result, idx) => {
        const [key, entry] = entries[idx];
        if (result.status !== 'fulfilled') {
          // A status read can fail transiently (network blip, brief 5xx). Keep the
          // entry in the watch set and retry on the next tick; only give up once
          // the poll window has expired.
          logger.warn(
            `Failed to read status for deployment ${entry.deploymentId}:`,
            result.reason
          );
          if (Date.now() >= entry.expiresAt) {
            resolved.push(key);
            timedOut.push(key);
          }
          return;
        }

        if (!isTransitionalStatus(result.value.status)) {
          // The gateway has reported the outcome — nothing further to wait for.
          resolved.push(key);
        } else if (Date.now() >= entry.expiresAt) {
          // Still transitional past the server's own deployment timeout: the
          // acknowledgement never arrived, so show it as failed.
          resolved.push(key);
          timedOut.push(key);
        }
      });

      if (timedOut.length > 0) {
        setTimedOutDeployments((prev) => {
          const next = new Set(prev);
          for (const key of timedOut) next.add(key);
          return next;
        });
      }

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
    };

    poll();
    const intervalId = setInterval(poll, POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
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
        startPolling(result.deploymentId, gatewayId, result.status);

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
          startPolling(deploymentId, gatewayId, updated.status);
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
      startPolling,
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
        startPolling(result.deploymentId, gatewayId, result.status);

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
      startPolling,
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
      deployments: effectiveDeployments,
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
      readOnly: isReadOnly,
      canDeploy,
      canViewDeployments,
    }),
    [
      gateways,
      isLoading,
      error,
      fetchGateways,
      effectiveDeployments,
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
      isReadOnly,
      canDeploy,
      canViewDeployments,
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
