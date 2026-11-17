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
import { listCloudDeployments, type CloudArtifactKind } from './cloudDeployments.endpoints';

/** Org-scoped keys, so an org switch never serves another org's pipeline state. */
export const cloudDeploymentKeys = createResourceKeys('cloudDeployments');

export const cloudDeploymentQueries = {
  list: (org: OrgScope, kind: CloudArtifactKind, projectHandle: string, apiHandle: string) =>
    queryOptions({
      queryKey: cloudDeploymentKeys.list(org, { apiHandle, kind, projectHandle }),
      queryFn: ({ signal }) =>
        listCloudDeployments(kind, projectHandle, apiHandle, { orgId: org, signal }),
      staleTime: staleTimes.realtime,
    }),
};
