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

import { http } from '@/api/core/http';
import { useApiScope } from '@/api/core/scope';
import type { Deployment } from '@/api/resources/restApis/deployments';
import { runtimeConfig } from '@/config/runtime';

type StageGateway = {
  gatewayId: string;
  deploymentId?: string;
  status?: Deployment['status'] | 'NOT_DEPLOYED';
  createdAt?: string;
};

type Stage = { environment: string; gateways?: StageGateway[] };

const POLL_MS = 4000;

/**
 * An artifact's deployments as the project's pipeline reports them
 * (`/projects/{p}/apis/{a}/deployments`), the call the Deploy page reads, so the
 * Overview and the Deploy page cannot disagree.
 *
 * Cloud only (`cloudProxyEnabled`): the route exists only behind the cloud pipeline. Elsewhere the
 * query is disabled (`data` stays undefined) and the caller keeps the platform's
 * own deployment list. Gateways with no deployment (`NOT_DEPLOYED`) are left out.
 */
export function useCloudDeployments(
  kind: 'RestApi' | 'GraphQLApi',
  projectHandle: string | undefined,
  apiHandle: string | undefined,
) {
  const { org } = useApiScope();
  // `cloudProxyEnabled` is on when the server has the cloud upstream configured, which
  // is what puts the pipeline route behind it; a standalone console has neither.
  const enabled = runtimeConfig.cloudProxyEnabled && Boolean(org && projectHandle && apiHandle);

  return useQuery({
    queryKey: ['cloud-artifact-deployments', org, projectHandle, kind, apiHandle],
    enabled,
    queryFn: async ({ signal }): Promise<Deployment[]> => {
      const path = `/projects/${encodeURIComponent(projectHandle!)}/apis/${encodeURIComponent(
        apiHandle!,
      )}/deployments?kind=${kind}`;
      const response = await http.get<{ list?: Stage[] }>(path, { orgId: org, signal });
      return (response?.list ?? [])
        .flatMap((stage) => stage.gateways ?? [])
        .filter((gateway) => gateway.status && gateway.status !== 'NOT_DEPLOYED')
        .map((gateway) => ({
          gatewayId: gateway.gatewayId,
          deploymentId: gateway.deploymentId ?? gateway.gatewayId,
          status: gateway.status,
          createdAt: gateway.createdAt ?? '',
        })) as unknown as Deployment[];
    },
    refetchInterval: (query) =>
      query.state.data?.some((d) => d.status === 'DEPLOYING' || d.status === 'UNDEPLOYING')
        ? POLL_MS
        : false,
  });
}
