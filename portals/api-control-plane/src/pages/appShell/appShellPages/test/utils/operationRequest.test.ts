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

import {
  buildConsoleRequest,
  fillPathParameters,
  operationParameters,
  parameterValue,
} from './operationRequest';
import { buildRequestUrl } from '../curl/utils/toCurl';
import type { ConsoleRequest } from './types';

const BASE = 'https://gw.example.com/default/payments-api/v1.0';

/**
 * `buildConsoleRequest` for an operation the console offers.
 *
 * It declines a verb outside `HTTP_METHODS` by returning `undefined`; every
 * case below passes a supported one, so failing loudly here keeps the
 * assertions free of narrowing that would hide a genuine decline.
 */
const build = (args: Parameters<typeof buildConsoleRequest>[0]): ConsoleRequest => {
  const request = buildConsoleRequest(args);
  if (!request) throw new Error(`expected a request for method ${String(args.method)}`);
  return request;
};

const spec = {
  openapi: '3.0.1',
  paths: {
    '/payments': {
      get: {
        parameters: [
          { name: 'limit', in: 'query' },
          { name: 'status', in: 'query' },
          { name: 'X-Trace', in: 'header' },
        ],
      },
      post: {},
    },
    '/payments/{paymentId}': {
      // Shared across every method on this path — easy to miss, invisible when
      // missed.
      parameters: [{ name: 'paymentId', in: 'path' }],
      get: { parameters: [{ name: 'expand', in: 'query' }] },
      delete: {},
    },
    '/sessions': {
      get: {
        parameters: [
          { name: 'sid', in: 'cookie' },
          { name: 'ok', in: 'query' },
        ],
      },
    },
  },
} as Record<string, unknown>;

describe('operationParameters', () => {
  it('reads the operation’s own parameters', () => {
    expect(operationParameters(spec, '/payments', 'GET')).toEqual([
      { name: 'limit', in: 'query' },
      { name: 'status', in: 'query' },
      { name: 'X-Trace', in: 'header' },
    ]);
  });

  it('includes path-level parameters shared by every method', () => {
    expect(operationParameters(spec, '/payments/{paymentId}', 'DELETE')).toEqual([
      { name: 'paymentId', in: 'path' },
    ]);
  });

  it('merges path-level and operation-level parameters', () => {
    expect(operationParameters(spec, '/payments/{paymentId}', 'GET')).toEqual([
      { name: 'paymentId', in: 'path' },
      { name: 'expand', in: 'query' },
    ]);
  });

  it('lets an operation-level parameter override a path-level one', () => {
    const overriding = {
      paths: {
        '/x': {
          parameters: [{ name: 'a', in: 'query' }],
          get: { parameters: [{ name: 'a', in: 'query' }] },
        },
      },
    };

    // Per OpenAPI's override rule, the operation's wins — and it must appear
    // once, not twice.
    expect(operationParameters(overriding, '/x', 'GET')).toEqual([{ name: 'a', in: 'query' }]);
  });

  it('drops cookie parameters, which the console does not offer', () => {
    expect(operationParameters(spec, '/sessions', 'GET')).toEqual([{ name: 'ok', in: 'query' }]);
  });

  it('accepts a lowercase or uppercase method', () => {
    expect(operationParameters(spec, '/payments', 'get')).toHaveLength(3);
  });

  it('returns nothing for an unknown path or method instead of throwing', () => {
    expect(operationParameters(spec, '/nope', 'GET')).toEqual([]);
    expect(operationParameters(spec, '/payments', 'PUT')).toEqual([]);
    expect(operationParameters({}, '/payments', 'GET')).toEqual([]);
  });

  it('ignores malformed parameter entries', () => {
    const malformed = { paths: { '/x': { get: { parameters: [null, 'nope', { name: 'a' }] } } } };

    expect(operationParameters(malformed, '/x', 'GET')).toEqual([]);
  });
});

describe('parameterValue', () => {
  it('reads swagger’s in.name key form', () => {
    expect(parameterValue({ 'query.limit': '10' }, { name: 'limit', in: 'query' })).toBe('10');
  });

  it('falls back to the bare name, the other identifier form', () => {
    expect(parameterValue({ limit: '10' }, { name: 'limit', in: 'query' })).toBe('10');
  });

  it('stringifies a numeric or boolean value', () => {
    expect(parameterValue({ 'query.limit': 10 }, { name: 'limit', in: 'query' })).toBe('10');
    expect(parameterValue({ 'query.all': false }, { name: 'all', in: 'query' })).toBe('false');
  });

  it('treats blank and absent alike', () => {
    expect(parameterValue({ 'query.limit': '  ' }, { name: 'limit', in: 'query' })).toBeUndefined();
    expect(parameterValue({}, { name: 'limit', in: 'query' })).toBeUndefined();
  });
});

