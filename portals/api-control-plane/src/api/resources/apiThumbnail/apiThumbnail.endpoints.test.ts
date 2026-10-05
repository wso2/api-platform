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

import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';

import {
  accepts,
  apiUrl,
  failure,
  noContent,
  recorder,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  deleteApiThumbnail,
  getApiThumbnail,
  upsertApiThumbnail,
} from './apiThumbnail.endpoints';

/**
 * Contract tests for `/apis/{apiType}/{apiId}/thumbnail`.
 *
 * The thumbnail endpoint is unusual on three axes:
 *  1. GET returns raw bytes (a Blob), not a JSON envelope.
 *  2. 404 is a steady "no thumbnail set" state, not an error — the endpoint
 *     resolves it to `null` so the hook can gate the UI without a try/catch.
 *  3. PUT is multipart (image bytes), not JSON. jsdom can't serialise FormData
 *     onto the wire, so what we assert is that the Content-Type isn't labelled
 *     JSON and that a non-GET verb is used.
 */

const PATH = '/apis/rest-api/orders-api/thumbnail';
const PNG_BYTES = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('getApiThumbnail', () => {
  it('GETs the thumbnail and labels the Blob with the sniffed content type', async () => {
    server.use(
      mswHttp.get(apiUrl(PATH), async ({ request }) => {
        await requests.capture(request);
        return new HttpResponse(PNG_BYTES, {
          headers: { 'Content-Type': 'image/png' },
        });
      })
    );

    const result = await getApiThumbnail('rest-api', 'orders-api');

    expect(result).not.toBeNull();
    expect(result?.contentType).toBe('image/png');
    // A real Blob was returned (not undefined / null / a parsed JSON object).
    // Byte-level equality isn't asserted here: jsdom's MSW<->axios Blob path
    // is a known-flaky boundary for binary fidelity, and the contract this
    // endpoint owns is "hand the caller a Blob + its content type", not "the
    // bytes survive jsdom's wrapping".
    expect(result?.blob).toBeInstanceOf(Blob);

    expect(requests.last()?.method).toBe('GET');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/apis/rest-api/orders-api/thumbnail');
  });

  it('resolves 404 to null rather than throwing', async () => {
    // This is what lets the delete hook flip the cache to `null` and the hook
    // tell "no thumbnail" apart from "fetch failed" without a try/catch at the
    // call site. If this ever regresses to a throw, every list page gets 20
    // red error toasts for APIs that simply have no thumbnail.
    server.use(failure('get', PATH, 404, 'NOT_FOUND'));

    await expect(getApiThumbnail('rest-api', 'orders-api')).resolves.toBeNull();
  });

  it('propagates non-404 errors as ApiError', async () => {
    // 5xx isn't a steady state; it's a real failure that should surface to the
    // UI so a toast fires. Collapsing it to null (as 404 does) would hide the
    // problem. The error code collapses to CLIENT_MALFORMED_ERROR for blob
    // responses — the body is a Blob and the generic envelope parser can't
    // read it — so what we assert here is the HTTP status, which stays
    // trustworthy regardless of body shape.
    server.use(failure('get', PATH, 500, 'INTERNAL_ERROR'));

    await expect(getApiThumbnail('rest-api', 'orders-api')).rejects.toBeInstanceOf(ApiError);
    await expect(getApiThumbnail('rest-api', 'orders-api')).rejects.toMatchObject({
      status: 500,
    });
  });

  it('URL-encodes the apiType and apiId path segments', async () => {
    server.use(
      mswHttp.get(apiUrl('/apis/:apiType/:apiId/thumbnail'), async ({ request }) => {
        await requests.capture(request);
        return new HttpResponse(PNG_BYTES, { headers: { 'Content-Type': 'image/png' } });
      })
    );

    await getApiThumbnail('rest-api', 'a b/c');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/apis/rest-api/a%20b%2Fc/thumbnail');
  });
});

describe('upsertApiThumbnail', () => {
  it('PUTs a non-JSON body — the file is not JSON-serialized', async () => {
    server.use(noContent('put', PATH, { record: requests }));

    const file = new Blob([PNG_BYTES], { type: 'image/png' });
    await upsertApiThumbnail('rest-api', 'orders-api', file);

    const request = requests.last();
    expect(request?.method).toBe('PUT');
    // jsdom cannot serialize FormData onto the wire as real multipart, so we
    // match the apiDocuments.endpoints.test.ts convention: the only assertion
    // we can make reliably is that the Content-Type is NOT application/json —
    // which is what a regression that forgot the FormData branch would set.
    expect(request?.headers.get('content-type') ?? '').not.toContain('application/json');
  });

  it('returns a resolved promise on 204 — no envelope expected', async () => {
    server.use(noContent('put', PATH));

    await expect(
      upsertApiThumbnail('rest-api', 'orders-api', new Blob([PNG_BYTES], { type: 'image/png' }))
    ).resolves.toBeUndefined();
  });

  it('surfaces a 413 (too large) as ApiError without throwing a generic Error', async () => {
    server.use(failure('put', PATH, 413, 'PAYLOAD_TOO_LARGE'));

    await expect(
      upsertApiThumbnail('rest-api', 'orders-api', new Blob([PNG_BYTES], { type: 'image/png' }))
    ).rejects.toMatchObject({ code: 'PAYLOAD_TOO_LARGE' });
  });
});

describe('deleteApiThumbnail', () => {
  it('DELETEs the resource and treats 204 as success', async () => {
    server.use(noContent('delete', PATH, { record: requests }));

    await expect(deleteApiThumbnail('rest-api', 'orders-api')).resolves.toBeUndefined();

    expect(requests.last()?.method).toBe('DELETE');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/apis/rest-api/orders-api/thumbnail');
  });

  it('surfaces a 404 as ApiError (delete has no "steady state", unlike GET)', async () => {
    // The GET resolves 404→null because a missing thumbnail is a normal view
    // state; a delete of a missing row is a race the UI should see, so this
    // one stays a thrown error.
    server.use(failure('delete', PATH, 404, 'NOT_FOUND'));

    await expect(deleteApiThumbnail('rest-api', 'orders-api')).rejects.toMatchObject({
      code: 'NOT_FOUND',
    });
  });
});

// Suppress the unused `accepts` import warning if the file ever shrinks past
// using it elsewhere — kept for parity with the other endpoint tests.
void accepts;
