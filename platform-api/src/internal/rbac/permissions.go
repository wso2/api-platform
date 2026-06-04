/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package rbac

// Permission is a typed string identifying a single platform capability.
type Permission string

// Scope returns the OAuth2 scope string as Thunder issues it in the token (e.g. "ap:gateway:create").
// Thunder prefixes scopes with the resource-server identifier when issuing tokens.
func (p Permission) Scope() string {
	return "ap:" + string(p)
}

const (
	// Gateway
	GatewayManage       Permission = "gateway:manage"
	GatewayCreate       Permission = "gateway:create"
	GatewayRead         Permission = "gateway:read"
	GatewayUpdate       Permission = "gateway:update"
	GatewayDelete       Permission = "gateway:delete"
	GatewayTokenManage  Permission = "gateway:token:manage"
	GatewayTokenRead    Permission = "gateway:token:read"
	GatewayTokenCreate  Permission = "gateway:token:create"
	GatewayTokenDelete  Permission = "gateway:token:delete"
	GatewayPolicyManage Permission = "gateway:policy:manage"
	GatewayPolicyRead   Permission = "gateway:policy:read"
	GatewayPolicyCreate Permission = "gateway:policy:create"
	GatewayPolicyDelete Permission = "gateway:policy:delete"
	GatewayArtifactsRead Permission = "gateway:artifacts:read"
	GatewayManifestRead  Permission = "gateway:manifest:read"
	GatewayStatusRead    Permission = "gateway:status:read"

	// REST API
	APIManage  Permission = "rest_api:manage"
	APICreate  Permission = "rest_api:create"
	APIRead    Permission = "rest_api:read"
	APIUpdate  Permission = "rest_api:update"
	APIDelete  Permission = "rest_api:delete"
	APIPublish Permission = "rest_api:publish"
	APIImport  Permission = "rest_api:import"

	// REST API → Gateways (sub-resource scopes nested under rest_api)
	APIGatewayManage Permission = "rest_api:gateway:manage"
	APIGatewayCreate Permission = "rest_api:gateway:create"
	APIGatewayRead   Permission = "rest_api:gateway:read"

	// REST API → Deployments (sub-resource scopes nested under rest_api)
	APIDeploymentManage   Permission = "rest_api:deployment:manage"
	APIDeploymentCreate   Permission = "rest_api:deployment:create"
	APIDeploymentRead     Permission = "rest_api:deployment:read"
	APIDeploymentDelete   Permission = "rest_api:deployment:delete"
	APIDeploymentUndeploy Permission = "rest_api:deployment:undeploy"
	APIDeploymentRestore  Permission = "rest_api:deployment:restore"

	// Project
	ProjectManage Permission = "project:manage"
	ProjectCreate Permission = "project:create"
	ProjectRead   Permission = "project:read"
	ProjectUpdate Permission = "project:update"
	ProjectDelete Permission = "project:delete"

	// Application
	ApplicationManage    Permission = "application:manage"
	ApplicationCreate    Permission = "application:create"
	ApplicationRead      Permission = "application:read"
	ApplicationUpdate    Permission = "application:update"
	ApplicationDelete    Permission = "application:delete"

	// Application → API Keys (sub-resource)
	ApplicationAPIKeyManage Permission = "application:api_key:manage"
	ApplicationAPIKeyCreate Permission = "application:api_key:create"
	ApplicationAPIKeyRead   Permission = "application:api_key:read"
	ApplicationAPIKeyDelete Permission = "application:api_key:delete"

	// Application → Associations (sub-resource)
	ApplicationAssociationsManage    Permission = "application:associations:manage"
	ApplicationAssociationsCreate    Permission = "application:associations:create"
	ApplicationAssociationsRead      Permission = "application:associations:read"
	ApplicationAssociationsDelete    Permission = "application:associations:delete"
	ApplicationAssociationsAPIKeyRead Permission = "application:associations:api_key:read"

	// DevPortal — DevPortalManage is the root scope covering CRUD and lifecycle operations
	// (activate/deactivate/set-default).
	DevPortalManage Permission = "devportal:manage"
	DevPortalCreate Permission = "devportal:create"
	DevPortalRead   Permission = "devportal:read"
	DevPortalUpdate Permission = "devportal:update"
	DevPortalDelete Permission = "devportal:delete"

	// Subscription
	SubscriptionManage Permission = "subscription:manage"
	SubscriptionCreate Permission = "subscription:create"
	SubscriptionRead   Permission = "subscription:read"
	SubscriptionUpdate Permission = "subscription:update"
	SubscriptionDelete Permission = "subscription:delete"

	// SubscriptionPlan
	SubscriptionPlanManage Permission = "subscription_plan:manage"
	SubscriptionPlanCreate Permission = "subscription_plan:create"
	SubscriptionPlanRead   Permission = "subscription_plan:read"
	SubscriptionPlanUpdate Permission = "subscription_plan:update"
	SubscriptionPlanDelete Permission = "subscription_plan:delete"

	// REST API → API Keys (sub-resource scopes nested under rest_api)
	APIKeyManage Permission = "rest_api:api_key:manage"
	APIKeyCreate Permission = "rest_api:api_key:create"
	APIKeyRead   Permission = "rest_api:api_key:read"
	APIKeyUpdate Permission = "rest_api:api_key:update"
	APIKeyDelete Permission = "rest_api:api_key:delete"

	// LLM provider template
	LLMTemplateManage Permission = "llm_template:manage"
	LLMTemplateCreate Permission = "llm_template:create"
	LLMTemplateRead   Permission = "llm_template:read"
	LLMTemplateUpdate Permission = "llm_template:update"
	LLMTemplateDelete Permission = "llm_template:delete"

	// LLM provider
	LLMProviderManage             Permission = "llm_provider:manage"
	LLMProviderCreate             Permission = "llm_provider:create"
	LLMProviderRead               Permission = "llm_provider:read"
	LLMProviderUpdate             Permission = "llm_provider:update"
	LLMProviderDelete             Permission = "llm_provider:delete"
	LLMProviderDeploymentManage   Permission = "llm_provider:deployment:manage"
	LLMProviderDeploymentCreate   Permission = "llm_provider:deployment:create"
	LLMProviderDeploymentRead     Permission = "llm_provider:deployment:read"
	LLMProviderDeploymentDelete   Permission = "llm_provider:deployment:delete"
	LLMProviderDeploymentUndeploy Permission = "llm_provider:deployment:undeploy"
	LLMProviderDeploymentRestore  Permission = "llm_provider:deployment:restore"
	LLMProviderAPIKeyManage       Permission = "llm_provider:api_key:manage"
	LLMProviderAPIKeyCreate       Permission = "llm_provider:api_key:create"
	LLMProviderAPIKeyRead         Permission = "llm_provider:api_key:read"
	LLMProviderAPIKeyDelete       Permission = "llm_provider:api_key:delete"

	// LLM proxy
	LLMProxyManage             Permission = "llm_proxy:manage"
	LLMProxyCreate             Permission = "llm_proxy:create"
	LLMProxyRead               Permission = "llm_proxy:read"
	LLMProxyUpdate             Permission = "llm_proxy:update"
	LLMProxyDelete             Permission = "llm_proxy:delete"
	LLMProxyDeploymentManage   Permission = "llm_proxy:deployment:manage"
	LLMProxyDeploymentCreate   Permission = "llm_proxy:deployment:create"
	LLMProxyDeploymentRead     Permission = "llm_proxy:deployment:read"
	LLMProxyDeploymentDelete   Permission = "llm_proxy:deployment:delete"
	LLMProxyDeploymentUndeploy Permission = "llm_proxy:deployment:undeploy"
	LLMProxyDeploymentRestore  Permission = "llm_proxy:deployment:restore"
	LLMProxyAPIKeyManage       Permission = "llm_proxy:api_key:manage"
	LLMProxyAPIKeyCreate       Permission = "llm_proxy:api_key:create"
	LLMProxyAPIKeyRead         Permission = "llm_proxy:api_key:read"
	LLMProxyAPIKeyDelete       Permission = "llm_proxy:api_key:delete"

	// MCP proxy
	MCPProxyManage             Permission = "mcp_proxy:manage"
	MCPProxyCreate             Permission = "mcp_proxy:create"
	MCPProxyRead               Permission = "mcp_proxy:read"
	MCPProxyUpdate             Permission = "mcp_proxy:update"
	MCPProxyDelete             Permission = "mcp_proxy:delete"
	MCPProxyDeploymentManage   Permission = "mcp_proxy:deployment:manage"
	MCPProxyDeploymentCreate   Permission = "mcp_proxy:deployment:create"
	MCPProxyDeploymentRead     Permission = "mcp_proxy:deployment:read"
	MCPProxyDeploymentDelete   Permission = "mcp_proxy:deployment:delete"
	MCPProxyDeploymentUndeploy Permission = "mcp_proxy:deployment:undeploy"
	MCPProxyDeploymentRestore  Permission = "mcp_proxy:deployment:restore"

	// WebSub API
	WebSubAPIManage             Permission = "websub_api:manage"
	WebSubAPICreate             Permission = "websub_api:create"
	WebSubAPIRead               Permission = "websub_api:read"
	WebSubAPIUpdate             Permission = "websub_api:update"
	WebSubAPIDelete             Permission = "websub_api:delete"
	WebSubAPIDeploymentManage   Permission = "websub_api:deployment:manage"
	WebSubAPIDeploymentCreate   Permission = "websub_api:deployment:create"
	WebSubAPIDeploymentRead     Permission = "websub_api:deployment:read"
	WebSubAPIDeploymentDelete   Permission = "websub_api:deployment:delete"
	WebSubAPIDeploymentUndeploy Permission = "websub_api:deployment:undeploy"
	WebSubAPIDeploymentRestore  Permission = "websub_api:deployment:restore"
	WebSubAPIPublish            Permission = "websub_api:publish"
	WebSubAPIKeyManage          Permission = "websub_api:api_key:manage"
	WebSubAPIKeyCreate          Permission = "websub_api:api_key:create"
	WebSubAPIKeyUpdate          Permission = "websub_api:api_key:update"
	WebSubAPIKeyDelete          Permission = "websub_api:api_key:delete"

	// WebBroker API
	WebBrokerAPIManage             Permission = "webbroker_api:manage"
	WebBrokerAPICreate             Permission = "webbroker_api:create"
	WebBrokerAPIRead               Permission = "webbroker_api:read"
	WebBrokerAPIUpdate             Permission = "webbroker_api:update"
	WebBrokerAPIDelete             Permission = "webbroker_api:delete"
	WebBrokerAPIDeploymentManage   Permission = "webbroker_api:deployment:manage"
	WebBrokerAPIDeploymentCreate   Permission = "webbroker_api:deployment:create"
	WebBrokerAPIDeploymentRead     Permission = "webbroker_api:deployment:read"
	WebBrokerAPIDeploymentDelete   Permission = "webbroker_api:deployment:delete"
	WebBrokerAPIDeploymentUndeploy Permission = "webbroker_api:deployment:undeploy"
	WebBrokerAPIDeploymentRestore  Permission = "webbroker_api:deployment:restore"
	WebBrokerAPIPublish            Permission = "webbroker_api:publish"
	WebBrokerAPIKeyManage          Permission = "webbroker_api:api_key:manage"
	WebBrokerAPIKeyCreate          Permission = "webbroker_api:api_key:create"
	WebBrokerAPIKeyUpdate          Permission = "webbroker_api:api_key:update"
	WebBrokerAPIKeyDelete          Permission = "webbroker_api:api_key:delete"

	// Git
	GitRead Permission = "git:read"
)

