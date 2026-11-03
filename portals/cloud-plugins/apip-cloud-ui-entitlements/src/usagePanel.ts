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
 * The badge's hover panel: how much of the interfaces budget is used, and what
 * used it.
 *
 * Kept separate from the component so the arithmetic unit-tests without pulling
 * the UI library into a plain Node test run.
 */

import {
  APIS,
  INTERFACE_TYPE_METRICS,
  MANAGED_INTERFACES,
  METRIC_LABELS,
  STANDALONE_METRICS,
  productOf,
} from './entitlementLimits';
import type { Entitlement, EntitlementSummary } from './entitlementTypes';

/** Mirrors oxygen-ui's `TrialBadgeUsageItem`, without importing the UI package. */
export type UsageItem = {
  key: string;
  label: string;
  count: number;
  limit: number;
};

/** Mirrors oxygen-ui's `TrialBadgeUsageSection`. */
export type UsageSection = {
  key: string;
  title: string;
  items: UsageItem[];
  /** Shown after the section total; omitted to suppress the total entirely. */
  totalSuffix?: string;
};

export type UsagePanel = {
  sections: UsageSection[];
};

const INTERFACES_LABEL = METRIC_LABELS[MANAGED_INTERFACES];

/**
 * A section header reads `sum(count) of sum(limit)`, so its items must add up.
 * The type breakdown does not — the types share one budget, so summing their
 * limits would claim a ceiling nothing enforces. Hence its own section, with
 * no total; the budget's own figure is the one above it.
 */
const BY_TYPE_SECTION_KEY = 'interfaces-by-type';

const isCapped = (entitlement: Entitlement | undefined): entitlement is Entitlement =>
  Boolean(entitlement) &&
  entitlement!.limit !== null &&
  entitlement!.limit !== undefined;

/**
 * Builds the hover panel, or `null` when there is nothing worth showing — no
 * summary, or an organization whose interfaces budget is unlimited and so has
 * nothing to count and nowhere to upgrade to.
 *
 * **One section per budget, deliberately.** The component totals a section by
 * summing its items, so two figures may only share a section when adding them
 * together means something — and none of these three may. APIs, self-hosted
 * gateways and developer portals are each capped independently of the others,
 * so a shared section would total unrelated budgets into a single figure that
 * describes nothing. One section each, every total its own.
 */
export function toUsagePanel(
  summary: EntitlementSummary | null | undefined
): UsagePanel | null {
  const product = productOf(summary);
  if (!product) return null;

  const byCode = new Map<string, Entitlement>();
  for (const e of product.entitlements ?? []) {
    if (e?.name) byCode.set(e.name, e);
  }

  // The shared budget is the headline; fall back to `apis` for a billing
  // deployment that does not report it, rather than showing no panel at all.
  const interfaces = byCode.get(MANAGED_INTERFACES) ?? byCode.get(APIS);
  // Unlimited (or absent): nothing to count. Matches the rule that hides the
  // upgrade affordance for an organization with no cap to hit.
  if (!isCapped(interfaces)) return null;

  const sections: UsageSection[] = [
    {
      key: MANAGED_INTERFACES,
      title: INTERFACES_LABEL,
      items: [
        {
          key: MANAGED_INTERFACES,
          label: INTERFACES_LABEL,
          count: interfaces.used ?? 0,
          limit: interfaces.limit!,
        },
      ],
      // "used", not "left": the figure beside it is what has been consumed.
      totalSuffix: 'used',
    },
  ];

  // What the budget was spent on. Only capped types: an omitted one is absent,
  // not zero, and rendering it would invent a cap billing never set.
  const byTypeItems = INTERFACE_TYPE_METRICS.flatMap((code) => {
    const entitlement = byCode.get(code);
    if (!isCapped(entitlement)) return [];
    return [
      {
        key: code,
        label: METRIC_LABELS[code] ?? code,
        count: entitlement.used ?? 0,
        limit: entitlement.limit!,
      },
    ];
  });

  if (byTypeItems.length > 0) {
    sections.push({
      key: BY_TYPE_SECTION_KEY,
      title: 'By type',
      items: byTypeItems,
      // No total — see BY_TYPE_SECTION_KEY.
    });
  }

  // Gateways and portals are capped independently of the APIs budget and of
  // each other, so neither belongs in the section above — creating a gateway
  // takes nothing from the APIs budget. One section each, rather than one
  // shared section, because a shared one would total two unrelated budgets
  // into a single meaningless figure.
  for (const code of STANDALONE_METRICS) {
    const entitlement = byCode.get(code);
    if (!isCapped(entitlement)) continue;
    const label = METRIC_LABELS[code] ?? code;
    sections.push({
      key: code,
      title: label,
      items: [
        {
          key: code,
          label,
          count: entitlement.used ?? 0,
          limit: entitlement.limit!,
        },
      ],
      totalSuffix: 'used',
    });
  }

  return { sections };
}
