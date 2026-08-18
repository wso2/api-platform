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

import { useMemo } from 'react';
import { useIntl } from 'react-intl';

/** Anything a timestamp may be: API string, epoch, or `Date`. */
export type DateLike = string | number | Date | null | undefined;

export type Formatters = {
  /** "Aug 17, 2026" */
  shortDate: (value: DateLike) => string;
  /** "Aug 17, 2026, 5:45 PM" */
  dateTime: (value: DateLike) => string;
  /** "3 hours ago", "in 2 days" */
  relativeTime: (value: DateLike) => string;
};

/**
 * Largest unit first would read oddly ("0 years ago"), so this walks upward:
 * each entry says how many of the current unit fit in the next one.
 */
const RELATIVE_DIVISIONS: {
  amount: number;
  unit: Intl.RelativeTimeFormatUnit;
}[] = [
  { amount: 60, unit: 'second' },
  { amount: 60, unit: 'minute' },
  { amount: 24, unit: 'hour' },
  { amount: 7, unit: 'day' },
  { amount: 4.34524, unit: 'week' },
  { amount: 12, unit: 'month' },
  { amount: Number.POSITIVE_INFINITY, unit: 'year' },
];

/** Return '' for missing/unparseable dates; callers provide fallbacks. */
function toDate(value: DateLike): Date | null {
  if (value === undefined || value === null || value === '') return null;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Picks the coarsest relative-time unit for `date` vs `now`. */
export function selectRelativeTimeParts(
  date: Date,
  now: number
): { value: number; unit: Intl.RelativeTimeFormatUnit } | null {
  let duration = (date.getTime() - now) / 1000;
  for (const division of RELATIVE_DIVISIONS) {
    if (Math.abs(duration) < division.amount) {
      return { value: Math.round(duration), unit: division.unit };
    }
    duration /= division.amount;
  }
  return null;
}

/**
 * Locale-aware date, time, and relative-time formatting.
 * Memoized on `intl` for stable references between renders.
 */
export function useFormatters(): Formatters {
  const intl = useIntl();

  return useMemo<Formatters>(
    () => ({
      shortDate: (value) => {
        const date = toDate(value);
        return date ? intl.formatDate(date, { format: 'short' }) : '';
      },
      dateTime: (value) => {
        const date = toDate(value);
        return date ? intl.formatDate(date, { format: 'withTime' }) : '';
      },
      relativeTime: (value) => {
        const date = toDate(value);
        if (!date) return '';
        const parts = selectRelativeTimeParts(date, Date.now());
        // `numeric: 'auto'` yields "yesterday" over "1 day ago" for single-unit
        // distances; anything larger reads the same either way.
        return parts
          ? intl.formatRelativeTime(parts.value, parts.unit, {
              numeric: 'auto',
            })
          : '';
      },
    }),
    [intl]
  );
}
