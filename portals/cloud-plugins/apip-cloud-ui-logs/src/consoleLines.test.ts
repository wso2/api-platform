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
  deriveFacets,
  matchesView,
  mergeBufferedLines,
  orderLines,
  toBufferedLines,
  toConsoleLine,
  toDetails,
  type BufferedLine,
} from './consoleLines';
import type { LogEntry, LogViewFilters } from './types';

const entry = (over: Partial<LogEntry> = {}): LogEntry => ({
  timestamp: '2026-09-12T06:00:00.000Z',
  log: 'started',
  kind: 'operational',
  ...over,
});

const noView: LogViewFilters = { projects: [] };

describe('toConsoleLine', () => {
  it('summarizes an access log and keeps the whole line behind raw', () => {
    const access = entry({
      kind: 'access',
      log: '{"method":"GET","path":"/orders","response_code":200,"duration":12}',
    });
    const line = toConsoleLine(access);
    expect(line.message).toBe('GET /orders 200 12ms');
    expect(line.raw).toBe(access.log);
  });

  it('offers no raw payload when the message is already the whole line', () => {
    expect(toConsoleLine(entry()).raw).toBeUndefined();
  });

  it('defaults the level, because an access log carries none', () => {
    expect(toConsoleLine(entry()).level).toBe('LOG');
    expect(toConsoleLine(entry({ level: 'warn' })).level).toBe('WARN');
  });

  it('names the component, falling back to the pod', () => {
    expect(toConsoleLine(entry({ componentName: 'gw', podName: 'gw-abc' })).source).toBe('gw');
    expect(toConsoleLine(entry({ podName: 'gw-abc' })).source).toBe('gw-abc');
    expect(toConsoleLine(entry()).source).toBeUndefined();
  });
});

describe('toBufferedLines', () => {
  // A gateway under load emits byte-identical access lines in the same
  // millisecond. Without the occurrence counter they would collapse to one row.
  it('keeps byte-identical lines apart', () => {
    const ids = toBufferedLines([entry(), entry(), entry()]).map((b) => b.line.id);
    expect(new Set(ids).size).toBe(3);
  });
});

describe('mergeBufferedLines', () => {
  it('appends oldest first, so the newest row is at the bottom', () => {
    // The API answers newest-first; a console reads the other way round.
    const merged = mergeBufferedLines(
      [],
      [
        entry({ timestamp: '2026-09-12T06:00:02.000Z', log: 'third' }),
        entry({ timestamp: '2026-09-12T06:00:01.000Z', log: 'second' }),
        entry({ timestamp: '2026-09-12T06:00:00.000Z', log: 'first' }),
      ],
      100
    );
    expect(merged.map((b) => b.line.message)).toEqual(['first', 'second', 'third']);
  });

  it('drops rows the buffer already holds, so a rolling window only adds', () => {
    const first = mergeBufferedLines([], [entry({ log: 'a' })], 100);
    const second = mergeBufferedLines(first, [entry({ log: 'b' }), entry({ log: 'a' })], 100);
    expect(second.map((b) => b.line.message)).toEqual(['a', 'b']);
  });

  // How a filter change is applied: the rows on screen answer the old question,
  // so the first page of the new one is merged into an empty buffer rather than
  // onto them. A live tick passes the buffer instead and appends.
  it('replaces the buffer when merged into an empty one', () => {
    const before = mergeBufferedLines([], [entry({ log: 'old' })], 100);
    const after = mergeBufferedLines([], [entry({ log: 'new' })], 100);
    expect(before.map((b) => b.line.message)).toEqual(['old']);
    expect(after.map((b) => b.line.message)).toEqual(['new']);
  });

  it('returns the same array when a poll brings nothing new', () => {
    // Identity matters: a new array on every idle tick would re-render the
    // console, and with it every row.
    const first = mergeBufferedLines([], [entry({ log: 'a' })], 100);
    expect(mergeBufferedLines(first, [entry({ log: 'a' })], 100)).toBe(first);
  });

  it('trims the oldest rows once the cap is reached', () => {
    const incoming = ['a', 'b', 'c'].map((log, index) =>
      entry({ log, timestamp: `2026-09-12T06:00:0${index}.000Z` })
    );
    const merged = mergeBufferedLines([], incoming, 2);
    expect(merged.map((b) => b.line.message)).toEqual(['b', 'c']);
  });

  // Ingestion lag differs per pod, so a poll can carry a record older than the
  // lines already on screen. Appending it would show it out of order.
  it('places a late record by its timestamp, not by the poll it arrived on', () => {
    const first = mergeBufferedLines(
      [],
      [entry({ timestamp: '2026-09-12T06:00:02.000Z', log: 'newer' })],
      100
    );
    const second = mergeBufferedLines(
      first,
      [entry({ timestamp: '2026-09-12T06:00:01.000Z', log: 'late' })],
      100
    );
    expect(second.map((b) => b.line.message)).toEqual(['late', 'newer']);
  });

  it('trims the oldest by timestamp, so a late record cannot evict a newer row', () => {
    const first = mergeBufferedLines(
      [],
      [entry({ timestamp: '2026-09-12T06:00:02.000Z', log: 'newer' })],
      1
    );
    const second = mergeBufferedLines(
      first,
      [entry({ timestamp: '2026-09-12T06:00:01.000Z', log: 'late' })],
      1
    );
    expect(second.map((b) => b.line.message)).toEqual(['newer']);
  });

  it('leaves an undated record where it arrived rather than at the top', () => {
    const first = mergeBufferedLines(
      [],
      [entry({ timestamp: '2026-09-12T06:00:00.000Z', log: 'dated' })],
      100
    );
    const second = mergeBufferedLines(first, [entry({ timestamp: '', log: 'undated' })], 100);
    expect(second.map((b) => b.line.message)).toEqual(['dated', 'undated']);
  });

  it('keeps a record whose timestamp cannot be parsed rather than hiding it', () => {
    const merged = mergeBufferedLines([], [entry({ timestamp: '', log: 'undated' })], 100);
    expect(merged.map((b) => b.line.message)).toEqual(['undated']);
  });
});

