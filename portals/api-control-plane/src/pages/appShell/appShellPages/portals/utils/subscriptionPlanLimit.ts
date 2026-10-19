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

import { defineMessages, type MessageDescriptor } from 'react-intl';

import type { SubscriptionPlan } from '@/api/resources/subscriptionPlans';

const messages = defineMessages({
  unitRequests: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitRequests',
    defaultMessage:
      'requests / {amount, plural, one {{timeUnit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {timeUnit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description:
      'Request-count limit unit. {amount} is how many time units the window spans; {timeUnit} is MINUTE, HOUR, DAY or MONTH.',
  },
  unitBandwidth: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitBandwidth',
    defaultMessage:
      '{unit} / {amount, plural, one {{timeUnit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {timeUnit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description:
      'Bandwidth limit unit. {unit} is the server-supplied limitCountUnit (e.g. "MB"); {amount} and {timeUnit} describe the window.',
  },
  unitTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitTokens',
    defaultMessage:
      'tokens / {amount, plural, one {{timeUnit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {timeUnit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description: 'Token-count limit unit. {amount} and {timeUnit} describe the window.',
  },
  noRequestCap: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.noRequestCap',
    defaultMessage: 'no request cap',
  },
});

/**
 * `unit`/`unitValues` always describe the caption under the big number — one
 * shape for both branches, so a renderer never has to branch on what kind of
 * value `unit` holds; only `count` is absent when `unlimited`.
 */
export type PlanLimitDisplay =
  | { unlimited: true; unit: MessageDescriptor }
  | { unlimited: false; count: number; unit: MessageDescriptor; unitValues?: Record<string, string | number> };

/**
 * The plan card's big number and its unit caption. A plan with no limits
 * configured has no cap at all — rendered as "Unlimited" / "no request cap",
 * never an empty cell. Only `limits[0]` is read: platform-api persists and
 * enforces just the first entry, so that's the only one that is ever real.
 */
export const getPlanLimitDisplay = (plan: SubscriptionPlan): PlanLimitDisplay => {
  const limit = plan.limits?.[0];
  if (!limit) return { unit: messages.noRequestCap, unlimited: true };

  const timeWindow = { amount: limit.timeAmount ?? 1, timeUnit: limit.timeUnit };

  if (limit.limitType === 'BANDWIDTH') {
    return {
      count: limit.limitCount,
      unit: messages.unitBandwidth,
      unitValues: { ...timeWindow, unit: limit.limitCountUnit ?? '' },
      unlimited: false,
    };
  }
  if (limit.limitType === 'TOTAL_TOKEN_COUNT') {
    return { count: limit.limitCount, unit: messages.unitTokens, unitValues: timeWindow, unlimited: false };
  }
  return { count: limit.limitCount, unit: messages.unitRequests, unitValues: timeWindow, unlimited: false };
};
