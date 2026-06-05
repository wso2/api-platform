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

import { PLATFORM_API_BASE_URL } from '../config.env';
import { getStoredToken, getOrgToken } from '../clients/choreoApiClient';
import { logger } from '../utils/logger';

// ============================================================================
// Platform API Types
// ============================================================================

/**
 * Organization schema from the Platform API.
 *
 * curl reference:
 *   POST https://localhost:9243/api/v1/organizations
 *   -H 'Authorization: Bearer <token>'
 *   -H 'accept: application/json' -H 'content-type: application/json'
 *   --data-raw '{"id":"<uuid>","name":"<name>","handle":"<handle>","region":"us"}'
 *   --insecure
 *
 * TODO: [REMOVE BEFORE PRODUCTION] Bearer token is currently hardcoded in
 *       choreoApiClient.ts (DEV_FALLBACK_TOKEN). Replace with proper auth.
 */
export interface PlatformOrganization {
  /** UUID v4 — client-generated and sent on registration */
  id: string;
  /** URL-friendly unique handle, pattern: ^[a-z0-9-]+$ */
  handle: string;
  /** Display name */
  name: string;
  /** Geographic region, e.g. "us", "eu", "ap" */
  region: string;
  createdAt?: string;
  updatedAt?: string;
}

export type RegisterOrganizationRequest = Pick<
  PlatformOrganization,
  'id' | 'handle' | 'name' | 'region'
>;

// ============================================================================
// Helpers
// ============================================================================

const platformUrl = (path: string): string => `${PLATFORM_API_BASE_URL}${path}`;

/**
 * Headers for POST /organizations (registration token — no org claim required).
 * TODO: [REMOVE BEFORE PRODUCTION] Remove DEV_FALLBACK_TOKEN from getStoredToken()
 */
const authHeaders = (): Record<string, string> => ({
  'Content-Type': 'application/json',
  Accept: 'application/json',
  Authorization: `Bearer ${getStoredToken()}`,
});

/**
 * Headers for GET /organizations calls — uses the org-specific token whose JWT
 * `organization` claim matches the registered org UUID.
 * TODO: [REMOVE BEFORE PRODUCTION] Remove DEV_GET_ORG_TOKEN from getOrgToken()
 */
const orgAuthHeaders = (): Record<string, string> => ({
  'Content-Type': 'application/json',
  Accept: 'application/json',
  Authorization: `Bearer ${getOrgToken()}`,
});

const parseErrorMessage = async (res: Response): Promise<string> => {
  try {
    const body = await res.json();
    return body?.message ?? body?.error ?? `HTTP ${res.status}`;
  } catch {
    return `HTTP ${res.status}`;
  }
};

// ============================================================================
// Organization API Functions
// ============================================================================

/**
 * Register a new organization.
 *
 * Endpoint: POST /organizations
 * Auth:     Bearer token (sent even though security: [] — server accepts it)
 *
 * TODO: [REMOVE BEFORE PRODUCTION] Remove hardcoded token from authHeaders().
 */
export async function registerOrganization(
  org: RegisterOrganizationRequest,
): Promise<PlatformOrganization> {
  logger.info('Registering organization:', org.handle);

  const response = await fetch(platformUrl('/organizations'), {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(org),
  });

  if (!response.ok) {
    const message = await parseErrorMessage(response);
    logger.error('registerOrganization failed:', response.status, message);

    if (response.status === 409) {
      throw new Error(`Organization with handle "${org.handle}" already exists.`);
    }
    if (response.status === 400) {
      throw new Error(`Invalid organization data: ${message}`);
    }
    throw new Error(`Failed to register organization: ${message}`);
  }

  const created: PlatformOrganization = await response.json();
  logger.info('Organization registered successfully:', created.id);
  return created;
}

/**
 * Get the current user's organization.
 *
 * Endpoint: GET /organizations
 * Auth:     Bearer token (org resolved from JWT claim)
 *
 * TODO: [REMOVE BEFORE PRODUCTION] Remove hardcoded token from authHeaders().
 */
export async function getOrganization(): Promise<PlatformOrganization> {
  const response = await fetch(platformUrl('/organizations'), {
    method: 'GET',
    headers: orgAuthHeaders(),
  });

  if (!response.ok) {
    const message = await parseErrorMessage(response);
    logger.error('getOrganization failed:', response.status, message);
    throw new Error(`Failed to get organization: ${message}`);
  }

  return response.json();
}

/**
 * Check if an organization exists by UUID (HEAD request).
 *
 * Endpoint: HEAD /organizations/{organizationId}
 * Auth:     Bearer token
 *
 * TODO: [REMOVE BEFORE PRODUCTION] Remove hardcoded token from authHeaders().
 */
export async function checkOrganizationExists(
  organizationId: string,
): Promise<boolean> {
  const response = await fetch(platformUrl(`/organizations/${organizationId}`), {
    method: 'HEAD',
    headers: orgAuthHeaders(),
  });

  if (response.status === 404) return false;
  if (response.ok) return true;

  const message = await parseErrorMessage(response);
  throw new Error(`Failed to check organization: ${message}`);
}
