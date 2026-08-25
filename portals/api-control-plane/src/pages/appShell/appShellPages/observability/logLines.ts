/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { RestApiObservabilityLog } from '../../../../api/resources/restApis';
import type { ConsoleLine } from '../../../../components/LogConsole';

/** Shape of the JSON payload the gateway emits for a proxied request. */
export type GatewayTrafficLog = {
  timestamp?: string;
  correlationId?: string;
  status?: number;
  operation?: { method?: string; path?: string };
  target?: { statusCode?: number };
};

/**
 * Parses a gateway traffic event out of a log line, tolerating the `[pol]`
 * prefix the policy engine adds. Returns `undefined` for free-form lines, which
 * are then shown as-is.
 */
export const parseGatewayTrafficLog = (
  entry: RestApiObservabilityLog
): GatewayTrafficLog | undefined => {
  if (typeof entry.log !== 'string') return undefined;
  try {
    const structuredLog = entry.log.trim().replace(/^\[pol\]\s*/, '');
    const parsed = JSON.parse(structuredLog) as unknown;
    return parsed && typeof parsed === 'object'
      ? (parsed as GatewayTrafficLog)
      : undefined;
  } catch {
    return undefined;
  }
};

/** Condenses a traffic event into one console-width line. */
const trafficSummary = (traffic: GatewayTrafficLog): string => {
  const status = traffic.status ?? traffic.target?.statusCode;
  const request = [traffic.operation?.method, traffic.operation?.path]
    .filter(Boolean)
    .join(' ');
  const parts = [
    request || undefined,
    status !== undefined ? `-> ${status}` : undefined,
    traffic.correlationId ? `[${traffic.correlationId}]` : undefined,
  ].filter(Boolean);
  return parts.join(' ');
};

/**
 * Content-derived identity for a record. The API returns no per-record id and a
 * rolling live window re-sends records the console already holds, so identity
 * has to come from the payload. `occurrence` separates byte-identical lines
 * within one response, and stays stable across polls as long as the set of
 * identical lines does.
 */
const lineId = (
  timestamp: string | undefined,
  level: string,
  body: string,
  occurrence: number
): string => `${timestamp ?? 'no-ts'}|${level}|${body}|#${occurrence}`;

/** Maps one observability record onto a console row. */
export const toConsoleLine = (
  entry: RestApiObservabilityLog,
  occurrence = 0
): ConsoleLine => {
  const traffic = parseGatewayTrafficLog(entry);
  const level = String(entry.level || entry.logLevel || 'INFO').toUpperCase();
  const timestamp = traffic?.timestamp || entry.timestamp;
  const raw = typeof entry.log === 'string' ? entry.log.trim() : undefined;
  const message = (traffic ? trafficSummary(traffic) : raw) || raw || '';

  return {
    id: lineId(timestamp, level, raw ?? message, occurrence),
    level,
    message,
    raw: raw !== undefined && raw !== message ? raw : undefined,
    timestamp,
  };
};

/** Maps one response page onto console rows, numbering repeated lines. */
export const toConsoleLines = (
  entries: RestApiObservabilityLog[]
): ConsoleLine[] => {
  const occurrences = new Map<string, number>();
  return entries.map((entry) => {
    const candidate = toConsoleLine(entry);
    const seen = occurrences.get(candidate.id) ?? 0;
    occurrences.set(candidate.id, seen + 1);
    return seen === 0 ? candidate : toConsoleLine(entry, seen);
  });
};

const parseTime = (line: ConsoleLine): number =>
  line.timestamp ? Date.parse(line.timestamp) : Number.NaN;

/** Sort key. Records without a usable timestamp sort as oldest. */
const timeValue = (line: ConsoleLine): number => {
  const parsed = parseTime(line);
  return Number.isNaN(parsed) ? 0 : parsed;
};

/**
 * Whether a record post-dates the caller's watermark. A record with no usable
 * timestamp is kept: dropping it would hide it permanently, while keeping it
 * only risks showing it once more after a clear.
 */
const isAfter = (line: ConsoleLine, since: number): boolean => {
  if (since === 0) return true;
  const parsed = parseTime(line);
  return Number.isNaN(parsed) || parsed > since;
};

/**
 * Appends a freshly polled page to the console buffer, oldest row first.
 *
 * The API returns newest-first pages, so each batch is sorted ascending (the
 * server's order breaking ties, which keeps records without a usable timestamp
 * where they arrived) before being appended. Rows already in the buffer are
 * dropped, and the buffer is trimmed from the front so a long live tail stays
 * bounded.
 *
 * `since` (epoch ms) is the caller's watermark: records at or before it are
 * ignored. Clearing the console sets it to the moment of the clear, so the next
 * poll of an unchanged rolling window does not refill what was just wiped.
 */
export const mergeLogLines = (
  existing: ConsoleLine[],
  incoming: RestApiObservabilityLog[],
  limit: number,
  since = 0
): ConsoleLine[] => {
  const seen = new Set(existing.map((line) => line.id));
  const fresh = toConsoleLines(incoming)
    .filter((line) => !seen.has(line.id) && isAfter(line, since))
    .map((line, index) => ({ index, line }))
    .sort((a, b) => timeValue(a.line) - timeValue(b.line) || a.index - b.index)
    .map((item) => item.line);

  if (fresh.length === 0) return existing;

  const merged = [...existing, ...fresh];
  return merged.length > limit ? merged.slice(merged.length - limit) : merged;
};
