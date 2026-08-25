/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it } from 'vitest';

import {
  mergeLogLines,
  parseGatewayTrafficLog,
  toConsoleLine,
  toConsoleLines,
} from './logLines';

describe('parseGatewayTrafficLog', () => {
  it('parses a structured API gateway traffic event', () => {
    expect(
      parseGatewayTrafficLog({
        log: JSON.stringify({
          status: 200,
          operation: { method: 'GET', path: '/orders/{id}' },
        }),
      })
    ).toMatchObject({
      status: 200,
      operation: { method: 'GET', path: '/orders/{id}' },
    });
  });

  it('returns undefined for an unstructured log line', () => {
    expect(parseGatewayTrafficLog({ log: 'plain text' })).toBeUndefined();
  });

  it('parses a policy-engine-prefixed gateway traffic event', () => {
    expect(
      parseGatewayTrafficLog({
        log: '[pol] {"status":201,"operation":{"method":"POST"}}',
      })
    ).toMatchObject({ status: 201, operation: { method: 'POST' } });
  });
});

describe('toConsoleLine', () => {
  it('summarises a traffic event and keeps the raw payload', () => {
    const log = JSON.stringify({
      timestamp: '2026-08-22T10:00:00.500Z',
      correlationId: 'abc-1',
      status: 200,
      operation: { method: 'GET', path: '/orders' },
    });

    const line = toConsoleLine({ level: 'info', log });

    expect(line.level).toBe('INFO');
    expect(line.message).toBe('GET /orders -> 200 [abc-1]');
    expect(line.raw).toBe(log);
    expect(line.timestamp).toBe('2026-08-22T10:00:00.500Z');
  });

  it('falls back to the envelope timestamp and level field', () => {
    const line = toConsoleLine({
      logLevel: 'error',
      log: 'upstream timeout',
      timestamp: '2026-08-22T10:00:01Z',
    });

    expect(line.level).toBe('ERROR');
    expect(line.message).toBe('upstream timeout');
    // A plain line is already its own raw form, so nothing extra is kept.
    expect(line.raw).toBeUndefined();
    expect(line.timestamp).toBe('2026-08-22T10:00:01Z');
  });

  it('defaults the level when the record carries none', () => {
    expect(toConsoleLine({ log: 'hello' }).level).toBe('INFO');
  });
});

describe('toConsoleLines', () => {
  it('gives byte-identical lines in one page distinct ids', () => {
    const entry = { level: 'INFO', log: 'retry', timestamp: '2026-08-22T10:00:00Z' };

    const ids = toConsoleLines([entry, entry, entry]).map((line) => line.id);

    expect(new Set(ids).size).toBe(3);
  });
});

const entry = (seconds: number, message: string) => ({
  level: 'INFO',
  log: message,
  timestamp: `2026-08-22T10:00:${String(seconds).padStart(2, '0')}Z`,
});

describe('mergeLogLines', () => {
  it('appends a newest-first page in ascending order', () => {
    const merged = mergeLogLines(
      [],
      [entry(3, 'third'), entry(2, 'second'), entry(1, 'first')],
      100
    );

    expect(merged.map((line) => line.message)).toEqual([
      'first',
      'second',
      'third',
    ]);
  });

  it('keeps existing rows and appends only new records', () => {
    const first = mergeLogLines([], [entry(1, 'first')], 100);
    const second = mergeLogLines(first, [entry(2, 'second'), entry(1, 'first')], 100);

    expect(second).toHaveLength(2);
    expect(second[0]).toBe(first[0]);
    expect(second[1].message).toBe('second');
  });

  it('returns the same array when a poll brings nothing new', () => {
    const first = mergeLogLines([], [entry(1, 'first')], 100);

    expect(mergeLogLines(first, [entry(1, 'first')], 100)).toBe(first);
  });

  it('trims the oldest rows past the buffer limit', () => {
    const merged = mergeLogLines(
      [],
      [entry(1, 'first'), entry(2, 'second'), entry(3, 'third')],
      2
    );

    expect(merged.map((line) => line.message)).toEqual(['second', 'third']);
  });

  it('ignores records at or before the clear watermark', () => {
    const since = Date.parse('2026-08-22T10:00:02Z');

    const merged = mergeLogLines(
      [],
      [entry(1, 'before'), entry(2, 'at'), entry(3, 'after')],
      100,
      since
    );

    expect(merged.map((line) => line.message)).toEqual(['after']);
  });

  it('keeps records without a usable timestamp rather than hiding them', () => {
    const merged = mergeLogLines(
      [],
      [{ level: 'WARN', log: 'no timestamp' }],
      100,
      Date.parse('2026-08-22T10:00:02Z')
    );

    expect(merged.map((line) => line.message)).toEqual(['no timestamp']);
  });
});
