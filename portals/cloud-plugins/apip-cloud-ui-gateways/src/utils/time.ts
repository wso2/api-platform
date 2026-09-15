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

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 1000 * 60 * 60 * 24 * 365],
  ['month', 1000 * 60 * 60 * 24 * 30],
  ['day', 1000 * 60 * 60 * 24],
  ['hour', 1000 * 60 * 60],
  ['minute', 1000 * 60],
];

const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

/**
 * e.g. "2 hours ago", "just now". Falls back to that floor once a value is under
 * a minute. Renders "—" for a missing or unparseable timestamp (managed gateways
 * carry none).
 *
 * `now` is a parameter so a caller can be tested without a fake clock; leave it
 * unset everywhere else.
 */
export function relativeTime(iso: string, now: number = Date.now()): string {
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return '—';
  const diffMs = parsed - now;
  for (const [unit, ms] of UNITS) {
    if (Math.abs(diffMs) >= ms) {
      return relative.format(Math.round(diffMs / ms), unit);
    }
  }
  return 'just now';
}

const absolute = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
});

/**
 * The same instant spelled out in the viewer's own locale and zone, for the
 * hover behind a relative time — "5 minutes ago" is the readable form and the
 * exact moment is what someone correlating with a deployment actually needs.
 *
 * Empty for a missing or unparseable timestamp, so a caller can use it directly
 * as a tooltip title (an empty title renders no tooltip).
 */
export function absoluteTime(iso: string): string {
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return '';
  return absolute.format(parsed);
}
