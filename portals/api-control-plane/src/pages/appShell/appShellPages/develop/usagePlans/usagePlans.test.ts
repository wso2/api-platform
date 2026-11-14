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
import { describe, expect, it } from 'vitest';

import {
  attachedPlanIdsOf,
  policiesToAttach,
  policiesToRemove,
  validationMove,
  validationToMove,
  withoutOperationValidation,
  withAttachedPlans,
  withPoliciesAdded,
  withPoliciesRemoved,
} from './usagePlans';

const window10 = { limits: [{ count: 10, timeAmount: 1, timeUnit: 'MINUTE' }], onExceed: 'reject' };
const requests = { conditions: [{ id: 'requests', params: window10, type: 'request-count' }] };
const tools = { conditions: [{ id: 'tool-calls', params: window10, type: 'tool-call-count' }] };
const none = { attached: new Set<string>(), partial: new Set<string>() };

describe('usage plans helpers', () => {
  it('reads the API’s plan references (or older bare handles) and writes { id } references', () => {
    expect(attachedPlanIdsOf({})).toEqual([]);
    expect(attachedPlanIdsOf({ subscriptionPlans: ['gold'] })).toEqual(['gold']);
    expect(
      attachedPlanIdsOf({
        subscriptionPlans: [
          { displayName: 'Gold', id: 'gold', sourceStatus: 'UNCHANGED' },
          { displayName: 'Silver', id: 'silver', sourceStatus: 'UPDATED' },
          { displayName: 'Old', id: 'old', sourceStatus: 'DELETED' },
          'bronze',
        ],
      }),
    ).toEqual(['gold', 'silver', 'bronze']);
    expect(withAttachedPlans({ id: 'a', subscriptionPlans: ['x'] }, ['gold', 'silver'])).toEqual({
      id: 'a',
      subscriptionPlans: [{ id: 'gold' }, { id: 'silver' }],
    });
  });

  it('offers subscription-validation first, and nothing already on every request', () => {
    expect(policiesToAttach([requests as never], 'rest', none)).toEqual(['subscription-validation']);
    expect(
      policiesToAttach([requests as never], 'rest', {
        attached: new Set(['subscription-validation']),
        partial: new Set(),
      }),
    ).toEqual([]);
  });

  it('does not offer a policy that is on some resources only', () => {
    expect(
      policiesToAttach([requests as never], 'rest', {
        attached: new Set(),
        partial: new Set(['subscription-validation']),
      }),
    ).toEqual([]);
  });

  it('offers removal only of what no remaining plan needs; subscription-validation once no plan is left', () => {
    const onApi = [{ name: 'subscription-validation', version: 'v1' }];
    expect(policiesToRemove(requests as never, [tools as never], 'rest', onApi)).toEqual([]);
    expect(policiesToRemove(requests as never, [], 'rest', onApi)).toEqual(['subscription-validation']);
    // Never offered when it is not an API-level policy.
    expect(policiesToRemove(requests as never, [], 'rest', [])).toEqual([]);
  });

  it('offers a custom condition’s own policy, attached at its version with no params', () => {
    const region = {
      conditions: [
        { id: 'region-limit', params: { policy: { name: 'region-guard', version: 'v2' } }, type: 'custom' },
      ],
    };
    const keys = policiesToAttach([region as never], 'rest', none);
    expect(keys).toEqual(['subscription-validation', 'region-guard@v2']);
    expect(withPoliciesAdded([], keys)).toEqual([
      { name: 'subscription-validation', params: { subscriptionKeyHeader: 'Subscription-Key' }, version: 'v1' },
      { name: 'region-guard', params: {}, version: 'v2' },
    ]);
    const onApi = withPoliciesAdded([], keys);
    expect(policiesToRemove(region as never, [], 'rest', onApi)).toEqual(keys);
    expect(withPoliciesRemoved(onApi, ['region-guard@v2'])).toHaveLength(1);
  });

  it('adds missing policies at the end with their defaults, and removes by name', () => {
    const cors = { name: 'cors', params: {}, version: 'v1' };
    const added = withPoliciesAdded([cors], ['subscription-validation']);
    expect(added).toEqual([
      cors,
      {
        name: 'subscription-validation',
        params: { subscriptionKeyHeader: 'Subscription-Key' },
        version: 'v1',
      },
    ]);
    expect(withPoliciesAdded(added, ['subscription-validation'])).toHaveLength(2);
    expect(withPoliciesRemoved(added, ['subscription-validation'])).toEqual([cors]);
  });

  it('adds a policy two keys name only once, with the first key’s defaults', () => {
    const added = withPoliciesAdded([], ['subscription-validation', 'subscription-validation@v1']);
    expect(added).toEqual([
      { name: 'subscription-validation', params: { subscriptionKeyHeader: 'Subscription-Key' }, version: 'v1' },
    ]);
  });

  it('adds each major version of a policy, and a built-in key at its default version', () => {
    const added = withPoliciesAdded([], ['region-guard@v1', 'region-guard@v2', 'region-guard', 'region-guard@v1']);
    expect(added.map((policy) => `${policy.name}@${policy.version}`)).toEqual(['region-guard@v1', 'region-guard@v2']);
    // Already on the API at v1: only v2 is added.
    const onApi = [{ name: 'region-guard', params: {}, version: 'v1' }];
    expect(withPoliciesAdded(onApi, ['region-guard@v1', 'region-guard@v2']).map((p) => p.version)).toEqual(['v1', 'v2']);
  });

  it('puts subscription-validation before the attached policies that depend on it', () => {
    const cors = { name: 'cors', params: {}, version: 'v1' };
    const guard = { name: 'region-guard', params: {}, version: 'v1' };
    const added = withPoliciesAdded([cors, guard], ['subscription-validation'], [
      'subscription-validation',
      'region-guard@v1',
    ]);
    expect(added.map((policy) => policy.name)).toEqual(['cors', 'subscription-validation', 'region-guard']);
    // With nothing depending on it, it goes at the end.
    expect(withPoliciesAdded([cors], ['subscription-validation']).map((p) => p.name)).toEqual([
      'cors',
      'subscription-validation',
    ]);
  });

  it('moves subscription-validation from the operations to API level, first, when a plan policy is added', () => {
    const onOps = [
      { policies: [{ name: 'subscription-validation', params: { subscriptionKeyHeader: 'X-Key' }, version: 'v1' }] },
      { policies: [{ name: 'cors', params: {}, version: 'v1' }, { name: 'subscription-validation', params: { subscriptionKeyHeader: 'X-Key' }, version: 'v1' }] },
    ];
    const moved = validationToMove([], onOps, ['subscription-request-ratelimit']);
    // As the operations had it: the same version and key header.
    expect(moved).toEqual({ name: 'subscription-validation', params: { subscriptionKeyHeader: 'X-Key' }, version: 'v1' });
    expect(withoutOperationValidation(onOps).map((op) => (op.policies ?? []).map((p) => p.name))).toEqual([[], ['cors']]);
    expect(
      withPoliciesAdded([moved as Policy], ['subscription-request-ratelimit']).map((p) => p.name),
    ).toEqual(['subscription-validation', 'subscription-request-ratelimit']);
    // Nothing moves when it is already at API level, on no operation, or nothing depends on it.
    const atApi: Policy[] = [{ name: 'subscription-validation', params: {}, version: 'v1' }];
    expect(validationToMove(atApi, onOps, ['subscription-request-ratelimit'])).toBeNull();
    expect(validationToMove([], [{ policies: [] }], ['subscription-request-ratelimit'])).toBeNull();
    expect(validationToMove([], onOps, ['subscription-validation'])).toBeNull();
    expect(validationMove([], onOps, ['region-guard@v1'])).toEqual({ kind: 'move', policy: onOps[0].policies[0] });
  });

  it('moves nothing when subscription-validation is on only some operations', () => {
    const someOps = [
      { policies: [{ name: 'subscription-validation', params: { subscriptionKeyHeader: 'X-Key' }, version: 'v1' }] },
      { policies: [{ name: 'cors', params: {}, version: 'v1' }] },
    ];
    expect(validationMove([], someOps, ['region-guard@v1'])).toEqual({ kind: 'partial' });
    expect(validationToMove([], someOps, ['region-guard@v1'])).toBeNull();
  });

  it('moves nothing when the operations configure subscription-validation differently', () => {
    const sv = (params: Record<string, unknown>, version = 'v1') => ({
      policies: [{ name: 'subscription-validation', params, version }],
    });
    expect(validationMove([], [sv({ subscriptionKeyHeader: 'X-Key' }), sv({ subscriptionKeyHeader: 'Y-Key' })], ['region-guard@v1'])).toEqual({ kind: 'differs' });
    expect(validationMove([], [sv({}, 'v1'), sv({}, 'v2')], ['region-guard@v1'])).toEqual({ kind: 'differs' });
    // The same parameters in another key order are the same configuration.
    const a = sv({ a: 1, b: { c: 2, d: 3 } });
    const b = sv({ b: { d: 3, c: 2 }, a: 1 });
    expect(validationMove([], [a, b], ['region-guard@v1'])?.kind).toBe('move');
    // Already at API level: nothing to decide.
    const atApi: Policy[] = [{ name: 'subscription-validation', params: {}, version: 'v1' }];
    expect(validationMove(atApi, [sv({ x: 1 }), { policies: [] }], ['region-guard@v1'])).toBeNull();
  });
});
