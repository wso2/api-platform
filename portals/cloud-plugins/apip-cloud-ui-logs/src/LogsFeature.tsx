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
import { Alert, Button, Chip, PageContent, PageTitle, Stack, Typography } from '@wso2/oxygen-ui';
import { Clock } from '@wso2/oxygen-ui-icons-react';
import LogConsole, { type ConsoleLine } from './LogConsole';
import LogsToolbar from './LogsToolbar';
import {
  deriveFacets,
  matchesView,
  mergeBufferedLines,
  type BufferedLine,
} from './consoleLines';
import { activeFilterCount, completenessNotes, downloadFilename, sameQuery } from './format';
import { createLogsClient } from './logsApi';
import { useDebounced } from './useDebounced';
import type { AIWorkspaceHostPort } from './hostPort';
import type { LogPage, LogQuery, LogViewFilters } from './types';

export type LogsFeatureProps = {
  port: AIWorkspaceHostPort;
};

/** Live-tail cadence. */
const LIVE_POLL_MS = 5_000;

/**
 * How long the query must hold still before it reaches the network. Over the
 * whole query rather than the search box alone, so two boxes ticked in a second
 * cost one fetch.
 */
const QUERY_DEBOUNCE_MS = 300;

/**
 * Rows the console keeps. Real DOM nodes rather than a virtualised window, so
 * the buffer is capped for a long tail; oldest rows fall off first.
 */
const MAX_CONSOLE_LINES = 2000;

/** Records requested per poll. */
const PAGE_LIMIT = 100;

const INITIAL_QUERY: LogQuery = {
  rangeMinutes: 60,
  kinds: [],
  levels: [],
  searchPhrase: '',
  limit: PAGE_LIMIT,
  environment: '',
};

const INITIAL_VIEW: LogViewFilters = { projects: [] };

/** Until the first response says what retention really is. */
const ASSUMED_RETENTION_DAYS = 3;

/**
 * The extension's `render(port)` result: the organization's logs from `/logs`,
 * shown as a terminal-style console.
 *
 * Scope is the whole organization unless an environment is picked — that one is
 * a query parameter. Narrowing further, to a project, happens in the browser:
 * the observability API offers no filter for it.
 *
 * Nothing is committed by a button. A query filter updates `query` at once and
 * the debounced copy is what the fetch reads; the project filter is not a query
 * at all and only re-renders what is loaded.
 */
