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

import { afterEach, describe, expect, it } from 'vitest';

import { latestProjectId, readStoredScope } from './landing';

const KEY = 'oxygen-ui:header-breadcrumbs';

afterEach(() => window.localStorage.clear());

describe('readStoredScope', () => {
  it('reads the organization and project the header remembered', () => {
    window.localStorage.setItem(
      KEY,
      JSON.stringify({
        version: 1,
        value: { organization: 'acme', project: 'retail' },
        updatedAt: 1,
      }),
    );
    expect(readStoredScope()).toEqual({ organization: 'acme', project: 'retail' });
  });

  it.each([
    ['nothing stored', null],
    ['not JSON', 'nope'],
    ['no project', JSON.stringify({ value: { organization: 'acme' } })],
    ['blank organization', JSON.stringify({ value: { organization: ' ', project: 'p' } })],
    ['wrong shape', JSON.stringify({ value: 'acme/p' })],
  ])('ignores %s', (_name, stored) => {
    if (stored !== null) window.localStorage.setItem(KEY, stored);
    expect(readStoredScope()).toBeNull();
  });
});

describe('latestProjectId', () => {
  it('picks the most recently changed project, falling back to its creation date', () => {
    expect(
      latestProjectId([
        { id: 'old', createdAt: '2026-01-01T00:00:00Z' },
        { id: 'edited', createdAt: '2026-01-02T00:00:00Z', updatedAt: '2026-03-01T00:00:00Z' },
        { id: 'new', createdAt: '2026-02-01T00:00:00Z' },
      ]),
    ).toBe('edited');
  });

  it('answers undefined when there are no projects', () => {
    expect(latestProjectId([])).toBeUndefined();
  });
});
