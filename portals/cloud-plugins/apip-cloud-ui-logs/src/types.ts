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

/** Whether a line is one proxied request or a workload talking about itself. */
export type LogKind = 'access' | 'operational';

export type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

export type LogEntry = {
  /** RFC 3339. Empty when the backend's value could not be parsed. */
  timestamp: string;
  /** Exactly as the workload wrote it — an access log arrives as a JSON string. */
  log: string;
  level?: string;
  kind: LogKind;
  /** Names the source column when the component name is absent. */
  podName?: string;
  containerName?: string;
  componentName?: string;
  projectName?: string;
  environment?: string;
};

/**
 * The range actually searched, which is not always the one asked for, plus the
 * horizon that decides it. `retentionDays` is served so the picker never
 * hardcodes a number that has already changed once.
 */
export type LogWindow = {
  startTime: string;
  endTime: string;
  clamped: boolean;
  retentionDays: number;
};

export type LogPage = {
  /** Entries in `list`, after the kind filter. */
  count: number;
  list: LogEntry[];
  window: LogWindow;
  /** Entries the backend returned, before the kind filter. */
  fetched: number;
  /** The backend's own count of matching lines, capped at 1000. */
  total: number;
  /** More matched than this response carries. No cursor to follow, so the UI
   * says so rather than offering a "load more". */
  truncated: boolean;
};

/** What the toolbar can ask for. Absent fields take the server's defaults. */
export type LogQuery = {
  /** Minutes back from now. The window is computed at fetch time, not pinned. */
  rangeMinutes: number;
  /**
   * Which kinds to show. Empty means every kind.
   *
   * A list because the panel offers a checkbox each, but the endpoint takes one
   * `kind` — so a selection of two or more is the same request as none, and only
   * a single choice narrows it. `buildLogsQuery` is where that is decided.
   */
  kinds: LogKind[];
  levels: LogLevel[];
  searchPhrase: string;
  limit: number;
  /** Environment name, sent to the query. Empty means every environment. */
  environment: string;
};

/**
 * Narrowing applied to the lines already fetched. Not query parameters: the
 * endpoint selects no project, so sending one would claim a precision the query
 * lacks. Changing these re-renders; it never refetches.
 */
export type LogViewFilters = {
  projects: string[];
};

/** One option in the filter panel: what it is, and how much of the buffer it is. */
export type Facet = { value: string; label: string; count: number };

/**
 * What the filter panel offers, counted over the lines currently loaded.
 *
 * Read off the buffer rather than a catalogue, so a value with nothing in this
 * window is not offered — picking it could only produce an empty console. The
 * counts are of loaded lines for the same reason, and the panel says so.
 *
 * `environments` is the fallback for the Environment group, which prefers the
 * organization's real list.
 */
export type LogFacets = {
  kinds: Facet[];
  projects: Facet[];
  environments: Facet[];
  levels: Facet[];
};

export type EnvironmentSummary = { id: string; name: string };