describe('fillPathParameters', () => {
  const parameters = [{ name: 'paymentId', in: 'path' }];

  it('substitutes a filled placeholder', () => {
    expect(
      fillPathParameters('/payments/{paymentId}', parameters, { 'path.paymentId': 'p_1' }),
    ).toBe('/payments/p_1');
  });

  it('leaves an unfilled placeholder visible rather than blanking it', () => {
    // Blanking would change which resource is addressed and hide the omission;
    // the visible `{paymentId}` says exactly what is missing.
    expect(fillPathParameters('/payments/{paymentId}', parameters, {})).toBe(
      '/payments/{paymentId}',
    );
  });

  it('replaces every occurrence of the same placeholder', () => {
    expect(
      fillPathParameters('/a/{id}/b/{id}', [{ name: 'id', in: 'path' }], { 'path.id': 'x' }),
    ).toBe('/a/x/b/x');
  });

  // A path parameter is one segment's worth of value, and `allowReserved` is a
  // query-parameter option only. swagger-client escapes these the same way when
  // it executes the request, so leaving them raw would make the generated
  // command disagree with what was actually sent.
  describe('encoding a value into the path', () => {
    const idParam = [{ name: 'id', in: 'path' }];
    const fill = (value: string) =>
      fillPathParameters('/books/{id}', idParam, { 'path.id': value });

    it('escapes a slash so the value stays one segment', () => {
      expect(fill('a/b')).toBe('/books/a%2Fb');
    });

    it('escapes characters that would start a query or fragment', () => {
      expect(fill('a?b')).toBe('/books/a%3Fb');
      expect(fill('a#b')).toBe('/books/a%23b');
    });

    it('escapes a space', () => {
      expect(fill('the hobbit')).toBe('/books/the%20hobbit');
    });

    it('leaves RFC 3986 unreserved characters as written', () => {
      expect(fill('v1.0~beta_2-x')).toBe('/books/v1.0~beta_2-x');
    });

    it('escapes the sub-delimiters encodeURIComponent would keep', () => {
      // `!'()*` are reserved per RFC 3986 even though encodeURIComponent
      // passes them through; swagger escapes them, so this must too.
      expect(fill("o'brien(1)!*")).toBe('/books/o%27brien%281%29%21%2A');
    });

    it('encodes non-ASCII as UTF-8 bytes', () => {
      expect(fill('café')).toBe('/books/caf%C3%A9');
    });
  });

  it('ignores non-path parameters', () => {
    expect(
      fillPathParameters('/payments/{paymentId}', [{ name: 'paymentId', in: 'query' }], {
        'query.paymentId': 'p_1',
      }),
    ).toBe('/payments/{paymentId}');
  });
});

