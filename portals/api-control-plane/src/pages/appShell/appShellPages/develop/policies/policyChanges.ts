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

/** The policies the page edits: API-level ones, and each resource's own. */
export type PolicyEdits = {
  apiPolicies: Policy[];
  operations: Array<{ policies?: Policy[] }>;
};

/**
 * How many edits turn `saved` into `current` for one list of policies. A policy added or
 * removed is one edit; changing a policy's parameters shows up as one removal plus one
 * addition and still counts once; the same policies in a new order count once.
 */
const countListChanges = (saved: Policy[], current: Policy[]): number => {
  const before = saved.map((policy) => JSON.stringify(policy));
  const after = current.map((policy) => JSON.stringify(policy));

  const unmatched = [...before];
  let added = 0;
  after.forEach((entry) => {
    const at = unmatched.indexOf(entry);
    if (at === -1) added += 1;
    else unmatched.splice(at, 1);
  });

  const edits = Math.max(added, unmatched.length);
  if (edits > 0) return edits;
  return before.some((entry, index) => entry !== after[index]) ? 1 : 0;
};

/** The number of unsaved edits across the API-level list and every resource's list. */
export const countPolicyChanges = (saved: PolicyEdits, current: PolicyEdits): number => {
  const resources = Math.max(saved.operations.length, current.operations.length);
  let total = countListChanges(saved.apiPolicies, current.apiPolicies);
  for (let index = 0; index < resources; index += 1) {
    total += countListChanges(
      saved.operations[index]?.policies ?? [],
      current.operations[index]?.policies ?? [],
    );
  }
  return total;
};
