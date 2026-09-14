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

/** The `kind` filter, which adds "no filter" to the two real kinds. */
export type LogKindFilter = LogKind | 'all';

export type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

export type LogEntry = {
  /** RFC 3339. Empty when the backend's value could not be parsed. */
  timestamp: string;
  /** Exactly as the workload wrote it — an access log arrives as a JSON string. */
  log: string;
  level?: string;
  kind: LogKind;
  /** The only way to tell one gateway from another. Filtered in the browser:
   * the observability API has no podName filter. */
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
  kind: LogKindFilter;
  levels: LogLevel[];
  searchPhrase: string;
  limit: number;
  /** Environment name, sent to the query. Empty means every environment. */
  environment: string;
};

/**
 * Narrowing applied to the lines already fetched. Not query parameters: the
 * endpoint selects neither a project nor a pod, so sending them would claim a
 * precision the query lacks. The toolbar says so.
 */
export type LogViewFilters = {
  project: string;
  pod: string;
};

/**
 * What the view filters offer: the values present in the loaded lines. Derived
 * from the buffer, not a catalogue — a project with nothing in this window
 * cannot be picked, and picking it could only yield an empty console.
 *
 * `environments` is the fallback for the Environment select, which prefers the
 * organization's real list.
 */
export type LogFacets = {
  projects: string[];
  pods: string[];
  /** Pods seen under each project, so picking a project narrows the next list. */
  podsByProject: Record<string, string[]>;
  environments: string[];
};

export type EnvironmentSummary = { id: string; name: string };
