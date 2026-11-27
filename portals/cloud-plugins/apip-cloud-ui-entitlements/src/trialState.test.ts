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

import { describe, expect, it } from 'vitest';

import { endedOnTrial, trialBadgeState } from './trialState';
import type { BillingSubscription } from './types';

const NOW = Date.parse('2026-10-29T12:00:00.000Z');

const org = (subscription: BillingSubscription | null) => ({ subscription });

const trial = (trial_end: string, trial_days?: number) => ({
  days_remaining: 0,
  trial_end,
  trial_days,
});

// Billing's shapes for one product subscription after its trial ended.
const lapsedTrial = (status: string): BillingSubscription => ({
  status,
  current_period_start: '2026-10-01T00:00:00.000Z',
  trial: trial('2026-10-31T00:00:00.000Z', 30),
});
const lapsedPaidPlan: BillingSubscription = {
  status: 'suspended',
  current_period_start: '2026-10-31T00:00:00.000Z',
  trial: trial('2026-10-31T09:30:00.000Z', 30),
};

describe('trialBadgeState', () => {
  it('counts down a running trial against its own length', () => {
    expect(
      trialBadgeState(org({ status: 'trial', trial: trial('2026-11-08T12:00:00.000Z', 30) }), NOW)
    ).toEqual({ kind: 'running', daysLeft: 10, totalDays: 30, trialEnd: '2026-11-08T12:00:00.000Z' });
  });

  it('assumes the product trial length when billing does not send one', () => {
    expect(
      trialBadgeState(org({ status: 'trial', trial: trial('2026-11-08T12:00:00.000Z') }), NOW)
    ).toMatchObject({ totalDays: 30 });
  });

  it('shows nothing for a trial whose end date cannot be read', () => {
    expect(trialBadgeState(org({ status: 'trial', trial: trial('soon') }), NOW)).toBeNull();
  });

  it('shows the trial expired once a trial is suspended', () => {
    expect(trialBadgeState(org(lapsedTrial('suspended')), NOW)).toEqual({
      kind: 'expired',
      totalDays: 30,
    });
  });

  it.each(['active', 'past_due', 'pending_cancellation', 'pending_activation'])(
    'shows nothing for a %s plan, even with an old trial on it',
    (status) => {
      expect(trialBadgeState(org(lapsedTrial(status)), NOW)).toBeNull();
    }
  );

  it('shows nothing for a suspended paid plan', () => {
    expect(trialBadgeState(org(lapsedPaidPlan), NOW)).toBeNull();
  });

  it('shows nothing without billing or a subscription', () => {
    expect(trialBadgeState(null, NOW)).toBeNull();
    expect(trialBadgeState(org(null), NOW)).toBeNull();
  });
});

describe('endedOnTrial', () => {
  it('needs both the trial end and the period start', () => {
    expect(endedOnTrial({ status: 'suspended', trial: trial('2026-10-31T00:00:00.000Z') })).toBe(false);
    expect(
      endedOnTrial({ status: 'suspended', current_period_start: '2026-10-01T00:00:00.000Z' })
    ).toBe(false);
  });

  // A paid period starts on the trial's last day, at any hour of it.
  it('reads a period starting on the trial end day as paid', () => {
    expect(endedOnTrial(lapsedPaidPlan)).toBe(false);
    expect(endedOnTrial(lapsedTrial('suspended'))).toBe(true);
  });
});