// ManagedBy maps fine-grained write permissions to the resource-level manage permission
// that acts as a root scope covering all of them. A token carrying resource:manage is
// authorised for every permission listed here that maps to that manage scope.
// Read permissions and resource:manage itself are not included — they are checked directly.
var ManagedBy = map[Permission]Permission{
	// Gateway
	GatewayCreate:       GatewayManage,
	GatewayUpdate:       GatewayManage,
	GatewayDelete:       GatewayManage,
	GatewayTokenManage:  GatewayManage,
	GatewayTokenRead:    GatewayTokenManage,
	GatewayTokenCreate:  GatewayTokenManage,
	GatewayTokenDelete:  GatewayTokenManage,
	GatewayPolicyManage: GatewayManage,
	GatewayPolicyRead:   GatewayPolicyManage,
	GatewayPolicyCreate: GatewayPolicyManage,
	GatewayPolicyDelete: GatewayPolicyManage,
	GatewayArtifactsRead: GatewayManage,
	GatewayManifestRead:  GatewayManage,
	GatewayStatusRead:    GatewayManage,

	// API
	APICreate:  APIManage,
	APIUpdate:  APIManage,
	APIDelete:  APIManage,
	APIPublish: APIManage,
	APIImport:  APIManage,

	// API Gateways
	APIGatewayManage: APIManage,
	APIGatewayCreate: APIGatewayManage,

	// API Deployments
	APIDeploymentManage:   APIManage,
	APIDeploymentCreate:   APIDeploymentManage,
	APIDeploymentDelete:   APIDeploymentManage,
	APIDeploymentUndeploy: APIDeploymentManage,
	APIDeploymentRestore:  APIDeploymentManage,

	// Project
	ProjectCreate: ProjectManage,
	ProjectUpdate: ProjectManage,
	ProjectDelete: ProjectManage,

	// Application
	ApplicationCreate:    ApplicationManage,
	ApplicationUpdate:    ApplicationManage,
	ApplicationDelete:    ApplicationManage,
	ApplicationAPIKeyManage: ApplicationManage,
	ApplicationAPIKeyCreate: ApplicationAPIKeyManage,
	ApplicationAPIKeyDelete: ApplicationAPIKeyManage,
	ApplicationAssociationsManage:    ApplicationManage,
	ApplicationAssociationsCreate:    ApplicationAssociationsManage,
	ApplicationAssociationsDelete:    ApplicationAssociationsManage,

	// DevPortal
	DevPortalCreate: DevPortalManage,
	DevPortalUpdate: DevPortalManage,
	DevPortalDelete: DevPortalManage,

	// Subscription
	SubscriptionCreate: SubscriptionManage,
	SubscriptionUpdate: SubscriptionManage,
	SubscriptionDelete: SubscriptionManage,

	// SubscriptionPlan
	SubscriptionPlanCreate: SubscriptionPlanManage,
	SubscriptionPlanUpdate: SubscriptionPlanManage,
	SubscriptionPlanDelete: SubscriptionPlanManage,

	// APIKey
	APIKeyCreate: APIKeyManage,
	APIKeyUpdate: APIKeyManage,
	APIKeyDelete: APIKeyManage,

	// LLM provider template
	LLMTemplateCreate: LLMTemplateManage,
	LLMTemplateUpdate: LLMTemplateManage,
	LLMTemplateDelete: LLMTemplateManage,

	// LLM provider
	LLMProviderCreate:             LLMProviderManage,
	LLMProviderUpdate:             LLMProviderManage,
	LLMProviderDelete:             LLMProviderManage,
	LLMProviderDeploymentManage:   LLMProviderManage,
	LLMProviderDeploymentCreate:   LLMProviderDeploymentManage,
	LLMProviderDeploymentDelete:   LLMProviderDeploymentManage,
	LLMProviderDeploymentUndeploy: LLMProviderDeploymentManage,
	LLMProviderDeploymentRestore:  LLMProviderDeploymentManage,
	LLMProviderAPIKeyCreate:       LLMProviderAPIKeyManage,
	LLMProviderAPIKeyDelete:       LLMProviderAPIKeyManage,

	// LLM proxy
	LLMProxyCreate:             LLMProxyManage,
	LLMProxyUpdate:             LLMProxyManage,
	LLMProxyDelete:             LLMProxyManage,
	LLMProxyDeploymentManage:   LLMProxyManage,
	LLMProxyDeploymentCreate:   LLMProxyDeploymentManage,
	LLMProxyDeploymentDelete:   LLMProxyDeploymentManage,
	LLMProxyDeploymentUndeploy: LLMProxyDeploymentManage,
	LLMProxyDeploymentRestore:  LLMProxyDeploymentManage,
	LLMProxyAPIKeyCreate:       LLMProxyAPIKeyManage,
	LLMProxyAPIKeyDelete:       LLMProxyAPIKeyManage,

	// MCP proxy
	MCPProxyCreate:             MCPProxyManage,
	MCPProxyUpdate:             MCPProxyManage,
	MCPProxyDelete:             MCPProxyManage,
	MCPProxyDeploymentManage:   MCPProxyManage,
	MCPProxyDeploymentCreate:   MCPProxyDeploymentManage,
	MCPProxyDeploymentDelete:   MCPProxyDeploymentManage,
	MCPProxyDeploymentUndeploy: MCPProxyDeploymentManage,
	MCPProxyDeploymentRestore:  MCPProxyDeploymentManage,

	// WebSub API
	WebSubAPICreate:             WebSubAPIManage,
	WebSubAPIUpdate:             WebSubAPIManage,
	WebSubAPIDelete:             WebSubAPIManage,
	WebSubAPIDeploymentManage:   WebSubAPIManage,
	WebSubAPIDeploymentCreate:   WebSubAPIDeploymentManage,
	WebSubAPIDeploymentDelete:   WebSubAPIDeploymentManage,
	WebSubAPIDeploymentUndeploy: WebSubAPIDeploymentManage,
	WebSubAPIDeploymentRestore:  WebSubAPIDeploymentManage,
	WebSubAPIPublish:            WebSubAPIManage,
	WebSubAPIKeyCreate:          WebSubAPIKeyManage,
	WebSubAPIKeyUpdate:          WebSubAPIKeyManage,
	WebSubAPIKeyDelete:          WebSubAPIKeyManage,

	// WebBroker API
	WebBrokerAPICreate:             WebBrokerAPIManage,
	WebBrokerAPIUpdate:             WebBrokerAPIManage,
	WebBrokerAPIDelete:             WebBrokerAPIManage,
	WebBrokerAPIDeploymentManage:   WebBrokerAPIManage,
	WebBrokerAPIDeploymentCreate:   WebBrokerAPIDeploymentManage,
	WebBrokerAPIDeploymentDelete:   WebBrokerAPIDeploymentManage,
	WebBrokerAPIDeploymentUndeploy: WebBrokerAPIDeploymentManage,
	WebBrokerAPIDeploymentRestore:  WebBrokerAPIDeploymentManage,
	WebBrokerAPIPublish:            WebBrokerAPIManage,
	WebBrokerAPIKeyCreate:          WebBrokerAPIKeyManage,
	WebBrokerAPIKeyUpdate:          WebBrokerAPIKeyManage,
	WebBrokerAPIKeyDelete:          WebBrokerAPIKeyManage,
}
