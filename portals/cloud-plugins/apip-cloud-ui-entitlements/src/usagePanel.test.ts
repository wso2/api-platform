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

/** Trial: 5 APIs. */
const trial = (used: Record<string, number> = {}) =>
  summary([quota('apis', 5, used.apis ?? 0)]);

describe('toUsagePanel', () => {
  it('shows nothing when there is no summary', () => {
    expect(toUsagePanel(null)).toBeNull();
    expect(toUsagePanel({ products: [] })).toBeNull();
  });

  // Nothing to count and nowhere to upgrade to, so the badge falls back to a
  // plain countdown. The same rule that hides the upgrade affordance.
  it('shows nothing when the APIs budget is unlimited', () => {
    expect(toUsagePanel(summary([quota('apis', null, 9000)]))).toBeNull();
  });

  it('shows nothing when the APIs budget is absent', () => {
    expect(toUsagePanel(summary([quota('hybrid_gateways', 1, 0)]))).toBeNull();
  });

  // The figure is the `apis` meter's own — the cap billing enforces and the
  // create button gates on.
  it('leads with the APIs budget', () => {
    const panel = toUsagePanel(trial({ apis: 3 }));
    const headline = panel!.sections[0];
    expect(headline.title).toBe('APIs');
    expect(headline.items).toEqual([{ key: 'apis', label: 'APIs', count: 3, limit: 5 }]);
    expect(headline.totalSuffix).toBe('left');
  });

  // A response still carrying the dropped meters must not add rows for them.
  it('ignores pooled and proxy meters the response may still carry', () => {
    const panel = toUsagePanel(
      summary([
        quota('apis', 5, 1),
        quota('managed_interfaces', 10, 4),
        quota('mcp_proxies', 5, 2),
        quota('llm_proxies', 5, 1),
        quota('agent_proxies', 5, 1),
      ]),
    );
    expect(panel!.sections.map((section) => section.key)).toEqual(['apis']);
    const labels = panel!.sections.flatMap((section) =>
      section.items.map((item) => item.label),
    );
    expect(labels).toEqual(['APIs']);
  });

  it('gives gateways and portals a section each, with their own totals', () => {
    const panel = toUsagePanel(
      summary([
        quota('apis', 5, 3),
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
      expect(section.totalSuffix).toBe('left');
    }
    expect(standalone[0].items[0]).toEqual({
      key: 'hybrid_gateways',
      label: 'Gateways',
      count: 0,
      limit: 1,
    });
    expect(standalone[1].items[0].count).toBe(1);
  });

  // Three independent budgets, so three sections — never folded together,
  // because a gateway takes nothing from the APIs budget.
  it('keeps each budget in its own section', () => {
    const panel = toUsagePanel(
      summary([
        quota('apis', 5, 1),
        quota('hybrid_gateways', 1, 1),
        quota('api_developer_portals', 1, 1),
      ]),
    );
    expect(panel!.sections.map((section) => section.key)).toEqual([
      'apis',
      'hybrid_gateways',
      'api_developer_portals',
    ]);
    for (const section of panel!.sections) expect(section.items).toHaveLength(1);
  });

  it('omits a standalone section that has no ceiling', () => {
    const panel = toUsagePanel(
      summary([
        quota('apis', 5, 1),
        quota('hybrid_gateways', null, 4),
        quota('api_developer_portals', 1, 0),
      ]),
    );
    expect(panel!.sections.map((section) => section.key)).toEqual([
      'apis',
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
