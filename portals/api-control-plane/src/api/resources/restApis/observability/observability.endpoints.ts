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

import { http, type RequestOptions } from '../../../core/http';

/** Log levels accepted by the cloud observability extension. */
export type ObservabilityLogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

/** Observer log record returned by the cloud Platform API extension. */
export type RestApiObservabilityLog = {
  timestamp?: string;
  log?: string;
  level?: string;
  logLevel?: string;
  metadata?: Record<string, unknown>;
  [key: string]: unknown;
};

export type RestApiObservabilityLogsQuery = {
  startTime: string;
  endTime: string;
  limit?: number;
  query?: string;
  logLevels?: ObservabilityLogLevel[];
  component?: string;
  environment?: string;
  project?: string;
};

export type ObservabilityLogsScope = {
  projectId?: string;
  restApiId?: string;
};

export type RestApiObservabilityLogsPage = {
  items: RestApiObservabilityLog[];
  pagination: {
    limit: number;
    nextCursor?: string | null;
  };
};

export type RestApiObservabilityTrace = {
  traceId: string;
  traceName?: string;
  rootSpanName?: string;
  startTime?: string;
  endTime?: string;
  durationNs?: number;
  spanCount?: number;
  hasErrors?: boolean;
};

export type RestApiObservabilitySpan = {
  spanId?: string;
  parentSpanId?: string;
  spanName?: string;
  spanKind?: string;
  startTime?: string;
  endTime?: string;
  durationNs?: number;
  status?: { code?: string; message?: string };
  attributes?: Record<string, unknown>;
  resourceAttributes?: Record<string, unknown>;
};

export type RestApiObservabilityTracesQuery = Pick<
  RestApiObservabilityLogsQuery,
  'startTime' | 'endTime' | 'limit' | 'query' | 'environment'
>;

export type RestApiObservabilityTracesPage = {
  items: RestApiObservabilityTrace[];
  pagination: { limit: number; nextCursor?: string | null };
};

export type RestApiObservabilityTraceDetail = {
  spans: RestApiObservabilitySpan[];
  total: number;
};

const logsPath = (restApiId: string): string =>
  `/rest-apis/${encodeURIComponent(restApiId)}/observability/logs`;

const scopedLogsPath = (scope: ObservabilityLogsScope): string => {
  if (scope.restApiId) return logsPath(scope.restApiId);
  if (scope.projectId) {
    return `/projects/${encodeURIComponent(scope.projectId)}/observability/logs`;
  }
  return '/observability/logs';
};

/**
 * Queries the cloud-only observability extension for one REST API. This
 * endpoint is intentionally hand-typed: it is supplied by the cloud wrapper
 * and therefore is not part of api-platform's generated base OpenAPI types.
 */
export const listRestApiObservabilityLogs = async (
  restApiId: string,
  query: RestApiObservabilityLogsQuery,
  options?: RequestOptions
): Promise<RestApiObservabilityLogsPage> => {
  return listObservabilityLogs({ restApiId }, query, options);
};

/** Queries gateway logs at organization, project, or API scope. */
export const listObservabilityLogs = async (
  scope: ObservabilityLogsScope,
  query: RestApiObservabilityLogsQuery,
  options?: RequestOptions
): Promise<RestApiObservabilityLogsPage> => {
  const search = query.query?.trim();

  return http.get<RestApiObservabilityLogsPage>(scopedLogsPath(scope), {
    ...options,
    operationName: 'QueryRESTAPIObservabilityLogs',
    query: {
      startTime: query.startTime,
      endTime: query.endTime,
      limit: query.limit ?? 100,
      query: search || undefined,
      logLevel: query.logLevels,
      component: query.component,
      environment: query.environment,
      project: query.project,
    },
  });
};

export const listRestApiObservabilityTraces = async (
  restApiId: string,
  query: RestApiObservabilityTracesQuery,
  options?: RequestOptions
): Promise<RestApiObservabilityTracesPage> => {
  const search = query.query?.trim();
  return http.get<RestApiObservabilityTracesPage>(
    `/rest-apis/${encodeURIComponent(restApiId)}/observability/traces`,
    {
      ...options,
      operationName: 'QueryRESTAPIObservabilityTraces',
      query: {
        startTime: query.startTime,
        endTime: query.endTime,
        limit: query.limit ?? 100,
        query: search || undefined,
        environment: query.environment,
      },
    }
  );
};

export const getRestApiObservabilityTrace = async (
  restApiId: string,
  traceId: string,
  query: Pick<RestApiObservabilityTracesQuery, 'startTime' | 'endTime' | 'environment'>,
  options?: RequestOptions
): Promise<RestApiObservabilityTraceDetail> =>
  http.get<RestApiObservabilityTraceDetail>(
    `/rest-apis/${encodeURIComponent(restApiId)}/observability/traces/${encodeURIComponent(traceId)}`,
    {
      ...options,
      operationName: 'GetRESTAPIObservabilityTrace',
      query,
    }
  );
