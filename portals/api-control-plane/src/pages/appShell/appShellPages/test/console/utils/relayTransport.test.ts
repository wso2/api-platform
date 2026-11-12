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
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, type ApiErrorKind } from '@/api/core/errors';
import type { InvokeRestApiOnGatewayVariables } from '@/api/resources/restApis/apiGateways/apiGateways.hooks';
import type { ConsoleSendRequest } from '@/components/ApiTestConsole';
import { createRelayTransport, encodeRequestBody, type RelayContext } from './relayTransport';

const intl = createIntl({ locale: 'en', messages: {} }, createIntlCache());

/** What the transport asked to invoke, and how the invoke answers. */
let sent: InvokeRestApiOnGatewayVariables | undefined;
let answer: () => Promise<unknown>;

/** Stands in for `useInvokeRestApiOnGateway`'s `mutateAsync`. */
const invoke = async (variables: InvokeRestApiOnGatewayVariables) => {
  sent = variables;
  if (variables.signal?.aborted) throw new ApiError('Request was canceled', { kind: 'aborted' });
  return answer();
};

const transportOf = (overrides: Partial<RelayContext> = {}) =>
  createRelayTransport({
    gatewayId: 'gw-prod',
    intl,
    invoke,
    restApiId: 'api-1',
    ...overrides,
  } as RelayContext);

/** A request the console would hand over, under the gateway `https://gw.example.com/pizza/v1`. */
const request = (overrides: Partial<ConsoleSendRequest> = {}): ConsoleSendRequest => ({
  baseUrl: 'https://gw.example.com/pizza/v1',
  headers: [],
  method: 'GET',
  path: '/order',
  query: [],
  ...overrides,
});

const live = new AbortController().signal;

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

const respondWith = (payload: object) => {
  answer = async () => payload;
};

/** The invoke request was not relayed: an error response, or no response at all. */
const failWith = (kind: ApiErrorKind, code?: string) => {
  answer = async () => {
    throw new ApiError('ignored', { kind, code, status: kind === 'http' ? 400 : undefined });
  };
};

beforeEach(() => {
  sent = undefined;
  respondWith(relayed());
});

describe('createRelayTransport — what reaches the Platform API', () => {
  it('names the API and gateway rather than a URL', async () => {
    // A browser-supplied URL would make the Platform API an open proxy.
    await transportOf()(
      request({ headers: [['X-API-Key', 'test-key']], query: [['size', 'L']] }),
      live,
    );

    expect(sent).toEqual({
      body: {
        restApiId: 'api-1',
        gatewayId: 'gw-prod',
        method: 'GET',
        path: '/order',
        query: [{ name: 'size', value: 'L' }],
        headers: [{ name: 'X-API-Key', value: 'test-key' }],
        body: '',
        bodyEncoding: 'utf8',
      },
      signal: live,
    });
  });

  it('sends a text body as is', async () => {
    await transportOf()(
      request({
        body: { kind: 'text', mediaType: 'application/json', text: '{"a":1}' },
        headers: [['Content-Type', 'application/json']],
        method: 'POST',
      }),
      live,
    );

    expect(sent?.body.body).toBe('{"a":1}');
    expect(sent?.body.bodyEncoding).toBe('utf8');
  });

  it('replaces any Content-Type when it supplies a multipart boundary', async () => {
    // Two Content-Type headers would reach the gateway if this appended, and
    // the gateway would then be unable to split the body it was handed.
    await transportOf()(
      request({
        body: { kind: 'multipart', parts: [{ name: 'field', value: 'value' }] },
        headers: [['Content-Type', 'multipart/form-data']],
        method: 'POST',
      }),
      live,
    );

    const contentTypes = (sent?.body.headers ?? []).filter(
      (header) => header.name.toLowerCase() === 'content-type',
    );
    expect(contentTypes).toHaveLength(1);
    expect(contentTypes[0].value).toMatch(/^multipart\/form-data; boundary=/);
  });

  it('refuses a path that would leave the gateway, and sends nothing', async () => {
    for (const path of ['//evil.example.com/x', '/a/../../admin', '/a\\b', 'order']) {
      await expect(transportOf()(request({ path }), live)).rejects.toThrow(
        /cannot be sent as written/i,
      );
    }
    expect(sent).toBeUndefined();
  });

  it('refuses a header that would let a caller steer the request', async () => {
    for (const header of [
      ['Host', 'evil.example.com'],
      ['X-Forwarded-Host', 'evil.example.com'],
      ['X-Test', 'a\r\nInjected: yes'],
    ] as [string, string][]) {
      await expect(transportOf()(request({ headers: [header] }), live)).rejects.toThrow(
        /cannot be sent as written/i,
      );
    }
    expect(sent).toBeUndefined();
  });

  it('refuses to send before a gateway is selected', async () => {
    await expect(transportOf({ gatewayId: '' })(request(), live)).rejects.toThrow(
      /select a deployed gateway/i,
    );
    expect(sent).toBeUndefined();
  });
});

