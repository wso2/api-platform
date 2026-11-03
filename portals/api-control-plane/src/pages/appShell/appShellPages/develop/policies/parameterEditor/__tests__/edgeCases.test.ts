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

const definition = (properties: Record<string, unknown>): PolicyDefinition => ({
  name: 'edge-cases',
  parameters: { properties, type: 'object' },
  version: '1.0',
});

describe('json fields', () => {
  const form = (initialConfig: Record<string, unknown>) =>
    createPolicyForm(definition({ payload: {} }), { initialConfig });

  it.each([
    ['a string', 'hello'],
    ['a number', 42],
    ['a list', [1, 'two']],
    ['an object', { nested: true }],
  ])('loads %s and submits it unchanged', (_label, saved) => {
    const resolution = form({ payload: saved }).getResolution();

    expect(resolution.errors).toEqual([]);
    expect(resolution.config.payload).toEqual(saved);
  });
});

describe('map fields', () => {
  const labels = definition({
    labels: { additionalProperties: { type: 'string' }, type: 'object' },
  });

  it('keeps a `__proto__` entry as an own property instead of dropping it', () => {
    const saved = JSON.parse('{"__proto__":"x","a":"y"}');
    const { config } = createPolicyForm(labels, {
      initialConfig: { labels: saved },
    }).getResolution();

    const resolved = config.labels as Record<string, unknown>;
    expect(Object.keys(resolved).sort()).toEqual(['__proto__', 'a']);
    expect(Object.getOwnPropertyDescriptor(resolved, '__proto__')?.value).toBe('x');
    expect(Object.getPrototypeOf(resolved)).toBe(Object.prototype);
  });
});

describe('error placement', () => {
  it('attaches an error to the entry it belongs to when two entries share a dotted name', () => {
    const nested = definition({
      m: {
        additionalProperties: {
          properties: { b: { minLength: 1, type: 'string' } },
          required: ['b'],
          type: 'object',
        },
        type: 'object',
      },
    });
    const resolution = createPolicyForm(nested, {
      initialConfig: { m: { a: { b: 'fine' }, 'a.b': { b: '' } } },
    }).getResolution();

    const entries = resolution.view.children!.find((node) => node.field.key === 'm')!.entries!;
    const owner = (index: number) => entries[index].node.pathKey;
    const owners = resolution.errors.map((error) => error.pathKey);

    expect(resolution.errors.length).toBeGreaterThan(0);
    expect(owners.some((key) => key.startsWith(owner(1)))).toBe(true);
    expect(owners.every((key) => !key.startsWith(owner(0)))).toBe(true);
  });
});
