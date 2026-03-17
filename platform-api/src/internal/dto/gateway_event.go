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

package dto

// GatewayEventDTO represents the wire format for events sent to gateways.
// This DTO separates the internal model from the JSON structure sent over WebSocket.
type GatewayEventDTO struct {
	// Type identifies the event category
	Type string `json:"type"`

	// Payload contains event-specific data (structure varies by type)
	Payload interface{} `json:"payload"`

	// Timestamp records when the event was created (ISO 8601 format)
	Timestamp string `json:"timestamp"`

	// CorrelationID provides request tracing identifier
	CorrelationID string `json:"correlationId"`

	// UserId is an optional temporary user identifier (from x-user-id header)
	UserId string `json:"userId,omitempty"`
}

// ConnectionAckDTO represents the acknowledgment message sent when a gateway connects.
type ConnectionAckDTO struct {
	// Type is always "connection.ack"
	Type string `json:"type"`

	// GatewayID confirms the authenticated gateway identity
	GatewayID string `json:"gatewayId"`

	// ConnectionID provides a unique identifier for this connection instance
	ConnectionID string `json:"connectionId"`

	// Timestamp records when the connection was established
	Timestamp string `json:"timestamp"`
}

// DeploymentEventDTO is the wire format for API deployment notifications.
type DeploymentEventDTO struct {
	// ApiId identifies the deployed API
	ApiId string `json:"apiId"`

	// DeploymentID identifies the specific API deployment
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the deployment was initiated
	PerformedAt string `json:"performedAt"`
}

// APIUndeploymentEventDTO is the wire format for API undeployment notifications.
type APIUndeploymentEventDTO struct {
	// ApiId identifies the undeployed API
	ApiId string `json:"apiId"`

	// DeploymentID identifies the specific deployment being undeployed
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the undeployment was initiated
	PerformedAt string `json:"performedAt"`
}

// LLMProviderDeploymentEventDTO is the wire format for LLM provider deployment notifications.
type LLMProviderDeploymentEventDTO struct {
	// ProviderId identifies the deployed LLM provider (handle)
	ProviderId string `json:"providerId"`

	// DeploymentID identifies the specific deployment artifact
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the deployment was initiated
	PerformedAt string `json:"performedAt"`
}

// LLMProviderUndeploymentEventDTO is the wire format for LLM provider undeployment notifications.
type LLMProviderUndeploymentEventDTO struct {
	// ProviderId identifies the undeployed LLM provider (handle)
	ProviderId string `json:"providerId"`

	// DeploymentID identifies the specific deployment being undeployed
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the undeployment was initiated
	PerformedAt string `json:"performedAt"`
}

// LLMProxyDeploymentEventDTO is the wire format for LLM proxy deployment notifications.
type LLMProxyDeploymentEventDTO struct {
	// ProxyId identifies the deployed LLM proxy (handle)
	ProxyId string `json:"proxyId"`

	// DeploymentID identifies the specific deployment artifact
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the deployment was initiated
	PerformedAt string `json:"performedAt"`
}

// LLMProxyUndeploymentEventDTO is the wire format for LLM proxy undeployment notifications.
type LLMProxyUndeploymentEventDTO struct {
	// ProxyId identifies the undeployed LLM proxy (handle)
	ProxyId string `json:"proxyId"`

	// DeploymentID identifies the specific deployment being undeployed
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`

	// PerformedAt is the timestamp when the undeployment was initiated
	PerformedAt string `json:"performedAt"`
}

// GatewayConfigEventDTO is the wire format for gateway configuration updates.
type GatewayConfigEventDTO struct {
	// ConfigType identifies the configuration category
	ConfigType string `json:"configType"`

	// Action specifies the configuration change action
	Action string `json:"action"`
}

// MCPProxyDeploymentEventDTO is the wire format for MCP proxy deployment notifications.
type MCPProxyDeploymentEventDTO struct {
	// ProxyId identifies the deployed MCP proxy (handle)
	ProxyId string `json:"proxyId"`

	// DeploymentID identifies the specific deployment artifact
	DeploymentID string `json:"deploymentId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`
}

// MCPProxyUndeploymentEventDTO is the wire format for MCP proxy undeployment notifications.
type MCPProxyUndeploymentEventDTO struct {
	// ProxyId identifies the undeployed MCP proxy (handle)
	ProxyId string `json:"proxyId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`
}

// MCPProxyDeletionEventDTO is the wire format for MCP proxy deletion notifications.
type MCPProxyDeletionEventDTO struct {
	// ProxyId identifies the deleted MCP proxy
	ProxyId string `json:"proxyId"`

	// Vhost specifies the virtual host
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`
}
