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

/**
 * Kubernetes quantity parsing, enough to compare a typed value against the
 * `min`/`max` the platform declares — both of which arrive as quantity strings
 * too, so this is the only way to check a bound at all.
 *
 * It deliberately does NOT canonicalize. The server does that on write
 * ("1000m" comes back "1", "1024Mi" comes back "1Gi"), and a client that
 * re-spelled the value locally would either fight the response or make an
 * unchanged save look like a change. Re-render a quantity field from the PUT
 * response instead.
 */

/** Binary suffixes. A `Map`, so a suffix like `constructor` cannot resolve through the prototype. */
const BINARY = new Map<string, number>([
  ['Ki', 2 ** 10],
  ['Mi', 2 ** 20],
  ['Gi', 2 ** 30],
  ['Ti', 2 ** 40],
  ['Pi', 2 ** 50],
  ['Ei', 2 ** 60],
]);

/** Decimal SI suffixes, including the empty one. Case matters: `m` is milli, `M` is mega. */
const DECIMAL = new Map<string, number>([
  ['n', 1e-9],
  ['u', 1e-6],
  ['m', 1e-3],
  ['', 1],
  ['k', 1e3],
  ['M', 1e6],
  ['G', 1e9],
  ['T', 1e12],
  ['P', 1e15],
  ['E', 1e18],
]);

/**
 * Either a number with a suffix, or a number in exponent form with none —
 * Kubernetes allows both but not together, so `1e3` is a thousand, `1E` is one
 * exa, and `1e3Mi` is not a quantity at all.
 */
const QUANTITY = /^([+-]?(?:\d+|\d*\.\d+))([A-Za-z]*)$|^([+-]?(?:\d+|\d*\.\d+)[eE][+-]?\d+)$/;

/** The quantity as a plain number, or `null` when the text is not one. */
export function parseQuantity(text: string): number | null {
  const match = QUANTITY.exec(text.trim());
  if (!match) return null;
  const value = Number(match[1] ?? match[3]);
  if (!Number.isFinite(value)) return null;
  const suffix = match[1] === undefined ? '' : match[2];
  const multiplier = BINARY.get(suffix) ?? DECIMAL.get(suffix);
  if (multiplier === undefined) return null;
  // The value was finite; the product need not be. 308 digits with an `E`
  // suffix overflows to Infinity, which compares greater than any `max` but
  // would slip past a field that declares none.
  const scaled = value * multiplier;
  return Number.isFinite(scaled) ? scaled : null;
}
