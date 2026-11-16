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

import { useQuery } from '@tanstack/react-query';

import { runtimeConfig } from '../../../config/runtime';
import { useApiScope } from '../../core/scope';
import type { CloudArtifactKind } from './cloudDeployments.endpoints';
import { cloudDeploymentQueries } from './cloudDeployments.queries';

const POLL_MS = 4000;

/**
 * An artifact's deployments as the project's pipeline reports them, polling
 * while any of them is mid-transition.
 *
 * Cloud only (`cloudProxyEnabled`): the route exists only behind the cloud
 * pipeline. Elsewhere the query is disabled (`data` stays undefined) and the
 * caller keeps the platform's own deployment list.
 */
export function useCloudDeployments(
  kind: CloudArtifactKind,
  projectHandle: string | undefined,
  apiHandle: string | undefined,
) {
  const { org } = useApiScope();
  // `cloudProxyEnabled` is on when the server has the cloud upstream configured, which
  // is what puts the pipeline route behind it; a standalone console has neither.
  const enabled = runtimeConfig.cloudProxyEnabled && Boolean(org && projectHandle && apiHandle);

  return useQuery({
    ...cloudDeploymentQueries.list(org!, kind, projectHandle!, apiHandle!),
    enabled,
    refetchInterval: (query) =>
      query.state.data?.some((d) => d.status === 'DEPLOYING' || d.status === 'UNDEPLOYING')
        ? POLL_MS
        : false,
  });
}
