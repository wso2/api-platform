/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { describe, expect, it } from 'vitest';

import { isReadOnly, toResourceLimits } from './entitlementLimits';
import type { Entitlement, EntitlementSummary } from './entitlementTypes';

const quota = (name: string, limit: number | null, used: number): Entitlement => ({
  name,
  type: 'quota',
  limit,
  used,
  remaining: limit === null ? null : Math.max(0, limit - used),
});

const summary = (
  entitlements: Entitlement[],
  status = 'trial',
  // A suspended trial keeps its trial plan, so this is the default: the status
  // moves, the plan code does not. Pass a paid code for the other cases.
  planCode: string | undefined = 'api-platform-trial'
): EntitlementSummary => ({
  products: [
    {
      product: 'api-platform',
      subscription: {
        status,
        plan_code: planCode,
        entitlements_active: ['active', 'trial', 'past_due'].includes(status),
      },
      entitlements,
    },
  ],
});

/** Trial: 5 APIs, 1 gateway. */
const trial = (used: Record<string, number>) =>
  summary([
    quota('apis', 5, used.apis ?? 0),
    quota('hybrid_gateways', 1, used.gateways ?? 0),
  ]);

/** Growth: 5 APIs, 3 gateways. */
const growth = (used: Record<string, number>) =>
  summary([
    quota('apis', 5, used.apis ?? 0),
    quota('hybrid_gateways', 3, used.gateways ?? 0),
  ]);

describe('toResourceLimits', () => {
  it('reads nothing as unknown rather than as a cap', () => {
    expect(toResourceLimits(null)).toBeNull();
    expect(toResourceLimits(undefined)).toBeNull();
    expect(toResourceLimits({ products: [] })).toBeNull();
  });

  it('ignores another product’s entitlements', () => {
    const other: EntitlementSummary = {
      products: [
        { product: 'integration', entitlements: [quota('apis', 1, 1)] },
        { product: 'another', entitlements: [quota('apis', 1, 1)] },
      ],
    };
    expect(toResourceLimits(other)).toBeNull();
  });

  describe('the two gateway caps', () => {
    it('maps self-hosted gateways from hybrid_gateways', () => {
      const limits = toResourceLimits(growth({ gateways: 1 }));
      expect(limits?.selfHostedGateways).toMatchObject({ max: 3, used: 1 });
    });

    it('maps managed gateways from the gateways meter', () => {
      const limits = toResourceLimits(summary([quota('gateways', 1, 1)]));
      expect(limits?.managedGateways).toMatchObject({ max: 1, used: 1 });
      expect(limits?.managedGateways?.message).toContain('Managed Gateways');
    });

    // "May ANY gateway be created" — a full managed quota does not stop a
    // self-hosted one, so this reports whichever kind still has room.
    it('reports the roomiest kind for the generic gateways verdict', () => {
      const limits = toResourceLimits(
        summary([quota('gateways', 1, 1), quota('hybrid_gateways', 2, 0)])
      );
      expect(limits?.gateways).toMatchObject({ max: 2, used: 0 });
    });

    // With neither kind having room the verdict is "full" whichever meter
    // names it, so assert the verdict rather than which one won the tie.
    it('reports a full cap for the generic verdict only when both kinds are full', () => {
      const limits = toResourceLimits(
        summary([quota('gateways', 1, 1), quota('hybrid_gateways', 2, 2)])
      );
      expect(limits!.gateways!.used).toBeGreaterThanOrEqual(limits!.gateways!.max);
    });

    it('treats an unlimited kind as room for the generic verdict', () => {
      const limits = toResourceLimits(
        summary([quota('gateways', 1, 1), quota('hybrid_gateways', null, 9)])
      );
      expect(limits?.gateways?.max).toBe(-1);
    });
  });

  describe('the APIs budget', () => {
    // The Create API gate binds on `apis` and nothing else: MCP, LLM and
    // agent proxies are not metered, so there is no shared budget above it.
    it('binds on apis when it is exhausted', () => {
      const limits = toResourceLimits(growth({ apis: 5 }));
      expect(limits?.restApis).toMatchObject({ max: 5, used: 5 });
      expect(limits?.restApis?.message).toContain('APIs');
    });

    it('leaves headroom when the cap is not reached', () => {
      const limits = toResourceLimits(growth({ apis: 2 }));
      expect(limits?.restApis).toMatchObject({ max: 5, used: 2 });
    });
  });

  describe('unlimited', () => {
    it('maps a null limit to the host’s -1', () => {
      const limits = toResourceLimits(summary([quota('apis', null, 42)]));
      expect(limits?.restApis?.max).toBe(-1);
    });

    it('carries no message when the meter is unlimited', () => {
      const limits = toResourceLimits(summary([quota('apis', null, 42)]));
      expect(limits?.restApis?.message).toBeUndefined();
    });
  });

  describe('edge cases', () => {
    // Billing omits meters a plan does not define. Reading that as zero would
    // block creation outright.
    it('does not cap a metric that is absent', () => {
      const limits = toResourceLimits(summary([quota('hybrid_gateways', 3, 0)]));
      expect(limits?.restApis).toBeUndefined();
      expect(limits?.gateways).toMatchObject({ max: 3, used: 0 });
    });

    it('clamps a negative remaining to zero rather than reading it as headroom', () => {
      // Grandfathered above a lowered limit: 12 used against a cap of 10.
      const over: EntitlementSummary = summary([
        { name: 'apis', type: 'quota', limit: 10, used: 12, remaining: -2 },
      ]);
      const limits = toResourceLimits(over);
      expect(limits?.restApis).toMatchObject({ max: 10, used: 12 });
      expect(limits?.restApis?.message).toContain('APIs');
    });

    it('derives remaining when the field is absent', () => {
      const noRemaining: EntitlementSummary = summary([
        { name: 'apis', type: 'quota', limit: 10, used: 10 },
      ]);
      expect(toResourceLimits(noRemaining)?.restApis).toMatchObject({ max: 10, used: 10 });
    });
  });
});

