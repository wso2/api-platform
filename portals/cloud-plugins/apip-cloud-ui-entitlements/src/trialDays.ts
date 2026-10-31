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
 * Trial arithmetic, kept out of the component file so it can be unit-tested
 * without pulling the UI library (and its bundler-specific resolution needs)
 * into a plain Node test run.
 */

/**
 * Trial length to assume when billing does not report one.
 *
 * Only a fallback for an older billing deployment whose payload predates
 * `trial_days`; the real value is read from the API on every normal response.
 * 30 matches the seeded `api-platform` product (`trial_days = 30`), so a
 * deployment that does fall back still draws a ring against the right total.
 *
 * Do not treat this as the source of truth. This file's predecessor hardcoded
 * 14 against that same 30-day product, which pinned the badge's progress ring
 * at a full circle for the first 16 days of every trial.
 */
export const DEFAULT_TRIAL_DAYS = 30;

const MS_PER_DAY = 24 * 60 * 60 * 1000;

/**
 * Days left in the trial, derived from its end date rather than read from the
 * server's `days_remaining`.
 *
 * `days_remaining` is a snapshot taken when the response was built, so a tab
 * left open overnight keeps rendering yesterday's number even after a re-read.
 * The end date is absolute, so every recomputation is current without costing
 * a single extra request.
 *
 * It is recomputed only when the badge re-renders, though — nothing here ticks
 * — so an idle tab shows the figure from its last render until something
 * re-renders it, in practice the entitlement re-read on focus or the gate's
 * backstop interval. Add a timer if the number ever has to roll over on its
 * own.
 *
 * Returns a fractional value: the badge rounds up for display but uses the
 * exact figure for its status thresholds and progress ring. `null` means the
 * date could not be parsed, which the caller renders as nothing at all rather
 * than guessing at a number.
 */
export function daysLeftUntil(trialEnd: string, now: number = Date.now()): number | null {
  const end = new Date(trialEnd).getTime();
  if (Number.isNaN(end)) return null;
  return Math.max(0, (end - now) / MS_PER_DAY);
}
