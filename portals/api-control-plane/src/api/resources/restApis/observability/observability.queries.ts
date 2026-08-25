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

import { queryOptions } from '@tanstack/react-query';

import { staleTimes } from '../../../core/queryClient';
import { scopeKey, type OrgScope } from '../../../core/queryKeys';
import { restApiKeys } from '../restApis.queries';
import {
  listObservabilityLogs,
  listRestApiObservabilityLogs,
  type ObservabilityLogLevel,
  type ObservabilityLogsScope,
  type RestApiObservabilityLogsQuery,
} from './observability.endpoints';

/**
 * Filters for a rolling log tail. Carries a window *length* instead of absolute
 * bounds so the query key stays stable while the window slides.
 */
export type ObservabilityLogTailFilters = {
  durationMinutes: number;
  limit?: number;
  query?: string;
  logLevels?: ObservabilityLogLevel[];
  component?: string;
  environment?: string;
  project?: string;
};

export const restApiObservabilityQueries = {
  /**
   * Rolling tail of gateway logs at organization, project, or API scope.
   *
   * The time window is resolved inside `queryFn` rather than baked into the key:
   * a key carrying absolute bounds would allocate a fresh cache entry on every
   * poll, so a live tail would leak entries and never reuse its own data. With
   * the window derived per fetch, one key refetches against an advancing `now`.
   */
  tail: (
    org: OrgScope,
    scope: ObservabilityLogsScope,
    filters: ObservabilityLogTailFilters
  ) =>
    queryOptions({
      queryKey: [...scopeKey(org), 'observabilityLogTail', scope, filters],
      queryFn: ({ signal }) => {
        const end = new Date();
        return listObservabilityLogs(
          scope,
          {
            startTime: new Date(
              end.getTime() - filters.durationMinutes * 60 * 1000
            ).toISOString(),
            endTime: end.toISOString(),
            limit: filters.limit,
            query: filters.query,
            logLevels: filters.logLevels,
            component: filters.component,
            environment: filters.environment,
            project: filters.project,
          },
          { orgId: org, signal }
        );
      },
      staleTime: staleTimes.realtime,
    }),
  scopedLogs: (
    org: OrgScope,
    scope: ObservabilityLogsScope,
    query: RestApiObservabilityLogsQuery
  ) =>
    queryOptions({
      queryKey: [...scopeKey(org), 'observabilityLogs', scope, query],
      queryFn: ({ signal }) =>
        listObservabilityLogs(scope, query, { orgId: org, signal }),
      staleTime: staleTimes.realtime,
    }),
  logs: (
    org: OrgScope,
    restApiId: string,
    query: RestApiObservabilityLogsQuery
  ) =>
    queryOptions({
      queryKey: restApiKeys.child(org, restApiId, 'observabilityLogs', query),
      queryFn: ({ signal }) =>
        listRestApiObservabilityLogs(restApiId, query, {
          orgId: org,
          signal,
        }),
      staleTime: staleTimes.realtime,
    }),
};
