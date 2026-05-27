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

// Scope returns the OAuth2 scope string as Thunder issues it in the token (e.g. "api-platform:gateway:create").
// Thunder prefixes scopes with the resource-server identifier when issuing tokens.
func (p Permission) Scope() string {
	return "api-platform:" + string(p)
}

const (
	// Gateway
	GatewayCreate       Permission = "gateway:create"
	GatewayRead         Permission = "gateway:read"
	GatewayUpdate       Permission = "gateway:update"
	GatewayDelete       Permission = "gateway:delete"
	GatewayTokenManage  Permission = "gateway:token:manage"
	GatewayPolicyManage Permission = "gateway:policy:manage"

	// API
	APICreate  Permission = "api:create"
	APIRead    Permission = "api:read"
	APIUpdate  Permission = "api:update"
	APIDelete  Permission = "api:delete"
	APIDeploy  Permission = "api:deploy"
	APIPublish Permission = "api:publish"
	APIImport  Permission = "api:import"

	// Project
	ProjectCreate Permission = "project:create"
	ProjectRead   Permission = "project:read"
	ProjectUpdate Permission = "project:update"
	ProjectDelete Permission = "project:delete"

	// Application
	ApplicationCreate    Permission = "application:create"
	ApplicationRead      Permission = "application:read"
	ApplicationUpdate    Permission = "application:update"
	ApplicationDelete    Permission = "application:delete"
	ApplicationKeyManage Permission = "application:key:manage"

	// DevPortal
	DevPortalCreate Permission = "devportal:create"
	DevPortalRead   Permission = "devportal:read"
	DevPortalUpdate Permission = "devportal:update"
	DevPortalDelete Permission = "devportal:delete"
	DevPortalManage Permission = "devportal:manage"

	// Subscription
	SubscriptionCreate Permission = "subscription:create"
	SubscriptionRead   Permission = "subscription:read"
	SubscriptionUpdate Permission = "subscription:update"
	SubscriptionDelete Permission = "subscription:delete"

	// SubscriptionPlan
	SubscriptionPlanCreate Permission = "subscription_plan:create"
	SubscriptionPlanRead   Permission = "subscription_plan:read"
	SubscriptionPlanUpdate Permission = "subscription_plan:update"
	SubscriptionPlanDelete Permission = "subscription_plan:delete"

	// APIKey
	APIKeyCreate Permission = "api_key:create"
	APIKeyRead   Permission = "api_key:read"
	APIKeyUpdate Permission = "api_key:update"
	APIKeyDelete Permission = "api_key:delete"

	// LLM provider template
	LLMTemplateCreate Permission = "llm_template:create"
	LLMTemplateRead   Permission = "llm_template:read"
	LLMTemplateUpdate Permission = "llm_template:update"
	LLMTemplateDelete Permission = "llm_template:delete"

	// LLM provider
	LLMProviderCreate    Permission = "llm_provider:create"
	LLMProviderRead      Permission = "llm_provider:read"
	LLMProviderUpdate    Permission = "llm_provider:update"
	LLMProviderDelete    Permission = "llm_provider:delete"
	LLMProviderDeploy    Permission = "llm_provider:deploy"
	LLMProviderKeyManage Permission = "llm_provider:key:manage"

	// LLM proxy
	LLMProxyCreate    Permission = "llm_proxy:create"
	LLMProxyRead      Permission = "llm_proxy:read"
	LLMProxyUpdate    Permission = "llm_proxy:update"
	LLMProxyDelete    Permission = "llm_proxy:delete"
	LLMProxyDeploy    Permission = "llm_proxy:deploy"
	LLMProxyKeyManage Permission = "llm_proxy:key:manage"

	// MCP proxy
	MCPProxyCreate Permission = "mcp_proxy:create"
	MCPProxyRead   Permission = "mcp_proxy:read"
	MCPProxyUpdate Permission = "mcp_proxy:update"
	MCPProxyDelete Permission = "mcp_proxy:delete"
	MCPProxyDeploy Permission = "mcp_proxy:deploy"

	// WebSub API
	WebSubAPICreate    Permission = "websub_api:create"
	WebSubAPIRead      Permission = "websub_api:read"
	WebSubAPIUpdate    Permission = "websub_api:update"
	WebSubAPIDelete    Permission = "websub_api:delete"
	WebSubAPIDeploy    Permission = "websub_api:deploy"
	WebSubAPIPublish   Permission = "websub_api:publish"
	WebSubAPIKeyManage Permission = "websub_api:key:manage"

	// WebBroker API
	WebBrokerAPICreate    Permission = "webbroker_api:create"
	WebBrokerAPIRead      Permission = "webbroker_api:read"
	WebBrokerAPIUpdate    Permission = "webbroker_api:update"
	WebBrokerAPIDelete    Permission = "webbroker_api:delete"
	WebBrokerAPIDeploy    Permission = "webbroker_api:deploy"
	WebBrokerAPIPublish   Permission = "webbroker_api:publish"
	WebBrokerAPIKeyManage Permission = "webbroker_api:key:manage"

	// Git
	GitRead Permission = "git:read"
)
