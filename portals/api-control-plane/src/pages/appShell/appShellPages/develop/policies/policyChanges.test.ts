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

import { describe, expect, it } from 'vitest';

import { countPolicyChanges, type PolicyEdits } from './policyChanges';

const cors = { name: 'cors', params: {}, version: 'v1' };
const auth = { name: 'api-key-auth', params: { key: 'A' }, version: 'v1' };
const rate = { name: 'rate-limit', params: {}, version: 'v1' };

const edits = (
  apiPolicies: PolicyEdits['apiPolicies'],
  ...resources: PolicyEdits['apiPolicies'][]
) =>
  ({
    apiPolicies,
    operations: resources.map((policies) => ({ policies })),
  }) satisfies PolicyEdits;

describe('countPolicyChanges', () => {
  it('is zero when nothing differs', () => {
    expect(countPolicyChanges(edits([cors, auth], [rate]), edits([cors, auth], [rate]))).toBe(0);
  });

  it('counts each policy added or removed', () => {
    expect(countPolicyChanges(edits([cors]), edits([cors, auth]))).toBe(1);
    expect(countPolicyChanges(edits([cors, auth]), edits([cors]))).toBe(1);
    expect(countPolicyChanges(edits([]), edits([cors, auth, rate]))).toBe(3);
  });

  it('counts a changed policy once, not as a removal plus an addition', () => {
    const edited = { ...auth, params: { key: 'B' } };
    expect(countPolicyChanges(edits([auth]), edits([edited]))).toBe(1);
  });

  it('counts a pure reorder once', () => {
    expect(countPolicyChanges(edits([cors, auth]), edits([auth, cors]))).toBe(1);
  });

  it('adds up changes across the API and its resources', () => {
    expect(countPolicyChanges(edits([cors], [], [rate]), edits([], [auth], [rate]))).toBe(2);
  });

  it('treats a resource that appears or disappears as all-added or all-removed', () => {
    expect(countPolicyChanges(edits([], [rate]), edits([]))).toBe(1);
    expect(countPolicyChanges(edits([]), edits([], [rate, auth]))).toBe(2);
  });
});
