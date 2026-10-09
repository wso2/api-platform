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
import { apiThumbnailKeys, apiThumbnailParentId, apiThumbnailQueries } from './apiThumbnail.queries';

/**
 * Query-key tests for the thumbnail singleton. The hook test proves the blob
 * URL lifecycle; this file guards the key shape itself so a listing page
 * sharing one cache entry per API never breaks silently.
 */

const org = orgScope('acme')!;

describe('apiThumbnailParentId', () => {
  it('joins apiType and apiId so two kinds with the same handle do not share a thumbnail', () => {
    expect(apiThumbnailParentId('rest-api', 'orders')).toBe('rest-api/orders');
    expect(apiThumbnailParentId('rest-api', 'orders')).not.toBe(
      apiThumbnailParentId('mcp', 'orders')
    );
  });
});

describe('apiThumbnailQueries key shape', () => {
  it('nests the blob key under the API’s parent id', () => {
    const parentPrefix = apiThumbnailKeys.detail(org, apiThumbnailParentId('rest-api', 'orders'));
    const blob = apiThumbnailQueries.blob(org, 'rest-api', 'orders').queryKey;
    expect(blob.slice(0, parentPrefix.length)).toEqual([...parentPrefix]);
  });

  it('produces equal keys for the same API so the listing and detail page share one network call', () => {
    const a = apiThumbnailQueries.blob(org, 'rest-api', 'orders').queryKey;
    const b = apiThumbnailQueries.blob(org, 'rest-api', 'orders').queryKey;
    expect(a).toEqual(b);
  });

  it('produces different keys per API so one card’s delete cannot flip every other card to null', () => {
    const a = apiThumbnailQueries.blob(org, 'rest-api', 'orders').queryKey;
    const b = apiThumbnailQueries.blob(org, 'rest-api', 'returns').queryKey;
    expect(a).not.toEqual(b);
  });

  it('produces different keys per apiType so a REST and MCP API with the same handle are isolated', () => {
    const rest = apiThumbnailQueries.blob(org, 'rest-api', 'orders').queryKey;
    const mcp = apiThumbnailQueries.blob(org, 'mcp', 'orders').queryKey;
    expect(rest).not.toEqual(mcp);
  });

  it('disables retry — a 204 is a steady state, not a transient failure', () => {
    // Documented in the file: retrying the blob fetch after a 204 would flood
    // the network on every API with no thumbnail (the common case).
    expect(apiThumbnailQueries.blob(org, 'rest-api', 'orders').retry).toBe(false);
  });
});
