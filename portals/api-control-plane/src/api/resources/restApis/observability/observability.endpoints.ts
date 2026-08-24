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
};

export type RestApiObservabilityLogsPage = {
  items: RestApiObservabilityLog[];
  pagination: {
    limit: number;
    nextCursor?: string | null;
  };
};

const logsPath = (restApiId: string): string =>
  `/rest-apis/${encodeURIComponent(restApiId)}/observability/logs`;

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
  const search = query.query?.trim();

  return http.get<RestApiObservabilityLogsPage>(logsPath(restApiId), {
    ...options,
    operationName: 'QueryRESTAPIObservabilityLogs',
    query: {
      startTime: query.startTime,
      endTime: query.endTime,
      limit: query.limit ?? 100,
      query: search || undefined,
      logLevel: query.logLevels,
    },
  });
};
