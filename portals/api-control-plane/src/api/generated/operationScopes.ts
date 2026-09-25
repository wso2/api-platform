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
 * GENERATED FILE — do not edit.
 *
 * Produced by `scripts/generateOperationScopes.ts` (`npm run api:codegen`)
 * from platform-api/resources/openapi.yaml. Edit the spec, not this file.
 */

import type { operations } from './platform';

/** Every scope declared by the spec's OAuth2Security scheme. */
export type ApScope =
  | 'ap:api_key:all:manage'
  | 'ap:api_key:read'
  | 'ap:api_portal:create'
  | 'ap:api_portal:delete'
  | 'ap:api_portal:draft:manage'
  | 'ap:api_portal:draft:read'
  | 'ap:api_portal:draft:update'
  | 'ap:api_portal:manage'
  | 'ap:api_portal:publication:read'
  | 'ap:api_portal:read'
  | 'ap:api_portal:rest_api:deprecate'
  | 'ap:api_portal:rest_api:manage'
  | 'ap:api_portal:rest_api:publish'
  | 'ap:api_portal:rest_api:unpublish'
  | 'ap:api_portal:update'
  | 'ap:api_publication:read'
  | 'ap:application:api_key:create'
  | 'ap:application:api_key:delete'
  | 'ap:application:api_key:manage'
  | 'ap:application:api_key:read'
  | 'ap:application:association:api_key:read'
  | 'ap:application:association:create'
  | 'ap:application:association:delete'
  | 'ap:application:association:manage'
  | 'ap:application:association:read'
  | 'ap:application:create'
  | 'ap:application:delete'
  | 'ap:application:manage'
  | 'ap:application:read'
  | 'ap:application:update'
  | 'ap:gateway:create'
  | 'ap:gateway:delete'
  | 'ap:gateway:manage'
  | 'ap:gateway:manifest:read'
  | 'ap:gateway:read'
  | 'ap:gateway:token:create'
  | 'ap:gateway:token:delete'
  | 'ap:gateway:token:manage'
  | 'ap:gateway:token:read'
  | 'ap:gateway:update'
  | 'ap:gateway_custom_policy:create'
  | 'ap:gateway_custom_policy:delete'
  | 'ap:gateway_custom_policy:manage'
  | 'ap:gateway_custom_policy:read'
  | 'ap:llm_provider:api_key:create'
  | 'ap:llm_provider:api_key:delete'
  | 'ap:llm_provider:api_key:manage'
  | 'ap:llm_provider:api_key:read'
  | 'ap:llm_provider:build:create'
  | 'ap:llm_provider:build:delete'
  | 'ap:llm_provider:build:manage'
  | 'ap:llm_provider:build:read'
  | 'ap:llm_provider:create'
  | 'ap:llm_provider:delete'
  | 'ap:llm_provider:deployment:create'
  | 'ap:llm_provider:deployment:delete'
  | 'ap:llm_provider:deployment:manage'
  | 'ap:llm_provider:deployment:read'
  | 'ap:llm_provider:deployment:restore'
  | 'ap:llm_provider:deployment:undeploy'
  | 'ap:llm_provider:manage'
  | 'ap:llm_provider:read'
  | 'ap:llm_provider:update'
  | 'ap:llm_proxy:api_key:create'
  | 'ap:llm_proxy:api_key:delete'
  | 'ap:llm_proxy:api_key:manage'
  | 'ap:llm_proxy:api_key:read'
  | 'ap:llm_proxy:build:create'
  | 'ap:llm_proxy:build:delete'
  | 'ap:llm_proxy:build:manage'
  | 'ap:llm_proxy:build:read'
  | 'ap:llm_proxy:create'
  | 'ap:llm_proxy:delete'
  | 'ap:llm_proxy:deployment:create'
  | 'ap:llm_proxy:deployment:delete'
  | 'ap:llm_proxy:deployment:manage'
  | 'ap:llm_proxy:deployment:read'
  | 'ap:llm_proxy:deployment:restore'
  | 'ap:llm_proxy:deployment:undeploy'
  | 'ap:llm_proxy:manage'
  | 'ap:llm_proxy:read'
  | 'ap:llm_proxy:update'
  | 'ap:llm_template:create'
  | 'ap:llm_template:delete'
  | 'ap:llm_template:manage'
  | 'ap:llm_template:read'
  | 'ap:llm_template:update'
  | 'ap:mcp_proxy:build:create'
  | 'ap:mcp_proxy:build:delete'
  | 'ap:mcp_proxy:build:manage'
  | 'ap:mcp_proxy:build:read'
  | 'ap:mcp_proxy:create'
  | 'ap:mcp_proxy:delete'
  | 'ap:mcp_proxy:deployment:create'
  | 'ap:mcp_proxy:deployment:delete'
  | 'ap:mcp_proxy:deployment:manage'
  | 'ap:mcp_proxy:deployment:read'
  | 'ap:mcp_proxy:deployment:restore'
  | 'ap:mcp_proxy:deployment:undeploy'
  | 'ap:mcp_proxy:manage'
  | 'ap:mcp_proxy:read'
  | 'ap:mcp_proxy:update'
  | 'ap:organization:create'
  | 'ap:organization:manage'
  | 'ap:organization:read'
  | 'ap:project:create'
  | 'ap:project:delete'
  | 'ap:project:manage'
  | 'ap:project:read'
  | 'ap:project:update'
  | 'ap:rest_api:api_key:create'
  | 'ap:rest_api:api_key:delete'
  | 'ap:rest_api:api_key:manage'
  | 'ap:rest_api:api_key:update'
  | 'ap:rest_api:build:create'
  | 'ap:rest_api:build:delete'
  | 'ap:rest_api:build:manage'
  | 'ap:rest_api:build:read'
  | 'ap:rest_api:create'
  | 'ap:rest_api:delete'
  | 'ap:rest_api:deployment:create'
  | 'ap:rest_api:deployment:delete'
  | 'ap:rest_api:deployment:manage'
  | 'ap:rest_api:deployment:read'
  | 'ap:rest_api:deployment:restore'
  | 'ap:rest_api:deployment:undeploy'
  | 'ap:rest_api:gateway:create'
  | 'ap:rest_api:gateway:manage'
  | 'ap:rest_api:gateway:read'
  | 'ap:rest_api:manage'
  | 'ap:rest_api:read'
  | 'ap:rest_api:update'
  | 'ap:secret:create'
  | 'ap:secret:delete'
  | 'ap:secret:manage'
  | 'ap:secret:read'
  | 'ap:secret:update'
  | 'ap:subscription:create'
  | 'ap:subscription:delete'
  | 'ap:subscription:manage'
  | 'ap:subscription:read'
  | 'ap:subscription:update'
  | 'ap:subscription_plan:create'
  | 'ap:subscription_plan:delete'
  | 'ap:subscription_plan:manage'
  | 'ap:subscription_plan:read'
  | 'ap:subscription_plan:update';

