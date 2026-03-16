/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package controlplane

import "time"

// ConnectionAckMessage represents the acknowledgment message sent by the control plane
// upon successful WebSocket connection establishment
type ConnectionAckMessage struct {
	Type         string `json:"type"`         // Message type (always "connection.ack")
	GatewayID    string `json:"gatewayId"`    // Gateway UUID
	ConnectionID string `json:"connectionId"` // Unique connection UUID
	Timestamp    string `json:"timestamp"`    // RFC3339 timestamp
}

// APIDeployedEventPayload represents the payload of an API deployment event
type APIDeployedEventPayload struct {
	APIID        string `json:"apiId"`
	DeploymentID string `json:"deploymentId"`
	VHost        string `json:"vhost"`
}

// APIDeployedEvent represents the complete API deployment event
type APIDeployedEvent struct {
	Type          string                  `json:"type"`
	Payload       APIDeployedEventPayload `json:"payload"`
	Timestamp     string                  `json:"timestamp"`
	CorrelationID string                  `json:"correlationId"`
}

// LLMProviderDeployedEventPayload represents the payload of an LLM provider deployment event
type LLMProviderDeployedEventPayload struct {
	ProviderID   string `json:"providerId"`
	Environment  string `json:"environment"`
	DeploymentID string `json:"deploymentId"`
	VHost        string `json:"vhost"`
}

// LLMProviderDeployedEvent represents the complete LLM provider deployment event
type LLMProviderDeployedEvent struct {
	Type          string                          `json:"type"`
	Payload       LLMProviderDeployedEventPayload `json:"payload"`
	Timestamp     string                          `json:"timestamp"`
	CorrelationID string                          `json:"correlationId"`
}

// LLMProviderUndeployedEventPayload represents the payload of an LLM provider undeployment event
type LLMProviderUndeployedEventPayload struct {
	ProviderID  string `json:"providerId"`
	Environment string `json:"environment"`
	VHost       string `json:"vhost"`
}

// LLMProviderUndeployedEvent represents the complete LLM provider undeployment event
type LLMProviderUndeployedEvent struct {
	Type          string                            `json:"type"`
	Payload       LLMProviderUndeployedEventPayload `json:"payload"`
	Timestamp     string                            `json:"timestamp"`
	CorrelationID string                            `json:"correlationId"`
}

// LLMProxyDeployedEventPayload represents the payload of an LLM proxy deployment event
type LLMProxyDeployedEventPayload struct {
	ProxyID      string `json:"proxyId"`
	Environment  string `json:"environment"`
	DeploymentID string `json:"deploymentId"`
	VHost        string `json:"vhost"`
}

// LLMProxyDeployedEvent represents the complete LLM proxy deployment event
type LLMProxyDeployedEvent struct {
	Type          string                       `json:"type"`
	Payload       LLMProxyDeployedEventPayload `json:"payload"`
	Timestamp     string                       `json:"timestamp"`
	CorrelationID string                       `json:"correlationId"`
}

// LLMProxyUndeployedEventPayload represents the payload of an LLM proxy undeployment event
type LLMProxyUndeployedEventPayload struct {
	ProxyID     string `json:"proxyId"`
	Environment string `json:"environment"`
	VHost       string `json:"vhost"`
}

// LLMProxyUndeployedEvent represents the complete LLM proxy undeployment event
type LLMProxyUndeployedEvent struct {
	Type          string                         `json:"type"`
	Payload       LLMProxyUndeployedEventPayload `json:"payload"`
	Timestamp     string                         `json:"timestamp"`
	CorrelationID string                         `json:"correlationId"`
}

// APIUndeployedEventPayload represents the payload of an API undeployment event
type APIUndeployedEventPayload struct {
	APIID       string `json:"apiId"`
	Environment string `json:"environment"`
	VHost       string `json:"vhost"`
}

// APIUndeployedEvent represents the complete API undeployment event
type APIUndeployedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIUndeployedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
}

// APIDeletedEventPayload represents the payload of an API deletion event
type APIDeletedEventPayload struct {
	APIID string `json:"apiId"`
	VHost string `json:"vhost"`
}

// APIDeletedEvent represents the complete API deletion event
type APIDeletedEvent struct {
	Type          string                 `json:"type"`
	Payload       APIDeletedEventPayload `json:"payload"`
	Timestamp     string                 `json:"timestamp"`
	CorrelationID string                 `json:"correlationId"`
}

