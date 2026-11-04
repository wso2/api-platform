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
 * The self-hosted-gateway data client, beside `gatewaysApi.ts` (which wraps
 * `/managed-gateways`). It talks to apip-platform-api's cloud
 * `/self-hosted-gateways` resource rather than platform-api's native `/gateways`,
 * whose create and token routes are internal-only on cloud.
 *
 * The resource takes the environment as its own field and binds the gateway to
 * it server-side, and it only ever answers for self-hosted gateways, so the list
 * needs no filtering against `/managed-gateways`. A self-hosted gateway is never
 * its environment's default: only a WSO2-managed gateway is what an environment
 * resolves to.
 */

import type { ApiFetch } from './hostPort';
import type { Gateway, GatewayType, SelfHostedGatewayInput } from './types';

const SELF_HOSTED_PATH = '/self-hosted-gateways';

/** A `/self-hosted-gateways` record: the platform-api gateway record plus its bound environment. */
type SelfHostedGatewayDTO = {
  id: string;
  displayName?: string;
  description?: string;
  endpoints?: string[];
  functionalityType?: string;
  version?: string;
  isCritical?: boolean;
  isActive?: boolean;
  environment?: string;
  createdAt?: string;
  updatedAt?: string;
};

type GatewayTokenResponse = {
  id?: string;
  token?: string;
};

type GatewayTokenListResponse = {
  list?: { id?: string; status?: string }[];
};

/**
 * No real address exists for a self-hosted gateway until its own controller
 * dials in and reports one — but platform-api requires at least one endpoint up
 * front. This unroutable placeholder fills that slot; `mapGateway` never
 * surfaces it as the gateway's URL.
 */
export const PENDING_ENDPOINT = 'pending://awaiting-connection';

const normalizeType = (functionalityType?: string): GatewayType =>
  functionalityType === 'ai' || functionalityType === 'event' ? functionalityType : 'regular';

const mapGateway = (dto: SelfHostedGatewayDTO): Gateway => ({
  id: dto.id,
  name: dto.displayName || dto.id,
  description: dto.description,
  type: normalizeType(dto.functionalityType),
  environmentId: dto.environment ?? '',
  hosting: 'self-hosted',
  url: (dto.endpoints ?? []).find((endpoint) => endpoint !== PENDING_ENDPOINT) ?? '',
  // No provisioning step exists for a self-hosted gateway to be mid-way
  // through, so its status is a straight read of connectivity: `active` once
  // its own controller has dialled in with the registration token, `inactive`
  // until then (never `provisioning` or `failed`, which both describe WSO2
  // standing up infrastructure this gateway does not have).
  status: dto.isActive ? 'active' : 'inactive',
  isCritical: dto.isCritical ?? false,
  isDefault: false,
  version: dto.version,
  createdAt: dto.createdAt ?? '',
  updatedAt: dto.updatedAt ?? '',
});

/** The gateway's own path, with the id encoded. */
const gatewayPath = (gatewayId: string) => `${SELF_HOSTED_PATH}/${encodeURIComponent(gatewayId)}`;

export function createSelfHostedGatewaysClient(apiFetch: ApiFetch) {
  /** Lists the self-hosted gateways of the given kinds, or every kind when none are given. */
  const listGateways = async (types: GatewayType[] = []): Promise<Gateway[]> => {
    const query =
      types.length > 0 ? `?functionalityType=${types.map(encodeURIComponent).join(',')}` : '';
    const response = await apiFetch<{ list?: SelfHostedGatewayDTO[] }>('GET', `${SELF_HOSTED_PATH}${query}`);
    return (response?.list ?? []).map(mapGateway);
  };

  /** The ids of the gateway's active registration tokens. */
  const listActiveTokenIds = async (gatewayId: string): Promise<string[]> => {
    const response = await apiFetch<GatewayTokenListResponse>('GET', `${gatewayPath(gatewayId)}/tokens`);
    return (response?.list ?? [])
      .filter((token) => token.id && (!token.status || token.status === 'active'))
      .map((token) => token.id as string);
  };

  /**
   * Mints a new registration token. The token is shown exactly once, here, in
   * the response — platform-api stores only its hash, so there is no "view it
   * again" later, only "reconfigure".
   */
  const generateToken = async (gatewayId: string): Promise<string> => {
    const response = await apiFetch<GatewayTokenResponse>('POST', `${gatewayPath(gatewayId)}/tokens`, {});
    if (!response?.token) {
      throw new Error('The gateway registration service did not return a token.');
    }
    return response.token;
  };

  return {
    listGateways,
    generateToken,

    /** Whether the gateway has any active registration token. */
    async hasActiveToken(gatewayId: string): Promise<boolean> {
      return (await listActiveTokenIds(gatewayId)).length > 0;
    },

    /**
     * Revokes the gateway's existing registration tokens, then mints a new one.
     * Revoking first is what makes this a reconfigure: a gateway still running
     * on an old token loses its connection. If the new token cannot be minted
     * after the revoke, the gateway is left with none, and the next visit to its
     * setup steps generates one.
     */
    async reconfigureToken(gatewayId: string): Promise<string> {
      const existing = await listActiveTokenIds(gatewayId);
      for (const tokenId of existing) {
        await apiFetch('DELETE', `${gatewayPath(gatewayId)}/tokens/${encodeURIComponent(tokenId)}`);
      }
      return generateToken(gatewayId);
    },

    /**
     * Registers a self-hosted gateway in its environment and immediately mints
     * its first registration token, so the caller can open its setup steps with
     * the token in hand. `token` is `null` only if minting failed after an
     * otherwise successful create — the gateway still exists, and its setup
     * steps generate a token when opened.
     */
    async createGateway(input: SelfHostedGatewayInput): Promise<{ gateway: Gateway; token: string | null }> {
      const url = input.url?.trim();
      const created = await apiFetch<SelfHostedGatewayDTO>('POST', SELF_HOSTED_PATH, {
        displayName: input.name,
        description: input.description,
        environment: input.environmentId,
        functionalityType: input.type,
        endpoints: [url || PENDING_ENDPOINT],
        // `major.minor` (platform-api's own format) — a declared hint, not a
        // provisioned fact; a connecting controller may later report a
        // different one. Omitted (not an empty string) when unset, so
        // platform-api's own "1.0" default applies instead.
        ...(input.version ? { version: input.version } : {}),
      });
      if (!created?.id) {
        throw new Error('The gateway registration service returned an incomplete response.');
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
     * Only display name and description are editable; environment and type are
     * fixed at creation, same as `/managed-gateways`. Properties are not sent,
     * so the server keeps them as they are.
     */
    async updateGateway(id: string, input: SelfHostedGatewayInput): Promise<void> {
      await apiFetch('PUT', gatewayPath(id), {
        displayName: input.name,
        description: input.description,
      });
    },

    async deleteGateway(id: string): Promise<void> {
      await apiFetch('DELETE', gatewayPath(id));
    },
  };
}
