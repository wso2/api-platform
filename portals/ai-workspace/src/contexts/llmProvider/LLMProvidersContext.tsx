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
  LLMProvider,
  CreateLLMProviderRequest,
  UpdateLLMProviderRequest,
  LLMProvidersResponse,
} from '../../utils/types';
import * as llmProviderApis from '../../apis/llmProviderApis';
import {
  createSecret,
  buildSecretPlaceholder,
  generateSecretHandle,
} from '../../apis/secretApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import { logger } from '../../utils/logger';
import {
  trackLLMProviderCreate,
  trackLLMProviderUpdate,
  trackLLMProviderDelete,
} from '../../utils/app-insights';

// ============================================================================
// LLM Providers List Context - For managing the list of all providers
// ============================================================================

// Default empty response matching API format
const EMPTY_PROVIDERS_RESPONSE: LLMProvidersResponse = {
  count: 0,
  list: [],
  pagination: { total: 0, offset: 0, limit: 20 },
};

type LLMProvidersContextValue = {
  /** Full API response with count, list, and pagination */
  providersResponse: LLMProvidersResponse;
  isLoading: boolean;
  error: Error | null;
  createProvider: (provider: CreateLLMProviderRequest) => Promise<LLMProvider>;
  updateProvider: (
    providerId: string,
    updates: UpdateLLMProviderRequest
  ) => Promise<LLMProvider>;
  deleteProvider: (providerId: string) => Promise<void>;
  refreshProviders: () => Promise<void>;
  getProviderById: (providerId: string) => LLMProvider | undefined;
};

const LLMProvidersContext = createContext<LLMProvidersContextValue>({
  providersResponse: EMPTY_PROVIDERS_RESPONSE,
  isLoading: false,
  error: null,
  createProvider: async () => {
    throw new Error('LLMProvidersContext not initialized');
  },
  updateProvider: async () => {
    throw new Error('LLMProvidersContext not initialized');
  },
  deleteProvider: async () => {
    throw new Error('LLMProvidersContext not initialized');
  },
  refreshProviders: async () => {
    throw new Error('LLMProvidersContext not initialized');
  },
  getProviderById: () => undefined,
});

interface LLMProvidersProviderProps {
  children: React.ReactNode;
}