// APIKeyCreatedEventPayload represents the payload of an API key created event.
type APIKeyCreatedEventPayload struct {
	UUID          string  `json:"uuid"`           // UUID v7 from platform API for cross-system correlation
	ApiId         string  `json:"apiId"`
	ApiKeyHashes  string  `json:"apiKeyHashes"`   // JSON string of hashed API key values keyed by algorithm e.g. {"sha256": "<hash>"}
	MaskedApiKey  string  `json:"maskedApiKey"`   // Masked representation of the API key for display
	Name          string  `json:"name,omitempty"` // URL-safe identifier (3-63 chars, lowercase alphanumeric with hyphens)
	ExternalRefId *string `json:"externalRefId,omitempty"`
	ExpiresAt     *string `json:"expiresAt,omitempty"` // ISO 8601 format
	ExpiresIn     *struct {
		Duration int    `json:"duration,omitempty"`
		Unit     string `json:"unit,omitempty"`
	} `json:"expiresIn,omitempty"`
	Issuer         *string `json:"issuer,omitempty"` // nil if not provided by the platform API
}

// APIKeyCreatedEvent represents the complete API key created event
type APIKeyCreatedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIKeyCreatedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
	UserId        string                    `json:"userId"`
}

type APIKeyUpdatedEventPayload struct {
	ApiId         string  `json:"apiId"`
	KeyName       string  `json:"keyName"`
	ApiKeyHashes  string  `json:"apiKeyHashes"`  // JSON string of hashed API key values keyed by algorithm e.g. {"sha256": "<hash>"}
	MaskedApiKey  string  `json:"maskedApiKey"`  // Masked representation of the API key for display
	ExternalRefId *string  `json:"externalRefId"`
	ExpiresAt     *string `json:"expiresAt,omitempty"` // ISO 8601 format
	ExpiresIn     *struct {
		Duration int    `json:"duration,omitempty"`
		Unit     string `json:"unit,omitempty"`
	} `json:"expiresIn,omitempty"`
	Issuer         *string `json:"issuer,omitempty"` // nil if not provided by the platform API
}

// APIKeyUpdatedEvent represents the complete API key updated event
type APIKeyUpdatedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIKeyUpdatedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
	UserId        string                    `json:"userId"`
}

// APIKeyRevokedEventPayload represents the payload of an API key revoked event
type APIKeyRevokedEventPayload struct {
	ApiId   string `json:"apiId"`
	KeyName string `json:"keyName"`
}

// APIKeyRevokedEvent represents the complete API key revoked event
type APIKeyRevokedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIKeyRevokedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
	UserId        string                    `json:"userId"`
}

// MCPProxyDeployedEventPayload represents the payload of an MCP proxy deployment event
type MCPProxyDeployedEventPayload struct {
	ProxyID      string `json:"proxyId"`
	Environment  string `json:"environment"`
	DeploymentID string `json:"deploymentId"`
	VHost        string `json:"vhost"`
}

// MCPProxyDeployedEvent represents the complete MCP proxy deployment event
type MCPProxyDeployedEvent struct {
	Type          string                       `json:"type"`
	Payload       MCPProxyDeployedEventPayload `json:"payload"`
	Timestamp     string                       `json:"timestamp"`
	CorrelationID string                       `json:"correlationId"`
}

// MCPProxyUndeployedEventPayload represents the payload of an MCP proxy undeployment event
type MCPProxyUndeployedEventPayload struct {
	ProxyID     string `json:"proxyId"`
	Environment string `json:"environment"`
	VHost       string `json:"vhost"`
}

// MCPProxyUndeployedEvent represents the complete MCP proxy undeployment event
type MCPProxyUndeployedEvent struct {
	Type          string                         `json:"type"`
	Payload       MCPProxyUndeployedEventPayload `json:"payload"`
	Timestamp     string                         `json:"timestamp"`
	CorrelationID string                         `json:"correlationId"`
}

// MCPProxyDeletedEventPayload represents the payload of an MCP proxy deletion event
type MCPProxyDeletedEventPayload struct {
	ProxyID string `json:"proxyId"`
	VHost   string `json:"vhost"`
}

// MCPProxyDeletedEvent represents the complete MCP proxy deletion event
type MCPProxyDeletedEvent struct {
	Type          string                      `json:"type"`
	Payload       MCPProxyDeletedEventPayload `json:"payload"`
	Timestamp     string                      `json:"timestamp"`
	CorrelationID string                      `json:"correlationId"`
}

