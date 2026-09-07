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

import type { ApiFetch } from './hostPort';
import type { Build, DeploymentStatus, Environment, Gateway, GatewayHealth } from './types';

/** Wire shapes. These mirror the deployment endpoints field for field. */
type GatewayDeploymentDTO = {
  gatewayId: string;
  deploymentId?: string;
  status?: DeploymentStatus;
  statusReason?: string;
  createdAt?: string;
  buildId?: string;
  endpointUrl?: string;
  isDefault?: boolean;
};

type StageDTO = {
  environment: string;
  gateways?: GatewayDeploymentDTO[];
};

type BuildDTO = { buildId: string; createdBy?: string; createdAt?: string };

/**
 * The gateways resource, which owns a gateway's identity and health. The
 * deployment endpoints key everything by gateway handle and say nothing about
 * the gateway itself, so the name, host and whether it is up are read from here.
 */
type ManagedGatewayDTO = {
  id: string;
  displayName?: string;
  host?: string;
  isActive?: boolean;
};

/**
 * The deployment data client, built from the host-injected `apiFetch`.
 *
 * Everything is addressed by project and API handle, and the server resolves the
 * pipeline, its environments and their gateways — so this client never assembles
 * a pipeline itself, and the rules the server enforces cannot be bypassed here.
 */
export function createDeployClient(apiFetch: ApiFetch, projectHandle: string, apiHandle: string) {
  const base = `/projects/${encodeURIComponent(projectHandle)}/apis/${encodeURIComponent(apiHandle)}`;

  return {
    /**
     * The pipeline's environments in promotion order, each with every gateway it
     * has. Identity and health come from the gateways resource; a gateway it does
     * not cover falls back to its handle and is treated as up, so a failure to
     * read it cannot make the page refuse to deploy.
     */
    async listEnvironments(): Promise<Environment[]> {
      const [stages, gateways] = await Promise.all([
        apiFetch<{ list?: StageDTO[] }>('GET', `${base}/deployments`),
        apiFetch<{ list?: ManagedGatewayDTO[] }>('GET', '/managed-gateways').catch(() => undefined),
      ]);

      const known = new Map<string, ManagedGatewayDTO>();
      for (const gateway of gateways?.list ?? []) known.set(gateway.id, gateway);

      return (stages?.list ?? []).map((stage) => ({
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
    },

    /** The API's builds, newest first. */
    async listBuilds(): Promise<Build[]> {
      const response = await apiFetch<{ list?: BuildDTO[] }>('GET', `${base}/builds`);
      return (response?.list ?? []).map((dto) => ({
        buildId: dto.buildId,
        createdBy: dto.createdBy,
        createdAt: dto.createdAt,
      }));
    },

    /**
     * Deploys to one gateway of an environment with the endpoint it should serve.
     *
     * Deploying without a `buildId` snapshots the API and deploys the new build.
     * Supplying `buildId` deploys that existing build. `fromEnvironment` promotes
     * instead, carrying that environment's build forward untouched.
     */
    async deploy(input: {
      environment: string;
      gatewayId: string;
      endpointUrl?: string;
      fromEnvironment?: string;
      buildId?: string;
    }): Promise<void> {
      await apiFetch('POST', `${base}/deployments`, {
        environment: input.environment,
        gatewayId: input.gatewayId,
        ...(input.fromEnvironment ? { fromEnvironment: input.fromEnvironment } : {}),
        ...(input.buildId ? { buildId: input.buildId } : {}),
        ...(input.endpointUrl ? { parameters: { productionEndpoint: input.endpointUrl } } : {}),
      });
    },

    /** Stops serving one deployment on one gateway; the rest are untouched. */
    async undeploy(environment: string, gatewayId: string, deploymentId: string): Promise<void> {
      const query = `?environment=${encodeURIComponent(environment)}&gatewayId=${encodeURIComponent(gatewayId)}`;
      await apiFetch(
        'POST',
        `${base}/deployments/${encodeURIComponent(deploymentId)}/undeploy${query}`
      );
    },
  };
}

export type DeployClient = ReturnType<typeof createDeployClient>;
