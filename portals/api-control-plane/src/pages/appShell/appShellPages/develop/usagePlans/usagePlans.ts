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
  type ApiKind,
  parsePolicyKey,
  type PolicyAttachment,
  policyKey,
  requiredPolicies,
  SUBSCRIPTION_VALIDATION,
} from './planPolicies';

/**
 * The one place that reads and writes the plans an API offers (its usage plans). The REST API
 * stores them as plan handles in `subscriptionPlans`. When the Platform API stores each plan with
 * its details (`{displayName, id, sourceStatus}`), only these two functions change.
 */
export const attachedPlanIdsOf = (api: { subscriptionPlans?: readonly string[] }): string[] => [
  ...(api.subscriptionPlans ?? []),
];

/** The update body with the API's usage plans replaced by `planIds`. */
export const withAttachedPlans = <T extends object>(
  body: T,
  planIds: readonly string[],
): T & { subscriptionPlans: string[] } => ({ ...body, subscriptionPlans: [...planIds] });

/**
 * How a policy is attached when a plan asks for it: its version as the Policies page stores it,
 * and the parameters it starts with. A policy not listed here is attached at `v1` with none.
 */
export const POLICY_DEFAULTS: Readonly<Record<string, Pick<Policy, 'version' | 'params'>>> = {
  [SUBSCRIPTION_VALIDATION]: {
    params: { subscriptionKeyHeader: 'Subscription-Key' },
    version: 'v1',
  },
};

/**
 * A policy as a plan attaches it, at API level, from its key: a name (a built-in condition's
 * policy, with its defaults) or `name@v<major>` (a custom condition's policy, with no params).
 */
export const defaultPolicy = (key: string): Policy => {
  const { name, version } = parsePolicyKey(key);
  if (version) return { name, params: {}, version };
  return { name, ...(POLICY_DEFAULTS[name] ?? { params: {}, version: 'v1' }) };
};

/** The keys a list of policies answers to: each by name, and by name at its major version. */
const keysOf = (policies: readonly Policy[]) =>
  new Set(policies.flatMap((policy) => [policy.name, policyKey(policy.name, policy.version || 'v1')]));

/**
 * The policies to offer when `plans` are ticked: those they need that apply to no request of the
 * API yet, subscription-validation first, since the policies after it read what it records. A
 * policy on some resources only is not offered: attaching it to the API as well would run it twice
 * on those resources, so the notes ask for it to be moved instead.
 */
export const policiesToAttach = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
  kind: ApiKind,
  attachment: PolicyAttachment,
): string[] =>
  requiredPolicies(plans, kind).filter(
    (name) => !attachment.attached.has(name) && !attachment.partial.has(name),
  );

/**
 * The API-level policies to offer for removal when `removed` is unticked: those it needed that no
 * plan still ticked needs. subscription-validation is among them only when no plan is left.
 */
export const policiesToRemove = (
  removed: Pick<SubscriptionPlan, 'conditions'>,
  remaining: readonly Pick<SubscriptionPlan, 'conditions'>[],
  kind: ApiKind,
  apiPolicies: readonly Policy[],
): string[] => {
  const stillNeeded = new Set(remaining.length > 0 ? requiredPolicies(remaining, kind) : []);
  const onApi = keysOf(apiPolicies);
  return requiredPolicies([removed], kind).filter(
    (name) => onApi.has(name) && !stillNeeded.has(name),
  );
};

/**
 * The policies the ticked `plans` need that do not apply to every request of the API yet (missing,
 * or on some resources only), subscription-validation first.
 */
export const missingPolicies = (
  plans: readonly Pick<SubscriptionPlan, 'conditions'>[],
  kind: ApiKind,
  attachment: PolicyAttachment,
): string[] =>
  plans.length === 0 ? [] : requiredPolicies(plans, kind).filter((name) => !attachment.attached.has(name));

/**
 * `policies` with the policies of `keys` added, each once: two keys for the same policy (a custom
 * condition naming `subscription-validation@v1` next to the built-in `subscription-validation`) add
 * it once, with the first key's defaults. subscription-validation goes before the first attached
 * policy in `dependents` (the policies the plans need, which read what it records); the others are
 * added at the end, in order.
 */
export const withPoliciesAdded = (
  policies: readonly Policy[],
  keys: readonly string[],
  dependents: readonly string[] = [],
): Policy[] => {
  const present = keysOf(policies);
  const addedNames = new Set<string>();
  const added: Policy[] = [];
  for (const key of keys) {
    if (present.has(key)) continue;
    const policy = defaultPolicy(key);
    if (addedNames.has(policy.name)) continue;
    addedNames.add(policy.name);
    added.push(policy);
  }
  const validation = added.find((policy) => policy.name === SUBSCRIPTION_VALIDATION);
  const rest = added.filter((policy) => policy !== validation);
  if (!validation) return [...policies, ...rest];
  const dependent = new Set(dependents.filter((key) => parsePolicyKey(key).name !== SUBSCRIPTION_VALIDATION));
  const at = policies.findIndex((policy) => [...keysOf([policy])].some((key) => dependent.has(key)));
  if (at < 0) return [...policies, validation, ...rest];
  return [...policies.slice(0, at), validation, ...policies.slice(at), ...rest];
};

/** `policies` without any matching `keys` (a name, or `name@v<major>`). */
export const withPoliciesRemoved = (
  policies: readonly Policy[],
  keys: readonly string[],
): Policy[] => {
  const drop = new Set(keys);
  return policies.filter(
    (policy) =>
      !drop.has(policy.name) && !drop.has(policyKey(policy.name, policy.version || 'v1')),
  );
};
