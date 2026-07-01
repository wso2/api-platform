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

import type {
  Proxy,
  ProxiesResponse,
  CreateProxyRequest,
  UpdateProxyRequest,
} from '../utils/types';

// ============================================================================
// Proxy API Functions
// ============================================================================

/**
 * Create a new LLM Proxy
 *
 * @param proxy - The proxy details
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the created proxy
 */
export async function createProxy(
  proxy: CreateProxyRequest,
  organizationId: string,
  baseUrl: string
): Promise<Proxy> {
  try {
    const response = await post<Proxy>(
      `/llm-proxies`,
      proxy,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to create proxy:', error);
    throw error;
  }
}

/**
 * Get all LLM Proxies
 *
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the list of proxies
 */
export async function getProxies(organizationId: string, projectId: string, baseUrl: string): Promise<ProxiesResponse> {
  try {
    const response = await get<ProxiesResponse>(
      `/llm-proxies?projectId=${encodeURIComponent(projectId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch proxies:', error);
    throw error;
  }
}

/**
 * Get all LLM Proxies for an organization (without project filter)
 *
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the list of proxies
 */
// TODO: listLLMProxies requires projectId per openapi.yaml — this function omits it and may 400; needs backend/product clarification
export async function getOrgProxies(organizationId: string, baseUrl: string): Promise<ProxiesResponse> {
  try {
    const response = await get<ProxiesResponse>(
      `/llm-proxies`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch organization proxies:', error);
    throw error;
  }
}

/**
 * Get a single LLM Proxy by ID
 *
 * @param proxyId - The proxy ID
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the proxy details
 */
export async function getProxy(
  proxyId: string,
  organizationId: string,
  baseUrl: string
): Promise<Proxy> {
  try {
    const response = await get<Proxy>(
      `/llm-proxies/${encodeURIComponent(proxyId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch proxy ${proxyId}:`, error);
    throw error;
  }
}

/**
 * Update an existing LLM Proxy
 *
 * @param proxyId - The proxy ID
 * @param updates - The fields to update
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the updated proxy
 */
export async function updateProxy(
  proxyId: string,
  updates: UpdateProxyRequest,
  organizationId: string,
  baseUrl: string
): Promise<Proxy> {
  try {
    const response = await put<Proxy>(
      `/llm-proxies/${encodeURIComponent(proxyId)}`,
      updates,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update proxy ${proxyId}:`, error);
    throw error;
  }
}

/**
 * Delete an LLM Proxy
 *
 * @param proxyId - The proxy ID
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise that resolves when the proxy is deleted
 */
export async function deleteProxy(
  proxyId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/llm-proxies/${encodeURIComponent(proxyId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete proxy ${proxyId}:`, error);
    throw error;
  }
}
