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

import { parseDurationSeconds } from './duration';

describe('parseDurationSeconds', () => {
  it('reads the bounds the platform declares for the one duration field', () => {
    // `…ratelimit_v1.memory.cleanup_interval` is bounded 30s – 1h.
    expect(parseDurationSeconds('30s')).toBe(30);
    expect(parseDurationSeconds('1h')).toBe(3600);
    expect(parseDurationSeconds('5m')).toBe(300);
  });

  it('sums a multi-unit duration', () => {
    expect(parseDurationSeconds('1h30m')).toBe(5400);
    expect(parseDurationSeconds('1m30s')).toBe(90);
    expect(parseDurationSeconds('1h0m0s')).toBe(3600);
  });

  it('accepts fractions and sub-second units', () => {
    expect(parseDurationSeconds('1.5h')).toBe(5400);
    expect(parseDurationSeconds('500ms')).toBeCloseTo(0.5);
    expect(parseDurationSeconds('1us')).toBeCloseTo(1e-6);
    expect(parseDurationSeconds('1µs')).toBeCloseTo(1e-6);
    expect(parseDurationSeconds('100ns')).toBeCloseTo(1e-7);
  });

  it('reads ms as milliseconds, not as minutes plus a stray s', () => {
    // The unit alternation is longest-first for this; read the other way
    // "500ms" would be 500 minutes and pass a 1h ceiling check backwards.
    expect(parseDurationSeconds('500ms')).not.toBe(500 * 60);
  });

  it('accepts a bare zero and a sign', () => {
    expect(parseDurationSeconds('0')).toBe(0);
    expect(parseDurationSeconds('-5m')).toBe(-300);
    expect(parseDurationSeconds('+5m')).toBe(300);
  });

  it('makes the two spellings of the same duration compare equal', () => {
    // The server stores a duration exactly as spelled — "5m" is never
    // re-spelled "5m0s" — so equality has to come from the parse, not the text.
    expect(parseDurationSeconds('5m')).toBe(parseDurationSeconds('5m0s'));
    expect(parseDurationSeconds('60s')).toBe(parseDurationSeconds('1m'));
  });

  it('rejects anything that is not a Go duration', () => {
    expect(parseDurationSeconds('')).toBeNull();
    expect(parseDurationSeconds('soon')).toBeNull();
    // Unitless, and not the one spelling Go allows unitless.
    expect(parseDurationSeconds('5')).toBeNull();
    expect(parseDurationSeconds('5x')).toBeNull();
    // Trailing junk: the whole string has to be consumed.
    expect(parseDurationSeconds('5m30')).toBeNull();
    expect(parseDurationSeconds('5m junk')).toBeNull();
  });

  it('rejects surrounding whitespace, which Go rejects too', () => {
    // The drawer sends the text verbatim, so accepting padding here would turn
    // a catchable typo into a 400 from the platform.
    expect(parseDurationSeconds(' 5m')).toBeNull();
    expect(parseDurationSeconds('5m ')).toBeNull();
    expect(parseDurationSeconds(' 5m ')).toBeNull();
    expect(parseDurationSeconds('5 m')).toBeNull();
    expect(parseDurationSeconds('5m')).toBe(300);
  });
});
