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
 * from platform-api/resources/openapi.yaml and
 * platform-api/resources/legacy-scope-mapping.yaml. Edit those, not this file.
 */

import type { operations } from './platform';

/** Every scope declared by the spec's OAuth2Security scheme. */
export type ApScope =
  | 'ap:api:agent:api_key:create'
  | 'ap:api:agent:api_key:delete'
  | 'ap:api:agent:api_key:manage'
  | 'ap:api:agent:api_key:read'
  | 'ap:api:agent:api_key:update'
  | 'ap:api:agent:build:create'
  | 'ap:api:agent:build:delete'
  | 'ap:api:agent:build:manage'
  | 'ap:api:agent:build:read'
  | 'ap:api:agent:create'
  | 'ap:api:agent:delete'
  | 'ap:api:agent:deployment:create'
  | 'ap:api:agent:deployment:delete'
  | 'ap:api:agent:deployment:manage'
  | 'ap:api:agent:deployment:read'
  | 'ap:api:agent:deployment:restore'
  | 'ap:api:agent:deployment:undeploy'
  | 'ap:api:agent:manage'
  | 'ap:api:agent:read'
  | 'ap:api:agent:update'
  | 'ap:api:graphql:api_key:create'
  | 'ap:api:graphql:api_key:delete'
  | 'ap:api:graphql:api_key:manage'
  | 'ap:api:graphql:api_key:update'
  | 'ap:api:graphql:build:create'
  | 'ap:api:graphql:build:delete'
  | 'ap:api:graphql:build:manage'
  | 'ap:api:graphql:build:read'
  | 'ap:api:graphql:create'
  | 'ap:api:graphql:delete'
  | 'ap:api:graphql:deployment:create'
  | 'ap:api:graphql:deployment:delete'
  | 'ap:api:graphql:deployment:manage'
  | 'ap:api:graphql:deployment:read'
  | 'ap:api:graphql:deployment:restore'
  | 'ap:api:graphql:deployment:undeploy'
  | 'ap:api:graphql:gateway:create'
  | 'ap:api:graphql:gateway:manage'
  | 'ap:api:graphql:gateway:read'
  | 'ap:api:graphql:manage'
  | 'ap:api:graphql:read'
  | 'ap:api:graphql:update'
  | 'ap:api:llm:api_key:create'
  | 'ap:api:llm:api_key:delete'
  | 'ap:api:llm:api_key:manage'
  | 'ap:api:llm:api_key:read'
  | 'ap:api:llm:build:create'
  | 'ap:api:llm:build:delete'
  | 'ap:api:llm:build:manage'
  | 'ap:api:llm:build:read'
  | 'ap:api:llm:create'
  | 'ap:api:llm:delete'
  | 'ap:api:llm:deployment:create'
  | 'ap:api:llm:deployment:delete'
  | 'ap:api:llm:deployment:manage'
  | 'ap:api:llm:deployment:read'
  | 'ap:api:llm:deployment:restore'
  | 'ap:api:llm:deployment:undeploy'
  | 'ap:api:llm:manage'
  | 'ap:api:llm:read'
  | 'ap:api:llm:update'
  | 'ap:api:manage'
  | 'ap:api:mcp:build:create'
  | 'ap:api:mcp:build:delete'
  | 'ap:api:mcp:build:manage'
  | 'ap:api:mcp:build:read'
  | 'ap:api:mcp:create'
  | 'ap:api:mcp:delete'
  | 'ap:api:mcp:deployment:create'
  | 'ap:api:mcp:deployment:delete'
  | 'ap:api:mcp:deployment:manage'
  | 'ap:api:mcp:deployment:read'
  | 'ap:api:mcp:deployment:restore'
  | 'ap:api:mcp:deployment:undeploy'
  | 'ap:api:mcp:manage'
  | 'ap:api:mcp:read'
  | 'ap:api:mcp:update'
  | 'ap:api:read'
  | 'ap:api:rest:api_key:create'
  | 'ap:api:rest:api_key:delete'
  | 'ap:api:rest:api_key:manage'
  | 'ap:api:rest:api_key:update'
  | 'ap:api:rest:build:create'
  | 'ap:api:rest:build:delete'
  | 'ap:api:rest:build:manage'
  | 'ap:api:rest:build:read'
  | 'ap:api:rest:create'
  | 'ap:api:rest:delete'
  | 'ap:api:rest:deployment:create'
  | 'ap:api:rest:deployment:delete'
  | 'ap:api:rest:deployment:manage'
  | 'ap:api:rest:deployment:read'
  | 'ap:api:rest:deployment:restore'
  | 'ap:api:rest:deployment:undeploy'
  | 'ap:api:rest:gateway:create'
  | 'ap:api:rest:gateway:manage'
  | 'ap:api:rest:gateway:read'
  | 'ap:api:rest:manage'
  | 'ap:api:rest:read'
  | 'ap:api:rest:update'
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
  | 'ap:content:document:manage'
  | 'ap:content:document:read'
  | 'ap:content:manage'
  | 'ap:content:read'
  | 'ap:content:thumbnail:manage'
  | 'ap:content:thumbnail:read'
  | 'ap:credential:api_key:all:manage'
  | 'ap:credential:api_key:read'
  | 'ap:credential:secret:create'
  | 'ap:credential:secret:delete'
  | 'ap:credential:secret:manage'
  | 'ap:credential:secret:read'
  | 'ap:credential:secret:update'
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
  | 'ap:gateway_policy:create'
  | 'ap:gateway_policy:delete'
  | 'ap:gateway_policy:manage'
  | 'ap:gateway_policy:read'
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
  | 'ap:llm_provider:template:create'
  | 'ap:llm_provider:template:delete'
  | 'ap:llm_provider:template:manage'
  | 'ap:llm_provider:template:read'
  | 'ap:llm_provider:template:update'
  | 'ap:llm_provider:update'
  | 'ap:manage'
  | 'ap:meter:create'
  | 'ap:meter:delete'
  | 'ap:meter:manage'
  | 'ap:meter:read'
  | 'ap:meter:update'
  | 'ap:organization:create'
  | 'ap:organization:manage'
  | 'ap:organization:read'
  | 'ap:portal:create'
  | 'ap:portal:delete'
  | 'ap:portal:draft:manage'
  | 'ap:portal:draft:read'
  | 'ap:portal:draft:update'
  | 'ap:portal:graphql_api:deprecate'
  | 'ap:portal:graphql_api:manage'
  | 'ap:portal:graphql_api:publish'
  | 'ap:portal:graphql_api:unpublish'
  | 'ap:portal:manage'
  | 'ap:portal:mcp_proxy:manage'
  | 'ap:portal:mcp_proxy:publish'
  | 'ap:portal:mcp_proxy:unpublish'
  | 'ap:portal:publication:read'
  | 'ap:portal:read'
  | 'ap:portal:rest_api:deprecate'
  | 'ap:portal:rest_api:manage'
  | 'ap:portal:rest_api:publish'
  | 'ap:portal:rest_api:unpublish'
  | 'ap:portal:update'
  | 'ap:project:create'
  | 'ap:project:delete'
  | 'ap:project:manage'
  | 'ap:project:read'
  | 'ap:project:update'
  | 'ap:publication:read'
  | 'ap:read'
  | 'ap:subscription:create'
  | 'ap:subscription:delete'
  | 'ap:subscription:manage'
  | 'ap:subscription:read'
  | 'ap:subscription:update'
  | 'ap:subscription_plan:create'
  | 'ap:subscription_plan:delete'
  | 'ap:subscription_plan:manage'
  | 'ap:subscription_plan:pricing:read'
  | 'ap:subscription_plan:read'
  | 'ap:subscription_plan:update';

