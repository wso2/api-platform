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

type BuildDTO = {
  buildId: string;
  description?: string;
  createdBy?: string;
  createdAt?: string;
};

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
 * The API's own record. Only its upstream is read: `main.url` is the backend the
 * API is defined against, which is what a deployment serves unless it is given
 * an endpoint of its own.
 */
type RestApiDTO = {
  upstream?: { main?: { url?: string } };
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
        description: dto.description,
        createdBy: dto.createdBy,
        createdAt: dto.createdAt,
      }));
    },

    /**
     * Deletes one of the API's builds, which is how room is made once an API is at
     * its build limit and deploying is refused.
     *
     * This goes straight to the API's own resource rather than through the project
     * path: a build belongs to the API, not to a pipeline, and the platform already
     * scopes the call to the caller's organization. Refused with 409 while a gateway
     * is serving the build; undeploying releases it. Deployments that ran the build
     * survive and stay redeployable from their own artifact, but stop reporting it,
     * so they can no longer be promoted onward.
     */
    async deleteBuild(buildId: string): Promise<void> {
      await apiFetch(
        'DELETE',
        `/rest-apis/${encodeURIComponent(apiHandle)}/builds/${encodeURIComponent(buildId)}`
      );
    },

    /**
     * The backend URL the API is defined against, which the deploy and promote
     * forms start from so a first deployment does not have to be typed out.
     * Absent when the API declares its upstream by reference rather than by URL.
     */
    async readApiEndpointUrl(): Promise<string | undefined> {
      const api = await apiFetch<RestApiDTO>('GET', `/rest-apis/${encodeURIComponent(apiHandle)}`);
      return api?.upstream?.main?.url;
    },

    /**
     * Deploys (or promotes) one build onto every gateway named, each with its own
     * endpoint. An environment runs a single build of an API at a time, so this is
     * one call rather than one per gateway: the backend puts the same build on all
     * of them and rolls every gateway back if any one fails.
     *
     * The gateways must include every gateway the API is already deployed on in
     * that environment; leaving one out is refused with 409.
     */
    async deploy(input: {
      environment: string;
      gateways: { gatewayId: string; endpointUrl?: string }[];
      fromEnvironment?: string;
      buildId?: string;
    }): Promise<void> {
      await apiFetch('POST', `${base}/deployments`, {
        environment: input.environment,
        gateways: input.gateways.map((gateway) => ({
          gatewayId: gateway.gatewayId,
          ...(gateway.endpointUrl ? { parameters: { productionEndpoint: gateway.endpointUrl } } : {}),
        })),
        ...(input.fromEnvironment ? { fromEnvironment: input.fromEnvironment } : {}),
        ...(input.buildId ? { buildId: input.buildId } : {}),
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
