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
import type { Build, DeploymentParameter, DeploymentStatus, Environment, Gateway } from './types';

/** Wire shapes returned by the deployment endpoints. */
type GatewayDeploymentDTO = {
  gatewayId: string;
  deploymentId?: string;
  name?: string;
  status?: string;
  statusReason?: string;
  createdAt?: string;
  buildId?: string;
};

type BuildDTO = { buildId: string; createdBy?: string; createdAt?: string };

type StageDTO = {
  environment: string;
  buildId?: string;
  gateways?: GatewayDeploymentDTO[];
};

type ParameterDTO = {
  name: string;
  label?: string;
  description?: string;
  type?: string;
  value?: string;
};

/**
 * The gateway list, used only to label a gateway. The deployment endpoints key
 * everything by gateway handle; this supplies the display name a person expects
 * to see instead.
 */
type ManagedGatewayDTO = { id: string; displayName?: string; host?: string };

/** Statuses the API can report. Anything else is treated as not deployed. */
const KNOWN_STATUSES = new Set<DeploymentStatus>([
  'DEPLOYED',
  'DEPLOYING',
  'UNDEPLOYED',
  'UNDEPLOYING',
  'FAILED',
  'ARCHIVED',
]);

const normalizeStatus = (status?: string): DeploymentStatus =>
  status && KNOWN_STATUSES.has(status as DeploymentStatus)
    ? (status as DeploymentStatus)
    : 'none';

/** Whether any gateway is mid-transition, which is what keeps the view polling. */
export const isSettling = (environments: Environment[]): boolean =>
  environments.some((environment) =>
    environment.gateways.some(
      (gateway) => gateway.status === 'DEPLOYING' || gateway.status === 'UNDEPLOYING'
    )
  );

/**
 * The deployment data client, built from the host-injected `apiFetch`.
 *
 * Everything is addressed by project and API handle, and the server resolves the
 * pipeline, the environments and their gateways — so this client never has to
 * assemble a pipeline itself, and the promotion rules it enforces cannot be
 * bypassed from here.
 */
export function createDeployClient(apiFetch: ApiFetch, projectHandle: string, apiHandle: string) {
  const base = `/projects/${encodeURIComponent(projectHandle)}/apis/${encodeURIComponent(apiHandle)}`;

  return {
    /**
     * The pipeline's environments in promotion order, each with every gateway it
     * has. Gateway display names are joined in from the gateway list; a gateway
     * that list does not cover falls back to its handle.
     */
    async listEnvironments(): Promise<Environment[]> {
      const [stages, gateways] = await Promise.all([
        apiFetch<{ list?: StageDTO[] }>('GET', `${base}/deployments`),
        // Labels only — a failure here must not hide the deployment state, so it
        // degrades to showing handles.
        apiFetch<{ list?: ManagedGatewayDTO[] }>('GET', '/managed-gateways').catch(
          () => undefined
        ),
      ]);

      const nameById = new Map<string, string>();
      for (const gateway of gateways?.list ?? []) {
        nameById.set(gateway.id, gateway.displayName || gateway.host || gateway.id);
      }

      return (stages?.list ?? []).map((stage) => ({
        name: stage.environment,
        buildId: stage.buildId,
        gateways: (stage.gateways ?? []).map(
          (dto): Gateway => ({
            id: dto.gatewayId,
            name: nameById.get(dto.gatewayId) ?? dto.gatewayId,
            status: normalizeStatus(dto.status),
            deploymentId: dto.deploymentId,
            deploymentName: dto.name,
            statusReason: dto.statusReason,
            deployedAt: dto.createdAt,
            buildId: dto.buildId,
          })
        ),
      }));
    },

    /**
     * Prepares a build: an immutable snapshot of the API as it stands now. This is
     * the step that fixes what a later deploy will send.
     */
    async prepare(): Promise<Build> {
      const built = await apiFetch<BuildDTO>('POST', `${base}/builds`);
      return {
        buildId: built?.buildId ?? '',
        createdBy: built?.createdBy,
        createdAt: built?.createdAt,
      };
    },

    /** The API's prepared builds, newest first. */
    async listBuilds(): Promise<Build[]> {
      const response = await apiFetch<{ list?: BuildDTO[] }>('GET', `${base}/builds`);
      return (response?.list ?? []).map((dto) => ({
        buildId: dto.buildId,
        createdBy: dto.createdBy,
        createdAt: dto.createdAt,
      }));
    },

    /**
     * Deploys a prepared build to the named gateways of an environment.
     * `fromEnvironment` promotes that environment's build forward instead; the
     * server rejects a promotion the pipeline does not allow, or one whose source
     * has nothing deployed.
     */
    async deploy(input: {
      environment: string;
      gatewayIds: string[];
      buildId?: string;
      fromEnvironment?: string;
      parameters?: Record<string, string>;
    }): Promise<void> {
      await apiFetch('POST', `${base}/deployments`, {
        environment: input.environment,
        gatewayIds: input.gatewayIds,
        ...(input.buildId ? { buildId: input.buildId } : {}),
        ...(input.fromEnvironment ? { fromEnvironment: input.fromEnvironment } : {}),
        ...(input.parameters ? { parameters: input.parameters } : {}),
      });
    },

    /** Stops serving one deployment on one gateway; the rest are untouched. */
    async undeploy(environment: string, gatewayId: string, deploymentId: string): Promise<void> {
      const query = `?environment=${encodeURIComponent(environment)}&gatewayId=${encodeURIComponent(gatewayId)}`;
      await apiFetch('POST', `${base}/deployments/${encodeURIComponent(deploymentId)}/undeploy${query}`);
    },

    /**
     * An environment's deployment settings. These are per environment, not per
     * gateway, and exist whether or not anything is deployed there — so the form
     * can be filled in before an environment is first deployed to.
     */
    async getParameters(environment: string): Promise<DeploymentParameter[]> {
      const response = await apiFetch<{ list?: ParameterDTO[] }>(
        'GET',
        `${base}/environments/${encodeURIComponent(environment)}/parameters`
      );
      return (response?.list ?? []).map((dto) => ({
        name: dto.name,
        label: dto.label || dto.name,
        description: dto.description ?? '',
        type: dto.type ?? 'string',
        value: dto.value ?? '',
      }));
    },

    /** Stores an environment's settings. An empty value clears that setting. */
    async setParameters(environment: string, values: Record<string, string>): Promise<void> {
      await apiFetch(
        'PUT',
        `${base}/environments/${encodeURIComponent(environment)}/parameters`,
        { parameters: values }
      );
    },
  };
}

export type DeployClient = ReturnType<typeof createDeployClient>;
