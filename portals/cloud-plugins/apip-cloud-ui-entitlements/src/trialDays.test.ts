/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { describe, expect, it } from 'vitest';

import { daysLeftUntil } from './trialDays';

const NOW = Date.parse('2026-10-29T12:00:00.000Z');

describe('daysLeftUntil', () => {
  it('counts whole days to the trial end', () => {
    expect(daysLeftUntil('2026-11-08T12:00:00.000Z', NOW)).toBe(10);
  });

  it('keeps the fraction, so the badge can threshold on the exact value', () => {
    expect(daysLeftUntil('2026-10-29T18:00:00.000Z', NOW)).toBe(0.25);
  });

  it('reports 0 once the trial end has passed, never a negative', () => {
    expect(daysLeftUntil('2026-10-20T12:00:00.000Z', NOW)).toBe(0);
  });

  it('returns null for an unparseable date, so the caller can render nothing', () => {
    expect(daysLeftUntil('not-a-date', NOW)).toBeNull();
  });

  // The whole reason this is derived rather than read from the server's
  // `days_remaining`: that figure is a snapshot taken when the response was
  // built, so a long-lived tab would keep showing it. This one re-derives.
  it('decreases as time passes without a new response', () => {
    const end = '2026-11-08T12:00:00.000Z';
    expect(daysLeftUntil(end, NOW)).toBe(10);
    expect(daysLeftUntil(end, NOW + 3 * 24 * 60 * 60 * 1000)).toBe(7);
  });
});
