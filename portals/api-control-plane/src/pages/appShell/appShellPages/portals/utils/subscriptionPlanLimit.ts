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
  unitRequestPerMinute: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitRequestPerMinute',
    defaultMessage: 'requests / minute',
  },
  unitRequestPerHour: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitRequestPerHour',
    defaultMessage: 'requests / hour',
  },
  unitRequestPerDay: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitRequestPerDay',
    defaultMessage: 'requests / day',
  },
  unitRequestPerMonth: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitRequestPerMonth',
    defaultMessage: 'requests / month',
  },
  unitBandwidth: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitBandwidth',
    defaultMessage: '{unit} / {window}',
    description:
      'Bandwidth limit unit. {unit} is the server-supplied limitCountUnit (e.g. "MB"); {window} is the lowercase time window (e.g. "month").',
  },
  unitTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.unitTokens',
    defaultMessage: 'tokens / {window}',
    description: 'Token-count limit unit. {window} is the lowercase time window (e.g. "month").',
  },
  noRequestCap: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.subscriptionPlanLimit.noRequestCap',
    defaultMessage: 'no request cap',
  },
});

const TIME_WINDOW_LABEL: Record<NonNullable<SubscriptionPlan['limits']>[number]['timeUnit'], string> = {
  MINUTE: 'minute',
  HOUR: 'hour',
  DAY: 'day',
  MONTH: 'month',
};

const REQUEST_UNIT_BY_WINDOW: Record<string, MessageDescriptor> = {
  minute: messages.unitRequestPerMinute,
  hour: messages.unitRequestPerHour,
  day: messages.unitRequestPerDay,
  month: messages.unitRequestPerMonth,
};

/**
 * `unit`/`unitValues` always describe the caption under the big number — one
 * shape for both branches, so a renderer never has to branch on what kind of
 * value `unit` holds; only `count` is absent when `unlimited`.
 */
export type PlanLimitDisplay =
  | { unlimited: true; unit: MessageDescriptor }
  | { unlimited: false; count: number; unit: MessageDescriptor; unitValues?: Record<string, string> };

/**
 * The plan card's big number and its unit caption. A plan with no limits
 * configured has no cap at all — rendered as "Unlimited" / "no request cap",
 * never an empty cell. Only `limits[0]` is read: platform-api persists and
 * enforces just the first entry, so that's the only one that is ever real.
 */
export const getPlanLimitDisplay = (plan: SubscriptionPlan): PlanLimitDisplay => {
  const limit = plan.limits?.[0];
  if (!limit) return { unit: messages.noRequestCap, unlimited: true };

  const window = TIME_WINDOW_LABEL[limit.timeUnit];

  if (limit.limitType === 'BANDWIDTH') {
    return {
      count: limit.limitCount,
      unit: messages.unitBandwidth,
      unitValues: { unit: limit.limitCountUnit ?? '', window },
      unlimited: false,
    };
  }
  if (limit.limitType === 'TOTAL_TOKEN_COUNT') {
    return { count: limit.limitCount, unit: messages.unitTokens, unitValues: { window }, unlimited: false };
  }
  return { count: limit.limitCount, unit: REQUEST_UNIT_BY_WINDOW[window], unlimited: false };
};
