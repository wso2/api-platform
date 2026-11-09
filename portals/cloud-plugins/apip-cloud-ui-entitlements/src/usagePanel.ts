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
  GATEWAYS,
  GATEWAY_METRICS,
  HYBRID_GATEWAYS,
  INFRASTRUCTURE_METRICS,
  MANAGED_INTERFACE_METRICS,
  METRIC_LABELS,
  byCode,
  displayQuotaOf,
  productOf,
  usedOf,
} from './entitlementLimits';
import type { Entitlement, EntitlementSummary } from './entitlementTypes';

/** Mirrors oxygen-ui's `TrialBadgeUsageItem`, without importing the UI package. */
export type UsageItem = {
  key: string;
  label: string;
  count: number;
  limit: number;
};

/**
 * Mirrors oxygen-ui's `TrialBadgeUsageGroup`: a run of rows inside a section,
 * under a sub-heading when `title` is set, plain otherwise.
 */
export type UsageGroup = {
  key: string;
  title?: string;
  items: UsageItem[];
};

/** Mirrors oxygen-ui's `TrialBadgeUsageSection`. */
export type UsageSection = {
  key: string;
  title: string;
  /** Rows directly under the section heading, drawn above any `groups`. */
  items?: UsageItem[];
  groups?: UsageGroup[];
  /** Shown after the section total; omitted to suppress the total entirely. */
  totalSuffix?: string;
};

export type UsagePanel = {
  sections: UsageSection[];
};

/**
 * A row is worth drawing only when the meter names a quota: the soft band's
 * starting point, or the block point. Unlimited, uncapped and unresolved
 * meters all arrive without one, and rendering them would invent a ceiling
 * billing never set.
 */
const quotaOf = (entitlement: Entitlement | undefined): number | undefined =>
  entitlement ? displayQuotaOf(entitlement) : undefined;

type RowsSpec = {
  codes: readonly string[];
  /**
   * Shorter labels for rows drawn under a sub-heading. The component indents
   * a grouped row out of its `noWrap` label column, so "Managed Gateways"
   * truncates there; the sub-heading already says what kind of row it is.
   */
  labels?: Record<string, string>;
};

/**
 * The panel's sections, in display order.
 *
 * A section is totalled only when summing its rows means something. The
 * component totals a section by adding up every row it draws, grouped ones
 * included, and these meters are capped independently of one another — a
 * managed-gateway allowance plus a self-hosted one plus a portal is a ceiling
 * nothing enforces — so Infrastructure carries no total and is just a heading
 * over related rows. Managed Interfaces holds a single metered row, so its
 * total is that row.
 *
 * Inside Infrastructure, the gateway kinds sit under a "Gateways" sub-heading
 * and Portals follows as a plain row. The component always draws a section's
 * own `items` first, so a plain row below a titled group goes in a trailing
 * untitled group instead.
 */
const SECTIONS: ReadonlyArray<
  RowsSpec & {
    key: string;
    title: string;
    groups?: ReadonlyArray<RowsSpec & { key: string; title?: string }>;
    /** "used", not "left": the figure beside the title is what was consumed. */
    totalSuffix?: string;
  }
> = [
  {
    key: 'managed-interfaces',
    title: 'Managed Interfaces',
    codes: MANAGED_INTERFACE_METRICS,
    totalSuffix: 'used',
  },
  {
    key: 'infrastructure',
    title: 'Infrastructure',
    codes: [],
    groups: [
      {
        key: 'gateways',
        title: 'Gateways',
        codes: GATEWAY_METRICS,
        labels: { [GATEWAYS]: 'WSO2 Managed', [HYBRID_GATEWAYS]: 'Self-Hosted' },
      },
      { key: 'portals', codes: INFRASTRUCTURE_METRICS },
    ],
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

  const entitlements = byCode(product);

  // Only capped meters: an omitted or unlimited one is absent, not zero,
  // and rendering it would invent a cap billing never set.
  const rowsOf = ({ codes, labels }: RowsSpec): UsageItem[] =>
    codes.flatMap((code) => {
      const entitlement = entitlements.get(code);
      const quota = quotaOf(entitlement);
      if (quota === undefined) return [];
      return [
        {
          key: code,
          label: labels?.[code] ?? METRIC_LABELS[code] ?? code,
          count: usedOf(entitlement),
          limit: quota,
        },
      ];
    });

  const sections: UsageSection[] = [];

  for (const spec of SECTIONS) {
    const items = rowsOf(spec);
    // A group, or a section, whose every meter is uncapped or absent is left
    // out entirely, rather than shown as an empty heading.
    const groups: UsageGroup[] = (spec.groups ?? []).flatMap((group) => {
      const rows = rowsOf(group);
      return rows.length > 0 ? [{ key: group.key, title: group.title, items: rows }] : [];
    });
    if (items.length === 0 && groups.length === 0) continue;
    sections.push({
      key: spec.key,
      title: spec.title,
      ...(items.length > 0 ? { items } : {}),
      ...(groups.length > 0 ? { groups } : {}),
      totalSuffix: spec.totalSuffix,
    });
  }

  return sections.length > 0 ? { sections } : null;
}
