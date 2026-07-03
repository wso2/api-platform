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

import { get, del } from '../clients/choreoApiClient';
import { logger } from '../utils/logger';

import type {
  UserAPIKeyListResponse,
} from '../utils/types';

// ============================================================================
// Key Management API Functions
// ============================================================================

/**
 * Revoke an API key for a REST API
 *
 * @param restApiId - The REST API handle
 * @param apiKeyId - The unique name/identifier of the API key to revoke
 * @param baseUrl - The APIM base URL
 * @returns Promise that resolves when the key is revoked
 */
export async function revokeAPIKey(
  restApiId: string,
  apiKeyId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/rest-apis/${encodeURIComponent(restApiId)}/api-keys/${encodeURIComponent(apiKeyId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to revoke API key ${apiKeyId} for API ${restApiId}:`, error);
    throw error;
  }
}

/**
 * List API keys for the current user
 *
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @param type - Optional comma-separated artifact types to filter by (e.g. "LlmProxy,LlmProvider,RestApi")
 * @returns Promise with the list of user API keys
 */
export async function listUserAPIKeys(
  organizationId: string,
  baseUrl: string,
  type?: string
): Promise<UserAPIKeyListResponse> {
  try {
    let url = `/me/api-keys?organizationId=${encodeURIComponent(organizationId)}`;
    if (type) {
      url += `&type=${encodeURIComponent(type)}`;
    }
    const response = await get<UserAPIKeyListResponse>(
      url,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to list user API keys:', error);
    throw error;
  }
}

export const keyManagementApis = {
  revokeAPIKey,
  listUserAPIKeys,
};
