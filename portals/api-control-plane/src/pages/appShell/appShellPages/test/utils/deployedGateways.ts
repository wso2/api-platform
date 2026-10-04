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

import type { Deployment } from '@/api/resources/restApis/deployments';
import type { RestApiGateway } from '@/api/resources/restApis/apiGateways/apiGateways.endpoints';

/**
 * Which gateways an API can actually be tested against, most recently deployed
 * first.
 *
 * A gateway the API is *not* deployed to has no working endpoint for it, so
 * offering it in the console's picker would produce requests that 404 for a
 * reason the user cannot see. Deployment state is the filter.
 */
export const deployedGateways = (
  gateways: readonly RestApiGateway[],
  deployments: readonly Deployment[],
): RestApiGateway[] => {
  const latestByGateway = new Map<string, number>();

  deployments
    .filter((deployment) => deployment.status === 'DEPLOYED')
    .forEach((deployment) => {
      // A missing `createdAt` sorts last rather than throwing off the ordering
      // with a NaN comparison.
      const time = new Date(deployment.createdAt || 0).getTime();
      const current = latestByGateway.get(deployment.gatewayId);
      if (current === undefined || time > current) {
        latestByGateway.set(deployment.gatewayId, time);
      }
    });

  return gateways
    .filter((gateway) => gateway.isDeployed || latestByGateway.has(gateway.id ?? ''))
    .slice()
    .sort(
      (first, second) =>
        (latestByGateway.get(second.id ?? '') || 0) - (latestByGateway.get(first.id ?? '') || 0),
    );
};
