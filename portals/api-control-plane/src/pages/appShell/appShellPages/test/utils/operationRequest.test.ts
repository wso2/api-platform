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

const BASE = 'https://gw.example.com/default/payments-api/v1.0';

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
    const request = buildConsoleRequest({
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

  it('omits an untouched optional query parameter rather than sending it empty', () => {
    const request = buildConsoleRequest({
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
    const request = buildConsoleRequest({
      spec,
      path: '/payments',
      method: 'get',
      baseUrl: BASE,
    });

    expect(request.headers.map((r) => [r.name, r.value])).toEqual([['X-Trace', '']]);
  });

  it('uppercases the method', () => {
    expect(
      buildConsoleRequest({ spec, path: '/payments', method: 'post', baseUrl: BASE }).method,
    ).toBe('POST');
  });

  it('strips a trailing slash from the base url', () => {
    expect(
      buildConsoleRequest({ spec, path: '/payments', method: 'get', baseUrl: `${BASE}/` }).baseUrl,
    ).toBe(BASE);
  });

  it('appends the console’s own headers after the operation’s', () => {
    const request = buildConsoleRequest({
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

  it('does not inject a header the operation already declares', () => {
    const withKeyHeader = {
      paths: { '/x': { get: { parameters: [{ name: 'Test-Key', in: 'header' }] } } },
    };

    const request = buildConsoleRequest({
      spec: withKeyHeader,
      path: '/x',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'header.Test-Key': 'user-typed' },
      extraHeaders: [{ id: 'k', name: 'test-key', value: 'injected', enabled: true, secret: true }],
    });

    // Shadowing the user's own value with an injected one, with no way to tell
    // which was sent, is worse than not injecting.
    expect(request.headers).toHaveLength(1);
    expect(request.headers[0].value).toBe('user-typed');
  });

  it('reports a JSON body as raw JSON', () => {
    const request = buildConsoleRequest({
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
    const request = buildConsoleRequest({
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

    const request = buildConsoleRequest({
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
    const request = buildConsoleRequest({
      spec,
      path: '/payments',
      method: 'post',
      baseUrl: BASE,
      bodyValue: '   ',
    });

    expect(request.bodyMode).toBe('none');
  });

  it('serializes a structured body', () => {
    const request = buildConsoleRequest({
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
    const request = buildConsoleRequest({
      spec,
      path: '/payments/{paymentId}',
      method: 'get',
      baseUrl: BASE,
      parameterValues: { 'path.paymentId': 'p_1', 'query.expand': 'refunds' },
    });

    expect(buildRequestUrl(request)).toBe(`${BASE}/payments/p_1?expand=refunds`);
  });

  it('gives every row a distinct id', () => {
    const request = buildConsoleRequest({
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
