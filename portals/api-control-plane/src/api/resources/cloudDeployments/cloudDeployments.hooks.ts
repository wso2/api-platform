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

import { runtimeConfig } from '@/config/runtime';
import { useApiScope } from '../../core/scope';
import { hasTransitioningDeployment } from '../restApis/deployments/deployments.queries';
import type { CloudDeploymentKind } from './cloudDeployments.endpoints';
import { cloudDeploymentQueries } from './cloudDeployments.queries';

/** Fixed poll interval while any deployment is still DEPLOYING or UNDEPLOYING. */
const POLL_MS = 4000;

/**
 * An artifact's deployments as the project's pipeline reports them, the call
 * the Deploy page reads, so the Overview and the Deploy page cannot disagree.
 *
 * Cloud only (`cloudProxyEnabled`): the route exists only behind the cloud
 * pipeline. Elsewhere the query is disabled (`data` stays undefined) and the
 * caller keeps the platform's own deployment list. Gateways with no deployment
 * (`NOT_DEPLOYED`) are left out.
 */
export function useCloudDeployments(
  kind: CloudDeploymentKind,
  projectHandle: string | undefined,
  apiHandle: string | undefined
) {
  const { org } = useApiScope();
  // `cloudProxyEnabled` is on when the server has the cloud upstream configured, which
  // is what puts the pipeline route behind it; a standalone console has neither.
  const enabled = runtimeConfig.cloudProxyEnabled && Boolean(org && projectHandle && apiHandle);

  return useQuery({
    ...cloudDeploymentQueries.list(org!, projectHandle!, apiHandle!, kind),
    enabled,
    refetchInterval: (query) => (hasTransitioningDeployment(query.state.data) ? POLL_MS : false),
  });
}