describe('deriveFacets', () => {
  const buffer = (...entries: LogEntry[]): BufferedLine[] => toBufferedLines(entries);

  // Most common first, so the list reads as "where the noise is". Ties break by
  // name, or a live tail would reshuffle the panel under the reader.
  it('counts what the loaded lines carry, commonest first', () => {
    const facets = deriveFacets(
      buffer(
        entry({ projectName: 'platform', environment: 'production' }),
        entry({ projectName: 'platform', environment: 'production' }),
        entry({ projectName: 'apip', environment: 'development' })
      )
    );
    expect(facets.projects).toEqual([
      { value: 'platform', label: 'platform', count: 2 },
      { value: 'apip', label: 'apip', count: 1 },
    ]);
    expect(facets.environments.map((facet) => facet.value)).toEqual([
      'production',
      'development',
    ]);
  });

  it('names a kind the way the rest of the page does', () => {
    const facets = deriveFacets(buffer(entry({ kind: 'access' })));
    expect(facets.kinds).toEqual([{ value: 'access', label: 'Gateway access', count: 1 }]);
  });

  /*
   * The level filter is a substring search of the raw line upstream, not a match
   * on a field, so an access log — which declares no level and contains no such
   * word — cannot be returned by ticking any level box. Counting it under a
   * default would put a number next to a box that does not keep those rows.
   */
  it('does not invent a level for a line that declares none', () => {
    const facets = deriveFacets(
      buffer(entry({ kind: 'access', level: undefined }), entry({ level: 'ERROR' }))
    );
    expect(facets.levels).toEqual([{ value: 'ERROR', label: 'ERROR', count: 1 }]);
  });

  it('still renders that line with a neutral chip rather than a level', () => {
    expect(toConsoleLine(entry({ kind: 'access', level: undefined })).level).toBe('LOG');
  });

  it('offers nothing when the lines carry no attribution', () => {
    const facets = deriveFacets(buffer(entry()));
    expect(facets.projects).toEqual([]);
    expect(facets.environments).toEqual([]);
  });
});

describe('toDetails', () => {
  // A missing field left in would read as a value of "none" rather than as "the
  // plane did not say".
  it('keeps only the fields the record carries', () => {
    const details = toDetails(entry({ podName: 'gw-1', projectName: 'wc-system' }));

    expect(details.map((detail) => detail.label)).toEqual([
      'Pod',
      'Project',
      'Type',
      'Timestamp',
    ]);
  });

  it('reports latency for an access line, and for nothing else', () => {
    const access = toDetails(
      entry({ kind: 'access', log: JSON.stringify({ method: 'GET', duration: 67 }) })
    );
    expect(access.find((detail) => detail.label === 'Latency')?.value).toBe('67ms');
    expect(toDetails(entry()).some((detail) => detail.label === 'Latency')).toBe(false);
  });
});

describe('matchesView', () => {
  it('matches everything when nothing is selected', () => {
    expect(matchesView(entry(), noView)).toBe(true);
  });

  it('keeps any of the selected projects', () => {
    expect(matchesView(entry({ projectName: 'p' }), { projects: ['p', 'q'] })).toBe(true);
    expect(matchesView(entry({ projectName: 'r' }), { projects: ['p', 'q'] })).toBe(false);
  });

  // The environment is a query parameter, so the browser must not narrow on it a
  // second time.
  it('ignores the environment', () => {
    expect(matchesView(entry({ environment: 'stage' }), noView)).toBe(true);
  });

  it('excludes a line with no project when one is selected', () => {
    expect(matchesView(entry(), { projects: ['p'] })).toBe(false);
  });
});

describe('orderLines', () => {
  it('leaves the buffer order alone when oldest-first', () => {
    const lines = [1, 2, 3];
    expect(orderLines(lines, false)).toBe(lines);
  });

  it('reverses a copy when newest-first, so the caller\u2019s buffer is untouched', () => {
    const lines = [1, 2, 3];
    expect(orderLines(lines, true)).toEqual([3, 2, 1]);
    expect(lines).toEqual([1, 2, 3]);
  });
});
