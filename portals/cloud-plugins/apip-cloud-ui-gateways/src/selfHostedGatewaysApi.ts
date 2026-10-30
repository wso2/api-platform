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

/**
 * The self-hosted-gateway data client. New file, separate from `gatewaysApi.ts`:
 * that one wraps the cloud-only `/managed-gateways` resource (WSO2 provisions
 * and runs the gateway); this one talks straight to platform-api's NATIVE
 * `/gateways` resource, the same one `api-platform-1/portals/ai-workspace`
 * already registers self-hosted gateways through (see its
 * `src/apis/gateway/gatewayApi.ts` and `src/hooks/useGateway.ts`). Both base
 * paths are served by the same platform-api instance behind this host's
 * `apiFetch` — `/gateways` needs no cloud wrapper the way `/managed-gateways`
 * does, since a self-hosted gateway has no WSO2-side infrastructure to
 * provision.
 *
 * Two things a managed gateway gets for free have no server-side equivalent
 * here, and are approximated client-side instead:
 *
 *  - ENVIRONMENT: the native record has no environment field, so it is stashed
 *    in `properties.environment`, the same convention the AI workspace uses.
 *  - DEFAULT: there is no partial-unique-index invariant enforcing one default
 *    per (environment, type) the way the managed-gateway binding table has.
 *    `properties.isDefault` is a plain marker, kept unique on a best-effort
 *    basis by `clearOtherDefaults` below — see the note on `Gateway.isDefault`
 *    in `types.ts`.
 */

import type { ApiFetch } from './hostPort';
import { gatewayHandleFromName } from './utils/name';
import type { Gateway, GatewayType, SelfHostedGatewayInput } from './types';

const GATEWAYS_PATH = '/gateways';

/** platform-api's native gateway record (api.GatewayResponse), not the `/managed-gateways` DTO. */
type NativeGatewayDTO = {
  id: string;
  displayName?: string;
  description?: string;
  endpoints?: string[];
  functionalityType?: string;
  version?: string;
  isCritical?: boolean;
  isActive?: boolean;
  properties?: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
};

type NativeGatewayTokenResponse = {
  id?: string;
  token?: string;
  message?: string;
};

/**
 * No real address exists for a self-hosted gateway until its own controller
 * dials in and reports one — but platform-api's native create requires at
 * least one endpoint up front. This unroutable placeholder fills that slot;
 * `mapGateway` never surfaces it as the gateway's URL.
 */
export const PENDING_ENDPOINT = 'pending://awaiting-connection';

const normalizeType = (functionalityType?: string): GatewayType =>
  functionalityType === 'ai' || functionalityType === 'event' ? functionalityType : 'regular';

const stringProperty = (properties: Record<string, unknown> | undefined, key: string): string => {
  const value = properties?.[key];
  return typeof value === 'string' ? value : '';
};

const mapGateway = (dto: NativeGatewayDTO): Gateway => ({
  id: dto.id,
  name: dto.displayName || dto.id,
  description: dto.description,
  type: normalizeType(dto.functionalityType),
  environmentId: stringProperty(dto.properties, 'environment'),
  hosting: 'self-hosted',
  url: (dto.endpoints ?? []).find((endpoint) => endpoint !== PENDING_ENDPOINT) ?? '',
  // No provisioning step exists for a self-hosted gateway to be mid-way
  // through, so its status is a straight read of connectivity: `active` once
  // its own controller has dialled in with the registration token, `inactive`
  // until then (never `provisioning` or `failed`, which both describe WSO2
  // standing up infrastructure this gateway does not have).
  status: dto.isActive ? 'active' : 'inactive',
  isCritical: dto.isCritical ?? false,
  isDefault: stringProperty(dto.properties, 'isDefault') === 'true',
  version: dto.version,
  createdAt: dto.createdAt ?? '',
  updatedAt: dto.updatedAt ?? '',
});

/** `<environment>-<name>` — the same handle shape `/managed-gateways` derives server-side. */
const deriveHandle = (name: string, environmentId: string): string => {
  const handle = gatewayHandleFromName(name);
  if (!handle) return '';
  return environmentId ? `${environmentId}-${handle}` : handle;
};

