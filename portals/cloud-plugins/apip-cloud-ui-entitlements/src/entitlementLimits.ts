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

/** The metered interface type. MCP, LLM and agent proxies are not metered. */
const APIS = 'apis';

/** What an organization publishes through, rather than what it manages. */
const API_DEVELOPER_PORTALS = 'api_developer_portals';

/**
 * Two gateway meters, capped separately: WSO2 provisions and runs the first,
 * the customer runs the second on their own boxes. They cost differently, so
 * neither spends the other's allowance.
 */
const GATEWAYS = 'gateways';
const HYBRID_GATEWAYS = 'hybrid_gateways';

/** WSO2-managed and self-hosted gateways, each with its own cap. */
const GATEWAY_METRICS = [GATEWAYS, HYBRID_GATEWAYS] as const;

/**
 * The panel's groups, in display order. A plan that does not define one of
 * these simply omits it.
 *
 * No `projects` row: billing carries the quota but nothing reports the count,
 * so the row would read 0 of 5 for an organization that has projects. A wrong
 * figure is worse than no figure — put it back once the count is metered.
 */
const INFRASTRUCTURE_METRICS = [API_DEVELOPER_PORTALS] as const;

/** Managed interfaces: only APIs are metered. */
const MANAGED_INTERFACE_METRICS = [APIS] as const;

/** Display names for the caps we may need to name in a tooltip. */
const METRIC_LABELS: Record<string, string> = {
  [APIS]: 'APIs',
  [GATEWAYS]: 'Managed Gateways',
  [HYBRID_GATEWAYS]: 'Self-Hosted Gateways',
  [API_DEVELOPER_PORTALS]: 'Portals',
};

/** What the host understands. Mirrors its own `LimitedComponent` union. */
export type LimitedComponent =
  | 'restApis'
  | 'gateways'
  | 'managedGateways'
  | 'selfHostedGateways'
  | 'portals';

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
 * The cap to report for "may any of these be created": the meter with the MOST
 * headroom, since one full kind does not stop the other. The opposite of the
 * binding-cap rule, which answers a question about one specific kind.
 *
 * `undefined` when none of the codes is present. An unlimited meter wins
 * outright — `remainingOf` gives it `Infinity`.
 */
function roomiestLimitOf(
  entitlements: Map<string, Entitlement>,
  codes: readonly string[]
): ResourceLimit | undefined {
  let roomiestCode: string | undefined;
  let mostRemaining = -1;

  for (const code of codes) {
    const entitlement = entitlements.get(code);
    if (!entitlement) continue;
    const remaining = remainingOf(entitlement);
    if (roomiestCode === undefined || remaining > mostRemaining) {
      roomiestCode = code;
      mostRemaining = remaining;
    }
  }

  return roomiestCode === undefined ? undefined : bindingLimit(entitlements, roomiestCode);
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

  // The APIs meter alone: nothing else caps how many APIs may be created.
  const restApis = bindingLimit(entitlements, APIS);
  if (restApis) limits.restApis = restApis;

  // Each hosting kind has its own cap, so a surface that commits to one asks
  // about that one. `gateways` answers the broader question — may ANY gateway
  // be created — for a surface that only offers the choice, so it binds on
  // whichever kind has the most room left rather than the least.
  const managedGateways = bindingLimit(entitlements, GATEWAYS);
  if (managedGateways) limits.managedGateways = managedGateways;

  const selfHostedGateways = bindingLimit(entitlements, HYBRID_GATEWAYS);
  if (selfHostedGateways) limits.selfHostedGateways = selfHostedGateways;

  const anyGateway = roomiestLimitOf(entitlements, GATEWAY_METRICS);
  if (anyGateway) limits.gateways = anyGateway;

  const portals = bindingLimit(entitlements, API_DEVELOPER_PORTALS);
  if (portals) limits.portals = portals;

  return Object.keys(limits).length > 0 ? limits : null;
}

/**
 * Whether this subscription is on a trial plan. The plan code, not the status:
 * an ended trial reports `suspended`, which alone cannot tell it apart from a
 * paid subscription suspended for another reason.
 *
 * `endsWith` rather than an exact code, so this holds for any product.
 */
function isTrialPlan(planCode: string | undefined): boolean {
  return planCode?.trim().toLowerCase().endsWith('-trial') ?? false;
}

/**
 * Whether the organization may write at all: billing says entitlements are
 * inactive AND the subscription is still on a trial plan — a trial that ran
 * out, not a paid subscription suspended for another reason.
 *
 * Scoped to trial expiry on purpose. A paid organization suspended for
 * non-payment cannot write either, but saying so needs its own message, so
 * this says nothing rather than something wrong. Note `past_due` counts as
 * active, so an unsettled invoice does not reach here.
 *
 * Anything absent reads as *not* read-only — a failed read must never lock the
 * console.
 */
export function isReadOnly(summary: EntitlementSummary | null | undefined): boolean {
  const subscription = productOf(summary)?.subscription;
  if (!subscription) return false;
  return subscription.entitlements_active === false && isTrialPlan(subscription.plan_code);
}

/** The meters the usage panel lists, in the order it lists them. */
export const USAGE_METRICS = [
  ...INFRASTRUCTURE_METRICS,
  ...GATEWAY_METRICS,
  ...MANAGED_INTERFACE_METRICS,
] as const;
export {
  METRIC_LABELS,
  APIS,
  GATEWAYS,
  HYBRID_GATEWAYS,
  API_DEVELOPER_PORTALS,
  INFRASTRUCTURE_METRICS,
  GATEWAY_METRICS,
  MANAGED_INTERFACE_METRICS,
};
