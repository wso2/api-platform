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

import { parseConsoleSpec } from '../spec/parseSpec';
import { paramKey } from './buildRequest';
import { initialDraft } from './draft';

const operationWith = (parameters: unknown[]) =>
  parseConsoleSpec({ paths: { '/x': { get: { parameters, responses: {} } } } }).operations[0];

describe('initialDraft — choices', () => {
  it('starts a required enum on its first value, and leaves an optional one unset', () => {
    const operation = operationWith([
      {
        in: 'query',
        name: 'must',
        required: true,
        schema: { enum: ['active', 'archived'], type: 'string' },
      },
      { in: 'query', name: 'may', schema: { enum: ['active', 'archived'], type: 'string' } },
    ]);
    const draft = initialDraft(operation);
    const [must, may] = operation.parameters;

    expect(draft.params[paramKey(must)]).toBe('active');
    expect(draft.params[paramKey(may)]).toBeUndefined();
  });

  it('keeps the spec’s own example for a required enum', () => {
    const operation = operationWith([
      {
        in: 'query',
        name: 'must',
        required: true,
        schema: { default: 'archived', enum: ['active', 'archived'], type: 'string' },
      },
    ]);
    expect(initialDraft(operation).params[paramKey(operation.parameters[0])]).toBe('archived');
  });
});
