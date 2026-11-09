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

import {
  checkConditions,
  draftsFromConditions,
  fieldKeys,
  newCondition,
  newPair,
  newWindow,
  parseValue,
  type ConditionDraft,
} from './planConditions';

const requestCount = (windows: Parameters<typeof newWindow>[0][]): ConditionDraft => ({
  ...newCondition('request-count'),
  windows: windows.map((window) => newWindow(window)),
});

const custom = (id: string, pairs: [string, string][] = []): ConditionDraft => ({
  ...newCondition('custom'),
  id,
  pairs: pairs.map(([name, value]) => ({ ...newPair(name), value })),
});

const messageIds = (drafts: ConditionDraft[]) =>
  [...checkConditions(drafts).errors.values()].map((error) => error.message.id);

describe('checkConditions', () => {
  it('builds the conditions the API takes, with the fixed id of a built-in type', () => {
    const tokens = { ...newCondition('token-count'), tokenType: 'completion' as const };
    tokens.windows = [newWindow({ count: 10, timeUnit: 'DAY' })];
    expect(checkConditions([tokens]).body).toEqual([
      {
        id: 'tokens',
        params: {
          limits: [{ count: 10, timeAmount: 1, timeUnit: 'DAY' }],
          onExceed: 'reject',
          tokenType: 'completion',
        },
        type: 'token-count',
      },
    ]);
  });

  it('sends tokenType only for token-count', () => {
    const body = checkConditions([requestCount([{ count: 1 }])]).body!;
    expect(body[0].params).not.toHaveProperty('tokenType');
  });

  it('treats 60 minutes and 1 hour as the same window, but not 30 days and a month', () => {
    const same = requestCount([
      { count: 1, timeAmount: 60, timeUnit: 'MINUTE' },
      { count: 2, timeAmount: 1, timeUnit: 'HOUR' },
    ]);
    expect(checkConditions([same]).errors.has(fieldKeys.unit(same.key, same.windows[1].key))).toBe(
      true,
    );
    const apart = requestCount([
      { count: 1, timeAmount: 30, timeUnit: 'DAY' },
      { count: 2, timeAmount: 1, timeUnit: 'MONTH' },
    ]);
    expect(checkConditions([apart]).body).not.toBeNull();
  });

  it('refuses more than 20 conditions', () => {
    const drafts = Array.from({ length: 21 }, (_, index) => custom(`c-${index}`));
    expect(checkConditions(drafts).errors.has(fieldKeys.conditions())).toBe(true);
  });

  it('refuses two custom conditions with one id, and a custom id of a built-in condition', () => {
    expect(messageIds([custom('tier'), custom('tier')])).toHaveLength(1);
    expect(messageIds([custom('tool-calls')])[0]).toMatch(/customIdBuiltIn$/);
  });

  it('refuses custom values of more than 4096 bytes in all', () => {
    const big = custom('big', [['text', 'x'.repeat(4100)]]);
    expect(checkConditions([big]).errors.has(fieldKeys.params(big.key))).toBe(true);
  });

  it('skips an empty pair, and refuses a key used twice', () => {
    expect(checkConditions([custom('tier', [['', '']])]).body).toEqual([
      { id: 'tier', params: {}, type: 'custom' },
    ]);
    expect(
      messageIds([
        custom('tier', [
          ['a', '1'],
          ['a', '2'],
        ]),
      ])[0],
    ).toMatch(/pairKeyTaken$/);
  });
});

describe('parseValue', () => {
  it('types numbers, booleans and JSON, and keeps other text', () => {
    expect(parseValue('42')).toBe(42);
    expect(parseValue('true')).toBe(true);
    expect(parseValue('{"a":1}')).toEqual({ a: 1 });
    expect(parseValue('{not json')).toBe('{not json');
    expect(parseValue('UTC')).toBe('UTC');
  });
});

describe('draftsFromConditions', () => {
  it('opens a saved condition with its windows, and a custom one with its values', () => {
    const [requests, tier] = draftsFromConditions([
      {
        id: 'requests',
        params: { limits: [{ count: 5, timeAmount: 2, timeUnit: 'HOUR' }], onExceed: 'allow' },
        type: 'request-count',
      },
      { id: 'tier', params: { code: '007' }, type: 'custom' },
    ]);
    expect(
      requests.windows.map(({ count, timeAmount, timeUnit }) => ({ count, timeAmount, timeUnit })),
    ).toEqual([{ count: '5', timeAmount: '2', timeUnit: 'HOUR' }]);
    expect(requests.onExceed).toBe('allow');
    expect(checkConditions([tier]).body).toEqual([
      { id: 'tier', params: { code: '007' }, type: 'custom' },
    ]);
  });
});
