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
  toBufferedLines,
  toConsoleLine,
  type BufferedLine,
} from './consoleLines';
import type { LogEntry, LogViewFilters } from './types';

const entry = (over: Partial<LogEntry> = {}): LogEntry => ({
  timestamp: '2026-09-12T06:00:00.000Z',
  log: 'started',
  kind: 'operational',
  ...over,
});

const noView: LogViewFilters = { project: '', component: '', environment: '' };

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
    expect(toConsoleLine(entry()).level).toBe('INFO');
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

  it('ignores records at or before the clear watermark', () => {
    const since = Date.parse('2026-09-12T06:00:05.000Z');
    const merged = mergeBufferedLines(
      [],
      [
        entry({ timestamp: '2026-09-12T06:00:09.000Z', log: 'after' }),
        entry({ timestamp: '2026-09-12T06:00:01.000Z', log: 'before' }),
      ],
      100,
      since
    );
    expect(merged.map((b) => b.line.message)).toEqual(['after']);
  });

  it('keeps a record whose timestamp cannot be parsed rather than hiding it', () => {
    const since = Date.parse('2026-09-12T06:00:05.000Z');
    const merged = mergeBufferedLines([], [entry({ timestamp: '', log: 'undated' })], 100, since);
    expect(merged.map((b) => b.line.message)).toEqual(['undated']);
  });
});

describe('deriveFacets', () => {
  const buffer = (...entries: LogEntry[]): BufferedLine[] => toBufferedLines(entries);

  it('lists only what the loaded lines actually carry, sorted and deduped', () => {
    const facets = deriveFacets(
      buffer(
        entry({ projectName: 'platform', componentName: 'gw-b', environment: 'production' }),
        entry({ projectName: 'platform', componentName: 'gw-a', environment: 'production' }),
        entry({ projectName: 'apip', componentName: 'gw-a', environment: 'development' })
      )
    );
    expect(facets.projects).toEqual(['apip', 'platform']);
    expect(facets.components).toEqual(['gw-a', 'gw-b']);
    expect(facets.environments).toEqual(['development', 'production']);
  });

  it('groups components under their project, so picking one narrows the next list', () => {
    const facets = deriveFacets(
      buffer(
        entry({ projectName: 'platform', componentName: 'gw-a' }),
        entry({ projectName: 'apip', componentName: 'gw-z' })
      )
    );
    expect(facets.componentsByProject).toEqual({ platform: ['gw-a'], apip: ['gw-z'] });
  });

  it('offers nothing when the lines carry no attribution', () => {
    const facets = deriveFacets(buffer(entry()));
    expect(facets.projects).toEqual([]);
    expect(facets.components).toEqual([]);
    expect(facets.environments).toEqual([]);
  });
});

describe('matchesView', () => {
  it('matches everything when nothing is selected', () => {
    expect(matchesView(entry(), noView)).toBe(true);
  });

  it('ANDs the three filters', () => {
    const line = entry({ projectName: 'p', componentName: 'c', environment: 'e' });
    expect(matchesView(line, { project: 'p', component: 'c', environment: 'e' })).toBe(true);
    expect(matchesView(line, { ...noView, component: 'other' })).toBe(false);
  });

  it('excludes a line with no value for a selected filter', () => {
    expect(matchesView(entry(), { ...noView, project: 'p' })).toBe(false);
  });
});
