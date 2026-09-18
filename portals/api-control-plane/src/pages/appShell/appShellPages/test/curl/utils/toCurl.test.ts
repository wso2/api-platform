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

import { execFileSync } from 'node:child_process';
import { describe, expect, it } from 'vitest';

import {
  buildRequestUrl,
  curlSummary,
  MASKED_VALUE,
  shellQuote,
  toCurl,
  toCurlScript,
} from './toCurl';
import {
  HTTP_METHODS,
  normalizeMethod,
  rawFormatFor,
  type ConsoleRequest,
  type KeyValueRow,
} from '../../utils/types';

const row = (overrides: Partial<KeyValueRow> = {}): KeyValueRow => ({
  id: 'r1',
  name: 'X-Thing',
  value: 'v',
  enabled: true,
  ...overrides,
});

const request = (overrides: Partial<ConsoleRequest> = {}): ConsoleRequest => ({
  method: 'GET',
  baseUrl: 'https://gw.example.com/default/payments-api/v1.0',
  path: '/payments',
  queryParams: [],
  headers: [],
  bodyMode: 'none',
  rawFormat: 'json',
  body: '',
  formFields: [],
  ...overrides,
});

/** A raw body of one format, saving `bodyMode`/`rawFormat` at every call site. */
const raw = (body: string, rawFormat: ConsoleRequest['rawFormat'] = 'json') =>
  ({ body, bodyMode: 'raw', rawFormat }) as Partial<ConsoleRequest>;

describe('shellQuote', () => {
  it('single-quotes so the shell expands nothing inside', () => {
    // Double quotes would let the shell expand these; the server must receive
    // them literally.
    expect(shellQuote('$HOME and `whoami`')).toBe("'$HOME and `whoami`'");
  });

  it('escapes an embedded single quote without breaking the quoted run', () => {
    // The classic bug: a naive wrapper produces 'it's', which the shell reads
    // as an unterminated string and the command fails to parse.
    expect(shellQuote("it's")).toBe("'it'\\''s'");
  });

  it('survives a value that is only quotes', () => {
    // Verified against a real `sh`: this string echoes back as exactly `'''`.
    expect(shellQuote("'''")).toBe("''\\'''\\'''\\'''");
  });

  it('leaves double quotes untouched', () => {
    expect(shellQuote('{"a":"b"}')).toBe(`'{"a":"b"}'`);
  });

  /**
   * The assertions above are hand-written literals, and a wrong one asserts the
   * wrong behaviour just as confidently as a right one. This runs the quoted
   * value through an actual POSIX shell and checks it comes back byte-identical
   * — the only assertion that tests what this function is really for.
   */
  describe('round-trips through a real shell', () => {
    const nasty = [
      "it's",
      "'''",
      '$HOME',
      '`whoami`',
      '"double"',
      '{"note":"it\'s $5 `x`"}',
      'a b\tc',
      'back\\slash',
      'semi;colon && echo pwned',
      '*',
      '~/path',
      'é中文',
      '',
    ];

    it.each(nasty)('%j survives the shell unchanged', (value) => {
      const printed = execFileSync('sh', ['-c', `printf %s ${shellQuote(value)}`], {
        encoding: 'utf8',
      });

      expect(printed).toBe(value);
    });
  });
});

