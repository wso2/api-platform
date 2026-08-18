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

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  createApiKey,
  revokeApiKey,
  updateApiKey,
  type CreateApiKeyBody,
  type CreateApiKeyResponse,
  type ListMyApiKeysQuery,
  type UpdateApiKeyBody,
  type UpdateApiKeyResponse,
} from './apiKeys.endpoints';
import { apiKeyKeys, apiKeyQueries } from './apiKeys.queries';

/**
 * The public hook surface for API keys.
 *
 * The read and the writes are scoped differently, which is unusual and worth
 * stating plainly: `useMyApiKeys` returns the *caller's* keys across all
 * artifacts, while the mutations act on one REST API's keys. Every mutation
 * therefore takes `restApiId` explicitly and invalidates the caller-scoped
 * list, because there is no per-API list to invalidate.
 */

export type ApiKeyListFilters = ListMyApiKeysQuery;

/**
 * Keys created by the signed-in user.
 *
 * Pass `type` to narrow to one artifact kind. Note this never shows keys
 * created by *other* users in the organization — the spec offers no endpoint
 * for that, so a screen implying otherwise would be misleading.
 */
export const useMyApiKeys = (
  filters: ApiKeyListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiKeyQueries.mine(org!, filters),
    enabled: Boolean(org),
  });
};

/**
 * Invalidation helper shared by every API key mutation.
 *
 * Invalidates the resource root, which covers every filtered variant of the
 * caller's key list — a new or revoked key changes counts and pagination on
 * filters the user has not currently got open.
 */
const useInvalidateApiKeys = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: apiKeyKeys.all(org) });
  };
};

/**
 * Issues a key for a REST API.
 *
 * The plaintext key is in the mutation result and is never retrievable again,
 * so the caller must show it immediately. It is deliberately not written into
 * the cache: a secret that only exists once should not sit in a store other
 * components can read.
 */
export const useCreateApiKey = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiKeys(orgId);

  return useMutation<
    CreateApiKeyResponse,
    ApiError,
    { restApiId: string; body: CreateApiKeyBody }
  >({
    mutationFn: ({ restApiId, body }) => createApiKey(restApiId, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useUpdateApiKey = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiKeys(orgId);

  return useMutation<
    UpdateApiKeyResponse,
    ApiError,
    { restApiId: string; apiKeyId: string; body: UpdateApiKeyBody }
  >({
    mutationFn: ({ restApiId, apiKeyId, body }) =>
      updateApiKey(restApiId, apiKeyId, body, { orgId }),
    // No optimistic write: the cached list is caller-scoped and may not even
    // contain this key (another user could have created it), so patching an
    // entry by id would be guesswork.
    onSuccess: () => invalidate(),
  });
};

/** Revokes a key. Any client using it starts failing immediately. */
export const useRevokeApiKey = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiKeys(orgId);

  return useMutation<void, ApiError, { restApiId: string; apiKeyId: string }>({
    mutationFn: ({ restApiId, apiKeyId }) =>
      revokeApiKey(restApiId, apiKeyId, { orgId }),
    onSuccess: () => invalidate(),
  });
};
