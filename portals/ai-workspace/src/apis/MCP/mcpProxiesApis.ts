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
 * Organization is resolved from the JWT token on the server side.
 *
 * @param mcpServer - The MCP server details
 * @param baseUrl - The APIM base URL
 * @returns Promise with the created MCP server
 */
export async function createMCPServer(
  mcpServer: CreateMCPServerRequest,
  baseUrl: string
): Promise<MCPServer> {
  try {
    const response = await post<MCPServer>(
      '/mcp-proxies',
      mcpServer,
      baseUrl
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
 * Organization is resolved from the JWT token on the server side.
 *
 * @param projectId - The project ID
 * @param baseUrl - The APIM base URL
 * @param limit - Maximum number of MCP servers to return
 * @param offset - Number of MCP servers to skip
 * @returns Promise with the list of MCP servers
 */
export async function getMCPServers(
  projectId: string,
  baseUrl: string,
  limit: number = 20,
  offset: number = 0
): Promise<MCPServerListResponse> {
  try {
    const response = await get<MCPServerListResponse>(
      `/mcp-proxies?projectId=${encodeURIComponent(projectId)}&limit=${limit}&offset=${offset}`,
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
 * Organization is resolved from the JWT token on the server side.
 *
 * @param mcpServerId - The MCP server ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the MCP server details
 */
export async function getMCPServer(
  mcpServerId: string,
  baseUrl: string
): Promise<MCPServer> {
  try {
    const response = await get<MCPServer>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}`,
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
 * Organization is resolved from the JWT token on the server side.
 *
 * @param mcpServerId - The MCP server ID
 * @param updates - The fields to update
 * @param baseUrl - The APIM base URL
 * @returns Promise with the updated MCP server
 */
export async function updateMCPServer(
  mcpServerId: string,
  updates: UpdateMCPServerRequest,
  baseUrl: string
): Promise<MCPServer> {
  try {
    const response = await put<MCPServer>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}`,
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
 * Organization is resolved from the JWT token on the server side.
 *
 * @param mcpServerId - The MCP server ID
 * @param baseUrl - The APIM base URL
 * @returns Promise that resolves when the MCP server is deleted
 */
export async function deleteMCPServer(
  mcpServerId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}`,
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
