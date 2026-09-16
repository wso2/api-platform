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

import { formatBody, validateBody } from './bodyValidation';

describe('validateBody — json', () => {
  it('accepts a parseable object', () => {
    expect(validateBody('{"a":1}', 'json')).toEqual({ state: 'valid' });
  });

  it('reports the parser’s own message, which names the position', () => {
    const result = validateBody('{"a":', 'json');

    expect(result.state).toBe('invalid');
    // Passed through untranslated on purpose: the position is the useful part,
    // and wording of our own would lose it.
    expect(result).toHaveProperty('reason', expect.stringMatching(/./));
  });

  it('treats a blank body as empty rather than invalid', () => {
    // Nothing typed yet is not a mistake, and flagging it would put an error
    // under an untouched editor.
    expect(validateBody('', 'json')).toEqual({ state: 'empty' });
    expect(validateBody('   \n  ', 'json')).toEqual({ state: 'empty' });
  });

  it('accepts a bare scalar, which is valid JSON', () => {
    expect(validateBody('42', 'json')).toEqual({ state: 'valid' });
    expect(validateBody('"text"', 'json')).toEqual({ state: 'valid' });
  });
});

describe('validateBody — xml', () => {
  it('accepts a well-formed document', () => {
    expect(validateBody('<book><title>The Hobbit</title></book>', 'xml')).toEqual({
      state: 'valid',
    });
  });

  it('rejects an unclosed tag', () => {
    expect(validateBody('<book><title>The Hobbit</book>', 'xml').state).toBe('invalid');
  });

  it('rejects mismatched nesting', () => {
    expect(validateBody('<a><b></a></b>', 'xml').state).toBe('invalid');
  });

  it('rejects a DOCTYPE declaration outright', () => {
    const result = validateBody('<!DOCTYPE foo><foo/>', 'xml');

    // Browsers do not resolve external entities in DOMParser, so this is not
    // the XXE hole it would be server-side — but a DTD in a request body is
    // far more likely a mistake than an intention, and refusing it keeps this
    // aligned with how the portal treats XML input everywhere else.
    expect(result.state).toBe('invalid');
    expect(result).toHaveProperty('reason', expect.stringContaining('DOCTYPE'));
  });

  it('rejects a lowercase doctype too', () => {
    expect(validateBody('<!doctype foo><foo/>', 'xml').state).toBe('invalid');
  });

  it('treats a blank body as empty', () => {
    expect(validateBody('  ', 'xml')).toEqual({ state: 'empty' });
  });
});

describe('validateBody — text', () => {
  it('never reports plain text as wrong, because it cannot be', () => {
    expect(validateBody('anything at all { < >', 'text')).toEqual({ state: 'unchecked' });
  });

  it('still treats a blank body as empty', () => {
    expect(validateBody('', 'text')).toEqual({ state: 'empty' });
  });
});

describe('formatBody', () => {
  it('re-indents valid JSON', () => {
    expect(formatBody('{"a":1,"b":[2]}', 'json')).toBe('{\n  "a": 1,\n  "b": [\n    2\n  ]\n}');
  });

  it('returns undefined for unparseable JSON, so the caller does nothing', () => {
    expect(formatBody('{"a":', 'json')).toBeUndefined();
  });

  it('refuses to reformat XML', () => {
    // Pretty-printing XML means inserting whitespace into text content, which
    // changes the bytes the server receives.
    expect(formatBody('<a><b/></a>', 'xml')).toBeUndefined();
  });

  it('refuses to reformat plain text', () => {
    expect(formatBody('hello', 'text')).toBeUndefined();
  });
});
