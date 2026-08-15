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

import {
  http as mswHttp,
  HttpResponse,
  type HttpHandler,
  type JsonBodyType,
} from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CSRF_HEADER, CSRF_HEADER_VALUE } from '../../features/auth/authConstants';
import { server } from '../../test/server';
import { ApiError, ClientErrorCode, ErrorCode } from './errors';
import {
  buildQueryString,
  http,
  resetHttpClient,
} from './http';
import { onSessionExpired, resetSessionExpiryNotice } from './sessionEvents';

/**
 * Transport-level behaviour, exercised through MSW rather than by stubbing
 * axios. Everything asserted here is invisible to the type system — headers,
 * status handling, cancellation — which is exactly why it needs tests.
 *
 * The base URL is what `runtimeConfig` resolves to when no platform host is
 * configured (`/api/{version}`), relative to jsdom's own origin.
 */
const BASE = `${window.location.origin}/api/v0.9`;

/** Captures what actually reached the wire for one request. */
type Captured = {
  headers: Headers;
  url: URL;
  body: string;
};

let captured: Captured | undefined;

/** Records the incoming request, then answers with `response`. */
const recording = (
  method: 'get' | 'post' | 'put' | 'delete',
  path: string,
  response: () => Response
): HttpHandler =>
  mswHttp[method](`${BASE}${path}`, async ({ request }) => {
    captured = {
      body: await request.clone().text(),
      headers: request.headers,
      url: new URL(request.url),
    };
    return response();
  });

const ok =
  (body: JsonBodyType = { ok: true }) =>
  () =>
    HttpResponse.json(body);

/**
 * Awaits a request that is expected to fail and hands back the `ApiError` it
 * rejected with.
 *
 * Doing this in one place keeps every failure test to a single assertion about
 * the thing it actually cares about, and quietly enforces the layer's central
 * promise along the way: if the transport ever rejects with something that is
 * not an `ApiError`, every one of these tests fails rather than silently
 * asserting against a raw `AxiosError`.
 */
const rejection = async (promise: Promise<unknown>): Promise<ApiError> => {
  try {
    await promise;
  } catch (error) {
    if (error instanceof ApiError) return error;
    throw new Error(
      `Expected an ApiError but the request rejected with: ${String(error)}`
    );
  }
  throw new Error('Expected the request to fail, but it succeeded.');
};

