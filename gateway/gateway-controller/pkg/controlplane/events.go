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
	ProvisionedBy  *string `json:"provisionedBy,omitempty"`  // nil if not provided by the platform API
	AllowedTargets *string `json:"allowedTargets,omitempty"` // nil means use default 'ALL'
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
	ExternalRefId string  `json:"externalRefId"`
	ExpiresAt     *string `json:"expiresAt,omitempty"` // ISO 8601 format
	ExpiresIn     *struct {
		Duration int    `json:"duration,omitempty"`
		Unit     string `json:"unit,omitempty"`
	} `json:"expiresIn,omitempty"`
	ProvisionedBy  *string `json:"provisionedBy,omitempty"`  // nil if not provided by the platform API
	AllowedTargets *string `json:"allowedTargets,omitempty"` // nil means use default 'ALL'
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
