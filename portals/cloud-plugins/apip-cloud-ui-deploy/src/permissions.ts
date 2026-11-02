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
  DeployApi: ['ap:deployment:deploy', 'ap:deployment:manage'],
  UndeployApi: ['ap:deployment:undeploy', 'ap:deployment:manage'],
  DeleteApiBuild: ['ap:deployment:manage'],
} as const satisfies Record<string, readonly string[]>;

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
 * Tooltip for a control disabled because the signed-in user lacks the scope.
 * Deliberately does not name the missing scope: that would tell a probing
 * caller what to look for, and means nothing to the person reading it.
 */
export const NO_PERMISSION_TOOLTIP = "You don't have permission to perform this action.";
