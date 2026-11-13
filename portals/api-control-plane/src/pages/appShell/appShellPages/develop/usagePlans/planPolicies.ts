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
  customPolicyOf,
  isBuiltInType,
  type BuiltInType,
} from '../../orgManage/components/planConditions';

/**
 * How a policy is named where its version matters: `name@v<major>`, as a custom condition names
 * the policy that enforces it (`region-guard@v1`). Built-in conditions name a policy by name alone.
 */
export const policyKey = (name: string, version?: string): string => {
  if (!version) return name;
  const major = version.trim().match(/^v?(\d+)/)?.[1];
  return `${name}@${major ? `v${major}` : version.trim()}`;
};

/** The name and (when the key carries one) version of a policy key. */
export const parsePolicyKey = (key: string): { name: string; version?: string } => {
  const at = key.lastIndexOf('@');
  return at > 0 ? { name: key.slice(0, at), version: key.slice(at + 1) } : { name: key };
};

/** A policy key as people read it: `region-guard v1`. */
export const policyLabel = (key: string): string => {
  const { name, version } = parsePolicyKey(key);
  return version ? `${name} ${version}` : name;
};

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
 * tool-call or token counts, or any custom condition, yet.
 */
export const CONDITION_POLICIES: {
  builtIn: Record<BuiltInType, string | null>;
  customById: Readonly<Record<string, string>>;
} = {
  builtIn: {
    'request-count': SUBSCRIPTION_VALIDATION,
    'token-count': null,
    'tool-call-count': null,
  },
  customById: {},
};

/** The kinds of API a plan can be offered on, as far as its conditions are concerned. */
export type ApiKind = 'rest' | 'graphql' | 'mcp' | 'llm' | 'event';

/**
 * Where each built-in condition is counted. A REST or GraphQL API never sees a tool call or a token
 * count, so a plan's tool-call-count or token-count condition does not apply to it, and no policy for it is needed there. Custom conditions apply wherever a policy reads them.
 */
export const CONDITION_APPLIES_TO: Record<BuiltInType, readonly ApiKind[]> = {
  'request-count': ['rest', 'graphql', 'mcp', 'llm'],
  'token-count': ['llm'],
  'tool-call-count': ['mcp'],
};

/** Whether a condition can apply to an API of this kind (always, when the kind is not given). */
export const appliesTo = (condition: { type: string }, kind?: ApiKind): boolean =>
  !kind || !isBuiltInType(condition.type) || CONDITION_APPLIES_TO[condition.type].includes(kind);

/** The kind of an API from its `{apiType}` on the publication endpoints (`rest-api`, `graphql-api`). */
export const apiKindOf = (apiType: string): ApiKind => {
  if (apiType.startsWith('graphql')) return 'graphql';
  if (apiType.startsWith('mcp')) return 'mcp';
  if (apiType.startsWith('llm')) return 'llm';
  if (/websub|websocket|event/.test(apiType)) return 'event';
  return 'rest';
};

/** Whether `record` has `key` as its own property, so `constructor` or `__proto__` is not found
 * on its prototype. */
const hasOwn = (record: object, key: string) => Object.prototype.hasOwnProperty.call(record, key);

/** The policy that enforces a condition, or `null` when none does yet. A custom condition that
 * names its policy (`params.policy`) is enforced by that policy at that version. */
export const enforcerOf = (condition: {
  type: string;
  id?: string;
  params?: unknown;
}): string | null => {
  if (isBuiltInType(condition.type)) return CONDITION_POLICIES.builtIn[condition.type];
  const named = customPolicyOf(condition);
  if (named) return policyKey(named.name, named.version);
  const id = condition.id ?? '';
  return hasOwn(CONDITION_POLICIES.customById, id) ? CONDITION_POLICIES.customById[id] : null;
};

/** What an API carries its policies in: its own list, and (a REST API) each operation's. */
type PolicyHolder = {
  policies?: Pick<Policy, 'name' | 'version'>[];
  operations?: {
    request?: { policies?: Pick<Policy, 'name' | 'version'>[] };
    policies?: Pick<Policy, 'name' | 'version'>[];
  }[];
};

/** Each policy by name, and by name at its major version (`name@v1`), so either can be looked up. */
const policyKeysOf = (policies: readonly Pick<Policy, 'name' | 'version'>[] = []) =>
  policies.flatMap((policy) => [policy.name, policyKey(policy.name, policy.version || 'v1')]);

/** Where an API's policies apply, as far as the plans are concerned. */
export type PolicyAttachment = {
  /** Policies on the API itself, or on every one of its operations: they see every request. */
  attached: Set<string>;
  /** Policies on some operations only: requests to the others pass without them. */
  partial: Set<string>;
};

/**
 * Which policies apply to every request of an API and which to some only. A policy counts as
 * attached when it is on the API (global) or on each of its operations; on some operations
 * only, a plan condition it enforces is not enforced on the rest, so it is `partial`.
 */
export const policyAttachment = (api: PolicyHolder): PolicyAttachment => {
  const global = new Set(policyKeysOf(api.policies));
  const perOperation = (api.operations ?? []).map(
    (operation) =>
      new Set(policyKeysOf([...(operation.request?.policies ?? []), ...(operation.policies ?? [])])),
  );
  const attached = new Set(global);
  const partial = new Set<string>();
  for (const name of new Set(perOperation.flatMap((names) => [...names]))) {
    if (global.has(name)) continue;
    if (perOperation.every((names) => names.has(name))) attached.add(name);
    else partial.add(name);
  }
  return { attached, partial };
};

