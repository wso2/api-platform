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
// Agent Proxy Deployment API Functions
// ============================================================================

/**
 * Deploy an Agent proxy to a gateway
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param agentProxyId - The ID of the Agent proxy
 * @param request - The deployment request containing gatewayId, name, base, and metadata
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function deployAgentProxy(
  agentProxyId: string,
  request: DeployRequest,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await post<DeploymentResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/deployments`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to deploy Agent proxy ${agentProxyId}:`, error);
    throw error;
  }
}

/**
 * Get deployments for an Agent proxy
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param agentProxyId - The ID of the Agent proxy
 * @param baseUrl - The base URL for the API
 * @param gatewayId - Optional gateway ID to filter deployments
 * @param status - Optional status to filter deployments (DEPLOYED, UNDEPLOYED, ARCHIVED)
 * @returns Promise with the deployment list response
 */
export async function getAgentProxyDeployments(
  agentProxyId: string,
  baseUrl: string,
  gatewayId?: string,
  status?: string
): Promise<DeploymentListResponse> {
  try {
    const params = new URLSearchParams();

    if (gatewayId) {
      params.append('gatewayId', gatewayId);
    }

    if (status) {
      params.append('status', status);
    }

    const query = params.toString();
    const response = await get<DeploymentListResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/deployments${query ? `?${query}` : ''}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get Agent proxy deployments for ${agentProxyId}:`, error);
    throw error;
  }
}

/**
 * Get a specific Agent proxy deployment by ID
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param agentProxyId - The ID of the Agent proxy
 * @param deploymentId - The ID of the deployment
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function getAgentProxyDeployment(
  agentProxyId: string,
  deploymentId: string,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await get<DeploymentResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/deployments/${encodeURIComponent(deploymentId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get Agent proxy deployment ${deploymentId}:`, error);
    throw error;
  }
}

/**
 * Delete an Agent proxy deployment
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param agentProxyId - The ID of the Agent proxy
 * @param deploymentId - The ID of the deployment to delete
 * @param baseUrl - The base URL for the API
 * @returns Promise that resolves when the deployment is deleted
 */
export async function deleteAgentProxyDeployment(
  agentProxyId: string,
  deploymentId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/deployments/${encodeURIComponent(deploymentId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(
      `Failed to delete Agent proxy deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

/**
 * Undeploy an Agent proxy deployment from a gateway
 *
 * Organization is resolved from the JWT token on the server side.
 * `gatewayId` is required by the spec — validated against the deployment's bound gateway.
 *
 * @param agentProxyId - The ID of the Agent proxy
 * @param deploymentId - The ID of the deployment to undeploy
 * @param baseUrl - The base URL for the API
 * @param gatewayId - Gateway ID for validation (required)
 * @returns Promise with the updated deployment response
 */
export async function undeployAgentProxyDeployment(
  agentProxyId: string,
  deploymentId: string,
  baseUrl: string,
  gatewayId: string
): Promise<DeploymentResponse> {
  try {
    const params = new URLSearchParams({ gatewayId });

    const response = await post<DeploymentResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/deployments/${encodeURIComponent(deploymentId)}/undeploy?${params.toString()}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to undeploy Agent proxy deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

/**
 * Restore a previous Agent proxy deployment
 *
 * Organization is resolved from the JWT token on the server side.
 * `gatewayId` is required by the spec — validated against the deployment's bound gateway.
 *
 * @param agentProxyId - The ID of the Agent proxy
 * @param deploymentId - The ID of the deployment to restore
 * @param baseUrl - The base URL for the API
 * @param gatewayId - Gateway ID for validation (required)
 * @returns Promise with the restored deployment response
 */
export async function restoreAgentProxyDeployment(
  agentProxyId: string,
  deploymentId: string,
  baseUrl: string,
  gatewayId: string
): Promise<DeploymentResponse> {
  try {
    const params = new URLSearchParams({ gatewayId });

    const response = await post<DeploymentResponse>(
      `/agent-proxies/${encodeURIComponent(agentProxyId)}/deployments/${encodeURIComponent(deploymentId)}/restore?${params.toString()}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to restore Agent proxy deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}
