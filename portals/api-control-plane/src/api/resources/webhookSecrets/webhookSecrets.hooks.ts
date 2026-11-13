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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { isApiError, type ApiError } from '../../core/errors';
import { requireOrgScope, useApiScope } from '../../core/scope';
import {
  createWebhookSecret,
  rotateWebhookSecret,
  type CreateWebhookSecretBody,
  type WebhookSecretWithValue,
} from './webhookSecrets.endpoints';
import { webhookSecretKeys, webhookSecretQueries } from './webhookSecrets.queries';

/**
 * One webhook secret's metadata, never its value. 404s whenever no secret with
 * this id exists yet - an expected outcome (a portal not yet set up to send
 * events), not a failure: callers branch on `query.error?.isNotFound`.
 */
export const useWebhookSecret = (
  secretId: string | undefined,
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...webhookSecretQueries.detail(org!, secretId!),
    enabled: Boolean(org && secretId),
  });
};

/**
 * Creates a webhook secret. The response is the only place its plaintext ever
 * appears, so it is returned to the caller and never cached.
 */
export const useCreateWebhookSecret = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();

  return useMutation<WebhookSecretWithValue, ApiError, CreateWebhookSecretBody>({
    mutationFn: async (body) =>
      createWebhookSecret(body, { orgId: requireOrgScope(orgId, 'createWebhookSecret') }),
    onSuccess: () => {
      if (org) void queryClient.invalidateQueries({ queryKey: webhookSecretKeys.all(org) });
    },
  });
};

/** Issues a new version of a secret; the plaintext of the new version is returned once. */
export const useRotateWebhookSecret = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();

  return useMutation<WebhookSecretWithValue, ApiError, { secretId: string }>({
    mutationFn: async ({ secretId }) =>
      rotateWebhookSecret(secretId, { orgId: requireOrgScope(orgId, 'rotateWebhookSecret') }),
    onSuccess: () => {
      if (org) void queryClient.invalidateQueries({ queryKey: webhookSecretKeys.all(org) });
    },
  });
};

/** No webhook secret with this id exists yet: the expected answer before one is issued. */
export const isWebhookSecretMissing = (error: unknown): boolean =>
  isApiError(error) && error.isNotFound;
