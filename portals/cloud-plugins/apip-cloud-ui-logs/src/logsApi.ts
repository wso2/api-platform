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

import type { ApiFetch } from './hostPort';
import type { LogEntry, LogKind, LogPage, LogQuery } from './types';

/** The `/logs` response as platform-api sends it. */
type LogPageDTO = {
  count?: number;
  list?: LogEntryDTO[];
  window?: { startTime?: string; endTime?: string; clamped?: boolean; retentionDays?: number };
  fetched?: number;
  total?: number;
  truncated?: boolean;
};

type LogEntryDTO = {
  timestamp?: string;
  log?: string;
  level?: string;
  kind?: string;
  podName?: string;
  containerName?: string;
  componentName?: string;
  projectName?: string;
  environment?: string;
};

/** Default retention, used only until the first response says otherwise. */
const FALLBACK_RETENTION_DAYS = 3;

const normalizeKind = (kind?: string): LogKind => (kind === 'access' ? 'access' : 'operational');

const mapEntry = (dto: LogEntryDTO): LogEntry => ({
  timestamp: dto.timestamp ?? '',
  log: dto.log ?? '',
  level: dto.level,
  kind: normalizeKind(dto.kind),
  podName: dto.podName,
  containerName: dto.containerName,
  componentName: dto.componentName,
  projectName: dto.projectName,
  environment: dto.environment,
});

/**
 * Builds the query string for one page. The window is computed at fetch time,
 * not pinned when the range was picked, so "last 15 minutes" still means that
 * on refresh. Only non-default values are sent, leaving the server's defaults
 * as their single definition.
 */
export function buildLogsQuery(query: LogQuery, nowMs: number): string {
  const params = new URLSearchParams();
  const end = new Date(nowMs);
  const start = new Date(nowMs - query.rangeMinutes * 60_000);
  params.set('startTime', start.toISOString());
  params.set('endTime', end.toISOString());
  params.set('limit', String(query.limit));
  if (query.kind !== 'all') params.set('kind', query.kind);
  for (const level of query.levels) params.append('logLevels', level);
  const phrase = query.searchPhrase.trim();
  if (phrase) params.set('searchPhrase', phrase);
  return params.toString();
}

/** The logs data client, built from the host-injected `apiFetch`. */
export function createLogsClient(apiFetch: ApiFetch) {
  return {
    async fetchLogs(query: LogQuery, nowMs: number = Date.now()): Promise<LogPage> {
      const response = await apiFetch<LogPageDTO>('GET', `/logs?${buildLogsQuery(query, nowMs)}`);
      const list = (response?.list ?? []).map(mapEntry);
      return {
        count: response?.count ?? list.length,
        list,
        window: {
          startTime: response?.window?.startTime ?? '',
          endTime: response?.window?.endTime ?? '',
          clamped: response?.window?.clamped ?? false,
          retentionDays: response?.window?.retentionDays ?? FALLBACK_RETENTION_DAYS,
        },
        fetched: response?.fetched ?? list.length,
        total: response?.total ?? list.length,
        truncated: response?.truncated ?? false,
      };
    },
  };
}
