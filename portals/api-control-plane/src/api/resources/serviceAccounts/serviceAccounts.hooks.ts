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

import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { ErrorCode, isApiError, isErrorCode, type ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  createServiceAccount,
  deleteServiceAccount,
  issueServiceAccountToken,
  regenerateServiceAccountSecret,
  updateServiceAccount,
  type CreateServiceAccountBody,
  type ListServiceAccountsQuery,
  type ServiceAccount,
  type ServiceAccountCredentials,
  type ServiceAccountTokenBody,
  type ServiceAccountTokenResponse,
  type UpdateServiceAccountBody,
} from './serviceAccounts.endpoints';
import { serviceAccountKeys, serviceAccountQueries } from './serviceAccounts.queries';

/**
 * The public hook surface for service accounts.
 *
 * Create, regenerate and the token exchange return a secret or a live token, so
 * those mutations use `gcTime: 0` and never seed the cache: the value lives only
 * in the dialog that shows it.
 */

/**
 * Service accounts in the active organization. Paging or searching changes the
 * key, so the previous page stays on screen until the next one arrives.
 */
export const useServiceAccounts = (
  query: ListServiceAccountsQuery = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...serviceAccountQueries.list(org!, query),
    enabled: Boolean(org),
    placeholderData: keepPreviousData,
  });
};

/**
 * One account, read fresh each time: the edit form starts from it, and an
 * update carries no version, so a cached copy could overwrite newer changes.
 */
export const useServiceAccount = (
  id: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...serviceAccountQueries.detail(org!, id!),
    enabled: Boolean(org && id),
    gcTime: 0,
  });
};

/** The roles an account may hold, each with the scopes it grants. */
export const useServiceAccountRoles = (overrides: { orgId?: string } = {}) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...serviceAccountQueries.roles(org!),
    enabled: Boolean(org),
  });
};

const useInvalidateServiceAccounts = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: serviceAccountKeys.all(org) });
  };
};

/** Creates an account. The result holds the only copy of its secret. */
export const useCreateServiceAccount = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateServiceAccounts(orgId);

  return useMutation<ServiceAccountCredentials, ApiError, CreateServiceAccountBody>({
    mutationFn: (body) => createServiceAccount(body, { orgId }),
    gcTime: 0,
    onSuccess: () => invalidate(),
  });
};

/** Edits metadata or roles, and disables or enables the account. */
export const useUpdateServiceAccount = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateServiceAccounts(orgId);

  return useMutation<ServiceAccount, ApiError, { id: string; body: UpdateServiceAccountBody }>({
    mutationFn: ({ id, body }) => updateServiceAccount(id, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useDeleteServiceAccount = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateServiceAccounts(orgId);

  return useMutation<void, ApiError, { id: string }>({
    mutationFn: ({ id }) => deleteServiceAccount(id, { orgId }),
    onSuccess: (_result, { id }) => {
      if (org) queryClient.removeQueries({ queryKey: serviceAccountKeys.detail(org, id) });
      invalidate();
    },
  });
};

/** Replaces the secret. The result holds the only copy of the new one. */
export const useRegenerateServiceAccountSecret = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateServiceAccounts(orgId);

  return useMutation<ServiceAccountCredentials, ApiError, { id: string }>({
    mutationFn: ({ id }) => regenerateServiceAccountSecret(id, { orgId }),
    gcTime: 0,
    onSuccess: () => invalidate(),
  });
};

/**
 * Exchanges a client ID and secret for an access token. Counts as using the
 * account on the server, so it refreshes the list's "last token issued".
 */
export const useIssueServiceAccountToken = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateServiceAccounts(orgId);

  return useMutation<ServiceAccountTokenResponse, ApiError, ServiceAccountTokenBody>({
    mutationFn: (body) => issueServiceAccountToken(body),
    gcTime: 0,
    onSuccess: () => invalidate(),
  });
};

/** Another request changed the account after this one read it. */
export const isServiceAccountConflict = (error: unknown): boolean =>
  isErrorCode(error, ErrorCode.CONFLICT);

/** The server has service accounts turned off, so the routes do not exist. */
export const isServiceAccountsDisabled = (error: unknown): boolean =>
  isApiError(error) && error.isNotFound;

/** `roles` as a list; one place to change if the API moves to a single role. */
export const rolesOf = (account: Pick<ServiceAccount, 'roles'>): string[] => account.roles ?? [];

/** Every scope the given roles grant, deduplicated, in role order. */
export const scopesOfRoles = (
  roleNames: readonly string[],
  roles: readonly { name: string; scopes: string[] }[]
): string[] => {
  const scopes = new Set<string>();
  for (const name of roleNames) {
    for (const scope of roles.find((role) => role.name === name)?.scopes ?? []) scopes.add(scope);
  }
  return [...scopes];
};
