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

import type { ConsoleLine } from './LogConsole';
import { summarizeLine } from './format';
import type { LogEntry, LogFacets, LogViewFilters } from './types';

/** Level assumed for a record that carries none — an access log never does. */
const DEFAULT_LEVEL = 'INFO';

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

/** Maps one log entry onto a console row. */
export function toConsoleLine(entry: LogEntry, occurrence = 0): ConsoleLine {
  const level = String(entry.level || DEFAULT_LEVEL).toUpperCase();
  const raw = entry.log.trim();
  const message = summarizeLine(entry).trim() || raw;
  // Component first — the name a reader recognises; the pod adds a scheduler
  // hash that only widens the column. Pod is the fallback: always present.
  const source = entry.componentName || entry.podName || undefined;

  return {
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
 * Whether a record post-dates the caller's watermark. One with no usable
 * timestamp is kept — dropping it hides it permanently, keeping it only risks
 * showing it once more after a clear.
 */
const isAfter = (line: ConsoleLine, since: number): boolean => {
  if (since === 0) return true;
  const parsed = parseTime(line);
  return Number.isNaN(parsed) || parsed > since;
};

/**
 * Merges a freshly polled page in, oldest row first. Pages arrive newest-first,
 * so each batch is sorted ascending (server order breaking ties); rows already
 * held are dropped and the buffer is trimmed from the front.
 *
 * `since` (epoch ms) is the caller's watermark — a clear sets it to that moment
 * so the next poll of an unchanged window does not refill what was wiped.
 */
export function mergeBufferedLines(
  existing: BufferedLine[],
  incoming: LogEntry[],
  limit: number,
  since = 0
): BufferedLine[] {
  const seen = new Set(existing.map((buffered) => buffered.line.id));
  const fresh = toBufferedLines(incoming)
    .filter((buffered) => !seen.has(buffered.line.id) && isAfter(buffered.line, since))
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

const sortedUnique = (values: Iterable<string>): string[] =>
  [...new Set(values)].filter(Boolean).sort((a, b) => a.localeCompare(b));

/**
 * What the view filters can offer, read off the buffer rather than a catalogue:
 * a project with nothing in this window is not offered, because picking it
 * could only produce an empty console.
 */
export function deriveFacets(buffer: BufferedLine[]): LogFacets {
  const projects: string[] = [];
  const environments: string[] = [];

  for (const { entry } of buffer) {
    if (entry.projectName) projects.push(entry.projectName);
    if (entry.environment) environments.push(entry.environment);
  }

  return {
    projects: sortedUnique(projects),
    environments: sortedUnique(environments),
  };
}

/** Whether an entry survives the view filter. An empty filter matches everything. */
export function matchesView(entry: LogEntry, view: LogViewFilters): boolean {
  if (view.project && entry.projectName !== view.project) return false;
  return true;
}
