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
import type { Schema } from '../../core/spec';

/**
 * Transport layer for an artifact's deployments as the cloud pipeline reports
 * them: `/projects/{projectHandle}/apis/{apiHandle}/deployments?kind=…`, the
 * call the cloud Deploy page reads.
 *
 * The route exists only behind the WSO2 Cloud pipeline, so it has no operation
 * in `platform-api/resources/openapi.yaml`. That makes this resource, like
 * `resources/policyHub`, an exception to the spec-derived rule in
 * `src/api/README.md`:
 *
 * 1. **No spec-derived wire types.** The stage shape below is hand-written, and
 *    the response is normalized here into the platform's own `Deployment`, so
 *    the Overview pages can use it in place of the platform's deployment list.
 * 2. **No `operationName`.** That string is also the permission key, and
 *    `src/permissions` has no spec operation to look this route up by; a
 *    made-up name would only be reported as scope-map drift on a 403.
 *
 * What it does not opt out of is `core/http`: the route sits behind the same
 * proxy as every other call, so it gets the same `X-Org-Id`, cancellation and
 * `ApiError` contract.
 */

/** The platform's deployment record, which the pipeline's answer is mapped onto. */
export type Deployment = Schema<'DeploymentResponse'>;

/** The artifact kinds the pipeline route answers for. */
export type CloudDeploymentKind = 'RestApi' | 'GraphQLApi';

/** One gateway's state within a pipeline stage, as the wire carries it. */
type StageGateway = {
  gatewayId: string;
  deploymentId?: string;
  status?: Deployment['status'] | 'NOT_DEPLOYED';
  createdAt?: string;
};

type Stage = { environment: string; gateways?: StageGateway[] };

type StageListResponse = { list?: Stage[] };

const collectionPath = (projectHandle: string, apiHandle: string): string =>
  `/projects/${encodeURIComponent(projectHandle)}/apis/${encodeURIComponent(apiHandle)}/deployments`;

/**
 * Flattens every stage's gateways into one deployment list, leaving out the
 * gateways with no deployment (`NOT_DEPLOYED`). Where the wire carries no
 * deployment id or timestamp, the gateway id and `''` stand in.
 *
 * Only the fields the Overview pages read are filled, hence the cast: the rest
 * of `Deployment` has no counterpart on this route.
 */
const toDeployments = (response: StageListResponse | undefined): Deployment[] =>
  (response?.list ?? [])
    .flatMap((stage) => stage.gateways ?? [])
    .filter((gateway) => gateway.status && gateway.status !== 'NOT_DEPLOYED')
    .map((gateway) => ({
      gatewayId: gateway.gatewayId,
      deploymentId: gateway.deploymentId ?? gateway.gatewayId,
      status: gateway.status,
      createdAt: gateway.createdAt ?? '',
    })) as unknown as Deployment[];

export const listCloudDeployments = async (
  projectHandle: string,
  apiHandle: string,
  kind: CloudDeploymentKind,
  options?: RequestOptions
): Promise<Deployment[]> =>
  toDeployments(
    await http.get<StageListResponse>(collectionPath(projectHandle, apiHandle), {
      ...options,
      query: { kind },
    })
  );
