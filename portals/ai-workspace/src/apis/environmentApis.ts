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
import { logger } from '../utils/logger';
import { PLATFORM_API_BASE_URL } from '../paths';

/**
 * An organization's deployment environment, as the control plane lists it.
 *
 * `id` is the identity — a gateway stores it in `properties.environment`, and
 * that is what resolves its key manager, so a gateway create request sends the
 * id. `name`/`displayName` are display text and are free to change without
 * breaking an existing binding.
 */
export interface Environment {
  id: string;
  name: string;
  displayName?: string;
  description?: string;
  isSandboxEnv?: boolean;
}

/** Standard control-plane list envelope (same shape as `/gateways`, `/projects`). */
export interface EnvironmentListResponse {
  count?: number;
  list?: Environment[];
}

/**
 * List the environments available to the caller's organization.
 *
 * The organization is resolved from the session token, so no id is sent — the
 * same contract as `getGateways`.
 */
export async function getEnvironments(): Promise<Environment[]> {
  try {
    const response = await get<EnvironmentListResponse>(
      '/environments',
      undefined,
      PLATFORM_API_BASE_URL
    );
    return response?.list ?? [];
  } catch (error) {
    logger.error('Failed to fetch environments:', error);
    throw error;
  }
}
