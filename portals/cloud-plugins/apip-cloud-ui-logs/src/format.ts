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

import type { LogEntry, LogKind, LogPage, LogQuery, LogViewFilters } from './types';

/** Time-range choices, capped at the server's retention by rangeOptionsFor. */
export const RANGE_OPTIONS: { label: string; minutes: number }[] = [
  { label: 'Last 15 minutes', minutes: 15 },
  { label: 'Last 1 hour', minutes: 60 },
  { label: 'Last 6 hours', minutes: 360 },
  { label: 'Last 24 hours', minutes: 1440 },
  { label: 'Last 3 days', minutes: 4320 },
  { label: 'Last 7 days', minutes: 10080 },
];

/**
 * The ranges worth offering for a given retention. Anything past the horizon is
 * dropped rather than clamped silently — offering "last 7 days" against 3 days
 * of data reads as a broken feature. A range equal to the horizon is kept, and
 * an implausible retention still yields one option rather than none.
 */
export function rangeOptionsFor(retentionDays: number): { label: string; minutes: number }[] {
  const horizon = Math.max(retentionDays, 1) * 1440;
  const within = RANGE_OPTIONS.filter((option) => option.minutes <= horizon);
  if (within.length > 0) return within;
  return [RANGE_OPTIONS[0]];
}

/** What each kind is called wherever it is shown. */
export const KIND_LABELS: Record<LogKind, string> = {
  access: 'Gateway access',
  operational: 'Gateway activity',
};

/**
 * Whether two queries ask the same thing. Field by field rather than by
 * identity, so setting a filter and undoing it inside the debounce window does
 * not count as a change and cost a fetch.
 */
const effectiveKind = (query: LogQuery): string =>
  query.kinds.length === 1 ? query.kinds[0] : '';

export function sameQuery(left: LogQuery, right: LogQuery): boolean {
  // As sets: the boxes are OR-ed server-side, so unticking Error and re-ticking
  // it reorders the list without changing the question — and an order-sensitive
  // compare would wipe the console and refetch for nothing.
  const sameList = (a: readonly string[], b: readonly string[]) => {
    if (a.length !== b.length) return false;
    const left_ = [...a].sort();
    const right_ = [...b].sort();
    return left_.every((value, index) => value === right_[index]);
  };
  return (
    left.rangeMinutes === right.rangeMinutes &&
    left.limit === right.limit &&
    left.environment === right.environment &&
    left.searchPhrase.trim() === right.searchPhrase.trim() &&
    // The request, not the boxes: `kind` is sent only when exactly one is
    // ticked, so none and both ask the same question and must not cost a fetch
    // and a buffer replace between them.
    effectiveKind(left) === effectiveKind(right) &&
    sameList(left.levels, right.levels)
  );
}

/** The picker's own wording for a range, or a plain minute count if it is not one. */
export function rangeLabel(minutes: number): string {
  return RANGE_OPTIONS.find((option) => option.minutes === minutes)?.label ?? `${minutes} minutes`;
}

/**
 * How many filters are on, which is what the Filters button counts and what
 * decides whether there is anything to reset.
 *
 * The time range is excluded: it has a control of its own and is always set to
 * something. The search phrase counts, even though its box lives outside the
 * panel: an empty console with a phrase typed is the commonest way to end up
 * filtered into nothing, and the reader needs something to clear.
 *
 * Kind counts once whenever anything is ticked, even though ticking both asks
 * the same question as ticking neither. This counts *controls the reader has
 * set*, not parameters sent — it is what enables Reset, and two ticked boxes
 * with a disabled Reset and "No filters" underneath them is a dead end.
 */
export function activeFilterCount(query: LogQuery, view: LogViewFilters): number {
  return (
    (query.kinds.length > 0 ? 1 : 0) +
    query.levels.length +
    view.projects.length +
    (query.environment ? 1 : 0) +
    (query.searchPhrase.trim() ? 1 : 0)
  );
}

/**
 * Latency in the line, when it says. Only an access log does — it is one of the
 * fields `summarizeLine` pulls out — so everything else reports nothing rather
 * than a zero that reads as "instant".
 */
export function latencyOf(entry: LogEntry): string | undefined {
  if (entry.kind !== 'access') return undefined;
  try {
    const parsed: unknown = JSON.parse(entry.log);
    if (!parsed || typeof parsed !== 'object') return undefined;
    const duration = Number((parsed as Record<string, unknown>).duration);
    // Same shape as the one-line summary, or a row and its own detail grid
    // disagree about the number they both took from the same field.
    return Number.isFinite(duration) ? `${duration}ms` : undefined;
  } catch {
    return undefined;
  }
}

/**
 * A one-line summary of an access-log entry, or the raw line for anything else.
 * An Envoy access log is ~40 JSON fields and unreadable raw, so the common ones
 * are pulled out; a malformed or customised line still shows in full.
 */
export function summarizeLine(entry: LogEntry): string {
  if (entry.kind !== 'access') return entry.log;
  let fields: Record<string, unknown>;
  try {
    const parsed: unknown = JSON.parse(entry.log);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return entry.log;
    fields = parsed as Record<string, unknown>;
  } catch {
    return entry.log;
  }
  const text = (key: string): string => {
    const value = fields[key];
    return typeof value === 'string' || typeof value === 'number' ? String(value) : '';
  };
  const parts = [
    text('method'),
    text('path'),
    text('response_code'),
    text('duration') && `${text('duration')}ms`,
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(' ') : entry.log;
}

/**
 * How complete a page is. Three facts, each misleading alone: the window was
 * clamped; `kind` filtered AFTER the fetch, so a short list is not few matches;
 * and more matched than can be asked for, with no cursor to follow.
 */
export function completenessNotes(
  page: LogPage,
  kindFiltered: boolean,
  liveTail = false
): string[] {
  const notes: string[] = [];
  if (page.window.clamped) {
    notes.push(
      `Only the last ${page.window.retentionDays} days are kept, so the range was shortened.`
    );
  }
  if (kindFiltered && page.fetched > page.count) {
    notes.push(
      `${page.count} of ${page.fetched} fetched lines match this type — the type filter is applied after fetching, so narrow the time range to see more.`
    );
  }
  // `limit` is per poll, so a busy organization is always truncated and this
  // would never clear while the tail runs. The other two notes are about the
  // window and the type filter, which polling does not undo.
  if (page.truncated && !liveTail) {
    notes.push('More lines matched than can be shown. Narrow the time range to see the rest.');
  }
  return notes;
}

/** Filename for a download, stamped so two saves never collide. */
export function downloadFilename(orgHandle: string, at: Date = new Date()): string {
  const stamp = at.toISOString().replace(/[:.]/g, '-');
  return `${orgHandle || 'organization'}-logs-${stamp}.log`;
}
