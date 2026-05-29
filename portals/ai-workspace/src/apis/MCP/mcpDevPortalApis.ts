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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { post } from '../../clients/choreoApiClient';
import { logger } from '../../utils/logger';

export interface PublishMCPServerRequest {
  /** The org handle used as namespace prefix in the registry ({orgHandle}/{proxy-name}) */
  orgHandle: string;
  /** Public-facing URL of the deployed MCP proxy endpoint */
  remoteUrl: string;
}

export interface PublishMCPServerResponse {
  /** Canonical registry entry name, e.g. "my-org/my-mcp-proxy" */
  refId: string;
}

function buildRegistryPath(
  devPortalBaseUrl: string,
  orgHandle: string,
  mcpProxyId: string
): string {
  return `${devPortalBaseUrl}/registry/${encodeURIComponent(orgHandle)}/v0.1/servers/${encodeURIComponent(mcpProxyId)}/versions`;
}

/**
 * Check if an MCP proxy is published in the Developer Portal.
 * Returns true if published (HTTP 200), false if not found (HTTP 404).
 * Throws for any other error.
 */
export async function checkMCPServerPublished(
  devPortalBaseUrl: string,
  orgHandle: string,
  mcpProxyId: string
): Promise<boolean> {
  const url = buildRegistryPath(devPortalBaseUrl, orgHandle, mcpProxyId);
  console.debug('[checkMCPServerPublished] url:', url);
  try {
    const response = await fetch(url);
    if (response.ok) return true;
    if (response.status === 404) return false;
    logger.error('Unexpected status checking MCP server published status:', response.status);
    throw new Error(`Unexpected status: ${response.status}`);
  } catch (error) {
    logger.error('Failed to check MCP server published status:', error);
    throw error;
  }
}

/**
 * Publish an MCP proxy to the Developer Portal via the APIM Publisher API.
 * APIM fetches the full proxy record, builds the registry payload, and POSTs to devportal.
 * POST {apimBaseUrl}/mcp-proxies/{id}/publish?organizationId={orgId}
 */
export async function publishMCPServer(
  apimBaseUrl: string,
  mcpProxyId: string,
  organizationId: string,
  request: PublishMCPServerRequest
): Promise<PublishMCPServerResponse> {
  try {
    const response = await post<PublishMCPServerResponse>(
      `/mcp-proxies/${encodeURIComponent(mcpProxyId)}/publish?organizationId=${encodeURIComponent(organizationId)}`,
      request,
      apimBaseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to publish MCP server:', error);
    throw error;
  }
}

/**
 * Unpublish an MCP proxy from the Developer Portal via the APIM Publisher API.
 * POST {apimBaseUrl}/mcp-proxies/{id}/unpublish?organizationId={orgId}
 */
export async function unpublishMCPServer(
  apimBaseUrl: string,
  mcpProxyId: string,
  organizationId: string,
  orgHandle: string
): Promise<void> {
  try {
    await post<unknown>(
      `/mcp-proxies/${encodeURIComponent(mcpProxyId)}/unpublish?organizationId=${encodeURIComponent(organizationId)}`,
      { orgHandle, remoteUrl: '' },
      apimBaseUrl
    );
  } catch (error) {
    logger.error('Failed to unpublish MCP server:', error);
    throw error;
  }
}
