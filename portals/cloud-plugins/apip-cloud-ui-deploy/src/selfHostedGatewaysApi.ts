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

import type { ApiFetch, GatewayVersionEntry } from './hostPort';

export type SelfHostedGatewayType = 'regular' | 'ai';

export const DEFAULT_SELF_HOSTED_NAME = 'Default Self-Hosted Gateway';

/** The default gateway's display name; an AI gateway is named for its type so it does not shadow the regular one. */
export const defaultSelfHostedName = (type: SelfHostedGatewayType): string =>
  type === 'ai' ? 'Default AI Self-Hosted Gateway' : DEFAULT_SELF_HOSTED_NAME;
const DEFAULT_SELF_HOSTED_DESCRIPTION = 'Default API Platform Self-Hosted Gateway';

/**
 * platform-api's native create needs at least one endpoint, and a self-hosted
 * gateway has none until its own controller dials in. Must match the Gateways
 * plugin's placeholder (`PENDING_ENDPOINT` in its `selfHostedGatewaysApi`), which
 * hides it from the gateway's displayed URL.
 */
export const PENDING_ENDPOINT = 'pending://awaiting-connection';

/** The gateways table's handle column holds 40 characters. */
const MAX_HANDLE_LENGTH = 40;

/** The latest release of a gateway type: the first entry of that type in the host's config. */
export const latestVersionFor = (
  versions: GatewayVersionEntry[] | undefined,
  type: SelfHostedGatewayType
): GatewayVersionEntry | undefined => versions?.find((entry) => entry.type === type);

/** The latest regular-gateway release. */
export const latestRegularVersion = (versions: GatewayVersionEntry[] | undefined) =>
  latestVersionFor(versions, 'regular');

const slugify = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '');

/** A short, stable fingerprint of the full input, so truncated names stay distinct. */
const fingerprint = (value: string): string => {
  let hash = 5381;
  for (let index = 0; index < value.length; index += 1) {
    hash = ((hash << 5) + hash + value.charCodeAt(index)) >>> 0;
  }
  return hash.toString(36).padStart(6, '0').slice(-6);
};

/**
 * `<environment>-<name>`, as the Gateways page derives it. When that does not fit the
 * 40-character column the environment is shortened and a fingerprint of the complete
 * environment name is added, so two environments sharing a long prefix keep distinct ids.
 */
export const deriveHandle = (name: string, environment: string): string => {
  const slug = slugify(name);
  const env = slugify(environment) || 'env';
  const full = `${env}-${slug}`;
  if (full.length <= MAX_HANDLE_LENGTH) return full;
  const tag = fingerprint(`${environment}|${name}`);
  const room = MAX_HANDLE_LENGTH - slug.length - tag.length - 2;
  if (room < 1) return `${tag}-${slug}`.slice(0, MAX_HANDLE_LENGTH).replace(/-+$/, '');
  return `${env.slice(0, room).replace(/-+$/, '')}-${tag}-${slug}`;
};

/**
 * Registers and sets up a self-hosted gateway. The calls are the Gateways page's
 * own: platform-api's native `/gateways` resource, with the environment and the
 * default marking carried in `properties`.
 */
export function createSelfHostedGatewaysClient(apiFetch: ApiFetch) {
  const generateToken = async (gatewayId: string): Promise<string> => {
    const response = await apiFetch<{ token?: string }>(
      'POST',
      `/gateways/${encodeURIComponent(gatewayId)}/tokens`,
      {}
    );
    if (!response?.token) {
      throw new Error('The gateway registration service did not return a token.');
    }
    return response.token;
  };

  return {
    generateToken,

    /**
     * Creates the default self-hosted gateway in `environment` at `version` (a
     * `major.minor`-style release from the host's config; omitted when the host has
     * none, so platform-api's own default applies) and mints its first token.
     * `token` is null only when minting failed after the create succeeded.
     */
    async createDefaultGateway(
      environment: string,
      version?: string,
      type: SelfHostedGatewayType = 'regular'
    ): Promise<{ id: string; token: string | null }> {
      const name = defaultSelfHostedName(type);
      const created = await apiFetch<{ id?: string }>('POST', '/gateways', {
        id: deriveHandle(name, environment),
        displayName: name,
        description: DEFAULT_SELF_HOSTED_DESCRIPTION,
        endpoints: [PENDING_ENDPOINT],
        functionalityType: type,
        isCritical: false,
        ...(version ? { version } : {}),
        properties: { environment, isDefault: 'true' },
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
      return { id: created.id, token };
    },
  };
}
