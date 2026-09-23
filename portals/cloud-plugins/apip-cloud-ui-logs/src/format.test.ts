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
  activeFilterCount,
  completenessNotes,
  downloadFilename,
  latencyOf,
  rangeOptionsFor,
  sameQuery,
  summarizeLine,
} from './format';
import type { LogEntry, LogPage, LogQuery, LogViewFilters } from './types';

const baseQuery: LogQuery = {
  rangeMinutes: 60,
  kinds: [],
  levels: [],
  searchPhrase: '',
  limit: 100,
  environment: '',
};

const noView: LogViewFilters = { projects: [] };

const anEntry = (overrides: Partial<LogEntry>): LogEntry => ({
  timestamp: '2026-09-12T06:00:00.000Z',
  log: '',
  kind: 'access',
  ...overrides,
});

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

  // `limit` is per poll, so a busy organization is always truncated and that one
  // note would never clear. The other two are about the window and the type
  // filter, which polling does not undo.
  it('drops only the truncation note while the tail is running', () => {
    expect(completenessNotes(page({ truncated: true }), false, true)).toEqual([]);
    expect(completenessNotes(page({ count: 3, fetched: 200 }), true, true)).toHaveLength(1);
  });
});

describe('activeFilterCount', () => {
  // The time range has a control of its own and is always set to something, so
  // counting it would mean the Filters button never reads as empty.
  it('ignores the time range', () => {
    expect(activeFilterCount({ ...baseQuery, rangeMinutes: 1440 }, noView)).toBe(0);
  });

  it('counts every box ticked, across both classes', () => {
    expect(
      activeFilterCount(
        {
          ...baseQuery,
          kinds: ['access'],
          levels: ['ERROR', 'WARN'],
          environment: 'production',
          searchPhrase: '504',
        },
        { projects: ['wc-system'] }
      )
    ).toBe(6);
  });

  // Both kinds ticked asks the same question as neither — but the reader has set
  // two controls, and Reset is the only thing that clears them. Counting it zero
  // disabled Reset and printed "No filters" under two ticked boxes.
  it('counts ticked kinds even when both are ticked', () => {
    expect(activeFilterCount({ ...baseQuery, kinds: ['access', 'operational'] }, noView)).toBe(1);
    expect(activeFilterCount({ ...baseQuery, kinds: [] }, noView)).toBe(0);
  });

  it('counts a search phrase, and ignores one that is only whitespace', () => {
    expect(activeFilterCount({ ...baseQuery, searchPhrase: ' 504 ' }, noView)).toBe(1);
    expect(activeFilterCount({ ...baseQuery, searchPhrase: '   ' }, noView)).toBe(0);
  });
});

describe('sameQuery', () => {
  // Setting a filter and undoing it inside the debounce window must not count as
  // a change, or the console refetches and replaces its buffer for nothing.
  // None and both send no `kind` at all, so moving between them must not cost a
  // fetch and a buffer replace.
  it('sees no kinds and both kinds as the same request', () => {
    expect(
      sameQuery(
        { ...baseQuery, kinds: [] },
        { ...baseQuery, kinds: ['access', 'operational'] }
      )
    ).toBe(true);
    expect(
      sameQuery({ ...baseQuery, kinds: [] }, { ...baseQuery, kinds: ['access'] })
    ).toBe(false);
  });

  it('sees a round trip back to the same values as no change', () => {
    expect(sameQuery(baseQuery, { ...baseQuery })).toBe(true);
    expect(sameQuery(baseQuery, { ...baseQuery, searchPhrase: '  ' })).toBe(true);
  });

  // The boxes are OR-ed server-side, so re-ticking one in a different order asks
  // the same question — and a change would wipe the console and refetch.
  it('ignores the order the boxes were ticked in', () => {
    expect(
      sameQuery({ ...baseQuery, levels: ['ERROR', 'WARN'] }, { ...baseQuery, levels: ['WARN', 'ERROR'] })
    ).toBe(true);
  });

  it('sees an actual change', () => {
    expect(sameQuery(baseQuery, { ...baseQuery, kinds: ['access'] })).toBe(false);
    expect(sameQuery(baseQuery, { ...baseQuery, levels: ['ERROR'] })).toBe(false);
    expect(sameQuery(baseQuery, { ...baseQuery, environment: 'production' })).toBe(false);
  });
});

describe('latencyOf', () => {
  it('reads the duration out of an access line', () => {
    expect(
      latencyOf(anEntry({ log: JSON.stringify({ duration: 67 }) }))
    ).toBe('67ms');
  });

  // Only an access log times itself. Anything else reporting 0 would read as
  // "instant" rather than as "not measured".
  it('reports nothing for anything else', () => {
    expect(latencyOf(anEntry({ kind: 'operational', log: 'started' }))).toBeUndefined();
    expect(latencyOf(anEntry({ log: 'not json' }))).toBeUndefined();
    expect(latencyOf(anEntry({ log: '{}' }))).toBeUndefined();
  });
});

describe('downloadFilename', () => {
  it('stamps the organization and the moment, with no characters a shell dislikes', () => {
    const name = downloadFilename('acme', new Date('2026-09-22T06:30:00.000Z'));
    expect(name).toBe('acme-logs-2026-09-22T06-30-00-000Z.log');
  });

  it('still names a file when the host port has no organization yet', () => {
    expect(downloadFilename('')).toContain('organization-logs-');
  });
});