describe('isReadOnly', () => {
  it('is false when nothing is known, so a failed read never locks the console', () => {
    expect(isReadOnly(null)).toBe(false);
    expect(isReadOnly({ products: [] })).toBe(false);
  });

  it('is false while a trial is running', () => {
    expect(isReadOnly(trial({}))).toBe(false);
  });

  it('is true once an inactive subscription is still on the trial plan', () => {
    expect(isReadOnly(summary([], 'suspended'))).toBe(true);
    expect(isReadOnly(summary([], 'cancelled'))).toBe(true);
  });

  // The narrow scope: this banner speaks about a finished trial, so a paid
  // subscription suspended for some other reason (non-payment, an admin
  // action) is left alone rather than shown trial-expiry wording. It cannot
  // write either — billing refuses it — but saying so needs its own message.
  it('is false for an inactive subscription on a paid plan', () => {
    expect(isReadOnly(summary([], 'suspended', 'api-platform-growth'))).toBe(false);
    expect(isReadOnly(summary([], 'cancelled', 'api-platform-enterprise'))).toBe(false);
  });

  it('is false when the plan code is missing, rather than guessing', () => {
    // Built inline rather than through `summary`: passing `undefined` to a
    // parameter with a default gets the default, which is the trial code.
    const noPlanCode: EntitlementSummary = {
      products: [
        {
          product: 'api-platform',
          subscription: { status: 'suspended', entitlements_active: false },
          entitlements: [],
        },
      ],
    };
    expect(isReadOnly(noPlanCode)).toBe(false);
  });

  // Every product codes its trial plan the same way, so this holds without
  // naming api-platform's own code.
  it('recognises any product\'s trial plan, case and padding aside', () => {
    expect(isReadOnly(summary([], 'suspended', 'integration-platform-trial'))).toBe(true);
    expect(isReadOnly(summary([], 'suspended', '  API-Platform-Trial  '))).toBe(true);
  });

  // past_due counts as active server-side: an unsettled invoice is not the same
  // as having no plan, and needs its own treatment rather than this banner.
  it('is false for past_due', () => {
    expect(isReadOnly(summary([], 'past_due'))).toBe(false);
  });

  it('is false when the verdict is absent, as on an older billing deployment', () => {
    const noVerdict: EntitlementSummary = {
      products: [{ product: 'api-platform', subscription: { status: 'active' } }],
    };
    expect(isReadOnly(noVerdict)).toBe(false);
  });
});
