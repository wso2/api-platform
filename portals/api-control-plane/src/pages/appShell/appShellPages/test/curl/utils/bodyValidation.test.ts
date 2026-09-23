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

  it('rejects duplicate keys, which JSON.parse would silently collapse', () => {
    // Reported rather than accepted so "valid" keeps meaning the Format button
    // will work — formatting cannot represent both pairs.
    const result = validateBody('{"a":1,"a":2}', 'json');

    expect(result.state).toBe('invalid');
    expect(result).toHaveProperty('reason', expect.stringMatching(/./));
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

  it('leaves empty containers on one line, as JSON.stringify does', () => {
    expect(formatBody('{"a":{},"b":[]}', 'json')).toBe('{\n  "a": {},\n  "b": []\n}');
  });

  it('formats a bare scalar without adding structure', () => {
    expect(formatBody('42', 'json')).toBe('42');
    expect(formatBody('"text"', 'json')).toBe('"text"');
  });

  it('is unaffected by the whitespace already in the body', () => {
    expect(formatBody('{ "a" :\n\t[ 1 ] }', 'json')).toBe('{\n  "a": [\n    1\n  ]\n}');
  });

  it('keeps structural characters inside strings intact', () => {
    // The scanner has to know it is inside a string, or these would be treated
    // as syntax and rewritten.
    expect(formatBody('{"s":"a, b: {c} [d]"}', 'json')).toBe('{\n  "s": "a, b: {c} [d]"\n}');
    expect(formatBody('{"s":"quote \\" backslash \\\\"}', 'json')).toBe(
      '{\n  "s": "quote \\" backslash \\\\"\n}',
    );
  });

  // Formatting rewrites the body that gets sent, so a re-indent that changes a
  // value silently sends something the user never typed. `JSON.parse` +
  // `JSON.stringify` does exactly that to each of these.
  describe('preserves values a parse/stringify round-trip would corrupt', () => {
    it('keeps an integer wider than a double', () => {
      expect(formatBody('{"id":12345678901234567890}', 'json')).toBe(
        '{\n  "id": 12345678901234567890\n}',
      );
    });

    it('keeps negative zero', () => {
      expect(formatBody('{"z":-0}', 'json')).toBe('{\n  "z": -0\n}');
    });

    it('keeps a literal that overflows to Infinity', () => {
      // `JSON.stringify(JSON.parse('1e400'))` is `null` — the value disappears.
      expect(formatBody('{"big":1e400}', 'json')).toBe('{\n  "big": 1e400\n}');
    });

    it('keeps the written form of a number rather than its parsed form', () => {
      expect(formatBody('{"n":1.0,"e":1E+2,"f":0.10}', 'json')).toBe(
        '{\n  "n": 1.0,\n  "e": 1E+2,\n  "f": 0.10\n}',
      );
    });

    it('refuses to format duplicate keys rather than dropping one', () => {
      // `JSON.parse` would silently collapse this to `{"a":2}` and write that
      // back as the body. Declining to format leaves what the user typed.
      expect(formatBody('{"a":1,"a":2}', 'json')).toBeUndefined();
    });

    it('reorders integer-like keys, which is accepted', () => {
      // The one loss kept: JS object property order puts integer-like keys
      // first, ascending, and no parser producing plain objects avoids it.
      // Nothing about the document's *values* changes, and RFC 8259 defines an
      // object as unordered — unlike the numeric cases above, which do.
      expect(formatBody('{"2":"b","1":"a"}', 'json')).toBe('{\n  "1": "a",\n  "2": "b"\n}');
    });
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
