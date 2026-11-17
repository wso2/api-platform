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

type NamedPolicy = { name: string };

/**
 * The policies a policy panel saves: the API-level list and, for a REST API,
 * each operation's own list. Only the names are read.
 */
export type PolicyAttachments = {
  apiPolicies: readonly NamedPolicy[];
  operations?: ReadonlyArray<{ method: string; path: string; policies?: readonly NamedPolicy[] }>;
};

export type PolicyAttachment = { name: string; scope: 'api' | 'operation' };

const operationKey = (operation: { method: string; path: string }): string =>
  `${operation.method.toUpperCase()} ${operation.path}`;

/** Policy names per operation, merged when two entries share a method and path. */
const namesByOperation = (operations: PolicyAttachments['operations']) => {
  const names = new Map<string, Set<string>>();
  for (const operation of operations ?? []) {
    const key = operationKey(operation);
    const set = names.get(key) ?? new Set<string>();
    for (const policy of operation.policies ?? []) set.add(policy.name);
    names.set(key, set);
  }
  return names;
};

/**
 * The policies `after` attaches that `before` did not, compared by name only
 * (a save normalises versions, and a parameter edit is not a new attachment).
 * API-level policies are compared with the API-level list, and an operation's
 * with the same operation's (matched by method and path). Each name is listed
 * once per scope however many operations it was added to, in the order it
 * first appears; API-level ones come first. Removing a policy, reordering, or
 * removing and re-adding it in the same save attaches nothing new.
 */
export function newlyAttachedPolicies(
  before: PolicyAttachments,
  after: PolicyAttachments,
): PolicyAttachment[] {
  const attached: PolicyAttachment[] = [];
  const seen = new Set<string>();
  const add = (name: string, scope: PolicyAttachment['scope']) => {
    const key = `${scope}\u0000${name}`;
    if (seen.has(key)) return;
    seen.add(key);
    attached.push({ name, scope });
  };

  const apiBefore = new Set(before.apiPolicies.map((policy) => policy.name));
  for (const policy of after.apiPolicies) {
    if (!apiBefore.has(policy.name)) add(policy.name, 'api');
  }

  const operationsBefore = namesByOperation(before.operations);
  for (const operation of after.operations ?? []) {
    const savedNames = operationsBefore.get(operationKey(operation));
    for (const policy of operation.policies ?? []) {
      if (!savedNames?.has(policy.name)) add(policy.name, 'operation');
    }
  }

  return attached;
}
