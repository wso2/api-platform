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

import { get, del, postForm, putForm } from '../clients/choreoApiClient';

// ============================================================================
// Types
// ============================================================================

export type SecretType = 'GENERIC' | 'CERTIFICATE';
export type SecretStatus = 'ACTIVE' | 'DEPRECATED';

export interface CreateSecretRequest {
  handle: string;
  name: string;
  description?: string;
  value: string;
  type?: SecretType;
}

export interface UpdateSecretRequest {
  value: string;
  name?: string;
  description?: string;
}

export interface SecretMetadata {
  uuid: string;
  handle: string;
  name: string;
  description?: string;
  type: SecretType;
  provider: string;
  status: SecretStatus;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSecretResponse extends SecretMetadata {}
export interface UpdateSecretResponse extends SecretMetadata {}

export interface ListSecretsResponse {
  list: SecretMetadata[];
  pagination: {
    total: number;
    limit: number;
    offset: number;
  };
}

export interface SecretReference {
  type: string;
  handle: string;
  name: string;
}

export interface DeleteSecretConflict {
  error: string;
  references: SecretReference[];
}

export class SecretConflictError extends Error {
  readonly status = 409;
  readonly conflict: DeleteSecretConflict;
  constructor(conflict: DeleteSecretConflict) {
    super(conflict.error);
    this.name = 'SecretConflictError';
    this.conflict = conflict;
  }
}

// ============================================================================
// API
// ============================================================================

/**
 * Creates an encrypted secret via the BFF proxy.
 * Sent as multipart/form-data; the API never returns the plaintext value.
 */
export async function createSecret(
  request: CreateSecretRequest,
): Promise<CreateSecretResponse> {
  const form = new FormData();
  form.append('handle', request.handle);
  form.append('name', request.name);
  if (request.description) form.append('description', request.description);
  form.append('value', request.value);
  if (request.type) form.append('type', request.type);
  return postForm<CreateSecretResponse>('/secrets', form);
}

/**
 * Lists all secrets in the organization (metadata only — values are never returned).
 */
export async function listSecrets(
  params?: { limit?: number; offset?: number },
): Promise<ListSecretsResponse> {
  return get<ListSecretsResponse>('/secrets', params);
}

/**
 * Returns metadata for a single secret by handle.
 */
export async function getSecret(handle: string): Promise<SecretMetadata> {
  return get<SecretMetadata>(`/secrets/${handle}`);
}

/**
 * Rotates a secret's value. All {{ secret "handle" }} references remain valid —
 * the gateway picks up the new value on its next sync cycle.
 * Sent as multipart/form-data; the new plaintext value is never returned.
 */
export async function updateSecret(
  handle: string,
  request: UpdateSecretRequest,
): Promise<UpdateSecretResponse> {
  const form = new FormData();
  form.append('value', request.value);
  if (request.name) form.append('name', request.name);
  if (request.description) form.append('description', request.description);
  return putForm<UpdateSecretResponse>(`/secrets/${handle}`, form);
}

/**
 * Soft-deletes a secret (sets status to DEPRECATED).
 * Throws SecretConflictError (with a populated .conflict.references list) if
 * the secret is still referenced by a deployed resource.
 */
export async function deleteSecret(handle: string): Promise<void> {
  try {
    return await del<void>(`/secrets/${handle}`);
  } catch (err: unknown) {
    if (err instanceof Error && (err as { status?: number }).status === 409) {
      const data = (err as { data?: unknown }).data as DeleteSecretConflict | undefined;
      if (data?.references) {
        throw new SecretConflictError(data);
      }
    }
    throw err;
  }
}

/**
 * Builds the {{ secret "name" }} placeholder string for use in resource configs.
 */
export function buildSecretPlaceholder(secretName: string): string {
  return `{{ secret "${secretName}" }}`;
}

/**
 * Generates a unique secret handle. Each call returns a different value so
 * re-creating a resource with the same name never collides with a prior
 * (possibly soft-deleted) secret.
 */
export function generateSecretHandle(): string {
  return crypto.randomUUID();
}

/**
 * Extracts the secret handle from a {{ secret "handle" }} placeholder string.
 * Returns null if the value is not a placeholder.
 */
export function extractSecretHandle(placeholder: string): string | null {
  const match = placeholder.match(/^\{\{\s*secret\s+"([^"]+)"\s*\}\}$/);
  return match ? match[1] : null;
}
