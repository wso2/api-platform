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
 * Go duration parsing (`time.ParseDuration`), enough to compare a typed value
 * against the declared `min`/`max`, which are duration strings themselves.
 *
 * Like `quantity.ts` this does not normalise: the server stores a duration
 * exactly as it was spelled — "5m" stays "5m", not "5m0s" — precisely so an
 * unchanged save does not look like a change and roll the gateway pods. Never
 * write the parsed value back.
 */

/** Longest unit first, so `ms` is never read as `m` followed by a stray `s`. */
const UNIT_SECONDS = new Map<string, number>([
  ['ns', 1e-9],
  ['us', 1e-6],
  ['µs', 1e-6],
  ['μs', 1e-6],
  ['ms', 1e-3],
  ['s', 1],
  ['m', 60],
  ['h', 3600],
]);

/** Sticky, so the loop below can require that the WHOLE string was consumed. */
const TOKEN = /(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h)/y;

/** The duration in seconds, or `null` when the text is not a Go duration. */
export function parseDurationSeconds(text: string): number | null {
  // NOT trimmed. The drawer sends the user's text verbatim, so anything this
  // accepts must be something Go's time.ParseDuration accepts -- and it takes
  // no surrounding whitespace. Trimming here made " 5m " pass in the browser
  // and 400 on the server, which is the one outcome this parser exists to
  // prevent.
  let rest = text;
  if (!rest) return null;

  let sign = 1;
  if (rest.startsWith('-')) {
    sign = -1;
    rest = rest.slice(1);
  } else if (rest.startsWith('+')) {
    rest = rest.slice(1);
  }

  // The one spelling with no unit. Go accepts it; nothing else unitless.
  if (rest === '0') return 0;

  TOKEN.lastIndex = 0;
  let total = 0;
  for (let match = TOKEN.exec(rest); match; match = TOKEN.exec(rest)) {
    const seconds = UNIT_SECONDS.get(match[2]);
    if (seconds === undefined) return null;
    total += Number(match[1]) * seconds;
    if (TOKEN.lastIndex === rest.length) return sign * total;
  }
  // Ran out of matches before the end: there is trailing junk, or a unit this
  // does not know. Either way it is not a duration.
  return null;
}
