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
import { GATEWAY_TYPES_FOR_KIND } from './deployApi';
import type { Build, Environment } from './types';
import { managedGatewaysPath, toEnvironments, type ManagedGatewayDTO, type StageDTO } from './wire';

/**
 * The provider's own record. Only its upstream auth type is read: it decides whether a
 * deployment can name the header its credential is sent in, which only an api-key upstream
 * can. The credential itself never comes back — it is write-only.
 */
type ProviderDTO = {
  upstream?: { main?: { url?: string; auth?: { type?: string; header?: string } } };
};

/**
 * How the provider talks to its upstream, which is what the deploy form starts from: the
 * backend it routes to, how it authenticates, and the header it sends its credential in.
 * The credential itself is never here, because it is write-only.
 */
export type ProviderUpstream = {
  url?: string;
  authType?: string;
  authHeader?: string;
};

/** One of the provider's builds, as the platform reports it. */
type BuildDTO = {
  buildId: string;
  description?: string;
  createdBy?: string;
  createdAt?: string;
};

/**
 * The data client for deploying an LLM PROVIDER across the organization's
 * environments.
 *
 * A provider belongs to the organization rather than to a project, so it has no
 * deployment pipeline: there is nothing to order its environments and nothing to
 * promote between them. Its routes are therefore the provider's own rather than a
 * project's, and every environment is deployed to directly.
 *
 * Everything else is the same as deploying through a pipeline, and deliberately so
 * — the server resolves which environments exist and which of their gateways can
 * run a provider, so this client never assembles that itself and cannot offer a
 * target the deploy would reject.
 */
