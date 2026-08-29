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

import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { IntlProvider } from 'react-intl';
import { describe, expect, it } from 'vitest';

import { DISPLAY_TIME_ZONE, INTL_FORMATS } from './formats';
import { selectRelativeTimeParts, useFormatters } from './useFormatters';

function renderFormatters(locale = 'en') {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <IntlProvider
      locale={locale}
      defaultLocale="en"
      messages={{}}
      formats={INTL_FORMATS}
      timeZone={DISPLAY_TIME_ZONE}
    >
      {children}
    </IntlProvider>
  );
  return renderHook(() => useFormatters(), { wrapper }).result;
}

// Avoids stale browser-locale formatters after a locale switch; use the provider's locale.
describe('useFormatters', () => {
  const instant = new Date('2026-08-17T17:45:00Z');

  it('follows the provider locale, not the browser locale', () => {
    const en = renderFormatters('en').current;
    const de = renderFormatters('de').current;

    // Month name and ordering both differ; the point is only that they differ.
    expect(en.shortDate(instant)).not.toBe(de.shortDate(instant));
    expect(en.relativeTime(new Date(Date.now() - 3 * 3_600_000))).toBe(
      '3 hours ago'
    );
    expect(de.relativeTime(new Date(Date.now() - 3 * 3_600_000))).toBe(
      'vor 3 Stunden'
    );
  });

  it('uses the shared named formats — month name, never an ambiguous 8/17', () => {
    // dateStyle: 'medium': a numeric date would reverse day/month per locale.
    expect(renderFormatters().current.shortDate(instant)).toMatch(/Aug/);
  });

  it('renders a date with a time for dateTime, and without for shortDate', () => {
    const { shortDate, dateTime } = renderFormatters().current;

    expect(dateTime(instant)).toMatch(/Aug.*\d{1,2}:\d{2}/);
    expect(shortDate(instant)).not.toMatch(/\d{1,2}:\d{2}/);
  });

  it('returns "" for missing or unparseable values, leaving the placeholder to the caller', () => {
    const { shortDate, dateTime, relativeTime } = renderFormatters().current;

    for (const format of [shortDate, dateTime, relativeTime]) {
      expect(format(undefined)).toBe('');
      expect(format(null)).toBe('');
      expect(format('')).toBe('');
      expect(format('not-a-date')).toBe('');
    }
  });

  it('reads "yesterday" rather than "1 day ago"', () => {
    const { relativeTime } = renderFormatters().current;

    expect(relativeTime(new Date(Date.now() - 24 * 3_600_000))).toBe(
      'yesterday'
    );
  });
});

describe('selectRelativeTimeParts', () => {
  const now = Date.UTC(2026, 7, 17, 12, 0, 0);
  const at = (ms: number) => new Date(now + ms);

  it('picks the coarsest unit that still describes the distance', () => {
    expect(selectRelativeTimeParts(at(-30_000), now)).toEqual({
      value: -30,
      unit: 'second',
    });
    expect(selectRelativeTimeParts(at(-3 * 3_600_000), now)).toEqual({
      value: -3,
      unit: 'hour',
    });
    expect(selectRelativeTimeParts(at(-2 * 86_400_000), now)).toEqual({
      value: -2,
      unit: 'day',
    });
    expect(selectRelativeTimeParts(at(-60 * 86_400_000), now)).toEqual({
      value: -2,
      unit: 'month',
    });
  });

  it('keeps the sign so future timestamps read as "in …"', () => {
    expect(selectRelativeTimeParts(at(2 * 3_600_000), now)).toEqual({
      value: 2,
      unit: 'hour',
    });
  });
});
