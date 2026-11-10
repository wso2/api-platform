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

import { queryOptions } from '@tanstack/react-query';

import { staleTimes } from '../../core/queryClient';
import { createResourceKeys, type OrgScope } from '../../core/queryKeys';
import { listCloudDeployments, type CloudDeploymentKind } from './cloudDeployments.endpoints';

/**
 * Keys of its own rather than `restApiKeys.child(...)`: the route is addressed
 * by project and API handle, not the platform's API id, and serves GraphQL APIs
 * too. It also keeps the platform's deploy mutations, which invalidate their
 * API's `deployments` children, from reaching this entry.
 */
export const cloudDeploymentKeys = createResourceKeys('cloudDeployments');

export const cloudDeploymentQueries = {
  list: (org: OrgScope, projectHandle: string, apiHandle: string, kind: CloudDeploymentKind) =>
    queryOptions({
      queryKey: cloudDeploymentKeys.list(org, { project: projectHandle, api: apiHandle, kind }),
      // `signal` comes from TanStack Query: navigating away aborts the
      // in-flight request instead of letting it land in the cache.
      queryFn: ({ signal }) =>
        listCloudDeployments(projectHandle, apiHandle, kind, { orgId: org, signal }),
      // The client default, which this query has always run on; the hook's
      // poll is what keeps a deployment that is still settling live.
      staleTime: staleTimes.standard,
    }),
};
