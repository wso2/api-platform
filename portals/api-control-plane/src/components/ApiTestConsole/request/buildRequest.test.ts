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

import { parseConsoleSpec } from '../spec/parseSpec';
import type { ConsoleOperation } from '../types';
import {
  BODY_PROBLEM_KEY,
  buildSendRequest,
  headerProblemKey,
  paramProblemKey,
  type RequestDraft,
} from './buildRequest';

const SPEC = {
  paths: {
    '/books/{id}': {
      get: {
        parameters: [
          { in: 'path', name: 'id', schema: { type: 'string' } },
          { in: 'query', name: 'limit', schema: { maximum: 100, minimum: 1, type: 'integer' } },
          { in: 'query', name: 'status', schema: { enum: ['a', 'b'], type: 'string' } },
          { in: 'query', name: 'tags', schema: { type: 'array', items: { type: 'string' } } },
          { in: 'header', name: 'X-Trace', schema: { type: 'string' } },
        ],
        responses: {},
      },
      put: {
        requestBody: {
          content: { 'application/json': { schema: { type: 'object' } }, 'application/xml': {} },
          required: true,
        },
        parameters: [{ in: 'path', name: 'id', schema: { type: 'string' } }],
        responses: {},
      },
    },
    '/form': {
      post: {
        requestBody: {
          content: {
            'multipart/form-data': {
              schema: {
                properties: { file: { format: 'binary', type: 'string' }, note: { type: 'string' } },
                required: ['file'],
                type: 'object',
              },
            },
          },
        },
        responses: {},
      },
    },
    '/urlencoded': {
      post: {
        requestBody: {
          content: {
            'application/x-www-form-urlencoded': {
              schema: { properties: { name: { type: 'string' } }, required: ['name'], type: 'object' },
            },
          },
        },
        responses: {},
      },
    },
  },
};

const ops = parseConsoleSpec(SPEC).operations;
const op = (key: string): ConsoleOperation => ops.find((o) => o.key === key) as ConsoleOperation;

const draft = (overrides: Partial<RequestDraft> = {}): RequestDraft => ({
  bodyMediaType: '',
  bodyText: '',
  files: {},
  formValues: {},
  headers: [],
  params: {},
  ...overrides,
});

const build = (key: string, d: RequestDraft, extra: Partial<Parameters<typeof buildSendRequest>[0]> = {}) =>
  buildSendRequest({
    autoHeaders: [],
    autoQuery: [],
    baseUrl: 'https://gw.example.com/p/',
    draft: d,
    operation: op(key),
    ...extra,
  });

describe('buildSendRequest — parameters', () => {
  it('requires a path parameter', () => {
    const result = build('GET /books/{id}', draft());
    expect(result.request).toBeUndefined();
    expect(result.problems[paramProblemKey({ in: 'path', name: 'id' })]).toEqual({ code: 'required' });
  });

  it('encodes a path value as one segment, so it cannot climb or add segments', () => {
    const result = build('GET /books/{id}', draft({ params: { 'path:id': '../admin/x' } }));
    expect(result.request?.path).toBe('/books/..%2Fadmin%2Fx');
  });

  it('keeps the gateway URL without a trailing slash', () => {
    expect(build('GET /books/{id}', draft({ params: { 'path:id': '1' } })).request?.baseUrl).toBe(
      'https://gw.example.com/p',
    );
  });

  it('checks numbers, ranges and enums', () => {
    const problems = (params: Record<string, string>) =>
      build('GET /books/{id}', draft({ params: { 'path:id': '1', ...params } })).problems;

    expect(problems({ 'query:limit': 'abc' })['param:query:limit']).toEqual({ code: 'integer' });
    expect(problems({ 'query:limit': '1.5' })['param:query:limit']).toEqual({ code: 'integer' });
    expect(problems({ 'query:limit': '0' })['param:query:limit']).toMatchObject({ code: 'min', min: 1 });
    expect(problems({ 'query:limit': '101' })['param:query:limit']).toMatchObject({ code: 'max', max: 100 });
    expect(problems({ 'query:status': 'c' })['param:query:status']).toEqual({ code: 'enum' });
    expect(problems({ 'query:limit': '10', 'query:status': 'a' })).toEqual({});
  });

  it('omits an empty optional parameter rather than sending it blank', () => {
    const { request } = build('GET /books/{id}', draft({ params: { 'path:id': '1', 'query:limit': '' } }));
    expect(request?.query).toEqual([]);
  });

  it('sends an array as repeated query parameters', () => {
    const { request } = build('GET /books/{id}', draft({ params: { 'path:id': '1', 'query:tags': 'a, b ,,c' } }));
    expect(request?.query).toEqual([['tags', 'a'], ['tags', 'b'], ['tags', 'c']]);
  });

  it('sends a declared header parameter', () => {
    const { request } = build('GET /books/{id}', draft({ params: { 'path:id': '1', 'header:X-Trace': 't1' } }));
    expect(request?.headers).toEqual([['X-Trace', 't1']]);
  });
});

