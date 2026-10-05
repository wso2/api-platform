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

import { http, type RequestOptions } from '../../core/http';
import type {
  BodyOf,
  PathOf,
  QueryOf,
  ResponseOf,
  Schema,
  UrlEncodedBodyOf,
} from '../../core/spec';

/**
 * Transport layer for `/service-accounts` and `/service-account-roles`.
 *
 * Create and regenerate return the only copy of a client secret, and the token
 * exchange returns a live bearer token. Nothing here caches; see the hooks for
 * how those responses are kept out of the query store.
 */

export type ServiceAccount = Schema<'ServiceAccount'>;
export type ServiceAccountCredentials = Schema<'ServiceAccountCredentials'>;
export type ServiceAccountRole = Schema<'ServiceAccountRole'>;
export type ServiceAccountListResponse = ResponseOf<'listServiceAccounts'>;
export type ServiceAccountRoleListResponse = ResponseOf<'listServiceAccountRoles'>;
export type ServiceAccountTokenResponse = ResponseOf<'issueServiceAccountToken'>;
export type ListServiceAccountsQuery = QueryOf<'listServiceAccounts'>;
export type CreateServiceAccountBody = BodyOf<'createServiceAccount'>;
export type UpdateServiceAccountBody = BodyOf<'updateServiceAccount'>;
export type ServiceAccountTokenBody = UrlEncodedBodyOf<'issueServiceAccountToken'>;

const BASE = '/service-accounts';

const resourcePath = (id: PathOf<'getServiceAccount'>['serviceAccountId']): string =>
  `${BASE}/${encodeURIComponent(id)}`;

export const listServiceAccounts = async (
  options?: RequestOptions
): Promise<ServiceAccountListResponse> =>
  http.get<ServiceAccountListResponse>(BASE, {
    ...options,
    operationName: 'listServiceAccounts',
  });

export const getServiceAccount = async (
  id: string,
  options?: RequestOptions
): Promise<ServiceAccount> =>
  http.get<ServiceAccount>(resourcePath(id), {
    ...options,
    operationName: 'getServiceAccount',
  });

export const listServiceAccountRoles = async (
  options?: RequestOptions
): Promise<ServiceAccountRoleListResponse> =>
  http.get<ServiceAccountRoleListResponse>('/service-account-roles', {
    ...options,
    operationName: 'listServiceAccountRoles',
  });

/** The response carries the only copy of the client secret. */
export const createServiceAccount = async (
  body: CreateServiceAccountBody,
  options?: RequestOptions
): Promise<ServiceAccountCredentials> =>
  http.post<ServiceAccountCredentials>(BASE, body, {
    ...options,
    operationName: 'createServiceAccount',
  });

/** Metadata, roles or status. A 409 means another request changed it first. */
export const updateServiceAccount = async (
  id: string,
  body: UpdateServiceAccountBody,
  options?: RequestOptions
): Promise<ServiceAccount> =>
  http.put<ServiceAccount>(resourcePath(id), body, {
    ...options,
    operationName: 'updateServiceAccount',
  });

export const deleteServiceAccount = async (
  id: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(id), {
    ...options,
    operationName: 'deleteServiceAccount',
  });
};

/** The old secret stops working at once; the response carries the new one. */
export const regenerateServiceAccountSecret = async (
  id: string,
  options?: RequestOptions
): Promise<ServiceAccountCredentials> =>
  http.post<ServiceAccountCredentials>(`${resourcePath(id)}/regenerate-secret`, undefined, {
    ...options,
    operationName: 'regenerateServiceAccountSecret',
  });

/**
 * OAuth2 client credentials exchange, form-encoded. A 401 here means the
 * client ID or secret was wrong, so it must not be read as an expired session.
 */
export const issueServiceAccountToken = async (
  body: ServiceAccountTokenBody,
  options?: RequestOptions
): Promise<ServiceAccountTokenResponse> => {
  const form = new URLSearchParams({ grant_type: body.grant_type });
  if (body.client_id) form.set('client_id', body.client_id);
  if (body.client_secret) form.set('client_secret', body.client_secret);
  if (body.scope) form.set('scope', body.scope);
  return http.post<ServiceAccountTokenResponse>(`${BASE}/token`, form, {
    ...options,
    authFailureIsSession: false,
    operationName: 'issueServiceAccountToken',
  });
};
