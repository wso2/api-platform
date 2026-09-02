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
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { requireOrgScope, useApiScope } from '../../core/scope';
import {
  createGateway,
  deleteGateway,
  rotateGatewayToken,
  updateGateway,
  type CreateGatewayBody,
  type Gateway,
  type GatewayListResponse,
  type ListGatewaysQuery,
  type TokenRotationResponse,
  type UpdateGatewayBody,
} from './gateways.endpoints';
import { gatewayKeys, gatewayQueries } from './gateways.queries';

/**
 * The public hook surface for gateways — the only thing components import.
 *
 * Same two conventions as every other resource module:
 *
 * - **Scope is implicit but overridable.** Hooks default to the route's
 *   organization via `useApiScope()`; an explicit override is accepted for the
 *   rare cross-org case. Components pass nothing in the common path.
 * - **Errors are `ApiError`.** The transport normalizes everything, so
 *   `query.error` always carries `.code`, `.fieldErrors` and `.status` —
 *   components never see an `AxiosError` and never parse a body.
 *
 * Gateways are organization-scoped, not project-scoped: one gateway serves
 * every project in the org, so the only gate here is `org`.
 */

/** Everything a caller may vary on the list request. */
export type GatewayListFilters = ListGatewaysQuery;

/**
 * Gateways in the active organization.
 *
 * The query does not run until the organization is known, so this never fires
 * a request the server would reject — and, critically, never writes into a
 * cache entry keyed by an empty scope. Components branch on `isPending` rather
 * than `isLoading` for exactly that reason.
 *
 * `keepPreviousData` matches the other list hooks: paging or searching changes
 * the key, and without it the listing would unmount into a loading state on
 * every change instead of dimming the page it already has.
 */
export const useGateways = (
  filters: GatewayListFilters = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...gatewayQueries.list(org!, filters),
    enabled: Boolean(org),
    placeholderData: keepPreviousData,
  });
};

/**
 * A single gateway by handle.
 *
 * `poll` exists because `isActive` flips out-of-band, when a self-hosted agent
 * finishes connecting — the setup flow watches this query for that transition
 * rather than asking the user to reload.
 */
export const useGateway = (
  gatewayId: string | undefined,
  options: { poll?: boolean } = {},
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...gatewayQueries.detail(org!, gatewayId!),
    enabled: Boolean(org && gatewayId),
    refetchInterval: options.poll ? GATEWAY_POLL_INTERVAL_MS : false,
  });
};

/** How often a watched gateway is re-read while waiting for its agent. */
const GATEWAY_POLL_INTERVAL_MS = 5000;

/**
 * Invalidation helper shared by every gateway mutation.
 *
 * It invalidates the resource root rather than a specific list key, because a
 * create or delete shifts pagination and counts on list pages the user has not
 * visited yet — invalidating only the current page leaves those stale. Prefix
 * invalidation covers all of them in one call, and TanStack Query only
 * refetches the queries that are actually mounted.
 */
const useInvalidateGateways = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: gatewayKeys.all(org) });
  };
};

export const useCreateGateway = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateGateways(orgId);

  return useMutation<Gateway, ApiError, CreateGatewayBody>({
    // `async` so a missing scope rejects the mutation rather than throwing
    // synchronously into the caller — the error belongs on `mutation.error`,
    // where the form already renders it.
    mutationFn: async (body) =>
      createGateway(body, { orgId: requireOrgScope(orgId, 'CreateGateway') }),
    onSuccess: (created) => {
      // Seed the detail cache from the create response so the setup flow the
      // user lands on renders instantly instead of showing a loading state for
      // data the server just gave us. `id` is optional in the spec, so it is
      // guarded rather than assumed.
      if (org && created.id) {
        queryClient.setQueryData(gatewayKeys.detail(org, created.id), created);
      }
      invalidate();
    },
  });
};

export const useUpdateGateway = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateGateways(orgId);

  return useMutation<
    Gateway,
    ApiError,
    { gatewayId: string; body: UpdateGatewayBody }
  >({
    mutationFn: async ({ gatewayId, body }) =>
      updateGateway(gatewayId, body, {
        orgId: requireOrgScope(orgId, 'UpdateGateway'),
      }),
    // No optimistic write here, unlike projects: the response carries
    // connection state the client cannot predict, and a gateway edit is a
    // deliberate, low-frequency action rather than inline typing.
    onSettled: () => invalidate(),
  });
};

export const useDeleteGateway = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateGateways(orgId);

  return useMutation<void, ApiError, { gatewayId: string }>({
    mutationFn: async ({ gatewayId }) =>
      deleteGateway(gatewayId, { orgId: requireOrgScope(orgId, 'DeleteGateway') }),
    onSuccess: (_result, { gatewayId }) => {
      if (org) {
        // Drops the gateway's tokens and manifest with it: both are filed
        // beneath this detail key, so one removal covers all three instead of
        // refetching a deleted resource just to receive a 404.
        queryClient.removeQueries({ queryKey: gatewayKeys.detail(org, gatewayId) });
      }
      invalidate();
    },
  });
};

/**
 * The gateway's deployment manifest — what the platform believes this gateway
 * should currently be running, including every policy installed on it.
 *
 * No `poll` option, unlike `useGateway`: a manifest changes when someone
 * deploys, not on its own, so it is read when the view that needs it mounts
 * and left alone after that.
 */
export const useGatewayManifest = (
  gatewayId: string | undefined,
  overrides: { orgId?: string } = {}
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...gatewayQueries.manifest(org!, gatewayId!),
    enabled: Boolean(org && gatewayId),
  });
};

/**
 * Issues a fresh registration token for a gateway.
 *
 * The plaintext token comes back in this response and nowhere else — it is
 * never readable again — so the caller must render it immediately rather than
 * relying on a refetch. Nothing is written into the query cache for that
 * reason: the token belongs to the moment, not to the cache.
 *
 * Rotating revokes the gateway's previous token, which disconnects an agent
 * still using it. Both the token list and the gateway itself are invalidated,
 * since `isActive` follows that disconnection.
 */
export const useRotateGatewayToken = (
  gatewayId: string,
  overrides: { orgId?: string } = {}
) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();

  return useMutation<TokenRotationResponse, ApiError, void>({
    mutationFn: async () =>
      rotateGatewayToken(gatewayId, {
        orgId: requireOrgScope(orgId, 'RotateGatewayToken'),
      }),
    onSuccess: () => {
      if (!org) return;
      void queryClient.invalidateQueries({
        queryKey: gatewayKeys.children(org, gatewayId, 'tokens'),
      });
      void queryClient.invalidateQueries({
        queryKey: gatewayKeys.detail(org, gatewayId),
      });
    },
  });
};

/**
 * Selector for gateway pickers (deploy targets, filters), which only need an
 * id and a label. `select` runs after the cache read, so a component using
 * this re-renders only when the derived options change — not when an unrelated
 * field on some gateway does.
 */
export const useGatewayOptions = (filters: GatewayListFilters = {}) => {
  const { org } = useApiScope();

  return useQuery({
    ...gatewayQueries.list(org!, filters),
    enabled: Boolean(org),
    select: (data: GatewayListResponse) =>
      (data.list ?? []).map((gateway) => ({
        id: gateway.id ?? '',
        label: gateway.displayName,
      })),
  });
};
