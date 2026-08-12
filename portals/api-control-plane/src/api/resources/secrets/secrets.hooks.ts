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

import { isErrorCode, type ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  createSecret,
  deleteSecret,
  rotateSecret,
  type CreateSecretBody,
  type ListSecretsQuery,
  type RotateSecretBody,
  type SecretListResponse,
  type SecretResponse,
} from './secrets.endpoints';
import { secretKeys, secretQueries } from './secrets.queries';

/**
 * The public hook surface for secrets — the only thing components import.
 *
 * Scope is implicit but overridable, and `query.error` is always an `ApiError`
 * with `.code`, `.fieldErrors` and `.status`.
 *
 * No mutation here seeds its response into the cache. Reads return metadata
 * only, and keeping it that way means a secret's value can never be recovered
 * from the query store.
 */

export type SecretListFilters = ListSecretsQuery;

/** Secrets in the active organization. Metadata only; values are never read. */
export const useSecrets = (
  filters: SecretListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...secretQueries.list(org!, filters),
    enabled: Boolean(org),
  });
};

/** A single secret's metadata. */
export const useSecret = (
  secretId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...secretQueries.detail(org!, secretId!),
    enabled: Boolean(org && secretId),
  });
};

/**
 * Invalidation helper shared by every secret mutation.
 *
 * Invalidates the resource root rather than a specific list key, because a
 * create or delete shifts pagination and counts on list pages the user has not
 * visited yet.
 */
const useInvalidateSecrets = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: secretKeys.all(org) });
  };
};

/** Creates a secret. The submitted value is never readable again afterwards. */
export const useCreateSecret = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateSecrets(orgId);

  return useMutation<SecretResponse, ApiError, CreateSecretBody>({
    mutationFn: (body) => createSecret(body, { orgId }),
    // Deliberately no `setQueryData` seeding here, unlike other resources: the
    // create response describes a secret, and secrets stay out of the store.
    onSuccess: () => invalidate(),
  });
};

/** Replaces a secret's value in place, keeping its id and references. */
export const useRotateSecret = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateSecrets(orgId);

  return useMutation<
    SecretResponse,
    ApiError,
    { secretId: string; body: RotateSecretBody }
  >({
    mutationFn: ({ secretId, body }) => rotateSecret(secretId, body, { orgId }),
    // No optimistic write either: rotation changes server-managed metadata
    // (updatedAt, version) that cannot be predicted client-side.
    onSuccess: () => invalidate(),
  });
};

/**
 * Deletes a secret.
 *
 * The backend refuses while other resources still reference it. Use
 * {@link isSecretInUse} on the error to tell that case apart, and read
 * `error.details` for the blocking resources rather than showing a generic
 * failure.
 */
export const useDeleteSecret = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateSecrets(orgId);

  return useMutation<void, ApiError, { secretId: string }>({
    mutationFn: ({ secretId }) => deleteSecret(secretId, { orgId }),
    onSuccess: (_result, { secretId }) => {
      if (org) {
        // Drop the detail entry outright — refetching a deleted secret just to
        // receive a 404 is a wasted round trip and an error the user would
        // briefly see.
        queryClient.removeQueries({ queryKey: secretKeys.detail(org, secretId) });
      }
      invalidate();
    },
  });
};

/**
 * True when a delete was blocked because the secret is still referenced.
 *
 * `error.details` then carries the referencing resources, which is what a
 * useful message needs — "in use by 3 APIs" rather than "delete failed".
 */
export const isSecretInUse = (error: unknown): boolean =>
  isErrorCode(error, 'SECRET_IN_USE');

/**
 * Selector example: a secret picker only needs id/label, so it should not
 * re-render when an unrelated field changes.
 */
export const useSecretOptions = (filters: SecretListFilters = {}) => {
  const { org } = useApiScope();

  return useQuery({
    ...secretQueries.list(org!, filters),
    enabled: Boolean(org),
    select: (data: SecretListResponse) =>
      (data.list ?? []).map((secret) => ({
        id: secret.id,
        label: secret.displayName,
      })),
  });
};
