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
import { BFF_API_BASE_URL } from '../../paths';

import type {
  MCPServer,
  MCPServerListResponse,
  CreateMCPServerRequest,
  UpdateMCPServerRequest,
  Publication,
  PublicationDraftDetails,
  PublicationDraftDetailsInput,
} from '../../utils/types';

// ============================================================================
// MCP Server API Functions
// ============================================================================

function normalizeLegacyAuthType<T extends CreateMCPServerRequest | UpdateMCPServerRequest>(
  payload: T
): T {
  const auth = payload.upstream?.main?.auth;
  if (auth?.type !== 'header') return payload;

  return {
    ...payload,
    upstream: {
      ...payload.upstream,
      main: {
        ...payload.upstream!.main,
        auth: { ...auth, type: 'api-key' },
      },
    },
  } as T;
}

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
    // Routed through the BFF composite endpoint so the BFF can compensate by
    // deleting the pre-created secret if the MCP server creation fails.
    const response = await post<MCPServer>(
      '/mcp-proxies',
      normalizeLegacyAuthType(mcpServer),
      BFF_API_BASE_URL
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
      normalizeLegacyAuthType(updates),
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

const MCP_PROXY_API_TYPE = 'mcp-proxy';

/** Base path for every publication-related call on one (MCP proxy, portal) pairing. */
function mcpProxyPortalPath(apiPortalId: string, mcpProxyId: string, suffix: string): string {
  return `/api-portals/${encodeURIComponent(apiPortalId)}/apis/${MCP_PROXY_API_TYPE}/${encodeURIComponent(mcpProxyId)}${suffix}`;
}

/**
 * Get the live listing for this MCP proxy on the given API Portal.
 *
 * Throws on a 404 (no publication record — i.e. not currently published);
 * callers should treat that specifically as "unpublished", any other error
 * as a genuine failure.
 */
export async function getMcpProxyApiPortalPublication(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Publication> {
  return get<Publication>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/publication'), undefined, baseUrl);
}

/**
 * Get the saved publication draft for this MCP proxy on the given API Portal.
 *
 * Throws on a 404 (no draft saved yet); callers publishing for the first time
 * should treat that specifically as "no existing draft to merge with".
 */
export async function getMcpProxyApiPortalDraft(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<PublicationDraftDetails> {
  return get<PublicationDraftDetails>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft'), undefined, baseUrl);
}

/**
 * Save the publication draft and publish it, in one call.
 *
 * Handled by the BFF, which does the two Platform-API calls the contract needs
 * — PUT .../draft with `draft`, then the body-less POST .../publish — so a
 * saved draft is never left unpublished because the tab went away in between.
 * A rejected draft is relayed back and publish never runs.
 *
 * Returns the resulting listing — 201 on a first publish or a re-publish after
 * unpublish, 200 otherwise; both carry the same `Publication` body.
 */
export async function publishMcpProxyToApiPortal(
  apiPortalId: string,
  mcpProxyId: string,
  draft: PublicationDraftDetailsInput
): Promise<Publication> {
  return post<Publication>(
    `/api-portals/${encodeURIComponent(apiPortalId)}/mcp-proxies/${encodeURIComponent(mcpProxyId)}/publish`,
    draft,
    BFF_API_BASE_URL
  );
}

/**
 * Remove the listing (and its subscriptions and API keys) from the API Portal.
 *
 * A draft always survives: the publication is demoted into a draft when none
 * exists, otherwise the existing draft is kept. Valid only when currently
 * published or deprecated; responds 204 with no body.
 */
export async function unpublishMcpProxyFromApiPortal(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<void> {
  await post<void>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/unpublish'), undefined, baseUrl);
}

export const mcpProxiesApis = {
  createMCPServer,
  getMCPServers,
  getMCPServer,
  updateMCPServer,
  deleteMCPServer,
  getMcpProxyApiPortalPublication,
  getMcpProxyApiPortalDraft,
  publishMcpProxyToApiPortal,
  unpublishMcpProxyFromApiPortal,
};
