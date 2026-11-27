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

import { DEFAULT_TRIAL_DAYS, daysLeftUntil } from './trialDays';
import type { BillingOrganization, BillingSubscription } from './types';

/** What the header badge shows: a running trial's countdown, or that it expired. */
export type TrialBadgeState =
  | { kind: 'running'; daysLeft: number; totalDays: number; trialEnd: string }
  | { kind: 'expired'; totalDays: number };

/**
 * Billing keeps `trial_end` when a trial converts to a paid plan, and starts the
 * paid period on the trial's last day, while a trial's own period starts with
 * the trial. So a suspended subscription whose period began before that day
 * was never paid for. Without both dates nothing says it was a trial.
 * Mirrors Cloud Console's `endedOnTrial`.
 */
export function endedOnTrial(subscription: BillingSubscription): boolean {
  if (subscription.status !== 'suspended') return false;
  const trialEnd = Date.parse(subscription.trial?.trial_end ?? '');
  const periodStart = Date.parse(subscription.current_period_start ?? '');
  if (Number.isNaN(trialEnd) || Number.isNaN(periodStart)) return false;
  const end = new Date(trialEnd);
  return periodStart < Date.UTC(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate());
}

/**
 * The badge for this console's product subscription, or `null` for none: a paid
 * plan (active, past due, cancelling, being set up), a suspended paid plan, or
 * a trial whose end date cannot be read.
 */
export function trialBadgeState(
  organization: BillingOrganization | null | undefined,
  now: number = Date.now()
): TrialBadgeState | null {
  const subscription = organization?.subscription;
  if (!subscription) return null;
  const totalDays = subscription.trial?.trial_days ?? DEFAULT_TRIAL_DAYS;

  if (subscription.status === 'trial' && subscription.trial) {
    const daysLeft = daysLeftUntil(subscription.trial.trial_end, now);
    if (daysLeft === null) return null;
    return { kind: 'running', daysLeft, totalDays, trialEnd: subscription.trial.trial_end };
  }
  return endedOnTrial(subscription) ? { kind: 'expired', totalDays } : null;
}
