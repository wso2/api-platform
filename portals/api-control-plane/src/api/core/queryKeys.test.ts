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

import { hashKey } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';

import {
  createGlobalResourceKeys,
  createResourceKeys,
  normalizeParams,
  orgScope,
  scopeKey,
  type OrgScope,
} from './queryKeys';

/**
 * Query-key construction is the layer that keeps one organization's cached data
 * from being served to another, so these tests are written as guarantees about
 * *isolation* first and ergonomics second.
 *
 * `hashKey` is React Query's own key-serializer. Comparing hashes rather than
 * deep-equal arrays is deliberate: the cache looks up entries by hash, so two
 * keys that hash alike ARE the same cache entry, whatever they look like.
 */

const acme = orgScope('acme-org') as OrgScope;
const globex = orgScope('globex-org') as OrgScope;

const restApis = createResourceKeys('restApis');

describe('orgScope', () => {
  it('accepts a real organization id', () => {
    expect(orgScope('acme-org')).toBe('acme-org');
  });

  it.each([
    ['an empty string', ''],
    ['undefined', undefined],
    ['null', null],
  ])(
    'refuses to produce a scope from %s, so a query keyed on it cannot run',
    (_label, value) => {
      expect(orgScope(value)).toBeUndefined();
    }
  );
});

describe('cross-organization isolation', () => {
  /**
   * The defect this guards against: the previous layer built keys as
   * `queryKeys.projects(orgHandle || '')`, so any hook rendering before its org
   * resolved wrote into a shared `['projects', '']` bucket. Two organizations
   * could then read each other's cached list.
   */
  it('gives two organizations different keys for the same resource list', () => {
    expect(hashKey(restApis.list(acme, { projectId: 'p1' }))).not.toBe(
      hashKey(restApis.list(globex, { projectId: 'p1' }))
    );
  });

  it('gives two organizations different keys for the same resource id', () => {
    expect(hashKey(restApis.detail(acme, 'pizza-shack'))).not.toBe(
      hashKey(restApis.detail(globex, 'pizza-shack'))
    );
  });

  it('nests every resource key under its organization, so one org can be evicted alone', () => {
    // Prefix-based invalidation is how `removeQueries({ queryKey: scopeKey(org) })`
    // drops exactly one tenant. That only works if the org segment comes first.
    const orgPrefix = scopeKey(acme);
    const detail = restApis.detail(acme, 'pizza-shack');

    expect(detail.slice(0, orgPrefix.length)).toEqual([...orgPrefix]);
  });

  it('keeps a different org outside that prefix', () => {
    const orgPrefix = scopeKey(acme);
    const otherOrgDetail = restApis.detail(globex, 'pizza-shack');

    expect(otherOrgDetail.slice(0, orgPrefix.length)).not.toEqual([...orgPrefix]);
  });
});

describe('key hierarchy', () => {
  it('places a list under the resource root, so invalidating the root covers it', () => {
    const root = restApis.all(acme);
    const list = restApis.list(acme, { projectId: 'p1' });

    expect(list.slice(0, root.length)).toEqual([...root]);
  });

  it('places a detail under the resource root as well', () => {
    const root = restApis.all(acme);
    const detail = restApis.detail(acme, 'pizza-shack');

    expect(detail.slice(0, root.length)).toEqual([...root]);
  });

  it('places a sub-resource under its parent detail, so deleting the parent evicts its children', () => {
    const parent = restApis.detail(acme, 'pizza-shack');
    const children = restApis.child(acme, 'pizza-shack', 'deployments');

    expect(children.slice(0, parent.length)).toEqual([...parent]);
  });

  it('places every variant of a sub-resource under its `children` prefix', () => {
    // `child` is not a shared prefix; `children` is the common prefix.
    const prefix = restApis.children(acme, 'pizza-shack', 'deployments');

    for (const variant of [
      restApis.child(acme, 'pizza-shack', 'deployments'),
      restApis.child(acme, 'pizza-shack', 'deployments', { status: 'DEPLOYED' }),
      restApis.child(acme, 'pizza-shack', 'deployments', { deploymentId: 'dep-1' }),
    ]) {
      expect(variant.slice(0, prefix.length)).toEqual([...prefix]);
    }
  });

  it('keeps a `children` prefix clear of a sibling sub-resource', () => {
    const prefix = restApis.children(acme, 'pizza-shack', 'deployments');
    const sibling = restApis.child(acme, 'pizza-shack', 'api-keys');

    expect(sibling.slice(0, prefix.length)).not.toEqual([...prefix]);
  });

  it('keeps two sub-resources of the same parent distinct', () => {
    expect(hashKey(restApis.child(acme, 'pizza-shack', 'deployments'))).not.toBe(
      hashKey(restApis.child(acme, 'pizza-shack', 'api-keys'))
    );
  });

  it('keeps two resources distinct even when everything else matches', () => {
    const gateways = createResourceKeys('gateways');

    expect(hashKey(restApis.detail(acme, 'shared-id'))).not.toBe(
      hashKey(gateways.detail(acme, 'shared-id'))
    );
  });
});

