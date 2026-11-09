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

import type { ApiError } from '../../../core/errors';
import { HANDLED_LOCALLY } from '../../../core/queryClient';
import { useApiScope } from '../../../core/scope';
import {
  addGatewaysToApi,
  invokeRestApiOnGateway,
  type AddGatewaysToApiBody,
  type InvokeRestApiOnGatewayBody,
  type InvokeRestApiOnGatewayResponse,
  type ListRestApiGatewaysQuery,
  type RestApiGatewayListResponse,
} from './apiGateways.endpoints';
import { apiGatewayQueries } from './apiGateways.queries';
import { restApiKeys } from '../restApis.queries';

/**
 * The public hook surface for an API's gateway associations.
 *
 * Like `deployments`, every hook takes `restApiId` explicitly rather than
 * reading it from scope: the route's API is not always the one being acted on.
 *
 * The spec offers no remove operation, so there is no unassociate hook. Adding
 * is a bulk POST that returns the API's full gateway list.
 */

export type RestApiGatewayListFilters = ListRestApiGatewaysQuery;

/** Gateways this API is associated with, and therefore deployable to. */
export const useRestApiGateways = (
  restApiId: string | undefined,
  filters: RestApiGatewayListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiGatewayQueries.list(org!, restApiId!, filters),
    enabled: Boolean(org && restApiId),
  });
};

/**
 * Associates gateways with an API.
 *
 * Invalidates both the association list and the API's deployments: a newly
 * associated gateway changes where the API can be deployed, so a deploy target
 * picker reading the deployments view would otherwise stay stale.
 */
export const useAddGatewaysToApi = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();

  return useMutation<
    RestApiGatewayListResponse,
    ApiError,
    { restApiId: string; body: AddGatewaysToApiBody }
  >({
    mutationFn: ({ restApiId, body }) =>
      addGatewaysToApi(restApiId, body, { orgId }),
    onSuccess: (_result, { restApiId }) => {
      if (!org) return;
      void queryClient.invalidateQueries({
        queryKey: restApiKeys.child(org, restApiId, 'gateways'),
      });
      void queryClient.invalidateQueries({
        queryKey: restApiKeys.child(org, restApiId, 'deployments'),
      });
    },
  });
};

/**
 * Selector example: a deploy-target picker only needs id/label, so it should
 * not re-render when an unrelated field changes.
 */
export const useRestApiGatewayOptions = (
  restApiId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiGatewayQueries.list(org!, restApiId!),
    enabled: Boolean(org && restApiId),
    select: (data: RestApiGatewayListResponse) =>
      (data.list ?? []).map((gateway) => ({
        id: gateway.id,
        label: gateway.displayName,
      })),
  });
};

export type InvokeRestApiOnGatewayVariables = {
  body: InvokeRestApiOnGatewayBody;
  signal?: AbortSignal;
};

/**
 * Sends one test request to the REST API as deployed on a gateway (the Test
 * console). The console shows any failure itself, so the global error snackbar
 * stays out of it, and a test request is never resent behind the user's back.
 */
export const useInvokeRestApiOnGateway = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);

  return useMutation<InvokeRestApiOnGatewayResponse, ApiError, InvokeRestApiOnGatewayVariables>({
    mutationFn: ({ body, signal }) => invokeRestApiOnGateway(body, { orgId, signal }),
    meta: HANDLED_LOCALLY,
    retry: false,
  });
};
