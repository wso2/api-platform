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

import { newlyAttachedPolicies } from './newlyAttachedPolicies';

const p = (name: string, version = 'v1') => ({ name, version });

describe('newlyAttachedPolicies', () => {
  it('reports nothing when nothing new is attached', () => {
    const saved = {
      apiPolicies: [p('cors'), p('api-key-auth')],
      operations: [{ method: 'GET', path: '/orders', policies: [p('basic-ratelimit')] }],
    };
    expect(newlyAttachedPolicies(saved, saved)).toEqual([]);
  });

  it('ignores removals, reorders and edits to saved policies', () => {
    const saved = { apiPolicies: [p('cors'), p('api-key-auth'), p('set-headers')] };
    const current = { apiPolicies: [p('api-key-auth', 'v2'), { ...p('cors'), params: { a: 1 } }] };
    expect(newlyAttachedPolicies(saved, current)).toEqual([]);
  });

  it('treats a removal and re-add of the same policy in one save as nothing new', () => {
    const saved = {
      apiPolicies: [p('cors')],
      operations: [{ method: 'GET', path: '/orders', policies: [p('basic-ratelimit')] }],
    };
    // Removed, then dropped back on, so it now sits at the end of the list.
    const current = {
      apiPolicies: [p('cors')],
      operations: [
        { method: 'GET', path: '/orders', policies: [p('set-headers'), p('basic-ratelimit')] },
      ],
    };
    expect(newlyAttachedPolicies(saved, current)).toEqual([
      { name: 'set-headers', scope: 'operation' },
    ]);
  });

  it('reports a policy added at API level, in attachment order', () => {
    expect(
      newlyAttachedPolicies(
        { apiPolicies: [p('cors')] },
        { apiPolicies: [p('cors'), p('jwt-auth'), p('basic-ratelimit')] },
      ),
    ).toEqual([
      { name: 'jwt-auth', scope: 'api' },
      { name: 'basic-ratelimit', scope: 'api' },
    ]);
  });

  it('reports a duplicated new policy once per scope', () => {
    const saved = {
      apiPolicies: [],
      operations: [
        { method: 'GET', path: '/orders', policies: [] },
        { method: 'POST', path: '/orders', policies: [] },
      ],
    };
    const current = {
      apiPolicies: [p('set-headers'), p('set-headers')],
      operations: [
        { method: 'GET', path: '/orders', policies: [p('basic-ratelimit')] },
        { method: 'POST', path: '/orders', policies: [p('basic-ratelimit')] },
      ],
    };
    expect(newlyAttachedPolicies(saved, current)).toEqual([
      { name: 'set-headers', scope: 'api' },
      { name: 'basic-ratelimit', scope: 'operation' },
    ]);
  });

  it('does not count a second copy of an already saved policy at the same scope', () => {
    expect(
      newlyAttachedPolicies({ apiPolicies: [p('cors')] }, { apiPolicies: [p('cors'), p('cors')] }),
    ).toEqual([]);
  });

  it('reports a saved policy attached again at another scope', () => {
    const saved = {
      apiPolicies: [p('jwt-auth')],
      operations: [{ method: 'GET', path: '/orders', policies: [p('basic-ratelimit')] }],
    };
    const current = {
      apiPolicies: [p('jwt-auth'), p('basic-ratelimit')],
      operations: [
        { method: 'GET', path: '/orders', policies: [p('basic-ratelimit'), p('jwt-auth')] },
      ],
    };
    expect(newlyAttachedPolicies(saved, current)).toEqual([
      { name: 'basic-ratelimit', scope: 'api' },
      { name: 'jwt-auth', scope: 'operation' },
    ]);
  });

  it('compares each operation with the same operation, matched by method and path', () => {
    const saved = {
      apiPolicies: [],
      operations: [
        { method: 'GET', path: '/orders', policies: [p('basic-ratelimit')] },
        { method: 'GET', path: '/items', policies: [] },
      ],
    };
    // Same operations in another order; only /items gains the policy.
    const current = {
      apiPolicies: [],
      operations: [
        { method: 'GET', path: '/items', policies: [p('basic-ratelimit')] },
        { method: 'GET', path: '/orders', policies: [p('basic-ratelimit')] },
      ],
    };
    expect(newlyAttachedPolicies(saved, current)).toEqual([
      { name: 'basic-ratelimit', scope: 'operation' },
    ]);
  });

  it('works for an API with no operations (GraphQL)', () => {
    expect(newlyAttachedPolicies({ apiPolicies: [] }, { apiPolicies: [p('cors')] })).toEqual([
      { name: 'cors', scope: 'api' },
    ]);
  });
});
