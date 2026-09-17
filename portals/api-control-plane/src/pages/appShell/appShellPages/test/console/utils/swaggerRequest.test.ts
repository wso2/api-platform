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

import { fromSwaggerRequest, splitAgainstBase } from './swaggerRequest';

/**
 * The bridge between swagger-ui's undocumented internals and this console's own
 * model. Every test here is really asking the same question: when swagger hands
 * us something unexpected, do we return nothing rather than something wrong?
 */

const BASE = 'https://gw.example.com/default/payments-api/v1.0';

/** A minimal stand-in for an Immutable Map, matching the `.get`/`.toJS` duck type. */
const immutable = (entries: Record<string, unknown>) => ({
  get: (key: string) => entries[key],
  toJS: () => entries,
});

describe('splitAgainstBase', () => {
  it('strips the base path, leaving the operation path', () => {
    expect(splitAgainstBase(`${BASE}/payments`, BASE)).toEqual({ path: '/payments', query: [] });
  });

  it('reads the query string into pairs', () => {
    expect(splitAgainstBase(`${BASE}/payments?limit=10&status=paid`, BASE)?.query).toEqual([
      ['limit', '10'],
      ['status', 'paid'],
    ]);
  });

  it('decodes encoded query values', () => {
    expect(splitAgainstBase(`${BASE}/p?q=a%26b`, BASE)?.query).toEqual([['q', 'a&b']]);
  });

  it('returns / for a request at the base itself', () => {
    expect(splitAgainstBase(BASE, BASE)?.path).toBe('/');
  });

  it('tolerates a trailing slash on the configured base', () => {
    expect(splitAgainstBase(`${BASE}/payments`, `${BASE}/`)?.path).toBe('/payments');
  });

  it('refuses a URL on another host', () => {
    // A foreign URL is not this API's request; syncing it would put someone
    // else's host into the user's clipboard.
    expect(splitAgainstBase('https://evil.example.com/payments', BASE)).toBeUndefined();
  });

  it('refuses a URL outside the base path on the same host', () => {
    expect(splitAgainstBase('https://gw.example.com/other-api/v1/payments', BASE)).toBeUndefined();
  });

  it('refuses an unparseable URL', () => {
    expect(splitAgainstBase('not a url', BASE)).toBeUndefined();
  });

  it('refuses when the base itself is not a URL', () => {
    expect(splitAgainstBase(`${BASE}/payments`, '')).toBeUndefined();
  });
});

describe('fromSwaggerRequest', () => {
  it('builds a request from an Immutable-like object', () => {
    const result = fromSwaggerRequest(
      immutable({
        url: `${BASE}/payments?limit=10`,
        method: 'post',
        headers: immutable({ 'Content-Type': 'application/json' }),
        body: '{"amount":2500}',
      }),
      BASE,
    );

    expect(result).toMatchObject({
      method: 'POST',
      baseUrl: BASE,
      path: '/payments',
      bodyMode: 'raw',
      rawFormat: 'json',
      body: '{"amount":2500}',
    });
    expect(result?.queryParams).toHaveLength(1);
    expect(result?.queryParams[0]).toMatchObject({ name: 'limit', value: '10', enabled: true });
  });

  it('declines a verb the console does not offer', () => {
    // Reported as GET, the cURL panel would print `-X GET` for a request
    // swagger actually executed as TRACE.
    expect(
      fromSwaggerRequest({ url: `${BASE}/payments`, method: 'trace', headers: {} }, BASE),
    ).toBeUndefined();
  });

  it('works with a plain object too, not only Immutable', () => {
    const result = fromSwaggerRequest(
      { url: `${BASE}/payments`, method: 'get', headers: { Accept: 'application/json' } },
      BASE,
    );

    expect(result?.headers[0]).toMatchObject({ name: 'Accept', value: 'application/json' });
  });

  it('uppercases the method swagger stores lowercase', () => {
    expect(fromSwaggerRequest({ url: `${BASE}/p`, method: 'delete' }, BASE)?.method).toBe('DELETE');
  });

  it('marks the test-key header as secret and auto, case-insensitively', () => {
    const result = fromSwaggerRequest(
      { url: `${BASE}/p`, method: 'get', headers: { 'test-key': 'live-credential' } },
      BASE,
      'Test-Key',
    );

    // HTTP header names are case-insensitive, and swagger echoes whatever case
    // the spec used — a case-sensitive match would leave a live credential
    // rendered in plain text.
    expect(result?.headers[0]).toMatchObject({ secret: true, auto: true });
  });

  it('does not mark other headers as secret', () => {
    const result = fromSwaggerRequest(
      { url: `${BASE}/p`, method: 'get', headers: { Accept: 'application/json' } },
      BASE,
      'Test-Key',
    );

    expect(result?.headers[0].secret).toBe(false);
  });

  it('drops headers swagger left as null rather than emitting them', () => {
    const result = fromSwaggerRequest(
      { url: `${BASE}/p`, method: 'get', headers: { Accept: 'application/json', Absent: null } },
      BASE,
    );

    // Swagger nulls a header instead of removing the key; unfiltered, this
    // would print `-H 'Absent: undefined'`.
    expect(result?.headers).toHaveLength(1);
  });

  it('reports no body when there is none', () => {
    const result = fromSwaggerRequest({ url: `${BASE}/p`, method: 'get' }, BASE);

    expect(result?.bodyMode).toBe('none');
    expect(result?.body).toBe('');
  });

  it('treats a whitespace-only body as no body', () => {
    expect(
      fromSwaggerRequest({ url: `${BASE}/p`, method: 'post', body: '   ' }, BASE)?.bodyMode,
    ).toBe('none');
  });

  it('serializes a structured body rather than dropping it', () => {
    const result = fromSwaggerRequest(
      { url: `${BASE}/p`, method: 'post', body: { amount: 2500 } },
      BASE,
    );

    expect(result?.bodyMode).toBe('raw');
    expect(result?.rawFormat).toBe('json');
    expect(JSON.parse(result!.body)).toEqual({ amount: 2500 });
  });

  it('gives every synced row a distinct id, even for duplicate names', () => {
    const result = fromSwaggerRequest(
      { url: `${BASE}/p?a=1&a=2`, method: 'get', headers: { A: '1', B: '2' } },
      BASE,
    );

    const ids = [...result!.queryParams, ...result!.headers].map((r) => r.id);

    // Duplicate React keys would make one of two same-named rows un-editable.
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('returns undefined rather than a partial request when the url is missing', () => {
    // The console's rule: a failed sync leaves the previous command standing,
    // never a wrong one.
    expect(fromSwaggerRequest({ method: 'get' }, BASE)).toBeUndefined();
    expect(fromSwaggerRequest({ url: '', method: 'get' }, BASE)).toBeUndefined();
  });

  it('returns undefined for a request aimed at another host', () => {
    expect(
      fromSwaggerRequest({ url: 'https://evil.example.com/p', method: 'get' }, BASE),
    ).toBeUndefined();
  });

  it('returns undefined for junk instead of throwing', () => {
    // Swagger's internals are unpublished API; a shape change must degrade the
    // sync, not crash the page.
    expect(fromSwaggerRequest(undefined, BASE)).toBeUndefined();
    expect(fromSwaggerRequest(null, BASE)).toBeUndefined();
    expect(fromSwaggerRequest('nonsense', BASE)).toBeUndefined();
    expect(fromSwaggerRequest(42, BASE)).toBeUndefined();
  });
});
