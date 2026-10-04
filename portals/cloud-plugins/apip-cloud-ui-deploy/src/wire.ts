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

import type { DeploymentStatus, Environment, Gateway, GatewayHealth } from './types';

/**
 * The deployment wire shapes and the one mapping onto the page's own types.
 *
 * Two clients read these routes — an artifact deployed through its project's
 * pipeline, and one deployed across the organization's environments — and they
 * report an environment and a gateway identically. Keeping the shapes and the
 * mapping here is what makes that true by construction rather than by two copies
 * agreeing.
 */

/** One gateway's deployment of an artifact. Mirrors the endpoint field for field. */
export type GatewayDeploymentDTO = {
  gatewayId: string;
  deploymentId?: string;
  status?: DeploymentStatus;
  statusReason?: string;
  createdAt?: string;
  buildId?: string;
  endpointUrl?: string;
  isDefault?: boolean;
};

/** One environment and the artifact's state on each of its gateways. */
export type StageDTO = {
  environment: string;
  gateways?: GatewayDeploymentDTO[];
};

/**
 * The gateways resource, which owns a gateway's identity and health. The
 * deployment endpoints key everything by gateway handle and say nothing about
 * the gateway itself, so the name, host and whether it is up are read from here.
 */
export type ManagedGatewayDTO = {
  id: string;
  displayName?: string;
  host?: string;
  isActive?: boolean;
};

/**
 * The gateways resource path narrowed to the kinds of gateway an artifact runs
 * on, so the page asks for the gateways it can deploy to rather than pulling back
 * every gateway in the organization and dropping most of them here.
 */
export function managedGatewaysPath(gatewayTypes: readonly string[]): string {
  if (gatewayTypes.length === 0) return '/managed-gateways';
  const types = gatewayTypes
    .map((type) => `functionalityType=${encodeURIComponent(type)}`)
    .join('&');
  return `/managed-gateways?${types}`;
}

/**
 * Environments as the page reads them, in the order the server returned them.
 *
 * Identity and health come from the gateways resource; a gateway it does not
 * cover falls back to its handle and is treated as up, so a failure to read it
 * cannot make the page refuse to deploy.
 */
export function toEnvironments(
  stages: StageDTO[] | undefined,
  gateways: ManagedGatewayDTO[] | undefined
): Environment[] {
  const known = new Map<string, ManagedGatewayDTO>();
  for (const gateway of gateways ?? []) known.set(gateway.id, gateway);

  return (stages ?? []).map((stage) => ({
    name: stage.environment,
    gateways: (stage.gateways ?? []).map((dto): Gateway => {
      const gateway = known.get(dto.gatewayId);
      const health: GatewayHealth = gateway?.isActive === false ? 'inactive' : 'active';
      return {
        id: dto.gatewayId,
        name: gateway?.displayName || dto.gatewayId,
        host: gateway?.host,
        health,
        status: dto.status ?? 'NOT_DEPLOYED',
        isDefault: dto.isDefault,
        deploymentId: dto.deploymentId,
        buildId: dto.buildId,
        deployedAt: dto.createdAt,
        endpointUrl: dto.endpointUrl,
        statusReason: dto.statusReason,
      };
    }),
  }));
}
