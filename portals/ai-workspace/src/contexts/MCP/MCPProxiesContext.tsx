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
  MCPServer,
  MCPServerListResponse,
  CreateMCPServerRequest,
  UpdateMCPServerRequest,
} from '../../utils/types';
import { mcpProxiesApis } from '../../apis/MCP/mcpProxiesApis';
import { useAppShell } from '../AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import { logger } from '../../utils/logger';

// ============================================================================
// MCP Servers List Context - For managing the list of all MCP servers
// ============================================================================

const EMPTY_MCP_SERVERS_RESPONSE: MCPServerListResponse = {
  count: 0,
  list: [],
  pagination: { total: 0, offset: 0, limit: 20 },
};

type MCPServersContextValue = {
  /** Full API response with count, list, and pagination */
  mcpServersResponse: MCPServerListResponse;
  isLoading: boolean;
  error: Error | null;
  createMCPServer: (mcpServer: CreateMCPServerRequest) => Promise<MCPServer>;
  updateMCPServer: (mcpServerId: string, updates: UpdateMCPServerRequest) => Promise<MCPServer>;
  deleteMCPServer: (mcpServerId: string) => Promise<void>;
  refreshMCPServers: () => Promise<void>;
  getMCPServerById: (mcpServerId: string) => MCPServer | undefined;
};

const MCPServersContext = createContext<MCPServersContextValue>({
  mcpServersResponse: EMPTY_MCP_SERVERS_RESPONSE,
  isLoading: false,
  error: null,
  createMCPServer: async () => {
    throw new Error('MCPServersContext not initialized');
  },
  updateMCPServer: async () => {
    throw new Error('MCPServersContext not initialized');
  },
  deleteMCPServer: async () => {
    throw new Error('MCPServersContext not initialized');
  },
  refreshMCPServers: async () => {
    throw new Error('MCPServersContext not initialized');
  },
  getMCPServerById: () => undefined,
});

interface MCPServersProviderProps {
  children: React.ReactNode;
}

export function MCPServersProvider({ children }: MCPServersProviderProps) {
  const { currentOrganization, currentProject } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const [mcpServersResponse, setMCPServersResponse] = useState<MCPServerListResponse>(
    EMPTY_MCP_SERVERS_RESPONSE
  );
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';
  const projectId = currentProject?.id ?? '';

  // Fetch all MCP servers
  const fetchMCPServers = useCallback(async () => {
    if (!organizationId || !projectId) {
      setMCPServersResponse(EMPTY_MCP_SERVERS_RESPONSE);
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      setError(null);
      const response = await mcpProxiesApis.getMCPServers(
        projectId,
        apimBaseUrl
      );
      setMCPServersResponse(response as MCPServerListResponse);
    } catch (err) {
      logger.error('Failed to fetch MCP servers:', err);
      setError(
        err instanceof Error ? err : new Error('Failed to fetch MCP servers')
      );
    } finally {
      setIsLoading(false);
    }
  }, [organizationId, projectId, apimBaseUrl]);

  useEffect(() => {
    fetchMCPServers();
  }, [fetchMCPServers]);

  const createMCPServer = useCallback(
    async (mcpServer: CreateMCPServerRequest): Promise<MCPServer> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        const newMCPServer = await mcpProxiesApis.createMCPServer(
          mcpServer,
          apimBaseUrl
        );
        setMCPServersResponse((prev) => ({
          ...prev,
          count: prev.count + 1,
          list: [newMCPServer, ...prev.list],
          pagination: { ...prev.pagination, total: prev.pagination.total + 1 },
        }));
        return newMCPServer;
      } catch (err) {
        logger.error('Failed to create MCP server:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const updateMCPServer = useCallback(
    async (mcpServerId: string, updates: UpdateMCPServerRequest): Promise<MCPServer> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        const updatedMCPServer = await mcpProxiesApis.updateMCPServer(
          mcpServerId,
          updates,
          apimBaseUrl
        );
        setMCPServersResponse((prev) => ({
          ...prev,
          list: prev.list.map((mcpServer) =>
            mcpServer.id === mcpServerId ? updatedMCPServer : mcpServer
          ),
        }));
        return updatedMCPServer;
      } catch (err) {
        logger.error('Failed to update MCP server:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const deleteMCPServer = useCallback(
    async (mcpServerId: string): Promise<void> => {
      if (!organizationId) {
        throw new Error('Organization ID is missing');
      }
      try {
        await mcpProxiesApis.deleteMCPServer(mcpServerId, apimBaseUrl);
        setMCPServersResponse((prev) => ({
          ...prev,
          count: Math.max(0, prev.count - 1),
          list: prev.list.filter((mcpServer) => mcpServer.id !== mcpServerId),
          pagination: {
            ...prev.pagination,
            total: Math.max(0, prev.pagination.total - 1),
          },
        }));
      } catch (err) {
        logger.error('Failed to delete MCP server:', err);
        throw err;
      }
    },
    [organizationId, apimBaseUrl]
  );

  const refreshMCPServers = useCallback(async (): Promise<void> => {
    await fetchMCPServers();
  }, [fetchMCPServers]);

  const getMCPServerById = useCallback(
    (mcpServerId: string): MCPServer | undefined => {
      return mcpServersResponse.list.find((mcpServer) => mcpServer.id === mcpServerId);
    },
    [mcpServersResponse.list]
  );

  const value = useMemo(
    () => ({
      mcpServersResponse,
      isLoading,
      error,
      createMCPServer,
      updateMCPServer,
      deleteMCPServer,
      refreshMCPServers,
      getMCPServerById,
    }),
    [
      mcpServersResponse,
      isLoading,
      error,
      createMCPServer,
      updateMCPServer,
      deleteMCPServer,
      refreshMCPServers,
      getMCPServerById,
    ]
  );

  return (
    <MCPServersContext.Provider value={value}>{children}</MCPServersContext.Provider>
  );
}

export function useMCPServers(): MCPServersContextValue {
  return useContext(MCPServersContext);
}
