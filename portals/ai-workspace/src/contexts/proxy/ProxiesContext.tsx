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

import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  useCallback,
} from 'react';
import type {
  Proxy,
  CreateProxyRequest,
  UpdateProxyRequest,
  ProxiesResponse,
} from '../../utils/types';
import * as proxyApis from '../../apis/proxyApis';
import {
  createSecret,
  deleteSecret,
  buildSecretPlaceholder,
  generateSecretHandle,
  extractSecretHandle,
} from '../../apis/secretApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import { logger } from '../../utils/logger';
import {
  trackLLMProxyCreate,
  trackLLMProxyUpdate,
  trackLLMProxyDelete,
} from '../../utils/app-insights';

// ============================================================================
// Proxies List Context - For managing the list of all proxies
// ============================================================================

const EMPTY_PROXIES_RESPONSE: ProxiesResponse = {
  count: 0,
  list: [],
  pagination: { total: 0, offset: 0, limit: 20 },
};

type ProxiesContextValue = {
  /** Full API response with count, list, and pagination */
  proxiesResponse: ProxiesResponse;
  isLoading: boolean;
  error: Error | null;
  createProxy: (proxy: CreateProxyRequest) => Promise<Proxy>;
  updateProxy: (proxyId: string, updates: UpdateProxyRequest) => Promise<Proxy>;
  deleteProxy: (proxyId: string) => Promise<void>;
  refreshProxies: () => Promise<void>;
  getProxyById: (proxyId: string) => Proxy | undefined;
};

const ProxiesContext = createContext<ProxiesContextValue>({
  proxiesResponse: EMPTY_PROXIES_RESPONSE,
  isLoading: false,
  error: null,
  createProxy: async () => {
    throw new Error('ProxiesContext not initialized');
  },
  updateProxy: async () => {
    throw new Error('ProxiesContext not initialized');
  },
  deleteProxy: async () => {
    throw new Error('ProxiesContext not initialized');
  },
  refreshProxies: async () => {
    throw new Error('ProxiesContext not initialized');
  },
  getProxyById: () => undefined,
});

interface ProxiesProviderProps {
  children: React.ReactNode;
}

export function ProxiesProvider({ children }: ProxiesProviderProps) {
  const { currentOrganization, currentProject } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [proxiesResponse, setProxiesResponse] = useState<ProxiesResponse>(
    EMPTY_PROXIES_RESPONSE
  );
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';
  const projectId = currentProject?.id ?? '';

  // Fetch all proxies
  const fetchProxies = useCallback(async () => {
    if (!organizationId || !projectId) {
      setProxiesResponse(EMPTY_PROXIES_RESPONSE);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const response = await proxyApis.getProxies(
        organizationId,
        projectId,
        apimBaseUrl
      );
      setProxiesResponse(response as ProxiesResponse);
    } catch (err) {
      logger.error('Failed to fetch proxies:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch proxies')
      );
    } finally {
      setIsLoading(false);
    }
  }, [organizationId, projectId, apimBaseUrl]);

  useEffect(() => {
    fetchProxies();
  }, [fetchProxies]);

  const createProxy = useCallback(
    async (proxy: CreateProxyRequest): Promise<Proxy> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        const newProxy = await proxyApis.createProxy(
          proxy,
          organizationId,
          apimBaseUrl
        );
        setProxiesResponse((prev) => ({
          ...prev,
          count: prev.count + 1,
          list: [newProxy, ...prev.list],
          pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
        }));

        // Track LLM Proxy creation
        trackLLMProxyCreate(
          organizationId,
          newProxy.id ?? proxy.id,
          proxy.providers ?? []
        );

        return newProxy;
      } catch (err) {
        logger.error('Failed to create proxy:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const updateProxy = useCallback(
    async (proxyId: string, updates: UpdateProxyRequest): Promise<Proxy> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        // If the provider auth value is a new plain-text credential (not already
        // a placeholder), create a new secret and substitute the placeholder.
        // After a successful proxy update the old secret is deleted best-effort.
        let updatesPayload = updates;
        const providerAuth = typeof updates.provider === 'object' ? updates.provider?.auth : undefined;
        const authValue = providerAuth?.value;
        const isAlreadyPlaceholder =
          typeof authValue === 'string' && authValue.includes('{{ secret ');

        if (authValue && !isAlreadyPlaceholder) {
          const secretHandle = generateSecretHandle();
          const secretResponse = await createSecret({
            id: secretHandle,
            displayName: `${proxyId} provider API Key`,
            description: `Auto-generated secret for LLM proxy ${proxyId}`,
            value: authValue,
            type: 'GENERIC',
          });
          logger.info('Created new secret for LLM proxy update', { secretHandle, proxyId });

          updatesPayload = {
            ...updates,
            provider: {
              ...(typeof updates.provider === 'object' ? updates.provider : { id: updates.provider ?? '' }),
              auth: {
                ...providerAuth,
                value: buildSecretPlaceholder(secretResponse.id),
              },
            },
          };
        }

        const updatedProxy = await proxyApis.updateProxy(
          proxyId,
          updatesPayload,
          organizationId,
          apimBaseUrl
        );

        // Best-effort: delete the old secret after the proxy update succeeds.
        if (authValue && !isAlreadyPlaceholder) {
          const currentProxy = proxiesResponse.list.find((p) => p.id === proxyId);
          const existingAuth = typeof currentProxy?.provider === 'object' ? currentProxy?.provider?.auth : undefined;
          const oldHandle = existingAuth?.value ? extractSecretHandle(existingAuth.value) : null;
          if (oldHandle) {
            deleteSecret(oldHandle).catch((err) => {
              logger.warn('Could not delete old secret after proxy update', { oldHandle, err });
            });
          }
        }

        setProxiesResponse((prev) => ({
          ...prev,
          list: prev.list.map((proxy) =>
            proxy.id === proxyId ? updatedProxy : proxy
          ),
        }));

        // Track LLM Proxy update
        trackLLMProxyUpdate(organizationId, proxyId, updates.providers);

        return updatedProxy;
      } catch (err) {
        logger.error('Failed to update proxy:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl, proxiesResponse.list]
  );

  const deleteProxy = useCallback(
    async (proxyId: string): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        await proxyApis.deleteProxy(proxyId, organizationId, apimBaseUrl);
        setProxiesResponse((prev) => ({
          ...prev,
          count: Math.max(0, prev.count - 1),
          list: prev.list.filter((proxy) => proxy.id !== proxyId),
          pagination: {
            ...prev.pagination,
            total: Math.max(0, prev.pagination.total - 1),
          },
        }));

        // Track LLM Proxy deletion
        trackLLMProxyDelete(organizationId, proxyId);
      } catch (err) {
        logger.error('Failed to delete proxy:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const refreshProxies = useCallback(async (): Promise<void> => {
    await fetchProxies();
  }, [fetchProxies]);

  const getProxyById = useCallback(
    (proxyId: string): Proxy | undefined => {
      return proxiesResponse.list.find((proxy) => proxy.id === proxyId);
    },
    [proxiesResponse.list]
  );

  const value = useMemo(
    () => ({
      proxiesResponse,
      isLoading,
      error,
      createProxy,
      updateProxy,
      deleteProxy,
      refreshProxies,
      getProxyById,
    }),
    [
      proxiesResponse,
      isLoading,
      error,
      createProxy,
      updateProxy,
      deleteProxy,
      refreshProxies,
      getProxyById,
    ]
  );

  return (
    <ProxiesContext.Provider value={value}>{children}</ProxiesContext.Provider>
  );
}

export function useProxies(): ProxiesContextValue {
  return useContext(ProxiesContext);
}
