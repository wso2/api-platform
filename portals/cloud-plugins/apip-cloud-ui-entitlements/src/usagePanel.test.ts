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

/** Trial: 5 APIs, 1 managed gateway, 1 self-hosted gateway, 1 portal. */
const trial = (used: Record<string, number> = {}) =>
  summary([
    quota('apis', 5, used.apis ?? 0),
    quota('gateways', 1, used.managedGateways ?? 0),
    quota('hybrid_gateways', 1, used.gateways ?? 0),
    quota('api_developer_portals', 1, used.portals ?? 0),
  ]);

describe('toUsagePanel', () => {
  it('shows nothing when there is no summary', () => {
    expect(toUsagePanel(null)).toBeNull();
    expect(toUsagePanel({ products: [] })).toBeNull();
  });

  // Nothing to count and nowhere to upgrade to, so the badge falls back to a
  // plain countdown. The same rule that hides the upgrade affordance.
  it('shows nothing when no meter has a ceiling', () => {
    expect(
      toUsagePanel(summary([quota('apis', null, 9000), quota('hybrid_gateways', null, 4)])),
    ).toBeNull();
  });

  it('groups infrastructure, then gateways, then managed interfaces', () => {
    const panel = toUsagePanel(trial({ apis: 3, gateways: 1, portals: 1 }));
    expect(panel!.sections.map((section) => section.title)).toEqual([
      'Infrastructure',
      'Gateways',
      'Managed Interfaces',
    ]);
    expect(panel!.sections[0].items.map((item) => item.key)).toEqual([
      'api_developer_portals',
    ]);
    expect(panel!.sections[2].items).toEqual([
      { key: 'apis', label: 'APIs', count: 3, limit: 5 },
    ]);
  });

  // A deployment whose plan caps only one gateway kind shows only that row,
  // rather than inventing a ceiling for the other.
  it('shows only the gateway kinds the plan caps', () => {
    const panel = toUsagePanel(summary([quota('hybrid_gateways', 3, 1)]));
    expect(panel!.sections.map((section) => section.key)).toEqual(['gateways']);
    expect(panel!.sections[0].items.map((item) => item.key)).toEqual(['hybrid_gateways']);
  });

  // Billing carries a `projects` quota, but nothing reports the count, so the
  // row would read 0 of 5 for an organization that has projects.
  it('never shows a projects row, even when the plan caps them', () => {
    const panel = toUsagePanel(
      summary([quota('projects', 5, 0), quota('api_developer_portals', 1, 0)]),
    );
    expect(panel!.sections[0].items.map((item) => item.key)).toEqual([
      'api_developer_portals',
    ]);
  });

  it('totals managed interfaces only, never a group of independent caps', () => {
    const panel = toUsagePanel(trial({ apis: 3, gateways: 1, portals: 1 }));
    expect(panel!.sections[0].totalSuffix).toBeUndefined();
    expect(panel!.sections[1].totalSuffix).toBeUndefined();
    expect(panel!.sections[2].totalSuffix).toBe('used');
  });

  // MCP, LLM and agent proxies are not metered, so the panel never names them
  // and never shows a shared interfaces budget above the APIs row.
  it('ignores unmetered interface types and any legacy shared budget', () => {
    const panel = toUsagePanel(
      summary([
        quota('managed_interfaces', 10, 4),
        quota('apis', 5, 1),
        quota('mcp_proxies', 5, 2),
        quota('llm_proxies', 5, 1),
        quota('agent_proxies', 5, 0),
      ]),
    );
    expect(panel!.sections.map((section) => section.key)).toEqual(['managed-interfaces']);
    expect(panel!.sections[0].items.map((item) => item.key)).toEqual(['apis']);
  });

  // A meter this plan does not cap is absent, not zero — rendering it would
  // invent a ceiling billing never set.
  it('omits a row that has no ceiling', () => {
    const panel = toUsagePanel(
      summary([
        quota('apis', 5, 1),
        quota('hybrid_gateways', null, 4),
        quota('api_developer_portals', 1, 0),
      ]),
    );
    expect(panel!.sections[0].items.map((item) => item.key)).toEqual([
      'api_developer_portals',
    ]);
    expect(panel!.sections.map((section) => section.key)).not.toContain('gateways');
  });

  it('drops a group whose every meter is uncapped', () => {
    const panel = toUsagePanel(
      summary([quota('apis', null, 9000), quota('hybrid_gateways', 1, 0)]),
    );
    // No Managed Interfaces group: its one meter is unlimited.
    expect(panel!.sections.map((section) => section.key)).toEqual(['gateways']);
  });

  it('gives every item and section a stable key', () => {
    const panel = toUsagePanel(trial({ apis: 1 }));
    for (const section of panel!.sections) {
      expect(section.key).toBeTruthy();
      for (const item of section.items) expect(item.key).toBeTruthy();
    }
  });
});
