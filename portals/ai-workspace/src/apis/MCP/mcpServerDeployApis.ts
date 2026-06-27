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

import { get, post, del } from '../../clients/choreoApiClient';
import { logger } from '../../utils/logger';
import type {
  DeploymentListResponse,
  DeploymentResponse,
  DeployRequest,
} from '../../utils/types';

// ============================================================================
// MCP Server Deployment API Functions
// ============================================================================

/**
 * Deploy an MCP server to a gateway
 * @param mcpServerId - The ID of the MCP server
 * @param organizationId - The organization ID
 * @param request - The deployment request containing gatewayId, name, base, and metadata
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function deployMCPServer(
  mcpServerId: string,
  organizationId: string,
  request: DeployRequest,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await post<DeploymentResponse>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}/deployments?organizationId=${encodeURIComponent(organizationId)}`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to deploy MCP server ${mcpServerId}:`, error);
    throw error;
  }
}

/**
 * Get deployments for an MCP server
 * @param mcpServerId - The ID of the MCP server
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @param gatewayId - Optional gateway ID to filter deployments
 * @param status - Optional status to filter deployments (DEPLOYED, UNDEPLOYED, ARCHIVED)
 * @returns Promise with the deployment list response
 */
export async function getMCPServerDeployments(
  mcpServerId: string,
  organizationId: string,
  baseUrl: string,
  gatewayId?: string,
  status?: string
): Promise<DeploymentListResponse> {
  try {
    const params = new URLSearchParams({
      organizationId: organizationId,
    });

    if (gatewayId) {
      params.append('gatewayId', gatewayId);
    }

    if (status) {
      params.append('status', status);
    }

    const response = await get<DeploymentListResponse>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}/deployments?${params.toString()}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get MCP server deployments for ${mcpServerId}:`, error);
    throw error;
  }
}

/**
 * Get a specific MCP server deployment by ID
 * @param mcpServerId - The ID of the MCP server
 * @param deploymentId - The ID of the deployment
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function getMCPServerDeployment(
  mcpServerId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await get<DeploymentResponse>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}/deployments/${encodeURIComponent(deploymentId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get MCP server deployment ${deploymentId}:`, error);
    throw error;
  }
}

/**
 * Delete an MCP server deployment
 * @param mcpServerId - The ID of the MCP server
 * @param deploymentId - The ID of the deployment to delete
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise that resolves when the deployment is deleted
 */
export async function deleteMCPServerDeployment(
  mcpServerId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}/deployments/${encodeURIComponent(deploymentId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(
      `Failed to delete MCP server deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

/**
 * Undeploy an MCP server deployment from a gateway
 * @param mcpServerId - The ID of the MCP server
 * @param deploymentId - The ID of the deployment to undeploy
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @param gatewayId - Gateway ID for validation
 * @returns Promise with the updated deployment response
 */
export async function undeployMCPServerDeployment(
  mcpServerId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string,
  gatewayId?: string
): Promise<DeploymentResponse> {
  try {
    const params = new URLSearchParams({ organizationId });
    if (gatewayId) {
      params.append('gatewayId', gatewayId);
    }

    const response = await post<DeploymentResponse>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}/deployments/${encodeURIComponent(deploymentId)}/undeploy?${params.toString()}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to undeploy MCP server deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

/**
 * Restore a previous MCP server deployment
 * @param mcpServerId - The ID of the MCP server
 * @param deploymentId - The ID of the deployment to restore
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @param gatewayId - Gateway ID for validation
 * @returns Promise with the restored deployment response
 */
export async function restoreMCPServerDeployment(
  mcpServerId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string,
  gatewayId?: string
): Promise<DeploymentResponse> {
  try {
    const params = new URLSearchParams({ organizationId });
    if (gatewayId) {
      params.append('gatewayId', gatewayId);
    }

    const response = await post<DeploymentResponse>(
      `/mcp-proxies/${encodeURIComponent(mcpServerId)}/deployments/${encodeURIComponent(deploymentId)}/restore?${params.toString()}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to restore MCP server deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}
