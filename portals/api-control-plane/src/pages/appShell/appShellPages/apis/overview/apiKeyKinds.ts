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

import {
  useCreateApiKey as useCreateRestApiKey,
  useRevokeApiKey as useRevokeRestApiKey,
  type ApiKeyArtifactType,
  type CreateApiKeyBody,
  type CreateApiKeyResponse,
} from '@/api/resources/apiKeys';
import {
  useCreateApiKey as useCreateGraphQLApiKey,
  useRevokeApiKey as useRevokeGraphQLApiKey,
} from '@/api/resources/graphqlApis/apiKeys';
import type { ApiError } from '@/api/core/errors';
import type { PermissionOperation } from '@/permissions/evaluate';

/**
 * The kinds of API whose keys `ApiKeysPanel` and `CreateApiKeyDialog` manage.
 * Both kinds share one request/response contract (`CreateAPIKeyRequest` /
 * `CreateAPIKeyResponse`) and one server-side key service; what differs is the
 * collection the request goes to, the artifact type the shared `/me/api-keys`
 * read is narrowed to, and the operation each permission check names.
 */
export type ApiKeyApiKind = 'rest' | 'graphql';

type ApiKeyKindConfig = {
  /** The `type` filter value for the caller's key inventory. */
  artifactType: NonNullable<ApiKeyArtifactType>[number];
  /** `operationId`s gating the add and revoke actions. */
  createOperation: PermissionOperation;
  revokeOperation: PermissionOperation;
};

export const API_KEY_KINDS: Record<ApiKeyApiKind, ApiKeyKindConfig> = {
  graphql: {
    artifactType: 'GraphQLApi',
    createOperation: 'CreateGraphQLAPIKey',
    revokeOperation: 'RevokeGraphQLAPIKey',
  },
  rest: {
    artifactType: 'RestApi',
    createOperation: 'CreateAPIKey',
    revokeOperation: 'RevokeAPIKey',
  },
};

type MutateCallbacks<T> = {
  onSuccess?: (data: T) => void;
  onError?: (error: ApiError) => void;
};

/**
 * Issues a key for an API of `kind`. Both kinds' mutations are created on every
 * render (hooks can't be conditional); only the one for `kind` is ever called.
 */
export function useCreateApiKeyForKind(kind: ApiKeyApiKind) {
  const rest = useCreateRestApiKey();
  const graphql = useCreateGraphQLApiKey();

  const mutate = (
    apiId: string,
    body: CreateApiKeyBody,
    callbacks?: MutateCallbacks<CreateApiKeyResponse>,
  ) =>
    kind === 'graphql'
      ? graphql.mutate({ body, graphqlApiId: apiId }, callbacks)
      : rest.mutate({ body, restApiId: apiId }, callbacks);

  return { isPending: (kind === 'graphql' ? graphql : rest).isPending, mutate };
}

/** Revokes a key of an API of `kind`. See `useCreateApiKeyForKind`. */
export function useRevokeApiKeyForKind(kind: ApiKeyApiKind) {
  const rest = useRevokeRestApiKey();
  const graphql = useRevokeGraphQLApiKey();

  const mutate = (apiId: string, apiKeyId: string, callbacks?: MutateCallbacks<void>) =>
    kind === 'graphql'
      ? graphql.mutate({ apiKeyId, graphqlApiId: apiId }, callbacks)
      : rest.mutate({ apiKeyId, restApiId: apiId }, callbacks);

  return { isPending: (kind === 'graphql' ? graphql : rest).isPending, mutate };
}