describe('buildRequestUrl', () => {
  it('joins base and path', () => {
    expect(buildRequestUrl(request())).toBe(
      'https://gw.example.com/default/payments-api/v1.0/payments',
    );
  });

  it('does not double the slash when the base has a trailing one', () => {
    expect(buildRequestUrl(request({ baseUrl: 'https://gw.example.com/v1/' }))).toBe(
      'https://gw.example.com/v1/payments',
    );
  });

  it('adds the missing leading slash to a hand-typed path', () => {
    expect(buildRequestUrl(request({ baseUrl: 'https://gw.example.com', path: 'payments' }))).toBe(
      'https://gw.example.com/payments',
    );
  });

  it('leaves an empty path as the bare base', () => {
    expect(buildRequestUrl(request({ baseUrl: 'https://gw.example.com', path: '' }))).toBe(
      'https://gw.example.com',
    );
  });

  it('keeps spec placeholders legible rather than percent-encoding them', () => {
    // Encoding the path would turn `{paymentId}` into `%7BpaymentId%7D`, which
    // is not what the user typed and not what the gateway routes on.
    expect(buildRequestUrl(request({ path: '/payments/{paymentId}' }))).toContain(
      '/payments/{paymentId}',
    );
  });

  it('appends enabled query parameters', () => {
    const url = buildRequestUrl(
      request({
        queryParams: [
          row({ id: 'q1', name: 'limit', value: '10' }),
          row({ id: 'q2', name: 'status', value: 'paid' }),
        ],
      }),
    );

    expect(url).toContain('?limit=10&status=paid');
  });

  it('encodes query values, so an & cannot split one parameter into two', () => {
    const url = buildRequestUrl(request({ queryParams: [row({ name: 'q', value: 'a&b=c' })] }));

    expect(url).toContain('?q=a%26b%3Dc');
  });

  it('encodes query names too', () => {
    expect(buildRequestUrl(request({ queryParams: [row({ name: 'a b', value: '1' })] }))).toContain(
      '?a%20b=1',
    );
  });

  it('omits unchecked rows', () => {
    const url = buildRequestUrl(
      request({
        queryParams: [
          row({ id: 'q1', name: 'limit', value: '10' }),
          row({ id: 'q2', name: 'secret', value: 'x', enabled: false }),
        ],
      }),
    );

    expect(url).toBe('https://gw.example.com/default/payments-api/v1.0/payments?limit=10');
  });

  it('omits the blank trailing row the table always renders', () => {
    // The editor keeps an empty row at the bottom to type into; it must never
    // reach the URL as `=`.
    expect(
      buildRequestUrl(request({ queryParams: [row({ name: '  ', value: '' })] })),
    ).not.toContain('?');
  });
});

describe('toCurl', () => {
  it('emits the method explicitly, even for GET', () => {
    expect(toCurl(request(), { revealSecrets: true })).toContain('curl -X GET');
  });

  it('quotes the URL so a query string cannot be split by the shell', () => {
    const command = toCurl(request({ queryParams: [row({ name: 'a', value: '1' })] }), {
      revealSecrets: true,
    });

    // Shell-quoted like every other value in the command, rather than wrapped
    // in double quotes: single quotes protect `&`, `?` and `$` alike, and one
    // quoting rule for the whole command is one rule to get right.
    expect(command).toContain(`'https://gw.example.com/default/payments-api/v1.0/payments?a=1'`);
  });

  it('emits one -H per enabled header', () => {
    const command = toCurl(
      request({
        headers: [
          row({ id: 'h1', name: 'Test-Key', value: 'abc' }),
          row({ id: 'h2', name: 'Content-Type', value: 'application/json' }),
        ],
      }),
      { revealSecrets: true },
    );

    expect(command).toContain("-H 'Test-Key: abc'");
    expect(command).toContain("-H 'Content-Type: application/json'");
  });

  it('omits unchecked and unnamed headers', () => {
    const command = toCurl(
      request({
        headers: [
          row({ id: 'h1', name: 'Kept', value: '1' }),
          row({ id: 'h2', name: 'Dropped', value: '2', enabled: false }),
          row({ id: 'h3', name: '', value: 'nameless' }),
        ],
      }),
      { revealSecrets: true },
    );

    expect(command).toContain("-H 'Kept: 1'");
    expect(command).not.toContain('Dropped');
    expect(command).not.toContain('nameless');
  });

  it('masks a secret header when secrets are not revealed', () => {
    const command = toCurl(
      request({ headers: [row({ name: 'Test-Key', value: 'live-credential', secret: true })] }),
      { revealSecrets: false },
    );

    expect(command).toContain(`-H 'Test-Key: ${MASKED_VALUE}'`);
    expect(command).not.toContain('live-credential');
  });

  it('prints a secret in full when revealed, since a masked command does not run', () => {
    const command = toCurl(
      request({ headers: [row({ name: 'Test-Key', value: 'live-credential', secret: true })] }),
      { revealSecrets: true },
    );

    expect(command).toContain("-H 'Test-Key: live-credential'");
  });

  it('never masks a non-secret header', () => {
    const command = toCurl(request({ headers: [row({ name: 'X-Trace', value: 'plain' })] }), {
      revealSecrets: false,
    });

    expect(command).toContain("-H 'X-Trace: plain'");
  });

  it('emits -d for a JSON body', () => {
    const command = toCurl(request({ method: 'POST', ...raw('{"amount": 2500}') }), {
      revealSecrets: true,
    });

    // Byte for byte as typed. Re-serialising through `JSON.parse` would round a
    // number too wide for a double and drop a duplicate key, so the command
    // would stop describing the body the console sends.
    expect(command).toContain(`-d '{"amount": 2500}'`);
  });

  it('keeps a pretty-printed JSON body exactly as written', () => {
    const body = '{\n  "amount": 2500,\n  "currency": "USD"\n}';
    const command = toCurl(request({ method: 'POST', ...raw(body) }), { revealSecrets: true });

    // The newlines survive inside the single quotes, so the pasted command
    // sends the same bytes the editor shows.
    expect(command).toContain(`-d '${body}'`);
  });

  it('emits no -d in None mode, even with body text left over', () => {
    // Switching to None must not send a body the user thought they had removed.
    const command = toCurl(request({ method: 'POST', body: '{"stale": true}', bodyMode: 'none' }), {
      revealSecrets: true,
    });

    expect(command).not.toContain('-d');
  });

  it('emits no -d for an empty JSON body, rather than -d with nothing in it', () => {
    const command = toCurl(request({ method: 'POST', ...raw('   ') }), { revealSecrets: true });

    expect(command).not.toContain('-d');
  });

  it('quotes a body containing single quotes so the command still parses', () => {
    const command = toCurl(request({ method: 'POST', ...raw(`{"note":"it's fine"}`) }), {
      revealSecrets: true,
    });

    expect(command).toContain(`-d '{"note":"it'\\''s fine"}'`);
  });

  it('leaves no dangling line continuation on the last line', () => {
    // A trailing backslash makes the shell wait for more input, so pasting the
    // command appears to hang.
    expect(toCurl(request(), { revealSecrets: true })).not.toMatch(/\\$/);
    expect(toCurl(request({ headers: [row()] }), { revealSecrets: true })).not.toMatch(/\\$/);
    expect(toCurl(request({ method: 'POST', ...raw('{}') }), { revealSecrets: true })).not.toMatch(
      /\\$/,
    );
  });

  it('continues every line except the last', () => {
    const lines = toCurl(request({ headers: [row()] }), { revealSecrets: true }).split('\n');

    expect(lines.slice(0, -1).every((line) => line.endsWith('\\'))).toBe(true);
    expect(lines.at(-1)?.endsWith('\\')).toBe(false);
  });
});

