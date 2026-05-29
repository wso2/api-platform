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
  LLMProvider,
  ProxiesResponse,
  UpdateLLMProviderRequest,
  APIKeyListResponse,
} from '../../utils/types';
import * as llmProviderApis from '../../apis/llmProviderApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import { logger } from '../../utils/logger';

// ============================================================================
// Single LLM Provider Context - For managing a single provider by ID
// ============================================================================

export type LLMProviderContextValue = {
  provider: LLMProvider | null;
  isLoading: boolean;
  error: Error | null;
  getProviderProxies: () => Promise<ProxiesResponse>;
  updateProvider: (updates: UpdateLLMProviderRequest) => Promise<LLMProvider>;
  deleteProvider: () => Promise<void>;
  getProviderAPIKeys: () => Promise<APIKeyListResponse>;
  deleteProviderAPIKey: (keyName: string) => Promise<void>;
  refetch: () => Promise<void>;
  isDraftMode: boolean;
};

export const LLMProviderContext = createContext<LLMProviderContextValue>({
  provider: null,
  isLoading: false,
  error: null,
  getProviderProxies: async () => {
    throw new Error('LLMProviderContext not initialized');
  },
  updateProvider: async () => {
    throw new Error('LLMProviderContext not initialized');
  },
  deleteProvider: async () => {
    throw new Error('LLMProviderContext not initialized');
  },
  getProviderAPIKeys: async () => {
    throw new Error('LLMProviderContext not initialized');
  },
  deleteProviderAPIKey: async () => {
    throw new Error('LLMProviderContext not initialized');
  },
  refetch: async () => {
    throw new Error('LLMProviderContext not initialized');
  },
  isDraftMode: false,
});

interface LLMProviderProviderProps {
  children: React.ReactNode;
  providerId: string;
}