export function createProviderDeployClient(apiFetch: ApiFetch, providerHandle: string) {
  const nativeBase = `/llm-providers/${encodeURIComponent(providerHandle)}`;
  const base = `${nativeBase}/environment-deployments`;
  // Providers run on AI gateways, which is the same mapping the server enforces
  // the deploy against — asked for here only to narrow what the page requests.
  const gatewaysPath = managedGatewaysPath(GATEWAY_TYPES_FOR_KIND.LlmProvider);

  return {
    /**
     * Every environment in the organization, each with the gateways this provider
     * can be deployed to and whatever is deployed on them. The order is the
     * organization's own and carries no promotion meaning.
     */
    async listEnvironments(): Promise<Environment[]> {
      const [stages, gateways] = await Promise.all([
        apiFetch<{ list?: StageDTO[] }>('GET', base),
        apiFetch<{ list?: ManagedGatewayDTO[] }>('GET', gatewaysPath).catch(() => undefined),
      ]);
      return toEnvironments(stages?.list, gateways?.list);
    },

    /**
     * The provider's upstream, so the deploy form opens on what the provider itself uses
     * rather than on blanks, and knows whether a credential can be named at all. An empty
     * result leaves those fields out rather than offering ones the deploy would refuse.
     */
    async readUpstream(): Promise<ProviderUpstream> {
      const provider = await apiFetch<ProviderDTO>('GET', nativeBase);
      const main = provider?.upstream?.main;
      return { url: main?.url, authType: main?.auth?.type, authHeader: main?.auth?.header };
    },

    /**
     * The provider's builds, newest first.
     *
     * Read from the provider's own resource rather than through an environment route:
     * a build belongs to the provider, not to any one environment, and the platform
     * already scopes the call to the caller's organization.
     */
    async listBuilds(): Promise<Build[]> {
      const response = await apiFetch<{ list?: BuildDTO[] }>('GET', `${nativeBase}/builds`);
      return (response?.list ?? []).map((dto) => ({
        buildId: dto.buildId,
        description: dto.description,
        createdBy: dto.createdBy,
        createdAt: dto.createdAt,
      }));
    },

    /**
     * Deletes one of the provider's builds, which is how room is made once it is at
     * its build limit and deploying is refused.
     *
     * Refused with 409 while a gateway is serving the build; stopping it releases it.
     * Deployments that ran the build survive and stay deployable from the provider
     * itself, but stop reporting it.
     */
    async deleteBuild(buildId: string): Promise<void> {
      await apiFetch('DELETE', `${nativeBase}/builds/${encodeURIComponent(buildId)}`);
    },

    /**
     * Stores a credential as a secret and returns the reference that stands for it.
     *
     * The key itself goes straight to the platform's secrets resource and is never
     * held anywhere else — not in a deploy request, not in a deployment's metadata.
     * What travels from here on is the reference. This is the same exchange the
     * provider's own API key goes through when it is set on the Connection tab.
     *
     * Each call takes a fresh handle, so re-keying a gateway never collides with a
     * secret a previous key left behind.
     */
    async storeCredential(key: string, label: string): Promise<string> {
      const handle = crypto.randomUUID();
      const form = new FormData();
      form.append('id', handle);
      form.append('displayName', label);
      form.append('description', `API key for ${label}`);
      form.append('value', key);
      form.append('type', 'GENERIC');
      const stored = await apiFetch<{ id?: string }>('POST', '/secrets', form);
      return `{{ secret "${stored?.id ?? handle}" }}`;
    },

    /**
     * Discards a credential stored for a deploy that then failed, so a key typed into
     * a deploy that never happened does not linger in the organization's secrets.
     *
     * Safe to call whenever the deploy did not report success, including when its
     * outcome is unknown: the platform refuses to delete a secret a deployment
     * references, so one that did reach a gateway survives this. A refusal is
     * therefore the correct answer, not an error worth surfacing.
     */
    async discardCredential(reference: string): Promise<void> {
      const handle = reference.match(/^\{\{\s*secret\s+"([^"]+)"\s*\}\}$/)?.[1];
      if (!handle) return;
      await apiFetch('DELETE', `/secrets/${encodeURIComponent(handle)}`);
    },

    /**
     * Deploys the provider as it stands onto every gateway named of one
     * environment. The gateways go in one call because the environment is deployed
     * as a set: if any gateway fails, the backend puts them all back.
     *
     * A gateway's `apiKey` is the reference returned by storeCredential, never a key.
     * Omitting it leaves that gateway on the provider's own credential.
     *
     * `buildId` deploys an existing build. Omitting it ships the provider as it stands:
     * the platform snapshots the definition into a build and records the deployment of
     * it in one transaction, so an edit made since the last deploy is included and no
     * build is left behind if the deploy fails.
     *
     * The gateways must include every gateway the provider is already deployed on
     * in that environment; leaving one out is refused with 409.
     */
    async deploy(input: {
      environment: string;
      gateways: { gatewayId: string; apiKey?: string; authHeader?: string; endpointUrl?: string }[];
      buildId?: string;
    }): Promise<void> {
      await apiFetch('POST', base, {
        environment: input.environment,
        ...(input.buildId ? { buildId: input.buildId } : {}),
        gateways: input.gateways.map((gateway) => {
          // The header travels only with a key: on its own it would name where to put a
          // credential this deployment does not have, and the platform refuses it.
          const parameters: Record<string, string> = {};
          if (gateway.endpointUrl) parameters.productionEndpoint = gateway.endpointUrl;
          if (gateway.apiKey) {
            parameters.apiKey = gateway.apiKey;
            if (gateway.authHeader) parameters.authHeader = gateway.authHeader;
          }
          return {
            gatewayId: gateway.gatewayId,
            ...(Object.keys(parameters).length > 0 ? { parameters } : {}),
          };
        }),
      });
    },

    /** Stops serving one deployment on one gateway; the rest are untouched. */
    async undeploy(environment: string, gatewayId: string, deploymentId: string): Promise<void> {
      const query = `?environment=${encodeURIComponent(environment)}&gatewayId=${encodeURIComponent(gatewayId)}`;
      await apiFetch('POST', `${base}/${encodeURIComponent(deploymentId)}/undeploy${query}`);
    },
  };
}

export type ProviderDeployClient = ReturnType<typeof createProviderDeployClient>;