describe('Content-Type', () => {
  it('is derived from the raw format, not from a header the user must maintain', () => {
    const command = (format: ConsoleRequest['rawFormat'], body: string) =>
      toCurl(request({ method: 'POST', ...raw(body, format) }), { revealSecrets: true });

    expect(command('json', '{}')).toContain("-H 'Content-Type: application/json'");
    expect(command('xml', '<x/>')).toContain("-H 'Content-Type: application/xml'");
    expect(command('text', 'hello')).toContain("-H 'Content-Type: text/plain'");
  });

  it('is set for a url-encoded body', () => {
    const command = toCurl(
      request({ bodyMode: 'url-encoded', formFields: [row({ name: 'a', value: '1' })] }),
      { revealSecrets: true },
    );

    expect(command).toContain("-H 'Content-Type: application/x-www-form-urlencoded'");
  });

  it('is NOT set for form-data, because curl must generate the boundary', () => {
    const command = toCurl(
      request({ bodyMode: 'form-data', formFields: [row({ name: 'a', value: '1' })] }),
      { revealSecrets: true },
    );

    // A multipart Content-Type we wrote by hand would carry no boundary, and
    // the server could not split the body it was handed.
    expect(command).not.toContain('Content-Type');
  });

  it('is not set when there is no body', () => {
    expect(toCurl(request(), { revealSecrets: true })).not.toContain('Content-Type');
    expect(
      toCurl(request({ bodyMode: 'url-encoded', formFields: [] }), { revealSecrets: true }),
    ).not.toContain('Content-Type');
  });

  it('yields to a Content-Type the user typed, rather than emitting both', () => {
    const command = toCurl(
      request({
        headers: [row({ name: 'content-type', value: 'application/vnd.api+json' })],
        method: 'POST',
        ...raw('{}'),
      }),
      { revealSecrets: true },
    );

    // Matched case-insensitively, as HTTP header names are. Emitting both would
    // leave which one applies up to the server.
    expect(command).toContain("-H 'content-type: application/vnd.api+json'");
    expect(command).not.toContain('application/json');
  });
});