/**
 * The full scope catalog, for building test personas and for validating an
 * operator-supplied scope string. Sorted, so it is diff-stable.
 */
export const AP_SCOPES: readonly ApScope[] = [
  'ap:api:agent:api_key:create',
  'ap:api:agent:api_key:delete',
  'ap:api:agent:api_key:manage',
  'ap:api:agent:api_key:read',
  'ap:api:agent:api_key:update',
  'ap:api:agent:build:create',
  'ap:api:agent:build:delete',
  'ap:api:agent:build:manage',
  'ap:api:agent:build:read',
  'ap:api:agent:create',
  'ap:api:agent:delete',
  'ap:api:agent:deployment:create',
  'ap:api:agent:deployment:delete',
  'ap:api:agent:deployment:manage',
  'ap:api:agent:deployment:read',
  'ap:api:agent:deployment:restore',
  'ap:api:agent:deployment:undeploy',
  'ap:api:agent:manage',
  'ap:api:agent:read',
  'ap:api:agent:update',
  'ap:api:graphql:api_key:create',
  'ap:api:graphql:api_key:delete',
  'ap:api:graphql:api_key:manage',
  'ap:api:graphql:api_key:update',
  'ap:api:graphql:build:create',
  'ap:api:graphql:build:delete',
  'ap:api:graphql:build:manage',
  'ap:api:graphql:build:read',
  'ap:api:graphql:create',
  'ap:api:graphql:delete',
  'ap:api:graphql:deployment:create',
  'ap:api:graphql:deployment:delete',
  'ap:api:graphql:deployment:manage',
  'ap:api:graphql:deployment:read',
  'ap:api:graphql:deployment:restore',
  'ap:api:graphql:deployment:undeploy',
  'ap:api:graphql:gateway:create',
  'ap:api:graphql:gateway:manage',
  'ap:api:graphql:gateway:read',
  'ap:api:graphql:manage',
  'ap:api:graphql:read',
  'ap:api:graphql:update',
  'ap:api:llm:api_key:create',
  'ap:api:llm:api_key:delete',
  'ap:api:llm:api_key:manage',
  'ap:api:llm:api_key:read',
  'ap:api:llm:build:create',
  'ap:api:llm:build:delete',
  'ap:api:llm:build:manage',
  'ap:api:llm:build:read',
  'ap:api:llm:create',
  'ap:api:llm:delete',
  'ap:api:llm:deployment:create',
  'ap:api:llm:deployment:delete',
  'ap:api:llm:deployment:manage',
  'ap:api:llm:deployment:read',
  'ap:api:llm:deployment:restore',
  'ap:api:llm:deployment:undeploy',
  'ap:api:llm:manage',
  'ap:api:llm:read',
  'ap:api:llm:update',
  'ap:api:manage',
  'ap:api:mcp:build:create',
  'ap:api:mcp:build:delete',
  'ap:api:mcp:build:manage',
  'ap:api:mcp:build:read',
  'ap:api:mcp:create',
  'ap:api:mcp:delete',
  'ap:api:mcp:deployment:create',
  'ap:api:mcp:deployment:delete',
  'ap:api:mcp:deployment:manage',
  'ap:api:mcp:deployment:read',
  'ap:api:mcp:deployment:restore',
  'ap:api:mcp:deployment:undeploy',
  'ap:api:mcp:manage',
  'ap:api:mcp:read',
  'ap:api:mcp:update',
  'ap:api:read',
  'ap:api:rest:api_key:create',
  'ap:api:rest:api_key:delete',
  'ap:api:rest:api_key:manage',
  'ap:api:rest:api_key:update',
  'ap:api:rest:build:create',
  'ap:api:rest:build:delete',
  'ap:api:rest:build:manage',
  'ap:api:rest:build:read',
  'ap:api:rest:create',
  'ap:api:rest:delete',
  'ap:api:rest:deployment:create',
  'ap:api:rest:deployment:delete',
  'ap:api:rest:deployment:manage',
  'ap:api:rest:deployment:read',
  'ap:api:rest:deployment:restore',
  'ap:api:rest:deployment:undeploy',
  'ap:api:rest:gateway:create',
  'ap:api:rest:gateway:manage',
  'ap:api:rest:gateway:read',
  'ap:api:rest:manage',
  'ap:api:rest:read',
  'ap:api:rest:update',
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
  'ap:content:document:manage',
  'ap:content:document:read',
  'ap:content:manage',
  'ap:content:read',
  'ap:content:thumbnail:manage',
  'ap:content:thumbnail:read',
  'ap:credential:api_key:all:manage',
  'ap:credential:api_key:read',
  'ap:credential:secret:create',
  'ap:credential:secret:delete',
  'ap:credential:secret:manage',
  'ap:credential:secret:read',
  'ap:credential:secret:update',
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
  'ap:gateway_policy:create',
  'ap:gateway_policy:delete',
  'ap:gateway_policy:manage',
  'ap:gateway_policy:read',
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
  'ap:llm_provider:template:create',
  'ap:llm_provider:template:delete',
  'ap:llm_provider:template:manage',
  'ap:llm_provider:template:read',
  'ap:llm_provider:template:update',
  'ap:llm_provider:update',
  'ap:manage',
  'ap:meter:create',
  'ap:meter:delete',
  'ap:meter:manage',
  'ap:meter:read',
  'ap:meter:update',
  'ap:organization:create',
  'ap:organization:manage',
  'ap:organization:read',
  'ap:portal:create',
  'ap:portal:delete',
  'ap:portal:draft:manage',
  'ap:portal:draft:read',
  'ap:portal:draft:update',
  'ap:portal:graphql_api:deprecate',
  'ap:portal:graphql_api:manage',
  'ap:portal:graphql_api:publish',
  'ap:portal:graphql_api:unpublish',
  'ap:portal:manage',
  'ap:portal:mcp_proxy:manage',
  'ap:portal:mcp_proxy:publish',
  'ap:portal:mcp_proxy:unpublish',
  'ap:portal:publication:read',
  'ap:portal:read',
  'ap:portal:rest_api:deprecate',
  'ap:portal:rest_api:manage',
  'ap:portal:rest_api:publish',
  'ap:portal:rest_api:unpublish',
  'ap:portal:update',
  'ap:project:create',
  'ap:project:delete',
  'ap:project:manage',
  'ap:project:read',
  'ap:project:update',
  'ap:publication:read',
  'ap:read',
  'ap:subscription:create',
  'ap:subscription:delete',
  'ap:subscription:manage',
  'ap:subscription:read',
  'ap:subscription:update',
  'ap:subscription_plan:create',
  'ap:subscription_plan:delete',
  'ap:subscription_plan:manage',
  'ap:subscription_plan:pricing:read',
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
    'ap:application:api_key:create',
    'ap:application:api_key:manage',
    'ap:application:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  AddApplicationAssociations: [
    'ap:application:association:create',
    'ap:application:association:manage',
    'ap:application:manage',
    'ap:manage',
  ],
  AddGatewaysToAPI: [
    'ap:api:manage',
    'ap:api:rest:gateway:create',
    'ap:api:rest:gateway:manage',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  AddGatewaysToGraphQLAPI: [
    'ap:api:graphql:gateway:create',
    'ap:api:graphql:gateway:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  copyLLMProviderTemplateVersion: [
    'ap:llm_provider:manage',
    'ap:llm_provider:template:create',
    'ap:llm_provider:template:manage',
    'ap:manage',
  ],
  createAgentProxy: ['ap:api:agent:create', 'ap:api:agent:manage', 'ap:api:manage', 'ap:manage'],
  createAgentProxyAPIKey: [
    'ap:api:agent:api_key:create',
    'ap:api:agent:api_key:manage',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  createAgentProxyBuild: [
    'ap:api:agent:build:create',
    'ap:api:agent:build:manage',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  createAgentProxyDeployment: [
    'ap:api:agent:deployment:create',
    'ap:api:agent:deployment:manage',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  CreateAPIDocument: ['ap:content:document:manage', 'ap:content:manage', 'ap:manage'],
  CreateAPIKey: [
    'ap:api:manage',
    'ap:api:rest:api_key:create',
    'ap:api:rest:api_key:manage',
    'ap:api:rest:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  CreateApiPortal: ['ap:manage', 'ap:portal:create', 'ap:portal:manage'],
  CreateApplication: ['ap:application:create', 'ap:application:manage', 'ap:manage'],
  CreateBuild: [
    'ap:api:manage',
    'ap:api:rest:build:create',
    'ap:api:rest:build:manage',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  CreateGateway: ['ap:gateway:create', 'ap:gateway:manage', 'ap:manage'],
  CreateGraphQLAPI: [
    'ap:api:graphql:create',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  CreateGraphQLAPIBuild: [
    'ap:api:graphql:build:create',
    'ap:api:graphql:build:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  CreateGraphQLAPIKey: [
    'ap:api:graphql:api_key:create',
    'ap:api:graphql:api_key:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  createLLMProvider: ['ap:llm_provider:create', 'ap:llm_provider:manage', 'ap:manage'],
  createLLMProviderAPIKey: [
    'ap:credential:api_key:all:manage',
    'ap:llm_provider:api_key:create',
    'ap:llm_provider:api_key:manage',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  CreateLLMProviderBuild: [
    'ap:llm_provider:build:create',
    'ap:llm_provider:build:manage',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  createLLMProviderTemplate: [
    'ap:llm_provider:manage',
    'ap:llm_provider:template:create',
    'ap:llm_provider:template:manage',
    'ap:manage',
  ],
  createLLMProxy: ['ap:api:llm:create', 'ap:api:llm:manage', 'ap:api:manage', 'ap:manage'],
  createLLMProxyAPIKey: [
    'ap:api:llm:api_key:create',
    'ap:api:llm:api_key:manage',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  CreateLLMProxyBuild: [
    'ap:api:llm:build:create',
    'ap:api:llm:build:manage',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  createMCPProxy: ['ap:api:manage', 'ap:api:mcp:create', 'ap:api:mcp:manage', 'ap:manage'],
  CreateMCPProxyBuild: [
    'ap:api:manage',
    'ap:api:mcp:build:create',
    'ap:api:mcp:build:manage',
    'ap:api:mcp:manage',
    'ap:manage',
  ],
  CreateMeter: ['ap:manage', 'ap:meter:create', 'ap:meter:manage'],
  CreateProject: ['ap:manage', 'ap:project:create', 'ap:project:manage'],
  CreateRESTAPI: ['ap:api:manage', 'ap:api:rest:create', 'ap:api:rest:manage', 'ap:manage'],
  createSecret: ['ap:credential:secret:create', 'ap:credential:secret:manage', 'ap:manage'],
  CreateSubscription: ['ap:manage', 'ap:subscription:create', 'ap:subscription:manage'],
  CreateSubscriptionPlan: [
    'ap:manage',
    'ap:subscription_plan:create',
    'ap:subscription_plan:manage',
  ],
  deleteAgentProxy: ['ap:api:agent:delete', 'ap:api:agent:manage', 'ap:api:manage', 'ap:manage'],
  deleteAgentProxyBuild: [
    'ap:api:agent:build:delete',
    'ap:api:agent:build:manage',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  deleteAgentProxyDeployment: [
    'ap:api:agent:deployment:delete',
    'ap:api:agent:deployment:manage',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  DeleteAPIDocument: ['ap:content:document:manage', 'ap:content:manage', 'ap:manage'],
  DeleteApiPortal: ['ap:manage', 'ap:portal:delete', 'ap:portal:manage'],
  DeleteAPIThumbnail: ['ap:content:manage', 'ap:content:thumbnail:manage', 'ap:manage'],
  DeleteApplication: ['ap:application:delete', 'ap:application:manage', 'ap:manage'],
  DeleteBuild: [
    'ap:api:manage',
    'ap:api:rest:build:delete',
    'ap:api:rest:build:manage',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  DeleteDeployment: [
    'ap:api:manage',
    'ap:api:rest:deployment:delete',
    'ap:api:rest:deployment:manage',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  DeleteGateway: ['ap:gateway:delete', 'ap:gateway:manage', 'ap:manage'],
  DeleteGatewayCustomPolicy: ['ap:gateway_policy:delete', 'ap:gateway_policy:manage', 'ap:manage'],
  DeleteGraphQLAPI: [
    'ap:api:graphql:delete',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  DeleteGraphQLAPIBuild: [
    'ap:api:graphql:build:delete',
    'ap:api:graphql:build:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  DeleteGraphQLAPIDeployment: [
    'ap:api:graphql:deployment:delete',
    'ap:api:graphql:deployment:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  deleteLLMProvider: ['ap:llm_provider:delete', 'ap:llm_provider:manage', 'ap:manage'],
  deleteLLMProviderAPIKey: [
    'ap:credential:api_key:all:manage',
    'ap:llm_provider:api_key:delete',
    'ap:llm_provider:api_key:manage',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  DeleteLLMProviderBuild: [
    'ap:llm_provider:build:delete',
    'ap:llm_provider:build:manage',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  deleteLLMProviderDeployment: [
    'ap:llm_provider:deployment:delete',
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  deleteLLMProviderTemplateVersion: [
    'ap:llm_provider:manage',
    'ap:llm_provider:template:delete',
    'ap:llm_provider:template:manage',
    'ap:manage',
  ],
  deleteLLMProxy: ['ap:api:llm:delete', 'ap:api:llm:manage', 'ap:api:manage', 'ap:manage'],
  deleteLLMProxyAPIKey: [
    'ap:api:llm:api_key:delete',
    'ap:api:llm:api_key:manage',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  DeleteLLMProxyBuild: [
    'ap:api:llm:build:delete',
    'ap:api:llm:build:manage',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  deleteLLMProxyDeployment: [
    'ap:api:llm:deployment:delete',
    'ap:api:llm:deployment:manage',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  deleteMCPProxy: ['ap:api:manage', 'ap:api:mcp:delete', 'ap:api:mcp:manage', 'ap:manage'],
  DeleteMCPProxyBuild: [
    'ap:api:manage',
    'ap:api:mcp:build:delete',
    'ap:api:mcp:build:manage',
    'ap:api:mcp:manage',
    'ap:manage',
  ],
  DeleteMCPProxyDeployment: [
    'ap:api:manage',
    'ap:api:mcp:deployment:delete',
    'ap:api:mcp:deployment:manage',
    'ap:api:mcp:manage',
    'ap:manage',
  ],
  DeleteMeter: ['ap:manage', 'ap:meter:delete', 'ap:meter:manage'],
  DeleteProject: ['ap:manage', 'ap:project:delete', 'ap:project:manage'],
  DeleteRESTAPI: ['ap:api:manage', 'ap:api:rest:delete', 'ap:api:rest:manage', 'ap:manage'],
  deleteSecret: ['ap:credential:secret:delete', 'ap:credential:secret:manage', 'ap:manage'],
  DeleteSubscription: ['ap:manage', 'ap:subscription:delete', 'ap:subscription:manage'],
  DeleteSubscriptionPlan: [
    'ap:manage',
    'ap:subscription_plan:delete',
    'ap:subscription_plan:manage',
  ],
  DeployAPI: [
    'ap:api:manage',
    'ap:api:rest:deployment:create',
    'ap:api:rest:deployment:manage',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  DeployGraphQLAPI: [
    'ap:api:graphql:deployment:create',
    'ap:api:graphql:deployment:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  deployLLMProvider: [
    'ap:llm_provider:deployment:create',
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  deployLLMProxy: [
    'ap:api:llm:deployment:create',
    'ap:api:llm:deployment:manage',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  DeployMCPProxy: [
    'ap:api:manage',
    'ap:api:mcp:deployment:create',
    'ap:api:mcp:deployment:manage',
    'ap:api:mcp:manage',
    'ap:manage',
  ],
  deprecateGraphQLApiOnApiPortal: [
    'ap:manage',
    'ap:portal:graphql_api:deprecate',
    'ap:portal:graphql_api:manage',
    'ap:portal:manage',
  ],
  deprecateRestApiOnApiPortal: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:rest_api:deprecate',
    'ap:portal:rest_api:manage',
  ],
  fetchAgentCard: [
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  fetchMCPProxyServerInfo: [
    'ap:api:manage',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getAgentProxy: [
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getAgentProxyBuild: [
    'ap:api:agent:build:manage',
    'ap:api:agent:build:read',
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getAgentProxyDeployment: [
    'ap:api:agent:deployment:manage',
    'ap:api:agent:deployment:read',
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetAPIDocument: [
    'ap:content:document:manage',
    'ap:content:document:read',
    'ap:content:manage',
    'ap:content:read',
    'ap:manage',
    'ap:read',
  ],
  GetAPIDocumentContent: [
    'ap:content:document:manage',
    'ap:content:document:read',
    'ap:content:manage',
    'ap:content:read',
    'ap:manage',
    'ap:read',
  ],
  GetApiPortal: ['ap:manage', 'ap:portal:manage', 'ap:portal:read', 'ap:read'],
  getApiPublication: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:publication:read',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationDefinition: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:publication:read',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationDraft: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:read',
    'ap:portal:manage',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationDraftDefinition: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:read',
    'ap:portal:manage',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationDraftLandingPage: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:read',
    'ap:portal:manage',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationDraftThumbnail: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:read',
    'ap:portal:manage',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationLandingPage: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:publication:read',
    'ap:portal:read',
    'ap:read',
  ],
  getApiPublicationThumbnail: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:publication:read',
    'ap:portal:read',
    'ap:read',
  ],
  GetAPIThumbnail: [
    'ap:content:manage',
    'ap:content:read',
    'ap:content:thumbnail:manage',
    'ap:content:thumbnail:read',
    'ap:manage',
    'ap:read',
  ],
  GetApplication: ['ap:application:manage', 'ap:application:read', 'ap:manage', 'ap:read'],
  GetBuild: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:build:manage',
    'ap:api:rest:build:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  GetBuilds: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:build:manage',
    'ap:api:rest:build:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  GetDeployment: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:deployment:manage',
    'ap:api:rest:deployment:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  GetDeployments: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:deployment:manage',
    'ap:api:rest:deployment:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  GetGateway: ['ap:gateway:manage', 'ap:gateway:read', 'ap:manage', 'ap:read'],
  GetGatewayCustomPolicy: [
    'ap:gateway_policy:manage',
    'ap:gateway_policy:read',
    'ap:manage',
    'ap:read',
  ],
  GetGatewayManifest: [
    'ap:gateway:manage',
    'ap:gateway:manifest:read',
    'ap:gateway:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPI: [
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPIBuild: [
    'ap:api:graphql:build:manage',
    'ap:api:graphql:build:read',
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPIBuilds: [
    'ap:api:graphql:build:manage',
    'ap:api:graphql:build:read',
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPIDeployment: [
    'ap:api:graphql:deployment:manage',
    'ap:api:graphql:deployment:read',
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPIDeployments: [
    'ap:api:graphql:deployment:manage',
    'ap:api:graphql:deployment:read',
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPIGateways: [
    'ap:api:graphql:gateway:manage',
    'ap:api:graphql:gateway:read',
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:gateway:manage',
    'ap:gateway:read',
    'ap:manage',
    'ap:read',
  ],
  GetGraphQLAPISDL: [
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProvider: ['ap:llm_provider:manage', 'ap:llm_provider:read', 'ap:manage', 'ap:read'],
  GetLLMProviderBuild: [
    'ap:llm_provider:build:manage',
    'ap:llm_provider:build:read',
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:manage',
    'ap:read',
  ],
  GetLLMProviderBuilds: [
    'ap:llm_provider:build:manage',
    'ap:llm_provider:build:read',
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProviderDeployment: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:read',
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProviderDeployments: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:read',
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProviderTemplate: [
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:llm_provider:template:manage',
    'ap:llm_provider:template:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProxy: [
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetLLMProxyBuild: [
    'ap:api:llm:build:manage',
    'ap:api:llm:build:read',
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetLLMProxyBuilds: [
    'ap:api:llm:build:manage',
    'ap:api:llm:build:read',
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProxyDeployment: [
    'ap:api:llm:deployment:manage',
    'ap:api:llm:deployment:read',
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getLLMProxyDeployments: [
    'ap:api:llm:deployment:manage',
    'ap:api:llm:deployment:read',
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  getMCPProxy: [
    'ap:api:manage',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetMCPProxyBuild: [
    'ap:api:manage',
    'ap:api:mcp:build:manage',
    'ap:api:mcp:build:read',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetMCPProxyBuilds: [
    'ap:api:manage',
    'ap:api:mcp:build:manage',
    'ap:api:mcp:build:read',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetMCPProxyDeployment: [
    'ap:api:manage',
    'ap:api:mcp:deployment:manage',
    'ap:api:mcp:deployment:read',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetMCPProxyDeployments: [
    'ap:api:manage',
    'ap:api:mcp:deployment:manage',
    'ap:api:mcp:deployment:read',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  GetMeter: ['ap:manage', 'ap:meter:manage', 'ap:meter:read', 'ap:read'],
  GetOrganization: ['ap:manage', 'ap:organization:manage', 'ap:organization:read', 'ap:read'],
  GetProject: ['ap:manage', 'ap:project:manage', 'ap:project:read', 'ap:read'],
  GetRESTAPI: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  GetRESTAPIGateways: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:gateway:manage',
    'ap:api:rest:gateway:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:gateway:manage',
    'ap:gateway:read',
    'ap:manage',
    'ap:read',
  ],
  GetRESTAPISpec: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  getSecret: ['ap:credential:secret:manage', 'ap:credential:secret:read', 'ap:manage', 'ap:read'],
  GetSubscription: ['ap:manage', 'ap:read', 'ap:subscription:manage', 'ap:subscription:read'],
  GetSubscriptionPlan: [
    'ap:manage',
    'ap:read',
    'ap:subscription_plan:manage',
    'ap:subscription_plan:read',
  ],
  GetSubscriptionPlanPricing: [
    'ap:manage',
    'ap:read',
    'ap:subscription_plan:manage',
    'ap:subscription_plan:pricing:read',
    'ap:subscription_plan:read',
  ],
  HeadOrganization: ['ap:manage', 'ap:organization:manage', 'ap:organization:read', 'ap:read'],
  ImportOpenAPI: ['ap:api:manage', 'ap:api:rest:create', 'ap:api:rest:manage', 'ap:manage'],
  listAgentProxies: [
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  listAgentProxyAPIKeys: [
    'ap:api:agent:api_key:manage',
    'ap:api:agent:api_key:read',
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:credential:api_key:all:manage',
    'ap:manage',
    'ap:read',
  ],
  listAgentProxyBuilds: [
    'ap:api:agent:build:manage',
    'ap:api:agent:build:read',
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  listAgentProxyDeployments: [
    'ap:api:agent:deployment:manage',
    'ap:api:agent:deployment:read',
    'ap:api:agent:manage',
    'ap:api:agent:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  ListAPIDocuments: [
    'ap:content:document:manage',
    'ap:content:document:read',
    'ap:content:manage',
    'ap:content:read',
    'ap:manage',
    'ap:read',
  ],
  ListApiPortals: ['ap:manage', 'ap:portal:manage', 'ap:portal:read', 'ap:read'],
  listApiPublications: ['ap:manage', 'ap:publication:read', 'ap:read'],
  ListApplicationAPIKeys: [
    'ap:application:api_key:manage',
    'ap:application:api_key:read',
    'ap:application:manage',
    'ap:application:read',
    'ap:credential:api_key:all:manage',
    'ap:manage',
    'ap:read',
  ],
  ListApplicationAssociationAPIKeys: [
    'ap:application:association:api_key:read',
    'ap:application:association:manage',
    'ap:application:association:read',
    'ap:application:manage',
    'ap:application:read',
    'ap:credential:api_key:all:manage',
    'ap:manage',
    'ap:read',
  ],
  ListApplicationAssociations: [
    'ap:application:association:manage',
    'ap:application:association:read',
    'ap:application:manage',
    'ap:application:read',
    'ap:manage',
    'ap:read',
  ],
  ListApplications: ['ap:application:manage', 'ap:application:read', 'ap:manage', 'ap:read'],
  ListGatewayCustomPolicies: [
    'ap:gateway_policy:manage',
    'ap:gateway_policy:read',
    'ap:manage',
    'ap:read',
  ],
  ListGateways: ['ap:gateway:manage', 'ap:gateway:read', 'ap:manage', 'ap:read'],
  listGatewayTokens: [
    'ap:gateway:manage',
    'ap:gateway:read',
    'ap:gateway:token:manage',
    'ap:gateway:token:read',
    'ap:manage',
    'ap:read',
  ],
  ListGraphQLAPIs: [
    'ap:api:graphql:manage',
    'ap:api:graphql:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  listLLMProviderAPIKeys: [
    'ap:credential:api_key:all:manage',
    'ap:llm_provider:api_key:manage',
    'ap:llm_provider:api_key:read',
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:manage',
    'ap:read',
  ],
  listLLMProviders: ['ap:llm_provider:manage', 'ap:llm_provider:read', 'ap:manage', 'ap:read'],
  listLLMProviderTemplates: [
    'ap:llm_provider:manage',
    'ap:llm_provider:read',
    'ap:llm_provider:template:manage',
    'ap:llm_provider:template:read',
    'ap:manage',
    'ap:read',
  ],
  listLLMProxies: [
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  listLLMProxiesByProvider: [
    'ap:api:llm:deployment:manage',
    'ap:api:llm:deployment:read',
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  listLLMProxyAPIKeys: [
    'ap:api:llm:api_key:manage',
    'ap:api:llm:api_key:read',
    'ap:api:llm:manage',
    'ap:api:llm:read',
    'ap:api:manage',
    'ap:api:read',
    'ap:credential:api_key:all:manage',
    'ap:manage',
    'ap:read',
  ],
  listMCPProxies: [
    'ap:api:manage',
    'ap:api:mcp:manage',
    'ap:api:mcp:read',
    'ap:api:read',
    'ap:manage',
    'ap:read',
  ],
  ListMeters: ['ap:manage', 'ap:meter:manage', 'ap:meter:read', 'ap:read'],
  ListOrganizations: ['ap:manage', 'ap:organization:manage', 'ap:organization:read', 'ap:read'],
  ListProjects: ['ap:manage', 'ap:project:manage', 'ap:project:read', 'ap:read'],
  ListRESTAPIs: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  ListRESTAPIVersions: [
    'ap:api:manage',
    'ap:api:read',
    'ap:api:rest:manage',
    'ap:api:rest:read',
    'ap:manage',
    'ap:read',
  ],
  listSecrets: ['ap:credential:secret:manage', 'ap:credential:secret:read', 'ap:manage', 'ap:read'],
  ListSubscriptionPlans: [
    'ap:manage',
    'ap:read',
    'ap:subscription_plan:manage',
    'ap:subscription_plan:read',
  ],
  ListSubscriptions: ['ap:manage', 'ap:read', 'ap:subscription:manage', 'ap:subscription:read'],
  listUserAPIKeys: [
    'ap:credential:api_key:all:manage',
    'ap:credential:api_key:read',
    'ap:manage',
    'ap:read',
  ],
  publishGraphQLApiToApiPortal: [
    'ap:manage',
    'ap:portal:graphql_api:manage',
    'ap:portal:graphql_api:publish',
    'ap:portal:manage',
  ],
  publishRestApiToApiPortal: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:rest_api:manage',
    'ap:portal:rest_api:publish',
  ],
  RegisterOrganization: ['ap:manage', 'ap:organization:create', 'ap:organization:manage'],
  RemoveApplicationAPIKey: [
    'ap:application:api_key:delete',
    'ap:application:api_key:manage',
    'ap:application:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  RemoveApplicationAssociation: [
    'ap:application:association:delete',
    'ap:application:association:manage',
    'ap:application:manage',
    'ap:manage',
  ],
  restoreAgentProxyDeployment: [
    'ap:api:agent:deployment:manage',
    'ap:api:agent:deployment:restore',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  RestoreDeployment: [
    'ap:api:manage',
    'ap:api:rest:deployment:manage',
    'ap:api:rest:deployment:restore',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  RestoreGraphQLAPIDeployment: [
    'ap:api:graphql:deployment:manage',
    'ap:api:graphql:deployment:restore',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  restoreLLMProviderDeployment: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:restore',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  restoreLLMProxyDeployment: [
    'ap:api:llm:deployment:manage',
    'ap:api:llm:deployment:restore',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  RestoreMCPProxyDeployment: [
    'ap:api:manage',
    'ap:api:mcp:deployment:manage',
    'ap:api:mcp:deployment:restore',
    'ap:api:mcp:manage',
    'ap:manage',
  ],
  revokeAgentProxyAPIKey: [
    'ap:api:agent:api_key:delete',
    'ap:api:agent:api_key:manage',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  RevokeAPIKey: [
    'ap:api:manage',
    'ap:api:rest:api_key:delete',
    'ap:api:rest:api_key:manage',
    'ap:api:rest:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  revokeGatewayToken: [
    'ap:gateway:manage',
    'ap:gateway:token:delete',
    'ap:gateway:token:manage',
    'ap:manage',
  ],
  RevokeGraphQLAPIKey: [
    'ap:api:graphql:api_key:delete',
    'ap:api:graphql:api_key:manage',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  rotateGatewayToken: [
    'ap:gateway:manage',
    'ap:gateway:token:create',
    'ap:gateway:token:manage',
    'ap:manage',
  ],
  rotateSecret: ['ap:credential:secret:manage', 'ap:credential:secret:update', 'ap:manage'],
  saveApiPublicationDraft: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:update',
    'ap:portal:manage',
  ],
  saveApiPublicationDraftDefinition: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:update',
    'ap:portal:manage',
  ],
  saveApiPublicationDraftLandingPage: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:update',
    'ap:portal:manage',
  ],
  saveApiPublicationDraftThumbnail: [
    'ap:manage',
    'ap:portal:draft:manage',
    'ap:portal:draft:update',
    'ap:portal:manage',
  ],
  setLLMProviderTemplateVersionEnabled: [
    'ap:llm_provider:manage',
    'ap:llm_provider:template:manage',
    'ap:llm_provider:template:update',
    'ap:manage',
  ],
  SyncCustomPolicy: ['ap:gateway_policy:create', 'ap:gateway_policy:manage', 'ap:manage'],
  undeployAgentProxyDeployment: [
    'ap:api:agent:deployment:manage',
    'ap:api:agent:deployment:undeploy',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  UndeployDeployment: [
    'ap:api:manage',
    'ap:api:rest:deployment:manage',
    'ap:api:rest:deployment:undeploy',
    'ap:api:rest:manage',
    'ap:manage',
  ],
  UndeployGraphQLAPIDeployment: [
    'ap:api:graphql:deployment:manage',
    'ap:api:graphql:deployment:undeploy',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  undeployLLMProviderDeployment: [
    'ap:llm_provider:deployment:manage',
    'ap:llm_provider:deployment:undeploy',
    'ap:llm_provider:manage',
    'ap:manage',
  ],
  undeployLLMProxyDeployment: [
    'ap:api:llm:deployment:manage',
    'ap:api:llm:deployment:undeploy',
    'ap:api:llm:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  UndeployMCPProxyDeployment: [
    'ap:api:manage',
    'ap:api:mcp:deployment:manage',
    'ap:api:mcp:deployment:undeploy',
    'ap:api:mcp:manage',
    'ap:manage',
  ],
  unpublishGraphQLApiFromApiPortal: [
    'ap:manage',
    'ap:portal:graphql_api:manage',
    'ap:portal:graphql_api:unpublish',
    'ap:portal:manage',
  ],
  unpublishRestApiFromApiPortal: [
    'ap:manage',
    'ap:portal:manage',
    'ap:portal:rest_api:manage',
    'ap:portal:rest_api:unpublish',
  ],
  updateAgentProxy: ['ap:api:agent:manage', 'ap:api:agent:update', 'ap:api:manage', 'ap:manage'],
  updateAgentProxyAPIKey: [
    'ap:api:agent:api_key:manage',
    'ap:api:agent:api_key:update',
    'ap:api:agent:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  UpdateAPIDocument: ['ap:content:document:manage', 'ap:content:manage', 'ap:manage'],
  UpdateAPIKey: [
    'ap:api:manage',
    'ap:api:rest:api_key:manage',
    'ap:api:rest:api_key:update',
    'ap:api:rest:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  UpdateApiPortal: ['ap:manage', 'ap:portal:manage', 'ap:portal:update'],
  UpdateApplication: ['ap:application:manage', 'ap:application:update', 'ap:manage'],
  UpdateGateway: ['ap:gateway:manage', 'ap:gateway:update', 'ap:manage'],
  UpdateGraphQLAPI: [
    'ap:api:graphql:manage',
    'ap:api:graphql:update',
    'ap:api:manage',
    'ap:manage',
  ],
  UpdateGraphQLAPIKey: [
    'ap:api:graphql:api_key:manage',
    'ap:api:graphql:api_key:update',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:credential:api_key:all:manage',
    'ap:manage',
  ],
  updateLLMProvider: ['ap:llm_provider:manage', 'ap:llm_provider:update', 'ap:manage'],
  updateLLMProviderTemplate: [
    'ap:llm_provider:manage',
    'ap:llm_provider:template:manage',
    'ap:llm_provider:template:update',
    'ap:manage',
  ],
  updateLLMProxy: ['ap:api:llm:manage', 'ap:api:llm:update', 'ap:api:manage', 'ap:manage'],
  updateMCPProxy: ['ap:api:manage', 'ap:api:mcp:manage', 'ap:api:mcp:update', 'ap:manage'],
  UpdateMeter: ['ap:manage', 'ap:meter:manage', 'ap:meter:update'],
  UpdateProject: ['ap:manage', 'ap:project:manage', 'ap:project:update'],
  UpdateRESTAPI: ['ap:api:manage', 'ap:api:rest:manage', 'ap:api:rest:update', 'ap:manage'],
  UpdateRESTAPISpec: ['ap:api:manage', 'ap:api:rest:manage', 'ap:api:rest:update', 'ap:manage'],
  UpdateSubscription: ['ap:manage', 'ap:subscription:manage', 'ap:subscription:update'],
  UpdateSubscriptionPlan: [
    'ap:manage',
    'ap:subscription_plan:manage',
    'ap:subscription_plan:update',
  ],
  UpsertAPIThumbnail: ['ap:content:manage', 'ap:content:thumbnail:manage', 'ap:manage'],
  ValidateGraphQLSchema: [
    'ap:api:graphql:create',
    'ap:api:graphql:manage',
    'ap:api:manage',
    'ap:manage',
  ],
  ValidateOpenAPISpec: ['ap:api:manage', 'ap:api:rest:create', 'ap:api:rest:manage', 'ap:manage'],
} as const satisfies Record<keyof operations, readonly ApScope[]>;

/**
 * Pre-permission-tree scope names the Platform API still accepts, and the
 * permissions each one grants. Mirrors legacy-scope-mapping.yaml, which the
 * API applies to every request; the evaluator applies it to a session's scopes
 * so a token or role that still carries an old name is offered what the API
 * will let it do. Keys are not `ApScope`s: the spec no longer declares them.
 */
export const LEGACY_SCOPES: Readonly<Record<string, readonly ApScope[]>> = {
  'ap:agent_proxy:api_key:create': ['ap:api:agent:api_key:create'],
  'ap:agent_proxy:api_key:delete': ['ap:api:agent:api_key:delete'],
  'ap:agent_proxy:api_key:manage': ['ap:api:agent:api_key:manage'],
  'ap:agent_proxy:api_key:read': ['ap:api:agent:api_key:read'],
  'ap:agent_proxy:api_key:update': ['ap:api:agent:api_key:update'],
  'ap:agent_proxy:build:create': ['ap:api:agent:build:create'],
  'ap:agent_proxy:build:delete': ['ap:api:agent:build:delete'],
  'ap:agent_proxy:build:manage': ['ap:api:agent:build:manage'],
  'ap:agent_proxy:build:read': ['ap:api:agent:build:read'],
  'ap:agent_proxy:create': ['ap:api:agent:create'],
  'ap:agent_proxy:delete': ['ap:api:agent:delete'],
  'ap:agent_proxy:deployment:create': ['ap:api:agent:deployment:create'],
  'ap:agent_proxy:deployment:delete': ['ap:api:agent:deployment:delete'],
  'ap:agent_proxy:deployment:manage': ['ap:api:agent:deployment:manage'],
  'ap:agent_proxy:deployment:read': ['ap:api:agent:deployment:read'],
  'ap:agent_proxy:deployment:restore': ['ap:api:agent:deployment:restore'],
  'ap:agent_proxy:deployment:undeploy': ['ap:api:agent:deployment:undeploy'],
  'ap:agent_proxy:manage': ['ap:api:agent:manage'],
  'ap:agent_proxy:read': ['ap:api:agent:read'],
  'ap:agent_proxy:update': ['ap:api:agent:update'],
  'ap:api_key:all:manage': ['ap:credential:api_key:all:manage'],
  'ap:api_key:read': ['ap:credential:api_key:read'],
  'ap:api_portal:create': ['ap:portal:create'],
  'ap:api_portal:delete': ['ap:portal:delete'],
  'ap:api_portal:draft:manage': ['ap:portal:draft:manage'],
  'ap:api_portal:draft:read': ['ap:portal:draft:read'],
  'ap:api_portal:draft:update': ['ap:portal:draft:update'],
  'ap:api_portal:graphql_api:deprecate': ['ap:portal:graphql_api:deprecate'],
  'ap:api_portal:graphql_api:manage': ['ap:portal:graphql_api:manage'],
  'ap:api_portal:graphql_api:publish': ['ap:portal:graphql_api:publish'],
  'ap:api_portal:graphql_api:unpublish': ['ap:portal:graphql_api:unpublish'],
  'ap:api_portal:manage': [
    'ap:portal:create',
    'ap:portal:delete',
    'ap:portal:read',
    'ap:portal:update',
  ],
  'ap:api_portal:publication:read': ['ap:portal:publication:read'],
  'ap:api_portal:read': ['ap:portal:read'],
  'ap:api_portal:rest_api:deprecate': ['ap:portal:rest_api:deprecate'],
  'ap:api_portal:rest_api:manage': ['ap:portal:rest_api:manage'],
  'ap:api_portal:rest_api:publish': ['ap:portal:rest_api:publish'],
  'ap:api_portal:rest_api:unpublish': ['ap:portal:rest_api:unpublish'],
  'ap:api_portal:update': ['ap:portal:update'],
  'ap:api_publication:read': ['ap:publication:read'],
  'ap:docs:manage': ['ap:content:document:manage'],
  'ap:docs:read': ['ap:content:document:read'],
  'ap:gateway_custom_policy:create': ['ap:gateway_policy:create'],
  'ap:gateway_custom_policy:delete': ['ap:gateway_policy:delete'],
  'ap:gateway_custom_policy:manage': ['ap:gateway_policy:manage'],
  'ap:gateway_custom_policy:read': ['ap:gateway_policy:read'],
  'ap:graphql_api:api_key:create': ['ap:api:graphql:api_key:create'],
  'ap:graphql_api:api_key:delete': ['ap:api:graphql:api_key:delete'],
  'ap:graphql_api:api_key:manage': ['ap:api:graphql:api_key:manage'],
  'ap:graphql_api:api_key:update': ['ap:api:graphql:api_key:update'],
  'ap:graphql_api:build:create': ['ap:api:graphql:build:create'],
  'ap:graphql_api:build:delete': ['ap:api:graphql:build:delete'],
  'ap:graphql_api:build:manage': ['ap:api:graphql:build:manage'],
  'ap:graphql_api:build:read': ['ap:api:graphql:build:read'],
  'ap:graphql_api:create': ['ap:api:graphql:create'],
  'ap:graphql_api:delete': ['ap:api:graphql:delete'],
  'ap:graphql_api:deployment:create': ['ap:api:graphql:deployment:create'],
  'ap:graphql_api:deployment:delete': ['ap:api:graphql:deployment:delete'],
  'ap:graphql_api:deployment:manage': ['ap:api:graphql:deployment:manage'],
  'ap:graphql_api:deployment:read': ['ap:api:graphql:deployment:read'],
  'ap:graphql_api:deployment:restore': ['ap:api:graphql:deployment:restore'],
  'ap:graphql_api:deployment:undeploy': ['ap:api:graphql:deployment:undeploy'],
  'ap:graphql_api:gateway:create': ['ap:api:graphql:gateway:create'],
  'ap:graphql_api:gateway:manage': ['ap:api:graphql:gateway:manage'],
  'ap:graphql_api:gateway:read': ['ap:api:graphql:gateway:read'],
  'ap:graphql_api:manage': ['ap:api:graphql:manage'],
  'ap:graphql_api:read': ['ap:api:graphql:read'],
  'ap:graphql_api:update': ['ap:api:graphql:update'],
  'ap:llm_proxy:api_key:create': ['ap:api:llm:api_key:create'],
  'ap:llm_proxy:api_key:delete': ['ap:api:llm:api_key:delete'],
  'ap:llm_proxy:api_key:manage': ['ap:api:llm:api_key:manage'],
  'ap:llm_proxy:api_key:read': ['ap:api:llm:api_key:read'],
  'ap:llm_proxy:build:create': ['ap:api:llm:build:create'],
  'ap:llm_proxy:build:delete': ['ap:api:llm:build:delete'],
  'ap:llm_proxy:build:manage': ['ap:api:llm:build:manage'],
  'ap:llm_proxy:build:read': ['ap:api:llm:build:read'],
  'ap:llm_proxy:create': ['ap:api:llm:create'],
  'ap:llm_proxy:delete': ['ap:api:llm:delete'],
  'ap:llm_proxy:deployment:create': ['ap:api:llm:deployment:create'],
  'ap:llm_proxy:deployment:delete': ['ap:api:llm:deployment:delete'],
  'ap:llm_proxy:deployment:manage': ['ap:api:llm:deployment:manage'],
  'ap:llm_proxy:deployment:read': ['ap:api:llm:deployment:read'],
  'ap:llm_proxy:deployment:restore': ['ap:api:llm:deployment:restore'],
  'ap:llm_proxy:deployment:undeploy': ['ap:api:llm:deployment:undeploy'],
  'ap:llm_proxy:manage': ['ap:api:llm:manage'],
  'ap:llm_proxy:read': ['ap:api:llm:read'],
  'ap:llm_proxy:update': ['ap:api:llm:update'],
  'ap:llm_template:create': ['ap:llm_provider:template:create'],
  'ap:llm_template:delete': ['ap:llm_provider:template:delete'],
  'ap:llm_template:manage': ['ap:llm_provider:template:manage'],
  'ap:llm_template:read': ['ap:llm_provider:template:read'],
  'ap:llm_template:update': ['ap:llm_provider:template:update'],
  'ap:mcp_proxy:build:create': ['ap:api:mcp:build:create'],
  'ap:mcp_proxy:build:delete': ['ap:api:mcp:build:delete'],
  'ap:mcp_proxy:build:manage': ['ap:api:mcp:build:manage'],
  'ap:mcp_proxy:build:read': ['ap:api:mcp:build:read'],
  'ap:mcp_proxy:create': ['ap:api:mcp:create'],
  'ap:mcp_proxy:delete': ['ap:api:mcp:delete'],
  'ap:mcp_proxy:deployment:create': ['ap:api:mcp:deployment:create'],
  'ap:mcp_proxy:deployment:delete': ['ap:api:mcp:deployment:delete'],
  'ap:mcp_proxy:deployment:manage': ['ap:api:mcp:deployment:manage'],
  'ap:mcp_proxy:deployment:read': ['ap:api:mcp:deployment:read'],
  'ap:mcp_proxy:deployment:restore': ['ap:api:mcp:deployment:restore'],
  'ap:mcp_proxy:deployment:undeploy': ['ap:api:mcp:deployment:undeploy'],
  'ap:mcp_proxy:manage': ['ap:api:mcp:manage'],
  'ap:mcp_proxy:read': ['ap:api:mcp:read'],
  'ap:mcp_proxy:update': ['ap:api:mcp:update'],
  'ap:rest_api:api_key:create': ['ap:api:rest:api_key:create'],
  'ap:rest_api:api_key:delete': ['ap:api:rest:api_key:delete'],
  'ap:rest_api:api_key:manage': ['ap:api:rest:api_key:manage'],
  'ap:rest_api:api_key:update': ['ap:api:rest:api_key:update'],
  'ap:rest_api:build:create': ['ap:api:rest:build:create'],
  'ap:rest_api:build:delete': ['ap:api:rest:build:delete'],
  'ap:rest_api:build:manage': ['ap:api:rest:build:manage'],
  'ap:rest_api:build:read': ['ap:api:rest:build:read'],
  'ap:rest_api:create': ['ap:api:rest:create'],
  'ap:rest_api:delete': ['ap:api:rest:delete'],
  'ap:rest_api:deployment:create': ['ap:api:rest:deployment:create'],
  'ap:rest_api:deployment:delete': ['ap:api:rest:deployment:delete'],
  'ap:rest_api:deployment:manage': ['ap:api:rest:deployment:manage'],
  'ap:rest_api:deployment:read': ['ap:api:rest:deployment:read'],
  'ap:rest_api:deployment:restore': ['ap:api:rest:deployment:restore'],
  'ap:rest_api:deployment:undeploy': ['ap:api:rest:deployment:undeploy'],
  'ap:rest_api:gateway:create': ['ap:api:rest:gateway:create'],
  'ap:rest_api:gateway:manage': ['ap:api:rest:gateway:manage'],
  'ap:rest_api:gateway:read': ['ap:api:rest:gateway:read'],
  'ap:rest_api:manage': ['ap:api:rest:manage'],
  'ap:rest_api:read': ['ap:api:rest:read'],
  'ap:rest_api:update': ['ap:api:rest:update'],
  'ap:secret:create': ['ap:credential:secret:create'],
  'ap:secret:delete': ['ap:credential:secret:delete'],
  'ap:secret:manage': ['ap:credential:secret:manage'],
  'ap:secret:read': ['ap:credential:secret:read'],
  'ap:secret:update': ['ap:credential:secret:update'],
  'ap:thumbnail:manage': ['ap:content:thumbnail:manage'],
  'ap:thumbnail:read': ['ap:content:thumbnail:read'],
};
