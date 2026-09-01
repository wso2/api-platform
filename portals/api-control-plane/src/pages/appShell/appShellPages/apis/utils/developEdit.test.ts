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

import type { Policy } from '@/api/resources/restApis';
import { aRestApi } from '@/test/msw';
import {
  addOperation,
  type EditableOperation,
  addPolicy,
  backendPathsFromOperations,
  getBackendPath,
  isValidUrl,
  methodColor,
  operationsValid,
  policiesValid,
  movePolicy,
  removeOperation,
  removePolicy,
  reorderPolicies,
  replacePolicy,
  setBackendPath,
  toEditableOperations,
  toSpecOperations,
  updateOperation,
  updatePolicy,
  withPolicyEdits,
  withRoutingEdits,
} from './developEdit';

describe('developEdit operations', () => {
  it('adds a default GET / operation', () => {
    const ops = addOperation([]);
    expect(ops).toEqual([{ method: 'GET', path: '/' }]);
  });

  it('updates and removes operations immutably', () => {
    const start: EditableOperation[] = [
      { method: 'GET', path: '/a' },
      { method: 'POST', path: '/b' },
    ];
    const updated = updateOperation(start, 1, { method: 'PUT' });
    expect(updated[1]).toEqual({ method: 'PUT', path: '/b' });
    expect(start[1].method).toBe('POST'); // original untouched

    expect(removeOperation(start, 0)).toEqual([{ method: 'POST', path: '/b' }]);
  });

  it('validates leading-slash paths', () => {
    expect(operationsValid([{ method: 'GET', path: '/ok' }])).toBe(true);
    expect(operationsValid([{ method: 'GET', path: 'bad' }])).toBe(false);
    expect(operationsValid([])).toBe(true);
  });
});

describe('developEdit policies', () => {
  it('adds, updates, removes policies', () => {
    let policies: Policy[] = addPolicy([]);
    expect(policies[0]).toEqual({ name: '', version: '1.0.0' });
    policies = updatePolicy(policies, 0, { name: 'SET_HEADER' });
    expect(policies[0].name).toBe('SET_HEADER');
    expect(removePolicy(policies, 0)).toEqual([]);
  });

  it('requires name + version to be valid', () => {
    expect(policiesValid([{ name: 'X', version: '1.0.0' }])).toBe(true);
    expect(policiesValid([{ name: '', version: '1.0.0' }])).toBe(false);
    expect(policiesValid([{ name: 'X', version: '' }])).toBe(false);
  });

  it('replaces a policy at an index immutably', () => {
    const start: Policy[] = [
      { name: 'a', version: '1' },
      { name: 'b', version: '1' },
    ];
    const next = replacePolicy(start, 1, {
      name: 'b2',
      version: '2',
      params: { x: 1 },
    });
    expect(next[1]).toEqual({ name: 'b2', version: '2', params: { x: 1 } });
    expect(start[1].name).toBe('b'); // original untouched
  });

  it('moves a policy up/down and is a no-op at the edges', () => {
    const start: Policy[] = [
      { name: 'a', version: '1' },
      { name: 'b', version: '1' },
      { name: 'c', version: '1' },
    ];
    expect(movePolicy(start, 2, -1).map((p) => p.name)).toEqual(['a', 'c', 'b']);
    expect(movePolicy(start, 0, 1).map((p) => p.name)).toEqual(['b', 'a', 'c']);
    expect(movePolicy(start, 0, -1)).toBe(start); // no-op at top
    expect(movePolicy(start, 2, 1)).toBe(start); // no-op at bottom
  });

  it('reorders a policy from one index to another (drag-drop)', () => {
    const start: Policy[] = [
      { name: 'a', version: '1' },
      { name: 'b', version: '1' },
      { name: 'c', version: '1' },
    ];
    expect(reorderPolicies(start, 0, 2).map((p) => p.name)).toEqual(['b', 'c', 'a']);
    expect(reorderPolicies(start, 2, 0).map((p) => p.name)).toEqual(['c', 'a', 'b']);
    expect(reorderPolicies(start, 1, 1)).toBe(start); // no-op same index
    expect(reorderPolicies(start, 0, 9)).toBe(start); // no-op out of range
  });
});

describe('developEdit helpers', () => {
  it('maps HTTP methods to distinct chip colors', () => {
    expect(methodColor('GET')).toBe('success');
    expect(methodColor('post')).toBe('primary');
    expect(methodColor('PUT')).toBe('warning');
    expect(methodColor('DELETE')).toBe('error');
    expect(methodColor('PATCH')).toBe('secondary');
    expect(methodColor('HEAD')).toBe('default');
  });

  it('accepts empty and well-formed http(s) URLs, rejects others', () => {
    expect(isValidUrl('')).toBe(true);
    expect(isValidUrl('https://backend.example.com/api')).toBe(true);
    expect(isValidUrl('http://localhost:5000')).toBe(true);
    expect(isValidUrl('not a url')).toBe(false);
    expect(isValidUrl('ftp://x.com')).toBe(false);
  });
});

