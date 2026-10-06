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

import { get, post, put, del } from '../clients/choreoApiClient';
import { CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { PLATFORM_API_BASE_URL } from '../paths';
import { buildApiError } from '../utils/apiError';

// ============================================================================
// Types
// ============================================================================

export type ServiceAccountStatus = 'active' | 'disabled';

export interface ServiceAccount {
  id: string;
  displayName: string;
  description: string;
  clientId: string;
  maskedSecret: string;
  roles: string[];
  status: ServiceAccountStatus;
  lastUsedAt?: string;
  lastUsedIp?: string;
  secretRegeneratedAt?: string;
  createdBy?: string;
  createdAt?: string;
  updatedBy?: string;
  updatedAt?: string;
}

export interface ServiceAccountCredentials {
  serviceAccount: ServiceAccount;
  clientId: string;
  clientSecret: string;
}

export interface ServiceAccountRole {
  name: string;
  scopes: string[];
}

interface Pagination {
  total: number;
  limit: number;
  offset: number;
}

export interface ListServiceAccountsResponse {
  count: number;
  list: ServiceAccount[];
  pagination: Pagination;
}

export interface ListServiceAccountRolesResponse {
  count: number;
  list: ServiceAccountRole[];
  pagination: Pagination;
}

export interface CreateServiceAccountRequest {
  id: string;
  displayName: string;
  roles: string[];
  description?: string;
}

export interface UpdateServiceAccountRequest {
  displayName?: string;
  description?: string;
  roles?: string[];
  status?: ServiceAccountStatus;
}

export interface ServiceAccountTokenResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  scope?: string;
}

export const SERVICE_ACCOUNT_EXISTS = 'SERVICE_ACCOUNT_EXISTS';
export const SERVICE_ACCOUNT_INVALID_SCOPE = 'SERVICE_ACCOUNT_INVALID_SCOPE';

// ============================================================================
// API
// ============================================================================

const BASE = '/service-accounts';
const accountPath = (id: string) => `${BASE}/${encodeURIComponent(id)}`;

export function listServiceAccounts(params: {
  limit?: number;
  offset?: number;
  query?: string;
}): Promise<ListServiceAccountsResponse> {
  return get<ListServiceAccountsResponse>(BASE, params);
}

export function getServiceAccount(id: string): Promise<ServiceAccount> {
  return get<ServiceAccount>(accountPath(id));
}

export function listServiceAccountRoles(): Promise<ListServiceAccountRolesResponse> {
  return get<ListServiceAccountRolesResponse>('/service-account-roles');
}

export function createServiceAccount(
  request: CreateServiceAccountRequest,
): Promise<ServiceAccountCredentials> {
  return post<ServiceAccountCredentials>(BASE, request);
}

export function updateServiceAccount(
  id: string,
  request: UpdateServiceAccountRequest,
): Promise<ServiceAccount> {
  return put<ServiceAccount>(accountPath(id), request);
}

export function deleteServiceAccount(id: string): Promise<void> {
  return del<void>(accountPath(id));
}

export function regenerateServiceAccountSecret(id: string): Promise<ServiceAccountCredentials> {
  return post<ServiceAccountCredentials>(`${accountPath(id)}/regenerate-secret`);
}

/**
 * The client credentials grant. Form-encoded, and sent outside `request`: a
 * wrong secret is a 401 UNAUTHORIZED, which must not sign the admin out.
 */
export async function issueServiceAccountToken(
  clientId: string,
  clientSecret: string,
  scopes: readonly string[],
): Promise<ServiceAccountTokenResponse> {
  const form = new URLSearchParams({
    grant_type: 'client_credentials',
    client_id: clientId,
    client_secret: clientSecret,
  });
  if (scopes.length > 0) form.set('scope', scopes.join(' '));

  const res = await fetch(`${PLATFORM_API_BASE_URL}${BASE}/token`, {
    method: 'POST',
    credentials: 'include',
    headers: { Accept: 'application/json', [CSRF_HEADER]: CSRF_VALUE },
    body: form,
  });
  let data: unknown;
  try {
    data = await res.json();
  } catch { /* body not JSON */ }
  if (!res.ok) throw buildApiError(res.status, data, `HTTP ${res.status}`);
  return data as ServiceAccountTokenResponse;
}

/** Every scope the named roles grant, without duplicates. */
export function scopesOfRoles(
  roleNames: readonly string[],
  roles: readonly ServiceAccountRole[],
): string[] {
  const scopes = new Set<string>();
  for (const name of roleNames) {
    for (const scope of roles.find((role) => role.name === name)?.scopes ?? []) scopes.add(scope);
  }
  return [...scopes];
}
