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

import type { Policy } from '@/api/resources/restApis';
import type { SubscriptionPlan } from '@/api/resources/subscriptionPlans';
import {
  BUILT_IN_IDS,
  BUILT_IN_TYPES,
  isBuiltInType,
  type BuiltInType,
} from '../../orgManage/components/planConditions';

/**
 * The gateway policy that checks subscriptions. Without it on an API, no plan, condition or
 * subscription of that API is checked, so every API that offers plans needs it.
 */
export const SUBSCRIPTION_VALIDATION = 'subscription-validation';

/**
 * Which policy enforces each kind of plan condition, as the gateway ships today. The one place to
 * change when a policy for another condition lands: `null` means no policy enforces it yet, and a
 * custom condition is read only by a policy listed under its ID.
 * subscription-validation v1 applies the plan's request limit (request-count); nothing reads
 * tool-call, token or event counts, or any custom condition, yet.
 */
export const CONDITION_POLICIES: {
  builtIn: Record<BuiltInType, string | null>;
  customById: Readonly<Record<string, string>>;
} = {
  builtIn: {
    'event-count': null,
    'request-count': SUBSCRIPTION_VALIDATION,
    'token-count': null,
    'tool-call-count': null,
  },
  customById: {},
};

/** The policy that enforces a condition, or `null` when none does yet. */
export const enforcerOf = (condition: { type: string; id?: string }): string | null =>
  isBuiltInType(condition.type)
    ? CONDITION_POLICIES.builtIn[condition.type]
    : (CONDITION_POLICIES.customById[condition.id ?? ''] ?? null);

/** What an API carries its policies in: its own list, and (a REST API) each operation's. */
type PolicyHolder = {
  policies?: Policy[];
  operations?: { request?: { policies?: Policy[] }; policies?: Policy[] }[];
};

/** The names of every policy attached to an API, globally or on any operation. */
export const attachedPolicyNames = (api: PolicyHolder): Set<string> =>
  new Set([
    ...(api.policies ?? []).map((policy) => policy.name),
    ...(api.operations ?? []).flatMap((operation) =>
      [...(operation.request?.policies ?? []), ...(operation.policies ?? [])].map(
        (policy) => policy.name,
      ),
    ),
  ]);

/**
 * The policies an API needs for these plans to be enforced: subscription-validation always, then
 * the policy of each condition the plans hold, once each. A condition no policy enforces needs none.
 */
export const requiredPolicies = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
): string[] => {
  const names = new Set<string>([SUBSCRIPTION_VALIDATION]);
  for (const plan of plans) {
    for (const condition of plan.conditions ?? []) {
      const enforcer = enforcerOf(condition);
      if (enforcer) names.add(enforcer);
    }
  }
  return [...names];
};

/** Whether a condition is enforced on the API, not (yet attached), or has no policy at all. */
export type EnforcementStatus = 'enforced' | 'notEnforced' | 'noPolicy';

/** One row of the "Required by subscription plans" notes: a built-in type, or a custom ID. */
export type EnforcementRow = {
  /** The condition type, or `custom`. */
  type: BuiltInType | 'custom';
  /** The condition's ID: fixed for a built-in type, chosen for a custom one. */
  id: string;
  /** The policy that enforces it, if any does. */
  policy: string | null;
  status: EnforcementStatus;
};

export type PlanEnforcement = {
  /** subscription-validation is not attached, so nothing of the plans is checked. */
  validationMissing: boolean;
  /** Built-in types in the API Portal's order, then custom IDs in the order the plans list them. */
  rows: EnforcementRow[];
  enforced: number;
};

/**
 * What the plans an API offers need from its attached policies: one row per built-in condition
 * type any plan holds, then one per custom condition ID. A condition counts as enforced only
 * while subscription-validation is attached too, since nothing is checked without it.
 */
export const planEnforcement = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
  attached: ReadonlySet<string>,
): PlanEnforcement => {
  const validationMissing = !attached.has(SUBSCRIPTION_VALIDATION);
  const conditions = plans.flatMap((plan) => plan.conditions ?? []);
  const statusOf = (policy: string | null): EnforcementStatus => {
    if (!policy) return 'noPolicy';
    return attached.has(policy) && !validationMissing ? 'enforced' : 'notEnforced';
  };

  const builtInRows = BUILT_IN_TYPES.filter((type) =>
    conditions.some((condition) => condition.type === type),
  ).map((type): EnforcementRow => {
    const policy = CONDITION_POLICIES.builtIn[type];
    return { id: BUILT_IN_IDS[type], policy, status: statusOf(policy), type };
  });
  const customIds = [
    ...new Set(
      conditions
        .filter((condition) => !isBuiltInType(condition.type) && condition.id)
        .map((condition) => condition.id!),
    ),
  ];
  const customRows = customIds.map((id): EnforcementRow => {
    const policy = enforcerOf({ id, type: 'custom' });
    return { id, policy, status: statusOf(policy), type: 'custom' };
  });
  const rows = [...builtInRows, ...customRows];
  return {
    enforced: rows.filter((row) => row.status === 'enforced').length,
    rows,
    validationMissing,
  };
};
