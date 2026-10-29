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

import { get, getBlob, getText, post, put, putFormData, putText, del } from '../../clients/choreoApiClient';
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
  PublicationSummaryResponse,
  MCPProxyDocument,
  MCPProxyDocumentContent,
  MCPProxyDocumentListResponse,
} from '../../utils/types';

// ============================================================================
// MCP Server API Functions
// ============================================================================

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
      mcpServer,
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
      updates,
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

/** The server's maximum page size for a publication list. */
const PUBLICATIONS_PAGE_SIZE = 100;

/**
 * Every API Portal registered in the organization, each annotated with this MCP
 * proxy's publication status on it (`GET /api-publications`). An organization with
 * no portals gets an empty list, not an error.
 *
 * Scope: `ap:api_publication:read`.
 */
export async function listMcpProxyApiPortalPublications(
  mcpProxyId: string,
  baseUrl: string
): Promise<PublicationSummaryResponse> {
  return get<PublicationSummaryResponse>(
    '/api-publications',
    { apiType: MCP_PROXY_API_TYPE, apiId: mcpProxyId, limit: PUBLICATIONS_PAGE_SIZE },
    baseUrl
  );
}

/**
 * Replace the publication draft wholesale, creating it on first save.
 *
 * Unlike `publishMcpProxyToApiPortal` this does not publish: the page saves the
 * draft and its definition separately, then publishes what the draft holds.
 */
export async function saveMcpProxyApiPortalDraft(
  apiPortalId: string,
  mcpProxyId: string,
  draft: PublicationDraftDetailsInput,
  baseUrl: string
): Promise<PublicationDraftDetails> {
  return put<PublicationDraftDetails>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft'), draft, baseUrl);
}

/**
 * Store the MCP definition (the tools, resources and prompts the listing
 * advertises) on an existing draft. Sent as `application/json`; the draft must
 * already have been saved or the call is a 404.
 */
export async function saveMcpProxyApiPortalDraftDefinition(
  apiPortalId: string,
  mcpProxyId: string,
  definition: Record<string, unknown>,
  baseUrl: string
): Promise<void> {
  await put<void>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft/definition'), definition, baseUrl);
}

/** The definition stored on the draft; 404 when the draft has not stored one. */
export async function getMcpProxyApiPortalDraftDefinition(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Record<string, unknown>> {
  return get<Record<string, unknown>>(
    mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft/definition'),
    undefined,
    baseUrl
  );
}

/** The definition stored on the live listing; 404 when the listing has none. */
export async function getMcpProxyApiPortalPublicationDefinition(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Record<string, unknown>> {
  return get<Record<string, unknown>>(
    mcpProxyPortalPath(apiPortalId, mcpProxyId, '/publication/definition'),
    undefined,
    baseUrl
  );
}

/** Publish the saved draft. Takes no body; 201 on a first publish, 200 on a refresh. */
export async function publishSavedMcpProxyDraft(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Publication> {
  return post<Publication>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/publish'), undefined, baseUrl);
}

/** Mark the live listing as deprecated. Valid only while it is published. */
export async function deprecateMcpProxyOnApiPortal(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Publication> {
  return post<Publication>(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/deprecate'), undefined, baseUrl);
}

/** The draft's landing page as Markdown; 404 when it has none (`hasLandingPage`). */
export async function getMcpProxyApiPortalDraftLandingPage(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<string> {
  return (await getText(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft/landing-page'), baseUrl)).text;
}

/** Create or replace the draft's landing page. The draft must already exist or the call is a 404. */
export async function saveMcpProxyApiPortalDraftLandingPage(
  apiPortalId: string,
  mcpProxyId: string,
  markdown: string,
  baseUrl: string
): Promise<void> {
  await putText(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft/landing-page'), markdown, 'text/markdown', baseUrl);
}

/** The landing page on the live listing as Markdown; 404 when it has none (`hasLandingPage`). */
export async function getMcpProxyApiPortalPublicationLandingPage(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<string> {
  return (await getText(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/publication/landing-page'), baseUrl)).text;
}

/** The draft's thumbnail; 404 when it has none (`hasThumbnail`). */
export async function getMcpProxyApiPortalDraftThumbnail(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Blob> {
  return getBlob(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft/thumbnail'), baseUrl);
}

/**
 * Create or replace the draft's thumbnail, as a multipart `file`. The draft must already
 * exist or the call is a 404. The server accepts PNG and JPEG, judged by the bytes.
 */
export async function saveMcpProxyApiPortalDraftThumbnail(
  apiPortalId: string,
  mcpProxyId: string,
  image: Blob,
  baseUrl: string
): Promise<void> {
  const form = new FormData();
  form.append('file', image);
  await putFormData(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/draft/thumbnail'), form, baseUrl);
}

/** The thumbnail on the live listing; 404 when it has none (`hasThumbnail`). */
export async function getMcpProxyApiPortalPublicationThumbnail(
  apiPortalId: string,
  mcpProxyId: string,
  baseUrl: string
): Promise<Blob> {
  return getBlob(mcpProxyPortalPath(apiPortalId, mcpProxyId, '/publication/thumbnail'), baseUrl);
}

/** The server's maximum page size for a document list. */
const DOCUMENTS_PAGE_SIZE = 100;

/**
 * Every document of the MCP proxy, paging through the list until the server's total is
 * held. A document created while the pages are read can come up twice; it is listed once.
 * Scope: `ap:docs:read`.
 */
export async function listMcpProxyDocuments(mcpProxyId: string, baseUrl: string): Promise<MCPProxyDocument[]> {
  const path = `/apis/${MCP_PROXY_API_TYPE}/${encodeURIComponent(mcpProxyId)}/docs`;
  const documents: MCPProxyDocument[] = [];
  const seen = new Set<string>();
  let received = 0;
  for (;;) {
    const page = await get<MCPProxyDocumentListResponse>(path, { limit: DOCUMENTS_PAGE_SIZE, offset: received }, baseUrl);
    received += page.list.length;
    for (const document of page.list) {
      if (seen.has(document.id)) continue;
      seen.add(document.id);
      documents.push(document);
    }
    if (page.list.length === 0 || received >= page.pagination.total) return documents;
  }
}

/** One document's body as stored, with the content type it was stored under. Scope: `ap:docs:read`. */
export async function getMcpProxyDocumentContent(
  mcpProxyId: string,
  docId: string,
  baseUrl: string
): Promise<MCPProxyDocumentContent> {
  return getText(
    `/apis/${MCP_PROXY_API_TYPE}/${encodeURIComponent(mcpProxyId)}/docs/${encodeURIComponent(docId)}/content`,
    baseUrl
  );
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
  listMcpProxyApiPortalPublications,
  saveMcpProxyApiPortalDraft,
  saveMcpProxyApiPortalDraftDefinition,
  getMcpProxyApiPortalDraftDefinition,
  getMcpProxyApiPortalPublicationDefinition,
  publishSavedMcpProxyDraft,
  deprecateMcpProxyOnApiPortal,
  getMcpProxyApiPortalDraftLandingPage,
  saveMcpProxyApiPortalDraftLandingPage,
  getMcpProxyApiPortalPublicationLandingPage,
  getMcpProxyApiPortalDraftThumbnail,
  saveMcpProxyApiPortalDraftThumbnail,
  getMcpProxyApiPortalPublicationThumbnail,
  listMcpProxyDocuments,
  getMcpProxyDocumentContent,
};