describe('createRelayTransport — what comes back', () => {
  it('reports the call the user made, without the query string', async () => {
    const result = await transportOf()(request({ query: [['api-key', 'secret']] }), live);

    // The query can carry a credential, so it is not echoed back into the result.
    expect(result.url).toBe('https://gw.example.com/pizza/v1/order');
  });

  it('passes a gateway error status through as a response, not a thrown relay failure', async () => {
    respondWith(relayed({ status: 502, statusText: 'Bad Gateway', body: 'backend down' }));

    const result = await transportOf()(request(), live);

    expect(result.status).toBe(502);
    expect(result.bodyText).toBe('backend down');
  });

  it('keeps every response header, including repeats', async () => {
    respondWith(
      relayed({
        headers: [
          { name: 'Set-Cookie', value: 'a=1' },
          { name: 'Set-Cookie', value: 'b=2' },
          { name: 'Content-Type', value: 'application/json' },
        ],
      }),
    );

    const result = await transportOf()(request(), live);

    expect(result.headers.filter(([name]) => name === 'Set-Cookie')).toHaveLength(2);
  });

  it('decodes a base64 body back to text', async () => {
    respondWith(
      relayed({
        body: btoa('hello'),
        bodyEncoding: 'base64',
        headers: [{ name: 'Content-Type', value: 'text/plain' }],
      }),
    );

    expect((await transportOf()(request(), live)).bodyText).toBe('hello');
  });

  it('does not show a binary body as text', async () => {
    respondWith(
      relayed({
        body: btoa(String.fromCharCode(0x89, 0x50, 0x4e, 0x47)),
        bodyEncoding: 'base64',
        headers: [{ name: 'Content-Type', value: 'image/png' }],
      }),
    );

    const result = await transportOf()(request(), live);

    expect(result.binary).toBe(true);
    expect(result.bodyText).toBe('');
  });

  it('carries a 204 through', async () => {
    respondWith(relayed({ status: 204, statusText: 'No Content', body: '' }));

    expect((await transportOf()(request(), live)).status).toBe(204);
  });

  it('translates each relay failure into its own wording', async () => {
    for (const [code, expected] of [
      ['REST_API_INVOKE_TARGET_NOT_ALLOWED', /not configured to reach this gateway/i],
      ['REST_API_INVOKE_GATEWAY_NOT_DEPLOYED', /not deployed to the selected gateway/i],
      ['REST_API_NOT_FOUND', /API could not be found/i],
      ['REST_API_INVOKE_GATEWAY_NO_ENDPOINT', /no endpoint configured/i],
      ['REST_API_INVOKE_GATEWAY_UNSUPPORTED_SCHEME', /not an HTTP address/i],
      ['REST_API_INVOKE_UPSTREAM_TIMEOUT', /did not respond in time/i],
      ['REST_API_INVOKE_UPSTREAM_UNREACHABLE', /could not reach this gateway/i],
      ['REST_API_INVOKE_BUSY', /too many test requests/i],
      ['REST_API_INVOKE_REQUEST_INVALID', /cannot be sent as written/i],
      ['PAYLOAD_TOO_LARGE', /too large to send/i],
      ['UNAUTHORIZED', /session has expired/i],
      ['FORBIDDEN', /could not be sent/i],
    ] as const) {
      failWith('http', code);

      await expect(transportOf()(request(), live)).rejects.toThrow(expected);
    }
  });

  it('never shows the server’s own message text', async () => {
    answer = async () => {
      throw new ApiError('internal: db password=hunter2', {
        kind: 'http',
        code: 'WHATEVER',
        status: 500,
      });
    };

    await expect(transportOf()(request(), live)).rejects.toThrow(/could not be sent/i);
    await expect(transportOf()(request(), live)).rejects.not.toThrow(/hunter2/);
  });

  it('reports a Platform API it could not reach as an unreachable gateway', async () => {
    failWith('network');

    await expect(transportOf()(request(), live)).rejects.toThrow(/could not reach this gateway/i);
  });

  it('does not relabel an abort as a failure', async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(transportOf()(request(), controller.signal)).rejects.toMatchObject({
      aborted: true,
    });
  });

  it('reports a reply that is not a relay envelope as a usable message', async () => {
    respondWith({ outcome: 'response', response: { status: 200, headers: 'nope' } });

    await expect(transportOf()(request(), live)).rejects.toThrow(/could not be sent/i);
  });
});

