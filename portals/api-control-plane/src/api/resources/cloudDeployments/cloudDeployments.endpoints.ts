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

import { http, type RequestOptions } from '../../core/http';
import type { Deployment } from '../restApis/deployments';

/**
 * Transport for the cloud pipeline's per-artifact deployments
 * (`/projects/{p}/apis/{a}/deployments`) — the call the Deploy page reads, so
 * the Overview and the Deploy page cannot disagree.
 *
 * Not spec-derived: the route exists only behind the cloud upstream, not in
 * platform-api's OpenAPI document.
 */

/** The kinds of artifact the pipeline route reports on. */
export type CloudArtifactKind = 'RestApi' | 'GraphQLApi';

type StageGateway = {
  gatewayId: string;
  deploymentId?: string;
  status?: Deployment['status'] | 'NOT_DEPLOYED';
  createdAt?: string;
};

type Stage = { environment: string; gateways?: StageGateway[] };

const collectionPath = (projectHandle: string, apiHandle: string): string =>
  `/projects/${encodeURIComponent(projectHandle)}/apis/${encodeURIComponent(apiHandle)}/deployments`;

/**
 * Every environment's gateways, flattened into the platform's `Deployment`
 * shape. Gateways with no deployment (`NOT_DEPLOYED`) are left out.
 */
export const listCloudDeployments = async (
  kind: CloudArtifactKind,
  projectHandle: string,
  apiHandle: string,
  options?: RequestOptions,
): Promise<Deployment[]> => {
  const response = await http.get<{ list?: Stage[] }>(collectionPath(projectHandle, apiHandle), {
    ...options,
    query: { kind },
  });
  return (response?.list ?? [])
    .flatMap((stage) => stage.gateways ?? [])
    .filter((gateway) => gateway.status && gateway.status !== 'NOT_DEPLOYED')
    .map((gateway) => ({
      gatewayId: gateway.gatewayId,
      deploymentId: gateway.deploymentId ?? gateway.gatewayId,
      status: gateway.status,
      createdAt: gateway.createdAt ?? '',
    })) as unknown as Deployment[];
};
