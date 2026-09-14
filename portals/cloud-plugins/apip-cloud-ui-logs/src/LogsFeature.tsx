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

import { useCallback, useEffect, useMemo, useRef, useState, type FC } from 'react';
import {
  Alert,
  Button,
  CircularProgress,
  FormControlLabel,
  PageContent,
  PageTitle,
  Stack,
  Switch,
  Typography,
} from '@wso2/oxygen-ui';
import LogConsole from './LogConsole';
import LogsToolbar from './LogsToolbar';
import { deriveFacets, matchesView, mergeBufferedLines, type BufferedLine } from './consoleLines';
import { completenessNotes } from './format';
import { createLogsClient } from './logsApi';
import type { AIWorkspaceHostPort } from './hostPort';
import type { LogPage, LogQuery, LogViewFilters } from './types';

export type LogsFeatureProps = {
  port: AIWorkspaceHostPort;
};

/** Live-tail cadence. */
const LIVE_POLL_MS = 5_000;

/**
 * Rows the console keeps. Real DOM nodes rather than a virtualised window, so
 * the buffer is capped for a long tail; oldest rows fall off first.
 */
const MAX_CONSOLE_LINES = 2000;

/** Records requested per poll. */
const PAGE_LIMIT = 100;

const INITIAL_QUERY: LogQuery = {
  rangeMinutes: 60,
  kind: 'all',
  levels: [],
  searchPhrase: '',
  limit: PAGE_LIMIT,
  environment: '',
};

const INITIAL_VIEW: LogViewFilters = { project: '', pod: '' };

/** Until the first response says what retention really is. */
const ASSUMED_RETENTION_DAYS = 3;

/**
 * The extension's `render(port)` result: the organization's logs from
 * platform-api's `/logs`, shown as a terminal-style console.
 *
 * Scope is the whole organization unless an environment is picked — that one is
 * a query parameter. Narrowing further, to a project or a single pod, happens in
 * the browser: the observability API offers no filter for either.
 */
