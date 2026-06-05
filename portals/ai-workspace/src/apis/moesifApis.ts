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

import { get, post } from '../clients/choreoApiClient';
import { API_BASE_URLS } from '../config.env';
import type { MoesifTokenResponse } from '../utils/types';
import { logger } from '../utils/logger';

// ============================================================================
// Moesif API Functions
// ============================================================================

export interface RegisterMoesifOrganizationResponse {
  organizationId?: string;
  apps?: { name: string }[];
}

/**
 * Fetch Moesif ID token using the provided bearer token.
 *
 * @returns Promise with Moesif token response
 */
export async function getMoesifToken(): Promise<MoesifTokenResponse> {
  const response = await get<MoesifTokenResponse>(
    '/id_token',
    undefined,
    API_BASE_URLS.moesifAPI
  );

  return response;
}

/**
 * Registers the organization with Moesif (required before fetching Moesif key).
 * POST body: { user_name, apps: [{ name: env_name }, ...] }.
 * Returns the response body; if response.apps is empty
 */
export async function registerMoesifOrganization(
  userName: string,
  apps: { name: string }[]
): Promise<RegisterMoesifOrganizationResponse | undefined> {
  if (!apps?.length) return undefined;
  try {
    const response = await post<RegisterMoesifOrganizationResponse>(
      '/organization',
      {
        user_name: userName || 'User',
        apps: apps.map((a) => ({ name: a.name })),
      },
      API_BASE_URLS.moesifAPI
    );
    return response ?? undefined;
  } catch (e) {
    logger.error('Error registering Moesif organization', e);
    throw e;
  }
}

/**
 * Fetches the Moesif key for a given environment (e.g. gateway's associated environment).
 * Uses query params expose=true and env=<env>.
 */
export async function getMoesifKey(env: string): Promise<string | null> {
  if (!env || !env.trim()) return null;
  try {
    const response = await get<{
      moesif_key?: string;
      organization_id?: string;
    }>(
      '/moesif_key',
      { expose: 'true', env: env.trim() },
      API_BASE_URLS.moesifAPI
    );
    return response?.moesif_key ?? null;
  } catch (e) {
    logger.error('Error fetching Moesif key for environment', e);
    return null;
  }
}