describe('buildSendRequest — headers', () => {
  const base = { params: { 'path:id': '1' } };

  it('refuses a header the browser or gateway owns, and a malformed one', () => {
    const result = build(
      'GET /books/{id}',
      draft({
        ...base,
        headers: [
          { id: 'a', name: 'Host', value: 'evil.example.com' },
          { id: 'b', name: 'Bad Name', value: 'x' },
          { id: 'c', name: 'X-Ok', value: 'a\r\nInjected: 1' },
        ],
      }),
    );
    expect(result.request).toBeUndefined();
    expect(result.problems[headerProblemKey('a')]).toEqual({ code: 'headerForbidden' });
    expect(result.problems[headerProblemKey('b')]).toEqual({ code: 'headerName' });
    expect(result.problems[headerProblemKey('c')]).toEqual({ code: 'headerValue' });
  });

  it('lets the console’s credential replace one the user typed under the same name', () => {
    const { request, secretHeaders } = build(
      'GET /books/{id}',
      draft({ ...base, headers: [{ id: 'a', name: 'x-api-key', value: 'user-typed' }] }),
      { autoHeaders: [{ name: 'X-API-Key', secret: true, value: 'from-console' }] },
    );
    expect(request?.headers).toEqual([['X-API-Key', 'from-console']]);
    expect(secretHeaders.has('x-api-key')).toBe(true);
  });

  it('lets the console’s credential replace a query parameter of the same name', () => {
    const { request, secretQuery } = build(
      'GET /books/{id}',
      draft({ params: { 'path:id': '1', 'query:status': 'a' } }),
      { autoQuery: [{ name: 'status', secret: true, value: 'k' }] },
    );
    expect(request?.query).toEqual([['status', 'k']]);
    expect(secretQuery.has('status')).toBe(true);
  });

  it('leaves Content-Type to the body’s media type', () => {
    const { request } = build(
      'PUT /books/{id}',
      draft({
        bodyMediaType: 'application/json',
        bodyText: '{"a":1}',
        headers: [{ id: 'a', name: 'Content-Type', value: 'text/html' }],
        params: { 'path:id': '1' },
      }),
    );
    expect(request?.headers).toEqual([['Content-Type', 'application/json']]);
  });
});

describe('buildSendRequest — body', () => {
  const params = { 'path:id': '1' };

  it('requires a required body', () => {
    const result = build('PUT /books/{id}', draft({ bodyMediaType: 'application/json', params }));
    expect(result.problems[BODY_PROBLEM_KEY]).toEqual({ code: 'required' });
  });

  it('rejects malformed JSON and XML, and accepts well-formed ones', () => {
    const json = (text: string) =>
      build('PUT /books/{id}', draft({ bodyMediaType: 'application/json', bodyText: text, params }));
    const xml = (text: string) =>
      build('PUT /books/{id}', draft({ bodyMediaType: 'application/xml', bodyText: text, params }));

    expect(json('{oops').problems[BODY_PROBLEM_KEY]).toEqual({ code: 'json' });
    expect(json('{"a":1}').request?.body).toEqual({ kind: 'text', mediaType: 'application/json', text: '{"a":1}' });
    expect(xml('<a><b></a>').problems[BODY_PROBLEM_KEY]).toEqual({ code: 'xml' });
    expect(xml('<a><b/></a>').request?.body).toMatchObject({ kind: 'text' });
  });

  it('sends no body on a GET, whatever is in the editor', () => {
    const { request } = build('GET /books/{id}', draft({ bodyText: '{"a":1}', params }));
    expect(request?.body).toBeUndefined();
  });

  it('builds a url-encoded body and requires its required fields', () => {
    const missing = build('POST /urlencoded', draft({ bodyMediaType: 'application/x-www-form-urlencoded' }));
    expect(missing.problems['field:name']).toEqual({ code: 'required' });

    const ok = build(
      'POST /urlencoded',
      draft({ bodyMediaType: 'application/x-www-form-urlencoded', formValues: { name: 'x' } }),
    );
    expect(ok.request?.body).toEqual({ fields: [['name', 'x']], kind: 'form' });
    expect(ok.request?.headers).toContainEqual(['Content-Type', 'application/x-www-form-urlencoded']);
  });

  it('builds a multipart body with a chosen file, and adds no Content-Type of its own', () => {
    const file = new File(['hi'], 'a.txt', { type: 'text/plain' });

    const missing = build('POST /form', draft({ bodyMediaType: 'multipart/form-data' }));
    expect(missing.problems['field:file']).toEqual({ code: 'file' });

    const ok = build(
      'POST /form',
      draft({ bodyMediaType: 'multipart/form-data', files: { file }, formValues: { note: 'n' } }),
    );
    expect(ok.request?.body).toEqual({
      kind: 'multipart',
      parts: [{ file, name: 'file' }, { name: 'note', value: 'n' }],
    });
    expect(ok.request?.headers).toEqual([]);
  });
});

describe('buildSendRequest — preview', () => {
  it('shows the valid part of the target even while something is wrong', () => {
    const result = build(
      'GET /books/{id}',
      draft({ params: { 'path:id': '7', 'query:limit': 'nope', 'query:status': 'a' } }),
    );
    expect(result.request).toBeUndefined();
    expect(result.preview).toEqual({ path: '/books/7', query: [['status', 'a']] });
  });
});
