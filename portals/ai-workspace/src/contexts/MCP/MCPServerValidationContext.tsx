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
  useMemo,
  useState,
  useCallback,
} from 'react';
import type {
  MCPServerInfoFetchRequest,
  MCPServerInfoFetchResponse,
} from '../../utils/types';
import * as mcpServerValidationApis from '../../apis/MCP/mcpServerValidationApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import { logger } from '../../utils/logger';

// ============================================================================
// MCP Server Validation Context
// ============================================================================

type MCPServerValidationContextValue = {
  serverInfo: MCPServerInfoFetchResponse | null;
  isLoading: boolean;
  error: Error | null;
  fetchServerInfo: (request: MCPServerInfoFetchRequest) => Promise<MCPServerInfoFetchResponse>;
  reset: () => void;
};

const MCPServerValidationContext = createContext<MCPServerValidationContextValue>({
  serverInfo: null,
  isLoading: false,
  error: null,
  fetchServerInfo: async () => {
    throw new Error('MCPServerValidationContext not initialized');
  },
  reset: () => {},
});

interface MCPServerValidationProviderProps {
  children: React.ReactNode;
}

export function MCPServerValidationProvider({ children }: MCPServerValidationProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [serverInfo, setServerInfo] = useState<MCPServerInfoFetchResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  const fetchServerInfo = useCallback(
    async (request: MCPServerInfoFetchRequest): Promise<MCPServerInfoFetchResponse> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        setIsLoading(true);
        setError(null);
        const response = await mcpServerValidationApis.fetchMCPProxyServerInfo(
          request,
          organizationId,
          apimBaseUrl
        );
        setServerInfo(response);
        return response;
      } catch (err) {
        logger.error('Failed to fetch MCP server info:', err);
        const error = err instanceof Error ? err : new Error('Failed to fetch MCP server info');
        setError(error);
        throw err;
      } finally {
        setIsLoading(false);
      }
    },
    [organizationId, apimBaseUrl]
  );

  const reset = useCallback(() => {
    setServerInfo(null);
    setError(null);
    setIsLoading(false);
  }, []);

  const value = useMemo(
    () => ({
      serverInfo,
      isLoading,
      error,
      fetchServerInfo,
      reset,
    }),
    [serverInfo, isLoading, error, fetchServerInfo, reset]
  );

  return (
    <MCPServerValidationContext.Provider value={value}>
      {children}
    </MCPServerValidationContext.Provider>
  );
}

export function useMCPServerValidation(): MCPServerValidationContextValue {
  return useContext(MCPServerValidationContext);
}