// SubscriptionCreatedEventPayload represents the payload of a subscription created event.
type SubscriptionCreatedEventPayload struct {
	APIID              string `json:"apiId"`
	SubscriptionID     string `json:"subscriptionId"`
	ApplicationID      string `json:"applicationId,omitempty"`
	SubscriptionToken  string `json:"subscriptionToken"`
	SubscriptionPlanId string `json:"subscriptionPlanId,omitempty"`
	Status             string `json:"status"`
}

// SubscriptionCreatedEvent represents the complete subscription.created event.
type SubscriptionCreatedEvent struct {
	Type          string                          `json:"type"`
	Payload       SubscriptionCreatedEventPayload `json:"payload"`
	Timestamp     string                          `json:"timestamp"`
	CorrelationID string                          `json:"correlationId"`
}

// SubscriptionUpdatedEventPayload represents the payload of a subscription updated event.
type SubscriptionUpdatedEventPayload struct {
	APIID              string `json:"apiId"`
	SubscriptionID     string `json:"subscriptionId"`
	ApplicationID      string `json:"applicationId,omitempty"`
	SubscriptionToken  string `json:"subscriptionToken"`
	SubscriptionPlanId string `json:"subscriptionPlanId,omitempty"`
	Status             string `json:"status"`
}

// SubscriptionUpdatedEvent represents the complete subscription.updated event.
type SubscriptionUpdatedEvent struct {
	Type          string                          `json:"type"`
	Payload       SubscriptionUpdatedEventPayload `json:"payload"`
	Timestamp     string                          `json:"timestamp"`
	CorrelationID string                          `json:"correlationId"`
}

// SubscriptionDeletedEventPayload represents the payload of a subscription deleted event.
type SubscriptionDeletedEventPayload struct {
	APIID             string `json:"apiId"`
	SubscriptionID    string `json:"subscriptionId"`
	ApplicationID     string `json:"applicationId,omitempty"`
	SubscriptionToken string `json:"subscriptionToken"`
}

// SubscriptionDeletedEvent represents the complete subscription.deleted event.
type SubscriptionDeletedEvent struct {
	Type          string                          `json:"type"`
	Payload       SubscriptionDeletedEventPayload `json:"payload"`
	Timestamp     string                          `json:"timestamp"`
	CorrelationID string                          `json:"correlationId"`
}

// SubscriptionPlanCreatedEventPayload represents the payload of a subscriptionPlan.created event.
type SubscriptionPlanCreatedEventPayload struct {
	PlanId             string     `json:"planId"`
	PlanName           string     `json:"planName"`
	BillingPlan        string     `json:"billingPlan,omitempty"`
	StopOnQuotaReach   bool       `json:"stopOnQuotaReach"`
	ThrottleLimitCount *int       `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  string     `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *time.Time `json:"expiryTime,omitempty"`
	Status             string     `json:"status"`
}

// SubscriptionPlanCreatedEvent represents the complete subscriptionPlan.created event.
type SubscriptionPlanCreatedEvent struct {
	Type          string                              `json:"type"`
	Payload       SubscriptionPlanCreatedEventPayload `json:"payload"`
	Timestamp     string                              `json:"timestamp"`
	CorrelationID string                              `json:"correlationId"`
}

// SubscriptionPlanUpdatedEventPayload represents the payload of a subscriptionPlan.updated event.
type SubscriptionPlanUpdatedEventPayload struct {
	PlanId             string     `json:"planId"`
	PlanName           string     `json:"planName"`
	BillingPlan        string     `json:"billingPlan,omitempty"`
	StopOnQuotaReach   bool       `json:"stopOnQuotaReach"`
	ThrottleLimitCount *int       `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  string     `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *time.Time `json:"expiryTime,omitempty"`
	Status             string     `json:"status"`
}

// SubscriptionPlanUpdatedEvent represents the complete subscriptionPlan.updated event.
type SubscriptionPlanUpdatedEvent struct {
	Type          string                              `json:"type"`
	Payload       SubscriptionPlanUpdatedEventPayload `json:"payload"`
	Timestamp     string                              `json:"timestamp"`
	CorrelationID string                              `json:"correlationId"`
}

// SubscriptionPlanDeletedEventPayload represents the payload of a subscriptionPlan.deleted event.
type SubscriptionPlanDeletedEventPayload struct {
	PlanId   string `json:"planId"`
	PlanName string `json:"planName"`
}

// SubscriptionPlanDeletedEvent represents the complete subscriptionPlan.deleted event.
type SubscriptionPlanDeletedEvent struct {
	Type          string                              `json:"type"`
	Payload       SubscriptionPlanDeletedEventPayload `json:"payload"`
	Timestamp     string                              `json:"timestamp"`
	CorrelationID string                              `json:"correlationId"`
}
