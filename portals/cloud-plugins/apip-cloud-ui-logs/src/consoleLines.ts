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

import type { ConsoleDetail, ConsoleLine } from './LogConsole';
import { KIND_LABELS, latencyOf, summarizeLine } from './format';
import type { Facet, LogEntry, LogFacets, LogKind, LogViewFilters } from './types';

/**
 * Shown for a record that declares no level — an access log never does.
 *
 * Deliberately not `INFO`. The level filter is a **substring search of the raw
 * line** upstream (`*ERROR*`, case-insensitive), not a match on a field, so a
 * line rendered as INFO would not be selected by ticking Info. Labelling it as a
 * level it does not have, and cannot be found by, is the lie the panel's counts
 * would then repeat.
 */
const DEFAULT_LEVEL = 'LOG';

/**
 * Content-derived identity: the API returns no per-record id and a rolling
 * window re-sends records the console holds. `occurrence` separates
 * byte-identical lines and is stable across polls while their set is.
 */
const lineId = (
  timestamp: string | undefined,
  level: string,
  body: string,
  occurrence: number
): string => `${timestamp || 'no-ts'}|${level}|${body}|#${occurrence}`;

/**
 * What an expanded row shows: the attribution the one-line form drops. Only the
 * fields the record actually carries — an absent value would read as a value of
 * "none" rather than as "the plane did not say".
 */
export function toDetails(entry: LogEntry): ConsoleDetail[] {
  const pairs: [string, string | undefined][] = [
    ['Pod', entry.podName],
    ['Container', entry.containerName],
    ['Component', entry.componentName],
    ['Project', entry.projectName],
    ['Environment', entry.environment],
    ['Type', KIND_LABELS[entry.kind]],
    ['Latency', latencyOf(entry)],
    ['Timestamp', entry.timestamp],
  ];
  return pairs
    .filter(([, value]) => Boolean(value))
    .map(([label, value]) => ({ label, value: value as string }));
}

/** Maps one log entry onto a console row. */
export function toConsoleLine(entry: LogEntry, occurrence = 0): ConsoleLine {
  const level = String(entry.level || DEFAULT_LEVEL).toUpperCase();
  const raw = entry.log.trim();
  const message = summarizeLine(entry).trim() || raw;
  // Component first — the name a reader recognises; the pod adds a scheduler
  // hash that only widens the column. Pod is the fallback: always present.
  const source = entry.componentName || entry.podName || undefined;

  return {
    details: toDetails(entry),
    id: lineId(entry.timestamp, level, raw, occurrence),
    level,
    message,
    raw: raw !== message ? raw : undefined,
    source,
    timestamp: entry.timestamp || undefined,
  };
}

/**
 * The rendered line plus the entry it came from. The console needs only `line`,
 * but the view filters select on fields that live on the entry. Keeping both
 * lets the buffer stay unfiltered, so the facet lists do not collapse to
 * whatever is selected.
 */
export type BufferedLine = { line: ConsoleLine; entry: LogEntry };

/** Maps one response page onto buffered rows, numbering repeated lines. */
export function toBufferedLines(entries: LogEntry[]): BufferedLine[] {
  const occurrences = new Map<string, number>();
  return entries.map((entry) => {
    const candidate = toConsoleLine(entry);
    const seen = occurrences.get(candidate.id) ?? 0;
    occurrences.set(candidate.id, seen + 1);
    return { entry, line: seen === 0 ? candidate : toConsoleLine(entry, seen) };
  });
}

const parseTime = (line: ConsoleLine): number =>
  line.timestamp ? Date.parse(line.timestamp) : Number.NaN;

/** Sort key. Records without a usable timestamp sort as oldest. */
const timeValue = (line: ConsoleLine): number => {
  const parsed = parseTime(line);
  return Number.isNaN(parsed) ? 0 : parsed;
};

/**
 * Merges a freshly polled page in, oldest row first. Pages arrive newest-first,
 * so each batch is sorted ascending (server order breaking ties); rows already
 * held are dropped and the buffer is trimmed from the front.
 *
 * Passing an empty `existing` is how a filter change replaces the console rather
 * than merging into it.
 */
export function mergeBufferedLines(
  existing: BufferedLine[],
  incoming: LogEntry[],
  limit: number
): BufferedLine[] {
  const seen = new Set(existing.map((buffered) => buffered.line.id));
  const fresh = toBufferedLines(incoming)
    .filter((buffered) => !seen.has(buffered.line.id))
    .map((buffered, index) => ({ buffered, index }))
    .sort((a, b) => timeValue(a.buffered.line) - timeValue(b.buffered.line) || a.index - b.index)
    .map((item) => item.buffered);

  if (fresh.length === 0) return existing;

  /*
   * Ordered as a whole rather than batch-appended: ingestion lag differs per
   * pod, so a poll can carry a record older than lines already on screen, and
   * appending it would show it below them — and then trimming to `limit` could
   * drop a newer row instead of it. A row with no usable timestamp inherits the
   * key of the row above it, which keeps it where it arrived rather than
   * sinking it to the top of the buffer.
   */
  let key = 0;
  const merged = [...existing, ...fresh]
    .map((buffered, index) => {
      const parsed = parseTime(buffered.line);
      if (!Number.isNaN(parsed)) key = parsed;
      return { buffered, index, key };
    })
    .sort((a, b) => a.key - b.key || a.index - b.index)
    .map((item) => item.buffered);

  return merged.length > limit ? merged.slice(merged.length - limit) : merged;
}

/**
 * Counted options for one field, most common first, ties broken by name so the
 * list does not reshuffle as a live tail arrives.
 */
const countFacets = (
  values: Iterable<string>,
  label: (value: string) => string = (value) => value
): Facet[] => {
  const counts = new Map<string, number>();
  for (const value of values) {
    if (value) counts.set(value, (counts.get(value) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([value, count]) => ({ value, label: label(value), count }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
};

/**
 * What the filter panel can offer, read off the buffer rather than a catalogue:
 * a value with nothing in this window is not offered, because picking it could
 * only produce an empty console.
 *
 * Every count is of *loaded* lines. Kind, level and environment are query
 * parameters, so once one is set the others count zero and drop out — the panel
 * keeps a selected value visible for exactly that reason.
 */
export function deriveFacets(buffer: BufferedLine[]): LogFacets {
  const entries = buffer.map(({ entry }) => entry);
  return {
    kinds: countFacets(
      entries.map((entry) => entry.kind),
      (value) => KIND_LABELS[value as LogKind] ?? value
    ),
    projects: countFacets(entries.map((entry) => entry.projectName ?? '')),
    environments: countFacets(entries.map((entry) => entry.environment ?? '')),
    // Only what a record actually declares. Counting a level-less line under some
    // default would promise rows that ticking that box cannot return.
    levels: countFacets(entries.map((entry) => (entry.level ?? '').toUpperCase())),
  };
}

/** Whether an entry survives the view filters. An empty filter matches everything. */
export function matchesView(entry: LogEntry, view: LogViewFilters): boolean {
  if (view.projects.length === 0) return true;
  return Boolean(entry.projectName) && view.projects.includes(entry.projectName as string);
}

/**
 * The rows in the order they are shown. The buffer is always held oldest-first
 * — `mergeBufferedLines` depends on that to re-sort late arrivals and to trim
 * the right end — so newest-first is a reversal at the edge, never a different
 * buffer.
 */
export function orderLines<T>(lines: T[], newestFirst: boolean): T[] {
  return newestFirst ? [...lines].reverse() : lines;
}
