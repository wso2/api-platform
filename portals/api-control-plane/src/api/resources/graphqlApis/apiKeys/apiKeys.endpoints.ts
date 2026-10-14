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

import { http, type RequestOptions } from '../../../core/http';
import type { BodyOf, PathOf, ResponseOf } from '../../../core/spec';

/**
 * Transport layer for a GraphQL API's own API keys — the write side only.
 * Mirrors `apiKeys/apiKeys.endpoints.ts`'s REST write side exactly; the read
 * (`/me/api-keys`, caller-scoped across every artifact kind) stays that
 * shared module's `listMyApiKeys`/`useMyApiKeys` — there is nothing
 * GraphQL-specific to add there.
 */

export type CreateApiKeyBody = BodyOf<'CreateGraphQLAPIKey'>;
export type CreateApiKeyResponse = ResponseOf<'CreateGraphQLAPIKey'>;
export type UpdateApiKeyBody = BodyOf<'UpdateGraphQLAPIKey'>;
export type UpdateApiKeyResponse = ResponseOf<'UpdateGraphQLAPIKey'>;

const collectionPath = (
  graphqlApiId: PathOf<'CreateGraphQLAPIKey'>['graphqlApiId'],
): string => `/graphql-apis/${encodeURIComponent(graphqlApiId)}/api-keys`;

const resourcePath = (
  graphqlApiId: string,
  apiKeyId: PathOf<'UpdateGraphQLAPIKey'>['apiKeyId'],
): string => `${collectionPath(graphqlApiId)}/${encodeURIComponent(apiKeyId)}`;

/**
 * Issues a key for a GraphQL API. The plaintext key is returned once, in this
 * response, and is never retrievable again.
 */
export const createApiKey = async (
  graphqlApiId: string,
  body: CreateApiKeyBody,
  options?: RequestOptions,
): Promise<CreateApiKeyResponse> => {
  return http.post<CreateApiKeyResponse>(collectionPath(graphqlApiId), body, {
    ...options,
    operationName: 'CreateGraphQLAPIKey',
  });
};

export const updateApiKey = async (
  graphqlApiId: string,
  apiKeyId: string,
  body: UpdateApiKeyBody,
  options?: RequestOptions,
): Promise<UpdateApiKeyResponse> => {
  return http.put<UpdateApiKeyResponse>(resourcePath(graphqlApiId, apiKeyId), body, {
    ...options,
    operationName: 'UpdateGraphQLAPIKey',
  });
};

/** Revokes a key. Irreversible; any client using it starts failing immediately. */
export const revokeApiKey = async (
  graphqlApiId: string,
  apiKeyId: string,
  options?: RequestOptions,
): Promise<void> => {
  await http.delete<void>(resourcePath(graphqlApiId, apiKeyId), {
    ...options,
    operationName: 'RevokeGraphQLAPIKey',
  });
};
