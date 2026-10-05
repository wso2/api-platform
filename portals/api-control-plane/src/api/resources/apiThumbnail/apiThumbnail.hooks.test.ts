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

import { waitFor } from '@testing-library/react';
import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  apiUrl,
  failure,
  noContent,
  recorder,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient, type BlobResponse } from '../../core/http';
import {
  useApiThumbnail,
  useDeleteApiThumbnail,
  useUpsertApiThumbnail,
} from './apiThumbnail.hooks';
import { apiThumbnailQueries } from './apiThumbnail.queries';

/**
 * Hook-layer tests for API thumbnails.
 *
 * The thumbnail hook has three behaviours a component test can't see on its
 * own, so they live here:
 *
 *  1. The `<img src>` URL is a `blob:` URL built from the fetched Blob. The
 *     component only renders a string — it can't tell "the URL points at the
 *     right bytes" from "the URL points anywhere at all". Here we assert that
 *     a `blob:` URL is produced when a Blob is cached and that it is
 *     `URL.revokeObjectURL`'d on unmount, so the handle doesn't leak.
 *  2. A 404 from the server resolves to a `null` cache entry and a `url` of
 *     `undefined` — not an error. This is what lets a listing page render
 *     initials fallback instead of 20 error toasts.
 *  3. The delete mutation uses `setQueryData(key, null)` rather than
 *     `invalidateQueries`. A refetch after invalidation would 404 and React
 *     Query would keep the previous blob on screen until a page refresh.
 */

const API_TYPE = 'rest-api';
const API_ID = 'orders-api';
const PATH = `/apis/${API_TYPE}/${API_ID}/thumbnail`;
const PNG_BYTES = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

let requests: Recorder;
let createdObjectURLs: string[];
let revokedObjectURLs: string[];

// jsdom has no URL.createObjectURL / URL.revokeObjectURL at all, so install
// stubs once at file scope rather than per test. Per-test restore would wipe
// them while React's passive cleanup effects (which fire during unmount
// scheduled by the test harness) are still trying to revoke a URL.
createdObjectURLs = [];
revokedObjectURLs = [];
URL.createObjectURL = vi.fn((_blob: Blob | MediaSource): string => {
  const url = `blob:test/${createdObjectURLs.length}`;
  createdObjectURLs.push(url);
  return url;
});
URL.revokeObjectURL = vi.fn((url: string) => {
  revokedObjectURLs.push(url);
});

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  createdObjectURLs.length = 0;
  revokedObjectURLs.length = 0;
});

const servePng = () => {
  server.use(
    mswHttp.get(apiUrl(PATH), async ({ request }) => {
      await requests.capture(request);
      return new HttpResponse(PNG_BYTES, { headers: { 'Content-Type': 'image/png' } });
    })
  );
};

const serveNoThumbnail = () => {
  server.use(failure('get', PATH, 404, 'NOT_FOUND', { record: requests }));
};

describe('useApiThumbnail — fetch and lifecycle', () => {
  it('does not fetch while the apiId is unknown', async () => {
    servePng();

    renderApiHook(() => useApiThumbnail(API_TYPE, undefined));
    await settle();

    expect(requests.count()).toBe(0);
  });

  it('builds a blob: URL from the fetched Blob', async () => {
    servePng();

    const { result } = renderApiHook(() => useApiThumbnail(API_TYPE, API_ID));

    await waitFor(() => expect(result.current.url).toBeDefined());
    expect(result.current.url).toMatch(/^blob:/);
    expect(createdObjectURLs).toHaveLength(1);
  });

  it('releases the blob: URL on unmount so the handle does not leak', async () => {
    servePng();

    const { result, unmount } = renderApiHook(() => useApiThumbnail(API_TYPE, API_ID));
    await waitFor(() => expect(result.current.url).toBeDefined());
    const createdUrl = result.current.url;

    unmount();
    // The hook defers revocation via `setTimeout(0)` so a quick remount can
    // reuse the same URL — wait a beat for that microtask to fire before
    // asserting the revoke actually happened.
    await settle();

    // The URL built for this component must have been revoked at unmount.
    // Without this check, a long-lived listing page that mounts and unmounts
    // 20 thumbnail avatars per filter would accumulate blob URLs the GC
    // can't reclaim because the browser keeps the backing bytes alive.
    expect(revokedObjectURLs).toContain(createdUrl);
  });

  it('surfaces 404 as a null url, no error', async () => {
    // The listing page uses `url` to decide between an `<img>` and an initials
    // fallback. A thrown error on this hook would break every API card with
    // no custom thumbnail — which is the common case.
    serveNoThumbnail();

    const { result } = renderApiHook(() => useApiThumbnail(API_TYPE, API_ID));

    await waitFor(() => expect(result.current.isPending).toBe(false));
    expect(result.current.url).toBeUndefined();
    expect(result.current.error).toBeNull();
  });
});