beforeEach(() => {
  captured = undefined;
  // The client is memoised on first use; drop it so each test builds one from
  // the config in effect right now.
  resetHttpClient();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('request context headers', () => {
  it('sends X-Org-Id when the caller supplies an organization', async () => {
    // Regression guard: org scope is plumbed through the axios config, and a
    // break here means every request silently leaves the org unscoped while
    // still compiling and passing every other test.
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis', { orgId: 'acme-org' });

    expect(captured?.headers.get('X-Org-Id')).toBe('acme-org');
  });

  it('omits X-Org-Id for genuinely global endpoints', async () => {
    server.use(recording('get', '/organizations', ok()));

    await http.get('/organizations');

    expect(captured?.headers.get('X-Org-Id')).toBeNull();
  });

  it('asks for JSON', async () => {
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis');

    expect(captured?.headers.get('Accept')).toBe('application/json');
  });

  it('attaches a correlation id so a browser report can be tied to a server log', async () => {
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis');

    expect(captured?.headers.get('X-Request-Id')).toBeTruthy();
  });

  it('gives each request its own correlation id', async () => {
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis');
    const first = captured?.headers.get('X-Request-Id');
    await http.get('/rest-apis');
    const second = captured?.headers.get('X-Request-Id');

    expect(first).not.toBe(second);
  });

  it('puts the correlation id on the error, matching the header', async () => {
    server.use(recording('get', '/rest-apis/pizza-shack', () =>
        HttpResponse.json(
            { status: 'error', code: 'INTERNAL_ERROR', message: 'x' }, {status: 500})
    ));
    const error = await rejection(http.get('/rest-apis/pizza-shack'));
    expect(error.requestId).toBeTruthy();
});
});

describe('CSRF protection', () => {
  it.each(['post', 'put', 'delete'] as const)(
    'sends the CSRF header on %s, which the BFF requires to accept the request',
    async (method) => {
      server.use(recording(method, '/rest-apis', ok()));

      await http[method]('/rest-apis', method === 'delete' ? undefined : {});

      expect(captured?.headers.get(CSRF_HEADER)).toBe(CSRF_HEADER_VALUE);
    }
  );

  it('omits it on GET, which the BFF exempts as non-mutating', async () => {
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis');

    expect(captured?.headers.get(CSRF_HEADER)).toBeNull();
  });
});

describe('request bodies', () => {
  it('serialises a JSON body and labels it as JSON', async () => {
    // axios does not infer a content type for an already-stringified body, so
    // without an explicit header the server sees an unlabelled payload.
    server.use(recording('post', '/rest-apis', ok()));

    await http.post('/rest-apis', { displayName: 'Pizza Shack' });

    expect(captured?.headers.get('Content-Type')).toContain('application/json');
    expect(JSON.parse(captured!.body)).toEqual({ displayName: 'Pizza Shack' });
  });

  it('does not label a file upload as JSON, leaving the browser to set multipart itself', async () => {
    // multipart needs a generated boundary, so the transport must stay out of
    // the way rather than forcing a content type.
    //
    // Only the *absence* of application/json is asserted. jsdom's XHR does not
    // implement FormData bodies — verified by sending one through plain axios
    // with none of this module involved, which coerces it to "[object FormData]"
    // exactly the same way. The real boundary behaviour therefore cannot be
    // observed here and belongs in a browser-run test.
    server.use(recording('post', '/secrets', ok()));
    const form = new FormData();
    form.append('name', 'signing-key');

    await http.post('/secrets', form);

    expect(captured?.headers.get('Content-Type')).not.toContain('application/json');
  });

  it('sends no body at all when none is given', async () => {
    server.use(recording('delete', '/rest-apis/pizza-shack', () =>
      new HttpResponse(null, { status: 204 })
    ));

    await http.delete('/rest-apis/pizza-shack');

    expect(captured?.body).toBe('');
  });
});

describe('query strings', () => {
  it('appends the caller’s parameters to the URL', async () => {
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis', { query: { projectId: 'default', limit: 20 } });

    expect(captured?.url.searchParams.get('projectId')).toBe('default');
    expect(captured?.url.searchParams.get('limit')).toBe('20');
  });

  it('omits parameters that were left undefined', async () => {
    // `?limit=undefined` is a distinct URL from `?`, so it would also be a
    // distinct cache entry for identical data.
    server.use(recording('get', '/rest-apis', ok()));

    await http.get('/rest-apis', { query: { projectId: 'default', limit: undefined } });

    expect(captured?.url.search).toBe('?projectId=default');
  });

  describe('buildQueryString', () => {
    it('returns an empty string when there is nothing to send', () => {
      expect(buildQueryString()).toBe('');
      expect(buildQueryString({})).toBe('');
      expect(buildQueryString({ limit: undefined, query: null })).toBe('');
    });

    it('repeats a key for each value in an array', () => {
      expect(buildQueryString({ status: ['ACTIVE', 'DRAFT'] })).toBe(
        '?status=ACTIVE&status=DRAFT'
      );
    });

    it('keeps zero and false, which are meaningful values', () => {
      expect(buildQueryString({ offset: 0, latest: false })).toBe(
        '?offset=0&latest=false'
      );
    });

    it('percent-encodes values so a search term cannot alter the URL', () => {
      expect(buildQueryString({ query: 'a&b=c' })).toBe('?query=a%26b%3Dc');
    });
  });
});

describe('successful responses', () => {
  it('resolves to the parsed body, with no envelope for the caller to unwrap', async () => {
    server.use(recording('get', '/rest-apis/pizza-shack', ok({ id: 'pizza-shack' })));

    await expect(http.get('/rest-apis/pizza-shack')).resolves.toEqual({
      id: 'pizza-shack',
    });
  });

  it('resolves a 204 to undefined rather than failing to parse an empty body', async () => {
    // The spec uses 204 for deletes and undeploys; JSON.parse('') would throw.
    server.use(recording('delete', '/rest-apis/pizza-shack', () =>
      new HttpResponse(null, { status: 204 })
    ));

    await expect(http.delete('/rest-apis/pizza-shack')).resolves.toBeUndefined();
  });
});

describe('failure responses', () => {
  const failWith = (status: number, body: JsonBodyType) =>
    server.use(
      mswHttp.get(`${BASE}/rest-apis/pizza-shack`, () =>
        HttpResponse.json(body, { status })
      )
    );

  it('rejects with an ApiError, never a raw AxiosError', async () => {
    failWith(404, { status: 'error', code: 'REST_API_NOT_FOUND', message: 'Gone.' });

    await expect(http.get('/rest-apis/pizza-shack')).rejects.toBeInstanceOf(ApiError);
  });

  it.each([400, 401, 403, 404, 409, 500, 503])(
    'treats %i as a failure carrying its status, not as a network problem',
    async (status) => {
      // Guards the `validateStatus` wiring: if axios were left to reject on
      // 4xx/5xx itself, every one of these would surface as a network failure
      // with no code and no field errors.
      failWith(status, {
        status: 'error',
        code: 'INTERNAL_ERROR',
        message: 'Something went wrong.',
      });

      const error = await rejection(http.get('/rest-apis/pizza-shack'));

      expect(error.kind).toBe('http');
      expect(error.status).toBe(status);
    }
  );

  it('keeps the stable error code the UI branches on', async () => {
    failWith(404, { status: 'error', code: 'REST_API_NOT_FOUND', message: 'Gone.' });

    const error = await rejection(http.get('/rest-apis/pizza-shack'));

    expect(error.code).toBe(ErrorCode.REST_API_NOT_FOUND);
  });

  it('keeps per-field validation errors so a form can display them', async () => {
    failWith(400, {
      status: 'error',
      code: 'VALIDATION_FAILED',
      message: 'Invalid.',
      errors: [{ field: 'version', message: 'must match semver' }],
    });

    const error = await rejection(http.get('/rest-apis/pizza-shack'));

    expect(error.fieldErrorMap()).toEqual({ version: 'must match semver' });
  });

  it('keeps the server tracking id from a 5xx for support correlation', async () => {
    failWith(500, {
      status: 'error',
      code: 'INTERNAL_ERROR',
      message: 'Something went wrong.',
      trackingId: 'trk-42',
    });

    const error = await rejection(http.get('/rest-apis/pizza-shack'));

    expect(error.trackingId).toBe('trk-42');
  });

  it('does not leak a non-JSON error page into the message shown to the user', async () => {
    // An intermediary can answer with HTML; that markup must never reach the UI.
    server.use(
      mswHttp.get(`${BASE}/rest-apis/pizza-shack`, () =>
        new HttpResponse('<html><body>502 Bad Gateway</body></html>', {
          status: 502,
          headers: { 'Content-Type': 'text/html' },
        })
      )
    );

    const error = await rejection(http.get('/rest-apis/pizza-shack'));

    expect(error.message).not.toContain('<html>');
    expect(error.code).toBe(ClientErrorCode.CLIENT_MALFORMED_ERROR);
    expect(error.status).toBe(502);
  });

  it('reports a request that never reached the server as a network failure', async () => {
    server.use(mswHttp.get(`${BASE}/rest-apis`, () => HttpResponse.error()));

    const error = await rejection(http.get('/rest-apis'));

    expect(error.kind).toBe('network');
    expect(error.code).toBe(ClientErrorCode.CLIENT_NETWORK_ERROR);
  });
});

describe('cancellation and deadlines', () => {
  it('aborts in flight when the caller cancels, so a superseded query cannot win the race', async () => {
    // React Query passes its own signal into every queryFn; typing in a filter
    // must not let an older response land after a newer one.
    server.use(
      mswHttp.get(`${BASE}/rest-apis`, async () => {
        await new Promise((resolve) => setTimeout(resolve, 1_000));
        return HttpResponse.json({ ok: true });
      })
    );
    const controller = new AbortController();
    const pending = http.get('/rest-apis', { signal: controller.signal });

    controller.abort();
    const error = await rejection(pending);

    expect(error.kind).toBe('aborted');
    expect(error.code).toBe(ClientErrorCode.CLIENT_REQUEST_ABORTED);
  });

  it('never starts a request whose signal was already aborted', async () => {
    let reached = false;
    server.use(
      mswHttp.get(`${BASE}/rest-apis`, () => {
        reached = true;
        return HttpResponse.json({ ok: true });
      })
    );

    await http
      .get('/rest-apis', { signal: AbortSignal.abort() })
      .catch(() => undefined);

    expect(reached).toBe(false);
  });

  it('reports an elapsed deadline as a timeout, distinct from a cancellation', async () => {
    // The two are told apart deliberately: a timeout is a failure worth
    // retrying and surfacing, while a cancellation is expected and silent.
    server.use(
      mswHttp.get(`${BASE}/rest-apis`, async () => {
        await new Promise((resolve) => setTimeout(resolve, 500));
        return HttpResponse.json({ ok: true });
      })
    );

    const error = await rejection(http.get('/rest-apis', { timeout: 20 }));

    expect(error.kind).toBe('timeout');
    expect(error.code).toBe(ClientErrorCode.CLIENT_TIMEOUT);
  });

  it('detaches its deadline listener from a signal that outlives the request', async () => {
    // React Query reuses one signal per query observer, so a listener left
    // attached after each request is a real leak, not a theoretical one.
    server.use(recording('get', '/rest-apis', ok()));
    const controller = new AbortController();
    const removeSpy = vi.spyOn(controller.signal, 'removeEventListener');

    await http.get('/rest-apis', { signal: controller.signal });

    expect(removeSpy).toHaveBeenCalledWith('abort', expect.any(Function));
  });
});

describe('session expiry', () => {
  // The notification is debounced across a few seconds of real time, so without
  // clearing that window each test would be suppressed by whichever earlier
  // test happened to trigger a 401 first.
  beforeEach(resetSessionExpiryNotice);

  const failWith401 = () =>
    server.use(
      mswHttp.get(`${BASE}/rest-apis`, () =>
        HttpResponse.json(
          { status: 'error', code: 'UNAUTHORIZED', message: 'Session expired.' },
          { status: 401 }
        )
      )
    );

  it('notifies subscribers when the BFF reports the session is gone', async () => {
    failWith401();
    const notified = vi.fn();
    const unsubscribe = onSessionExpired(notified);

    await http.get('/rest-apis').catch(() => undefined);

    expect(notified).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  it('notifies once for a burst of parallel 401s, so a dashboard triggers one redirect', async () => {
    // Eight widgets loading against a dead session must not queue eight
    // redirects to the login page.
    failWith401();
    const notified = vi.fn();
    const unsubscribe = onSessionExpired(notified);

    await Promise.all(
      Array.from({ length: 8 }, () => http.get('/rest-apis').catch(() => undefined))
    );

    expect(notified).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  it('stops notifying a subscriber that has unsubscribed', async () => {
    failWith401();
    const notified = vi.fn();
    onSessionExpired(notified)();

    await http.get('/rest-apis').catch(() => undefined);

    expect(notified).not.toHaveBeenCalled();
  });
});