describe('encoded bodies', () => {
  it('emits --data-urlencode per field, letting curl encode the values', () => {
    const command = toCurl(
      request({
        bodyMode: 'url-encoded',
        formFields: [
          row({ id: 'f1', name: 'title', value: 'The Hobbit' }),
          row({ id: 'f2', name: 'note', value: 'a&b=c' }),
        ],
      }),
      { revealSecrets: true },
    );

    // Joining these into one `-d 'a=1&b=2'` would mean encoding them
    // ourselves, and an unescaped `&` in a value would split one field in two.
    expect(command).toContain("--data-urlencode 'title=The Hobbit'");
    expect(command).toContain("--data-urlencode 'note=a&b=c'");
  });

  it('emits -F per field for form data', () => {
    const command = toCurl(
      request({
        bodyMode: 'form-data',
        formFields: [row({ name: 'title', value: 'The Hobbit' })],
      }),
      { revealSecrets: true },
    );

    // `--form-string`, never `-F`: `-F` reads a leading `@` or `<` as a file
    // path, so a value the user typed could make the pasted command upload a
    // local file.
    expect(command).toContain("--form-string 'title=The Hobbit'");
  });

  it('omits unchecked and unnamed fields', () => {
    const command = toCurl(
      request({
        bodyMode: 'url-encoded',
        formFields: [
          row({ id: 'f1', name: 'kept', value: '1' }),
          row({ id: 'f2', name: 'dropped', value: '2', enabled: false }),
          row({ id: 'f3', name: '  ', value: 'nameless' }),
        ],
      }),
      { revealSecrets: true },
    );

    expect(command).toContain("--data-urlencode 'kept=1'");
    expect(command).not.toContain('dropped');
    expect(command).not.toContain('nameless');
  });

  it('shell-quotes a field value containing a quote', () => {
    const command = toCurl(
      request({ bodyMode: 'form-data', formFields: [row({ name: 'note', value: "it's" })] }),
      { revealSecrets: true },
    );

    expect(command).toContain(`--form-string 'note=it'\\''s'`);
  });

  it('keeps the last line free of a dangling continuation', () => {
    const command = toCurl(
      request({ bodyMode: 'url-encoded', formFields: [row({ name: 'a', value: '1' })] }),
      { revealSecrets: true },
    );

    expect(command).not.toMatch(/\\$/);
  });

  it('sends no body fields when the mode is raw', () => {
    // Switching back to raw must not smuggle the form fields along.
    const command = toCurl(
      request({ formFields: [row({ name: 'stale', value: '1' })], method: 'POST', ...raw('{}') }),
      { revealSecrets: true },
    );

    expect(command).not.toContain('stale');
  });
});

describe('raw bodies other than JSON', () => {
  it('emits XML verbatim, preserving its newlines', () => {
    const body = '<book>\n  <title>The Hobbit</title>\n</book>';
    const command = toCurl(request({ method: 'POST', ...raw(body, 'xml') }), {
      revealSecrets: true,
    });

    // Collapsing whitespace here would change the text content the server
    // receives, which is why only JSON is re-serialised.
    expect(command).toContain(body);
  });

  it('emits text verbatim', () => {
    const command = toCurl(request({ method: 'POST', ...raw('line one\nline two', 'text') }), {
      revealSecrets: true,
    });

    expect(command).toContain('line one\nline two');
  });

  it('emits invalid JSON as typed rather than dropping the body', () => {
    const command = toCurl(request({ method: 'POST', ...raw('{"unclosed":') }), {
      revealSecrets: true,
    });

    expect(command).toContain(`-d '{"unclosed":'`);
  });
});

