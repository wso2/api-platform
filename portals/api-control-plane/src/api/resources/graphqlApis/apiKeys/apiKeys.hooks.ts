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

import { useMutation, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '../../../core/errors';
import { useApiScope } from '../../../core/scope';
import { apiKeyKeys } from '../../apiKeys/apiKeys.queries';
import type { UserApiKeyListResponse } from '../../apiKeys';
import {
  createApiKey,
  revokeApiKey,
  updateApiKey,
  type CreateApiKeyBody,
  type CreateApiKeyResponse,
  type UpdateApiKeyBody,
  type UpdateApiKeyResponse,
} from './apiKeys.endpoints';

/**
 * Write-side hooks for a GraphQL API's own API keys — mirrors
 * `apiKeys/apiKeys.hooks.ts`'s REST mutations exactly, sharing that module's
 * `apiKeyKeys` cache namespace (and therefore its `useMyApiKeys` read) since
 * `/me/api-keys` is one caller-scoped list across every artifact kind.
 */

const useInvalidateApiKeys = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: apiKeyKeys.all(org) });
  };
};

export const useCreateApiKey = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiKeys(orgId);

  return useMutation<
    CreateApiKeyResponse,
    ApiError,
    { graphqlApiId: string; body: CreateApiKeyBody }
  >({
    mutationFn: ({ graphqlApiId, body }) => createApiKey(graphqlApiId, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useUpdateApiKey = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiKeys(orgId);

  return useMutation<
    UpdateApiKeyResponse,
    ApiError,
    { graphqlApiId: string; apiKeyId: string; body: UpdateApiKeyBody }
  >({
    mutationFn: ({ graphqlApiId, apiKeyId, body }) =>
      updateApiKey(graphqlApiId, apiKeyId, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

/** Revokes a key. Any client using it starts failing immediately. */
export const useRevokeApiKey = (overrides: { orgId?: string } = {}) => {
  const queryClient = useQueryClient();
  const { org, orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiKeys(orgId);

  return useMutation<void, ApiError, { graphqlApiId: string; apiKeyId: string }>({
    mutationFn: ({ graphqlApiId, apiKeyId }) => revokeApiKey(graphqlApiId, apiKeyId, { orgId }),
    onSuccess: (_data, { graphqlApiId, apiKeyId }) => {
      // Same reasoning as the REST revoke hook: a revoke is a status change on
      // `/me/api-keys`, so remove the exact key from every cached filter
      // immediately rather than waiting on the invalidated refetch.
      if (org) {
        queryClient.setQueriesData<UserApiKeyListResponse>(
          { queryKey: apiKeyKeys.all(org) },
          (current) => {
            if (!current) return current;
            const list = current.list.filter(
              (key) =>
                !(
                  key.artifactType === 'GraphQLApi' &&
                  key.artifactId === graphqlApiId &&
                  key.id === apiKeyId
                ),
            );
            return {
              ...current,
              count: Math.max(0, current.count - (current.list.length - list.length)),
              list,
            };
          },
        );
      }
      invalidate();
    },
  });
};