const LogsFeature: FC<LogsFeatureProps> = ({ port }) => {
  const { apiFetch, notify } = port;
  const client = useMemo(() => createLogsClient(apiFetch), [apiFetch]);

  const [query, setQuery] = useState<LogQuery>(INITIAL_QUERY);
  const [view, setView] = useState<LogViewFilters>(INITIAL_VIEW);
  const [buffer, setBuffer] = useState<BufferedLine[]>([]);
  const [page, setPage] = useState<LogPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [fetching, setFetching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [live, setLive] = useState(true);
  const [stale, setStale] = useState(false);
  const [environments, setEnvironments] = useState<string[]>([]);

  // Fetches overlap (an Apply during a live tick), so only the newest may write
  // state — a slower earlier response would append lines the filters no longer
  // describe.
  const loadSeqRef = useRef(0);
  /*
   * Watermark set by Clear, so the next poll cannot refill what was wiped. A ref
   * rather than state: read inside the merge, so a Clear during an in-flight
   * fetch still wins, and nothing renders from it.
   */
  const clearedAtRef = useRef(0);

  const load = useCallback(
    async ({ silent = false }: { silent?: boolean } = {}) => {
      const seq = ++loadSeqRef.current;
      setFetching(true);
      if (!silent) {
        setLoading(true);
        setError(null);
      }
      try {
        const result = await client.fetchLogs(query);
        if (seq !== loadSeqRef.current) return;
        setPage(result);
        setBuffer((previous) =>
          mergeBufferedLines(previous, result.list, MAX_CONSOLE_LINES, clearedAtRef.current)
        );
        setError(null);
        setStale(false);
      } catch (loadError) {
        if (seq !== loadSeqRef.current) return;
        // A failed background tick raises no alert — the lines on screen remain
        // and the next tick may succeed. Only a foreground load surfaces it. But
        // the console must stop calling itself live, or stale rows read as the
        // current answer for as long as the failure lasts.
        setStale(true);
        if (!silent) {
          setError(loadError instanceof Error ? loadError.message : 'Unable to load logs.');
        }
      } finally {
        if (seq === loadSeqRef.current) {
          setLoading(false);
          setFetching(false);
        }
      }
    },
    [client, query]
  );

  useEffect(() => {
    void load();
  }, [load]);

  // Fails soft: without the catalogue the select falls back to the environments
  // seen in the loaded lines.
  useEffect(() => {
    let cancelled = false;
    client
      .fetchEnvironments()
      .then((list) => {
        if (!cancelled) setEnvironments(list.map((environment) => environment.name));
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [client]);

  useEffect(() => {
    if (!live) return undefined;
    const timer = window.setInterval(() => {
      // A hidden tab stops polling: the window is rolling, so nothing is missed
      // by waiting.
      if (document.hidden) return;
      void load({ silent: true });
    }, LIVE_POLL_MS);
    return () => window.clearInterval(timer);
  }, [live, load]);

  const applyFilters = (nextQuery: LogQuery, nextView: LogViewFilters) => {
    setBuffer([]);
    clearedAtRef.current = 0;
    setQuery(nextQuery);
    setView(nextView);
  };

  const clearConsole = () => {
    clearedAtRef.current = Date.now();
    setBuffer([]);
  };

  const facets = useMemo(() => deriveFacets(buffer), [buffer]);
  const visible = useMemo(
    () => buffer.filter(({ entry }) => matchesView(entry, view)).map(({ line }) => line),
    [buffer, view]
  );

  const retentionDays = page?.window.retentionDays ?? ASSUMED_RETENTION_DAYS;
  const notes = [
    ...(page ? completenessNotes(page, query.kind !== 'all') : []),
    // The view filters run in the browser, so a short console is not a quiet
    // organization. Say how much was hidden, or it reads as the whole answer.
    ...(visible.length < buffer.length
      ? [`${visible.length} of ${buffer.length} loaded lines match the project and pod filters.`]
      : []),
  ];

  return (
    <PageContent fullWidth>
      <PageTitle sx={{ mb: 2 }}>
        <PageTitle.Header>Logs</PageTitle.Header>
        <PageTitle.SubHeader>
          Runtime logs from every workload in this organization, including gateway traffic and
          gateway activity. Kept for {retentionDays} days.
        </PageTitle.SubHeader>
      </PageTitle>

      <LogsToolbar
        environments={environments}
        facets={facets}
        query={query}
        retentionDays={retentionDays}
        view={view}
        onApply={applyFilters}
      />

      {error ? (
        <Alert
          severity="error"
          sx={{ mb: 1.5 }}
          action={
            <Button color="inherit" size="small" onClick={() => void load()}>
              Retry
            </Button>
          }
        >
          {error}
        </Alert>
      ) : null}

      {notes.map((note) => (
        <Alert key={note} severity="info" sx={{ mb: 1.5 }}>
          {note}
        </Alert>
      ))}

      <LogConsole
        actions={
          <Stack alignItems="center" direction="row" spacing={1}>
            {fetching ? <CircularProgress size={14} color="inherit" /> : null}
            {!live ? (
              <Button
                disabled={fetching}
                size="small"
                sx={{ color: 'inherit' }}
                onClick={() => void load({ silent: true })}
              >
                Refresh
              </Button>
            ) : null}
            <FormControlLabel
              sx={{ mr: 0 }}
              control={
                <Switch
                  checked={live}
                  color="success"
                  size="small"
                  onChange={(event) => setLive(event.target.checked)}
                />
              }
              label={<Typography variant="caption">Live tail</Typography>}
            />
          </Stack>
        }
        emptyMessage={
          loading
            ? 'Loading logs…'
            : 'No logs in this window. Try a wider time range or remove a filter.'
        }
        label="Organization log output"
        lines={visible}
        live={live}
        stale={stale}
        onClear={clearConsole}
        onCopyError={(message) => notify(message, 'error')}
      />
    </PageContent>
  );
};

export default LogsFeature;
