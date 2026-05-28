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

import React, { createContext, useContext, useEffect, useMemo, useState, useCallback } from 'react';
import type {
  Proxy,
  UpdateProxyRequest,
  APIKeyListResponse,
  CreateLLMProxyAPIKeyRequest,
  CreateLLMProxyAPIKeyResponse,
} from '../../utils/types';
import * as proxyApis from '../../apis/proxyApis';
import * as llmProxiesApis from '../../apis/llmProxiesApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import { logger } from '../../utils/logger';

// ============================================================================
// Single Proxy Context - For managing a single proxy by ID
// ============================================================================

type ProxyContextValue = {
  proxy: Proxy | null;
  isLoading: boolean;
  error: Error | null;
  /** Update a field locally without calling the API */
  setLocalProxy: React.Dispatch<React.SetStateAction<Proxy | null>>;
  updateProxy: (updates: UpdateProxyRequest) => Promise<Proxy>;
  deleteProxy: () => Promise<void>;
  getProxyAPIKeys: () => Promise<APIKeyListResponse>;
  createProxyAPIKey: (
    request: CreateLLMProxyAPIKeyRequest
  ) => Promise<CreateLLMProxyAPIKeyResponse>;
  deleteProxyAPIKey: (keyName: string) => Promise<void>;
  refetch: () => Promise<void>;
};

const ProxyContext = createContext<ProxyContextValue>({
  proxy: null,
  isLoading: false,
  error: null,
  setLocalProxy: () => {},
  updateProxy: async () => {
    throw new Error('ProxyContext not initialized');
  },
  deleteProxy: async () => {
    throw new Error('ProxyContext not initialized');
  },
  getProxyAPIKeys: async () => {
    throw new Error('ProxyContext not initialized');
  },
  createProxyAPIKey: async () => {
    throw new Error('ProxyContext not initialized');
  },
  deleteProxyAPIKey: async () => {
    throw new Error('ProxyContext not initialized');
  },
  refetch: async () => {
    throw new Error('ProxyContext not initialized');
  },
});

interface ProxyProviderProps {
  children: React.ReactNode;
  proxyId: string;
}

export function ProxyProvider({ children, proxyId }: ProxyProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [proxy, setProxy] = useState<Proxy | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch single proxy
  const fetchProxy = useCallback(async () => {
    if (!proxyId || !organizationId) {
      setProxy(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const fetchedProxy = await proxyApis.getProxy(proxyId, organizationId, apimBaseUrl);
      setProxy(fetchedProxy);
    } catch (err) {
      logger.error(`Failed to fetch proxy ${proxyId}:`, err);
      setError(err instanceof Error ? err : new Error('Failed to fetch proxy'));
      setProxy(null);
    } finally {
      setIsLoading(false);
    }
  }, [proxyId, organizationId, apimBaseUrl]);

  useEffect(() => {
    fetchProxy();
  }, [fetchProxy]);

  const updateProxy = useCallback(async (updates: UpdateProxyRequest): Promise<Proxy> => {
    if (!proxyId || !organizationId) {
      throw new Error('Proxy ID or Organization ID is missing');
    }
    try {
      const updatedProxy = await proxyApis.updateProxy(proxyId, updates, organizationId, apimBaseUrl);
      setProxy(updatedProxy);
      return updatedProxy;
    } catch (err) {
      logger.error('Failed to update proxy:', err);
      throw err;
    }
  }, [proxyId, organizationId, apimBaseUrl]);

  const deleteProxy = useCallback(async (): Promise<void> => {
    if (!proxyId || !organizationId) {
      throw new Error('Proxy ID or Organization ID is missing');
    }
    try {
      await proxyApis.deleteProxy(proxyId, organizationId, apimBaseUrl);
      setProxy(null);
    } catch (err) {
      logger.error('Failed to delete proxy:', err);
      throw err;
    }
  }, [proxyId, organizationId, apimBaseUrl]);

  const getProxyAPIKeys = useCallback(async (): Promise<APIKeyListResponse> => {
    if (!proxyId || !organizationId) {
      throw new Error('Proxy ID or Organization ID is missing');
    }
    try {
      return await llmProxiesApis.getLLMProxyAPIKeys(proxyId, organizationId);
    } catch (err) {
      logger.error(`Failed to fetch API keys for proxy ${proxyId}:`, err);
      throw err;
    }
  }, [proxyId, organizationId]);

  const createProxyAPIKey = useCallback(
    async (
      request: CreateLLMProxyAPIKeyRequest
    ): Promise<CreateLLMProxyAPIKeyResponse> => {
      if (!proxyId || !organizationId) {
        throw new Error('Proxy ID or Organization ID is missing');
      }
      try {
        return await llmProxiesApis.createLLMProxyAPIKey(
          proxyId,
          organizationId,
          request
        );
      } catch (err) {
        logger.error(`Failed to create API key for proxy ${proxyId}:`, err);
        throw err;
      }
    },
    [proxyId, organizationId]
  );

  const deleteProxyAPIKey = useCallback(
    async (keyName: string): Promise<void> => {
      if (!proxyId || !organizationId) {
        throw new Error('Proxy ID or Organization ID is missing');
      }
      try {
        await llmProxiesApis.deleteLLMProxyAPIKey(
          proxyId,
          keyName,
          organizationId
        );
      } catch (err) {
        logger.error(
          `Failed to delete API key ${keyName} for proxy ${proxyId}:`,
          err
        );
        throw err;
      }
    },
    [proxyId, organizationId]
  );

  const refetch = useCallback(async (): Promise<void> => {
    await fetchProxy();
  }, [fetchProxy]);

  const value = useMemo(
    () => ({
      proxy,
      isLoading,
      error,
      setLocalProxy: setProxy,
      updateProxy,
      deleteProxy,
      getProxyAPIKeys,
      createProxyAPIKey,
      deleteProxyAPIKey,
      refetch,
    }),
    [
      proxy,
      isLoading,
      error,
      updateProxy,
      deleteProxy,
      getProxyAPIKeys,
      createProxyAPIKey,
      deleteProxyAPIKey,
      refetch,
    ]
  );

  return (
    <ProxyContext.Provider value={value}>
      {children}
    </ProxyContext.Provider>
  );
}

export function useProxy(): ProxyContextValue {
  return useContext(ProxyContext);
}
