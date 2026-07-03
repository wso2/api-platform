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

import { PLATFORM_API_BASE_URL, CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { logger } from '../utils/logger';

// ============================================================================
// Platform API Types
// ============================================================================

/**
 * Organization schema from the Platform API.
 *
 * Requests are routed same-origin through the BFF proxy. The browser holds no
 * token: every call rides the HttpOnly `_bff_session` cookie and the BFF injects
 * the bearer token when proxying to the Platform API.
 */
export interface PlatformOrganization {
  /** Handle (URL-friendly slug), pattern: ^[a-z0-9-]+$ — readOnly, server-assigned */
  id: string;
  /** Display name */
  displayName: string;
  /** Geographic region, e.g. "us", "eu", "ap" */
  region: string;
  createdAt?: string;
  updatedAt?: string;
}

export type RegisterOrganizationRequest = Pick<
  PlatformOrganization,
  'id' | 'displayName' | 'region'
>;

// ============================================================================
// Helpers
// ============================================================================

const platformUrl = (path: string): string => `${PLATFORM_API_BASE_URL}${path}`;

/**
 * Base JSON headers. No Authorization — the BFF injects the bearer token from
 * the session when proxying. All calls below use `credentials: 'include'` so the
 * HttpOnly `_bff_session` cookie rides along and the BFF can resolve the token.
 */
const jsonHeaders = (): Record<string, string> => ({
  'Content-Type': 'application/json',
  Accept: 'application/json',
});

/**
 * Headers for state-mutating requests (POST/PUT/PATCH/DELETE). Adds the custom
 * CSRF header the BFF requires — cross-site attackers cannot set a custom header
 * because CORS is closed. Must match the BFF's CSRF_HEADER config.
 */
const mutatingHeaders = (): Record<string, string> => ({
  ...jsonHeaders(),
  [CSRF_HEADER]: CSRF_VALUE,
});

const parseErrorMessage = async (res: Response): Promise<string> => {
  try {
    const body = await res.json();
    return body?.message ?? body?.error ?? `HTTP ${res.status}`;
  } catch {
    return `HTTP ${res.status}`;
  }
};

/**
 * Build an Error that carries the originating HTTP status, so callers can
 * branch on it (e.g. surface a "session expired" logout flow on 401) instead
 * of string-matching the message.
 */
const httpError = (message: string, status: number): Error & { status: number } => {
  const err = new Error(message) as Error & { status: number };
  err.status = status;
  return err;
};

// ============================================================================
// Organization API Functions
// ============================================================================

/**
 * Register a new organization.
 *
 * Endpoint: POST /organizations
 * Auth:     BFF session cookie; the BFF injects the bearer token.
 */
export async function registerOrganization(
  org: RegisterOrganizationRequest,
): Promise<PlatformOrganization> {
  logger.info('Registering organization:', org.id);

  const response = await fetch(platformUrl('/organizations'), {
    method: 'POST',
    credentials: 'include',
    headers: mutatingHeaders(),
    body: JSON.stringify(org),
  });

  if (!response.ok) {
    const message = await parseErrorMessage(response);
    logger.error('registerOrganization failed:', response.status, message);

    if (response.status === 409) {
      throw httpError(`Organization with handle "${org.id}" already exists.`, 409);
    }
    if (response.status === 400) {
      throw httpError(`Invalid organization data: ${message}`, 400);
    }
    throw httpError(`Failed to register organization: ${message}`, response.status);
  }

  const created: PlatformOrganization = await response.json();
  logger.info('Organization registered successfully:', created.id);
  return created;
}

/**
 * Get the current user's organization.
 *
 * Endpoint: GET /organizations
 * Auth:     BFF session cookie; the BFF injects the bearer token.
 */
export async function getOrganization(): Promise<PlatformOrganization> {
  const response = await fetch(platformUrl('/organizations'), {
    method: 'GET',
    credentials: 'include',
    headers: jsonHeaders(),
  });

  if (!response.ok) {
    const message = await parseErrorMessage(response);
    logger.error('getOrganization failed:', response.status, message);
    throw new Error(`Failed to get organization: ${message}`);
  }

  return response.json();
}

/**
 * Fetch an organization by its handle.
 * Returns null when the org is not yet registered (404).
 *
 * Endpoint: GET /organizations/{organizationId}
 *           where {organizationId} is the org handle.
 * Auth:     BFF session cookie; the BFF injects the bearer token.
 */
export async function getOrganizationById(
  handle: string,
): Promise<PlatformOrganization | null> {
  const response = await fetch(platformUrl(`/organizations/${handle}`), {
    method: 'GET',
    credentials: 'include',
    headers: jsonHeaders(),
  });

  if (response.status === 404) return null;
  if (!response.ok) {
    const message = await parseErrorMessage(response);
    logger.error('getOrganizationById failed:', response.status, message);
    throw new Error(`Failed to get organization: ${message}`);
  }
  return response.json();
}

/**
 * Fetch an organization by its handle.
 * Returns null when the org is not yet registered (404).
 *
 * Endpoint: GET /organizations/{handle}
 * Auth:     BFF session cookie; the BFF injects the bearer token.
 */
export async function getOrganizationByHandle(
  handle: string,
): Promise<PlatformOrganization | null> {
  const response = await fetch(platformUrl(`/organizations/${handle}`), {
    method: 'GET',
    credentials: 'include',
    headers: jsonHeaders(),
  });

  if (response.status === 404) return null;
  if (!response.ok) {
    const message = await parseErrorMessage(response);
    logger.error('getOrganizationByHandle failed:', response.status, message);
    throw new Error(`Failed to get organization: ${message}`);
  }
  return response.json();
}

/**
 * Check if an organization exists by handle (HEAD request).
 *
 * Endpoint: HEAD /organizations/{organizationId}
 *           where {organizationId} is the org handle.
 * Auth:     BFF session cookie; the BFF injects the bearer token.
 */
export async function checkOrganizationExists(
  handle: string,
): Promise<boolean> {
  const response = await fetch(platformUrl(`/organizations/${handle}`), {
    method: 'HEAD',
    credentials: 'include',
    headers: jsonHeaders(),
  });

  if (response.status === 404) return false;
  if (response.ok) return true;

  const message = await parseErrorMessage(response);
  throw httpError(`Failed to check organization: ${message}`, response.status);
}
