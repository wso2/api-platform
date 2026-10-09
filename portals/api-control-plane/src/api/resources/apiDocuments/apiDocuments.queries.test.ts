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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it } from 'vitest';

import { orgScope } from '../../core/queryKeys';
import {
  apiDocumentKeys,
  apiDocumentParentId,
  apiDocumentQueries,
  nextDocumentsOffset,
} from './apiDocuments.queries';

/**
 * Query-key factories are hashed by TanStack Query to decide what is one entry
 * and what is two. Any drift here silently splits or merges cache entries, so
 * the shape rules are asserted directly rather than inferred from a hook test.
 */

const org = orgScope('acme')!;

describe('apiDocumentParentId', () => {
  it('joins apiType and apiId so two API kinds with the same handle do not share cache entries', () => {
    // A handle is unique only within its kind: a REST API and an MCP API can
    // both be `orders`. Dropping the kind from the parent id would let their
    // documents collide in the cache.
    expect(apiDocumentParentId('rest-api', 'orders')).toBe('rest-api/orders');
    expect(apiDocumentParentId('mcp', 'orders')).toBe('mcp/orders');
    expect(apiDocumentParentId('rest-api', 'orders')).not.toBe(
      apiDocumentParentId('mcp', 'orders')
    );
  });
});

describe('apiDocumentQueries key shape', () => {
  it('nests every variant under one API detail so a parent invalidation covers them', () => {
    // apiDocumentKeys is a child-resource factory: all children sit under
    // ['platform', org, 'apiDocuments', 'detail', '<apiType/apiId>'], and
    // `useInvalidateApiDocuments` invalidates exactly that prefix. If any
    // variant below has a different prefix, that one invalidation would leak
    // stale pages onto the UI.
    const parentPrefix = apiDocumentKeys.detail(org, apiDocumentParentId('rest-api', 'orders'));

    const list = apiDocumentQueries.list(org, 'rest-api', 'orders', { limit: 20 }).queryKey;
    const pages = apiDocumentQueries.pages(org, 'rest-api', 'orders', { type: 'HowTo' })
      .queryKey;
    const detail = apiDocumentQueries.detail(org, 'rest-api', 'orders', 'getting-started')
      .queryKey;
    const content = apiDocumentQueries.content(org, 'rest-api', 'orders', 'getting-started')
      .queryKey;

    for (const key of [list, pages, detail, content]) {
      expect(key.slice(0, parentPrefix.length)).toEqual([...parentPrefix]);
    }
  });

  it('produces structurally distinct keys for each variant', () => {
    const list = apiDocumentQueries.list(org, 'rest-api', 'orders').queryKey;
    const pages = apiDocumentQueries.pages(org, 'rest-api', 'orders').queryKey;
    const detail = apiDocumentQueries.detail(org, 'rest-api', 'orders', 'a').queryKey;
    const content = apiDocumentQueries.content(org, 'rest-api', 'orders', 'a').queryKey;

    const serialised = new Set([list, pages, detail, content].map((k) => JSON.stringify(k)));
    expect(serialised.size).toBe(4);
  });

  it('two detail requests for the same doc produce equal keys so React Query dedupes', () => {
    // `toEqual` is intentional — React Query hashes the key value, not the
    // reference. Equal-by-value keys must be one entry in the cache.
    const a = apiDocumentQueries.detail(org, 'rest-api', 'orders', 'getting-started').queryKey;
    const b = apiDocumentQueries.detail(org, 'rest-api', 'orders', 'getting-started').queryKey;
    expect(a).toEqual(b);
    expect(a).not.toBe(b);
  });

  it('a filter difference produces a different list key', () => {
    const noFilter = apiDocumentQueries.list(org, 'rest-api', 'orders').queryKey;
    const howtoFilter = apiDocumentQueries.list(org, 'rest-api', 'orders', { type: 'HowTo' })
      .queryKey;
    expect(noFilter).not.toEqual(howtoFilter);
  });

  it('normalises filters that only differ by explicit-undefined, so one cache entry serves both', () => {
    // `{ type: undefined }` and `{}` are the same request. If the key factory
    // treated them differently every unfiltered page would double-fetch.
    const noFilter = apiDocumentQueries.list(org, 'rest-api', 'orders').queryKey;
    const emptyExplicit = apiDocumentQueries.list(org, 'rest-api', 'orders', {}).queryKey;
    const undefinedValue = apiDocumentQueries.list(org, 'rest-api', 'orders', { type: undefined })
      .queryKey;
    expect(noFilter).toEqual(emptyExplicit);
    expect(noFilter).toEqual(undefinedValue);
  });

  it('a different docId produces a different content key', () => {
    const docA = apiDocumentQueries.content(org, 'rest-api', 'orders', 'a').queryKey;
    const docB = apiDocumentQueries.content(org, 'rest-api', 'orders', 'b').queryKey;
    expect(docA).not.toEqual(docB);
  });
});

describe('nextDocumentsOffset', () => {
  // Covered by one case in the hooks test alongside the integration behaviour,
  // repeated here as pure-input/output pairs so a regression surfaces on the
  // smaller test first.
  const page = (
    offset: number,
    limit: number,
    total: number,
    length = limit
  ) => ({
    count: length,
    list: Array.from({ length }, (_, i) => ({
      id: `doc-${offset}-${i}`,
      type: 'HowTo' as const,
      displayName: 'd',
    })),
    pagination: { offset, limit, total },
  });

  it('returns the next offset when more pages remain', () => {
    expect(nextDocumentsOffset(page(0, 5, 10))).toBe(5);
  });

  it('returns undefined when total is reached', () => {
    expect(nextDocumentsOffset(page(5, 5, 10))).toBeUndefined();
  });

  it('returns undefined for an empty page (nothing to append to)', () => {
    expect(nextDocumentsOffset(page(0, 20, 0, 0))).toBeUndefined();
  });

  it('returns undefined when the short-page math alone would hide the end', () => {
    // 7 of 7 arrived in two pages of 5 and 2. The second page's `list.length`
    // is less than limit but that is a legitimate end — using
    // `list.length < limit` to detect "end" would also stop early mid-list on
    // a sparse page. The implementation uses total; this test asserts that.
    expect(nextDocumentsOffset(page(5, 5, 7, 2))).toBeUndefined();
  });
});