/** The names of the policies that apply to every request of an API (see `policyAttachment`). */
export const attachedPolicyNames = (api: PolicyHolder): Set<string> =>
  policyAttachment(api).attached;

/**
 * The policies an API needs for these plans to be enforced: subscription-validation always, then
 * the policy of each condition the plans hold, once each. A condition no policy enforces needs none,
 * and neither does one this kind of API never counts.
 */
export const requiredPolicies = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
  kind?: ApiKind,
): string[] => {
  const names = new Set<string>([SUBSCRIPTION_VALIDATION]);
  for (const plan of plans) {
    for (const condition of plan.conditions ?? []) {
      if (!appliesTo(condition, kind)) continue;
      const enforcer = enforcerOf(condition);
      if (enforcer) names.add(enforcer);
    }
  }
  return [...names];
};

/** Whether a condition is enforced on the API, not (yet attached), or has no policy at all. */
export type EnforcementStatus = 'enforced' | 'notEnforced' | 'noPolicy' | 'notApplicable';

/** One row of the "Required by subscription plans" notes: a built-in type, or a custom ID. */
export type EnforcementRow = {
  /** The condition type, or `custom`. */
  type: BuiltInType | 'custom';
  /** The condition's ID: fixed for a built-in type, chosen for a custom one. */
  id: string;
  /** The policy that enforces it, if any does: a name, or `name@v<major>` (see `policyKey`). */
  policy: string | null;
  status: EnforcementStatus;
  /** Not enforced because its policy, or subscription-validation, is on some resources only. */
  partial: boolean;
};

export type PlanEnforcement = {
  /** subscription-validation is not attached, so nothing of the plans is checked. */
  validationMissing: boolean;
  /** subscription-validation is on some resources only, so the others are not checked. */
  validationPartial: boolean;
  /** Built-in types in the API Portal's order, then custom IDs in the order the plans list them. */
  rows: EnforcementRow[];
  enforced: number;
  /** Rows whose condition applies to this kind of API: the total that `enforced` is out of. */
  applicable: number;
};

/**
 * What the plans an API offers need from its attached policies: one row per built-in condition
 * type any plan holds, then one per custom condition ID. A condition counts as enforced only
 * while subscription-validation is attached too, since nothing is checked without it, and only
 * when both apply to every request: a policy on some resources only (`partial`) does not count.
 */
export const planEnforcement = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
  attached: ReadonlySet<string>,
  partial: ReadonlySet<string> = new Set(),
  kind?: ApiKind,
): PlanEnforcement => {
  const validationAttached = attached.has(SUBSCRIPTION_VALIDATION);
  const validationPartial = !validationAttached && partial.has(SUBSCRIPTION_VALIDATION);
  const validationMissing = !validationAttached && !validationPartial;
  const conditions = plans.flatMap((plan) => plan.conditions ?? []);
  const rowOf = (type: EnforcementRow['type'], id: string, policy: string | null): EnforcementRow => {
    if (!policy) return { id, partial: false, policy, status: 'noPolicy', type };
    if (attached.has(policy) && validationAttached) {
      return { id, partial: false, policy, status: 'enforced', type };
    }
    const isPartial =
      (attached.has(policy) || partial.has(policy)) && (validationAttached || validationPartial);
    return { id, partial: isPartial, policy, status: 'notEnforced', type };
  };

  const builtInRows = BUILT_IN_TYPES.filter((type) =>
    conditions.some((condition) => condition.type === type),
  ).map((type) =>
    appliesTo({ type }, kind)
      ? rowOf(type, BUILT_IN_IDS[type], CONDITION_POLICIES.builtIn[type])
      : { id: BUILT_IN_IDS[type], partial: false, policy: null, status: 'notApplicable' as const, type },
  );
  // One row per custom ID, enforced by the policy the first condition with that ID names.
  const customById = new Map<string, (typeof conditions)[number]>();
  for (const condition of conditions) {
    if (!isBuiltInType(condition.type) && condition.id && !customById.has(condition.id)) {
      customById.set(condition.id, condition);
    }
  }
  const customRows = [...customById].map(([id, condition]) =>
    rowOf('custom', id, enforcerOf({ ...condition, id, type: 'custom' })),
  );
  const rows = [...builtInRows, ...customRows];
  return {
    applicable: rows.filter((row) => row.status !== 'notApplicable').length,
    enforced: rows.filter((row) => row.status === 'enforced').length,
    rows,
    validationMissing,
    validationPartial,
  };
};

/**
 * The condition types (and `custom`) the plans hold that no policy enforces yet, once each, in
 * the API Portal's order with `custom` last.
 */
export const unenforcedConditionTypes = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
  kind?: ApiKind,
): (BuiltInType | 'custom')[] => {
  const unenforced = plans
    .flatMap((plan) => plan.conditions ?? [])
    .filter((condition) => appliesTo(condition, kind) && enforcerOf(condition) === null);
  return [...BUILT_IN_TYPES, 'custom' as const].filter((type) =>
    unenforced.some((condition) =>
      type === 'custom' ? !isBuiltInType(condition.type) : condition.type === type,
    ),
  );
};
