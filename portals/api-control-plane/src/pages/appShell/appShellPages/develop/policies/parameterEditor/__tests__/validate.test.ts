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

import { createPolicyForm, type PolicyDefinition } from '../engine';
import { isTemplateExpression } from '../schemaUtils';

const definition: PolicyDefinition = {
  name: 'constraints',
  parameters: {
    properties: {
      count: { maximum: 10, minimum: 2, multipleOf: 2, type: 'integer' },
      kind: { const: 'fixed', type: 'string' },
      name: { maxLength: 5, minLength: 3, pattern: '^[a-z]+$', type: 'string' },
      ratio: { exclusiveMaximum: 1, exclusiveMinimum: 0, type: 'number' },
      site: { format: 'uri', type: 'string' },
      tags: {
        items: { type: 'string' },
        maxItems: 3,
        minItems: 2,
        type: 'array',
        uniqueItems: true,
      },
    },
    type: 'object',
  },
  version: '1.0',
};

/** The messages the form reports for a config. */
const errorsFor = (initialConfig: Record<string, unknown>): string[] =>
  createPolicyForm(definition, { initialConfig })
    .getResolution()
    .errors.map((error) => error.message);

describe('policy form validation', () => {
  it('accepts values inside every constraint', () => {
    expect(
      errorsFor({
        count: 4,
        name: 'abc',
        ratio: 0.5,
        site: 'https://example.com/path',
        tags: ['a', 'b'],
      }),
    ).toEqual([]);
  });

  it('checks string length, pattern and format', () => {
    expect(errorsFor({ name: 'ab' })).toContain('Use at least 3 characters');
    expect(errorsFor({ name: 'abcdef' })).toContain('Use at most 5 characters');
    expect(errorsFor({ name: 'AB1' }).join(' ')).toMatch(/expected format/);
    expect(errorsFor({ site: 'not a url' })).toContain(
      'Enter a full URL, e.g. https://example.com/path',
    );
  });

  it('lets a template expression stand in for a pattern or URL', () => {
    expect(errorsFor({ name: '{{ env "NAME" }}', site: '{{ env "SITE" }}' })).toEqual([]);
  });

  it('checks numeric bounds, exclusive bounds and multiples', () => {
    expect(errorsFor({ count: 1 })).toContain('Must be 2 or more');
    expect(errorsFor({ count: 12 })).toContain('Must be 10 or less');
    expect(errorsFor({ count: 5 })).toContain('Must be a multiple of 2');
    expect(errorsFor({ ratio: 0 })).toContain('Must be more than 0');
    expect(errorsFor({ ratio: 1 })).toContain('Must be less than 1');
  });

  it('checks list size and duplicates', () => {
    expect(errorsFor({ tags: ['a'] })).toContain('Add at least 2 items');
    expect(errorsFor({ tags: ['a', 'b', 'c', 'd'] })).toContain('Use at most 3 items');
    expect(errorsFor({ tags: ['a', 'a'] })).toContain('Duplicate item');
  });

  it('holds a constant to its value', () => {
    expect(errorsFor({ kind: 'other' })).toContain('Must be fixed');
  });
});

describe('isTemplateExpression', () => {
  it('recognises a {{ ... }} expression anywhere in a string', () => {
    expect(isTemplateExpression('{{ env "A" }}')).toBe(true);
    expect(isTemplateExpression('prefix-{{ secret "s" }}-suffix')).toBe(true);
  });

  it('rejects plain text and non-strings', () => {
    expect(isTemplateExpression('plain')).toBe(false);
    expect(isTemplateExpression('{{}}')).toBe(false);
    expect(isTemplateExpression(42)).toBe(false);
  });
});
