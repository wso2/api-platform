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

import { prettyPrintJson } from './prettyJson';

describe('prettyPrintJson', () => {
  it('indents nested JSON', () => {
    expect(prettyPrintJson('{"a":[1,2,{"b":null}],"c":{}}')).toBe(
      '{\n  "a": [\n    1,\n    2,\n    {\n      "b": null\n    }\n  ],\n  "c": {}\n}',
    );
  });

  it('keeps an integer above the safe range exactly as written', () => {
    expect(prettyPrintJson('{"id":9007199254740993}')).toBe('{\n  "id": 9007199254740993\n}');
  });

  it('does not touch braces, commas or colons inside strings', () => {
    expect(prettyPrintJson('{"s":"a, {b}: [c] \\" d"}')).toBe('{\n  "s": "a, {b}: [c] \\" d"\n}');
  });

  it('throws on invalid JSON so the caller can keep the original text', () => {
    expect(() => prettyPrintJson('{"a":')).toThrow(SyntaxError);
  });
});
