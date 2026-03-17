/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"encoding/json"
	"time"
)

// GatewayEvent represents a notification to be sent to a gateway instance.
// Events are sent over the WebSocket connection to notify gateways of
// platform-side changes that require gateway action (e.g., API deployment).
type GatewayEvent struct {
	// Type identifies the event category (e.g., "api.deployed", "api.undeployed", "api.deleted")
	Type string `json:"type"`

	// Payload contains event-specific data as raw JSON.
	// The structure depends on the event type:
	//   - "api.deployed": DeploymentEvent
	//   - "api.undeployed": APIUndeploymentEvent
	//   - "api.deleted": APIDeletionEvent
	//   - "gateway.config.updated": GatewayConfigEvent
	Payload json.RawMessage `json:"payload"`

	// GatewayID identifies the target gateway for this event (UUID)
	GatewayID string `json:"gatewayId"`

	// Timestamp records when the event was created on the platform
	Timestamp time.Time `json:"timestamp"`

	// CorrelationID provides a unique identifier for request tracing across systems
	CorrelationID string `json:"correlationId"`
}

// DeploymentEvent contains payload data for "api.deployed" event type.
// This event is sent when an API is successfully deployed to a gateway.
type DeploymentEvent struct {
	// ApiId identifies the deployed API
	ApiId string `json:"apiId"`

	// DeploymentID identifies the specific API deployment
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host where the API is deployed
	Vhost string `json:"vhost"`

	// PerformedAt is the timestamp when the deployment was initiated (concurrency token)
	PerformedAt time.Time `json:"performedAt"`
}

// APIUndeploymentEvent contains payload data for "api.undeployed" event type.
// This event is sent when an API is undeployed from a gateway.
type APIUndeploymentEvent struct {
	// ApiId identifies the undeployed API
	ApiId string `json:"apiId"`

	// DeploymentID identifies the specific deployment being undeployed
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host from which the API is undeployed
	Vhost string `json:"vhost"`

	// PerformedAt is the timestamp when the undeployment was initiated (concurrency token)
	PerformedAt time.Time `json:"performedAt"`
}

// APIDeletionEvent contains payload data for "api.deleted" event type.
// This event is sent when an API is permanently deleted from the platform.
type APIDeletionEvent struct {
	// ApiId identifies the deleted API
	ApiId string `json:"apiId"`

	// Vhost specifies the virtual host from which the API should be removed
	Vhost string `json:"vhost"`
}

// LLMProviderDeploymentEvent contains payload data for "llmprovider.deployed" event type.
// This event is sent when an LLM provider is successfully deployed to a gateway.
type LLMProviderDeploymentEvent struct {
	// ProviderId identifies the deployed LLM provider (handle)
	ProviderId string `json:"providerId"`

	// DeploymentID identifies the specific deployment artifact
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host where the provider is deployed
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the deployment was initiated (concurrency token)
	PerformedAt time.Time `json:"performedAt"`
}

// LLMProviderUndeploymentEvent contains payload data for "llmprovider.undeployed" event type.
// This event is sent when an LLM provider is undeployed from a gateway.
type LLMProviderUndeploymentEvent struct {
	// ProviderId identifies the undeployed LLM provider (handle)
	ProviderId string `json:"providerId"`

	// DeploymentID identifies the specific deployment being undeployed
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host from which the provider is undeployed
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the undeployment was initiated (concurrency token)
	PerformedAt time.Time `json:"performedAt"`
}

// LLMProxyDeploymentEvent contains payload data for "llmproxy.deployed" event type.
// This event is sent when an LLM proxy is successfully deployed to a gateway.
type LLMProxyDeploymentEvent struct {
	// ProxyId identifies the deployed LLM proxy (handle)
	ProxyId string `json:"proxyId"`

	// DeploymentID identifies the specific deployment artifact
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host where the proxy is deployed
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the deployment was initiated (concurrency token)
	PerformedAt time.Time `json:"performedAt"`
}

// LLMProxyUndeploymentEvent contains payload data for "llmproxy.undeployed" event type.
// This event is sent when an LLM proxy is undeployed from a gateway.
type LLMProxyUndeploymentEvent struct {
	// ProxyId identifies the undeployed LLM proxy (handle)
	ProxyId string `json:"proxyId"`

	// DeploymentID identifies the specific deployment being undeployed
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host from which the proxy is undeployed
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the undeployment was initiated (concurrency token)
	PerformedAt time.Time `json:"performedAt"`
}

// GatewayConfigEvent contains payload data for "gateway.config.updated" event type.
// This event is sent when gateway configuration needs to be refreshed.
type GatewayConfigEvent struct {
	// ConfigType identifies the configuration category (e.g., "rate-limit", "cors")
	ConfigType string `json:"configType"`

	// Action specifies the configuration change action ("update", "delete", "refresh")
	Action string `json:"action"`
}

// MCPProxyDeploymentEvent contains payload data for "mcpproxy.deployed" event type.
// This event is sent when an MCP proxy is successfully deployed to a gateway.
type MCPProxyDeploymentEvent struct {
	// ProxyId identifies the deployed MCP proxy (handle)
	ProxyId string `json:"proxyId"`

	// DeploymentID identifies the specific deployment artifact
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host where the proxy is deployed
	Vhost string `json:"vhost"`
}

// MCPProxyUndeploymentEvent contains payload data for "mcpproxy.undeployed" event type.
// This event is sent when an MCP proxy is undeployed from a gateway.
type MCPProxyUndeploymentEvent struct {
	// ProxyId identifies the undeployed MCP proxy (handle)
	ProxyId string `json:"proxyId"`

	// Vhost specifies the virtual host from which the proxy is undeployed
	Vhost string `json:"vhost"`
}

// MCPProxyDeletionEvent contains payload data for "mcpproxy.deleted" event type.
// This event is sent when an MCP proxy is permanently deleted from the platform.
type MCPProxyDeletionEvent struct {
	// ProxyId identifies the deleted MCP proxy
	ProxyId string `json:"proxyId"`

	// Vhost specifies the virtual host from which the proxy should be removed
	Vhost string `json:"vhost"`
}
