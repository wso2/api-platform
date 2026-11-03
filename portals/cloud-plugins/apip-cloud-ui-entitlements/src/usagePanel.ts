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
 * The badge's hover panel: how much of each metered quota is used.
 *
 * Kept separate from the component so the arithmetic unit-tests without pulling
 * the UI library into a plain Node test run.
 */

import {
  GATEWAY_METRICS,
  INFRASTRUCTURE_METRICS,
  MANAGED_INTERFACE_METRICS,
  METRIC_LABELS,
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

const isCapped = (entitlement: Entitlement | undefined): entitlement is Entitlement =>
  Boolean(entitlement) &&
  entitlement!.limit !== null &&
  entitlement!.limit !== undefined;

/**
 * The panel's groups, in display order.
 *
 * A group is totalled only when summing its rows means something. The
 * component totals a section by adding its items up, and these meters are
 * capped independently of one another — a managed-gateway allowance plus a
 * self-hosted one is a ceiling nothing enforces — so those groups carry no
 * total and are just a heading over related rows. Managed Interfaces holds a single metered row,
 * so its total is that row.
 */
const GROUPS: ReadonlyArray<{
  key: string;
  title: string;
  codes: readonly string[];
  /** "used", not "left": the figure beside the title is what was consumed. */
  totalSuffix?: string;
}> = [
  { key: 'infrastructure', title: 'Infrastructure', codes: INFRASTRUCTURE_METRICS },
  { key: 'gateways', title: 'Gateways', codes: GATEWAY_METRICS },
  {
    key: 'managed-interfaces',
    title: 'Managed Interfaces',
    codes: MANAGED_INTERFACE_METRICS,
    totalSuffix: 'used',
  },
];

/**
 * Builds the hover panel, or `null` when there is nothing worth showing — no
 * summary, or an organization with no capped meter at all and so nothing to
 * count and nowhere to upgrade to.
 *
 * Only APIs are metered among the interface types — MCP, LLM and agent proxies
 * are not — so Managed Interfaces holds that one row and there is no shared
 * interfaces budget above it.
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

  const sections: UsageSection[] = [];

  for (const group of GROUPS) {
    // Only capped meters: an omitted or unlimited one is absent, not zero,
    // and rendering it would invent a cap billing never set.
    const items: UsageItem[] = group.codes.flatMap((code) => {
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
    // A group whose every meter is uncapped or absent is left out entirely,
    // rather than shown as an empty heading.
    if (items.length === 0) continue;
    sections.push({ key: group.key, title: group.title, items, totalSuffix: group.totalSuffix });
  }

  return sections.length > 0 ? { sections } : null;
}