describe('global resources — the few that exist outside any organization', () => {
  const organizations = createGlobalResourceKeys('organizations');

  it('keeps global keys outside every organization prefix', () => {
    // This is the guarantee the factory exists for: switching organizations
    // evicts `scopeKey(previousOrg)`, and that must not take the switcher's own
    // list of organizations with it.
    const globalList = organizations.list();

    expect(globalList.slice(0, scopeKey(acme).length)).not.toEqual([...scopeKey(acme)]);
    expect(globalList.slice(0, scopeKey(globex).length)).not.toEqual([...scopeKey(globex)]);
  });

  it('still nests its list and detail under one root, so the resource can be invalidated as a whole', () => {
    const root = organizations.all();

    expect(organizations.list().slice(0, root.length)).toEqual([...root]);
    expect(organizations.detail('acme-org').slice(0, root.length)).toEqual([...root]);
  });

  it('keeps two global resources distinct', () => {
    const regions = createGlobalResourceKeys('regions');

    expect(hashKey(organizations.detail('shared-id'))).not.toBe(
      hashKey(regions.detail('shared-id'))
    );
  });

  it('does not collide with an org-scoped resource of the same name', () => {
    const orgScopedOrganizations = createResourceKeys('organizations');

    expect(hashKey(organizations.detail('acme-org'))).not.toBe(
      hashKey(orgScopedOrganizations.detail(acme, 'acme-org'))
    );
  });
});

describe('normalizeParams', () => {
  /**
   * Two filter objects that mean the same request must produce the same cache
   * entry. Without normalization each variation below is a distinct key, so the
   * same data is fetched and stored several times over.
   */
  it.each([
    ['an explicitly undefined value', { projectId: 'p1', query: undefined }],
    ['a null value', { projectId: 'p1', query: null }],
    ['an empty string', { projectId: 'p1', query: '' }],
  ])('treats %s as absent', (_label, params) => {
    expect(normalizeParams(params)).toEqual(normalizeParams({ projectId: 'p1' }));
  });

  it('ignores the order keys were written in', () => {
    const written = normalizeParams({ sortBy: 'name', projectId: 'p1' });
    const reversed = normalizeParams({ projectId: 'p1', sortBy: 'name' });

    expect(hashKey([written])).toBe(hashKey([reversed]));
  });

  it('collapses a filter object with no meaningful values to undefined', () => {
    // Keeps `restApis.list(org)` and `restApis.list(org, {})` on one entry
    // rather than two identical ones.
    expect(normalizeParams({ query: '', sortBy: undefined })).toBeUndefined();
    expect(normalizeParams(undefined)).toBeUndefined();
  });

  it('keeps values that genuinely change the request', () => {
    expect(normalizeParams({ projectId: 'p1', limit: 20, sortBy: 'name' })).toEqual({
      limit: 20,
      projectId: 'p1',
      sortBy: 'name',
    });
  });

  it('keeps zero and false, which are meaningful values rather than empty ones', () => {
    // `offset: 0` is page one, not "no offset" — dropping it would collapse
    // page one and page two onto the same key.
    expect(normalizeParams({ offset: 0, latest: false })).toEqual({
      latest: false,
      offset: 0,
    });
  });

  it('distinguishes two genuinely different filters', () => {
    expect(hashKey(restApis.list(acme, { projectId: 'p1' }))).not.toBe(
      hashKey(restApis.list(acme, { projectId: 'p2' }))
    );
  });
});
