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

import { post } from '../../clients/choreoApiClient';
import { logger } from '../../utils/logger';

import type {
  MCPServerInfoFetchRequest,
  MCPServerInfoFetchResponse,
} from '../../utils/types';

// ============================================================================
// MCP Server Validation API Functions
// ============================================================================

/**
 * Fetch server info from MCP proxy backend services
 *
 * Validates connectivity and retrieves metadata about the backend services.
 *
 * @param request - The server info fetch request
 * @param organizationId - The organization ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the server info response
 */
export async function fetchMCPProxyServerInfo(
  request: MCPServerInfoFetchRequest,
  organizationId: string,
  baseUrl: string
): Promise<MCPServerInfoFetchResponse> {
  try {
    const response = await post<MCPServerInfoFetchResponse>(
      `/mcp-proxies/fetch-server-info?organizationId=${encodeURIComponent(organizationId)}`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch MCP proxy server info:', error);
    throw error;
  }
}
