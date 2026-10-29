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

import type { SubscriptionPlan, SubscriptionPlanLimit } from '../../../../utils/types';
import { getPlanLimitDisplay } from './subscriptionPlanLimit';

const planWith = (...limits: SubscriptionPlanLimit[]): SubscriptionPlan => ({
  id: 'gold',
  displayName: 'Gold',
  status: 'ACTIVE',
  limits,
});

describe('getPlanLimitDisplay', () => {
  it('shows a plan with no limits as unlimited, never an empty cell', () => {
    const display = getPlanLimitDisplay(planWith());

    expect(display.unlimited).toBe(true);
    expect(display.unit.defaultMessage).toBe('no request cap');
  });

  it('treats a plan whose limits were never set the same way', () => {
    expect(getPlanLimitDisplay({ id: 'x', displayName: 'X' }).unlimited).toBe(true);
  });

  it('describes a request limit by its count and time window', () => {
    const display = getPlanLimitDisplay(
      planWith({ limitType: 'REQUEST_COUNT', limitCount: 1000, timeAmount: 5, timeUnit: 'MINUTE' })
    );

    expect(display).toMatchObject({
      count: 1000,
      unitValues: { amount: 5, timeUnit: 'MINUTE' },
      unlimited: false,
    });
    expect(display.unit.id).toMatch(/planLimit\.unitRequests$/);
  });

  it('describes a bandwidth limit with the unit the server supplied', () => {
    const display = getPlanLimitDisplay(
      planWith({ limitType: 'BANDWIDTH', limitCount: 50, limitCountUnit: 'MB', timeAmount: 1, timeUnit: 'DAY' })
    );

    expect(display).toMatchObject({ count: 50, unitValues: { amount: 1, timeUnit: 'DAY', unit: 'MB' } });
    expect(display.unit.id).toMatch(/planLimit\.unitBandwidth$/);
  });

  it('describes a token limit', () => {
    const display = getPlanLimitDisplay(
      planWith({ limitType: 'TOTAL_TOKEN_COUNT', limitCount: 100000, timeAmount: 1, timeUnit: 'MONTH' })
    );

    expect(display).toMatchObject({ count: 100000, unlimited: false });
    expect(display.unit.id).toMatch(/planLimit\.unitTokens$/);
  });

  it('reads a limit of an unnamed type as a request limit over one month', () => {
    const display = getPlanLimitDisplay(planWith({ limitCount: 20 }));

    expect(display).toMatchObject({ count: 20, unitValues: { amount: 1, timeUnit: 'MONTH' } });
    expect(display.unit.id).toMatch(/planLimit\.unitRequests$/);
  });

  it('reads only the first limit, since only that one is enforced', () => {
    const display = getPlanLimitDisplay(
      planWith({ limitType: 'REQUEST_COUNT', limitCount: 10 }, { limitType: 'BANDWIDTH', limitCount: 999 })
    );

    expect(display).toMatchObject({ count: 10 });
  });
});
