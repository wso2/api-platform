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

import { parseQuantity } from './quantity';

describe('parseQuantity', () => {
  it('reads the bounds the platform actually declares', () => {
    // Every one of these is a `min` or `max` from the deployed allowlist, and
    // all of them arrive as strings — `Number('50m')` is NaN, which is the
    // whole reason this function exists.
    expect(parseQuantity('50m')).toBeCloseTo(0.05);
    expect(parseQuantity('4')).toBe(4);
    expect(parseQuantity('128Mi')).toBe(134217728);
    expect(parseQuantity('8Gi')).toBe(8589934592);
  });

  it('makes the spellings the server canonicalizes compare equal', () => {
    // "1000m" comes back "1" and "1024Mi" comes back "1Gi" — a client that
    // compared strings would report a change that is not one.
    expect(parseQuantity('1000m')).toBe(parseQuantity('1'));
    expect(parseQuantity('1024Mi')).toBe(parseQuantity('1Gi'));
    expect(parseQuantity('500m')).toBeCloseTo(0.5);
  });

  it('tells the milli and mega suffixes apart', () => {
    expect(parseQuantity('1m')).toBeCloseTo(1e-3);
    expect(parseQuantity('1M')).toBe(1e6);
    expect(parseQuantity('1Mi')).toBe(1048576);
  });

  it('treats an exponent as part of the number and a bare E as exa', () => {
    expect(parseQuantity('1e3')).toBe(1000);
    expect(parseQuantity('1E3')).toBe(1000);
    expect(parseQuantity('1E')).toBe(1e18);
    expect(parseQuantity('1Ei')).toBe(2 ** 60);
  });

  it('accepts decimals and surrounding whitespace', () => {
    expect(parseQuantity('1.5Gi')).toBe(1610612736);
    expect(parseQuantity('  256Mi  ')).toBe(268435456);
    expect(parseQuantity('.5')).toBeCloseTo(0.5);
  });

  it('rejects anything that is not a quantity', () => {
    expect(parseQuantity('')).toBeNull();
    expect(parseQuantity('abc')).toBeNull();
    expect(parseQuantity('1x')).toBeNull();
    expect(parseQuantity('1e')).toBeNull();
    expect(parseQuantity('1 Mi')).toBeNull();
    // Kubernetes allows an exponent OR a suffix, never both — and the server
    // rejects it, so accepting it here would only cost a round trip.
    expect(parseQuantity('1e3Mi')).toBeNull();
  });

  it('does not resolve a suffix through the prototype chain', () => {
    // The suffix comes from user input and the lookup tables are Maps for
    // exactly this reason: on a plain object `'constructor' in table` is true,
    // and the multiplier would come back as a function.
    expect(parseQuantity('1constructor')).toBeNull();
    expect(parseQuantity('1toString')).toBeNull();
  });
});
