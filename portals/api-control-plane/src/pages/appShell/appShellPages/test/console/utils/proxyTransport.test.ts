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

import { HttpResponse, http } from 'msw';
import { createIntl, createIntlCache } from 'react-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '@/test/server';
import {
  createRelayFetch,
  encodeRequestBody,
  testConsoleRelayPlugin,
  toResponseLike,
  type RelayContext,
} from './proxyTransport';

const INVOKE_URL = 'http://localhost:3000/api/test-console/invoke';

const intl = createIntl({ locale: 'en', messages: {} }, createIntlCache());

const contextRef = (overrides: Partial<RelayContext> = {}) => ({
  current: {
    baseUrl: 'https://gw.example.com/pizza/v1',
    gatewayId: 'gw-prod',
    intl,
    orgHandle: 'acme',
    restApiId: 'api-1',
    ...overrides,
  } as RelayContext,
});

/** A relay reply carrying whatever the gateway answered. */
const relayed = (overrides: Record<string, unknown> = {}) => ({
  outcome: 'response',
  response: {
    status: 200,
    statusText: 'OK',
    headers: [{ name: 'Content-Type', value: 'application/json' }],
    body: '{"ok":true}',
    bodyEncoding: 'utf8',
    truncated: false,
    durationMs: 12,
    ...overrides,
  },
});

/** Blob with a declared size and observable reads. */
const stubbedBlob = (size: number, contents = '') => {
  const blob = new Blob([contents]);
  const read = vi.fn(async () => new TextEncoder().encode(contents).buffer);
  Object.defineProperty(blob, 'size', { value: size });
  Object.defineProperty(blob, 'arrayBuffer', { value: read });
  return { blob, read };
};

/** Captures the envelope the relay posted. */
let sent: Record<string, unknown> | undefined;

const respondWith = (payload: object, status = 200) => {
  server.use(
    http.post(INVOKE_URL, async ({ request }) => {
      sent = (await request.json()) as Record<string, unknown>;
      return HttpResponse.json(payload, { status });
    }),
  );
};

beforeEach(() => {
  sent = undefined;
});

describe('createRelayFetch — what reaches the BFF', () => {
  it('sends the API and gateway identifiers rather than a URL', async () => {
    // The browser naming a URL would make the BFF an open proxy; it names the
    // API and gateway instead, and the BFF resolves the address itself.
    respondWith(relayed());

    await createRelayFetch(contextRef())('ignored', {
      url: 'https://gw.example.com/pizza/v1/order?size=L',
      method: 'GET',
      headers: { 'X-API-Key': 'test-key' },
    });

    expect(sent).toMatchObject({
      orgHandle: 'acme',
      restApiId: 'api-1',
      gatewayId: 'gw-prod',
      method: 'GET',
      path: '/order',
      query: [{ name: 'size', value: 'L' }],
      headers: [{ name: 'X-API-Key', value: 'test-key' }],
    });
    expect(sent).not.toHaveProperty('url');
    expect(sent).not.toHaveProperty('baseUrl');
  });

  it('drops header entries swagger left unset', async () => {
    // Swagger stores absent headers as null rather than removing the key; an
    // unfiltered pass would send `Accept: undefined`.
    respondWith(relayed());

    await createRelayFetch(contextRef())('ignored', {
      url: 'https://gw.example.com/pizza/v1/order',
      method: 'GET',
      headers: { Accept: null, 'X-Real': 'kept' },
    });

    expect(sent?.headers).toEqual([{ name: 'X-Real', value: 'kept' }]);
  });

  it('replaces any Content-Type when it supplies a multipart boundary', async () => {
    // Two Content-Type headers would reach the gateway if this appended, and
    // the gateway would then be unable to split the body it was handed.
    respondWith(relayed());
    const form = new FormData();
    form.append('field', 'value');

    await createRelayFetch(contextRef())('ignored', {
      url: 'https://gw.example.com/pizza/v1/order',
      method: 'POST',
      headers: { 'Content-Type': 'multipart/form-data' },
      body: form,
    });

    const contentTypes = (sent?.headers as Array<{ name: string; value: string }>).filter(
      (header) => header.name.toLowerCase() === 'content-type',
    );
    expect(contentTypes).toHaveLength(1);
    expect(contentTypes[0].value).toMatch(/^multipart\/form-data; boundary=/);
  });

  it('refuses a request aimed at another origin', async () => {
    // Not this API's request. Relaying it would point the BFF somewhere the
    // console never offered.
    respondWith(relayed());

    await expect(
      createRelayFetch(contextRef())('ignored', {
        url: 'https://evil.example.com/pizza/v1/order',
        method: 'GET',
      }),
    ).rejects.toThrow(/cannot be sent as written/i);
    expect(sent).toBeUndefined();
  });

  it('refuses to send before a gateway is selected', async () => {
    respondWith(relayed());

    await expect(
      createRelayFetch(contextRef({ gatewayId: '' }))('ignored', {
        url: 'https://gw.example.com/pizza/v1/order',
        method: 'GET',
      }),
    ).rejects.toThrow(/select a deployed gateway/i);
    expect(sent).toBeUndefined();
  });

  it('reads the target live, so a gateway switch after mount is honoured', async () => {
    // swagger-ui keeps only the first `plugins` value it is given, so a
    // captured id would stay pinned to whatever was selected at mount.
    const ref = contextRef();
    respondWith(relayed());

    ref.current = { ...ref.current, gatewayId: 'gw-staging' };
    await createRelayFetch(ref)('ignored', {
      url: 'https://gw.example.com/pizza/v1/order',
      method: 'GET',
    });

    expect(sent?.gatewayId).toBe('gw-staging');
  });
});

