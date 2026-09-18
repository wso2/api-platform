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

import type { LogEntry, LogPage } from './types';

/** Time-range choices, capped at the server's retention by rangeOptionsFor. */
export const RANGE_OPTIONS: { label: string; minutes: number }[] = [
  { label: 'Last 15 minutes', minutes: 15 },
  { label: 'Last hour', minutes: 60 },
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
export function completenessNotes(page: LogPage, kindFiltered: boolean): string[] {
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
  if (page.truncated) {
    notes.push('More lines matched than can be shown. Narrow the time range to see the rest.');
  }
  return notes;
}
