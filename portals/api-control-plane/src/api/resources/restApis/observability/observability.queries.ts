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
import type { OrgScope } from '../../../core/queryKeys';
import { restApiKeys } from '../restApis.queries';
import {
  listRestApiObservabilityLogs,
  type RestApiObservabilityLogsQuery,
} from './observability.endpoints';

export const restApiObservabilityQueries = {
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
