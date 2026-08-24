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
import type { BodyOf, PathOf, QueryOf, ResponseOf } from '../../core/spec';

/**
 * Transport layer for API keys.
 *
 * This resource is shaped by the spec rather than by symmetry, and the shape is
 * asymmetric: **writes are scoped to one REST API**
 * (`/rest-apis/{restApiId}/api-keys`), while the only **read** is the
 * caller-scoped `/me/api-keys`, filterable by artifact type but not by a
 * specific API.
 *
 * There is therefore no way to list the keys belonging to one REST API. That is
 * a gap in platform-api, not an omission here — see the note on
 * `listMyApiKeys` below.
 */

export type UserApiKeyListResponse = ResponseOf<'listUserAPIKeys'>;
export type ListMyApiKeysQuery = QueryOf<'listUserAPIKeys'>;
export type CreateApiKeyBody = BodyOf<'CreateAPIKey'>;
export type CreateApiKeyResponse = ResponseOf<'CreateAPIKey'>;
export type UpdateApiKeyBody = BodyOf<'UpdateAPIKey'>;
export type UpdateApiKeyResponse = ResponseOf<'UpdateAPIKey'>;

/** Artifact kinds a key can be issued against. */
export type ApiKeyArtifactType = NonNullable<ListMyApiKeysQuery>['type'];

const collectionPath = (restApiId: PathOf<'CreateAPIKey'>['restApiId']): string =>
  `/rest-apis/${encodeURIComponent(restApiId)}/api-keys`;

const resourcePath = (
  restApiId: string,
  apiKeyId: PathOf<'UpdateAPIKey'>['apiKeyId']
): string => `${collectionPath(restApiId)}/${encodeURIComponent(apiKeyId)}`;

/**
 * Keys created by the signed-in user, across artifacts.
 *
 * This is the only read the spec offers, and it is scoped to the *caller*, not
 * to a resource: it cannot answer "which keys exist for this API". Until
 * platform-api adds a per-API listing, a screen that needs one has to filter
 * this client-side by artifact type and accept that it sees only the current
 * user's keys.
 */
export const listMyApiKeys = async (
  options?: RequestOptions
): Promise<UserApiKeyListResponse> => {
  return http.get<UserApiKeyListResponse>('/me/api-keys', {
    ...options,
    operationName: 'listUserAPIKeys',
  });
};

/**
 * Issues a key for a REST API.
 *
 * The plaintext key is returned once, in this response, and is never
 * retrievable again — callers must surface it immediately rather than caching
 * it for later display.
 */
export const createApiKey = async (
  restApiId: string,
  body: CreateApiKeyBody,
  options?: RequestOptions
): Promise<CreateApiKeyResponse> => {
  return http.post<CreateApiKeyResponse>(collectionPath(restApiId), body, {
    ...options,
    operationName: 'CreateAPIKey',
  });
};

export const updateApiKey = async (
  restApiId: string,
  apiKeyId: string,
  body: UpdateApiKeyBody,
  options?: RequestOptions
): Promise<UpdateApiKeyResponse> => {
  return http.put<UpdateApiKeyResponse>(resourcePath(restApiId, apiKeyId), body, {
    ...options,
    operationName: 'UpdateAPIKey',
  });
};

/** Revokes a key. Irreversible; any client using it starts failing immediately. */
export const revokeApiKey = async (
  restApiId: string,
  apiKeyId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(restApiId, apiKeyId), {
    ...options,
    operationName: 'RevokeAPIKey',
  });
};