/**
 * The full scope catalog, for building test personas and for validating an
 * operator-supplied scope string. Sorted, so it is diff-stable.
 */
export const AP_SCOPES: readonly ApScope[] = [
  'ap:api_key:all:manage',
  'ap:api_key:read',
  'ap:api_portal:create',
  'ap:api_portal:delete',
  'ap:api_portal:draft:manage',
  'ap:api_portal:draft:read',
  'ap:api_portal:draft:update',
  'ap:api_portal:manage',
  'ap:api_portal:publication:read',
  'ap:api_portal:read',
  'ap:api_portal:rest_api:deprecate',
  'ap:api_portal:rest_api:manage',
  'ap:api_portal:rest_api:publish',
  'ap:api_portal:rest_api:unpublish',
  'ap:api_portal:update',
  'ap:api_publication:read',
  'ap:application:api_key:create',
  'ap:application:api_key:delete',
  'ap:application:api_key:manage',
  'ap:application:api_key:read',
  'ap:application:association:api_key:read',
  'ap:application:association:create',
  'ap:application:association:delete',
  'ap:application:association:manage',
  'ap:application:association:read',
  'ap:application:create',
  'ap:application:delete',
  'ap:application:manage',
  'ap:application:read',
  'ap:application:update',
  'ap:gateway:create',
  'ap:gateway:delete',
  'ap:gateway:manage',
  'ap:gateway:manifest:read',
  'ap:gateway:read',
  'ap:gateway:token:create',
  'ap:gateway:token:delete',
  'ap:gateway:token:manage',
  'ap:gateway:token:read',
  'ap:gateway:update',
  'ap:gateway_custom_policy:create',
  'ap:gateway_custom_policy:delete',
  'ap:gateway_custom_policy:manage',
  'ap:gateway_custom_policy:read',
  'ap:llm_provider:api_key:create',
  'ap:llm_provider:api_key:delete',
  'ap:llm_provider:api_key:manage',
  'ap:llm_provider:api_key:read',
  'ap:llm_provider:build:create',
  'ap:llm_provider:build:delete',
  'ap:llm_provider:build:manage',
  'ap:llm_provider:build:read',
  'ap:llm_provider:create',
  'ap:llm_provider:delete',
  'ap:llm_provider:deployment:create',
  'ap:llm_provider:deployment:delete',
  'ap:llm_provider:deployment:manage',
  'ap:llm_provider:deployment:read',
  'ap:llm_provider:deployment:restore',
  'ap:llm_provider:deployment:undeploy',
  'ap:llm_provider:manage',
  'ap:llm_provider:read',
  'ap:llm_provider:update',
  'ap:llm_proxy:api_key:create',
  'ap:llm_proxy:api_key:delete',
  'ap:llm_proxy:api_key:manage',
  'ap:llm_proxy:api_key:read',
  'ap:llm_proxy:build:create',
  'ap:llm_proxy:build:delete',
  'ap:llm_proxy:build:manage',
  'ap:llm_proxy:build:read',
  'ap:llm_proxy:create',
  'ap:llm_proxy:delete',
  'ap:llm_proxy:deployment:create',
  'ap:llm_proxy:deployment:delete',
  'ap:llm_proxy:deployment:manage',
  'ap:llm_proxy:deployment:read',
  'ap:llm_proxy:deployment:restore',
  'ap:llm_proxy:deployment:undeploy',
  'ap:llm_proxy:manage',
  'ap:llm_proxy:read',
  'ap:llm_proxy:update',
  'ap:llm_template:create',
  'ap:llm_template:delete',
  'ap:llm_template:manage',
  'ap:llm_template:read',
  'ap:llm_template:update',
  'ap:mcp_proxy:build:create',
  'ap:mcp_proxy:build:delete',
  'ap:mcp_proxy:build:manage',
  'ap:mcp_proxy:build:read',
  'ap:mcp_proxy:create',
  'ap:mcp_proxy:delete',
  'ap:mcp_proxy:deployment:create',
  'ap:mcp_proxy:deployment:delete',
  'ap:mcp_proxy:deployment:manage',
  'ap:mcp_proxy:deployment:read',
  'ap:mcp_proxy:deployment:restore',
  'ap:mcp_proxy:deployment:undeploy',
  'ap:mcp_proxy:manage',
  'ap:mcp_proxy:read',
  'ap:mcp_proxy:update',
  'ap:organization:create',
  'ap:organization:manage',
  'ap:organization:read',
  'ap:project:create',
  'ap:project:delete',
  'ap:project:manage',
  'ap:project:read',
  'ap:project:update',
  'ap:rest_api:api_key:create',
  'ap:rest_api:api_key:delete',
  'ap:rest_api:api_key:manage',
  'ap:rest_api:api_key:update',
  'ap:rest_api:build:create',
  'ap:rest_api:build:delete',
  'ap:rest_api:build:manage',
  'ap:rest_api:build:read',
  'ap:rest_api:create',
  'ap:rest_api:delete',
  'ap:rest_api:deployment:create',
  'ap:rest_api:deployment:delete',
  'ap:rest_api:deployment:manage',
  'ap:rest_api:deployment:read',
  'ap:rest_api:deployment:restore',
  'ap:rest_api:deployment:undeploy',
  'ap:rest_api:gateway:create',
  'ap:rest_api:gateway:manage',
  'ap:rest_api:gateway:read',
  'ap:rest_api:manage',
  'ap:rest_api:read',
  'ap:rest_api:update',
  'ap:secret:create',
  'ap:secret:delete',
  'ap:secret:manage',
  'ap:secret:read',
  'ap:secret:update',
  'ap:subscription:create',
  'ap:subscription:delete',
  'ap:subscription:manage',
  'ap:subscription:read',
  'ap:subscription:update',
  'ap:subscription_plan:create',
  'ap:subscription_plan:delete',
  'ap:subscription_plan:manage',
  'ap:subscription_plan:read',
  'ap:subscription_plan:update',
];

