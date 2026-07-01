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

import { get, post, put, del } from '../clients/choreoApiClient';
import { logger } from '../utils/logger';
import { PLATFORM_API_BASE_URL, BFF_COMPOSITE_BASE_URL } from '../config.env';

// ============================================================================
// Type Definitions (moved to `src/utils/types.ts`)
// ============================================================================

import type {
  LLMProvider,
  LLMProvidersResponse,
  ProxiesResponse,
  CreateLLMProviderRequest,
  UpdateLLMProviderRequest,
  DeploymentListResponse,
  DeploymentResponse,
  DeployRequest,
  CreateLLMProviderAPIKeyRequest,
  CreateLLMProviderAPIKeyResponse,
  APIKeyListResponse,
} from '../utils/types';
import { buildFullProviderRequest } from '../utils/tmpSPRequest';

const sanitizeRateLimitEntry = (entry: unknown) => {
  if (!entry || typeof entry !== 'object' || Array.isArray(entry)) {
    return entry;
  }

  const nextEntry = { ...(entry as Record<string, unknown>) };
  const count = nextEntry.count;
  if (typeof count === 'number' && count <= 0) {
    delete nextEntry.count;
  }

  return nextEntry;
};

const sanitizeLimitConfig = (limit: unknown) => {
  if (!limit || typeof limit !== 'object' || Array.isArray(limit)) {
    return limit;
  }

  const nextLimit = { ...(limit as Record<string, unknown>) };
  if ('request' in nextLimit) {
    nextLimit.request = sanitizeRateLimitEntry(nextLimit.request);
  }
  if ('token' in nextLimit) {
    nextLimit.token = sanitizeRateLimitEntry(nextLimit.token);
  }

  return nextLimit;
};

const sanitizeResourceWiseRateLimiting = (resourceWise: unknown) => {
  if (
    !resourceWise ||
    typeof resourceWise !== 'object' ||
    Array.isArray(resourceWise)
  ) {
    return resourceWise;
  }

  const nextResourceWise = { ...(resourceWise as Record<string, unknown>) };
  if ('default' in nextResourceWise) {
    nextResourceWise.default = sanitizeLimitConfig(nextResourceWise.default);
  }
  if (Array.isArray(nextResourceWise.resources)) {
    nextResourceWise.resources = nextResourceWise.resources.map((resource) => {
      if (!resource || typeof resource !== 'object' || Array.isArray(resource)) {
        return resource;
      }
      const nextResource = { ...(resource as Record<string, unknown>) };
      if ('limit' in nextResource) {
        nextResource.limit = sanitizeLimitConfig(nextResource.limit);
      }
      return nextResource;
    });
  }

  return nextResourceWise;
};

const sanitizeProviderLevelRateLimiting = (level: unknown) => {
  if (!level || typeof level !== 'object' || Array.isArray(level)) {
    return level;
  }

  const nextLevel = { ...(level as Record<string, unknown>) };
  if ('global' in nextLevel) {
    nextLevel.global = sanitizeLimitConfig(nextLevel.global);
  }
  if ('resourceWise' in nextLevel) {
    nextLevel.resourceWise = sanitizeResourceWiseRateLimiting(
      nextLevel.resourceWise
    );
  }

  return nextLevel;
};

const sanitizeUpdatePayload = (
  updates: UpdateLLMProviderRequest
): UpdateLLMProviderRequest => {
  if (!updates.rateLimiting) return updates;

  const nextRateLimiting = {
    ...updates.rateLimiting,
  };

  if ('providerLevel' in nextRateLimiting) {
    nextRateLimiting.providerLevel = sanitizeProviderLevelRateLimiting(
      nextRateLimiting.providerLevel
    ) as typeof nextRateLimiting.providerLevel;
  }
  if ('consumerLevel' in nextRateLimiting) {
    nextRateLimiting.consumerLevel = sanitizeProviderLevelRateLimiting(
      nextRateLimiting.consumerLevel
    ) as typeof nextRateLimiting.consumerLevel;
  }

  return {
    ...updates,
    rateLimiting: nextRateLimiting,
  };
};

// ============================================================================
// LLM Provider API Functions
// ============================================================================

/**
 * Create a new LLM Provider
 *
 * @param provider - The LLM provider details
 * @param organizationId - The organization ID
 * @returns Promise with the created provider
 *
 * @example
 * ```ts
 * const provider = await createLLMProvider({
 *   name: "OpenAI GPT-4",
 *   description: "Primary OpenAI provider"
 * }, 'org-uuid');
 * console.log(provider); // { id: '...', name: 'OpenAI GPT-4', ... }
 * ```
 */
