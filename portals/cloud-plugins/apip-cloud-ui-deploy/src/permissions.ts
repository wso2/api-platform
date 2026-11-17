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

import type { ArtifactKind } from './deployApi';
import type { PermissionsPort } from './hostPort';

/**
 * The accepted-scope list of each operation this feature calls, as the
 * operation's `security` block declares it. Copied rather than generated: the
 * cloud operations' spec lives with the cloud plugin, outside this repo's
 * codegen, so this is the one place the feature names a scope.
 *
 * Each list is passed whole to the host's `permissions.canAny`, which is how
 * platform-api's `ScopeEnforcer` admits a call — one held scope matching one
 * accepted entry — so the broader `:manage` scope is listed beside the action
 * and the feature never reasons about inheritance itself.
 */

/**
 * Cloud Extension API: an API's builds and deployments through its project's
 * pipeline (`/projects/{projectId}/apis/{apiId}/builds` and `/deployments`).
 * Deleting a build is the one write with no scope of its own: only `:manage`
 * reaches it, as the spec declares.
 */
export const API_DEPLOYMENT_OPERATIONS = {
  ListApiBuilds: ['ap:deployment:read', 'ap:deployment:manage'],
  ListApiDeployments: ['ap:deployment:read', 'ap:deployment:manage'],
  // Deploy and stop also take the API kind's own deployment scopes, as the
  // platform's REST/GraphQL deployment operations do: a Developer or API
  // Publisher deploys the APIs they build without the pipeline-wide scopes.
  DeployApi: [
    'ap:deployment:deploy',
    'ap:deployment:manage',
    'ap:rest_api:deployment:create',
    'ap:rest_api:deployment:manage',
    'ap:rest_api:manage',
    'ap:graphql_api:deployment:create',
    'ap:graphql_api:deployment:manage',
    'ap:graphql_api:manage',
  ],
  UndeployApi: [
    'ap:deployment:undeploy',
    'ap:deployment:manage',
    'ap:rest_api:deployment:undeploy',
    'ap:rest_api:deployment:manage',
    'ap:rest_api:manage',
    'ap:graphql_api:deployment:undeploy',
    'ap:graphql_api:deployment:manage',
    'ap:graphql_api:manage',
  ],
  DeleteApiBuild: ['ap:deployment:manage'],
} as const satisfies Record<string, readonly string[]>;

/**
 * Platform API: adding the first gateway from the empty Deploy page
 * (`POST /managed-gateways`, `POST /self-hosted-gateways`); both accept the same list.
 */
export const GATEWAY_CREATE_OPERATIONS = ['ap:gateway:create', 'ap:gateway:manage'] as const;

/**
 * Cloud Extension API: an LLM provider's deployments straight to the
 * organization's environments (`/llm-providers/{providerId}/environment-deployments`).
 */
export const PROVIDER_DEPLOYMENT_OPERATIONS = {
  ListLlmProviderEnvironmentDeployments: ['ap:deployment:read', 'ap:deployment:manage'],
  DeployLlmProviderToEnvironment: ['ap:deployment:deploy', 'ap:deployment:manage'],
  UndeployLlmProviderFromEnvironment: ['ap:deployment:undeploy', 'ap:deployment:manage'],
} as const satisfies Record<string, readonly string[]>;

/**
 * Platform API (`platform-api/resources/openapi.yaml`): a provider's builds
 * belong to the provider itself, so the provider page reads and deletes them
 * through the provider's own resource (`/llm-providers/{llmProviderId}/builds`)
 * rather than a deployment route — and under that resource's scopes.
 */
export const PROVIDER_BUILD_OPERATIONS = {
  GetLLMProviderBuilds: [
    'ap:llm_provider:build:read',
    'ap:llm_provider:build:manage',
    'ap:llm_provider:manage',
  ],
  DeleteLLMProviderBuild: [
    'ap:llm_provider:build:delete',
    'ap:llm_provider:build:manage',
    'ap:llm_provider:manage',
  ],
} as const satisfies Record<string, readonly string[]>;

/**
 * Platform API: the artifact itself, read once for its display name and — for
 * a REST API — its backend URL and whether a gateway created it (`readOnly`),
 * which is what decides whether it may be deployed from here. The deployment
 * scopes above do not reach these reads, so a user holding only those sees the
 * pipeline but, for a REST API, cannot have its ownership confirmed and keeps
 * the deploy actions locked (see `DeployFeature`).
 */
export const ARTIFACT_READ_OPERATIONS = {
  RestApi: ['ap:rest_api:read', 'ap:rest_api:manage'],
  GraphQLApi: ['ap:graphql_api:read', 'ap:graphql_api:manage'],
  Mcp: ['ap:mcp_proxy:read', 'ap:mcp_proxy:manage'],
  LlmProxy: ['ap:llm_proxy:read', 'ap:llm_proxy:manage'],
  LlmProvider: ['ap:llm_provider:read', 'ap:llm_provider:manage'],
  AgentProxy: ['ap:agent_proxy:read', 'ap:agent_proxy:manage'],
} as const satisfies Record<ArtifactKind, readonly string[]>;

/**
 * Platform API (`/secrets`): a key typed into the provider deploy dialog is
 * stored as a secret before the deploy is sent, so entering a new one needs
 * this on top of the deploy scope. Deploying on a gateway's existing credential
 * does not, which is why the two are gated apart.
 */
export const SECRET_OPERATIONS = {
  CreateSecret: ['ap:secret:create', 'ap:secret:manage'],
} as const satisfies Record<string, readonly string[]>;

/**
 * Tooltip for a control disabled because the signed-in user lacks the scope.
 * Deliberately does not name the missing scope: that would tell a probing
 * caller what to look for, and means nothing to the person reading it.
 */
export const NO_PERMISSION_TOOLTIP = "You don't have permission to perform this action.";


/** The permission check with no host Port supplied: nothing is gated. */
const ALLOW_ALL: PermissionsPort = { canAny: () => true };

/**
 * The host's permission check, or one that gates nothing when the Port carries
 * none. A host without a permission layer leaves every control enabled exactly
 * as before — the posture `resourceLimits` takes when absent — and platform-api
 * still refuses what the user may not do.
 */
export const permissionsOf = (port: { permissions?: PermissionsPort }): PermissionsPort =>
  port.permissions ?? ALLOW_ALL;
