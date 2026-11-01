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

/** One quota's current state. */
export type Entitlement = {
  /**
   * The **metric code** — `apis`, `managed_interfaces`, `hybrid_gateways` —
   * not a display label, despite the field being called `name`. Billing fills
   * it from the plan resource's `Code`, so it matches the seed's `code` column.
   */
  name: string;
  /** Always `quota` on this route today; the handler hardcodes it. */
  type?: 'feature' | 'quota';
  description?: string;
  enabled?: boolean;
  /** `null` means unlimited. Absent when billing could not determine one. */
  limit?: number | null;
  /**
   * Current count. Billing reports this best-effort: on a metering read error
   * it sends 0 rather than failing the whole summary, so a 0 here is not proof
   * that nothing exists.
   */
  used?: number;
  /** `null` means unlimited. */
  remaining?: number | null;
  period?: string;
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