const LogsFeature: FC<LogsFeatureProps> = ({ port }) => {
  const { apiFetch, notify, orgHandle } = port;
  const client = useMemo(() => createLogsClient(apiFetch), [apiFetch]);

  const [query, setQuery] = useState<LogQuery>(INITIAL_QUERY);
  const debouncedQuery = useDebounced(query, QUERY_DEBOUNCE_MS, sameQuery);
  const [view, setView] = useState<LogViewFilters>(INITIAL_VIEW);
  const [buffer, setBuffer] = useState<BufferedLine[]>([]);
  const [page, setPage] = useState<LogPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [fetching, setFetching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [live, setLive] = useState(true);
  const [stale, setStale] = useState(false);
  const [environments, setEnvironments] = useState<string[]>([]);

  // Fetches overlap (a filter change during a live tick), so only the newest may
  // write state — a slower earlier response would append lines the filters no
  // longer describe.
  const loadSeqRef = useRef(0);
  /*
   * Set when the query changed: the rows on screen answer the old question, so
   * the next response replaces the buffer instead of merging into it. Cleared by
   * the response that wins, so a live tick landing first cannot consume it.
   */
  const replaceRef = useRef(false);

  const load = useCallback(
    async ({ silent = false }: { silent?: boolean } = {}) => {
      const seq = ++loadSeqRef.current;
      // Only a load the reader asked for shows as busy. A silent live poll
      // setting it would disable Refresh and blink the header spinner every five
      // seconds, and would leave that spinner meaning nothing when it is the one
      // signal that a filter change was heard.
      if (!silent) {
        setFetching(true);
        setLoading(true);
        setError(null);
      }
      try {
        const result = await client.fetchLogs(debouncedQuery);
        if (seq !== loadSeqRef.current) return;
        // Read BEFORE dispatching. React defers the updater below to the render
        // phase, so a ref read inside it would see whatever the ref holds by
        // then — here, always `false`.
        const replacing = replaceRef.current;
        replaceRef.current = false;
        setPage(result);
        setBuffer((previous) =>
          mergeBufferedLines(replacing ? [] : previous, result.list, MAX_CONSOLE_LINES)
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
        // Cleared by whichever load is still the newest, silent or not. Clearing
        // it only for the load that set it would strand the flag: a silent poll
        // starting mid-flight makes the foreground load stale, and then neither
        // would ever turn the spinner off.
        if (seq === loadSeqRef.current) {
          setLoading(false);
          setFetching(false);
        }
      }
    },
    [client, debouncedQuery]
  );

  // Fires on mount and on every settled query change — `load`'s identity tracks
  // `debouncedQuery`. The buffer is not emptied here: the old rows stay on screen
  // until the new ones land, so the console does not blink between filters.
  useEffect(() => {
    replaceRef.current = true;
    void load();
  }, [load]);

  // Fails soft: without the catalogue the Environment group falls back to the
  // environments seen in the loaded lines.
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

  // The buffer stays unfiltered so the facet counts do not collapse to whatever
  // is selected.
  const facets = useMemo(() => deriveFacets(buffer), [buffer]);
  const lines = useMemo(
    () => buffer.filter(({ entry }) => matchesView(entry, view)).map(({ line }) => line),
    [buffer, view]
  );

  // Everything the Filters button counts, including the search phrase — its box
  // lives outside the panel, but an empty console with a phrase typed is the
  // commonest way to end up filtered into nothing.
  const reset = () => {
    setQuery({ ...query, kinds: [], levels: [], environment: '', searchPhrase: '' });
    setView(INITIAL_VIEW);
  };

  const retentionDays = page?.window.retentionDays ?? ASSUMED_RETENTION_DAYS;
  const filterCount = activeFilterCount(query, view);
  const notes = [
    // `live` suppresses the truncation note only — see `completenessNotes`. The
    // window-clamped and type-filter notes hold whether or not the tail runs,
    // and a type filter that returns eight rows out of a hundred needs saying
    // most while the console is filling itself.
    ...(page ? completenessNotes(page, query.kinds.length === 1, live) : []),
    // The project filter runs in the browser, so a short console is not a quiet
    // organization. Say how much was hidden, or it reads as the whole answer.
    ...(lines.length < buffer.length
      ? [`${lines.length} of ${buffer.length} loaded lines match the project filter.`]
      : []),
  ];

  /** The project this row came from, for the "show only this" action. */
  const projectOf = (line: ConsoleLine) =>
    line.details?.find((detail) => detail.label === 'Project')?.value;

  return (
    <PageContent fullWidth>
      <Stack
        alignItems={{ sm: 'flex-start', xs: 'stretch' }}
        direction={{ sm: 'row', xs: 'column' }}
        justifyContent="space-between"
        spacing={2}
        sx={{ mb: 2 }}
      >
        <PageTitle sx={{ mb: 0 }}>
          <PageTitle.Header>Runtime logs</PageTitle.Header>
          <PageTitle.SubHeader>
            Every workload in this organization, including gateway traffic and gateway activity.
          </PageTitle.SubHeader>
        </PageTitle>
        <Chip
          icon={<Clock size={14} />}
          label={`Retained ${retentionDays} days`}
          size="small"
          sx={{ flexShrink: 0 }}
          variant="outlined"
        />
      </Stack>

      <LogsToolbar
        environments={environments}
        facets={facets}
        fetching={fetching}
        live={live}
        query={query}
        retentionDays={retentionDays}
        view={view}
        onLiveChange={setLive}
        onQueryChange={setQuery}
        onRefresh={() => void load()}
        onReset={reset}
        onViewChange={setView}
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
        busy={fetching}
        downloadFileName={() => downloadFilename(orgHandle)}
        emptyMessage={
          loading ? (
            'Loading logs…'
          ) : (
            <Stack alignItems="flex-start" spacing={1}>
              <Typography variant="inherit">
                No logs in this window. Try a wider time range, or clear a filter.
              </Typography>
              <Stack direction="row" spacing={1}>
                <Button disabled={fetching} size="small" onClick={() => void load()}>
                  Refresh
                </Button>
                {filterCount > 0 ? (
                  <Button size="small" onClick={reset}>
                    Clear filters
                  </Button>
                ) : null}
              </Stack>
            </Stack>
          )
        }
        label="Organization log output"
        lines={lines}
        live={live}
        newestFirst
        rowActions={(line) => {
          const project = projectOf(line);
          if (!project) return null;
          const only = view.projects.length === 1 && view.projects[0] === project;
          return (
            <Button
              onClick={() => setView({ ...view, projects: only ? [] : [project] })}
              size="small"
              variant="outlined"
            >
              {only ? 'Show every project' : `Show only ${project}`}
            </Button>
          );
        }}
        stale={stale}
        onCopyError={(message) => notify(message, 'error')}
      />
    </PageContent>
  );
};

export default LogsFeature;
