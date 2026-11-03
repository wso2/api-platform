/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { describe, expect, it } from 'vitest';

import type { Entitlement, EntitlementSummary } from './entitlementTypes';
import { toUsagePanel } from './usagePanel';

const quota = (name: string, limit: number | null, used: number): Entitlement => ({
  name,
  type: 'quota',
  limit,
  used,
  remaining: limit === null ? null : Math.max(0, limit - used),
});

const summary = (entitlements: Entitlement[]): EntitlementSummary => ({
  products: [{ product: 'api-platform', entitlements }],
});

/** Trial: 5 managed interfaces, all of them spendable on APIs. */
const trial = (used: Record<string, number> = {}) =>
  summary([
    quota('managed_interfaces', 5, used.interfaces ?? used.apis ?? 0),
    quota('apis', 5, used.apis ?? 0),
  ]);

describe('toUsagePanel', () => {
  it('shows nothing when there is no summary', () => {
    expect(toUsagePanel(null)).toBeNull();
    expect(toUsagePanel({ products: [] })).toBeNull();
  });

  // Nothing to count and nowhere to upgrade to, so the badge falls back to a
  // plain countdown. The same rule that hides the upgrade affordance.
  it('shows nothing when the interfaces budget is unlimited', () => {
    expect(toUsagePanel(summary([quota('managed_interfaces', null, 9000)]))).toBeNull();
  });

  it('shows nothing when the interfaces budget is absent', () => {
    expect(toUsagePanel(summary([quota('hybrid_gateways', 1, 0)]))).toBeNull();
  });

  // An older billing deployment reports no managed_interfaces meter at all.
  // Showing the narrower meter beats showing nothing.
  it('falls back to the apis meter when the budget is not reported', () => {
    const panel = toUsagePanel(summary([quota('apis', 5, 2)]));
    expect(panel!.sections[0].title).toBe('Managed Interfaces');
    expect(panel!.sections[0].items[0].count).toBe(2);
  });

  // The shared budget, not the `apis` meter: every interface type spends it,
  // so it is the figure that runs out and the one the create button gates on.
  it('leads with the managed interfaces budget', () => {
    const panel = toUsagePanel(trial({ apis: 3 }));
    const headline = panel!.sections[0];
    expect(headline.title).toBe('Managed Interfaces');
    expect(headline.items).toEqual([
      { key: 'managed_interfaces', label: 'Managed Interfaces', count: 3, limit: 5 },
    ]);
    // The figure beside the title is what has been spent, not what remains.
    expect(headline.totalSuffix).toBe('used');
  });

  // What the budget was spent on, in its own section: these figures share one
  // ceiling, so summing them into a section total would claim a ceiling of 15.
  it('breaks the budget down by type, with no total of its own', () => {
    const panel = toUsagePanel(
      summary([
        quota('managed_interfaces', 10, 4),
        quota('apis', 5, 1),
        quota('mcp_proxies', 5, 2),
        quota('llm_proxies', 5, 1),
        quota('agent_proxies', 5, 0),
      ]),
    );
    const byType = panel!.sections[1];
    expect(byType.title).toBe('By type');
    expect(byType.totalSuffix).toBeUndefined();
    expect(byType.items.map((item) => item.label)).toEqual([
      'APIs',
      'MCP Proxies',
      'LLM Proxies',
      'Agent Proxies',
    ]);
  });

  // A type this plan does not cap is absent, not zero — rendering it would
  // invent a ceiling billing never set.
  it('lists only the types the plan actually caps', () => {
    const panel = toUsagePanel(
      summary([
        quota('managed_interfaces', 5, 1),
        quota('apis', 5, 1),
        quota('mcp_proxies', null, 0),
      ]),
    );
    expect(panel!.sections[1].items.map((item) => item.key)).toEqual(['apis']);
  });

  it('omits the breakdown entirely when no type is capped', () => {
    const panel = toUsagePanel(summary([quota('managed_interfaces', 5, 1)]));
    expect(panel!.sections.map((section) => section.key)).toEqual(['managed_interfaces']);
  });

  it('gives gateways and portals a section each, with their own totals', () => {
    const panel = toUsagePanel(
      summary([
        quota('managed_interfaces', 5, 3),
        quota('hybrid_gateways', 1, 0),
        quota('api_developer_portals', 1, 1),
      ]),
    );
    const standalone = panel!.sections.filter((section) =>
      ['hybrid_gateways', 'api_developer_portals'].includes(section.key),
    );
    expect(standalone.map((section) => section.title)).toEqual([
      'Gateways',
      'Portals',
    ]);
    // Each totals only itself. Sharing one section would add two unrelated
    // budgets together and render "1 of 2", which means nothing.
    for (const section of standalone) {
      expect(section.items).toHaveLength(1);
      expect(section.totalSuffix).toBe('used');
    }
    expect(standalone[0].items[0]).toEqual({
      key: 'hybrid_gateways',
      label: 'Gateways',
      count: 0,
      limit: 1,
    });
    expect(standalone[1].items[0].count).toBe(1);
  });

  // Three independent budgets, so three sections at one level — never folded
  // together, because a gateway takes nothing from the interfaces budget.
  it('keeps each budget in its own section, at the same level', () => {
    const panel = toUsagePanel(
      summary([
        quota('managed_interfaces', 5, 1),
        quota('hybrid_gateways', 1, 1),
        quota('api_developer_portals', 1, 1),
      ]),
    );
    expect(panel!.sections.map((section) => section.key)).toEqual([
      'managed_interfaces',
      'hybrid_gateways',
      'api_developer_portals',
    ]);
    for (const section of panel!.sections) expect(section.items).toHaveLength(1);
  });

  it('omits a standalone section that has no ceiling', () => {
    const panel = toUsagePanel(
      summary([
        quota('managed_interfaces', 5, 1),
        quota('hybrid_gateways', null, 4),
        quota('api_developer_portals', 1, 0),
      ]),
    );
    expect(panel!.sections.map((section) => section.key)).toEqual([
      'managed_interfaces',
      'api_developer_portals',
    ]);
  });

  it('gives every item and section a stable key', () => {
    const panel = toUsagePanel(trial({ apis: 1 }));
    for (const section of panel!.sections) {
      expect(section.key).toBeTruthy();
      for (const item of section.items) expect(item.key).toBeTruthy();
    }
  });
});
