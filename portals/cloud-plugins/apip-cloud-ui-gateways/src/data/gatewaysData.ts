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

import type { ApiFetch } from '../hostPort';
import type { Gateway, GatewayType } from '../types';

/**
 * The list is a JOIN of two resources, because neither answers the whole
 * question:
 *
 *   GET /gateways          every gateway the organization has, self-hosted and
 *                          WSO2-managed alike, with the live `isActive` flag and
 *                          the registered `functionalityType`. Carries no
 *                          environment.
 *   GET /managed-gateways  the gateway-to-environment bindings — i.e. exactly
 *                          which gateways are WSO2-managed, and the only
 *                          authoritative source of a gateway's environment.
 *
 * The binding list is what gates the configuration action: a gateway with no
 * binding row has no cloud-managed configuration and that path answers 404, so
 * the icon must not be offered rather than offered and then failed.
 */

/** `GatewayResponse`, only the fields this list reads. */
type NativeGateway = {
  id: string;
  displayName?: string;
  description?: string;
  endpoints?: string[];
  functionalityType?: GatewayType;
  isActive?: boolean;
  updatedAt?: string;
};

type NativeGatewayList = { list?: NativeGateway[] };
type BindingList = { list?: Array<{ id: string; environment: string }> };

/**
 * The environment a gateway lives in, recovered from its id for a gateway with
 * no binding row. Ids are `wc-<org>-<env>-apip-<name>-gw`, so the environment
 * is the last segment before `-apip-`. A display fallback only — it does not
 * make the row manageable.
 */
export function environmentFromGatewayId(id: string): string {
  const head = id.split('-apip-')[0];
  if (head === id) return '';
  const parts = head.split('-');
  return parts[parts.length - 1] ?? '';
}

/**
 * The gateway rows, plus whether the managed-gateway half of the join could be
 * read at all.
 */
export type GatewayListing = {
  gateways: Gateway[];
  /**
   * `/managed-gateways` did not answer, so no row can be known to be managed
   * and every one reports `isManaged: false`. The caller must say so rather
   * than let the configuration affordance quietly disappear.
   */
  managedUnavailable: boolean;
};

export async function listGateways(
  apiFetch: ApiFetch
): Promise<GatewayListing> {
  // The binding list is an ENRICHMENT: it decides which rows are configurable,
  // while `/gateways` decides which rows exist. So its failure must not take
  // the page down with it -- a caller without the managed-gateway permission,
  // or an outage on that one route, would otherwise see no gateways at all
  // rather than the list it is entitled to.
  const [gateways, bindings] = await Promise.all([
    apiFetch<NativeGatewayList>('GET', '/gateways'),
    apiFetch<BindingList>('GET', '/managed-gateways').catch(() => null),
  ]);

  const environmentById = new Map(
    (bindings?.list ?? []).map((binding) => [binding.id, binding.environment])
  );

  // Annotated: without a contextual type the ternaries below widen to `string`
  // and stop matching the union.
  const rows: Gateway[] = (gateways.list ?? []).map((native) => ({
    description: native.description,
    environmentId:
      environmentById.get(native.id) ?? environmentFromGatewayId(native.id),
    id: native.id,
    isManaged: environmentById.has(native.id),
    name: native.displayName || native.id,
    status: native.isActive ? 'active' : 'inactive',
    // The field is required on create and defaults to `regular` in the schema,
    // so an absent one is a wire anomaly and `regular` is what it would be.
    type: native.functionalityType ?? 'regular',
    updatedAt: native.updatedAt ?? '',
    url: native.endpoints?.[0] ?? '',
  }));

  return { gateways: rows, managedUnavailable: bindings === null };
}