export function createSelfHostedGatewaysClient(apiFetch: ApiFetch) {
  /** Lists the self-hosted gateways of the given kinds, or every kind when none are given. */
  const listGateways = async (types: GatewayType[] = []): Promise<Gateway[]> => {
    const query =
      types.length > 0 ? `?functionalityType=${types.map(encodeURIComponent).join(',')}` : '';
    const response = await apiFetch<{ list?: NativeGatewayDTO[] }>('GET', `${GATEWAYS_PATH}${query}`);
    return (response?.list ?? []).map(mapGateway);
  };

  /**
   * Mints a new registration token for an existing self-hosted gateway. The
   * token is shown exactly once, here, in the response — platform-api stores
   * only its hash, so there is no "view it again" later, only "generate a new
   * one" (which revokes whichever token this gateway had).
   */
  const generateToken = async (gatewayId: string): Promise<string> => {
    const response = await apiFetch<NativeGatewayTokenResponse>(
      'POST',
      `${GATEWAYS_PATH}/${encodeURIComponent(gatewayId)}/tokens`,
      {}
    );
    if (!response?.token) {
      throw new Error('The gateway registration service did not return a token.');
    }
    return response.token;
  };

  /**
   * Clears `properties.isDefault` on every OTHER self-hosted gateway of the
   * same environment and type. Best-effort, not transactional — there is no
   * server-side uniqueness constraint over this marker the way the
   * managed-gateway binding table has one over `is_default` — so a caller
   * that cares about the outcome should treat a failure here as cosmetic, not
   * fail the create/update it is part of over it.
   */
  const clearOtherDefaults = async (
    keepGatewayId: string,
    environmentId: string,
    type: GatewayType
  ): Promise<void> => {
    const siblings = await listGateways([type]);
    await Promise.all(
      siblings
        .filter(
          (gateway) =>
            gateway.id !== keepGatewayId && gateway.environmentId === environmentId && gateway.isDefault
        )
        .map((gateway) =>
          apiFetch('PUT', `${GATEWAYS_PATH}/${encodeURIComponent(gateway.id)}`, {
            displayName: gateway.name,
            properties: { environment: gateway.environmentId },
          })
        )
    );
  };

  return {
    listGateways,
    generateToken,

    /**
     * Registers a self-hosted gateway and immediately mints its first
     * registration token, so a caller that never shows a dedicated "here is
     * your token" screen can still offer it right away (stashed wherever the
     * caller likes) rather than sending the admin straight to a gateway with
     * no way to connect it. `token` is `null` only if minting failed after an
     * otherwise successful create — the gateway still exists, and a token can
     * be generated for it later from the list.
     */
    async createGateway(input: SelfHostedGatewayInput): Promise<{ gateway: Gateway; token: string | null }> {
      const handle = deriveHandle(input.name, input.environmentId);
      const url = input.url?.trim();
      const created = await apiFetch<NativeGatewayDTO>('POST', GATEWAYS_PATH, {
        id: handle || undefined,
        displayName: input.name,
        description: input.description,
        endpoints: [url || PENDING_ENDPOINT],
        functionalityType: input.type,
        isCritical: false,
        // `major.minor` (platform-api's own format) — a declared hint, not a
        // provisioned fact; a connecting controller may later report a
        // different one. Omitted (not an empty string) when unset, so
        // platform-api's own "1.0" default applies instead.
        ...(input.version ? { version: input.version } : {}),
        properties: {
          environment: input.environmentId,
          ...(input.isDefault ? { isDefault: 'true' } : {}),
        },
      });
      if (!created?.id) {
        throw new Error('The gateway registration service returned an incomplete response.');
      }
      if (input.isDefault) {
        try {
          await clearOtherDefaults(created.id, input.environmentId, input.type);
        } catch {
          /* best-effort, see clearOtherDefaults */
        }
      }
      let token: string | null = null;
      try {
        token = await generateToken(created.id);
      } catch {
        token = null;
      }
      return { gateway: mapGateway(created), token };
    },

    /**
     * Only display name, description and the default marking are editable;
     * environment and type are fixed at creation, same as `/managed-gateways`.
     */
    async updateGateway(id: string, input: SelfHostedGatewayInput): Promise<void> {
      await apiFetch('PUT', `${GATEWAYS_PATH}/${encodeURIComponent(id)}`, {
        displayName: input.name,
        description: input.description,
        properties: {
          environment: input.environmentId,
          ...(input.isDefault ? { isDefault: 'true' } : {}),
        },
      });
      if (input.isDefault) {
        try {
          await clearOtherDefaults(id, input.environmentId, input.type);
        } catch {
          /* best-effort, see clearOtherDefaults */
        }
      }
    },

    async deleteGateway(id: string): Promise<void> {
      await apiFetch('DELETE', `${GATEWAYS_PATH}/${encodeURIComponent(id)}`);
    },
  };
}