describe('curlSummary', () => {
  it('counts only headers that travel', () => {
    const summary = curlSummary(
      request({
        headers: [
          row({ id: 'h1', name: 'A', value: '1' }),
          row({ id: 'h2', name: 'B', value: '2', enabled: false }),
          row({ id: 'h3', name: '', value: '3' }),
        ],
      }),
    );

    expect(summary).toEqual({ method: 'GET', headerCount: 1, bodyKind: 'none' });
  });

  it('names how the body is encoded', () => {
    expect(curlSummary(request(raw('{}'))).bodyKind).toBe('json');
    expect(curlSummary(request(raw('<x/>', 'xml'))).bodyKind).toBe('xml');
    expect(curlSummary(request(raw('hello', 'text'))).bodyKind).toBe('text');
    expect(
      curlSummary(request({ bodyMode: 'url-encoded', formFields: [row({ name: 'a' })] })).bodyKind,
    ).toBe('url-encoded');
    expect(
      curlSummary(request({ bodyMode: 'form-data', formFields: [row({ name: 'a' })] })).bodyKind,
    ).toBe('form-data');
  });

  it('reports no body when the mode carries nothing', () => {
    expect(curlSummary(request({ body: '{}', bodyMode: 'none' })).bodyKind).toBe('none');
    expect(curlSummary(request({ bodyMode: 'url-encoded', formFields: [] })).bodyKind).toBe('none');
  });
});

describe('toCurlScript', () => {
  it('is runnable and fails loudly, rather than exiting 0 on a failed curl', () => {
    const script = toCurlScript(request());

    expect(script.startsWith('#!/usr/bin/env sh')).toBe(true);
    expect(script).toContain('set -eu');
    expect(script.endsWith('\n')).toBe(true);
  });

  it('always includes real credentials — a masked script would not work', () => {
    const script = toCurlScript(
      request({ headers: [row({ name: 'Test-Key', value: 'live-credential', secret: true })] }),
    );

    expect(script).toContain('live-credential');
    expect(script).not.toContain(MASKED_VALUE);
  });
});

describe('rawFormatFor', () => {
  it('maps the media types the console can validate', () => {
    expect(rawFormatFor('application/json')).toBe('json');
    expect(rawFormatFor('application/xml')).toBe('xml');
    expect(rawFormatFor('text/xml')).toBe('xml');
    expect(rawFormatFor('text/plain')).toBe('text');
  });

  it('ignores parameters and case, as a real Content-Type carries both', () => {
    expect(rawFormatFor('Application/JSON; charset=utf-8')).toBe('json');
    expect(rawFormatFor('TEXT/Plain; charset=utf-8')).toBe('text');
  });

  it('reads a structured-syntax suffix', () => {
    // `application/problem+json` is JSON; `image/svg+xml` is XML.
    expect(rawFormatFor('application/problem+json')).toBe('json');
    expect(rawFormatFor('image/svg+xml')).toBe('xml');
  });

  it('falls back to json for anything unrecognised or absent', () => {
    // What every caller assumed unconditionally before this existed.
    expect(rawFormatFor(undefined)).toBe('json');
    expect(rawFormatFor('')).toBe('json');
    expect(rawFormatFor('application/octet-stream')).toBe('json');
  });
});

describe('normalizeMethod', () => {
  it('uppercases a lowercase OpenAPI path-item key', () => {
    expect(normalizeMethod('get')).toBe('GET');
    expect(normalizeMethod('  patch ')).toBe('PATCH');
  });

  it('declines a verb the console does not offer', () => {
    // Answering `GET` would have the cURL panel print `-X GET` for a request
    // executed as something else.
    expect(normalizeMethod('CONNECT')).toBeUndefined();
    expect(normalizeMethod('NOTAVERB')).toBeUndefined();
  });

  it('declines TRACE and CONNECT, which the relay refuses', () => {
    // The BFF relay keeps the same closed set in `allowedMethods`
    // (bff/internal/testproxy/sanitize.go) and answers `ErrInvalidMethod` to
    // anything outside it, so offering either here would put a verb in the
    // picker that can never be sent.
    expect(HTTP_METHODS).not.toContain('TRACE');
    expect(HTTP_METHODS).not.toContain('CONNECT');
    expect(normalizeMethod('trace')).toBeUndefined();
    expect(normalizeMethod('connect')).toBeUndefined();
  });

  it('recognises every verb the console offers', () => {
    for (const method of HTTP_METHODS) {
      expect(normalizeMethod(method.toLowerCase())).toBe(method);
    }
  });

  it('still defaults a missing value to GET', () => {
    // Absent is a value to fill in, not a verb to disagree with.
    expect(normalizeMethod(undefined)).toBe('GET');
    expect(normalizeMethod('')).toBe('GET');
    expect(normalizeMethod('   ')).toBe('GET');
  });
});
