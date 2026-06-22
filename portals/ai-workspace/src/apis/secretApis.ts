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

import { post } from '../clients/choreoApiClient';

// ============================================================================
// Types
// ============================================================================

export type SecretType = 'API_KEY' | 'CERTIFICATE' | 'PRIVATE_KEY' | 'GENERIC';

export interface CreateSecretRequest {
  name: string;
  displayName: string;
  description?: string;
  value: string;
  type: SecretType;
  projectId?: string;
}

export interface CreateSecretResponse {
  id: string;
  name: string;
  displayName: string;
  type: SecretType;
  provider: string;
  value: string;
  createdAt: string;
  createdBy: string;
}

// ============================================================================
// API
// ============================================================================

/**
 * Creates an encrypted secret in the Platform API.
 * The plaintext value is returned once in the response and never again.
 *
 * @param request - Secret creation payload
 * @param baseUrl - Platform API base URL
 * @returns The created secret (includes plaintext value — store or discard immediately)
 */
export async function createSecret(
  request: CreateSecretRequest,
  baseUrl: string
): Promise<CreateSecretResponse> {
  return post<CreateSecretResponse>('/secrets', request, baseUrl);
}

/**
 * Builds the {{ secret "name" }} placeholder string for use in resource configs.
 */
export function buildSecretPlaceholder(secretName: string): string {
  return `{{ secret "${secretName}" }}`;
}

/**
 * Generates a deterministic secret handle from a provider ID and field name.
 * Ensures the handle conforms to the allowed character set (lowercase alphanumeric + hyphens).
 *
 * Example: generateSecretHandle('wso2-openai', 'api-key') → 'wso2-openai-api-key'
 */
export function generateSecretHandle(providerId: string, fieldName = 'api-key'): string {
  const handle = `${providerId}-${fieldName}`
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/^-+|-+$/g, '');
  if (!handle) {
    throw new Error(`Cannot generate a valid secret handle from providerId="${providerId}" and fieldName="${fieldName}"`);
  }
  return handle;
}
