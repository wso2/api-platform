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

import { get, post, put, del } from '../../clients/choreoApiClient';
import { logger } from '../../utils/logger';
import { BFF_COMPOSITE_BASE_URL } from '../../config.env';

import type {
  MCPServer,
  MCPServerListResponse,
  CreateMCPServerRequest,
  UpdateMCPServerRequest,
} from '../../utils/types';

// ============================================================================
// MCP Server API Functions
// ============================================================================

/**
 * Create a new MCP Server
 *
 * @param mcpServer - The MCP server details
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the created MCP server
 */
export async function createMCPServer(
  mcpServer: CreateMCPServerRequest,
  organizationId: string,
  _baseUrl: string
): Promise<MCPServer> {
  try {
    // Routed through the BFF composite endpoint so the BFF can compensate by
    // deleting the pre-created secret if the MCP server creation fails.
    const response = await post<MCPServer>(
      `/mcp-proxies?organizationId=${encodeURIComponent(organizationId)}`,
      mcpServer,
      BFF_COMPOSITE_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to create MCP server:', error);
    throw error;
  }
}

/**
 * Get all MCP Servers
 *
 * @param organizationId - The organization ID
 * @param projectId - The project ID
 * @param baseUrl - The APIM base URL
 * @param limit - Maximum number of MCP servers to return
 * @param offset - Number of MCP servers to skip
 * @returns Promise with the list of MCP servers
 */
export async function getMCPServers(
  organizationId: string,
  projectId: string,
  baseUrl: string,
  limit: number = 20,
  offset: number = 0
): Promise<MCPServerListResponse> {
  try {
    const response = await get<MCPServerListResponse>(
      `/mcp-proxies?organizationId=${encodeURIComponent(organizationId)}&projectId=${encodeURIComponent(projectId)}&limit=${limit}&offset=${offset}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch MCP servers:', error);
    throw error;
  }
}

/**
 * Get a single MCP Server by ID
 *
 * @param mcpServerId - The MCP server ID
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the MCP server details
 */
export async function getMCPServer(
  mcpServerId: string,
  organizationId: string,
  baseUrl: string
): Promise<MCPServer> {
  try {
    const response = await get<MCPServer>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch MCP server ${mcpServerId}:`, error);
    throw error;
  }
}

/**
 * Update an existing MCP Server
 *
 * @param mcpServerId - The MCP server ID
 * @param updates - The fields to update
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the updated MCP server
 */
export async function updateMCPServer(
  mcpServerId: string,
  updates: UpdateMCPServerRequest,
  organizationId: string,
  baseUrl: string
): Promise<MCPServer> {
  try {
    const response = await put<MCPServer>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}?organizationId=${encodeURIComponent(organizationId)}`,
      updates,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update MCP server ${mcpServerId}:`, error);
    throw error;
  }
}

/**
 * Delete an MCP Server
 *
 * @param mcpServerId - The MCP server ID
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise that resolves when the MCP server is deleted
 */
export async function deleteMCPServer(
  mcpServerId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete MCP server ${mcpServerId}:`, error);
    throw error;
  }
}

export const mcpProxiesApis = {
  createMCPServer,
  getMCPServers,
  getMCPServer,
  updateMCPServer,
  deleteMCPServer,
};
