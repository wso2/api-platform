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

import { defineMessages, type MessageDescriptor } from 'react-intl';

import type { SubscriptionPlan } from '../../../../utils/types';

const messages = defineMessages({
  unitRequests: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planLimit.unitRequests',
    defaultMessage:
      'requests / {amount, plural, one {{timeUnit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {timeUnit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description:
      'Request-count limit unit. {amount} is how many time units the window spans; {timeUnit} is MINUTE, HOUR, DAY or MONTH.',
  },
  unitBandwidth: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planLimit.unitBandwidth',
    defaultMessage:
      '{unit} / {amount, plural, one {{timeUnit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {timeUnit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description:
      'Bandwidth limit unit. {unit} is the server-supplied limitCountUnit (e.g. "MB"); {amount} and {timeUnit} describe the window.',
  },
  unitTokens: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planLimit.unitTokens',
    defaultMessage:
      'tokens / {amount, plural, one {{timeUnit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {timeUnit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description: 'Token-count limit unit. {amount} and {timeUnit} describe the window.',
  },
  noRequestCap: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.planLimit.noRequestCap',
    defaultMessage: 'no request cap',
  },
});

/** `count` is absent when `unlimited`; `unit`/`unitValues` always describe the caption. */
export type PlanLimitDisplay =
  | { unlimited: true; unit: MessageDescriptor }
  | { unlimited: false; count: number; unit: MessageDescriptor; unitValues?: Record<string, string | number> };

/**
 * The plan card's number and its unit caption; a plan without limits reads "Unlimited".
 * Only `limits[0]` is read: platform-api enforces just the first entry.
 */
export const getPlanLimitDisplay = (plan: SubscriptionPlan): PlanLimitDisplay => {
  const limit = plan.limits?.[0];
  if (!limit) return { unit: messages.noRequestCap, unlimited: true };

  const timeWindow = { amount: limit.timeAmount ?? 1, timeUnit: limit.timeUnit ?? 'MONTH' };
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