export async function createLLMProvider(
  provider: CreateLLMProviderRequest,
  organizationId: string,
  _baseUrl: string
): Promise<LLMProvider> {
  try {
    // TODO: Remove buildFullProviderRequest once backend supports partial creation
    const fullProvider = buildFullProviderRequest(provider);
    // Routed through the BFF composite endpoint so the BFF can compensate by
    // deleting the pre-created secret if the provider creation fails.
    const response = await post<LLMProvider>(
      `/llm-providers?organizationId=${encodeURIComponent(organizationId)}`,
      fullProvider,
      BFF_COMPOSITE_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to create LLM provider:', error);
    throw error;
  }
}

/**
 * Get all LLM Providers
 *
 * @param organizationId - The organization ID
 * @returns Promise with the list of LLM providers
 *
 * @example
 * ```ts
 * const response = await getLLMProviders('org-uuid');
 * console.log(response); // { count: 1, list: [...], pagination: {...} }
 * ```
 */
export async function getLLMProviders(
  organizationId: string,
  baseUrl: string
): Promise<LLMProvidersResponse> {
  try {
    const response = await get<LLMProvidersResponse>(
      `/llm-providers?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch LLM providers:', error);
    throw error;
  }
}

/**
 * Get a single LLM Provider by ID
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @returns Promise with the provider details
 *
 * @example
 * ```ts
 * const provider = await getLLMProvider('wso2-openai-provider', 'org-uuid');
 * console.log(provider); // { id: 'wso2-openai-provider', name: '...', ... }
 * ```
 */
export async function getLLMProvider(
  providerId: string,
  organizationId: string,
  baseUrl: string
): Promise<LLMProvider> {
  try {
    const response = await get<LLMProvider>(
      `/llm-providers/${encodeURIComponent(providerId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch LLM provider ${providerId}:`, error);
    throw error;
  }
}

/**
 * Get all LLM proxies linked to an LLM provider
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise with the list of linked LLM proxies
 */
export async function getLLMProviderProxies(
  providerId: string,
  organizationId: string,
  baseUrl: string = PLATFORM_API_BASE_URL
): Promise<ProxiesResponse> {
  try {
    const response = await get<ProxiesResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/llm-proxies?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to fetch LLM proxies for provider ${providerId}:`,
      error
    );
    throw error;
  }
}

/**
 * Update an existing LLM Provider
 *
 * @param providerId - The LLM provider ID
 * @param updates - The fields to update
 * @param organizationId - The organization ID
 * @returns Promise with the updated provider
 *
 * @example
 * ```ts
 * const provider = await updateLLMProvider('wso2-openai-provider', {
 *   name: "OpenAI GPT-4 Turbo",
 *   description: "Updated provider"
 * }, 'org-uuid');
 * console.log(provider); // { id: '...', name: 'OpenAI GPT-4 Turbo', ... }
 * ```
 */
export async function updateLLMProvider(
  providerId: string,
  updates: UpdateLLMProviderRequest,
  organizationId: string,
  baseUrl: string
): Promise<LLMProvider> {
  try {
    const sanitizedUpdates = sanitizeUpdatePayload(updates);
    // Drop vhost only when it's an empty string; keep it when it has a value.
    const safeUpdates = Object.fromEntries(
      Object.entries(sanitizedUpdates).filter(
        ([key, value]) =>
          key !== 'vhost' ||
          !(typeof value === 'string' && value.trim().length === 0)
      )
    ) as UpdateLLMProviderRequest;
    const response = await put<LLMProvider>(
      `/llm-providers/${encodeURIComponent(providerId)}?organizationId=${encodeURIComponent(organizationId)}`,
      safeUpdates,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update LLM provider ${providerId}:`, error);
    throw error;
  }
}

/**
 * Delete an LLM Provider
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @returns Promise that resolves when the provider is deleted
 *
 * @example
 * ```ts
 * await deleteLLMProvider('wso2-openai-provider', 'org-uuid');
 * console.log('Provider deleted successfully');
 * ```
 */
export async function deleteLLMProvider(
  providerId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/llm-providers/${encodeURIComponent(providerId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete LLM provider ${providerId}:`, error);
    throw error;
  }
}

// ============================================================================
// LLM Provider Deployment API Functions
// ============================================================================

/**
 * Get all deployments for an LLM Provider
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @param gatewayId - Optional gateway ID to filter by
 * @param status - Optional status to filter by
 * @param baseUrl - The base URL for the API
 * @returns Promise with the list of deployments
 */
export async function getLLMProviderDeployments(
  providerId: string,
  organizationId: string,
  baseUrl: string,
  gatewayId?: string,
  status?: string
): Promise<DeploymentListResponse> {
  try {
    let url = `/llm-providers/${encodeURIComponent(providerId)}/deployments?organizationId=${encodeURIComponent(organizationId)}`;
    if (gatewayId) {
      url += `&gatewayId=${encodeURIComponent(gatewayId)}`;
    }
    if (status) {
      url += `&status=${encodeURIComponent(status)}`;
    }
    const response = await get<DeploymentListResponse>(url, undefined, baseUrl);
    return response;
  } catch (error) {
    logger.error(
      `Failed to fetch LLM provider deployments for ${providerId}:`,
      error
    );
    throw error;
  }
}

/**
 * Deploy an LLM Provider to a gateway
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @param deployRequest - The deployment configuration
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment response
 */
export async function deployLLMProvider(
  providerId: string,
  organizationId: string,
  deployRequest: DeployRequest,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await post<DeploymentResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/deployments?organizationId=${encodeURIComponent(organizationId)}`,
      deployRequest,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to deploy LLM provider ${providerId}:`, error);
    throw error;
  }
}

/**
 * Get a specific deployment by ID
 *
 * @param providerId - The LLM provider ID
 * @param deploymentId - The deployment ID
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise with the deployment details
 */
export async function getLLMProviderDeployment(
  providerId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await get<DeploymentResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/deployments/${encodeURIComponent(deploymentId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch deployment ${deploymentId}:`, error);
    throw error;
  }
}

/**
 * Delete a deployment
 *
 * @param providerId - The LLM provider ID
 * @param deploymentId - The deployment ID
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise that resolves when deleted
 */
export async function deleteLLMProviderDeployment(
  providerId: string,
  deploymentId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/llm-providers/${encodeURIComponent(providerId)}/deployments/${encodeURIComponent(deploymentId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete deployment ${deploymentId}:`, error);
    throw error;
  }
}

/**
 * Undeploy an LLM provider deployment from a gateway
 *
 * @param providerId - The LLM provider ID
 * @param deploymentId - The deployment ID
 * @param gatewayId - The gateway ID
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise with the updated deployment response
 */
export async function undeployLLMProviderDeployment(
  providerId: string,
  deploymentId: string,
  gatewayId: string,
  organizationId: string,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await post<DeploymentResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/deployments/undeploy?organizationId=${encodeURIComponent(organizationId)}&deploymentId=${encodeURIComponent(deploymentId)}&gatewayId=${encodeURIComponent(gatewayId)}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to undeploy deployment ${deploymentId}:`, error);
    throw error;
  }
}

/**
 * Restore a previous LLM provider deployment
 *
 * @param providerId - The LLM provider ID
 * @param deploymentId - The deployment ID
 * @param gatewayId - The gateway ID
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise with the restored deployment response
 */
export async function restoreLLMProviderDeployment(
  providerId: string,
  deploymentId: string,
  gatewayId: string,
  organizationId: string,
  baseUrl: string
): Promise<DeploymentResponse> {
  try {
    const response = await post<DeploymentResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/deployments/restore?organizationId=${encodeURIComponent(organizationId)}&deploymentId=${encodeURIComponent(deploymentId)}&gatewayId=${encodeURIComponent(gatewayId)}`,
      {},
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to restore deployment ${deploymentId}:`, error);
    throw error;
  }
}

// ============================================================================
// LLM Provider API Key Functions
// ============================================================================

/**
 * Create a new API key for an LLM provider
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @param request - The API key creation request
 * @param baseUrl - The base URL for the API
 * @returns Promise with the API key response
 */
export async function createLLMProviderAPIKey(
  providerId: string,
  organizationId: string,
  request: CreateLLMProviderAPIKeyRequest,
  baseUrl: string
): Promise<CreateLLMProviderAPIKeyResponse> {
  try {
    const response = await post<CreateLLMProviderAPIKeyResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/api-keys?organizationId=${encodeURIComponent(organizationId)}`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to create API key for provider ${providerId}:`, error);
    throw error;
  }
}

/**
 * List all API keys for an LLM provider
 *
 * @param providerId - The LLM provider ID
 * @param organizationId - The organization ID
 * @returns Promise with the list of API keys
 */
export async function getLLMProviderAPIKeys(
  providerId: string,
  organizationId: string
): Promise<APIKeyListResponse> {
  try {
    const response = await get<APIKeyListResponse>(
      `/llm-providers/${encodeURIComponent(providerId)}/api-keys?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch API keys for provider ${providerId}:`, error);
    throw error;
  }
}

/**
 * Delete an API key for an LLM provider
 *
 * @param providerId - The LLM provider ID
 * @param keyName - The name of the API key to delete
 * @param organizationId - The organization ID
 * @param baseUrl - The base URL for the API
 * @returns Promise that resolves when the API key is deleted
 */
export async function deleteLLMProviderAPIKey(
  providerId: string,
  keyName: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/llm-providers/${encodeURIComponent(providerId)}/api-keys/${encodeURIComponent(keyName)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete API key ${keyName} for provider ${providerId}:`, error);
    throw error;
  }
}
