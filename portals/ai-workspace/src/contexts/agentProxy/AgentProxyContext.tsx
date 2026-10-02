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
  UpdateAgentProxyRequest,
  APIKeyListResponse,
  CreateAgentProxyAPIKeyRequest,
  CreateAgentProxyAPIKeyResponse,
} from '../../utils/types';
import * as agentProxiesApis from '../../apis/agent/agentProxiesApis';
import { PLATFORM_API_BASE_URL } from '../../paths';
import { logger } from '../../utils/logger';

// ============================================================================
// Single Agent Proxy Context - For managing a single Agent proxy by ID
// ============================================================================

type AgentProxyContextValue = {
  agentProxy: AgentProxy | null;
  isLoading: boolean;
  error: Error | null;
  /** Update a field locally without calling the API */
  setLocalAgentProxy: React.Dispatch<React.SetStateAction<AgentProxy | null>>;
  updateAgentProxy: (updates: UpdateAgentProxyRequest) => Promise<AgentProxy>;
  deleteAgentProxy: () => Promise<void>;
  getAgentProxyAPIKeys: () => Promise<APIKeyListResponse>;
  createAgentProxyAPIKey: (
    request: CreateAgentProxyAPIKeyRequest
  ) => Promise<CreateAgentProxyAPIKeyResponse>;
  revokeAgentProxyAPIKey: (apiKeyId: string) => Promise<void>;
  refetch: () => Promise<void>;
};

const AgentProxyContext = createContext<AgentProxyContextValue>({
  agentProxy: null,
  isLoading: false,
  error: null,
  setLocalAgentProxy: () => {},
  updateAgentProxy: async () => {
    throw new Error('AgentProxyContext not initialized');
  },
  deleteAgentProxy: async () => {
    throw new Error('AgentProxyContext not initialized');
  },
  getAgentProxyAPIKeys: async () => {
    throw new Error('AgentProxyContext not initialized');
  },
  createAgentProxyAPIKey: async () => {
    throw new Error('AgentProxyContext not initialized');
  },
  revokeAgentProxyAPIKey: async () => {
    throw new Error('AgentProxyContext not initialized');
  },
  refetch: async () => {
    throw new Error('AgentProxyContext not initialized');
  },
});

interface AgentProxyProviderProps {
  children: React.ReactNode;
  agentProxyId: string;
}

export function AgentProxyProvider({
  children,
  agentProxyId,
}: AgentProxyProviderProps) {
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [agentProxy, setAgentProxy] = useState<AgentProxy | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  // Navigating between proxies leaves the previous fetch in flight; only the
  // most recent request, clears included, is allowed to write.
  const agentProxyRequestRef = useRef(0);

  // Fetch single Agent proxy
  const fetchAgentProxy = useCallback(async () => {
    const requestId = (agentProxyRequestRef.current += 1);
    if (!agentProxyId) {
      setAgentProxy(null);
      setError(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const fetched = await agentProxiesApis.getAgentProxy(
        agentProxyId,
        apimBaseUrl
      );
      if (agentProxyRequestRef.current !== requestId) return;
      setAgentProxy(fetched);
    } catch (err) {
      if (agentProxyRequestRef.current !== requestId) return;
      logger.error(`Failed to fetch Agent proxy ${agentProxyId}:`, err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch Agent proxy')
      );
      setAgentProxy(null);
    } finally {
      if (agentProxyRequestRef.current === requestId) {
        setIsLoading(false);
      }
    }
  }, [agentProxyId, apimBaseUrl]);

  useEffect(() => {
    fetchAgentProxy();
  }, [fetchAgentProxy]);

  const updateAgentProxy = useCallback(
    async (updates: UpdateAgentProxyRequest): Promise<AgentProxy> => {
      if (!agentProxyId) {
        throw new Error('Agent proxy ID is missing');
      }
      try {
        const updated = await agentProxiesApis.updateAgentProxy(
          agentProxyId,
          updates,
          apimBaseUrl
        );
        setAgentProxy(updated);
        return updated;
      } catch (err) {
        logger.error('Failed to update Agent proxy:', err);
        throw err;
      }
    },
    [agentProxyId, apimBaseUrl]
  );

  const deleteAgentProxy = useCallback(async (): Promise<void> => {
    if (!agentProxyId) {
      throw new Error('Agent proxy ID is missing');
    }
    try {
      await agentProxiesApis.deleteAgentProxy(agentProxyId, apimBaseUrl);
      setAgentProxy(null);
    } catch (err) {
      logger.error('Failed to delete Agent proxy:', err);
      throw err;
    }
  }, [agentProxyId, apimBaseUrl]);

  const getAgentProxyAPIKeys =
    useCallback(async (): Promise<APIKeyListResponse> => {
      if (!agentProxyId) {
        throw new Error('Agent proxy ID is missing');
      }
      try {
        return await agentProxiesApis.getAgentProxyAPIKeys(
          agentProxyId,
          apimBaseUrl
        );
      } catch (err) {
        logger.error(
          `Failed to fetch API keys for Agent proxy ${agentProxyId}:`,
          err
        );
        throw err;
      }
    }, [agentProxyId, apimBaseUrl]);

  const createAgentProxyAPIKey = useCallback(
    async (
      request: CreateAgentProxyAPIKeyRequest
    ): Promise<CreateAgentProxyAPIKeyResponse> => {
      if (!agentProxyId) {
        throw new Error('Agent proxy ID is missing');
      }
      try {
        return await agentProxiesApis.createAgentProxyAPIKey(
          agentProxyId,
          request,
          apimBaseUrl
        );
      } catch (err) {
        logger.error(
          `Failed to create API key for Agent proxy ${agentProxyId}:`,
          err
        );
        throw err;
      }
    },
    [agentProxyId, apimBaseUrl]
  );

  const revokeAgentProxyAPIKey = useCallback(
    async (apiKeyId: string): Promise<void> => {
      if (!agentProxyId) {
        throw new Error('Agent proxy ID is missing');
      }
      try {
        await agentProxiesApis.revokeAgentProxyAPIKey(
          agentProxyId,
          apiKeyId,
          apimBaseUrl
        );
      } catch (err) {
        logger.error(
          `Failed to revoke API key ${apiKeyId} for Agent proxy ${agentProxyId}:`,
          err
        );
        throw err;
      }
    },
    [agentProxyId, apimBaseUrl]
  );

  const refetch = useCallback(async (): Promise<void> => {
    await fetchAgentProxy();
  }, [fetchAgentProxy]);

  const value = useMemo(
    () => ({
      agentProxy,
      isLoading,
      error,
      setLocalAgentProxy: setAgentProxy,
      updateAgentProxy,
      deleteAgentProxy,
      getAgentProxyAPIKeys,
      createAgentProxyAPIKey,
      revokeAgentProxyAPIKey,
      refetch,
    }),
    [
      agentProxy,
      isLoading,
      error,
      updateAgentProxy,
      deleteAgentProxy,
      getAgentProxyAPIKeys,
      createAgentProxyAPIKey,
      revokeAgentProxyAPIKey,
      refetch,
    ]
  );

  return (
    <AgentProxyContext.Provider value={value}>
      {children}
    </AgentProxyContext.Provider>
  );
}

export function useAgentProxy(): AgentProxyContextValue {
  return useContext(AgentProxyContext);
}
