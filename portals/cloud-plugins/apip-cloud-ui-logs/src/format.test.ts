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

import { completenessNotes, rangeOptionsFor, summarizeLine } from './format';
import type { LogEntry, LogPage } from './types';

const page = (overrides: Partial<LogPage> = {}): LogPage => ({
  count: 10,
  list: [],
  window: { startTime: '', endTime: '', clamped: false, retentionDays: 3 },
  fetched: 10,
  total: 10,
  truncated: false,
  ...overrides,
});

describe('rangeOptionsFor', () => {
  it('drops ranges the backend cannot answer', () => {
    // 3 days of retention must not offer a 7-day range: the tail would come
    // back empty and read as a broken feature rather than an expired one.
    const labels = rangeOptionsFor(3).map((option) => option.label);
    expect(labels).toContain('Last 3 days');
    expect(labels).not.toContain('Last 7 days');
  });

  it('offers more when retention is longer', () => {
    expect(rangeOptionsFor(30).map((option) => option.label)).toContain('Last 7 days');
  });

  it('never returns an empty picker, even for an implausible retention', () => {
    for (const retention of [0, -5]) {
      const options = rangeOptionsFor(retention);
      expect(options.length).toBeGreaterThan(0);
      // Still honest about the horizon: a nonsense retention is floored at one
      // day rather than opening the picker back up.
      expect(Math.max(...options.map((option) => option.minutes))).toBeLessThanOrEqual(1440);
    }
  });
});

describe('summarizeLine', () => {
  const entry = (overrides: Partial<LogEntry>): LogEntry => ({
    timestamp: '',
    log: '',
    kind: 'access',
    ...overrides,
  });

  it('condenses an access log to the fields worth scanning', () => {
    expect(
      summarizeLine(
        entry({
          log: '{"method":"GET","path":"/pizzashack/1.0.0/menu","response_code":200,"duration":12,"user_agent":"curl"}',
        })
      )
    ).toBe('GET /pizzashack/1.0.0/menu 200 12ms');
  });

  it('leaves an operational line alone', () => {
    const line = '[2026-09-12 06:00:00][1][info][main] initializing';
    expect(summarizeLine(entry({ kind: 'operational', log: line }))).toBe(line);
  });

  it('falls back to the raw line when an access log is not the expected shape', () => {
    // A gateway whose json_fields were customised still has to be readable.
    expect(summarizeLine(entry({ log: 'not json at all' }))).toBe('not json at all');
    expect(summarizeLine(entry({ log: '{"trace_id":"abc"}' }))).toBe('{"trace_id":"abc"}');
    expect(summarizeLine(entry({ log: '[1,2,3]' }))).toBe('[1,2,3]');
  });
});

describe('completenessNotes', () => {
  it('says nothing when the page is complete', () => {
    expect(completenessNotes(page(), false)).toEqual([]);
  });

  it('explains a clamped window', () => {
    const notes = completenessNotes(page({ window: { startTime: '', endTime: '', clamped: true, retentionDays: 3 } }), false);
    expect(notes).toHaveLength(1);
    expect(notes[0]).toContain('3 days');
  });

  it('explains that the type filter ran after the fetch', () => {
    // The limit is applied by the backend BEFORE this filter, so a small list
    // does not mean few matches — without this note the reader would conclude
    // the wrong thing.
    const notes = completenessNotes(page({ count: 3, fetched: 200 }), true);
    expect(notes.join(' ')).toContain('3 of 200');
  });

  it('does not blame the type filter when none was applied', () => {
    expect(completenessNotes(page({ count: 3, fetched: 200 }), false)).toEqual([]);
  });

  it('says a narrower range is the only way to see more', () => {
    expect(completenessNotes(page({ truncated: true }), false)[0]).toContain('Narrow the time range');
  });
});
