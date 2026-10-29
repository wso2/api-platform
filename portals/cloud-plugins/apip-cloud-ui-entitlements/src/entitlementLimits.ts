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

/**
 * Billing metric codes → the host's product nouns.
 *
 * **All of the commercial knowledge lives here.** The host also ships on-prem
 * and OSS and must never learn any of it, so it is handed already-effective
 * `{max, used, message}` per product noun and nothing else. A plan change
 * touches only this file.
 */

import type { Entitlement, EntitlementSummary, ProductEntitlement } from './entitlementTypes';

/** The product this console's subscription is for. */
export const PRODUCT = 'api-platform';

/** No ceiling, in the host's vocabulary. The API says `null`; the host says -1. */
const UNLIMITED = -1;

/** The APIs meter. */
const APIS = 'apis';

/** Self-contained meter: self-hosted gateways have no shared budget. */
const HYBRID_GATEWAYS = 'hybrid_gateways';

/** Self-contained meter: developer portals have no shared budget either. */
const API_DEVELOPER_PORTALS = 'api_developer_portals';

/**
 * The caps the panel lists, in order. Each is independent, so no two of these
 * figures may ever be summed.
 */
const STANDALONE_METRICS = [HYBRID_GATEWAYS, API_DEVELOPER_PORTALS] as const;

/** Display names for the caps we may need to name in a tooltip. */
const METRIC_LABELS: Record<string, string> = {
  [APIS]: 'APIs',
  [HYBRID_GATEWAYS]: 'Gateways',
  [API_DEVELOPER_PORTALS]: 'Portals',
};

/** What the host understands. Mirrors its own `LimitedComponent` union. */
export type LimitedComponent = 'restApis' | 'gateways' | 'portals';

export type ResourceLimit = { max: number; used: number; message?: string };
export type ResourceLimitSet = Partial<Record<LimitedComponent, ResourceLimit>>;

/** The `api-platform` product's slice of a summary, or `undefined`. */
export function productOf(
  summary: EntitlementSummary | null | undefined
): ProductEntitlement | undefined {
  const products = summary?.products ?? [];
  // Billing returns exactly one product for a `?product=` query, but match by
  // code anyway rather than taking [0] — a future multi-product response would
  // otherwise silently supply another product's caps.
  return products.find((p) => p.product === PRODUCT) ?? (products.length === 1 ? products[0] : undefined);
}

function byCode(product: ProductEntitlement | undefined): Map<string, Entitlement> {
  const map = new Map<string, Entitlement>();
  for (const e of product?.entitlements ?? []) {
    if (e?.name) map.set(e.name, e);
  }
  return map;
}

/**
 * How much headroom a meter has, as a number `min()` can compare.
 *
 * `Infinity` means unlimited — both an explicit `null` remaining and an absent
 * metric. An absent metric is *not* a cap: billing omits meters a plan does not
 * define, and treating that as zero would block creation outright.
 */
function remainingOf(entitlement: Entitlement | undefined): number {
  if (!entitlement) return Infinity;
  if (entitlement.remaining === null) return Infinity;
  if (typeof entitlement.remaining === 'number') {
    // An organization grandfathered above a lowered limit reports negative.
    return Math.max(0, entitlement.remaining);
  }
  // No `remaining`: derive it, still treating a null/absent limit as unlimited.
  if (entitlement.limit === null || entitlement.limit === undefined) return Infinity;
  return Math.max(0, entitlement.limit - (entitlement.used ?? 0));
}

/** The host's `{max, used}` for one meter, with `null` limit becoming -1. */
function toLimit(entitlement: Entitlement): ResourceLimit {
  const unlimited = entitlement.limit === null || entitlement.limit === undefined;
  return {
    max: unlimited ? UNLIMITED : entitlement.limit!,
    used: entitlement.used ?? 0,
  };
}

/**
 * One meter's cap, as the host's `{max, used, message}`. `undefined` when
 * nothing caps this type; an unlimited meter carries no message.
 */
function bindingLimit(
  entitlements: Map<string, Entitlement>,
  code: string
): ResourceLimit | undefined {
  const entitlement = entitlements.get(code);
  if (!entitlement) return undefined;

  if (remainingOf(entitlement) === Infinity) {
    return { max: UNLIMITED, used: entitlement.used ?? 0 };
  }

  const limit = toLimit(entitlement);
  const label = METRIC_LABELS[code];
  return label
    ? {
        ...limit,
        message:
          `You cannot create more because your organization has reached ` +
          `the maximum of ${limit.max} ${label}.`,
      }
    : limit;
}

/**
 * The limit set the host is handed, in its own vocabulary.
 *
 * `null` means "unknown" — no summary, or none for this product — and the host
 * reads that as nothing capped. Never a lockout: a wrong "allowed" costs a
 * failed submit, while a wrong "denied" would lock a user out of their own
 * product, and the control plane refuses an over-limit create regardless.
 */
export function toResourceLimits(
  summary: EntitlementSummary | null | undefined
): ResourceLimitSet | null {
  const product = productOf(summary);
  if (!product) return null;

  const entitlements = byCode(product);
  const limits: ResourceLimitSet = {};

  const restApis = bindingLimit(entitlements, APIS);
  if (restApis) limits.restApis = restApis;

  const gateways = bindingLimit(entitlements, HYBRID_GATEWAYS);
  if (gateways) limits.gateways = gateways;

  const portals = bindingLimit(entitlements, API_DEVELOPER_PORTALS);
  if (portals) limits.portals = portals;

  return Object.keys(limits).length > 0 ? limits : null;
}

/**
 * Whether the organization may write at all.
 *
 * Decided by billing (`entitlements_active`), mirrored here. An absent verdict
 * reads as *not* read-only — an older billing deployment, or a failed read,
 * must never lock the console.
 */
export function isReadOnly(summary: EntitlementSummary | null | undefined): boolean {
  const product = productOf(summary);
  if (!product?.subscription) return false;
  return product.subscription.entitlements_active === false;
}

/** The meters the usage panel lists, in the order it lists them. */
export const USAGE_METRICS = [APIS, ...STANDALONE_METRICS] as const;
export { METRIC_LABELS, APIS, HYBRID_GATEWAYS, API_DEVELOPER_PORTALS, STANDALONE_METRICS };