/**
 * Scopes accepted for each operation, **any-of**: a caller holding at least one
 * of them may invoke it. Mirrors each operation's `security` block verbatim,
 * including the broader `:manage` scope the spec lists beside each narrow one —
 * which is why no scope implication happens on the client.
 *
 * An empty list means the operation requires no particular scope.
 *
 * `satisfies Record<keyof operations, ...>` is the drift guard: an operation
 * added to the spec but missing here fails `tsc`.
 */
export const OPERATION_SCOPES = {
  AddApplicationAPIKeys: [
    'ap:api_key:all:manage',
    'ap:application:api_key:create',
    'ap:application:api_key:manage',
    'ap:application:manage',
  ],
  AddApplicationAssociations: [
    'ap:application:association:create',
    'ap:application:association:manage',
    'ap:application:manage',
  ],
  AddGatewaysToAPI: [
    'ap:rest_api:gateway:create',
    'ap:rest_api:gateway:manage',
    'ap:rest_api:manage',
  ],
  copyLLMProviderTemplateVersion: ['ap:llm_template:create', 'ap:llm_template:manage'],
  CreateAPIKey: [
    'ap:api_key:all:manage',
    'ap:rest_api:api_key:create',
    'ap:rest_api:api_key:manage',
    'ap:rest_api:manage',
  ],
  CreateApiPortal: ['ap:api_portal:create', 'ap:api_portal:manage'],
  CreateApplication: ['ap:application:create', 'ap:application:manage'],
  CreateBuild: ['ap:rest_api:build:create', 'ap:rest_api:build:manage', 'ap:rest_api:manage'],
  CreateGateway: ['ap:gateway:create', 'ap:gateway:manage'],
  createLLMProvider: ['ap:llm_provider:create', 'ap:llm_provider:manage'],
  createLLMProviderAPIKey: [
    'ap:api_key:all:manage',
    'ap:llm_provider:api_key:create',
    'ap:llm_provider:api_key:manage',
    'ap:llm_provider:manage',
  ],
  CreateLLMProviderBuild: [
    'ap:llm_provider:build:create',
    'ap:llm_provider:build:manage',
    'ap:llm_provider:manage',
  ],
  createLLMProviderTemplate: ['ap:llm_template:create', 'ap:llm_template:manage'],
  createLLMProxy: ['ap:llm_proxy:create', 'ap:llm_proxy:manage'],
  createLLMProxyAPIKey: [
    'ap:api_key:all:manage',
    'ap:llm_proxy:api_key:create',
    'ap:llm_proxy:api_key:manage',
    'ap:llm_proxy:manage',
  ],
  CreateLLMProxyBuild: [
    'ap:llm_proxy:build:create',
    'ap:llm_proxy:build:manage',
    'ap:llm_proxy:manage',
  ],
  createMCPProxy: ['ap:mcp_proxy:create', 'ap:mcp_proxy:manage'],
  CreateMCPProxyBuild: [
    'ap:mcp_proxy:build:create',
    'ap:mcp_proxy:build:manage',
    'ap:mcp_proxy:manage',
  ],
  CreateProject: ['ap:project:create', 'ap:project:manage'],
  CreateRESTAPI: ['ap:rest_api:create', 'ap:rest_api:manage'],
  createSecret: ['ap:secret:create', 'ap:secret:manage'],
  CreateSubscription: ['ap:subscription:create', 'ap:subscription:manage'],
  CreateSubscriptionPlan: ['ap:subscription_plan:create', 'ap:subscription_plan:manage'],
  DeleteApiPortal: ['ap:api_portal:delete', 'ap:api_portal:manage'],
  DeleteApplication: ['ap:application:delete', 'ap:application:manage'],
  DeleteBuild: ['ap:rest_api:build:delete', 'ap:rest_api:build:manage', 'ap:rest_api:manage'],
  DeleteDeployment: [
    'ap:rest_api:deployment:delete',
    'ap:rest_api:deployment:manage',
    'ap:rest_api:manage',
  ],
  DeleteGateway: ['ap:gateway:delete', 'ap:gateway:manage'],
  DeleteGatewayCustomPolicy: ['ap:gateway_custom_policy:delete', 'ap:gateway_custom_policy:manage'],
  deleteLLMProvider: ['ap:llm_provider:delete', 'ap:llm_provider:manage'],
  deleteLLMProviderAPIKey: [
    'ap:api_key:all:manage',
    'ap:llm_provider:api_key:delete',
    'ap:llm_provider:api_key:manage',
    'ap:llm_provider:manage',
  ],
  DeleteLLMProviderBuild: [
    'ap:llm_provider:build:delete',
    'ap:llm_provider:build:manage',
    'ap:llm_provider:manage',
  ],
  deleteLLMProviderDeployment: [
    'ap:llm_provider:deployment:delete',
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:manage',
  ],
  deleteLLMProviderTemplateVersion: ['ap:llm_template:delete', 'ap:llm_template:manage'],
  deleteLLMProxy: ['ap:llm_proxy:delete', 'ap:llm_proxy:manage'],
  deleteLLMProxyAPIKey: [
    'ap:api_key:all:manage',
    'ap:llm_proxy:api_key:delete',
    'ap:llm_proxy:api_key:manage',
    'ap:llm_proxy:manage',
  ],
  DeleteLLMProxyBuild: [
    'ap:llm_proxy:build:delete',
    'ap:llm_proxy:build:manage',
    'ap:llm_proxy:manage',
  ],
  deleteLLMProxyDeployment: [
    'ap:llm_proxy:deployment:delete',
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:manage',
  ],
  deleteMCPProxy: ['ap:mcp_proxy:delete', 'ap:mcp_proxy:manage'],
  DeleteMCPProxyBuild: [
    'ap:mcp_proxy:build:delete',
    'ap:mcp_proxy:build:manage',
    'ap:mcp_proxy:manage',
  ],
  DeleteMCPProxyDeployment: [
    'ap:mcp_proxy:deployment:delete',
    'ap:mcp_proxy:deployment:manage',
    'ap:mcp_proxy:manage',
  ],
  DeleteProject: ['ap:project:delete', 'ap:project:manage'],
  DeleteRESTAPI: ['ap:rest_api:delete', 'ap:rest_api:manage'],
  deleteSecret: ['ap:secret:delete', 'ap:secret:manage'],
  DeleteSubscription: ['ap:subscription:delete', 'ap:subscription:manage'],
  DeleteSubscriptionPlan: ['ap:subscription_plan:delete', 'ap:subscription_plan:manage'],
  DeployAPI: [
    'ap:rest_api:deployment:create',
    'ap:rest_api:deployment:manage',
    'ap:rest_api:manage',
  ],
  deployLLMProvider: [
    'ap:llm_provider:deployment:create',
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:manage',
  ],
  deployLLMProxy: [
    'ap:llm_proxy:deployment:create',
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:manage',
  ],
  DeployMCPProxy: [
    'ap:mcp_proxy:deployment:create',
    'ap:mcp_proxy:deployment:manage',
    'ap:mcp_proxy:manage',
  ],
  deprecateRestApiOnApiPortal: [
    'ap:api_portal:rest_api:deprecate',
    'ap:api_portal:rest_api:manage',
  ],
  fetchMCPProxyServerInfo: ['ap:mcp_proxy:manage', 'ap:mcp_proxy:read'],
  GetApiPortal: ['ap:api_portal:manage', 'ap:api_portal:read'],
  getApiPublication: ['ap:api_portal:publication:read'],
  getApiPublicationDefinition: ['ap:api_portal:publication:read'],
  getApiPublicationDraft: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:read'],
  getApiPublicationDraftDefinition: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:read'],
  getApiPublicationDraftLandingPage: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:read'],
  getApiPublicationDraftThumbnail: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:read'],
  getApiPublicationLandingPage: ['ap:api_portal:publication:read'],
  getApiPublicationThumbnail: ['ap:api_portal:publication:read'],
  GetApplication: ['ap:application:manage', 'ap:application:read'],
  GetBuild: ['ap:rest_api:build:manage', 'ap:rest_api:build:read', 'ap:rest_api:manage'],
  GetBuilds: ['ap:rest_api:build:manage', 'ap:rest_api:build:read', 'ap:rest_api:manage'],
  GetDeployment: [
    'ap:rest_api:deployment:manage',
    'ap:rest_api:deployment:read',
    'ap:rest_api:manage',
  ],
  GetDeployments: [
    'ap:rest_api:deployment:manage',
    'ap:rest_api:deployment:read',
    'ap:rest_api:manage',
  ],
  GetGateway: ['ap:gateway:manage', 'ap:gateway:read'],
  GetGatewayCustomPolicy: ['ap:gateway_custom_policy:manage', 'ap:gateway_custom_policy:read'],
  GetGatewayManifest: ['ap:gateway:manage', 'ap:gateway:manifest:read'],
  getLLMProvider: ['ap:llm_provider:manage', 'ap:llm_provider:read'],
  GetLLMProviderBuild: [
    'ap:llm_provider:build:manage',
    'ap:llm_provider:build:read',
    'ap:llm_provider:manage',
  ],
  GetLLMProviderBuilds: [
    'ap:llm_provider:build:manage',
    'ap:llm_provider:build:read',
    'ap:llm_provider:manage',
  ],
  getLLMProviderDeployment: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:read',
    'ap:llm_provider:manage',
  ],
  getLLMProviderDeployments: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:read',
    'ap:llm_provider:manage',
  ],
  getLLMProviderTemplate: ['ap:llm_template:manage', 'ap:llm_template:read'],
  getLLMProxy: ['ap:llm_proxy:manage', 'ap:llm_proxy:read'],
  GetLLMProxyBuild: ['ap:llm_proxy:build:manage', 'ap:llm_proxy:build:read', 'ap:llm_proxy:manage'],
  GetLLMProxyBuilds: [
    'ap:llm_proxy:build:manage',
    'ap:llm_proxy:build:read',
    'ap:llm_proxy:manage',
  ],
  getLLMProxyDeployment: [
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:deployment:read',
    'ap:llm_proxy:manage',
  ],
  getLLMProxyDeployments: [
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:deployment:read',
    'ap:llm_proxy:manage',
  ],
  getMCPProxy: ['ap:mcp_proxy:manage', 'ap:mcp_proxy:read'],
  GetMCPProxyBuild: ['ap:mcp_proxy:build:manage', 'ap:mcp_proxy:build:read', 'ap:mcp_proxy:manage'],
  GetMCPProxyBuilds: [
    'ap:mcp_proxy:build:manage',
    'ap:mcp_proxy:build:read',
    'ap:mcp_proxy:manage',
  ],
  GetMCPProxyDeployment: [
    'ap:mcp_proxy:deployment:manage',
    'ap:mcp_proxy:deployment:read',
    'ap:mcp_proxy:manage',
  ],
  GetMCPProxyDeployments: [
    'ap:mcp_proxy:deployment:manage',
    'ap:mcp_proxy:deployment:read',
    'ap:mcp_proxy:manage',
  ],
  GetOrganization: ['ap:organization:manage', 'ap:organization:read'],
  GetProject: ['ap:project:manage', 'ap:project:read'],
  GetRESTAPI: ['ap:rest_api:manage', 'ap:rest_api:read'],
  GetRESTAPIGateways: [
    'ap:gateway:manage',
    'ap:gateway:read',
    'ap:rest_api:gateway:manage',
    'ap:rest_api:gateway:read',
    'ap:rest_api:manage',
  ],
  GetRESTAPISpec: ['ap:rest_api:manage', 'ap:rest_api:read'],
  getSecret: ['ap:secret:manage', 'ap:secret:read'],
  GetSubscription: ['ap:subscription:manage', 'ap:subscription:read'],
  GetSubscriptionPlan: ['ap:subscription_plan:manage', 'ap:subscription_plan:read'],
  HeadOrganization: ['ap:organization:manage', 'ap:organization:read'],
  ImportOpenAPI: ['ap:rest_api:create', 'ap:rest_api:manage'],
  ListApiPortals: ['ap:api_portal:manage', 'ap:api_portal:read'],
  listApiPublications: ['ap:api_publication:read'],
  ListApplicationAPIKeys: [
    'ap:api_key:all:manage',
    'ap:application:api_key:manage',
    'ap:application:api_key:read',
    'ap:application:manage',
  ],
  ListApplicationAssociationAPIKeys: [
    'ap:api_key:all:manage',
    'ap:application:association:api_key:read',
    'ap:application:association:manage',
    'ap:application:manage',
  ],
  ListApplicationAssociations: [
    'ap:application:association:manage',
    'ap:application:association:read',
    'ap:application:manage',
  ],
  ListApplications: ['ap:application:manage', 'ap:application:read'],
  ListGatewayCustomPolicies: ['ap:gateway_custom_policy:manage', 'ap:gateway_custom_policy:read'],
  ListGateways: ['ap:gateway:manage', 'ap:gateway:read'],
  listGatewayTokens: ['ap:gateway:manage', 'ap:gateway:token:manage', 'ap:gateway:token:read'],
  listLLMProviderAPIKeys: [
    'ap:api_key:all:manage',
    'ap:llm_provider:api_key:manage',
    'ap:llm_provider:api_key:read',
    'ap:llm_provider:manage',
  ],
  listLLMProviders: ['ap:llm_provider:manage', 'ap:llm_provider:read'],
  listLLMProviderTemplates: ['ap:llm_template:manage', 'ap:llm_template:read'],
  listLLMProxies: ['ap:llm_proxy:manage', 'ap:llm_proxy:read'],
  listLLMProxiesByProvider: [
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:deployment:read',
    'ap:llm_proxy:manage',
  ],
  listLLMProxyAPIKeys: [
    'ap:api_key:all:manage',
    'ap:llm_proxy:api_key:manage',
    'ap:llm_proxy:api_key:read',
    'ap:llm_proxy:manage',
  ],
  listMCPProxies: ['ap:mcp_proxy:manage', 'ap:mcp_proxy:read'],
  ListOrganizations: ['ap:organization:manage', 'ap:organization:read'],
  ListProjects: ['ap:project:manage', 'ap:project:read'],
  ListRESTAPIs: ['ap:rest_api:manage', 'ap:rest_api:read'],
  listSecrets: ['ap:secret:manage', 'ap:secret:read'],
  ListSubscriptionPlans: ['ap:subscription_plan:manage', 'ap:subscription_plan:read'],
  ListSubscriptions: ['ap:subscription:manage', 'ap:subscription:read'],
  listUserAPIKeys: ['ap:api_key:all:manage', 'ap:api_key:read'],
  publishRestApiToApiPortal: ['ap:api_portal:rest_api:manage', 'ap:api_portal:rest_api:publish'],
  RegisterOrganization: ['ap:organization:create', 'ap:organization:manage'],
  RemoveApplicationAPIKey: [
    'ap:api_key:all:manage',
    'ap:application:api_key:delete',
    'ap:application:api_key:manage',
    'ap:application:manage',
  ],
  RemoveApplicationAssociation: [
    'ap:application:association:delete',
    'ap:application:association:manage',
    'ap:application:manage',
  ],
  RestoreDeployment: [
    'ap:rest_api:deployment:manage',
    'ap:rest_api:deployment:restore',
    'ap:rest_api:manage',
  ],
  restoreLLMProviderDeployment: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:restore',
    'ap:llm_provider:manage',
  ],
  restoreLLMProxyDeployment: [
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:deployment:restore',
    'ap:llm_proxy:manage',
  ],
  RestoreMCPProxyDeployment: [
    'ap:mcp_proxy:deployment:manage',
    'ap:mcp_proxy:deployment:restore',
    'ap:mcp_proxy:manage',
  ],
  RevokeAPIKey: [
    'ap:api_key:all:manage',
    'ap:rest_api:api_key:delete',
    'ap:rest_api:api_key:manage',
    'ap:rest_api:manage',
  ],
  revokeGatewayToken: ['ap:gateway:manage', 'ap:gateway:token:delete', 'ap:gateway:token:manage'],
  rotateGatewayToken: ['ap:gateway:manage', 'ap:gateway:token:create', 'ap:gateway:token:manage'],
  rotateSecret: ['ap:secret:manage', 'ap:secret:update'],
  saveApiPublicationDraft: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:update'],
  saveApiPublicationDraftDefinition: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:update'],
  saveApiPublicationDraftLandingPage: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:update'],
  saveApiPublicationDraftThumbnail: ['ap:api_portal:draft:manage', 'ap:api_portal:draft:update'],
  setLLMProviderTemplateVersionEnabled: ['ap:llm_template:manage', 'ap:llm_template:update'],
  SyncCustomPolicy: ['ap:gateway_custom_policy:create', 'ap:gateway_custom_policy:manage'],
  UndeployDeployment: [
    'ap:rest_api:deployment:manage',
    'ap:rest_api:deployment:undeploy',
    'ap:rest_api:manage',
  ],
  undeployLLMProviderDeployment: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:undeploy',
    'ap:llm_provider:manage',
  ],
  undeployLLMProxyDeployment: [
    'ap:llm_proxy:deployment:manage',
    'ap:llm_proxy:deployment:undeploy',
    'ap:llm_proxy:manage',
  ],
  UndeployMCPProxyDeployment: [
    'ap:mcp_proxy:deployment:manage',
    'ap:mcp_proxy:deployment:undeploy',
    'ap:mcp_proxy:manage',
  ],
  unpublishRestApiFromApiPortal: [
    'ap:api_portal:rest_api:manage',
    'ap:api_portal:rest_api:unpublish',
  ],
  UpdateAPIKey: [
    'ap:api_key:all:manage',
    'ap:rest_api:api_key:manage',
    'ap:rest_api:api_key:update',
    'ap:rest_api:manage',
  ],
  UpdateApiPortal: ['ap:api_portal:manage', 'ap:api_portal:update'],
  UpdateApplication: ['ap:application:manage', 'ap:application:update'],
  UpdateGateway: ['ap:gateway:manage', 'ap:gateway:update'],
  updateLLMProvider: ['ap:llm_provider:manage', 'ap:llm_provider:update'],
  updateLLMProviderTemplate: ['ap:llm_template:manage', 'ap:llm_template:update'],
  updateLLMProxy: ['ap:llm_proxy:manage', 'ap:llm_proxy:update'],
  updateMCPProxy: ['ap:mcp_proxy:manage', 'ap:mcp_proxy:update'],
  UpdateProject: ['ap:project:manage', 'ap:project:update'],
  UpdateRESTAPI: ['ap:rest_api:manage', 'ap:rest_api:update'],
  UpdateRESTAPISpec: ['ap:rest_api:manage', 'ap:rest_api:update'],
  UpdateSubscription: ['ap:subscription:manage', 'ap:subscription:update'],
  UpdateSubscriptionPlan: ['ap:subscription_plan:manage', 'ap:subscription_plan:update'],
  ValidateOpenAPISpec: ['ap:rest_api:create', 'ap:rest_api:manage'],
} as const satisfies Record<keyof operations, readonly ApScope[]>;
