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

import { describe, expect, it, vi } from 'vitest';

import { buildLogsQuery, createLogsClient } from './logsApi';
import type { LogQuery } from './types';

const baseQuery: LogQuery = {
  rangeMinutes: 60,
  kind: 'all',
  levels: [],
  searchPhrase: '',
  limit: 200,
};

// 2026-09-12T12:00:00Z
const NOW = Date.UTC(2026, 8, 12, 12, 0, 0);

describe('buildLogsQuery', () => {
  it('derives the window from the range at fetch time', () => {
    const params = new URLSearchParams(buildLogsQuery(baseQuery, NOW));
    expect(params.get('endTime')).toBe('2026-09-12T12:00:00.000Z');
    expect(params.get('startTime')).toBe('2026-09-12T11:00:00.000Z');
    expect(params.get('limit')).toBe('200');
  });

  it('omits the filters that are at their default, leaving the server to define them', () => {
    const params = new URLSearchParams(buildLogsQuery(baseQuery, NOW));
    expect(params.has('kind')).toBe(false);
    expect(params.has('logLevels')).toBe(false);
    expect(params.has('searchPhrase')).toBe(false);
  });

  it('repeats logLevels rather than joining them', () => {
    const params = new URLSearchParams(
      buildLogsQuery({ ...baseQuery, levels: ['WARN', 'ERROR'] }, NOW)
    );
    expect(params.getAll('logLevels')).toEqual(['WARN', 'ERROR']);
  });

  it('sends a trimmed search phrase, and none at all when it is blank', () => {
    expect(
      new URLSearchParams(buildLogsQuery({ ...baseQuery, searchPhrase: '  /menu  ' }, NOW)).get(
        'searchPhrase'
      )
    ).toBe('/menu');
    expect(
      new URLSearchParams(buildLogsQuery({ ...baseQuery, searchPhrase: '   ' }, NOW)).has(
        'searchPhrase'
      )
    ).toBe(false);
  });

  it('sends kind only when it narrows', () => {
    const params = new URLSearchParams(buildLogsQuery({ ...baseQuery, kind: 'access' }, NOW));
    expect(params.get('kind')).toBe('access');
  });
});

describe('createLogsClient', () => {
  it('maps a page and keeps the counts that say how complete it is', async () => {
    const apiFetch = vi.fn().mockResolvedValue({
      count: 1,
      list: [
        {
          timestamp: '2026-09-12T11:59:00Z',
          log: '{"method":"GET"}',
          level: 'INFO',
          kind: 'access',
          podName: 'gw-runtime-1',
        },
      ],
      window: { startTime: 'a', endTime: 'b', clamped: true, retentionDays: 3 },
      fetched: 200,
      total: 1000,
      truncated: true,
    });

    const page = await createLogsClient(apiFetch).fetchLogs(baseQuery, NOW);

    expect(apiFetch).toHaveBeenCalledWith('GET', expect.stringContaining('/logs?'));
    expect(page.list[0].kind).toBe('access');
    expect(page.list[0].podName).toBe('gw-runtime-1');
    expect(page.fetched).toBe(200);
    expect(page.total).toBe(1000);
    expect(page.truncated).toBe(true);
    expect(page.window.clamped).toBe(true);
    expect(page.window.retentionDays).toBe(3);
  });

  it('survives an empty or partial response rather than throwing at the caller', async () => {
    const page = await createLogsClient(vi.fn().mockResolvedValue(undefined)).fetchLogs(
      baseQuery,
      NOW
    );
    expect(page.list).toEqual([]);
    expect(page.count).toBe(0);
    expect(page.truncated).toBe(false);
    // Falls back to the documented retention so the picker still has a bound.
    expect(page.window.retentionDays).toBe(3);
  });

  it('treats an unknown kind as operational, never as request traffic', async () => {
    const apiFetch = vi.fn().mockResolvedValue({ list: [{ log: 'x', kind: 'something-new' }] });
    const page = await createLogsClient(apiFetch).fetchLogs(baseQuery, NOW);
    expect(page.list[0].kind).toBe('operational');
  });
});