describe('createRelayFetch — what comes back', () => {
  it('reports the real gateway URL, so the console shows the call the user made', async () => {
    respondWith(relayed());

    const res = await createRelayFetch(contextRef())('ignored', {
      url: 'https://gw.example.com/pizza/v1/order?size=L',
      method: 'GET',
    });

    // Not the relay endpoint: this is what swagger renders as the Request URL.
    expect(res.url).toBe('https://gw.example.com/pizza/v1/order?size=L');
  });

  it('passes a gateway error status through as a response, not a thrown relay failure', async () => {
    respondWith(relayed({ status: 502, statusText: 'Bad Gateway', body: 'backend down' }));

    const res = await createRelayFetch(contextRef())('ignored', {
      url: 'https://gw.example.com/pizza/v1/order',
      method: 'GET',
    });

    // `ok: false` is what makes swagger-client raise it the same way it would
    // for a direct call, so the console's error rendering is unchanged.
    expect(res.status).toBe(502);
    expect(res.ok).toBe(false);
    expect(await res.text()).toBe('backend down');
  });

  it('translates each relay failure into its own wording', async () => {
    for (const [code, expected] of [
      ['TARGET_NOT_ALLOWED', /cannot be tested on the selected gateway/i],
      ['UPSTREAM_TIMEOUT', /did not respond in time/i],
      ['UPSTREAM_UNREACHABLE', /could not reach this gateway/i],
      ['RELAY_BUSY', /too many test requests/i],
      ['SESSION_EXPIRED', /session has expired/i],
    ] as const) {
      respondWith({ status: 'error', code, message: 'ignored' }, 400);

      await expect(
        createRelayFetch(contextRef())('ignored', {
          url: 'https://gw.example.com/pizza/v1/order',
          method: 'GET',
        }),
      ).rejects.toThrow(expected);
    }
  });

  it('does not relabel an abort as a failure', async () => {
    // An abort is the user navigating away; swagger has its own handling.
    const controller = new AbortController();
    server.use(http.post(INVOKE_URL, () => HttpResponse.json(relayed())));
    controller.abort();

    await expect(
      createRelayFetch(contextRef())('ignored', {
        url: 'https://gw.example.com/pizza/v1/order',
        method: 'GET',
        signal: controller.signal,
      }),
    ).rejects.toThrow(/abort/i);
  });
});

describe('createRelayFetch — oversized bodies', () => {
  it('reports an oversized attachment as a size error, not a relay failure', async () => {
    respondWith(relayed());
    const { blob } = stubbedBlob(64 * 1024 * 1024);

    await expect(
      createRelayFetch(contextRef())('ignored', {
        url: 'https://gw.example.com/pizza/v1/order',
        method: 'POST',
        body: blob,
      }),
    ).rejects.toThrow(/too large to send from the console/i);
    expect(sent).toBeUndefined();
  });
});