export function LLMProviderProvider({ children, providerId }: LLMProviderProviderProps) {
  const { currentOrganization } = useAppShell();
  const [provider, setProvider] = useState<LLMProvider | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch single provider
  const fetchProvider = useCallback(async () => {
    if (!providerId || !organizationId) {
      setProvider(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const fetchedProvider = await llmProviderApis.getLLMProvider(providerId, organizationId, PLATFORM_API_BASE_URL);
      setProvider(fetchedProvider);
    } catch (err) {
      logger.error(`Failed to fetch LLM provider ${providerId}:`, err);
      setError(err instanceof Error ? err : new Error('Failed to fetch provider'));
      setProvider(null);
    } finally {
      setIsLoading(false);
    }
  }, [providerId, organizationId]);

  useEffect(() => {
    fetchProvider();
  }, [fetchProvider]);

  const updateProvider = useCallback(async (updates: UpdateLLMProviderRequest): Promise<LLMProvider> => {
    if (!providerId || !organizationId) {
      throw new Error('Provider ID or Organization ID is missing');
    }
    try {
      const updatedProvider = await llmProviderApis.updateLLMProvider(providerId, updates, organizationId, PLATFORM_API_BASE_URL);
      setProvider(updatedProvider);
      return updatedProvider;
    } catch (err) {
      logger.error('Failed to update LLM provider:', err);
      throw err;
    }
  }, [providerId, organizationId, PLATFORM_API_BASE_URL]);

  const deleteProvider = useCallback(async (): Promise<void> => {
    if (!providerId || !organizationId) {
      throw new Error('Provider ID or Organization ID is missing');
    }
    try {
      await llmProviderApis.deleteLLMProvider(providerId, organizationId, PLATFORM_API_BASE_URL);
      setProvider(null);
    } catch (err) {
      logger.error('Failed to delete LLM provider:', err);
      throw err;
    }
  }, [providerId, organizationId, PLATFORM_API_BASE_URL]);

  const getProviderAPIKeys = useCallback(async (): Promise<APIKeyListResponse> => {
    if (!providerId || !organizationId) {
      throw new Error('Provider ID or Organization ID is missing');
    }

    try {
      return await llmProviderApis.getLLMProviderAPIKeys(
        providerId,
        organizationId
      );
    } catch (err) {
      logger.error(
        `Failed to fetch API keys for provider ${providerId}:`,
        err
      );
      throw err;
    }
  }, [providerId, organizationId, PLATFORM_API_BASE_URL]);

  const deleteProviderAPIKey = useCallback(async (keyName: string): Promise<void> => {
    if (!providerId || !organizationId) {
      throw new Error('Provider ID or Organization ID is missing');
    }

    try {
      await llmProviderApis.deleteLLMProviderAPIKey(
        providerId,
        keyName,
        organizationId,
        PLATFORM_API_BASE_URL
      );
    } catch (err) {
      logger.error(
        `Failed to delete API key ${keyName} for provider ${providerId}:`,
        err
      );
      throw err;
    }
  }, [providerId, organizationId, PLATFORM_API_BASE_URL]);

  const refetch = useCallback(async (): Promise<void> => {
    await fetchProvider();
  }, [fetchProvider]);

  const getProviderProxies = useCallback(async (): Promise<ProxiesResponse> => {
    if (!providerId || !organizationId) {
      throw new Error('Provider ID or Organization ID is missing');
    }

    try {
      return await llmProviderApis.getLLMProviderProxies(
        providerId,
        organizationId,
        PLATFORM_API_BASE_URL
      );
    } catch (err) {
      logger.error(
        `Failed to fetch LLM proxies for provider ${providerId}:`,
        err
      );
      throw err;
    }
  }, [providerId, organizationId, PLATFORM_API_BASE_URL]);

  const value = useMemo(
    () => ({
      provider,
      isLoading,
      error,
      getProviderProxies,
      updateProvider,
      deleteProvider,
      getProviderAPIKeys,
      deleteProviderAPIKey,
      refetch,
      isDraftMode: false,
    }),
    [
      provider,
      isLoading,
      error,
      getProviderProxies,
      updateProvider,
      deleteProvider,
      getProviderAPIKeys,
      deleteProviderAPIKey,
      refetch,
    ]
  );

  return (
    <LLMProviderContext.Provider value={value}>
      {children}
    </LLMProviderContext.Provider>
  );
}

export function useLLMProvider(): LLMProviderContextValue {
  return useContext(LLMProviderContext);
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Format a date string to a relative time string
 */
export function formatRelativeTime(value?: string): string {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown';

  const now = Date.now();
  const diffMs = now - date.getTime();
  const diffSeconds = Math.abs(diffMs) / 1000;

  if (diffSeconds < 45) return 'Just now';
  if (diffSeconds < 90) return '1 minute ago';

  const diffMinutes = diffSeconds / 60;
  if (diffMinutes < 45) return `${Math.round(diffMinutes)} minutes ago`;
  if (diffMinutes < 90) return '1 hour ago';

  const diffHours = diffMinutes / 60;
  if (diffHours < 22) return `${Math.round(diffHours)} hours ago`;
  if (diffHours < 36) return '1 day ago';

  const diffDays = diffHours / 24;
  if (diffDays < 26) return `${Math.round(diffDays)} days ago`;
  if (diffDays < 45) return '1 month ago';

  const diffMonths = diffDays / 30;
  if (diffMonths < 11) return `${Math.round(diffMonths)} months ago`;
  if (diffMonths < 18) return '1 year ago';

  const diffYears = diffDays / 365;
  return `${Math.round(diffYears)} years ago`;
}

/**
 * Build a unique provider ID from a name
 */
export function buildProviderId(name: string, takenIds: Set<string>): string {
  const base = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');

  if (!takenIds.has(base)) return base;

  let suffix = 2;
  while (takenIds.has(`${base}-${suffix}`)) {
    suffix += 1;
  }
  return `${base}-${suffix}`;
}
