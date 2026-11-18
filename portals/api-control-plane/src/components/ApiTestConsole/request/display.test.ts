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

import type { ConsoleSendRequest } from '../types';
import { displayUrl, formFileArg, MASKED_VALUE, shellQuote, toCurl } from './display';

const request: ConsoleSendRequest = {
  baseUrl: 'https://gw.example.com/p',
  body: { kind: 'text', mediaType: 'application/json', text: '{"a":"it\'s"}' },
  headers: [
    ['X-API-Key', 'top-secret'],
    ['Content-Type', 'application/json'],
  ],
  method: 'POST',
  path: '/order',
  query: [
    ['q', 'a b&c'],
    ['api_key', 'also-secret'],
  ],
};
const secrets = { secretHeaders: new Set(['x-api-key']), secretQuery: new Set(['api_key']) };

describe('displayUrl', () => {
  it('encodes query names and values', () => {
    expect(displayUrl(request, { reveal: true })).toBe(
      'https://gw.example.com/p/order?q=a%20b%26c&api_key=also-secret',
    );
  });

  it('masks a credential in the query when it is only being shown', () => {
    expect(displayUrl(request, { reveal: false, ...secrets })).toContain(
      `api_key=${encodeURIComponent(MASKED_VALUE)}`,
    );
  });
});

describe('toCurl', () => {
  it('masks credentials on screen and reveals them for the clipboard', () => {
    const shown = toCurl(request, { reveal: false, ...secrets });
    const copied = toCurl(request, { reveal: true, ...secrets });

    expect(shown).not.toContain('top-secret');
    expect(shown).not.toContain('also-secret');
    expect(shown).toContain(MASKED_VALUE);
    expect(copied).toContain('top-secret');
    expect(copied).toContain('also-secret');
  });

  it('quotes a body so it cannot end the shell quoting', () => {
    expect(toCurl(request, { reveal: true })).toContain(`--data-raw '{"a":"it'\\''s"}'`);
  });

  it('quotes a hostile value as one argument', () => {
    const hostile: ConsoleSendRequest = {
      ...request,
      body: undefined,
      headers: [['X-A', `x'; rm -rf / #`]],
    };
    expect(toCurl(hostile, { reveal: true })).toContain(`-H 'X-A: x'\\''; rm -rf / #'`);
  });

  it('does not let a form value be read as a file', () => {
    const multipart: ConsoleSendRequest = {
      ...request,
      body: { kind: 'multipart', parts: [{ name: 'note', value: '@/etc/passwd' }] },
    };
    expect(toCurl(multipart, { reveal: true })).toContain(`--form-string 'note=@/etc/passwd'`);
  });
});

describe('toCurl — files and HEAD', () => {
  const upload = (filename: string): ConsoleSendRequest => ({
    ...request,
    body: {
      kind: 'multipart',
      parts: [{ file: new File(['x'], filename), name: 'upload' }],
    },
  });

  it('keeps a comma or semicolon in a filename inside one curl form file', () => {
    expect(toCurl(upload('report,final.csv'), { reveal: true })).toContain(
      `-F 'upload=@"report,final.csv"'`,
    );
    expect(toCurl(upload('a;type=text.csv'), { reveal: true })).toContain(
      `-F 'upload=@"a;type=text.csv"'`,
    );
  });

  it('escapes quotes and backslashes in a filename for curl, then for the shell', () => {
    expect(formFileArg('f', 'a"b\\c')).toBe('f=@"a\\"b\\\\c"');
    expect(toCurl(upload("it's.txt"), { reveal: true })).toContain(`-F 'upload=@"it'\\''s.txt"'`);
  });

  it('uses --head for a HEAD request, and -X for the others', () => {
    const head = toCurl({ ...request, body: undefined, method: 'HEAD' }, { reveal: true });
    expect(head).toMatch(/^curl --globoff --head 'https:/);
    expect(head).not.toContain('-X');
    expect(toCurl(request, { reveal: true })).toMatch(/^curl --globoff -X 'POST'/);
  });

  it('turns off URL globbing, so [1-2] or {a,b} in a path stays one literal request', () => {
    const globbed = { ...request, body: undefined, path: '/reports/[1-2]/{a,b}' };
    expect(toCurl(globbed, { reveal: true })).toContain("curl --globoff -X 'POST' ");
    expect(toCurl({ ...globbed, method: 'HEAD' }, { reveal: true })).toContain(
      'curl --globoff --head ',
    );
  });

  it('never lets a file named `-` be read as standard input', () => {
    expect(toCurl(upload('-'), { reveal: true })).toContain(`-F 'upload=@"./-"'`);
    const binary: ConsoleSendRequest = {
      ...request,
      body: { file: new File(['x'], '-'), kind: 'binary', mediaType: 'application/octet-stream' },
    };
    expect(toCurl(binary, { reveal: true })).toContain(`--data-binary '@./-'`);
  });
});

describe('shellQuote', () => {
  it('wraps in single quotes and escapes embedded ones', () => {
    expect(shellQuote("a'b")).toBe(`'a'\\''b'`);
  });
});
