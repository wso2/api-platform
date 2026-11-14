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

import { encodePathValue, headerProblem, isSafePath, resolveTarget } from './safety';

describe('isSafePath', () => {
  it('accepts an ordinary path template', () => {
    expect(isSafePath('/books/{id}')).toBe(true);
    expect(isSafePath('/books/a%2Fb')).toBe(true);
  });

  it.each([
    ['protocol-relative', '//evil.example.com/x'],
    ['absolute', 'https://evil.example.com/x'],
    ['no leading slash', 'books'],
    ['dot segment', '/a/../b'],
    ['encoded dot segment', '/a/%2e%2E/b'],
    ['single dot', '/a/./b'],
    ['backslash', '/a\\b'],
    ['query smuggled in', '/a?x=1'],
    ['fragment smuggled in', '/a#x'],
    ['control character', '/a\u0000b'],
    ['newline', '/a\nb'],
  ])('rejects %s', (_label, path) => {
    expect(isSafePath(path)).toBe(false);
  });
});

describe('resolveTarget', () => {
  it('joins the gateway URL and the path', () => {
    expect(resolveTarget('https://gw.example.com/pizza/v1/', '/order')?.toString()).toBe(
      'https://gw.example.com/pizza/v1/order',
    );
  });

  it('refuses a non-http scheme', () => {
    expect(resolveTarget('file:///etc', '/passwd')).toBeUndefined();
    expect(resolveTarget('javascript:alert(1)', '/x')).toBeUndefined();
  });

  it('refuses a gateway URL that carries credentials', () => {
    expect(resolveTarget('https://user:pw@gw.example.com', '/x')).toBeUndefined();
  });

  it('refuses an unparseable gateway URL', () => {
    expect(resolveTarget('not a url', '/x')).toBeUndefined();
  });

  it('stays on the gateway whatever the path says', () => {
    expect(resolveTarget('https://gw.example.com/p', '//evil.example.com')).toBeUndefined();
    expect(resolveTarget('https://gw.example.com/p', '/a/../../x')).toBeUndefined();
  });
});

describe('encodePathValue', () => {
  it('turns a value into a single path segment', () => {
    expect(encodePathValue('a/b')).toBe('a%2Fb');
    expect(encodePathValue('../admin')).toBe('..%2Fadmin');
    expect(encodePathValue("it's (1)*")).toBe('it%27s%20%281%29%2A');
  });
});

describe('headerProblem', () => {
  it('allows an ordinary header', () => {
    expect(headerProblem('X-Request-Id', 'abc')).toBeUndefined();
    expect(headerProblem('Authorization', 'Bearer x')).toBeUndefined();
  });

  it.each([
    'Host',
    'cookie',
    'Content-Length',
    'Transfer-Encoding',
    'Origin',
    'X-Forwarded-For',
    'Proxy-Authorization',
    'Sec-Fetch-Mode',
    'X-HTTP-Method-Override',
  ])('refuses %s, which the browser or gateway owns', (name) => {
    expect(headerProblem(name, 'x')).toBe('forbidden');
  });

  it('refuses a name that is not a token', () => {
    expect(headerProblem('Bad Name', 'x')).toBe('name');
    expect(headerProblem('X:Y', 'x')).toBe('name');
    expect(headerProblem('', 'x')).toBe('name');
  });

  it('refuses a value that could start a second header', () => {
    expect(headerProblem('X-A', 'a\r\nX-B: b')).toBe('value');
    expect(headerProblem('X-A', 'a\nb')).toBe('value');
    expect(headerProblem('X-A', 'a\0b')).toBe('value');
  });
});

describe('isSafePath with encoded slashes', () => {
  it('rejects a dot segment hidden behind %2F', () => {
    expect(isSafePath('/a%2F..%2Fb')).toBe(false);
    expect(isSafePath('/a%2f%2e%2e')).toBe(false);
  });

  it('still allows an encoded slash inside a normal segment', () => {
    expect(isSafePath('/a%2Fb')).toBe(true);
  });
});
