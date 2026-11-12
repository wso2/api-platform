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

import { decodeBody, requestBodySize } from './response';

const bytes = (text: string) => new TextEncoder().encode(text);

describe('decodeBody', () => {
  it('treats a vendor JSON type as text, not binary', () => {
    expect(decodeBody(bytes('{"a":1}'), 'application/vnd.api+json')).toEqual({
      binary: false,
      text: '{"a":1}',
    });
  });

  it('still treats other vendor types as binary', () => {
    expect(decodeBody(bytes('x'), 'application/vnd.ms-excel').binary).toBe(true);
  });

  it('does not call a cut-off body binary because its last character is incomplete', () => {
    const cut = bytes('héllo').slice(0, 2); // 'h' and half of 'é'
    expect(decodeBody(cut, 'application/x-unknown', true).binary).toBe(false);
    expect(decodeBody(cut, 'application/x-unknown', false).binary).toBe(true);
  });
});

describe('requestBodySize', () => {
  it('counts UTF-8 bytes, not characters', () => {
    expect(requestBodySize({ kind: 'text', mediaType: 'text/plain', text: '€€' })).toBe(6);
  });

  it('counts a form as its encoded string', () => {
    expect(requestBodySize({ fields: [['a', 'b c']], kind: 'form' })).toBe('a=b+c'.length);
  });

  it('adds framing for multipart parts', () => {
    expect(
      requestBodySize({ kind: 'multipart', parts: [{ name: 'a', value: 'b' }] }),
    ).toBeGreaterThan(2);
  });
});
