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

import { readStoredDefinition, reformatDefinition } from './storedDefinition';

const COMPACT_JSON = '{"openapi":"3.0.3","info":{"title":"Loans"}}';
const PRETTY_JSON = '{\n  "openapi": "3.0.3",\n  "info": {\n    "title": "Loans"\n  }\n}';
const YAML = 'openapi: 3.0.3\ninfo:\n  title: Loans\n';

describe('readStoredDefinition', () => {
  it('reads a YAML content type as YAML, keeping the text exactly as stored', () => {
    expect(readStoredDefinition(YAML, 'application/yaml')).toEqual({ format: 'yaml', text: YAML });
  });

  it('reads a JSON content type as JSON and pretty-prints it', () => {
    expect(readStoredDefinition(COMPACT_JSON, 'application/json')).toEqual({
      format: 'json',
      text: PRETTY_JSON,
    });
  });

  it('tells JSON from YAML by the text when there is no content type', () => {
    expect(readStoredDefinition(COMPACT_JSON)?.format).toBe('json');
    expect(readStoredDefinition(YAML)?.format).toBe('yaml');
  });

  it('gives nothing to pre-fill from when the text is not an object', () => {
    expect(readStoredDefinition('{ "openapi": ')).toBeUndefined();
    expect(readStoredDefinition('[1, 2]')).toBeUndefined();
  });
});

describe('reformatDefinition', () => {
  it('returns the text untouched when it is already in the format', () => {
    expect(reformatDefinition({ format: 'yaml', text: YAML }, 'yaml')).toBe(YAML);
  });

  it('re-prints the definition in the other format, both ways', () => {
    expect(reformatDefinition({ format: 'json', text: PRETTY_JSON }, 'yaml')).toBe(YAML);
    expect(reformatDefinition({ format: 'yaml', text: YAML }, 'json')).toBe(PRETTY_JSON);
  });

  it('leaves text that cannot be read as it is', () => {
    const broken = '{ "openapi": ';
    expect(reformatDefinition({ format: 'json', text: broken }, 'yaml')).toBe(broken);
  });
});
