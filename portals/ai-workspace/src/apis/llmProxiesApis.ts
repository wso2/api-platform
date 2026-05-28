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

import { get, post, del } from '../clients/choreoApiClient';
import { logger } from '../utils/logger';
import { PLATFORM_API_BASE_URL } from '../config.env';
import type {
  DeploymentListResponse,
  DeploymentResponse,
  DeployRequest,
  CreateLLMProxyAPIKeyRequest,
  CreateLLMProxyAPIKeyResponse,
  APIKeyListResponse,
} from '../utils/types';

// ============================================================================
// LLM Proxy Deployment API Functions
// ============================================================================

/**
 * Deploy an LLM proxy to a gateway
 * @param proxyId - The ID of the LLM proxy
 * @param organizationId - The organization ID
 * @param request - The deployment request containing gatewayId, name, base, and metadata
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function deployLLMProxy(
  proxyId: string,
  organizationId: string,
  request: DeployRequest,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await post<DeploymentResponse>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/deployments?organizationId=${encodeURIComponent(organizationId)}`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to deploy LLM proxy ${proxyId}:`, error);
    throw error;
  }
}

/**
 * Get deployments for an LLM proxy
 * @param proxyId - The ID of the LLM proxy
 * @param organizationId - The organization ID
 * @param gatewayId - Optional gateway ID to filter deployments
 * @param status - Optional status to filter deployments (DEPLOYED, UNDEPLOYED, ARCHIVED)
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment list response
 */
export async function getLLMProxyDeployments(
  proxyId: string,
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
      `/llm-proxies/${encodeURIComponent(proxyId)}/deployments?${params.toString()}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get LLM proxy deployments for ${proxyId}:`, error);
    throw error;
  }
}

/**
 * Get a specific LLM proxy deployment by ID
 * @param proxyId - The ID of the LLM proxy
 * @param deploymentId - The ID of the deployment
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function getLLMProxyDeployment(
  proxyId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await get<DeploymentResponse>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/deployments/${encodeURIComponent(deploymentId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get LLM proxy deployment ${deploymentId}:`, error);
    throw error;
  }
}

/**
 * Delete an LLM proxy deployment
 * @param proxyId - The ID of the LLM proxy
 * @param deploymentId - The ID of the deployment to delete
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise that resolves when the deployment is deleted
 */
export async function deleteLLMProxyDeployment(
  proxyId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del(
      `/llm-proxies/${encodeURIComponent(proxyId)}/deployments/${encodeURIComponent(deploymentId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(
      `Failed to delete LLM proxy deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

/**
 * Undeploy an LLM proxy deployment from a gateway
 * @param proxyId - The ID of the LLM proxy
 * @param deploymentId - The ID of the deployment to undeploy
 * @param organizationId - The organization ID
 * @param gatewayId - Optional gateway ID for validation
 * @param baseUrl - The base URL for the API
 * @returns Promise with the updated deployment response
 */
export async function undeployLLMProxyDeployment(
  proxyId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string,
  gatewayId?: string
): Promise<DeploymentResponse> {
  try {
    const params = new URLSearchParams({
      organizationId: organizationId,
      deploymentId: deploymentId,
    });

    if (gatewayId) {
      params.append('gatewayId', gatewayId);
    }

    const response = await post<DeploymentResponse>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/deployments/undeploy?${params.toString()}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to undeploy LLM proxy deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

/**
 * Restore a previous LLM proxy deployment
 * @param proxyId - The ID of the LLM proxy
 * @param deploymentId - The ID of the deployment to restore
 * @param organizationId - The organization ID
 * @param gatewayId - Optional gateway ID for validation
 * @param baseUrl - The base URL for the API
 * @returns Promise with the restored deployment response
 */
export async function restoreLLMProxyDeployment(
  proxyId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string,
  gatewayId?: string
): Promise<DeploymentResponse> {
  try {
    const params = new URLSearchParams({
      organizationId: organizationId,
      deploymentId: deploymentId,
    });

    if (gatewayId) {
      params.append('gatewayId', gatewayId);
    }

    const response = await post<DeploymentResponse>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/deployments/restore?${params.toString()}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to restore LLM proxy deployment ${deploymentId}:`,
      error
    );
    throw error;
  }
}

// ============================================================================
// LLM Proxy API Key Functions
// ============================================================================

/**
 * Create a new API key for an LLM proxy
 * @param proxyId - The ID of the LLM proxy
 * @param organizationId - The organization ID
 * @param request - The API key creation request
 * @returns Promise with the API key response
 */
export async function createLLMProxyAPIKey(
  proxyId: string,
  organizationId: string,
  request: CreateLLMProxyAPIKeyRequest
): Promise<CreateLLMProxyAPIKeyResponse> {
  try {
    const response = await post<CreateLLMProxyAPIKeyResponse>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/api-keys?organizationId=${encodeURIComponent(organizationId)}`,
      request,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error(`Failed to create API key for LLM proxy ${proxyId}:`, error);
    throw error;
  }
}

/**
 * List all API keys for an LLM proxy
 * @param proxyId - The ID of the LLM proxy
 * @param organizationId - The organization ID
 * @returns Promise with the list of API keys
 */
export async function getLLMProxyAPIKeys(
  proxyId: string,
  organizationId: string
): Promise<APIKeyListResponse> {
  try {
    const response = await get<APIKeyListResponse>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/api-keys?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch API keys for LLM proxy ${proxyId}:`, error);
    throw error;
  }
}

/**
 * Delete an API key for an LLM proxy
 * @param proxyId - The ID of the LLM proxy
 * @param keyName - The name of the API key
 * @param organizationId - The organization ID
 * @returns Promise that resolves when key is deleted
 */
export async function deleteLLMProxyAPIKey(
  proxyId: string,
  keyName: string,
  organizationId: string
): Promise<void> {
  try {
    await del<void>(
      `/llm-proxies/${encodeURIComponent(proxyId)}/api-keys/${encodeURIComponent(keyName)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
  } catch (error) {
    logger.error(
      `Failed to delete API key ${keyName} for LLM proxy ${proxyId}:`,
      error
    );
    throw error;
  }
}