export function LLMProvidersProvider({ children }: LLMProvidersProviderProps) {
  const { currentOrganization } = useAppShell();
  const [providersResponse, setProvidersResponse] =
    useState<LLMProvidersResponse>(EMPTY_PROVIDERS_RESPONSE);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch all providers
  const fetchProviders = useCallback(async () => {
    if (!organizationId) {
      setProvidersResponse(EMPTY_PROVIDERS_RESPONSE);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const response = await llmProviderApis.getLLMProviders(
        organizationId,
        PLATFORM_API_BASE_URL
      );
      // Store the full API response as-is
      setProvidersResponse(response as LLMProvidersResponse);
    } catch (err) {
      logger.error('Failed to fetch LLM providers:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch providers')
      );
      // Keep existing response on error
    } finally {
      setIsLoading(false);
    }
  }, [organizationId, PLATFORM_API_BASE_URL]);

  useEffect(() => {
    fetchProviders();
  }, [fetchProviders]);

  const createProvider = useCallback(
    async (provider: CreateLLMProviderRequest): Promise<LLMProvider> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        // If the upstream auth value contains a plain-text credential (not already
        // a secret placeholder), encrypt it via the Platform API secrets endpoint
        // and substitute a {{ secret "..." }} placeholder before persisting.
        let providerPayload = provider;
        const authValue = provider.upstream?.main?.auth?.value;
        const isAlreadyPlaceholder =
          typeof authValue === 'string' && authValue.includes('{{ secret ');

        if (authValue && !isAlreadyPlaceholder) {
          const secretHandle = generateSecretHandle(provider.id, 'api-key');
          const secretResponse = await createSecret(
            {
              name: secretHandle,
              displayName: `${provider.name} API Key`,
              description: `Auto-generated secret for LLM provider ${provider.name}`,
              value: authValue,
              type: 'API_KEY',
            },
            PLATFORM_API_BASE_URL
          );
          logger.info('Created secret for LLM provider', {
            secretName: secretResponse.name,
            providerId: provider.id,
          });

          providerPayload = {
            ...provider,
            upstream: {
              ...provider.upstream,
              main: {
                ...provider.upstream.main,
                auth: {
                  ...provider.upstream.main.auth,
                  value: buildSecretPlaceholder(secretResponse.name),
                },
              },
            },
          };
        }

        const newProvider = await llmProviderApis.createLLMProvider(
          providerPayload,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        setProvidersResponse((prev) => ({
          ...prev,
          count: prev.count + 1,
          list: [newProvider, ...prev.list],
          pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
        }));

        // Track LLM Provider creation
        trackLLMProviderCreate(
          organizationId,
          newProvider.id ?? provider.id,
          provider.template ?? 'custom'
        );

        return newProvider;
      } catch (err) {
        logger.error('Failed to create LLM provider:', err);
        throw err;
      }
    },
    [organizationId, PLATFORM_API_BASE_URL]
  );

  const updateProvider = useCallback(
    async (
      providerId: string,
      updates: UpdateLLMProviderRequest
    ): Promise<LLMProvider> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        // If the upstream auth value is a new plain-text credential (not already a
        // placeholder), rotate the existing secret via PUT /secrets/:id or create a
        // new one, then substitute the placeholder before persisting.
        let updatesPayload = updates;
        const authValue = updates.upstream?.main?.auth?.value;
        const isAlreadyPlaceholder =
          typeof authValue === 'string' && authValue.includes('{{ secret ');

        if (authValue && !isAlreadyPlaceholder) {
          const secretHandle = generateSecretHandle(providerId, 'api-key');
          const secretResponse = await createSecret(
            {
              name: secretHandle,
              displayName: `${providerId} API Key`,
              description: `Auto-generated secret for LLM provider ${providerId}`,
              value: authValue,
              type: 'API_KEY',
            },
            PLATFORM_API_BASE_URL
          );
          logger.info('Rotated/created secret for LLM provider update', {
            secretName: secretResponse.name,
            providerId,
          });

          updatesPayload = {
            ...updates,
            upstream: {
              ...updates.upstream,
              main: {
                ...updates.upstream?.main,
                url: updates.upstream?.main?.url ?? '',
                auth: {
                  ...updates.upstream?.main?.auth,
                  value: buildSecretPlaceholder(secretResponse.name),
                },
              },
            },
          };
        }

        const updatedProvider = await llmProviderApis.updateLLMProvider(
          providerId,
          updatesPayload,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        setProvidersResponse((prev) => ({
          ...prev,
          list: prev.list.map((provider) =>
            provider.id === providerId ? updatedProvider : provider
          ),
        }));

        // Track LLM Provider update
        trackLLMProviderUpdate(
          organizationId,
          providerId,
          updatedProvider.template ?? 'custom'
        );

        return updatedProvider;
      } catch (err) {
        logger.error('Failed to update LLM provider:', err);
        throw err;
      }
    },
    [organizationId, PLATFORM_API_BASE_URL]
  );

  const deleteProvider = useCallback(
    async (providerId: string): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        // Get provider info before deletion for tracking
        const providerToDelete = providersResponse.list.find(
          (p) => p.id === providerId
        );

        await llmProviderApis.deleteLLMProvider(
          providerId,
          organizationId,
          PLATFORM_API_BASE_URL
        );
        setProvidersResponse((prev) => ({
          ...prev,
          count: Math.max(0, prev.count - 1),
          list: prev.list.filter((provider) => provider.id !== providerId),
          pagination: {
            ...prev.pagination,
            total: Math.max(0, prev.pagination.total - 1),
          },
        }));

        // Track LLM Provider deletion
        trackLLMProviderDelete(
          organizationId,
          providerId,
          providerToDelete?.template ?? ''
        );
      } catch (err) {
        logger.error('Failed to delete LLM provider:', err);
        throw err;
      }
    },
    [organizationId, PLATFORM_API_BASE_URL]
  );

  const refreshProviders = useCallback(async (): Promise<void> => {
    await fetchProviders();
  }, [fetchProviders]);

  const getProviderById = useCallback(
    (providerId: string): LLMProvider | undefined => {
      return providersResponse.list.find(
        (provider) => provider.id === providerId
      );
    },
    [providersResponse.list]
  );

  const value = useMemo(
    () => ({
      providersResponse,
      isLoading,
      error,
      createProvider,
      updateProvider,
      deleteProvider,
      refreshProviders,
      getProviderById,
    }),
    [
      providersResponse,
      isLoading,
      error,
      createProvider,
      updateProvider,
      deleteProvider,
      refreshProviders,
      getProviderById,
    ]
  );

  return (
    <LLMProvidersContext.Provider value={value}>
      {children}
    </LLMProvidersContext.Provider>
  );
}

export function useLLMProviders(): LLMProvidersContextValue {
  return useContext(LLMProvidersContext);
}