describe('toResponseLike', () => {
  it('exposes every response header, including repeats', () => {
    // A browser could not read either of these cross-origin without an explicit
    // Access-Control-Expose-Headers, so relaying shows more than a direct call.
    const res = toResponseLike(
      relayed({
        headers: [
          { name: 'Set-Cookie', value: 'a=1' },
          { name: 'Set-Cookie', value: 'b=2' },
          { name: 'X-Ratelimit-Remaining', value: '9' },
        ],
      }) as never,
      'https://gw.example.com/pizza/v1/order',
    );

    expect(res.headers.get('x-ratelimit-remaining')).toBe('9');
    expect(res.headers.get('set-cookie')).toContain('a=1');
    expect(res.headers.get('set-cookie')).toContain('b=2');
  });

  it('survives a header name the Headers API refuses', () => {
    // Dropping one malformed header beats failing the whole response.
    const res = toResponseLike(
      relayed({
        headers: [
          { name: 'Bad Header', value: 'x' },
          { name: 'X-Good', value: 'y' },
        ],
      }) as never,
      'https://gw.example.com/',
    );

    expect(res.headers.get('x-good')).toBe('y');
  });

  it('decodes a base64 body back to the bytes the gateway sent', async () => {
    const res = toResponseLike(
      relayed({ body: btoa('hello'), bodyEncoding: 'base64' }) as never,
      'https://gw.example.com/',
    );

    expect(await res.text()).toBe('hello');
    expect((await res.blob()).size).toBe(5);
  });

  it('carries a 204 through, which a constructed Response would have rejected', () => {
    const res = toResponseLike(
      relayed({ status: 204, statusText: 'No Content', body: '' }) as never,
      'https://gw.example.com/',
    );

    expect(res.status).toBe(204);
    expect(res.ok).toBe(true);
  });
});

describe('encodeRequestBody', () => {
  it('passes a string body through unchanged', async () => {
    expect(await encodeRequestBody('{"a":1}')).toEqual({ body: '{"a":1}', bodyEncoding: 'utf8' });
  });

  it('treats an absent body as empty', async () => {
    expect(await encodeRequestBody(undefined)).toEqual({ body: '', bodyEncoding: 'utf8' });
  });

  it('serialises url-encoded form fields', async () => {
    const params = new URLSearchParams({ size: 'L', topping: 'olives' });
    expect(await encodeRequestBody(params)).toEqual({
      body: 'size=L&topping=olives',
      bodyEncoding: 'utf8',
    });
  });

  it('produces a multipart boundary for FormData, since the browser no longer can', async () => {
    // Swagger strips its own multipart Content-Type so the browser can add the
    // boundary. The browser is not the sender any more, so it has to come from
    // here or the gateway cannot split the body it is handed.
    const form = new FormData();
    form.append('field', 'value');

    const encoded = await encodeRequestBody(form);

    expect(encoded.contentType).toMatch(/^multipart\/form-data; boundary=/);
    expect(encoded.body).toContain('field');
  });

  it('refuses an oversized blob without reading it', async () => {
    // The point of the guard: `size` is metadata, so an attachment too large to
    // send is refused before a single byte is pulled into memory.
    const { blob, read } = stubbedBlob(64 * 1024 * 1024);

    await expect(encodeRequestBody(blob)).rejects.toMatchObject({ code: 'REQUEST_TOO_LARGE' });
    expect(read).not.toHaveBeenCalled();
  });

  it('refuses an oversized multipart form before reading any part', async () => {
    // Uses entry metadata, so refusal occurs before reading any part. A text
    // part avoids jsdom's File lacking arrayBuffer(); the size pre-pass is shared.
    const form = new FormData();
    form.append('notes', 'x'.repeat(17 * 1024 * 1024));

    await expect(encodeRequestBody(form)).rejects.toMatchObject({ code: 'REQUEST_TOO_LARGE' });
  });

  it('base64-encodes bytes that are not valid UTF-8', async () => {
    const encoded = await encodeRequestBody(new Uint8Array([0x89, 0x50, 0xff, 0xfe]));
    expect(encoded.bodyEncoding).toBe('base64');
  });
});

describe('testConsoleRelayPlugin', () => {
  it('adds userFetch to the execute payload without disturbing it', () => {
    // userFetch is swagger-client's own per-request transport hook. Injecting
    // it leaves the request swagger built — and displays — untouched.
    const original = vi.fn();
    const plugin = testConsoleRelayPlugin(contextRef())();
    const wrapped = plugin.statePlugins.spec.wrapActions.executeRequest(original);

    wrapped({ pathName: '/order', method: 'get' });

    expect(original).toHaveBeenCalledWith(
      expect.objectContaining({
        pathName: '/order',
        method: 'get',
        userFetch: expect.any(Function),
      }),
    );
  });
});
