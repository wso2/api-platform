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
import type { Build, Environment } from './types';
import { managedGatewaysPath, toEnvironments, type ManagedGatewayDTO, type StageDTO } from './wire';

/**
 * The artifact kinds this page can deploy, named as the platform names them. A
 * handle is unique only WITHIN a kind, so every call says which one it means.
 */
export type ArtifactKind = 'RestApi' | 'Mcp' | 'LlmProxy' | 'LlmProvider';

/**
 * Each kind's own path on the platform API. The pipeline routes are shared, but a
 * few calls go straight to the artifact's own resource — deleting a build, reading
 * an API's backend URL — and those are split by kind.
 */
const NATIVE_PATH: Record<ArtifactKind, string> = {
  RestApi: 'rest-apis',
  Mcp: 'mcp-proxies',
  LlmProxy: 'llm-proxies',
  LlmProvider: 'llm-providers',
};

/**
 * The kinds of gateway each artifact kind runs on. A gateway's kind decides what
 * its runtime can serve, so this is the same mapping the server enforces the deploy
 * against — kept here only to narrow the gateways the page asks for, never as the
 * rule itself. A kind absent from this map asks for every gateway.
 */
export const GATEWAY_TYPES_FOR_KIND: Record<ArtifactKind, readonly string[]> = {
  RestApi: ['regular'],
  Mcp: ['ai'],
  LlmProxy: ['ai'],
  LlmProvider: ['ai'],
};

/**
 * Whether a deployment of this kind is made with a backend URL of its own.
 *
 * Only a REST API is: it proxies a backend, and which backend can differ per
 * gateway. An MCP server or an LLM proxy carries its upstream in its own
 * definition — there is nothing per-deployment to ask for, and asking made the
 * form impossible to complete, since nothing could fill it in either.
 *
 * An unclassified kind is assumed not to take one: the worst case is a parameter
 * nobody sets, rather than a required field with no answer.
 */
const KIND_TAKES_ENDPOINT: Record<ArtifactKind, boolean> = {
  RestApi: true,
  Mcp: false,
  LlmProxy: false,
  LlmProvider: false,
};

export const takesEndpointUrl = (kind: ArtifactKind): boolean => KIND_TAKES_ENDPOINT[kind] ?? false;

type BuildDTO = {
  buildId: string;
  description?: string;
  createdBy?: string;
  createdAt?: string;
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
export function createDeployClient(
  apiFetch: ApiFetch,
  projectHandle: string,
  apiHandle: string,
  kind: ArtifactKind = 'RestApi'
) {
  const base = `/projects/${encodeURIComponent(projectHandle)}/apis/${encodeURIComponent(apiHandle)}`;
  // The pipeline routes serve every artifact kind, so each call says which kind it
  // addresses: a handle is unique only within a kind. REST APIs are the default on
  // the server, but it is sent either way so the request is explicit.
  const forKind = `kind=${encodeURIComponent(kind)}`;
  // Ask the platform for only the gateways this artifact can run, rather than
  // pulling every gateway in the organization back and dropping most of them here.
  // Derived from the kind, not passed in by the host: the server refuses a deploy
  // across the same mapping, and a page that asked for a different set than the
  // server accepts would offer a target the deploy then rejects.
  const gatewaysPath = managedGatewaysPath(GATEWAY_TYPES_FOR_KIND[kind] ?? []);
  const withKind = (path: string) => (path.includes('?') ? `${path}&${forKind}` : `${path}?${forKind}`);

  return {
    /**
     * The pipeline's environments in promotion order, each with every gateway it
     * has. Identity and health come from the gateways resource; a gateway it does
     * not cover falls back to its handle and is treated as up, so a failure to
     * read it cannot make the page refuse to deploy.
     */
    async listEnvironments(): Promise<Environment[]> {
      const [stages, gateways] = await Promise.all([
        apiFetch<{ list?: StageDTO[] }>('GET', withKind(`${base}/deployments`)),
        apiFetch<{ list?: ManagedGatewayDTO[] }>('GET', gatewaysPath).catch(() => undefined),
      ]);
      return toEnvironments(stages?.list, gateways?.list);
    },

    /** The API's builds, newest first. */
    async listBuilds(): Promise<Build[]> {
      const response = await apiFetch<{ list?: BuildDTO[] }>('GET', withKind(`${base}/builds`));
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
        `/${NATIVE_PATH[kind]}/${encodeURIComponent(apiHandle)}/builds/${encodeURIComponent(buildId)}`
      );
    },

    /**
     * The backend URL the API is defined against, which the deploy and promote
     * forms start from so a first deployment does not have to be typed out.
     * Absent when the API declares its upstream by reference rather than by URL.
     */
    async readApiEndpointUrl(): Promise<string | undefined> {
      // Only REST APIs declare a backend URL this form can start from; the other
      // kinds either have none or shape it differently, so the field is simply left
      // empty for them rather than guessed at.
      if (kind !== 'RestApi') return undefined;
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
      await apiFetch('POST', withKind(`${base}/deployments`), {
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
        withKind(`${base}/deployments/${encodeURIComponent(deploymentId)}/undeploy${query}`)
      );
    },
  };
}

export type DeployClient = ReturnType<typeof createDeployClient>;
