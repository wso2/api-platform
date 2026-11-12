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

import { createIntl, createIntlCache } from 'react-intl';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ConsoleTransportError, type ConsoleSendRequest } from '../types';
import { createDirectTransport, DIRECT_TIMEOUT_MS } from './directTransport';
import { MAX_DISPLAY_BYTES } from './response';

const intl = createIntl({ locale: 'en', messages: {} }, createIntlCache());
const transport = createDirectTransport(intl);
const live = new AbortController().signal;

const request = (overrides: Partial<ConsoleSendRequest> = {}): ConsoleSendRequest => ({
  baseUrl: 'https://gw.example.com/p',
  headers: [],
  method: 'GET',
  path: '/order',
  query: [],
  ...overrides,
});

const fetchMock = vi.fn();

const respond = (body: BodyInit | null, init: ResponseInit & { type?: ResponseType } = {}) => {
  fetchMock.mockResolvedValueOnce(new Response(body, { status: 200, ...init }));
};

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  respond('{"ok":true}', { headers: { 'content-type': 'application/json' } });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('createDirectTransport — what leaves the browser', () => {
  it('sends no cookies, no referrer, follows no redirect, and caches nothing', async () => {
    await transport(request({ query: [['size', 'L']] }), live);

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toBe('https://gw.example.com/p/order?size=L');
    expect(init).toMatchObject({
      cache: 'no-store',
      credentials: 'omit',
      method: 'GET',
      mode: 'cors',
      redirect: 'manual',
      referrerPolicy: 'no-referrer',
    });
  });

  it('refuses a request it cannot prove stays on the gateway, and sends nothing', async () => {
    for (const path of ['//evil.example.com', '/a/../../b', 'order']) {
      await expect(transport(request({ path }), live)).rejects.toThrow(
        /cannot be sent as written/i,
      );
    }
    await expect(transport(request({ baseUrl: 'javascript:alert(1)' }), live)).rejects.toThrow(
      ConsoleTransportError,
    );
    await expect(
      transport(request({ baseUrl: 'https://u:p@gw.example.com' }), live),
    ).rejects.toThrow(ConsoleTransportError);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('refuses a forbidden or malformed header, and sends nothing', async () => {
    for (const header of [
      ['Host', 'x'],
      ['Cookie', 'a=b'],
      ['X-A', 'a\r\nb'],
    ] as [string, string][]) {
      await expect(transport(request({ headers: [header] }), live)).rejects.toThrow(
        /header .* not allowed/i,
      );
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('sends a form body as url-encoded fields', async () => {
    await transport(
      request({ body: { fields: [['a', 'b c']], kind: 'form' }, method: 'POST' }),
      live,
    );

    const body = fetchMock.mock.calls[0][1].body as URLSearchParams;
    expect(body.toString()).toBe('a=b+c');
  });
});

describe('createDirectTransport — what comes back', () => {
  it('reports status, headers, body and size', async () => {
    const result = await transport(request(), live);

    expect(result).toMatchObject({
      binary: false,
      bodyText: '{"ok":true}',
      status: 200,
      truncated: false,
      url: 'https://gw.example.com/p/order',
    });
    expect(result.headers).toContainEqual(['content-type', 'application/json']);
  });

  it('does not follow a redirect, and says so', async () => {
    fetchMock.mockReset();
    respond(null, { headers: { location: 'https://evil.example.com/' }, status: 302 });

    await expect(transport(request(), live)).rejects.toThrow(/does not follow redirects/i);
  });

  it('stops reading at the ceiling and says it was cut', async () => {
    fetchMock.mockReset();
    const chunk = new Uint8Array(1024 * 1024).fill(97);
    const pulled = vi.fn();
    const stream = new ReadableStream<Uint8Array>({
      pull(controller) {
        pulled();
        controller.enqueue(chunk);
      },
    });
    respond(stream, { headers: { 'content-type': 'text/plain' } });

    const result = await transport(request(), live);

    expect(result.truncated).toBe(true);
    expect(result.bodyText.length).toBe(MAX_DISPLAY_BYTES);
    // An endless body is not read to the end.
    expect(pulled.mock.calls.length).toBeLessThan(10);
  });

  it('does not show a binary body as text', async () => {
    fetchMock.mockReset();
    respond(new Uint8Array([0x89, 0x50, 0x4e, 0x47]), { headers: { 'content-type': 'image/png' } });

    const result = await transport(request(), live);

    expect(result.binary).toBe(true);
    expect(result.bodyText).toBe('');
  });

  it('returns a markup body as plain text, untouched', async () => {
    fetchMock.mockReset();
    respond('<img src=x onerror=alert(1)>', { headers: { 'content-type': 'text/html' } });

    expect((await transport(request(), live)).bodyText).toBe('<img src=x onerror=alert(1)>');
  });
});

describe('createDirectTransport — failures', () => {
  it('explains an unreachable or CORS-blocked gateway without exposing the browser’s error', async () => {
    fetchMock.mockReset();
    fetchMock.mockRejectedValueOnce(
      new TypeError('Failed to fetch https://gw.example.com/p/order?token=abc'),
    );

    const failure = await transport(request(), live).then(
      () => undefined,
      (e: ConsoleTransportError) => e,
    );

    expect(failure?.message).toMatch(/could not reach the gateway/i);
    expect(failure?.message).not.toContain('token=abc');
  });

  it('reports a cancel as an abort, not a failure', async () => {
    fetchMock.mockReset();
    fetchMock.mockImplementationOnce(
      (_url: unknown, init: RequestInit) =>
        new Promise((_resolve, reject) => {
          init.signal?.addEventListener('abort', () =>
            reject(new DOMException('aborted', 'AbortError')),
          );
        }),
    );
    const controller = new AbortController();

    const pending = transport(request(), controller.signal);
    controller.abort();

    await expect(pending).rejects.toMatchObject({ aborted: true });
  });

  it('gives up on a gateway that never answers', async () => {
    vi.useFakeTimers();
    fetchMock.mockReset();
    fetchMock.mockImplementationOnce(
      (_url: unknown, init: RequestInit) =>
        new Promise((_resolve, reject) => {
          init.signal?.addEventListener('abort', () =>
            reject(new DOMException('aborted', 'AbortError')),
          );
        }),
    );

    const pending = transport(request(), live);
    const assertion = expect(pending).rejects.toThrow(/did not respond in time/i);
    await vi.advanceTimersByTimeAsync(DIRECT_TIMEOUT_MS + 1);

    await assertion;
  });
});
