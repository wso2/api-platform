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

const noView: LogViewFilters = { project: '', pod: '' };

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
        entry({ projectName: 'platform', podName: 'gw-b-1', environment: 'production' }),
        entry({ projectName: 'platform', podName: 'gw-a-1', environment: 'production' }),
        entry({ projectName: 'apip', podName: 'gw-a-1', environment: 'development' })
      )
    );
    expect(facets.projects).toEqual(['apip', 'platform']);
    expect(facets.pods).toEqual(['gw-a-1', 'gw-b-1']);
    expect(facets.environments).toEqual(['development', 'production']);
  });

  // The two pods of one gateway differ only by their replicaset and pod
  // suffixes, so the filter has to keep the whole name.
  it('keeps each pod separate, suffixes and all', () => {
    const facets = deriveFacets(
      buffer(
        entry({ podName: 'gw-34e37ec4-gateway-gateway-runtime-7997466b86-958pf' }),
        entry({ podName: 'gw-34e37ec4-gateway-controller-8d9fbd895-sxgmf' })
      )
    );
    expect(facets.pods).toEqual([
      'gw-34e37ec4-gateway-controller-8d9fbd895-sxgmf',
      'gw-34e37ec4-gateway-gateway-runtime-7997466b86-958pf',
    ]);
  });

  it('groups pods under their project, so picking one narrows the next list', () => {
    const facets = deriveFacets(
      buffer(
        entry({ projectName: 'platform', podName: 'gw-a-1' }),
        entry({ projectName: 'apip', podName: 'gw-z-1' })
      )
    );
    expect(facets.podsByProject).toEqual({ platform: ['gw-a-1'], apip: ['gw-z-1'] });
  });

  it('offers nothing when the lines carry no attribution', () => {
    const facets = deriveFacets(buffer(entry()));
    expect(facets.projects).toEqual([]);
    expect(facets.pods).toEqual([]);
    expect(facets.environments).toEqual([]);
  });
});

describe('matchesView', () => {
  it('matches everything when nothing is selected', () => {
    expect(matchesView(entry(), noView)).toBe(true);
  });

  it('ANDs the two filters', () => {
    const line = entry({ projectName: 'p', podName: 'gw-1' });
    expect(matchesView(line, { project: 'p', pod: 'gw-1' })).toBe(true);
    expect(matchesView(line, { ...noView, pod: 'other' })).toBe(false);
  });

  // The environment is a query parameter now, so the browser must not narrow on
  // it a second time.
  it('ignores the environment', () => {
    expect(matchesView(entry({ environment: 'stage' }), noView)).toBe(true);
  });

  it('excludes a line with no value for a selected filter', () => {
    expect(matchesView(entry(), { ...noView, project: 'p' })).toBe(false);
  });
});