describe('buildConsoleRequest', () => {
  it('maps filled query parameters onto rows', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'query.limit': '10', 'query.status': 'paid' },
    });

    expect(request.queryParams.map((r) => [r.name, r.value])).toEqual([
      ['limit', '10'],
      ['status', 'paid'],
    ]);
  });

  describe('the body’s media type', () => {
    const xmlOp = {
      paths: {
        '/x': { post: { requestBody: { content: { 'application/xml': { schema: {} } } } } },
      },
    };

    it('follows the operation’s declared request media type', () => {
      // Called JSON, an XML body is labelled with the wrong Content-Type and
      // reported as malformed the moment the editor validates it.
      const request = build({
        spec: xmlOp,
        path: '/x',
        method: 'post',
        baseUrl: BASE,
        bodyValue: '<order/>',
      });

      expect(request.rawFormat).toBe('xml');
    });

    it('prefers the type the caller says is being composed', () => {
      const bothOp = {
        paths: {
          '/x': {
            post: {
              requestBody: {
                content: { 'application/json': { schema: {} }, 'application/xml': { schema: {} } },
              },
            },
          },
        },
      };

      // The document lists JSON first and means neither; the editor's own
      // selection is what the user is actually typing into.
      const request = build({
        spec: bothOp,
        path: '/x',
        method: 'post',
        baseUrl: BASE,
        bodyValue: '<order/>',
        contentType: 'application/xml',
      });

      expect(request.rawFormat).toBe('xml');
    });

    it('stays json when the operation declares no request body', () => {
      const request = build({ spec, path: '/payments', method: 'get', baseUrl: BASE });

      expect(request.rawFormat).toBe('json');
    });
  });

  it('declines an operation whose verb the console does not offer', () => {
    // Built as GET, this would render `curl -X GET` for an operation the relay
    // will not send at all — a command describing a request that cannot happen.
    expect(
      buildConsoleRequest({ baseUrl: BASE, method: 'connect', path: '/payments', spec }),
    ).toBeUndefined();
    expect(
      buildConsoleRequest({ baseUrl: BASE, method: 'trace', path: '/payments', spec }),
    ).toBeUndefined();
  });

  it('omits an untouched optional query parameter rather than sending it empty', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'query.limit': '10' },
    });

    // `?status=` and no `status` mean different things to most servers.
    expect(request.queryParams.map((r) => r.name)).toEqual(['limit']);
  });

  it('keeps a declared header even when empty, so the user can see it exists', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'get',
      baseUrl: BASE,
    });

    expect(request.headers.map((r) => [r.name, r.value])).toEqual([['X-Trace', '']]);
  });

  it('uppercases the method', () => {
    expect(build({ spec, path: '/payments', method: 'post', baseUrl: BASE }).method).toBe('POST');
  });

  it('strips a trailing slash from the base url', () => {
    expect(build({ spec, path: '/payments', method: 'get', baseUrl: `${BASE}/` }).baseUrl).toBe(
      BASE,
    );
  });

  it('appends the console’s own headers after the operation’s', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'get',
      baseUrl: BASE,
      extraHeaders: [
        { id: 'k', name: 'Test-Key', value: 'live', enabled: true, secret: true, auto: true },
      ],
    });

    expect(request.headers.map((r) => r.name)).toEqual(['X-Trace', 'Test-Key']);
    expect(request.headers[1]).toMatchObject({ secret: true, auto: true });
  });

  it('does not inject a query parameter the operation already declares', () => {
    const withKeyQuery = {
      paths: { '/x': { get: { parameters: [{ name: 'apiKey', in: 'query' }] } } },
    };

    const request = build({
      spec: withKeyQuery,
      path: '/x',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'query.apiKey': 'user-typed' },
      extraQueryParams: [
        { id: 'k', name: 'apiKey', value: 'injected', enabled: true, secret: true },
      ],
    });

    // Two rows of the same name put `?apiKey=user-typed&apiKey=injected` in the
    // generated command, while the interceptor's `searchParams.set` sends only
    // one — the command would stop describing the request.
    expect(request.queryParams).toHaveLength(1);
  });

  it('keeps a declared query parameter whose name differs only by case', () => {
    // Query parameter names are case-sensitive, so this is a second parameter
    // rather than the same one spelled differently.
    const withBothCases = {
      paths: { '/x': { get: { parameters: [{ name: 'apikey', in: 'query' }] } } },
    };

    const request = build({
      spec: withBothCases,
      path: '/x',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'query.apikey': 'user-typed' },
      extraQueryParams: [
        { id: 'k', name: 'apiKey', value: 'injected', enabled: true, secret: true },
      ],
    });

    expect(request.queryParams.map((r) => r.name)).toEqual(['apikey', 'apiKey']);
  });

  it('lets the injected header win over one the operation declares', () => {
    const withKeyHeader = {
      paths: { '/x': { get: { parameters: [{ name: 'Test-Key', in: 'header' }] } } },
    };

    const request = build({
      spec: withKeyHeader,
      path: '/x',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'header.Test-Key': 'user-typed' },
      extraHeaders: [{ id: 'k', name: 'test-key', value: 'injected', enabled: true, secret: true }],
    });

    // One row, not two: the same header sent twice is ambiguous, and the
    // console's own credential is the one that actually authenticates. Query
    // parameters resolve a name collision the same way.
    expect(request.headers).toHaveLength(1);
    expect(request.headers[0].value).toBe('injected');
  });

  it('reports a JSON body as raw JSON', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'post',
      baseUrl: BASE,
      bodyValue: '{"amount":2500}',
    });

    expect(request.bodyMode).toBe('raw');
    expect(request.rawFormat).toBe('json');
  });

  it('adds no Content-Type header, because the body implies it', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'post',
      baseUrl: BASE,
      bodyValue: '{"amount":2500}',
    });

    // `contentTypeFor` derives it and `toCurl` emits it, so switching the Body
    // tab's format changes the command with no header row to keep in step.
    expect(request.headers.map((r) => r.name)).not.toContain('Content-Type');
  });

  it('keeps a Content-Type the operation itself declares', () => {
    const withContentType = {
      paths: { '/x': { post: { parameters: [{ name: 'Content-Type', in: 'header' }] } } },
    };

    const request = build({
      spec: withContentType,
      path: '/x',
      method: 'post',
      baseUrl: BASE,
      parameterValues: { 'header.Content-Type': 'application/xml' },
      bodyValue: '<x/>',
    });

    expect(request.headers.filter((r) => r.name.toLowerCase() === 'content-type')).toHaveLength(1);
    expect(request.headers[0].value).toBe('application/xml');
  });

  it('reports no body when the body is blank', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'post',
      baseUrl: BASE,
      bodyValue: '   ',
    });

    expect(request.bodyMode).toBe('none');
  });

  it('serializes a structured body', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'post',
      baseUrl: BASE,
      bodyValue: { amount: 2500 },
    });

    expect(JSON.parse(request.body)).toEqual({ amount: 2500 });
  });

  it('produces a request whose url round-trips through the curl builder', () => {
    // The point of the shared model: what is built here is what gets printed.
    const request = build({
      spec,
      path: '/payments/{paymentId}',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'path.paymentId': 'p_1', 'query.expand': 'refunds' },
    });

    expect(buildRequestUrl(request)).toBe(`${BASE}/payments/p_1?expand=refunds`);
  });

  it('gives every row a distinct id', () => {
    const request = build({
      spec,
      path: '/payments',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'query.limit': '1', 'query.status': 'paid' },
    });

    const ids = [...request.queryParams, ...request.headers].map((r) => r.id);

    expect(new Set(ids).size).toBe(ids.length);
  });
});
