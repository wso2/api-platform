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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { createIntl } from 'react-intl';
import { describe, expect, it } from 'vitest';

import type { SubscriptionPlan } from '@/api/resources/subscriptionPlans';
import { getPlanLimitDisplay } from './subscriptionPlanLimit';

type Limit = NonNullable<SubscriptionPlan['limits']>[number];

const intl = createIntl({ locale: 'en' });

const planWith = (limit: Partial<Limit>): SubscriptionPlan =>
  ({
    limits: [{ limitCount: 100, limitType: 'REQUEST_COUNT', timeAmount: 1, timeUnit: 'HOUR', ...limit }],
  }) as SubscriptionPlan;

/** The caption as the plan card shows it. */
const caption = (limit: Partial<Limit>) => {
  const display = getPlanLimitDisplay(planWith(limit));
  return intl.formatMessage(display.unit, display.unlimited ? undefined : display.unitValues);
};

describe('getPlanLimitDisplay caption', () => {
  it('keeps the singular unit for a window of one', () => {
    expect(caption({ timeAmount: 1, timeUnit: 'MINUTE' })).toBe('requests / minute');
    expect(caption({ limitType: 'TOTAL_TOKEN_COUNT', timeAmount: 1, timeUnit: 'MONTH' })).toBe('tokens / month');
    expect(caption({ limitCount: 5, limitCountUnit: 'MB', limitType: 'BANDWIDTH', timeAmount: 1, timeUnit: 'DAY' })).toBe(
      'MB / day',
    );
  });

  it('treats a missing amount as one', () => {
    expect(caption({ timeAmount: undefined, timeUnit: 'DAY' })).toBe('requests / day');
  });

  it('shows the amount and the plural unit for a longer window', () => {
    expect(caption({ timeAmount: 2, timeUnit: 'HOUR' })).toBe('requests / 2 hours');
    expect(caption({ limitType: 'TOTAL_TOKEN_COUNT', timeAmount: 3, timeUnit: 'MONTH' })).toBe('tokens / 3 months');
    expect(caption({ limitCountUnit: 'GB', limitType: 'BANDWIDTH', timeAmount: 5, timeUnit: 'MINUTE' })).toBe(
      'GB / 5 minutes',
    );
  });

  it('reads a plan with no limits as unlimited', () => {
    expect(getPlanLimitDisplay({ limits: [] } as unknown as SubscriptionPlan)).toMatchObject({ unlimited: true });
  });
});
