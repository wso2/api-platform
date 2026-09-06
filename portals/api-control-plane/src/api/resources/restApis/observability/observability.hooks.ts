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

import { useQuery } from '@tanstack/react-query';

import { useApiScope } from '../../../core/scope';
import type {
  ObservabilityLogsScope,
  RestApiObservabilityLogsQuery,
  RestApiObservabilityTracesQuery,
} from './observability.endpoints';
import {
  restApiObservabilityQueries,
  type ObservabilityLogTailFilters,
} from './observability.queries';

/** Default live-tail poll cadence. */
export const LOG_TAIL_POLL_MS = 5000;

export const useRestApiObservabilityLogs = (
  restApiId: string | undefined,
  query: RestApiObservabilityLogsQuery,
  overrides: { orgId?: string } = {},
  enabled = true
) => {
  return useObservabilityLogs({ restApiId }, query, overrides, enabled);
};

export const useObservabilityLogs = (
  scope: ObservabilityLogsScope,
  query: RestApiObservabilityLogsQuery,
  overrides: { orgId?: string } = {},
  enabled = true
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...restApiObservabilityQueries.scopedLogs(org!, scope, query),
    enabled: Boolean(enabled && org),
  });
};

/**
 * Rolling tail of gateway logs, optionally polling for new records.
 *
 * `refetchIntervalInBackground` is left at its default of false, so a console
 * left open on a hidden tab stops polling until the tab is looked at again.
 */
export const useObservabilityLogTail = (
  scope: ObservabilityLogsScope,
  filters: ObservabilityLogTailFilters,
  options: {
    live?: boolean;
    enabled?: boolean;
    intervalMs?: number;
    orgId?: string;
  } = {}
) => {
  const { live = false, enabled = true, intervalMs = LOG_TAIL_POLL_MS } =
    options;
  const { org } = useApiScope({ orgId: options.orgId });

  return useQuery({
    ...restApiObservabilityQueries.tail(org!, scope, filters),
    enabled: Boolean(enabled && org),
    refetchInterval: live ? intervalMs : false,
  });
};

export const useRestApiObservabilityTraces = (
  restApiId: string | undefined,
  query: RestApiObservabilityTracesQuery,
  enabled = true
) => {
  const { org } = useApiScope();
  return useQuery({
    ...restApiObservabilityQueries.traces(org!, restApiId!, query),
    enabled: Boolean(enabled && org && restApiId),
  });
};

export const useRestApiObservabilityTrace = (
  restApiId: string | undefined,
  traceId: string | undefined,
  query: Pick<RestApiObservabilityTracesQuery, 'startTime' | 'endTime' | 'environment'>,
  enabled = true
) => {
  const { org } = useApiScope();
  return useQuery({
    ...restApiObservabilityQueries.trace(org!, restApiId!, traceId!, query),
    enabled: Boolean(enabled && org && restApiId && traceId),
  });
};
