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
import type { MCPServer, UpdateMCPServerRequest } from '../../utils/types';
import { mcpProxiesApis } from '../../apis/MCP/mcpProxiesApis';
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

// ============================================================================
// Single MCP Server Context - For managing a single MCP server by ID
// ============================================================================

type MCPServerContextValue = {
  mcpServer: MCPServer | null;
  isLoading: boolean;
  error: Error | null;
  /** Update a field locally without calling the API */
  setLocalMCPServer: React.Dispatch<React.SetStateAction<MCPServer | null>>;
  updateMCPServer: (updates: UpdateMCPServerRequest) => Promise<MCPServer>;
  deleteMCPServer: () => Promise<void>;
  refetch: () => Promise<void>;
};

const MCPServerContext = createContext<MCPServerContextValue>({
  mcpServer: null,
  isLoading: false,
  error: null,
  setLocalMCPServer: () => {},
  updateMCPServer: async () => {
    throw new Error('MCPServerContext not initialized');
  },
  deleteMCPServer: async () => {
    throw new Error('MCPServerContext not initialized');
  },
  refetch: async () => {
    throw new Error('MCPServerContext not initialized');
  },
});

interface MCPServerProviderProps {
  children: React.ReactNode;
  mcpServerId: string;
}

export function MCPServerProvider({ children, mcpServerId }: MCPServerProviderProps) {
  const { currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [mcpServer, setMCPServer] = useState<MCPServer | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  // Fetch single MCP server
  const fetchMCPServer = useCallback(async () => {
    if (!mcpServerId || !organizationId) {
      setMCPServer(null);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const fetchedMCPServer = await mcpProxiesApis.getMCPServer(mcpServerId, apimBaseUrl);
      setMCPServer(fetchedMCPServer);
    } catch (err) {
      logger.error(`Failed to fetch MCP server ${mcpServerId}:`, err);
      setError(err instanceof Error ? err : new Error('Failed to fetch MCP server'));
      setMCPServer(null);
    } finally {
      setIsLoading(false);
    }
  }, [mcpServerId, organizationId, apimBaseUrl]);

  useEffect(() => {
    fetchMCPServer();
  }, [fetchMCPServer]);

  const updateMCPServer = useCallback(async (updates: UpdateMCPServerRequest): Promise<MCPServer> => {
    if (!mcpServerId || !organizationId) {
      throw new Error('MCP Server ID or Organization ID is missing');
    }
    try {
      // If the upstream auth value is a new plain-text credential (not already a
      // placeholder), create a new secret and substitute the placeholder before
      // persisting. After a successful update the old secret is deleted best-effort.
      let updatesPayload = updates;
      const authValue = updates.upstream?.main?.auth?.value;
      const isAlreadyPlaceholder =
        typeof authValue === 'string' && authValue.includes('{{ secret ');

      if (authValue && !isAlreadyPlaceholder) {
        const secretHandle = generateSecretHandle();
        const secretResponse = await createSecret({
          id: secretHandle,
          displayName: `${mcpServerId} upstream auth`,
          description: `Auto-generated secret for MCP server ${mcpServerId}`,
          value: authValue,
          type: 'GENERIC',
        });
        logger.info('Created new secret for MCP server update', { secretHandle, mcpServerId });

        updatesPayload = {
          ...updates,
          upstream: {
            ...updates.upstream,
            main: {
              ...updates.upstream?.main,
              url: updates.upstream?.main?.url ?? mcpServer?.upstream?.main?.url ?? '',
              auth: {
                ...updates.upstream?.main?.auth,
                value: buildSecretPlaceholder(secretResponse.id),
              },
            },
          },
        };
      }

      const updatedMCPServer = await mcpProxiesApis.updateMCPServer(mcpServerId, updatesPayload, apimBaseUrl);
      setMCPServer(updatedMCPServer);

      // Best-effort: delete the old secret after the update succeeds.
      if (authValue && !isAlreadyPlaceholder) {
        const oldHandle = mcpServer?.upstream?.main?.auth?.value
          ? extractSecretHandle(mcpServer.upstream.main.auth.value)
          : null;
        if (oldHandle) {
          deleteSecret(oldHandle).catch((err) => {
            logger.warn('Could not delete old secret after MCP server update', { oldHandle, err });
          });
        }
      }

      return updatedMCPServer;
    } catch (err) {
      logger.error('Failed to update MCP server:', err);
      throw err;
    }
  }, [mcpServerId, organizationId, apimBaseUrl, mcpServer]);

  const deleteMCPServer = useCallback(async (): Promise<void> => {
    if (!mcpServerId || !organizationId) {
      throw new Error('MCP Server ID or Organization ID is missing');
    }
    try {
      await mcpProxiesApis.deleteMCPServer(mcpServerId, apimBaseUrl);
      setMCPServer(null);
    } catch (err) {
      logger.error('Failed to delete MCP server:', err);
      throw err;
    }
  }, [mcpServerId, organizationId, apimBaseUrl]);

  const refetch = useCallback(async (): Promise<void> => {
    await fetchMCPServer();
  }, [fetchMCPServer]);

  const value = useMemo(
    () => ({
      mcpServer,
      isLoading,
      error,
      setLocalMCPServer: setMCPServer,
      updateMCPServer,
      deleteMCPServer,
      refetch,
    }),
    [mcpServer, isLoading, error, updateMCPServer, deleteMCPServer, refetch]
  );

  return (
    <MCPServerContext.Provider value={value}>
      {children}
    </MCPServerContext.Provider>
  );
}

export function useMCPServer(): MCPServerContextValue {
  return useContext(MCPServerContext);
}
