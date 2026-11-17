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

import { apiCreationEntry, readApiCreationEntryPoint, withoutApiCreationEntry } from './entryPoint';

describe('API creation entry point', () => {
  it('round-trips through router state', () => {
    for (const entryPoint of [
      'api-list',
      'api-list-empty-state',
      'org-home',
      'project-quick-actions',
    ] as const) {
      expect(readApiCreationEntryPoint(apiCreationEntry(entryPoint).state)).toBe(entryPoint);
    }
  });

  it.each([
    ['no state (a typed URL or a reload)', null],
    ['state from another page', { from: '/somewhere' }],
    ['an unknown entry point', { apiCreationEntryPoint: 'a-new-button' }],
    ['a non-string value', { apiCreationEntryPoint: 42 }],
  ])('reads %s as a direct entry', (_name, state) => {
    expect(readApiCreationEntryPoint(state)).toBe('direct');
  });

  it('drops the entry point once read, so a reload of the wizard reads as direct', () => {
    const state = apiCreationEntry('api-list').state;

    expect(withoutApiCreationEntry(state)).toBeNull();
    expect(readApiCreationEntryPoint(withoutApiCreationEntry(state))).toBe('direct');
  });

  it('keeps any other router state, and leaves state without an entry point as it is', () => {
    expect(withoutApiCreationEntry({ ...apiCreationEntry('org-home').state, from: '/x' })).toEqual({
      from: '/x',
    });
    const other = { from: '/somewhere' };
    expect(withoutApiCreationEntry(other)).toBe(other);
    expect(withoutApiCreationEntry(null)).toBeNull();
  });
});
