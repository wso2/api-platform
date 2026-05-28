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
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useAuthContext } from '@asgardeo/auth-react';
import { API_BASE_URLS } from '../config.env';
import type { MoesifTokenResponse } from '../utils/types';
import { logger } from '../utils/logger';
import { useAppShell } from './AppShellContext';
import { useEnvironments } from '../hooks/useEnvironments';
import { registerMoesifOrganization } from '../apis/moesifApis';

// ============================================================================
// Moesif Context - For managing Moesif ID token
// ============================================================================

type MoesifContextValue = {
  moesifToken: MoesifTokenResponse | null;
  isLoading: boolean;
  error: Error | null;
  fetchMoesifToken: () => Promise<MoesifTokenResponse>;
  clearMoesifToken: () => void;
};

const MoesifContext = createContext<MoesifContextValue>({
  moesifToken: null,
  isLoading: false,
  error: null,
  fetchMoesifToken: async () => {
    throw new Error('MoesifContext not initialized');
  },
  clearMoesifToken: () => {
    throw new Error('MoesifContext not initialized');
  },
});

export function MoesifProvider({ children }: { children: React.ReactNode }) {
  const [moesifToken, setMoesifToken] = useState<MoesifTokenResponse | null>(
    null
  );
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const lastFetchedOrgIdRef = useRef<string | null>(null);
  const { httpRequest, getBasicUserInfo } = useAuthContext();
  const { isTokenExchanged, currentOrganization } = useAppShell();
  const { environments } = useEnvironments();

  const fetchMoesifToken =
    useCallback(async (): Promise<MoesifTokenResponse> => {
      try {
        logger.info('Fetching Moesif token...');
        setIsLoading(true);
        setError(null);
        const response = await httpRequest({
          url: `${API_BASE_URLS.moesifAPI}/id_token`,
          method: 'GET',
          headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
          },
        });
        const data = response?.data as MoesifTokenResponse;
        setMoesifToken(data);
        return data;
      } catch (err) {
        logger.error('Failed to fetch Moesif token:', err);
        const nextError =
          err instanceof Error
            ? err
            : new Error('Failed to fetch Moesif token');
        setError(nextError);
        throw nextError;
      } finally {
        setIsLoading(false);
      }
    }, [httpRequest]);

  const clearMoesifToken = useCallback(() => {
    setMoesifToken(null);
    setError(null);
  }, []);

  // Refetch Moesif token whenever org context changes and token exchange completes.
  // registerMoesifOrganization must succeed with apps before fetching the token.
  useEffect(() => {
    const orgId = currentOrganization?.id ? String(currentOrganization.id) : '';

    if (!orgId) {
      lastFetchedOrgIdRef.current = null;
      clearMoesifToken();
      return;
    }

    if (!isTokenExchanged) {
      lastFetchedOrgIdRef.current = null;
      clearMoesifToken();
      return;
    }

    if (!environments.length) return;

    if (lastFetchedOrgIdRef.current === orgId) return;

    lastFetchedOrgIdRef.current = orgId;

    (async () => {
      try {
        const basicUser = await getBasicUserInfo();
        const userName =
          (basicUser as { displayName?: string; username?: string })
            ?.displayName ||
          (basicUser as { username?: string })?.username ||
          'User';
        const moesifApps = environments
          .filter((env) => env.id && env.name)
          .map((env) => ({ name: env.name }));
        const registerResponse = await registerMoesifOrganization(
          userName,
          moesifApps
        );
        if (!registerResponse?.apps?.length) {
          // Organization has no apps — skip token fetch
          return;
        }
        await fetchMoesifToken();
      } catch {
        lastFetchedOrgIdRef.current = null;
        // Error state is already set in fetchMoesifToken
      }
    })();
  }, [
    currentOrganization?.id,
    isTokenExchanged,
    environments,
    fetchMoesifToken,
    clearMoesifToken,
    getBasicUserInfo,
  ]);

  const value = useMemo(
    () => ({
      moesifToken,
      isLoading,
      error,
      fetchMoesifToken,
      clearMoesifToken,
    }),
    [moesifToken, isLoading, error, fetchMoesifToken, clearMoesifToken]
  );

  return (
    <MoesifContext.Provider value={value}>{children}</MoesifContext.Provider>
  );
}

export function useMoesif(): MoesifContextValue {
  return useContext(MoesifContext);
}
