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
 * KIND, either express or implied, See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { get, post, put, del } from '../../clients/choreoApiClient';
import { logger } from '../../utils/logger';

import type {
  AgentProxy,
  AgentProxyListResponse,
  CreateAgentProxyRequest,
  UpdateAgentProxyRequest,
  FetchAgentCardRequest,
  AgentCardDocument,
  APIKeyListResponse,
  CreateAgentProxyAPIKeyRequest,
  CreateAgentProxyAPIKeyResponse,
} from '../../utils/types';

// ============================================================================
// Agent Proxy API Functions
// ============================================================================

/**
 * Create a new Agent Proxy
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param agentProxy - The Agent proxy details
 * @param baseUrl - The APIM base URL
 * @returns Promise with the created Agent proxy
 */
export async function createAgentProxy(
  agentProxy: CreateAgentProxyRequest,
  baseUrl: string
): Promise<AgentProxy> {
  try {
    const response = await post<AgentProxy>('/agent-proxies', agentProxy, baseUrl);
    return response;
  } catch (error) {
    logger.error('Failed to create Agent proxy:', error);
    throw error;
  }
}

/**
 * Get all Agent Proxies
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param projectId - The project ID
 * @param baseUrl - The APIM base URL
 * @param limit - Maximum number of Agent proxies to return
 * @param offset - Number of Agent proxies to skip
 * @returns Promise with the list of Agent proxies
 */
export async function getAgentProxies(
  projectId: string,
  baseUrl: string,
  limit: number = 20,
  offset: number = 0
): Promise<AgentProxyListResponse> {
  try {
    const response = await get<AgentProxyListResponse>(
      `/agent-proxies?projectId=${encodeURIComponent(projectId)}&limit=${limit}&offset=${offset}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch Agent proxies:', error);
    throw error;
  }
}

/**
 * Get a single Agent Proxy by ID
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param agentProxyId - The Agent proxy handle
 * @param baseUrl - The APIM base URL
 * @returns Promise with the Agent proxy details
 */
export async function getAgentProxy(
  agentProxyId: string,
  baseUrl: string
): Promise<AgentProxy> {
  try {
    const response = await get<AgentProxy>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch Agent proxy ${agentProxyId}:`, error);
    throw error;
  }
}

/**
 * Update an existing Agent Proxy
 *
 * Full-resource replace: an omitted associatedGateways empties the association set.
 *
 * @param agentProxyId - The Agent proxy handle
 * @param agentProxy - The full Agent proxy definition
 * @param baseUrl - The APIM base URL
 * @returns Promise with the updated Agent proxy
 */
export async function updateAgentProxy(
  agentProxyId: string,
  agentProxy: UpdateAgentProxyRequest,
  baseUrl: string
): Promise<AgentProxy> {
  try {
    const response = await put<AgentProxy>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}`,
      agentProxy,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update Agent proxy ${agentProxyId}:`, error);
    throw error;
  }
}

/**
 * Delete an Agent Proxy
 *
 * @param agentProxyId - The Agent proxy handle
 * @param baseUrl - The APIM base URL
 */
export async function deleteAgentProxy(
  agentProxyId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete Agent proxy ${agentProxyId}:`, error);
    throw error;
  }
}

/**
 * Fetch an Agent Card for discovery
 *
 * Either a direct url, or an agentProxyId that uses the stored endpoint and
 * credentials. The parsed card is returned but never stored.
 *
 * @param cardRequest - The discovery request
 * @param baseUrl - The APIM base URL
 * @returns Promise with the parsed Agent Card
 */
export async function fetchAgentCard(
  cardRequest: FetchAgentCardRequest,
  baseUrl: string
): Promise<AgentCardDocument> {
  try {
    const response = await post<AgentCardDocument>(
      '/agent-proxies/fetch-agent-card',
      cardRequest,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch Agent Card:', error);
    throw error;
  }
}

/**
 * List API keys of an Agent Proxy
 *
 * Key material is never returned by this operation.
 *
 * @param agentProxyId - The Agent proxy handle
 * @param baseUrl - The APIM base URL
 * @returns Promise with the list of API keys
 */
export async function getAgentProxyAPIKeys(
  agentProxyId: string,
  baseUrl: string
): Promise<APIKeyListResponse> {
  try {
    const response = await get<APIKeyListResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/api-keys`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to fetch API keys for Agent proxy ${agentProxyId}:`,
      error
    );
    throw error;
  }
}

/**
 * Create an API key for an Agent Proxy
 *
 * The secret is returned only in this response and cannot be read back later.
 *
 * @param agentProxyId - The Agent proxy handle
 * @param request - The API key creation request
 * @param baseUrl - The APIM base URL
 * @returns Promise with the created API key
 */
export async function createAgentProxyAPIKey(
  agentProxyId: string,
  request: CreateAgentProxyAPIKeyRequest,
  baseUrl: string
): Promise<CreateAgentProxyAPIKeyResponse> {
  try {
    const response = await post<CreateAgentProxyAPIKeyResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/api-keys`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to create API key for Agent proxy ${agentProxyId}:`,
      error
    );
    throw error;
  }
}

/**
 * Revoke an API key of an Agent Proxy
 *
 * @param agentProxyId - The Agent proxy handle
 * @param apiKeyId - The API key ID
 * @param baseUrl - The APIM base URL
 */
export async function revokeAgentProxyAPIKey(
  agentProxyId: string,
  apiKeyId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/api-keys/${encodeURIComponent(apiKeyId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(
      `Failed to revoke API key ${apiKeyId} for Agent proxy ${agentProxyId}:`,
      error
    );
    throw error;
  }
}

export const agentProxiesApis = {
  createAgentProxy,
  getAgentProxies,
  getAgentProxy,
  updateAgentProxy,
  deleteAgentProxy,
  fetchAgentCard,
  getAgentProxyAPIKeys,
  createAgentProxyAPIKey,
  revokeAgentProxyAPIKey,
};