describe('useUpsertApiThumbnail', () => {
  it('invalidates the thumbnail query on success so a live <img> refetches', async () => {
    server.use(noContent('put', PATH));

    const { result, queryClient, org } = renderApiHook(() => useUpsertApiThumbnail());
    const key = apiThumbnailQueries.blob(org, API_TYPE, API_ID).queryKey;
    // The tagged queryKey's `Updater<BlobResponse | null | undefined, ...>`
    // doesn't narrow a plain object literal correctly in current @tanstack/
    // react-query typings. Casting the typed seed through `as BlobResponse`
    // keeps the runtime payload identical while satisfying the signature.
    const seed: BlobResponse = { blob: new Blob([PNG_BYTES]), contentType: 'image/png' };
    queryClient.setQueryData(key, seed as BlobResponse);

    result.current.mutate({
      apiType: API_TYPE,
      apiId: API_ID,
      file: new Blob([PNG_BYTES], { type: 'image/png' }),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Invalidation (not setQueryData(null)) is correct for upsert: a GET
    // following the invalidate will return the newly-stored bytes, which the
    // hook needs so the img switches to the new thumbnail.
    await waitFor(() => expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true));
  });
});

describe('useDeleteApiThumbnail', () => {
  it('flips the cached blob to null without triggering a refetch', async () => {
    // The subtle one. A naive invalidate-then-refetch would send a GET that
    // returns 404; React Query treats a thrown-from-queryFn as an error and
    // keeps the previous `data` on screen (the user has to refresh the page
    // for the stale thumbnail to disappear). Setting the cache value to
    // `null` directly flips every live `<img>` to the initials fallback on
    // the next render with zero extra network calls.
    server.use(noContent('delete', PATH));
    // No GET handler is registered. If the hook tried to refetch, the test
    // would fail with onUnhandledRequest: 'error'.

    const { result, queryClient, org } = renderApiHook(() => useDeleteApiThumbnail());
    const key = apiThumbnailQueries.blob(org, API_TYPE, API_ID).queryKey;
    const seed: BlobResponse = { blob: new Blob([PNG_BYTES]), contentType: 'image/png' };
    queryClient.setQueryData(key, seed as BlobResponse);

    result.current.mutate({ apiType: API_TYPE, apiId: API_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(key)).toBeNull();
  });

  it('leaves another API’s cached thumbnail untouched', async () => {
    // A delete key is `[... , apiType/apiId , 'blob']`. If a regression wrote
    // to a broader key (e.g. the resource root), every API's thumbnail cache
    // in that tenant would flip to null at once.
    server.use(noContent('delete', PATH));

    const { result, queryClient, org } = renderApiHook(() => useDeleteApiThumbnail());
    const targetKey = apiThumbnailQueries.blob(org, API_TYPE, API_ID).queryKey;
    const otherKey = apiThumbnailQueries.blob(org, API_TYPE, 'other-api').queryKey;
    const seed: BlobResponse = { blob: new Blob([PNG_BYTES]), contentType: 'image/png' };
    queryClient.setQueryData(targetKey, seed as BlobResponse);
    queryClient.setQueryData(otherKey, seed as BlobResponse);

    result.current.mutate({ apiType: API_TYPE, apiId: API_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(targetKey)).toBeNull();
    expect(queryClient.getQueryData(otherKey)).not.toBeNull();
  });
});