describe('backend-resource mapping', () => {
  const op: EditableOperation = { method: 'GET', path: '/sample' };

  it('maps a proxy resource to a backend path via a rewrite policy', () => {
    const mapped = setBackendPath(op, '/book');
    expect(getBackendPath(mapped)).toBe('/book');
    expect(mapped.policies?.[0]).toMatchObject({
      name: 'mediation.rewrite_resource_path',
      params: { 'New Path': '/book' },
    });
  });

  it('clears the mapping (removes the rewrite policy) without touching others', () => {
    const withOther: EditableOperation = {
      ...op,
      policies: [{ name: 'cors', version: '1.0.0' }],
    };
    const mapped = setBackendPath(withOther, '/book');
    expect(mapped.policies).toHaveLength(2);
    const cleared = setBackendPath(mapped, undefined);
    expect(getBackendPath(cleared)).toBeUndefined();
    expect(cleared.policies).toEqual([{ name: 'cors', version: '1.0.0' }]);
  });

  it('collects distinct mapped backend paths across operations', () => {
    const ops = [
      setBackendPath({ method: 'GET', path: '/a' }, '/book'),
      setBackendPath({ method: 'POST', path: '/b' }, '/book'),
      setBackendPath({ method: 'GET', path: '/c' }, '/author'),
      { method: 'GET', path: '/d' } as EditableOperation,
    ];
    expect(backendPathsFromOperations(ops).sort()).toEqual(['/author', '/book']);
  });
});

describe('editing model ↔ spec shape', () => {
  it('flattens request fields onto the operation and nests them back', () => {
    const api = aRestApi({
      operations: [
        {
          name: 'listOrders',
          description: 'All orders',
          request: { method: 'POST', path: '/orders', policies: [{ name: 'cors', version: '1' }] },
        },
      ],
    });

    const editable = toEditableOperations(api);
    expect(editable).toEqual([
      {
        name: 'listOrders',
        description: 'All orders',
        method: 'POST',
        path: '/orders',
        policies: [{ name: 'cors', version: '1' }],
      },
    ]);

    // The round trip is lossless — nothing the spec carries is dropped by the
    // flattening the panels edit through.
    expect(toSpecOperations(editable)).toEqual(api.operations);
  });

  it('treats a missing operations list as empty rather than throwing', () => {
    expect(toEditableOperations(aRestApi({ operations: undefined }))).toEqual([]);
  });
});

describe('withRoutingEdits', () => {
  it('preserves upstream auth while replacing the URL', () => {
    const api = aRestApi({
      upstream: {
        main: {
          url: 'https://old.test',
          auth: { type: 'api-key', header: 'X-Key', value: 's3cret' },
        },
      },
    });

    const body = withRoutingEdits(api, { operations: [], prodUrl: 'https://new.test' });

    expect(body.upstream.main).toEqual({
      url: 'https://new.test',
      auth: { type: 'api-key', header: 'X-Key', value: 's3cret' },
    });
  });

  it('clears `ref` when a URL is set, since the spec makes them exclusive', () => {
    const api = aRestApi({ upstream: { main: { ref: 'shared-upstream' } } });
    const body = withRoutingEdits(api, { operations: [], prodUrl: 'https://new.test' });
    expect(body.upstream.main).toEqual({ url: 'https://new.test' });
  });

  it('keeps a `ref` when the URL is cleared', () => {
    const api = aRestApi({
      upstream: { main: { ref: 'shared-upstream', url: 'https://old.test' } },
    });
    const body = withRoutingEdits(api, { operations: [], prodUrl: '' });
    expect(body.upstream.main).toEqual({ ref: 'shared-upstream' });
  });

  it('drops the sandbox side when its URL is cleared', () => {
    const api = aRestApi({
      upstream: { main: { url: 'https://prod.test' }, sandbox: { url: 'https://sbx.test' } },
    });
    expect(
      withRoutingEdits(api, { operations: [], prodUrl: 'https://prod.test', sandboxUrl: '' })
        .upstream.sandbox,
    ).toBeUndefined();
  });

  it('carries every other field of the fetched API through untouched', () => {
    const api = aRestApi({ description: 'Ordering', policies: [{ name: 'cors', version: '1' }] });
    const body = withRoutingEdits(api, { operations: [], prodUrl: 'https://new.test' });
    // The spec's update body is the whole RESTAPI, so anything the panel does
    // not edit must survive the PUT — this is what replaced the old `raw` merge.
    expect(body.description).toBe('Ordering');
    expect(body.policies).toEqual([{ name: 'cors', version: '1' }]);
    expect(body.version).toBe(api.version);
  });
});

describe('withPolicyEdits', () => {
  it('replaces API-level and per-operation policies, leaving upstream alone', () => {
    const api = aRestApi({
      upstream: { main: { url: 'https://prod.test', auth: { type: 'bearer', value: 't' } } },
      policies: [{ name: 'old', version: '1' }],
    });

    const body = withPolicyEdits(api, {
      policies: [{ name: 'new', version: '2' }],
      operations: [{ method: 'GET', path: '/a', policies: [{ name: 'op', version: '1' }] }],
    });

    expect(body.policies).toEqual([{ name: 'new', version: '2' }]);
    expect(body.operations?.[0].request.policies).toEqual([{ name: 'op', version: '1' }]);
    expect(body.upstream).toEqual(api.upstream);
  });
});
