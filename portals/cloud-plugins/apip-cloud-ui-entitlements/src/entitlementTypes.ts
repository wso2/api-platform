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
 * `GET /entitlement/api/v1/summary?product=api-platform`, as the billing
 * service actually answers it. Hand-mirrored from
 * `billing-entitlement-api.v1.yaml` rather than generated, because this plugin
 * lives in a different repo from that spec.
 */

/**
 * How much of a quota is gone, as billing resolved it — after any override,
 * payment unlock and soft band. Fields that do not apply are **omitted**
 * rather than sent as null or zero, so an absent one says "no such thing
 * here", never "zero".
 */
export type QuotaStatus = {
  /** Current level, or the period total for a usage meter. */
  used: number;
  /** Where overage starts, when `enforcement.mode` is `soft`. */
  soft_limit?: number;
  /**
   * The block point: the quota in hard mode, the ceiling in soft mode.
   * Absent when there is none — unlimited, or soft with no ceiling.
   */
  hard_limit?: number;
  /** `hard_limit - used`, clamped at 0. Absent when there is no block point. */
  remaining?: number;
  /** Absent when unlimited, or when the quota is 0. */
  utilization_pct?: number;
  /** `max(used - quota, 0)`. Absent when 0. */
  overage?: number;
};

/** A warning threshold and whether the current value has crossed it. */
export type QuotaThreshold = {
  /** The threshold, as a percentage of the quota. */
  pct: number;
  breached: boolean;
};

/** The enforcement policy in effect for one quota. */
export type QuotaEnforcement = {
  /**
   * `hard` blocks at the quota, `soft` allows usage past it up to the ceiling
   * in `quota.hard_limit`, `unlimited` enforces nothing. This is the only
   * trustworthy statement of "unlimited" — an absent `hard_limit` alone could
   * equally mean billing resolved no limit at all.
   */
  mode: 'hard' | 'soft' | 'unlimited';
  /** Where the limit comes from. */
  source?: 'plan' | 'override';
  /** Ascending. Absent when the resource has none. */
  thresholds?: QuotaThreshold[];
  /** Only when `mode` is `soft`: usage past the quota accrues charges. */
  overage_billable?: boolean;
};

/** One quota's current state. */
export type Entitlement = {
  /**
   * The **metric code** — `apis`, `hybrid_gateways`, `api_developer_portals` —
   * not a display label, despite the field being called `name`. Billing fills
   * it from the plan resource's `Code`, so it matches the seed's `code` column.
   */
  name: string;
  /** Always `quota` on this route today; the handler hardcodes it. */
  type?: 'feature' | 'quota';
  description?: string;
  enabled?: boolean;
  /**
   * Current count, the same value as `quota.used` whenever `quota` is present.
   * Billing reports this best-effort: on a metering read error it sends 0
   * rather than failing the whole summary, so a 0 here is not proof that
   * nothing exists.
   */
  used?: number;
  period?: string;
  /**
   * The limits. Absent when billing resolved none, which is **not** the same
   * as unlimited — read `enforcement.mode` for that.
   */
  quota?: QuotaStatus;
  enforcement?: QuotaEnforcement;
};

export type SubscriptionInfo = {
  plan?: string;
  plan_code?: string;
  plan_version?: number;
  /** `trial` | `active` | `past_due` | `suspended` | `cancelled` | … */
  status?: string;
  /**
   * Computed server-side as `status ∈ {active, trial, past_due}`. This is the
   * read-only verdict, already decided — there is no tier logic on this side.
   *
   * Note `past_due` counts as **active**, so an unsettled invoice does not make
   * an organization read-only. Surfacing that state needs its own banner.
   */
  entitlements_active?: boolean;
};

export type ProductEntitlement = {
  product?: string;
  subscription?: SubscriptionInfo;
  entitlements?: Entitlement[];
};

export type EntitlementSummary = {
  products?: ProductEntitlement[];
};