describe('createRelayTransport — oversized bodies', () => {
  it('reports an oversized attachment as a size error, not a relay failure', async () => {
    const { blob } = stubbedBlob(64 * 1024 * 1024);

    await expect(
      transportOf()(
        request({
          body: {
            file: blob as unknown as File,
            kind: 'binary',
            mediaType: 'application/octet-stream',
          },
          method: 'POST',
        }),
        live,
      ),
    ).rejects.toThrow(/too large to send from the console/i);
    expect(sent).toBeUndefined();
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

describe('relay hardening', () => {
  it('keeps a leading byte-order mark in an uploaded file', async () => {
    const bytes = new Uint8Array([0xef, 0xbb, 0xbf, 0x68, 0x69]);
    const file = new File([bytes], 'bom.txt');
    // jsdom's File has no arrayBuffer(); give it the bytes it would read.
    Object.defineProperty(file, 'arrayBuffer', { value: async () => bytes.buffer });
    await transportOf()(
      request({ body: { file, kind: 'binary', mediaType: 'text/plain' }, method: 'POST' }),
      live,
    );

    expect(sent?.body.bodyEncoding).toBe('utf8');
    expect(sent?.body.body).toBe('\ufeffhi');
  });

  it('refuses a text body whose UTF-8 size is over the limit, whatever its character count', async () => {
    // 6 M characters of '€' are 18 MB in UTF-8 — over 16 MiB, though under it as a character count.
    const text = '€'.repeat(6_000_000);
    await expect(
      transportOf()(
        request({ body: { kind: 'text', mediaType: 'text/plain', text }, method: 'POST' }),
        live,
      ),
    ).rejects.toThrow();
    expect(sent).toBeUndefined();
  });

  it.each([
    ['a body that is not a string', { body: 42 }],
    ['an unknown body encoding', { bodyEncoding: 'rot13' }],
    ['base64 that is not base64', { body: '***', bodyEncoding: 'base64' }],
    ['a status text that is not a string', { statusText: null }],
  ])('rejects a reply with %s instead of converting it', async (_label, override) => {
    respondWith(relayed(override));

    await expect(transportOf()(request(), live)).rejects.toThrow();
  });

  it('marks the result truncated when the Platform API already clipped the body', async () => {
    respondWith(relayed({ truncated: true }));

    const result = await transportOf()(request(), live);

    expect(result.truncated).toBe(true);
  });
});
