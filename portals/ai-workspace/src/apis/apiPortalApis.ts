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

import { get } from '../clients/choreoApiClient';
import { API_PORTAL_URL } from '../config.env';
import { getHttpStatus } from '../utils/apiError';
import { logger } from '../utils/logger';

/**
 * An API Portal registered in the organization, as `GET /api-portals/{id}`
 * reports it. The shared key is write-only and never returned.
 */
export interface ApiPortal {
  id: string;
  handle: string;
  name: string;
  description?: string;
  /** Public HTTPS URL of the portal. Operator-supplied at registration. */
  url?: string;
  status: string;
}

/**
 * Read one API Portal by its handle.
 *
 * Scopes: `ap:api_portal:read` or `ap:api_portal:manage`.
 */
export async function getApiPortal(
  apiPortalId: string,
  baseUrl: string
): Promise<ApiPortal> {
  return get<ApiPortal>(
    `/api-portals/${encodeURIComponent(apiPortalId)}`,
    undefined,
    baseUrl
  );
}

/**
 * Where to send a user who wants to see a published listing in the portal.
 *
 * The portal's REGISTERED URL is the answer, not the listing's own
 * `endpoints.productionUrl`: that field is the API's invocation endpoint (for an
 * MCP proxy, the gateway URL the publish dialog filled in), so linking to it
 * sent "View in API Portal" at the gateway rather than at the portal.
 *
 * Resolution order, both steps optional so a deployment can satisfy either:
 *   1. the `url` on the organization's own API Portal registration, which is
 *      authoritative and per-organization;
 *   2. `api_portal_url` (`APIP_AIW_API_PORTAL_URL`), for a deployment whose
 *      portal is a fixed, well-known address and is not registered through
 *      this API at all.
 *
 * Returns undefined when neither is configured — the caller then shows no link
 * rather than a broken one. Never throws: a portal that cannot be read is the
 * same as one with no URL.
 */
export async function resolveApiPortalUrl(
  apiPortalId: string,
  baseUrl: string
): Promise<string | undefined> {
  try {
    const portal = await getApiPortal(apiPortalId, baseUrl);
    const registered = portal.url?.trim();
    if (registered) return registered;
  } catch (error) {
    const status = getHttpStatus(error);
    // Expected, not a fault: 404 is a deployment that never registers its
    // portal, and 401/403 a viewer without `ap:api_portal:read`. Both fall
    // through to the configured URL below.
    if (status === 401 || status === 403 || status === 404) {
      logger.debug(
        `API Portal registration not readable (${status}); using the configured URL.`
      );
    } else {
      logger.error('Failed to read the API Portal registration:', error);
    }
  }
  // Reached whenever step 1 produced nothing — a failure above, or a
  // registration that carries no `url`.
  return API_PORTAL_URL.trim() || undefined;
}
