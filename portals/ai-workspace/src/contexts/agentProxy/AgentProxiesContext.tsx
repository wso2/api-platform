/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  useCallback,
} from 'react';
import type {
  AgentProxy,
  AgentProxyListItem,
  AgentProxyListResponse,
  CreateAgentProxyRequest,
  UpdateAgentProxyRequest,
} from '../../utils/types';
import * as agentProxiesApis from '../../apis/agent/agentProxiesApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../paths';
import { logger } from '../../utils/logger';

// ============================================================================
// Agent Proxies List Context - For managing the list of all Agent proxies
// ============================================================================

const EMPTY_AGENT_PROXIES_RESPONSE: AgentProxyListResponse = {
  count: 0,
  list: [],
  pagination: { total: 0, offset: 0, limit: 20 },
};

/** The list carries a projection of the resource, so a created or updated
 * Agent proxy is narrowed to those fields before it goes into the list. */
const toListItem = (agentProxy: AgentProxy): AgentProxyListItem => ({
  id: agentProxy.id ?? '',
  displayName: agentProxy.displayName,
  description: agentProxy.description,
  version: agentProxy.version,
  projectId: agentProxy.projectId,
  protocol: agentProxy.protocol,
  context: agentProxy.context,
  vhost: agentProxy.vhost,
  readOnly: agentProxy.readOnly,
  createdAt: agentProxy.createdAt,
  createdBy: agentProxy.createdBy,
  updatedAt: agentProxy.updatedAt,
  updatedBy: agentProxy.updatedBy,
});

type AgentProxiesContextValue = {
  /** Full API response with count, list, and pagination */
  agentProxiesResponse: AgentProxyListResponse;
  isLoading: boolean;
  error: Error | null;
  createAgentProxy: (agentProxy: CreateAgentProxyRequest) => Promise<AgentProxy>;
  updateAgentProxy: (
    agentProxyId: string,
    updates: UpdateAgentProxyRequest
  ) => Promise<AgentProxy>;
  deleteAgentProxy: (agentProxyId: string) => Promise<void>;
  refreshAgentProxies: () => Promise<void>;
  getAgentProxyById: (agentProxyId: string) => AgentProxyListItem | undefined;
};

const AgentProxiesContext = createContext<AgentProxiesContextValue>({
  agentProxiesResponse: EMPTY_AGENT_PROXIES_RESPONSE,
  isLoading: false,
  error: null,
  createAgentProxy: async () => {
    throw new Error('AgentProxiesContext not initialized');
  },
  updateAgentProxy: async () => {
    throw new Error('AgentProxiesContext not initialized');
  },
  deleteAgentProxy: async () => {
    throw new Error('AgentProxiesContext not initialized');
  },
  refreshAgentProxies: async () => {
    throw new Error('AgentProxiesContext not initialized');
  },
  getAgentProxyById: () => undefined,
});

interface AgentProxiesProviderProps {
  children: React.ReactNode;
}

export function AgentProxiesProvider({ children }: AgentProxiesProviderProps) {
  const { currentProject } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [agentProxiesResponse, setAgentProxiesResponse] =
    useState<AgentProxyListResponse>(EMPTY_AGENT_PROXIES_RESPONSE);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const projectId = currentProject?.id ?? '';

  // Switching projects leaves the previous fetch in flight; only the most
  // recent request is allowed to write, so a slower earlier one cannot land
  // another project's Agent proxies on screen.
  const agentProxiesRequestRef = useRef(0);

  const fetchAgentProxies = useCallback(async () => {
    if (!projectId) {
      setAgentProxiesResponse(EMPTY_AGENT_PROXIES_RESPONSE);
      setIsLoading(false);
      return;
    }

    const requestId = (agentProxiesRequestRef.current += 1);
    try {
      setIsLoading(true);
      setError(null);
      const response = await agentProxiesApis.getAgentProxies(
        projectId,
        apimBaseUrl
      );
      if (agentProxiesRequestRef.current !== requestId) return;
      setAgentProxiesResponse(response);
    } catch (err) {
      if (agentProxiesRequestRef.current !== requestId) return;
      logger.error('Failed to fetch Agent proxies:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch Agent proxies')
      );
    } finally {
      if (agentProxiesRequestRef.current === requestId) {
        setIsLoading(false);
      }
    }
  }, [projectId, apimBaseUrl]);

  useEffect(() => {
    void fetchAgentProxies();
  }, [fetchAgentProxies]);

  const createAgentProxy = useCallback(
    async (agentProxy: CreateAgentProxyRequest): Promise<AgentProxy> => {
      try {
        const created = await agentProxiesApis.createAgentProxy(
          agentProxy,
          apimBaseUrl
        );
        setAgentProxiesResponse((prev) => ({
          ...prev,
          count: prev.count + 1,
          list: [toListItem(created), ...prev.list],
          pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
        }));
        return created;
      } catch (err) {
        logger.error('Failed to create Agent proxy:', err);
        throw err;
      }
    },
    [apimBaseUrl]
  );

  const updateAgentProxy = useCallback(
    async (
      agentProxyId: string,
      updates: UpdateAgentProxyRequest
    ): Promise<AgentProxy> => {
      try {
        const updated = await agentProxiesApis.updateAgentProxy(
          agentProxyId,
          updates,
          apimBaseUrl
        );
        setAgentProxiesResponse((prev) => ({
          ...prev,
          list: prev.list.map((item) =>
            item.id === agentProxyId ? toListItem(updated) : item
          ),
        }));
        return updated;
      } catch (err) {
        logger.error('Failed to update Agent proxy:', err);
        throw err;
      }
    },
    [apimBaseUrl]
  );

  const deleteAgentProxy = useCallback(
    async (agentProxyId: string): Promise<void> => {
      try {
        await agentProxiesApis.deleteAgentProxy(agentProxyId, apimBaseUrl);
        setAgentProxiesResponse((prev) => ({
          ...prev,
          count: Math.max(0, prev.count - 1),
          list: prev.list.filter((item) => item.id !== agentProxyId),
          pagination: {
            ...prev.pagination,
            total: Math.max(0, prev.pagination.total - 1),
          },
        }));
      } catch (err) {
        logger.error('Failed to delete Agent proxy:', err);
        throw err;
      }
    },
    [apimBaseUrl]
  );

  const refreshAgentProxies = useCallback(async (): Promise<void> => {
    await fetchAgentProxies();
  }, [fetchAgentProxies]);

  const getAgentProxyById = useCallback(
    (agentProxyId: string): AgentProxyListItem | undefined => {
      return agentProxiesResponse.list.find((item) => item.id === agentProxyId);
    },
    [agentProxiesResponse.list]
  );

  const value = useMemo(
    () => ({
      agentProxiesResponse,
      isLoading,
      error,
      createAgentProxy,
      updateAgentProxy,
      deleteAgentProxy,
      refreshAgentProxies,
      getAgentProxyById,
    }),
    [
      agentProxiesResponse,
      isLoading,
      error,
      createAgentProxy,
      updateAgentProxy,
      deleteAgentProxy,
      refreshAgentProxies,
      getAgentProxyById,
    ]
  );

  return (
    <AgentProxiesContext.Provider value={value}>
      {children}
    </AgentProxiesContext.Provider>
  );
}

export function useAgentProxies(): AgentProxiesContextValue {
  return useContext(AgentProxiesContext);
}
