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
  APIKeyRevokeRequest,
  UserAPIKey,
  UserAPIKeyListResponse,
} from '../utils/types';
import { keyManagementApis } from '../apis/keyManagementApis';
import { useAppShell } from './AppShellContext';
import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';

// ============================================================================
// Key Management Context
// ============================================================================

const EMPTY_USER_API_KEYS_RESPONSE: UserAPIKeyListResponse = {
  count: 0,
  items: [],
};

type KeyManagementContextValue = {
  /** Convenience accessor for the user API keys list */
  userAPIKeys: UserAPIKey[];
  /** Full API response with count and list */
  userAPIKeysResponse: UserAPIKeyListResponse;
  isLoading: boolean;
  error: Error | null;
  /** List user API keys, optionally filtered by artifact type */
  refreshUserAPIKeys: (type?: string) => Promise<void>;
  /** Revoke an API test token */
  revokeAPIKey: (
    apiId: string,
    request: APIKeyRevokeRequest
  ) => Promise<void>;
};

const KeyManagementContext = createContext<KeyManagementContextValue>({
  userAPIKeys: [],
  userAPIKeysResponse: EMPTY_USER_API_KEYS_RESPONSE,
  isLoading: false,
  error: null,
  refreshUserAPIKeys: async () => {
    throw new Error('KeyManagementContext not initialized');
  },
  revokeAPIKey: async () => {
    throw new Error('KeyManagementContext not initialized');
  },
});

interface KeyManagementProviderProps {
  children: React.ReactNode;
}

export function KeyManagementProvider({
  children,
}: KeyManagementProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [userAPIKeysResponse, setUserAPIKeysResponse] =
    useState<UserAPIKeyListResponse>(EMPTY_USER_API_KEYS_RESPONSE);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch user API keys
  const fetchUserAPIKeys = useCallback(
    async (type?: string) => {
      if (!organizationId) {
        setUserAPIKeysResponse(EMPTY_USER_API_KEYS_RESPONSE);
        setIsLoading(false);
        return;
      }

      try {
        setIsLoading(true);
        setError(null);
        const response = await keyManagementApis.listUserAPIKeys(
          organizationId,
          apimBaseUrl,
          type
        );
        setUserAPIKeysResponse(response);
      } catch (err) {
        logger.error('Failed to fetch user API keys:', err);
        setError(
          err instanceof Error
            ? err
            : new Error('Failed to fetch user API keys')
        );
      } finally {
        setIsLoading(false);
      }
    },
    [organizationId, apimBaseUrl]
  );

  useEffect(() => {
    fetchUserAPIKeys();
  }, [fetchUserAPIKeys]);

  const revokeAPIKey = useCallback(
    async (apiId: string, request: APIKeyRevokeRequest): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        await keyManagementApis.revokeAPIKey(
          apiId,
          request,
          organizationId,
          apimBaseUrl
        );
        // Refresh the list after revoking
        await fetchUserAPIKeys();
      } catch (err) {
        logger.error(`Failed to revoke API key for API ${apiId}:`, err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl, fetchUserAPIKeys]
  );

  const refreshUserAPIKeys = useCallback(
    async (type?: string): Promise<void> => {
      await fetchUserAPIKeys(type);
    },
    [fetchUserAPIKeys]
  );

  const value = useMemo(
    () => ({
      userAPIKeys: userAPIKeysResponse.items,
      userAPIKeysResponse,
      isLoading,
      error,
      refreshUserAPIKeys,
      revokeAPIKey,
    }),
    [
      userAPIKeysResponse,
      isLoading,
      error,
      refreshUserAPIKeys,
      revokeAPIKey,
    ]
  );

  return (
    <KeyManagementContext.Provider value={value}>
      {children}
    </KeyManagementContext.Provider>
  );
}

export function useKeyManagement(): KeyManagementContextValue {
  return useContext(KeyManagementContext);
}
